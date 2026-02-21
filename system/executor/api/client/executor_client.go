package client

import (
	"context"

	"github.com/xsxdot/aio/system/executor/internal/app"
	"github.com/xsxdot/aio/system/executor/internal/model"
)

// ExecutorClient 任务执行客户端（供其他组件调用）
type ExecutorClient struct {
	app *app.App
}

// NewExecutorClient 创建客户端实例
func NewExecutorClient(a *app.App) *ExecutorClient {
	return &ExecutorClient{
		app: a,
	}
}

// SubmitJob 提交任务
func (c *ExecutorClient) SubmitJob(ctx context.Context, targetService, method, argsJSON string,
	runAt int64, maxAttempts, priority int32, dedupKey string) (uint64, error) {
	
	return c.app.JobService.SubmitJob(ctx, targetService, method, argsJSON, runAt, maxAttempts, priority, dedupKey)
}

// GetJob 获取任务详情
func (c *ExecutorClient) GetJob(ctx context.Context, jobID uint64) (*model.ExecutorJobModel, error) {
	return c.app.JobService.GetJob(ctx, jobID)
}

// CancelJob 取消任务
func (c *ExecutorClient) CancelJob(ctx context.Context, jobID uint64) error {
	return c.app.JobService.CancelJob(ctx, jobID)
}

// RequeueJob 重新入队任务
func (c *ExecutorClient) RequeueJob(ctx context.Context, jobID uint64, runAt int64) error {
	return c.app.JobService.RequeueJob(ctx, jobID, runAt)
}

// GetStats 获取统计信息
func (c *ExecutorClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return c.app.JobService.GetStats(ctx)
}

// CleanupOldJobs 清理旧任务
func (c *ExecutorClient) CleanupOldJobs(ctx context.Context, succeededDays, canceledDays, deadDays int) (int64, error) {
	return c.app.JobService.CleanupOldJobs(ctx, succeededDays, canceledDays, deadDays)
}
