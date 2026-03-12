# 工作流 (Workflow) 组件架构设计文档

## 1. 业务目标与背景
实现一个轻量级、动态化、支持 AI Agent 编排的工作流引擎（类似于 LangGraph / Temporal）。
本组件运行在 `AIO` 项目中作为 Server 端，工作流的定义由另一个外部项目负责定义并下发。

### 核心能力要求
1. **动态定义**：流程定义基于 JSON 动态加载，不写死在代码中。
2. **与 Executor 结合**：底层的自动化任务派发基于现有的 `system/executor` 执行异步任务。
3. **人工介入 (Human-in-the-loop)**：支持审核节点、表单填写节点，执行到该节点时挂起流程，等待外部 API 唤醒。
4. **状态回滚与重做**：记录每一步的状态快照（Checkpoint），支持将工作流退回到历史的某个节点重新执行。
5. **全局状态 (State)**：类似于 LangGraph，整个工作流共享一个 State (JSON)，每个节点根据输入执行，并返回增量输出 (Update)，引擎负责合并 State。

---

## 2. 核心概念模型

### 2.1 节点类型 (Node Types)
*   **`Task` (自动任务)**：最常见的 AI 任务或 API 调用。引擎执行到此节点时，通过 `ExecutorClient` 提交一个 Job。Worker 拿到 Job 执行完成后，回调 Workflow 引擎更新状态并继续推进。
*   **`Approval / Wait` (人工节点)**：引擎执行到此节点时，将实例状态标记为 `WAITING`，不再自动向下流转。直到用户通过 API 提交审核结果，合并 State 后，引擎被唤醒并继续计算下一步。
*   **`Condition / Router` (条件路由)**：不执行具体操作，仅根据当前全局 State 的值，判断走哪一条 Edge。

### 2.2 数据模型定义 (DB Tables)

为了满足动态化和快速迭代，定义部分我们倾向于将整个 DAG（有向无环图）存储为大 JSON，而不是拆分成 Node/Edge 多张表。

#### 1. `workflow_def` (工作流定义表)
存储外部系统下发的流程模板。
*   `id` / `code`: 模板唯一标识
*   `version`: 版本号
*   `name`: 名称
*   `dag_json`: 包含所有的 Node 和 Edge 的结构（JSON）。
    *   例如：`{"nodes": [{"id":"node_a", "type":"task", "service":"ai_agent", "method":"generate_text"}], "edges": [{"from":"node_a", "to":"node_b", "condition":"state.score > 50"}]}`

#### 2. `workflow_instance` (工作流实例表)
每一次触发执行产生一个实例。
*   `id`: 实例 ID
*   `def_id` / `def_version`: 关联的定义
*   `status`: `RUNNING`, `WAITING` (等待人工), `COMPLETED`, `FAILED`, `CANCELED`
*   `current_state`: 当前最新的全局状态 (JSON)
*   `active_node_ids`: 当前正在执行的节点列表 (JSON 数组，支持并发分支)

#### 3. `workflow_checkpoint` (执行快照/回溯表)
**（核心表：用于实现回滚与追踪）**
记录每一个节点执行完成后的状态切片。
*   `id`: 快照 ID
*   `instance_id`: 实例 ID
*   `node_id`: 执行完毕的节点 ID
*   `node_output`: 节点执行产生的增量输出 (JSON)
*   `state_after`: 执行完毕合并后的完整 State (JSON)
*   `created_at`: 执行时间

### 2.3 存储策略与 State 设计规范

采用 MySQL 存储 JSON 时，需避免「状态爆炸」问题。遵循以下规范：

*   **State 轻量化原则**：`current_state`、`state_after`、`node_output` 等 JSON 字段应尽量控制在 100KB 以内。禁止在 State 中直接存放超过该阈值的业务 Payload（如长文本、文件 base64）。
*   **Claim Check 模式**：大体积数据（长文、提取的 PDF 片段、多轮完整对话历史）应先上传至 `base.OSS` 或文件系统，在 State 中仅保存引用（URL / Path / ID）。
*   **可选：Redis 热状态**：实例处于 `RUNNING` 时，可将最新 State 缓存于 `base.Cache`，引擎高频读写优先走 Redis；在关键节点或 `WAITING` 时再落库 `workflow_checkpoint`。
*   **Agent 状态修剪**：可设计 Summarizer 节点，在对话历史过长时由 LLM 产出摘要，用摘要覆盖原 State 中的大数组，兼顾 Token 成本和存储体积。

