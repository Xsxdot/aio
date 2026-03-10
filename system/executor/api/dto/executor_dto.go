package dto

// SubmitJobRequest 提交任务请求
type SubmitJobRequest struct {
	Env           string `json:"env" validate:"required"`             // 环境标识（必填，如 dev/prod/test）
	TargetService string `json:"target_service" validate:"required"`  // 目标服务名
	Method        string `json:"method" validate:"required"`          // 方法名
	ArgsJSON      string `json:"args_json"`                           // 参数 JSON
	RunAt         int64  `json:"run_at"`                              // 执行时间（Unix 时间戳秒），0表示立即执行
	MaxAttempts   int32  `json:"max_attempts"`                        // 最大重试次数，默认3次
	Priority      int32  `json:"priority"`                            // 优先级，数字越大优先级越高，默认0
	DedupKey      string `json:"dedup_key" validate:"required" comment:"幂等键"` // 幂等键（必填）
}

// ListJobsRequest 列出任务请求
type ListJobsRequest struct {
	Env           string `json:"env" query:"env"`                       // 环境标识（必填）
	TargetService string `json:"target_service" query:"target_service"` // 目标服务名（可选）
	Status        string `json:"status" query:"status"`                 // 状态过滤（可选）
	PageNum       int32  `json:"page_num" query:"page_num"`             // 页码，从1开始
	PageSize      int32  `json:"page_size" query:"page_size"`           // 每页数量
}

// RequeueJobRequest 重新入队请求
type RequeueJobRequest struct {
	RunAt int64 `json:"run_at"` // 执行时间（Unix 时间戳秒），0表示立即执行
}

// UpdateJobArgsRequest 更新任务参数请求
type UpdateJobArgsRequest struct {
	ArgsJSON string `json:"args_json"` // 参数 JSON
}

// CleanupJobsRequest 清理任务请求
type CleanupJobsRequest struct {
	Env           string `json:"env" validate:"required"` // 环境标识（必填，仅清理该 env 的任务）
	SucceededDays int    `json:"succeeded_days"`          // 清理N天前已成功的任务，0表示不清理
	CanceledDays  int    `json:"canceled_days"`           // 清理N天前已取消的任务，0表示不清理
	DeadDays      int    `json:"dead_days"`               // 清理N天前死信任务，0表示不清理
}

// GetStatsRequest 获取统计信息请求
type GetStatsRequest struct {
	Env string `json:"env" query:"env"` // 环境标识（必填）
}
