package dao

import (
	"context"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor/internal/model"

	"gorm.io/gorm"
)

// ExecutorJobDAO 任务数据访问层
type ExecutorJobDAO struct {
	db *gorm.DB
}

// NewExecutorJobDAO 创建任务DAO实例
func NewExecutorJobDAO() *ExecutorJobDAO {
	return &ExecutorJobDAO{
		db: base.DB,
	}
}

// Create 创建任务
func (d *ExecutorJobDAO) Create(ctx context.Context, job *model.ExecutorJobModel) error {
	return d.db.WithContext(ctx).Create(job).Error
}

// GetByID 根据ID获取任务
func (d *ExecutorJobDAO) GetByID(ctx context.Context, id uint64) (*model.ExecutorJobModel, error) {
	var job model.ExecutorJobModel
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByDedupKey 根据幂等键获取任务
func (d *ExecutorJobDAO) GetByDedupKey(ctx context.Context, dedupKey string) (*model.ExecutorJobModel, error) {
	var job model.ExecutorJobModel
	err := d.db.WithContext(ctx).Where("dedup_key = ?", dedupKey).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// AcquireJob 领取任务（使用原子更新实现竞争领取，兼容 MySQL 5.7+）
func (d *ExecutorJobDAO) AcquireJob(ctx context.Context, targetService, method, consumerID string, leaseDuration int32) (*model.ExecutorJobModel, *model.ExecutorJobAttemptModel, error) {
	var job model.ExecutorJobModel
	var attempt model.ExecutorJobAttemptModel
	now := time.Now()
	leaseUntil := now.Add(time.Duration(leaseDuration) * time.Second)

	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 检查该 consumer 是否已有未到期租约的任务（保证同 consumer 不并行）
		var existingJob model.ExecutorJobModel
		err := tx.Where("lease_owner = ? AND lease_until > ?", consumerID, now).
			First(&existingJob).Error
		if err == nil {
			// 已有任务在执行中
			return gorm.ErrRecordNotFound
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		// 2. 查找可领取的任务（按优先级降序，next_run_at升序）
		// 条件：target_service匹配 AND method匹配（如果指定） AND (状态为pending OR (状态为running但租约已过期)) AND next_run_at <= now
		// 使用子查询获取候选任务ID列表（不使用 FOR UPDATE SKIP LOCKED，兼容 MySQL 5.7）
		var candidateIDs []uint64
		err = tx.Raw(`
			SELECT id FROM executor_jobs
			WHERE target_service = ?
			  AND (? = '' OR method = ?)
			  AND (status = ? OR (status = ? AND (lease_until IS NULL OR lease_until <= ?)))
			  AND (next_run_at IS NULL OR next_run_at <= ?)
			ORDER BY priority DESC, next_run_at ASC, id ASC
			LIMIT 10
		`, targetService, method, method, model.JobStatusPending, model.JobStatusRunning, now, now).
			Scan(&candidateIDs).Error

		if err != nil {
			return err
		}

		if len(candidateIDs) == 0 {
			return gorm.ErrRecordNotFound
		}

		// 3. 使用原子更新尝试领取任务（乐观锁）
		// 遍历候选任务，尝试原子更新第一个可用的
		for _, jobID := range candidateIDs {
			// 使用 UPDATE ... WHERE 原子性地领取任务
			result := tx.Model(&model.ExecutorJobModel{}).
				Where("id = ?", jobID).
				Where("(status = ? OR (status = ? AND (lease_until IS NULL OR lease_until <= ?)))",
					model.JobStatusPending, model.JobStatusRunning, now).
				Updates(map[string]interface{}{
					"status":      model.JobStatusRunning,
					"lease_owner": consumerID,
					"lease_until": leaseUntil,
					"attempts":    gorm.Expr("attempts + 1"),
				})

			if result.Error != nil {
				return result.Error
			}

			// 如果更新成功（RowsAffected > 0），说明成功领取了任务
			if result.RowsAffected > 0 {
				// 查询更新后的任务信息
				if err := tx.Where("id = ?", jobID).First(&job).Error; err != nil {
					return err
				}

				// 4. 创建尝试记录
				attempt = model.ExecutorJobAttemptModel{
					JobID:     uint64(job.ID),
					AttemptNo: job.Attempts,
					WorkerID:  consumerID,
					Status:    model.JobStatusRunning,
					StartedAt: &now,
				}

				return tx.Create(&attempt).Error
			}

			// 如果 RowsAffected == 0，说明任务已被其他 worker 领取，继续尝试下一个
		}

		// 所有候选任务都已被其他 worker 领取
		return gorm.ErrRecordNotFound
	})

	if err != nil {
		return nil, nil, err
	}

	return &job, &attempt, nil
}

// RenewLease 续租
func (d *ExecutorJobDAO) RenewLease(ctx context.Context, jobID uint64, attemptNo int32, consumerID string, extendDuration int32) (*model.ExecutorJobModel, error) {
	var job model.ExecutorJobModel
	now := time.Now()
	newLeaseUntil := now.Add(time.Duration(extendDuration) * time.Second)

	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查询并校验
		err := tx.Where("id = ? AND attempts = ? AND lease_owner = ?", jobID, attemptNo, consumerID).
			First(&job).Error
		if err != nil {
			return err
		}

		// 更新租约
		job.LeaseUntil = &newLeaseUntil
		return tx.Save(&job).Error
	})

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// AckJob 确认任务执行结果
func (d *ExecutorJobDAO) AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
	status model.JobStatus, errorMsg, resultJSON string, retryAfter int32) error {

	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job model.ExecutorJobModel

		// 查询并校验
		err := tx.Where("id = ? AND attempts = ? AND lease_owner = ?", jobID, attemptNo, consumerID).
			First(&job).Error
		if err != nil {
			return err
		}

		// 更新尝试记录
		now := time.Now()
		err = tx.Model(&model.ExecutorJobAttemptModel{}).
			Where("job_id = ? AND attempt_no = ?", jobID, attemptNo).
			Updates(map[string]interface{}{
				"status":      status,
				"error":       errorMsg,
				"finished_at": now,
			}).Error
		if err != nil {
			return err
		}

		// 更新任务状态
		job.Status = status
		job.LeaseOwner = ""
		job.LeaseUntil = nil

		if status == model.JobStatusSucceeded {
			// 成功
			job.ResultJSON = resultJSON
			job.LastError = ""
		} else if status == model.JobStatusFailed {
			// 失败
			job.LastError = errorMsg

			// 判断是否超过最大重试次数
			if job.Attempts >= job.MaxAttempts {
				job.Status = model.JobStatusDead
			} else {
				// 重新入队，计算下次执行时间
				job.Status = model.JobStatusPending
				nextRunAt := calculateNextRunAt(job.Attempts, retryAfter)
				job.NextRunAt = &nextRunAt
			}
		}

		return tx.Save(&job).Error
	})
}