---

## 3. 核心引擎流转逻辑 (Engine Loop)

工作流的核心是一个**状态机循环**，主要在 `system/workflow/internal/app/engine.go` 中实现：

1.  **触发流转 (Advance)**：每当实例启动，或者收到 Executor/人工 的回调时，触发 `Advance(instanceId)`。
2.  **节点完成处理**：合并来自上一步的输出到 `current_state`，并生成 `workflow_checkpoint` 快照。
3.  **计算下一节点**：
    *   读取 `dag_json`。
    *   根据当前刚完成的节点，找到它的出边 (`Edges`)。
    *   评估出边的条件表达式（通过类似 `expr` 或简单的规则引擎），决定下一个/下一批要激活的 `Node ID`。
4.  **执行下一节点**：
    *   如果下一节点是 **Task**：调用 `ExecutorClient.SubmitJob()` 异步下发，将当前 `instanceId` 和 `nodeId` 作为业务参数。当前流转挂起，等待 Executor Worker 回调。
    *   如果下一节点是 **Approval**：将 `workflow_instance.status` 更新为 `WAITING`。流转挂起，等待业务端调用 `ApproveNode(instanceId, nodeId, result)`。
    *   如果是 **End**：更新实例状态为 `COMPLETED`。

---

## 4. 重点场景实现方案

### 4.1 如何与 `system/executor` 配合？
*   **提交时**：Workflow 引擎作为 Client。当需要执行 AI 节点时，将 `state` 和 `node.config` 序列化为 `argsJSON`，调用 `c.app.ExecutorClient.SubmitJob(...)`。
*   **执行时**：Worker 端（你自定义的 AI 调用端）接收到 Job，执行大模型调用。
*   **完成时**：Worker 执行完成后，不要只返回给 Executor，而是需要**调用 Workflow 提供的专门接口**（比如 `ReportNodeCompleted(instanceId, nodeId, output)`）。
*   *(备选方案)*：Executor 自身支持类似 Webhook 或者集成事件总线，Workflow 监听 Executor 的任务完成事件来推进。但为了简单起见，主动调用接口是最稳妥的。

### 4.2 如何实现“退回重做 (Rollback)”？
业务需求：用户在 B 节点，发现前置的 A 节点生成得不好，要求退回到 A 重新执行。
**实现步骤**：
1.  用户发起退回请求：`RollbackToNode(instanceId, targetNodeId="A")`。
2.  查询 `workflow_checkpoint` 表，找到 `targetNodeId` 之前最近的一个快照（或者如果 A 是起始节点，则使用初始 State）。
3.  用历史快照中的 `state_after` 覆盖实例当前的 `current_state`。
4.  如果是从 Executor 撤回，还需要调用 `ExecutorClient.CancelJob()` 取消掉正在执行的无用任务。
5.  将实例的 `active_node_ids` 重置为 `["A"]`。
6.  触发引擎 `Advance` 逻辑，A 节点将被重新派发给 Executor 执行。

---

## 5. 项目模块实施拆解 (实施计划)

我们将分批次实施这个架构，避免一次性变动过大：

### 第一阶段：基础模型与存储层 (Model & DAO)
*   建立 `system/workflow/internal/model` 目录，定义 `WorkflowDef`, `WorkflowInstance`, `WorkflowCheckpoint`。
*   建立对应的 DAO 层。
*   配置数据库迁移 (`migrate.go`)。

### 第二阶段：核心状态机与流转引擎 (Engine)
*   引入轻量级的条件表达式评估库（如 `github.com/antonmedv/expr`，用于解析 Edge condition）。
*   在 `internal/app/engine.go` 编写核心的图遍历算法、State 合并逻辑（如 JSON Patch / 深拷贝合并）。

### 第三阶段：外部集成与 API 暴露 (Facade & Controller)
*   集成 `system/executor` 的 Client。
*   实现节点状态报告接口 `ReportTaskCompleted`。
*   实现人工审核介入接口 `SubmitApproval`。
*   暴露接口供外部项目下发定义和创建实例。

### 第四阶段：回溯与历史能力
*   实现 `RollbackToNode` 逻辑。
*   提供获取 Workflow 完整执行轨迹的接口（前端可以据此画出 LangGraph 一样的执行过程动画）。