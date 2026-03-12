package callback

import "context"

// JobCompletionHandler 任务完成处理器（供 Workflow 等组件实现，按 Source 注册，任务成功时由 Executor 路由调用）
type JobCompletionHandler interface {
	OnJobCompleted(ctx context.Context, jobID uint64, callbackData, resultJSON string)
}
