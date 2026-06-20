// Package dao 提供 executor job 的持久化访问。
//
// 职责：
//   - 在一个事务内批量领取 executor job
//   - 按数据库方言选择 PostgreSQL 优化路径或 MySQL 兼容路径
//   - 维护租约、attempt 和 sequence_key 约束
//
// 边界：
//   - 不执行业务 handler
//   - 不处理 gRPC/SDK 参数转换
package dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/pkg/db/dialect"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/gorm"
)

// AcquireJobsMode 定义批量领取策略。
type AcquireJobsMode string

const (
	// AcquireJobsModeOnePerMethod 表示每个 method 最多领取一条 job。
	AcquireJobsModeOnePerMethod AcquireJobsMode = "ONE_PER_METHOD"
	// AcquireJobsModeFillSlots 表示按空闲 slot 数领取 job。
	AcquireJobsModeFillSlots AcquireJobsMode = "FILL_SLOTS"
)

// MethodSlot 表示一次批量领取中的 method 与 consumer slot 绑定。
type MethodSlot struct {
	Method     string
	ConsumerID string
}

// AcquireJobsInput 是 DAO 批量领取任务入参。
type AcquireJobsInput struct {
	Env           string
	TargetService string
	Methods       []string
	MethodSlots   []MethodSlot
	LeaseDuration int32
	Mode          AcquireJobsMode
}

// AcquiredJobLease 表示已租赁 job 与实际 consumer slot 的绑定。
type AcquiredJobLease struct {
	Job        *model.ExecutorJobModel
	AttemptNo  int32
	ConsumerID string
}

type acquiredJobRow struct {
	model.ExecutorJobModel
	ConsumerID string `gorm:"column:consumer_id"`
	Ord        int    `gorm:"column:ord"`
}

// AcquireJobs 批量领取任务，在一个事务中完成候选筛选、租约更新和 attempt 创建。
//
// 参数：
//   - ctx: 上下文，携带事务时通过 mvc.ExtractDB 透传
//   - in: env、target service、method/consumer slot、租约和模式
//
// 返回：
//   - 已领取任务列表；无任务时返回空切片
//   - 数据库或参数错误
//
// 注意：
//   - PostgreSQL 走单查询优化路径
//   - MySQL 5.7 走事务内逐 slot CAS fallback，保持一次 RPC 的核心收益
func (d *ExecutorJobDAO) AcquireJobs(ctx context.Context, in AcquireJobsInput) ([]AcquiredJobLease, error) {
	log := logger.GetLogger().WithEntryName("ExecutorJobDAO")
	leaseDuration := in.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = 30
	}
	now := time.Now()
	leaseUntil := now.Add(time.Duration(leaseDuration) * time.Second)
	db := mvc.ExtractDB(ctx, d.db)
	dialectName := dialect.DialectName(db)

	log.WithField("env", in.Env).
		WithField("target_service", in.TargetService).
		WithField("mode", in.Mode).
		WithField("slot_count", len(in.MethodSlots)).
		WithField("dialect", dialectName).
		Debug("开始批量领取 executor job")

	var leases []AcquiredJobLease
	err := db.Transaction(func(tx *gorm.DB) error {
		var rows []acquiredJobRow
		var err error
		if dialect.IsPostgres(tx) {
			rows, err = d.acquireJobsPostgres(ctx, tx, in, now, leaseUntil)
		} else {
			rows, err = d.acquireJobsMySQL(ctx, tx, in, now, leaseUntil)
		}
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		attempts := make([]model.ExecutorJobAttemptModel, 0, len(rows))
		leases = make([]AcquiredJobLease, 0, len(rows))
		for i := range rows {
			job := rows[i].ExecutorJobModel
			consumerID := rows[i].ConsumerID
			attempts = append(attempts, model.ExecutorJobAttemptModel{
				JobID:     job.ID,
				AttemptNo: job.Attempts,
				WorkerID:  consumerID,
				Status:    model.JobStatusRunning,
				StartedAt: &now,
			})
			leases = append(leases, AcquiredJobLease{
				Job:        &job,
				AttemptNo:  job.Attempts,
				ConsumerID: consumerID,
			})
		}
		return tx.Create(&attempts).Error
	})
	if err != nil {
		log.WithErr(err).
			WithField("mode", in.Mode).
			WithField("dialect", dialectName).
			Error("批量领取 executor job 失败")
		return nil, err
	}
	if len(leases) > 0 {
		log.WithField("mode", in.Mode).
			WithField("dialect", dialectName).
			WithField("leased_count", len(leases)).
			Info("批量领取 executor job 成功")
	}
	return leases, nil
}

