# Workflow 推进可靠性改造设计（边界1 + 边界2）

**Goal:** 消除工作流推进链路上两处「事务已提交、后续动作却可能不执行」的窗口，使一次节点完成要么完整推进（状态已更新 + 下游任务已入库），要么完全没发生并可重试；彻底失败时表现为 Admin 界面可见的死信，而不是静默的永久卡死。

**Architecture:** 问题的本质是 workflow 的推进跨越了两个「提交了但可能不执行」的边界，而这两个边界**两端都在同一个 Postgres 里**——所以它们本不该是边界。边界2（workflow 状态提交 → 下游任务派发）用**事务透传**直接消灭：`ExecutorClient` 是进程内直调，executor DAO 全线走 `mvc.ExtractDB(ctx, d.db)`，只要用 `mvc.WithTxToContext` 包一层 ctx，两次写入就是同一个事务。边界1（executor 的 `AckJob` → workflow 回调）跨模块且需要重试，用 **outbox 模式**：`AckJob` 事务内插一条 executor job 承载回调，由 aio 进程内的轻量 worker 消费。载体复用 executor 自身（重试退避、死信、Admin 可见性都是现成的），不新增任何调度机制。

**Tech Stack:** Go 1.26.1、GORM（MySQL/PostgreSQL 双兼容）、`pkg/core/mvc` 事务透传、`system/executor` 任务执行器、`system/workflow` 工作流引擎。

**范围边界：** 本次不引入消息队列（SQ 或其他）、不改 executor 的任务派发机制（仍为 1s 批量轮询）、不改动下游 worker（nova-audio-go / tk-server 无需升级 SDK 或重新部署）。

---

## 一、问题定义

### 边界1：`AckJob` → workflow 回调

`system/executor/internal/service/executor_job_service.go:369` 在 `AckJob` 中**同步、进程内**调用 `handler.OnJobCompleted(...)`：

```go
err := s.dao.AckJob(ctx, ...)   // 事务已提交
if status == Succeeded && source != "" {
    handler := s.handlers[source]
    if handler != nil {
        handler.OnJobCompleted(ctx, jobID, callbackData, resultJSON)  // ← 失败无重投
    }
}
```

**故障模式：** DB 事务提交成功后，回调 panic、返回错误、或进程崩溃 → 任务已是终态，但工作流永远收不到这次完成通知 → **实例永久卡死，无任何重试机制**。

### 边界2：workflow 状态提交 → 下游任务派发

`system/workflow/internal/app/engine.go:736` 在事务**提交之后**才触发下游节点：

```go
err := base.DB.Transaction(func(tx *gorm.DB) error { /* 收集 nextNodeIDs，提交状态 */ })
if err != nil { return ... }
// ↓ 事务已提交
for _, nextNodeID := range nextNodeIDs {
    a.triggerNode(ctx, &instance, node, &dag, env)   // ← 此处崩溃则无人补偿
}
```

**故障模式：** 事务提交后、`triggerNode` 执行前进程崩溃 → 实例 `Status=RUNNING`、`ActiveNodeIDs` 含下游节点，但**没有任何 executor job 被提交**。比边界1 更隐蔽：界面上看起来"正在运行"。

现有的补救逻辑（`engine.go:743`，触发失败则把实例置 `FAILED`）只覆盖"`triggerNode` 返回错误"，覆盖不了进程崩溃。

### 为什么现在必须处理

nova 的三个工作流定义（`nova_extractor` 23 节点 / `nova_replicator` 22 节点含 7 个 map 节点 / `nova_review` 19 节点）中，单个节点是分钟级到小时级的音视频与 AI 任务（`TaskTimeout` 实测最高 5 小时）。一次卡死的代价是数小时算力白费 + 人工排查介入。

---

## 二、边界2 设计：事务透传

### 2.1 改动本体

将 `ReportNodeCompleted` 中事务外的 `triggerNode` 循环移入事务，ctx 用 `mvc.WithTxToContext(ctx, tx)` 包装：

```go
err := mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
    // ... 现有推进逻辑，收集 nextNodeIDs ...

    // 事务透传：SubmitJob 与状态更新落在同一事务
    txCtx := mvc.WithTxToContext(ctx, tx)
    for _, nextNodeID := range nextNodeIDs {
        node := dag.GetNode(nextNodeID)
        if node == nil {
            continue
        }
        if err := a.triggerNode(txCtx, &instance, node, &dag, env); err != nil {
            return err   // 整体回滚：状态不推进，下游任务也不入库
        }
    }
    return nil
})
```

透传链路成立的依据：

