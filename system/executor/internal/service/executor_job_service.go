package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"

	"gorm.io/gorm"
)

// requireEnv 校验 env 参数，为空或仅空白则返回错误
func requireEnv(env string) (string, error) {
	e := strings.TrimSpace(env)
	if e == "" {
		return "", errors.New("env 不能为空")
	}
	return e, nil
}

// ExecutorJobService 任务服务层
type ExecutorJobService struct {
	dao *dao.ExecutorJobDAO
}

// NewExecutorJobService 创建任务服务实例
func NewExecutorJobService() *ExecutorJobService {
	return &ExecutorJobService{
		dao: dao.NewExecutorJobDAO(),
	}
}

// SubmitJob 提交任务
func (s *ExecutorJobService) SubmitJob(ctx context.Context, env, targetService, method, argsJSON string,
	runAt int64, maxAttempts, priority int32, dedupKey string) (uint64, error) {

	e, err := requireEnv(env)
	if err != nil {
		return 0, err
	}
	env = e

	if strings.TrimSpace(dedupKey) == "" {
		return 0, errors.New("dedupKey 不能为空")
	}

	// 默认最大重试次数
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	// 检查幂等键（按 env 隔离）
	if dedupKey != "" {
		existingJob, err := s.dao.GetByDedupKey(ctx, env, dedupKey)
		if err == nil {
			// 已存在相同幂等键的任务
			base.Logger.Info("任务已存在，返回已有任务ID")
			return uint64(existingJob.ID), nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}
	
	// 计算下次执行时间
	var nextRunAt *time.Time
	if runAt > 0 {
		t := time.Unix(runAt, 0)
		nextRunAt = &t
	} else {
		now := time.Now()
		nextRunAt = &now
	}
	
	// 创建任务，写入当前运行环境
	job := &model.ExecutorJobModel{
		Env:           env,
		TargetService: targetService,
		Method:        method,
		ArgsJSON:      argsJSON,
		Status:        model.JobStatusPending,
		Priority:      priority,
		NextRunAt:     nextRunAt,
		MaxAttempts:   maxAttempts,
		Attempts:      0,
		DedupKey:      dedupKey,
	}
	
	if err := s.dao.Create(ctx, job); err != nil {
		return 0, err
	}
	
	base.Logger.Info("任务提交成功")
	
	return uint64(job.ID), nil
}

// AcquireJob 领取任务（仅领取指定 env 的任务）
func (s *ExecutorJobService) AcquireJob(ctx context.Context, env, targetService, method, consumerID string, leaseDuration int32) (*model.ExecutorJobModel, error) {
	e, err := requireEnv(env)
	if err != nil {
		return nil, err
	}

	// 默认租约时长30秒
	if leaseDuration <= 0 {
		leaseDuration = 30
	}

	job, _, err := s.dao.AcquireJob(ctx, e, targetService, method, consumerID, leaseDuration)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 没有可领取的任务，返回空
			return nil, nil
		}
		return nil, err
	}
	
	base.Logger.Info("任务领取成功")
	
	return job, nil
}

// RenewLease 续租
func (s *ExecutorJobService) RenewLease(ctx context.Context, jobID uint64, attemptNo int32, consumerID string, extendDuration int32) (*model.ExecutorJobModel, error) {
	if extendDuration <= 0 {
		extendDuration = 30
	}
	
	job, err := s.dao.RenewLease(ctx, jobID, attemptNo, consumerID, extendDuration)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("任务不存在或租约信息不匹配")
		}
		return nil, err
	}
	
	base.Logger.Debug("任务租约续期成功")
	
	return job, nil
}

// AckJob 确认任务执行结果
func (s *ExecutorJobService) AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
	status model.JobStatus, errorMsg, resultJSON string, retryAfter int32) error {
	
	err := s.dao.AckJob(ctx, jobID, attemptNo, consumerID, status, errorMsg, resultJSON, retryAfter)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("任务不存在或租约信息不匹配")
		}
		return err
	}
	
	base.Logger.Info("任务确认成功")
	
	return nil
}

// GetJob 获取任务详情
func (s *ExecutorJobService) GetJob(ctx context.Context, jobID uint64) (*model.ExecutorJobModel, error) {
	job, err := s.dao.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("任务不存在")
		}
		return nil, err
	}
	return job, nil
}

// ListJobs 列出任务（按 env 过滤）
func (s *ExecutorJobService) ListJobs(ctx context.Context, env, targetService string, status model.JobStatus, pageNum, pageSize int32) ([]*model.ExecutorJobModel, int64, error) {
	e, err := requireEnv(env)
	if err != nil {
		return nil, 0, err
	}

	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	return s.dao.List(ctx, e, targetService, status, pageNum, pageSize)
}