func (d *ExecutorJobDAO) acquireJobsPostgres(ctx context.Context, tx *gorm.DB, in AcquireJobsInput, now, leaseUntil time.Time) ([]acquiredJobRow, error) {
	switch in.Mode {
	case AcquireJobsModeOnePerMethod:
		return d.acquireOnePerMethodRowsPostgres(ctx, tx, in, now, leaseUntil)
	case AcquireJobsModeFillSlots:
		return d.acquireFillSlotRowsPostgres(ctx, tx, in, now, leaseUntil)
	default:
		return nil, fmt.Errorf("unsupported acquire jobs mode: %s", in.Mode)
	}
}

func (d *ExecutorJobDAO) acquireOnePerMethodRowsPostgres(ctx context.Context, tx *gorm.DB, in AcquireJobsInput, now, leaseUntil time.Time) ([]acquiredJobRow, error) {
	if len(in.MethodSlots) == 0 {
		return nil, nil
	}
	valuesSQL, valuesArgs := buildMethodSlotValues(in.MethodSlots)
	sql := fmt.Sprintf(`
WITH method_slots(method, consumer_id, ord) AS (VALUES %s),
free_slots AS (
  SELECT method, consumer_id, ord
  FROM method_slots s
  WHERE NOT EXISTS (
    SELECT 1 FROM aio_executor_jobs active
    WHERE active.lease_owner = s.consumer_id
      AND active.lease_until > ?
  )
),
ranked_jobs AS (
  SELECT
    j.id,
    j.method,
    s.consumer_id,
    s.ord,
    row_number() OVER (
      PARTITION BY j.method
      ORDER BY j.priority DESC, j.next_run_at ASC NULLS FIRST, j.id ASC
    ) AS method_rank
  FROM aio_executor_jobs j
  JOIN free_slots s ON s.method = j.method
  WHERE j.env = ?
    AND j.target_service = ?
    AND (j.status = ? OR (j.status = ? AND (j.lease_until IS NULL OR j.lease_until <= ?)))
    AND (j.next_run_at IS NULL OR j.next_run_at <= ?)
    AND ((j.sequence_key IS NULL OR j.sequence_key = '')
      OR NOT EXISTS (
        SELECT 1 FROM aio_executor_jobs j2
        WHERE j2.sequence_key = j.sequence_key
          AND j2.sequence_key != ''
          AND j2.status = ?
          AND j2.lease_until > ?
          AND j2.id != j.id
      ))
),
selected AS (
  SELECT id, consumer_id, ord
  FROM ranked_jobs
  WHERE method_rank = 1
),
locked AS (
  SELECT j.id, s.consumer_id, s.ord
  FROM aio_executor_jobs j
  JOIN selected s ON s.id = j.id
  ORDER BY s.ord
  FOR UPDATE OF j SKIP LOCKED
),
leased AS (
  UPDATE aio_executor_jobs j
  SET status = ?,
      lease_owner = l.consumer_id,
      lease_until = ?,
      attempts = attempts + 1
  FROM locked l
  WHERE j.id = l.id
    AND (j.status = ? OR (j.status = ? AND (j.lease_until IS NULL OR j.lease_until <= ?)))
    AND ((j.sequence_key IS NULL OR j.sequence_key = '')
      OR NOT EXISTS (
        SELECT 1 FROM aio_executor_jobs j2
        WHERE j2.sequence_key = j.sequence_key
          AND j2.sequence_key != ''
          AND j2.status = ?
          AND j2.lease_until > ?
          AND j2.id != j.id
      ))
  RETURNING j.*, l.consumer_id, l.ord
)
SELECT * FROM leased ORDER BY ord
`, valuesSQL)
	args := append([]interface{}{}, valuesArgs...)
	args = append(args,
		now,
		in.Env, in.TargetService,
		model.JobStatusPending, model.JobStatusRunning, now,
		now,
		model.JobStatusRunning, now,
		model.JobStatusRunning, leaseUntil,
		model.JobStatusPending, model.JobStatusRunning, now,
		model.JobStatusRunning, now,
	)
	var rows []acquiredJobRow
	err := tx.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error
	return rows, err
}