| 环节 | 位置 | 事实 |
|---|---|---|
| `ExecutorClient.SubmitJob` | `system/executor/api/client/executor_client.go:25` | 进程内直调 `c.app.JobService.SubmitJob`，不走 gRPC |
| executor DAO | `system/executor/internal/dao/executor_job_dao.go:33` 等 18 处 | 全线 `mvc.ExtractDB(ctx, d.db)` |
| 事务注入 | `pkg/core/mvc/tx_context.go` | `WithTxToContext` / `ExtractDB` 现成 |

### 2.2 语义变化（这是改进，不是副作用）

触发失败从「把实例置为 `FAILED`」改为「整个事务回滚」。回滚后实例保持推进前的状态，由边界1 的 outbox job 重试兜底。原子性从「尽力补救」升级为「要么全有要么全无」。

删除 `engine.go:743` 起的防假死补救分支——事务回滚后不再需要它。

### 2.3 必须同步处理：5 处硬编码 `base.DB.Transaction`

事务透传后，这些入口若仍用 `base.DB` 会开**独立事务**，而外层事务正持有该实例行的 `FOR UPDATE` 锁 → **死锁**。

| 位置 | 方法 | 何时被嵌套调用 |
|---|---|---|
| `engine.go:415` | `ReportNodeCompleted` | condition 节点递归（`engine.go:810`） |
| `engine.go:958` | `updateInstanceStatusToWaitingWithLock` | approval 节点触发（`engine.go:806`） |
| `engine.go:1003` | `RollbackToNode` | 当前不会，防御性改造 |
| `engine.go:1264` | `RetryNode` | 当前不会，防御性改造 |
| `engine.go:1341` | `SendSignal` | 当前不会，防御性改造 |

统一改为 `mvc.ExtractDB(ctx, base.DB).Transaction(...)`：

- 有外层事务时 → GORM 使用 SavePoint（同连接、同事务），行锁可重入，不死锁
- 无外层事务时 → 行为与改造前**完全一致**

前两处是死锁必现路径，后三处是防御性改造，一并处理避免以后再踩。

### 2.4 已知代价

map 节点扇出 N 个子任务时，N 条 job INSERT 现在都在事务内，实例行锁多持有数十毫秒（本地同连接写入）。同一实例本就不应并发推进，可接受。

---

## 三、边界1 设计：outbox job + 内部 worker

### 3.1 outbox job 形态

`AckJob` 事务内提交一条普通 executor job 承载回调：

```go
SubmitJobInput{
    Env:           job.Env,
    TargetService: "aio",
    Method:        "internal.job_completed_callback",
    ArgsJSON:      `{"job_id":..,"source":"workflow","callback_data":..,"result_json":..}`,
    DedupKey:      fmt.Sprintf("jobcb_%d", jobID),
    MaxAttempts:   5,
    Priority:      10,   // 高于普通任务，尽快被领取
    Source:        "",   // 必须为空
}
```

**`Source` 必须为空**：否则这条回调 job 自身 Ack 时会再触发一次回调，无限递归。此约束需在代码注释中写明原因。

**method 刻意通用化**：不写成 `workflow.on_job_completed`，而是 `internal.job_completed_callback`，`source` 放进 args 由内部 worker 路由。这样 `RegisterJobCompletionHandler(source, handler)`（`executor_job_service.go:45`，`app/app.go:70` 注册 workflow）这个扩展点的语义原封不动保留，将来其他模块注册回调走同一条路。

### 3.2 `AckJob` 的事务包装

`s.dao.AckJob` 内部自行开事务（`executor_job_dao.go:187/212`）。在 service 层包一层外层事务，将 Ack、状态回读、outbox 提交纳入同一事务：

```go
err := mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
    txCtx := mvc.WithTxToContext(ctx, tx)
    if err := s.dao.AckJob(txCtx, jobID, attemptNo, consumerID, status, ...); err != nil {
        return err
    }
    // 回读判定是否需要回调：成功 或 重试耗尽转 dead
    // 需要则 s.SubmitJob(txCtx, outboxInput)
    return nil
})
```

DAO 内部的 `Transaction` 降级为 SavePoint。回调触发条件与现有逻辑保持一致：

- `status == Succeeded && source != ""` → 提交 outbox
- `status == Failed && source != ""` 且回读后 `Status == Dead` → 提交 outbox（携带 `{"error_msg": ...}`）

### 3.3 内部 worker

新文件 `system/executor/internal/worker/internal_worker.go`（约 100 行）。

