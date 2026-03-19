# Workflow 工作流引擎 (V2 高阶版)

轻量级、强类型安全、支持高并发编排的工作流引擎。专为复杂 AI Agent 调度、Map-Reduce 并发处理、人机协同（Human-in-the-loop）以及强状态隔离设计。需与 `system/executor` 配合使用。

## ✨ 核心特性

* **动态扇出与聚合 (Map-Reduce)**：原生支持基于数组状态动态裂变并发子任务，并利用底层行锁自动安全聚合结果。
* **安全状态归约器 (State Reducer)**：摒弃粗暴的状态覆盖，提供 `overwrite`、`append`、`deep_merge` 三种并发安全的状态合并策略。
* **柔性降级与异常路由 (Error Catch)**：单节点彻底失败时不会直接宕机，可无缝路由至降级容错节点。
* **合法的循环重试 (Loopback)**：打破传统 DAG 限制，允许配置安全的回退边，支持打回重做或多轮交互。
* **热更新与信号总线 (Signal)**：支持在不重启工作流的情况下，通过 API 注入局部数据并唤醒特定节点继续执行。
* **时光机回滚**：支持随时逆向回滚至历史特定节点状态。

---

## 🏗️ 核心概念

工作流以 DAG（有向无环图，包含特许的 Loopback 边）形式定义：

* **Node (节点)**：执行单元，支持普通任务、并发 Map、人工审批、条件网关。
* **Edge (边)**：节点间的流转路径，支持条件判断（基于 Expr 表达式）及异常捕获路由。
* **State (全局状态)**：由引擎托管的 `CurrentState`，划分为业务数据区 (`data`) 与引擎系统区 (`_sys`)，确保业务数据与并发控制计数器严格物理隔离。

---

## 📦 节点配置 (Node Types)

### 1. Task 节点 (异步任务)

将任务分发给 Executor 执行器，适用于各类后台计算或 AI Agent 调用。

```json
{
  "id": "extract_data",
  "type": "task",
  "config": {
    "service": "data_agent",
    "method": "extract",
    "state_update_mode": "deep_merge"
  }
}

```

### 2. Map 节点 (并发裂变与聚合)

根据前置节点产生的数据数组，动态拉起 N 个并发子任务。全部成功后向后流转。

```json
{
  "id": "parallel_analysis",
  "type": "map",
  "config": {
    "items_path": "state.raw_segments", 
    "item_alias": "segment",           
    "output_path": "state.analysis_results",
    "iterator": {                      
        "service": "ai_agent",
        "method": "analyze_segment"
    }
  }
}

```

### 3. Condition 节点 (条件网关)

自身不产生任务，仅用于评估出边的条件并决定后续路由。

```json
{
  "id": "check_gateway",
  "type": "condition"
}

```

### 4. Approval 节点 (人工审批)

使工作流挂起（进入 `WAITING` 状态），等待人类介入确认或提供数据。

```json
{
  "id": "human_review",
  "type": "approval"
}

```

---

## 🛤️ 边与路由配置 (Edge Config)

在 `edges` 数组中定义流转规则：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `from` | string | 源节点 ID |
| `to` | string | 目标节点 ID |
| `condition` | string | 可选。使用 expr 引擎评估，如 `state.score >= 80` |
| `type` | string | 可选。`""` (默认，成功时走此边)、`"error"` (Executor抛出彻底失败时走此降级边)、`"always"` |
| `is_loopback` | bool | 可选。声明为 `true` 表示这是一条合法的循环重试边（如打回重审），引擎将豁免其环路检测 |

---

## 💾 状态管理器 (State Reducer)

为了应对多任务并发回调时的数据覆盖灾难，引擎引入了 Reducer 机制。在节点的 `config` 中可配置 `state_update_mode`：

* **`overwrite` (默认)**：粗暴覆盖。若 `output` 为 `{"a": 1}`，将直接覆盖全局状态的 `a`。
* **`append` (并发场景必选)**：追加模式。当多个 Agent 并发结束返回相同的 key 时，引擎会在数据库行级悲观锁内，将其安全地组装为 Slice 数组（如 `[]interface{}`）。
* **`deep_merge`**：深度合并。针对深层嵌套的 Struct/JSON 更新。

---

## 🚦 信号总线与人工干预 (Signal API)

当节点执行降级或处于 `WAITING`、`COMPLETED` 状态时，可通过外部 API 注入热修复数据，并在不重启整个实例的情况下复苏局部节点。

```go
// 外部业务通过 SDK 发送信号
err := client.Workflow.SendSignal(
    ctx, 
    instanceID, 
    "patch_missing_image", // 信号名
    map[string]interface{}{"image_url": "https://..."}, // Payload，将被安全 merge 进当前状态
    "aggregator_node", // 指定唤醒的目标节点
    "prod",
)

```

---

## 💻 快速开始 (SDK 使用)

### 1. 注册与定义 DAG

```go
dagJSON := `{
    "nodes": [
        {"id": "fetch_data", "type": "task", "config": {"service": "crawler", "method": "fetch"}},
        {"id": "parallel_process", "type": "map", "config": {
            "items_path": "state.urls",
            "item_alias": "url",
            "state_update_mode": "append",
            "iterator": {"service": "processor", "method": "parse"}
        }},
        {"id": "error_handler", "type": "task", "config": {"service": "alert", "method": "notify"}},
        {"id": "summary", "type": "task", "config": {"service": "ai", "method": "summarize"}}
    ],
    "edges": [
        {"from": "fetch_data", "to": "parallel_process"},
        {"from": "parallel_process", "to": "summary"},
        {"from": "parallel_process", "to": "error_handler", "type": "error"} 
    ]
}`

// 创建或更新定义
defID, _, err := client.Workflow.CreateIfNotExists(ctx, "data_pipeline", "数据抓取流", dagJSON, 1)

```

### 2. 启动工作流

```go
initialData := map[string]interface{}{
    "search_keyword": "golang workflow engine",
}
// 启动，最后参数为环境隔离标识 (env)
instanceID, err := client.Workflow.StartWorkflow(ctx, "data_pipeline", initialData, "prod")

```

### 3. Worker 处理与状态回调 (Executor 侧)

```go
// Executor 的 Worker 注册方法
worker.Register("parse", func(ctx context.Context, job *sdk.AcquiredJob) (interface{}, error) {
    // 引擎自动注入的参数
    var args struct {
        InstanceID int64                  `json:"instance_id"`
        NodeID     string                 `json:"node_id"`
        State      map[string]interface{} `json:"state"` // 已自动过滤敏感前缀字段
        URL        string                 `json:"url"`   // Map 节点注入的 item_alias
    }
    json.Unmarshal([]byte(job.ArgsJSON), &args)

    // 执行业务逻辑...
    result := process(args.URL)

    // 返回的 map 将通过 Executor 回调总线，被引擎的 Reducer 合并进全局状态
    return map[string]interface{}{
        "parsed_entities": result,
    }, nil
    
    // 如果抛出 &sdk.JobFailedError{StopRetry: true, Message: "解析崩溃"}
    // Workflow 引擎将捕获此错误，尝试寻找 type="error" 的边进行降级路由。
})

```

### 4. 获取执行轨迹

```go
// 可用于前端绘制全息监控图
trail, _ := client.Workflow.GetExecutionTrail(ctx, instanceID)
for _, cp := range trail.Checkpoints {
    fmt.Printf("节点: %s | 状态: %v | 完成时间: %s\n", 
        cp.NodeID, cp.NodeOutput, cp.CreatedAt)
}

```