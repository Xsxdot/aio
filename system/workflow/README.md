# Workflow 工作流组件

轻量级工作流引擎，支持 AI Agent 编排、人工审核、状态回滚。与 `system/executor` 配合使用。

## 能力概览

- **动态定义**：流程通过 JSON 下发，支持 task、approval、condition 三种节点类型
- **与 Executor 联动**：Task 节点自动提交到 Executor 异步执行
- **人工介入**：Approval 节点挂起流程，等待 API 提交审核结果后继续
- **状态回滚**：基于 Checkpoint 快照，支持退回到指定节点重新执行
- **执行轨迹**：提供完整执行历史，供前端绘制 LangGraph 风格的过程动画

## API 一览

### 管理端（`/admin/workflow`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /defs | 创建工作流定义 |
| GET | /instances/:id | 获取实例详情 |
| GET | /instances/:id/trail | 获取执行轨迹 |
| POST | /instances/:id/rollback | 回滚到指定节点（Body: `{"target_node_id":"xxx"}`） |

## DAG 定义示例

```json
{
  "nodes": [
    {"id": "A", "type": "task", "config": {"service": "ai_agent", "method": "generate"}},
    {"id": "B", "type": "approval"},
    {"id": "C", "type": "condition"}
  ],
  "edges": [
    {"from": "A", "to": "B"},
    {"from": "B", "to": "C", "condition": "state.approved == true"}
  ]
}
```

条件表达式使用 [expr](https://github.com/expr-lang/expr)，可通过 `state.xxx` 访问当前全局状态。

## Executor Worker 回调

Worker 执行完 Task 节点后，需调用 Workflow 的 `/api/workflow/v1/report` 接口推进流程：

```json
POST /api/workflow/v1/report
{"instance_id": 1, "node_id": "A", "output": {"generated_text": "..."}}
```

## 跨组件调用

其他组件通过 `app.WorkflowModule.Client` 调用：

```go
instanceID, err := app.WorkflowModule.Client.StartWorkflow(ctx, "my_flow", map[string]interface{}{"input": "hello"})
```