| 属性 | 取值 | 理由 |
|---|---|---|
| 调用方式 | 直调 `JobService.AcquireJobs` / `AckJob` | 进程内，省网络层，无启动顺序依赖 |
| targetService | `"aio"` | 固定 |
| methods | `["internal.job_completed_callback"]` | 固定 |
| env | `base.ENV` | 见 §4.1 约束 |
| consumerID | 优先用 registry 自注册的 `instanceKey`（`system/registry/module.go:156`）；自注册未启用时降级为 `hostname-pid` | 多实例区分，且与 registry 中的实例身份对齐便于排查 |
| 轮询间隔 | 1s | 与 SDK worker 一致 |
| 并发度 | 5 | 回调都是短任务 |
| 生命周期 | 由 `executor.Module` 启停，随 `main.go` 的 `RegisterClose` 优雅停止 | 与现有模式一致 |

执行流程：领取 job → 解析 args → 按 `source` 查 `s.handlers[source]` → 调 `handler.OnJobCompleted(ctx, jobID, callbackData, resultJSON)` → 成功 `AckJob(Succeeded)`，失败 `AckJob(Failed)` 交由 executor 重试。

**多实例安全**：每个 aio 实例各跑一个内部 worker，`consumerID` 不同；`AcquireJobs` 的 `FOR UPDATE SKIP LOCKED` 天然去重，无需额外协调。

### 3.4 幂等（硬要求）

`DedupKey` 只保证**入队一次**，不保证**执行一次**——outbox job 失败重试时会重复调用 handler。而 `applyStateReducer`（`engine.go:82`）的 `append` 模式重放会把 output **再追加一遍**，造成状态污染（nova 的 map 节点大量使用该模式汇聚结果）。

新建幂等表 `aio_workflow_applied_callback`：

```go
type WorkflowAppliedCallbackModel struct {
    common.Model
    JobID      int64  `gorm:"column:job_id;not null;uniqueIndex:idx_wac_job_id" comment:"来源任务ID，幂等键"`
    InstanceID int64  `gorm:"column:instance_id;not null;index:idx_wac_instance" comment:"工作流实例ID"`
    NodeID     string `gorm:"column:node_id;size:100;not null" comment:"节点ID"`
}
```

在 `ReportNodeCompleted` 事务**最开头** INSERT；唯一键冲突 → 该回调已处理，直接返回 nil（让 outbox job Ack 成功，不再重试）。

**不复用 checkpoint 表做判据**：loopback 边有意允许同一节点多次 checkpoint（`edgeTargetEligibleForScheduling`，`engine.go:328`），`(instance_id, node_id)` 维度不唯一，无法用作幂等键。

**签名不破坏**：`ReportNodeCompleted` 现有三类调用方——回调（有 jobID）、condition 递归（无）、approval 人工推进（无）。新增 `ReportNodeCompletedFromJob(ctx, jobID, instanceID, nodeID, output, env, subJobID)`，内部与现有实现共用，幂等标记仅在 `jobID > 0` 时插入。旧签名对外契约不变。

清理：幂等表记录挂到现有的 executor 清理 cron（`main.go:148`）一并清理。

---

## 四、约束与回退

### 4.1 env 约束

内部 worker 只处理 `base.ENV` 的 outbox job。跨 env 的 workflow 回调需要对应 env 的 aio 实例在运行。

这一约束本就成立（dev 环境的任务本就该由 dev 侧执行），但需在代码注释与运维文档中明确，避免"prod 的 aio 上起了 env=dev 的 workflow，回调无人消费"这类排查困难的情况。

### 4.2 回调彻底失败的表现

5 次重试耗尽 → outbox job 转 `dead` 状态 → Admin 界面任务列表可见，可手动 Requeue。

这是本次改造的核心收益：**从「静默永久卡死」变为「可见的死信」**。

### 4.3 回退开关

新增配置段 `workflow`，含一个字段 `callback-mode`：

```yaml
workflow:
  callback-mode: ""   # "" 或 "outbox"（默认）→ 走 outbox；"sync" → 退回同步回调
```

**刻意用字符串而非 bool**：`pkg/core/start.Config` 是 struct 反序列化，bool 的零值是 `false`。若定义成 `callback_via_outbox bool` 并声称"默认 true"，配置缺失时实际会得到 `false`，与文档相反——这类零值陷阱正是需要在设计阶段消灭的。字符串空值语义明确：**未配置 = 新行为**，要退回必须显式写 `sync`。

落点：

```go
// pkg/core/config/workflow.go（新增）
type WorkflowConfig struct {
    CallbackMode string `yaml:"callback-mode" json:"callback-mode"`
}

// pkg/core/start/config.go：Config 结构体新增字段
Workflow config.WorkflowConfig `yaml:"workflow"`
```

注意 `pkg/core/start.Config` 被下游项目复用（见该文件 `NewConfiguresFromConfig` 注释），新增字段必须是纯增量且零值可用——上述设计满足。