func (d *ExecutorJobDAO) acquireFillSlotRowsPostgres(ctx context.Context, tx *gorm.DB, in AcquireJobsInput, now, leaseUntil time.Time) ([]acquiredJobRow, error) {
	if len(in.Methods) == 0 || len(in.MethodSlots) == 0 {
		return nil, nil
	}
	consumerSQL, consumerArgs := buildConsumerValues(in.MethodSlots)
	methodSQL, methodArgs := buildMethodValues(in.Methods)
	sql := fmt.Sprintf(`
WITH free_consumers(consumer_id, ord) AS (VALUES %s),
method_filter(method) AS (VALUES %s),
ranked_jobs AS (
  SELECT
    j.id,
    j.method,
    row_number() OVER (
      ORDER BY j.priority DESC, j.next_run_at ASC NULLS FIRST, j.id ASC
    ) AS assign_rank
  FROM aio_executor_jobs j
  JOIN method_filter mf ON mf.method = j.method
  WHERE j.env = ?
    AND j.target_service = ?
    AND (j.status = ? OR (j.status = ? AND (j.lease_until IS NULL OR j.lease_until <= ?)))
    AND (j.next_run_at IS NULL OR j.next_run_at <= ?)
    AND ((j.sequence_key IS NULL OR j.sequence_key = '')
      OR NOT EXISTS (
        SELECT 1 FROM aio_executor_jobs j2
        WHERE j2.sequence_key = j.sequence_key
          AND j2.sequence_key != ''
          AND j2.status = ?
          AND j2.lease_until > ?
          AND j2.id != j.id
      ))
  ORDER BY j.priority DESC, j.next_run_at ASC NULLS FIRST, j.id ASC
  LIMIT (SELECT count(*) FROM free_consumers)
),
selected AS (
  SELECT r.id, c.consumer_id, c.ord
  FROM ranked_jobs r
  JOIN free_consumers c ON c.ord = r.assign_rank
),
locked AS (
  SELECT j.id, s.consumer_id, s.ord
  FROM aio_executor_jobs j
  JOIN selected s ON s.id = j.id
  ORDER BY s.ord
  FOR UPDATE OF j SKIP LOCKED
),
leased AS (
  UPDATE aio_executor_jobs j
  SET status = ?,
      lease_owner = l.consumer_id,
      lease_until = ?,
      attempts = attempts + 1
  FROM locked l
  WHERE j.id = l.id
    AND (j.status = ? OR (j.status = ? AND (j.lease_until IS NULL OR j.lease_until <= ?)))
    AND ((j.sequence_key IS NULL OR j.sequence_key = '')
      OR NOT EXISTS (
        SELECT 1 FROM aio_executor_jobs j2
        WHERE j2.sequence_key = j.sequence_key
          AND j2.sequence_key != ''
          AND j2.status = ?
          AND j2.lease_until > ?
          AND j2.id != j.id
      ))
  RETURNING j.*, l.consumer_id, l.ord
)
SELECT * FROM leased ORDER BY ord
`, consumerSQL, methodSQL)
	args := append([]interface{}{}, consumerArgs...)
	args = append(args, methodArgs...)
	args = append(args,
		in.Env, in.TargetService,
		model.JobStatusPending, model.JobStatusRunning, now,
		now,
		model.JobStatusRunning, now,
		model.JobStatusRunning, leaseUntil,
		model.JobStatusPending, model.JobStatusRunning, now,
		model.JobStatusRunning, now,
	)
	var rows []acquiredJobRow
	err := tx.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error
	return rows, err
}