// CancelJob 取消任务
func (s *ExecutorJobService) CancelJob(ctx context.Context, jobID uint64) error {
	job, err := s.dao.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("任务不存在")
		}
		return err
	}
	
	// 只有 pending 或 running（租约过期）的任务才能取消
	if job.Status != model.JobStatusPending && job.Status != model.JobStatusRunning {
		return errors.New("只有待执行或执行中的任务才能取消")
	}
	
	err = s.dao.UpdateStatus(ctx, jobID, model.JobStatusCanceled)
	if err != nil {
		return err
	}
	
	base.Logger.Info("任务取消成功")
	
	return nil
}

// RequeueJob 重新入队任务
func (s *ExecutorJobService) RequeueJob(ctx context.Context, jobID uint64, runAt int64) error {
	job, err := s.dao.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("任务不存在")
		}
		return err
	}
	
	// 只有 failed、canceled、dead 状态的任务才能重新入队
	if job.Status != model.JobStatusFailed && 
	   job.Status != model.JobStatusCanceled && 
	   job.Status != model.JobStatusDead {
		return errors.New("只有失败、已取消或死信状态的任务才能重新入队")
	}
	
	// 计算执行时间
	var nextRunAt time.Time
	if runAt > 0 {
		nextRunAt = time.Unix(runAt, 0)
	} else {
		nextRunAt = time.Now()
	}
	
	err = s.dao.Requeue(ctx, jobID, nextRunAt)
	if err != nil {
		return err
	}
	
	base.Logger.Info("任务重新入队成功")
	
	return nil
}

// GetStats 获取统计信息（按 env 过滤）
func (s *ExecutorJobService) GetStats(ctx context.Context, env string) (map[string]interface{}, error) {
	e, err := requireEnv(env)
	if err != nil {
		return nil, err
	}
	env = e

	// 统计各状态任务数量
	statusCounts, err := s.dao.CountByStatus(ctx, env)
	if err != nil {
		return nil, err
	}
	
	// 统计到期任务数量
	dueCount, err := s.dao.CountDueJobs(ctx, env)
	if err != nil {
		return nil, err
	}
	
	// 获取重试次数分布
	retryDistribution, err := s.dao.GetRetryDistribution(ctx, env)
	if err != nil {
		return nil, err
	}
	
	// 计算队列长度（pending + due）
	queueLength := statusCounts[model.JobStatusPending]
	
	stats := map[string]interface{}{
		"queue_length":       queueLength,
		"pending_count":      statusCounts[model.JobStatusPending],
		"running_count":      statusCounts[model.JobStatusRunning],
		"succeeded_count":    statusCounts[model.JobStatusSucceeded],
		"failed_count":       statusCounts[model.JobStatusFailed],
		"canceled_count":     statusCounts[model.JobStatusCanceled],
		"dead_count":         statusCounts[model.JobStatusDead],
		"due_count":          dueCount,
		"retry_distribution": retryDistribution,
	}
	
	return stats, nil
}

// CleanupOldJobs 清理旧任务（仅清理指定 env，避免跨 env 误删）
func (s *ExecutorJobService) CleanupOldJobs(ctx context.Context, env string, succeededDays, canceledDays, deadDays int) (int64, error) {
	e, err := requireEnv(env)
	if err != nil {
		return 0, err
	}
	env = e

	now := time.Now()
	var totalDeleted int64

	// 清理已成功的任务
	if succeededDays > 0 {
		succeededOlderThan := now.AddDate(0, 0, -succeededDays)
		deleted, err := s.dao.DeleteOldSucceededJobs(ctx, env, succeededOlderThan)
		if err != nil {
			return totalDeleted, err
		}
		totalDeleted += deleted
		base.Logger.Info("清理已成功任务完成")
	}

	// 清理已取消的任务
	if canceledDays > 0 {
		canceledOlderThan := now.AddDate(0, 0, -canceledDays)
		deleted, err := s.dao.DeleteOldCanceledJobs(ctx, env, canceledOlderThan)
		if err != nil {
			return totalDeleted, err
		}
		totalDeleted += deleted
		base.Logger.Info("清理已取消任务完成")
	}

	// 清理死信任务
	if deadDays > 0 {
		deadOlderThan := now.AddDate(0, 0, -deadDays)
		deleted, err := s.dao.DeleteOldDeadJobs(ctx, env, deadOlderThan)
		if err != nil {
			return totalDeleted, err
		}
		totalDeleted += deleted
		base.Logger.Info("清理死信任务完成")
	}

	return totalDeleted, nil
}

// UpdateJobArgsJSON 更新任务参数JSON
func (s *ExecutorJobService) UpdateJobArgsJSON(ctx context.Context, jobID uint64, argsJSON string) error {
	// 获取任务
	job, err := s.dao.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("任务不存在")
		}
		return err
	}
	
	// 只有非 running 状态的任务才能修改参数
	if job.Status == model.JobStatusRunning {
		return errors.New("running 任务不允许修改参数")
	}
	
	// 更新参数
	err = s.dao.UpdateArgsJSON(ctx, jobID, argsJSON)
	if err != nil {
		return err
	}
	
	base.Logger.Info("任务参数更新成功")
	
	return nil
}