边界2 的事务透传是结构性改动，无法用开关控制，靠测试保证。

该开关是**过渡措施**，稳定运行一个版本后删除，需在代码注释中标注这一意图，避免长期滞留成为技术债。

### 4.4 明确不做

- **存量卡死实例不自动救活**。改造前已卡死的实例需另写一次性修复脚本，不在本次范围。
- **不改任务派发机制**。executor 仍为 1s 批量轮询；按真实负载测算（个位数 worker、内网、全部与 DB 同机、工作流总时长小时级），推送唤醒的收益占比不足 1%，ROI 为负，已明确排除。
- **不引入消息队列**。两个边界两端都在同一个 Postgres，引入 MQ 等于在有单库事务可用处硬造第二个事务域。

### 4.5 顺手清理

`engine.go:432` 的 `fmt.Printf` 改为 `logger`（违反 CLAUDE.md 日志规范）。

---

## 五、测试策略

按项目 TDD 规范，先写测试再实现。

| 用例 | 断言 |
|---|---|
| 同 jobID 回调执行两次 | `append` 模式的 state 只追加一次；第二次命中幂等表跳过 |
| `triggerNode` 中途失败 | 实例状态不变、无 job 残留、无 checkpoint 残留 |
| condition 节点递归 | 嵌套事务不死锁——**必须实际运行验证，不可仅靠代码审查** |
| approval 节点触发 | `updateInstanceStatusToWaitingWithLock` 嵌套不死锁 |
| map 节点扇出 N 个子任务 | N 条 job 与实例状态在同一事务提交 |
| 两个内部 worker 并发 | 同一条 outbox job 只被消费一次 |
| `callback-mode: sync` | 退回同步路径，行为与改造前一致 |
| `callback-mode` 未配置 | 默认走 outbox —— 验证零值语义与 §4.3 一致 |
| AckJob 成功 / 转 dead 两条分支 | 均正确提交 outbox job，且 `Source` 为空 |

数据库兼容性：所有测试需在 MySQL 与 PostgreSQL 两种方言下通过（SavePoint 行为、唯一键冲突错误码存在差异）。

---

## 六、可观测性（instrumenting-code 强制项）

**关键节点日志：**

| 位置 | 级别 | 字段 |
|---|---|---|
| outbox job 入队 | Info | job_id、instance_id、node_id、dedup_key、source |
| 内部 worker 领取 | Debug | job_id、consumer_id、method |
| handler 执行完成 | Info | job_id、source、耗时、结果 |
| **幂等命中跳过** | **Info** | job_id、instance_id、node_id |
| 推进事务回滚 | Error | instance_id、node_id、失败节点、完整错误链 |
| outbox job 转 dead | Error | job_id、attempts、last_error |

幂等命中必须打日志：否则重复回调静默发生，排查时完全看不见。

**注释要求：**

- 新文件（`internal_worker.go`、幂等表 model）顶部写职责与边界
- 新增导出方法（`ReportNodeCompletedFromJob` 等）写参数、返回、注意事项
- 以下三处必须有「为什么」注释：`Source` 留空的原因（防回调递归）、`ExtractDB(...).Transaction` 替换 `base.DB.Transaction` 的原因（防嵌套死锁）、回退开关的过渡性质与删除计划

---

## 七、涉及文件清单

**新增：**

- `system/executor/internal/worker/internal_worker.go` — 内部回调 worker
- `system/workflow/internal/model/workflow_applied_callback.go` — 幂等表 model

**修改：**

| 文件 | 改动 |
|---|---|
| `system/workflow/internal/app/engine.go` | `triggerNode` 移入事务 + 事务透传；5 处 `base.DB.Transaction` → `mvc.ExtractDB(...)`；新增 `ReportNodeCompletedFromJob`；幂等表插入；删除防假死补救分支；`fmt.Printf` → logger |
| `system/executor/internal/service/executor_job_service.go` | `AckJob` 包外层事务；同步回调 → 提交 outbox job（受 `callback-mode` 控制） |
| `system/executor/module.go` | 启停内部 worker |
| `system/workflow/migrate.go` | `AutoMigrate` 列表加入 `WorkflowAppliedCallbackModel` |
| `main.go` | 清理 cron（`main.go:148`）增加幂等表清理 |
| `pkg/core/config/workflow.go` | **新增**：`WorkflowConfig{ CallbackMode string }` |
| `pkg/core/start/config.go` | `Config` 新增 `Workflow config.WorkflowConfig` 字段 |
| `resources/config.yaml.example` | 补 `workflow.callback-mode` 示例与注释 |

**不改动：** `pkg/sdk/**`（下游 worker 无需升级 SDK 或重新部署）、executor 的任务派发与租约机制、proto 定义。