func (d *ExecutorJobDAO) acquireJobsMySQL(ctx context.Context, tx *gorm.DB, in AcquireJobsInput, now, leaseUntil time.Time) ([]acquiredJobRow, error) {
	switch in.Mode {
	case AcquireJobsModeOnePerMethod:
		return d.acquireOnePerMethodRowsCAS(ctx, tx, in, now, leaseUntil)
	case AcquireJobsModeFillSlots:
		return d.acquireFillSlotRowsCAS(ctx, tx, in, now, leaseUntil)
	default:
		return nil, fmt.Errorf("unsupported acquire jobs mode: %s", in.Mode)
	}
}

func (d *ExecutorJobDAO) acquireOnePerMethodRowsCAS(ctx context.Context, tx *gorm.DB, in AcquireJobsInput, now, leaseUntil time.Time) ([]acquiredJobRow, error) {
	rows := make([]acquiredJobRow, 0, len(in.MethodSlots))
	for _, slot := range in.MethodSlots {
		if slot.Method == "" || slot.ConsumerID == "" {
			continue
		}
		// ONE_PER_METHOD 保持旧单任务语义：同一派生 slot 未完成前不再领取新 job。
		busy, err := hasActiveLease(ctx, tx, slot.ConsumerID, now)
		if err != nil {
			return nil, err
		}
		if busy {
			continue
		}
		row, ok, err := acquireFirstCandidateByCAS(ctx, tx, candidateQuery{
			Env:           in.Env,
			TargetService: in.TargetService,
			Methods:       []string{slot.Method},
			ConsumerID:    slot.ConsumerID,
			Now:           now,
			LeaseUntil:    leaseUntil,
		})
		if err != nil {
			return nil, err
		}
		if ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (d *ExecutorJobDAO) acquireFillSlotRowsCAS(ctx context.Context, tx *gorm.DB, in AcquireJobsInput, now, leaseUntil time.Time) ([]acquiredJobRow, error) {
	rows := make([]acquiredJobRow, 0, len(in.MethodSlots))
	for _, slot := range in.MethodSlots {
		if slot.ConsumerID == "" {
			continue
		}
		// FILL_SLOTS 不做 DB busy filter：freeSlots 池已经把占用中的 slot 从请求中移除了。
		row, ok, err := acquireFirstCandidateByCAS(ctx, tx, candidateQuery{
			Env:           in.Env,
			TargetService: in.TargetService,
			Methods:       in.Methods,
			ConsumerID:    slot.ConsumerID,
			Now:           now,
			LeaseUntil:    leaseUntil,
		})
		if err != nil {
			return nil, err
		}
		if ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

type candidateQuery struct {
	Env           string
	TargetService string
	Methods       []string
	ConsumerID    string
	Now           time.Time
	LeaseUntil    time.Time
}

func acquireFirstCandidateByCAS(ctx context.Context, tx *gorm.DB, q candidateQuery) (acquiredJobRow, bool, error) {
	if len(q.Methods) == 0 {
		return acquiredJobRow{}, false, nil
	}
	var candidateIDs []int64
	if err := tx.WithContext(ctx).Raw(`
		SELECT j.id FROM aio_executor_jobs j
		WHERE j.env = ?
		  AND j.target_service = ?
		  AND j.method IN ?
		  AND (j.status = ? OR (j.status = ? AND (j.lease_until IS NULL OR j.lease_until <= ?)))
		  AND (j.next_run_at IS NULL OR j.next_run_at <= ?)
		  AND ((j.sequence_key IS NULL OR j.sequence_key = '')
		    OR NOT EXISTS (
		      SELECT 1 FROM aio_executor_jobs j2
		      WHERE j2.sequence_key = j.sequence_key
		        AND j2.sequence_key != ''
		        AND j2.status = ?
		        AND j2.lease_until > ?
		        AND j2.id != j.id
		    ))
		ORDER BY j.priority DESC, j.next_run_at ASC, j.id ASC
		LIMIT 50
	`, q.Env, q.TargetService, q.Methods, model.JobStatusPending, model.JobStatusRunning, q.Now, q.Now, model.JobStatusRunning, q.Now).
		Scan(&candidateIDs).Error; err != nil {
		return acquiredJobRow{}, false, err
	}
	for _, jobID := range candidateIDs {
		result := tx.WithContext(ctx).Model(&model.ExecutorJobModel{}).
			Where("id = ?", jobID).
			Where("(status = ? OR (status = ? AND (lease_until IS NULL OR lease_until <= ?)))",
				model.JobStatusPending, model.JobStatusRunning, q.Now).
			Where("(sequence_key IS NULL OR sequence_key = '' OR NOT EXISTS ("+
				"SELECT 1 FROM aio_executor_jobs j2 "+
				"WHERE j2.sequence_key = aio_executor_jobs.sequence_key "+
				"AND j2.sequence_key != '' "+
				"AND j2.status = ? AND j2.lease_until > ? AND j2.id != aio_executor_jobs.id))",
				model.JobStatusRunning, q.Now).
			Updates(map[string]interface{}{
				"status":      model.JobStatusRunning,
				"lease_owner": q.ConsumerID,
				"lease_until": q.LeaseUntil,
				"attempts":    gorm.Expr("attempts + 1"),
			})
		if result.Error != nil {
			return acquiredJobRow{}, false, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		var job model.ExecutorJobModel
		if err := tx.WithContext(ctx).Where("id = ?", jobID).First(&job).Error; err != nil {
			return acquiredJobRow{}, false, err
		}
		return acquiredJobRow{ExecutorJobModel: job, ConsumerID: q.ConsumerID}, true, nil
	}
	return acquiredJobRow{}, false, nil
}

func hasActiveLease(ctx context.Context, tx *gorm.DB, consumerID string, now time.Time) (bool, error) {
	var jobs []model.ExecutorJobModel
	if err := tx.WithContext(ctx).
		Where("lease_owner = ? AND lease_until > ?", consumerID, now).
		Limit(1).
		Find(&jobs).Error; err != nil {
		return false, err
	}
	return len(jobs) > 0, nil
}

func buildMethodSlotValues(slots []MethodSlot) (string, []interface{}) {
	parts := make([]string, 0, len(slots))
	args := make([]interface{}, 0, len(slots)*3)
	for i, slot := range slots {
		// PostgreSQL 对 VALUES 里的占位符类型推断可能与 row_number()/表字段比较错位；
		// 这里显式定型，避免出现 text = bigint 这类运行时 operator 错误。
		parts = append(parts, "(?::text, ?::text, ?::bigint)")
		args = append(args, slot.Method, slot.ConsumerID, i+1)
	}
	return strings.Join(parts, ","), args
}

func buildConsumerValues(slots []MethodSlot) (string, []interface{}) {
	parts := make([]string, 0, len(slots))
	args := make([]interface{}, 0, len(slots)*2)
	for i, slot := range slots {
		parts = append(parts, "(?::text, ?::bigint)")
		args = append(args, slot.ConsumerID, i+1)
	}
	return strings.Join(parts, ","), args
}

func buildMethodValues(methods []string) (string, []interface{}) {
	parts := make([]string, 0, len(methods))
	args := make([]interface{}, 0, len(methods))
	for _, method := range methods {
		parts = append(parts, "(?::text)")
		args = append(args, method)
	}
	return strings.Join(parts, ","), args
}