// calculateNextRunAt 计算下次执行时间（指数退避 + 抖动）
func calculateNextRunAt(attempts int32, retryAfter int32) time.Time {
	if retryAfter > 0 {
		return time.Now().Add(time.Duration(retryAfter) * time.Second)
	}

	// 指数退避：2^attempts 秒，最大 300 秒（5分钟）
	backoff := 1 << attempts
	if backoff > 300 {
		backoff = 300
	}

	// 添加 0-10% 的抖动
	jitter := backoff / 10
	if jitter == 0 {
		jitter = 1
	}

	return time.Now().Add(time.Duration(backoff+jitter) * time.Second)
}

// UpdateStatus 更新任务状态
func (d *ExecutorJobDAO) UpdateStatus(ctx context.Context, jobID uint64, status model.JobStatus) error {
	return d.db.WithContext(ctx).Model(&model.ExecutorJobModel{}).
		Where("id = ?", jobID).
		Update("status", status).Error
}

// UpdateArgsJSON 更新任务参数JSON
func (d *ExecutorJobDAO) UpdateArgsJSON(ctx context.Context, jobID uint64, argsJSON string) error {
	return d.db.WithContext(ctx).Model(&model.ExecutorJobModel{}).
		Where("id = ?", jobID).
		Update("args_json", argsJSON).Error
}

