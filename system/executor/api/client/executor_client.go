package client

import (
	"context"

	"github.com/xsxdot/aio/system/executor/api/dto"
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

// SubmitJob 提交任务（env 必填）
func (c *ExecutorClient) SubmitJob(ctx context.Context, req *dto.SubmitJobInput) (uint64, error) {
	return c.app.JobService.SubmitJob(ctx, req)
}

// GetJob 获取任务详情
func (c *ExecutorClient) GetJob(ctx context.Context, jobID uint64) (*model.ExecutorJobModel, error) {
	return c.app.JobService.GetJob(ctx, jobID)
}

// CancelJob 取消任务
func (c *ExecutorClient) CancelJob(ctx context.Context, jobID uint64) error {
	return c.app.JobService.CancelJob(ctx, jobID)
}

// GetJobByDedupKey 根据环境+幂等键获取任务（用于 Workflow 回滚时取消正在执行的任务）
func (c *ExecutorClient) GetJobByDedupKey(ctx context.Context, env, dedupKey string) (*model.ExecutorJobModel, error) {
	return c.app.JobService.GetJobByDedupKey(ctx, env, dedupKey)
}

// CancelJobByDedupKey 根据环境+幂等键取消任务（用于 Workflow 回滚时取消正在执行的任务）
func (c *ExecutorClient) CancelJobByDedupKey(ctx context.Context, env, dedupKey string) error {
	job, err := c.app.JobService.GetJobByDedupKey(ctx, env, dedupKey)
	if err != nil || job == nil {
		return err
	}
	if job.Status == model.JobStatusPending || job.Status == model.JobStatusRunning {
		return c.CancelJob(ctx, uint64(job.ID))
	}
	return nil
}

// RequeueJob 重新入队任务
func (c *ExecutorClient) RequeueJob(ctx context.Context, jobID uint64, runAt int64) error {
	return c.app.JobService.RequeueJob(ctx, jobID, runAt)
}

// GetStats 获取统计信息（env 必填）
func (c *ExecutorClient) GetStats(ctx context.Context, env string) (map[string]interface{}, error) {
	return c.app.JobService.GetStats(ctx, env)
}

// CleanupOldJobs 清理旧任务（env 必填，仅清理该 env 的任务）
func (c *ExecutorClient) CleanupOldJobs(ctx context.Context, env string, succeededDays, canceledDays, deadDays int) (int64, error) {
	return c.app.JobService.CleanupOldJobs(ctx, env, succeededDays, canceledDays, deadDays)
}
