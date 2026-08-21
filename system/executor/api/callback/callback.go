package callback

import "context"

// JobCompletionHandler 任务完成处理器（供 Workflow 等组件实现，按 Source 注册）。
//
// 注意：返回错误表示本次回调未成功应用，承载该回调的 outbox 任务会重试；
// 实现方必须保证按同一 jobID 重复调用是安全的（幂等）。
type JobCompletionHandler interface {
	OnJobCompleted(ctx context.Context, jobID uint64, callbackData, resultJSON string) error
}
