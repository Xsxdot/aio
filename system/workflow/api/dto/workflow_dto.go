package dto

// CreateDefRequest 创建工作流定义请求
type CreateDefRequest struct {
	Env     string `json:"env"` // 环境标识
	Code    string `json:"code" validate:"required"`
	Version int32  `json:"version"`
	Name    string `json:"name" validate:"required"`
	DAGJSON string `json:"dag_json" validate:"required"`
}

// StartWorkflowRequest 启动工作流请求
type StartWorkflowRequest struct {
	DefCode string                 `json:"def_code" validate:"required"`
	Initial map[string]interface{} `json:"initial"`
}

// ReportNodeCompletedRequest 报告节点完成请求（Executor Worker 回调）
type ReportNodeCompletedRequest struct {
	InstanceID int64                  `json:"instance_id" validate:"required"`
	NodeID     string                 `json:"node_id" validate:"required"`
	Output     map[string]interface{} `json:"output"`
}

// SubmitApprovalRequest 提交人工审核请求
type SubmitApprovalRequest struct {
	InstanceID int64                  `json:"instance_id" validate:"required"`
	NodeID     string                 `json:"node_id" validate:"required"`
	Result     map[string]interface{} `json:"result"`
}

// RollbackRequest 回滚请求（instance_id 来自 URL 路径）
type RollbackRequest struct {
	TargetNodeID string `json:"target_node_id" validate:"required"`
	Env          string `json:"env"` // 环境标识（如 dev/prod/test），空则用进程默认环境
}

// SignalRequest 信号请求（Human-in-the-loop 热更新干预，instance_id 来自 URL 路径）
type SignalRequest struct {
	SignalName string                 `json:"signal_name" validate:"required"` // 如 "patch_product_image"
	Payload    map[string]interface{} `json:"payload"`                         // 补传数据，合并入 state
	WakeupNode string                 `json:"wakeup_node"`                     // 唤醒的目标节点，空则不唤醒
	Env        string                 `json:"env"`                             // 环境标识，空则用实例 env
}

// ExecutionTrail 执行轨迹（用于前端绘制执行过程）
type ExecutionTrail struct {
	InstanceID    int64                      `json:"instance_id"`
	Status        string                     `json:"status"`
	CurrentState  string                     `json:"current_state"`
	ActiveNodeIDs string                     `json:"active_node_ids"`
	Checkpoints   []ExecutionTrailCheckpoint `json:"checkpoints"`
}

// ExecutionTrailCheckpoint 轨迹中的单步快照
type ExecutionTrailCheckpoint struct {
	NodeID     string                 `json:"node_id"`
	NodeOutput map[string]interface{} `json:"node_output"`
	StateAfter map[string]interface{} `json:"state_after"`
	CreatedAt  string                 `json:"created_at"`
}
