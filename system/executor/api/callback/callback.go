// Package callback 定义任务完成回调的对外契约。
//
// 职责：声明回调处理器接口，以及承载回调的 outbox 任务的固定字段与载荷结构。
//
// 边界：不含任何实现，仅为 executor 内部服务与 worker 消费者共享的契约，
// 放在 api/ 下以避免 service 与 worker 互相 import 形成环。
package callback

import "context"

const (
	// InternalTargetService outbox 回调任务的目标服务名，固定为 aio 自身。
	InternalTargetService = "aio"
	// MethodJobCompletedCallback outbox 回调任务的方法名。
	MethodJobCompletedCallback = "internal.job_completed_callback"
	// OutboxDedupKeyPrefix outbox 回调任务的幂等键前缀，完整形式为 {prefix}{jobID}。
	OutboxDedupKeyPrefix = "jobcb_"
	// OutboxMaxAttempts outbox 回调任务的最大尝试次数，超过后转死信、Admin 可见。
	OutboxMaxAttempts = 5
	// OutboxPriority outbox 回调任务的优先级，高于普通业务任务以尽快被领取。
	OutboxPriority = 10
)

// CallbackPayload 是 outbox 回调任务 ArgsJSON 的结构。
type CallbackPayload struct {
	JobID        uint64 `json:"job_id"`
	Source       string `json:"source"`
	CallbackData string `json:"callback_data"`
	ResultJSON   string `json:"result_json"`
}

// JobCompletionHandler 任务完成处理器（供 Workflow 等组件实现，按 Source 注册）。
//
// 注意：返回错误表示本次回调未成功应用，承载该回调的 outbox 任务会重试；
// 实现方必须保证按同一 jobID 重复调用是安全的（幂等）。
type JobCompletionHandler interface {
	OnJobCompleted(ctx context.Context, jobID uint64, callbackData, resultJSON string) error
}