// Requeue 重新入队任务
func (d *ExecutorJobDAO) Requeue(ctx context.Context, jobID uint64, runAt time.Time) error {
	updates := map[string]interface{}{
		"status":      model.JobStatusPending,
		"next_run_at": runAt,
		"lease_owner": "",
		"lease_until": nil,
	}

	return d.db.WithContext(ctx).Model(&model.ExecutorJobModel{}).
		Where("id = ?", jobID).
		Updates(updates).Error
}

// List 列出任务
func (d *ExecutorJobDAO) List(ctx context.Context, targetService string, status model.JobStatus, pageNum, pageSize int32) ([]*model.ExecutorJobModel, int64, error) {
	var jobs []*model.ExecutorJobModel
	var total int64

	query := d.db.WithContext(ctx).Model(&model.ExecutorJobModel{})

	if targetService != "" {
		query = query.Where("target_service = ?", targetService)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (pageNum - 1) * pageSize
	if err := query.Order("id DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&jobs).Error; err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

// CountByStatus 统计各状态任务数量
func (d *ExecutorJobDAO) CountByStatus(ctx context.Context) (map[model.JobStatus]int64, error) {
	type StatusCount struct {
		Status model.JobStatus
		Count  int64
	}

	var counts []StatusCount
	err := d.db.WithContext(ctx).
		Model(&model.ExecutorJobModel{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&counts).Error

	if err != nil {
		return nil, err
	}

	result := make(map[model.JobStatus]int64)
	for _, sc := range counts {
		result[sc.Status] = sc.Count
	}

	return result, nil
}

// CountDueJobs 统计到期任务数量
func (d *ExecutorJobDAO) CountDueJobs(ctx context.Context) (int64, error) {
	var count int64
	now := time.Now()

	err := d.db.WithContext(ctx).
		Model(&model.ExecutorJobModel{}).
		Where("status = ? AND (next_run_at IS NULL OR next_run_at <= ?)", model.JobStatusPending, now).
		Count(&count).Error

	return count, err
}

// GetRetryDistribution 获取重试次数分布
func (d *ExecutorJobDAO) GetRetryDistribution(ctx context.Context) (map[int32]int64, error) {
	type RetryCount struct {
		Attempts int32
		Count    int64
	}

	var counts []RetryCount
	err := d.db.WithContext(ctx).
		Model(&model.ExecutorJobModel{}).
		Select("attempts, COUNT(*) as count").
		Where("status != ?", model.JobStatusPending).
		Group("attempts").
		Find(&counts).Error

	if err != nil {
		return nil, err
	}

	result := make(map[int32]int64)
	for _, rc := range counts {
		result[rc.Attempts] = rc.Count
	}

	return result, nil
}

// DeleteOldSucceededJobs 删除旧的已成功任务（归档清理）
func (d *ExecutorJobDAO) DeleteOldSucceededJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := d.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.JobStatusSucceeded, olderThan).
		Delete(&model.ExecutorJobModel{})

	return result.RowsAffected, result.Error
}

// DeleteOldCanceledJobs 删除旧的已取消任务（归档清理）
func (d *ExecutorJobDAO) DeleteOldCanceledJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := d.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.JobStatusCanceled, olderThan).
		Delete(&model.ExecutorJobModel{})

	return result.RowsAffected, result.Error
}

// DeleteOldDeadJobs 删除旧的死信任务（归档清理）
func (d *ExecutorJobDAO) DeleteOldDeadJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := d.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.JobStatusDead, olderThan).
		Delete(&model.ExecutorJobModel{})

	return result.RowsAffected, result.Error
}
