package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/api/dto"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	errorc "github.com/xsxdot/gokit/err"

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
	dao      *dao.ExecutorJobDAO
	handlers map[string]callback.JobCompletionHandler // 按 Source 注册的任务完成处理器
	mu       sync.RWMutex
	err      *errorc.ErrorBuilder
}

// NewExecutorJobService 创建任务服务实例
func NewExecutorJobService() *ExecutorJobService {
	return &ExecutorJobService{
		dao:      dao.NewExecutorJobDAO(),
		handlers: make(map[string]callback.JobCompletionHandler),
		err:      errorc.NewErrorBuilder("ExecutorJobService"),
	}
}

// RegisterJobCompletionHandler 注册任务完成处理器（按 Source 路由）
func (s *ExecutorJobService) RegisterJobCompletionHandler(source string, h callback.JobCompletionHandler) {
	if source == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[source] = h
}

// SubmitJob 提交任务
func (s *ExecutorJobService) SubmitJob(ctx context.Context, req *dto.SubmitJobInput) (uint64, error) {
	e, err := requireEnv(req.Env)
	if err != nil {
		return 0, err
	}

	if strings.TrimSpace(req.DedupKey) == "" {
		return 0, errors.New("dedupKey 不能为空")
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	retryBackoffType := model.RetryBackoffType(req.RetryBackoffType)
	if retryBackoffType == "" {
		retryBackoffType = model.RetryBackoffExponential
	}

	var nextRunAtTime time.Time
	if req.RunAt > 0 {
		nextRunAtTime = time.Unix(req.RunAt, 0)
	} else {
		nextRunAtTime = time.Now()
	}

	// 检查幂等键（按 env 隔离）
	existingJob, err := s.dao.GetByDedupKey(ctx, e, req.DedupKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if err == nil {
		switch existingJob.Status {
		case model.JobStatusFailed, model.JobStatusCanceled, model.JobStatusDead:
			n, resubmitErr := s.dao.ResubmitTerminalJobByDedupKey(ctx, e, req.DedupKey,
				req.TargetService, req.Method, req.ArgsJSON,
				strings.TrimSpace(req.CallbackData), strings.TrimSpace(req.Source), strings.TrimSpace(req.SequenceKey),
				maxAttempts, req.Priority, retryBackoffType, req.RetryIntervalSec, nextRunAtTime)
			if resubmitErr != nil {
				return 0, resubmitErr
			}
			if n > 0 {
				base.Logger.Infof("终态任务已按新参数重新入队: dedup_key=%s", req.DedupKey)
				return uint64(existingJob.ID), nil
			}
			jobAgain, err2 := s.dao.GetByDedupKey(ctx, e, req.DedupKey)
			if err2 != nil {
				if !errors.Is(err2, gorm.ErrRecordNotFound) {
					return 0, err2
				}
				// 行已不存在，退化为新建
			} else {
				base.Logger.Info("任务已存在，返回已有任务ID")
				return uint64(jobAgain.ID), nil
			}
		default:
			base.Logger.Info("任务已存在，返回已有任务ID")
			return uint64(existingJob.ID), nil
		}
	}

	nextRunAt := &nextRunAtTime

	job := &model.ExecutorJobModel{
		Env:              e,
		TargetService:    req.TargetService,
		Method:           req.Method,
		ArgsJSON:         req.ArgsJSON,
		Status:           model.JobStatusPending,
		Priority:         req.Priority,
		NextRunAt:        nextRunAt,
		MaxAttempts:      maxAttempts,
		Attempts:         0,
		DedupKey:         req.DedupKey,
		RetryBackoffType: retryBackoffType,
		RetryIntervalSec: req.RetryIntervalSec,
		SequenceKey:      strings.TrimSpace(req.SequenceKey),
		Source:           strings.TrimSpace(req.Source),
		CallbackData:     req.CallbackData,
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

// AcquireJobsRequest 是服务层批量领取任务入参。
type AcquireJobsRequest struct {
	Env            string
	TargetService  string
	Methods        []string
	BaseConsumerID string
	ConsumerIDs    []string
	LeaseDuration  int32
	Mode           dao.AcquireJobsMode
}

// AcquiredJobResult 是服务层批量领取任务结果。
type AcquiredJobResult struct {
	Job        *model.ExecutorJobModel
	AttemptNo  int32
	ConsumerID string
}

// AcquireJobs 批量领取任务。
//
// 参数：
//   - ctx: 请求上下文
//   - req: 批量领取参数
//
// 返回：
//   - 已领取任务列表；无任务时为空切片
//   - 参数或数据库错误
//
// 注意：
//   - ONE_PER_METHOD 下 server 由 base_consumer_id 派生每 method slot
//   - FILL_SLOTS 下 consumer_ids 表示 SDK 当前空闲 slot 池
func (s *ExecutorJobService) AcquireJobs(ctx context.Context, req AcquireJobsRequest) ([]*AcquiredJobResult, error) {
	fail := func(err error) ([]*AcquiredJobResult, error) {
		base.Logger.WithErr(err).
			WithField("mode", req.Mode).
			WithField("method_count", len(req.Methods)).
			WithField("slot_count", len(req.ConsumerIDs)).
			Error("批量领取任务参数错误")
		return nil, err
	}

	e, err := requireEnv(req.Env)
	if err != nil {
		return fail(err)
	}
	targetService := strings.TrimSpace(req.TargetService)
	if targetService == "" {
		return fail(errors.New("targetService 不能为空"))
	}
	methods := normalizeNonEmptyStrings(req.Methods)
	if len(methods) == 0 {
		return fail(errors.New("methods 不能为空"))
	}
	if hasDuplicate(methods) {
		return fail(errors.New("methods 不能重复"))
	}
	if req.Mode != dao.AcquireJobsModeOnePerMethod && req.Mode != dao.AcquireJobsModeFillSlots {
		return fail(errors.New("mode 不合法"))
	}

	var methodSlots []dao.MethodSlot
	switch req.Mode {
	case dao.AcquireJobsModeOnePerMethod:
		baseConsumerID := strings.TrimSpace(req.BaseConsumerID)
		if baseConsumerID == "" {
			return fail(errors.New("base_consumer_id 不能为空"))
		}
		methodSlots = make([]dao.MethodSlot, 0, len(methods))
		for _, method := range methods {
			// 使用 method 原文派生 slot，避免 hash 碰撞并保留排障可读性。
			methodSlots = append(methodSlots, dao.MethodSlot{
				Method:     method,
				ConsumerID: baseConsumerID + "-m-" + method,
			})
		}
	case dao.AcquireJobsModeFillSlots:
		consumerIDs := normalizeNonEmptyStrings(req.ConsumerIDs)
		if len(consumerIDs) == 0 {
			return fail(errors.New("consumer_ids 不能为空"))
		}
		if hasDuplicate(consumerIDs) {
			return fail(errors.New("consumer_ids 不能重复"))
		}
		methodSlots = make([]dao.MethodSlot, 0, len(consumerIDs))
		for _, consumerID := range consumerIDs {
			methodSlots = append(methodSlots, dao.MethodSlot{ConsumerID: consumerID})
		}
	}
	if req.LeaseDuration <= 0 {
		req.LeaseDuration = 30
	}

	leases, err := s.dao.AcquireJobs(ctx, dao.AcquireJobsInput{
		Env:           e,
		TargetService: targetService,
		Methods:       methods,
		MethodSlots:   methodSlots,
		LeaseDuration: req.LeaseDuration,
		Mode:          req.Mode,
	})
	if err != nil {
		base.Logger.WithErr(err).
			WithField("mode", req.Mode).
			WithField("method_count", len(methods)).
			WithField("slot_count", len(methodSlots)).
			Error("批量领取任务失败")
		return nil, err
	}
	out := make([]*AcquiredJobResult, 0, len(leases))
	for _, lease := range leases {
		out = append(out, &AcquiredJobResult{
			Job:        lease.Job,
			AttemptNo:  lease.AttemptNo,
			ConsumerID: lease.ConsumerID,
		})
	}
	if len(out) > 0 {
		base.Logger.WithField("mode", req.Mode).
			WithField("job_count", len(out)).
			WithField("method_count", len(methods)).
			WithField("slot_count", len(methodSlots)).
			Info("批量领取任务成功")
	}
	return out, nil
}

func normalizeNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func hasDuplicate(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
	}
	return false
}

// RenewLease 续租
func (s *ExecutorJobService) RenewLease(ctx context.Context, jobID uint64, attemptNo int32, consumerID string, extendDuration int32) (*model.ExecutorJobModel, error) {
	if extendDuration <= 0 {
		extendDuration = 30
	}

	job, err := s.dao.RenewLease(ctx, jobID, attemptNo, consumerID, extendDuration)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, s.err.New("任务不存在或租约信息不匹配", err).WithTraceID(ctx)
		}
		return nil, err
	}

	base.Logger.Debug("任务租约续期成功")

	return job, nil
}

// AckJob 确认任务执行结果。
//
// 参数：
//   - jobID / attemptNo / consumerID: 定位任务与校验租约归属
//   - status: 本次执行结果
//   - errorMsg / errorType / resultJSON: 执行输出
//   - retryAfter / stopRetry / addMaxAttempts: 重试控制
//
// 返回：确认失败或 outbox 提交失败时返回错误，调用方应重试整个 Ack。
//
// 注意：状态更新与回调 outbox 任务在同一事务内提交，因此不存在
// 「任务已终态但回调丢失」的窗口——这正是本方法改造前的永久卡死成因。
func (s *ExecutorJobService) AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
	status model.JobStatus, errorMsg, resultJSON string, retryAfter int32,
	stopRetry bool, addMaxAttempts int32, errorType string) error {

	return mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
		txCtx := mvc.WithTxToContext(ctx, tx)

		job, preErr := s.dao.GetByID(txCtx, jobID)
		var source, callbackData, env string
		if preErr == nil && job != nil {
			source, callbackData, env = job.Source, job.CallbackData, job.Env
		}

		if err := s.dao.AckJob(txCtx, jobID, attemptNo, consumerID, status,
			errorMsg, resultJSON, retryAfter, stopRetry, addMaxAttempts, errorType); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return s.err.New("任务不存在或租约信息不匹配", err).WithTraceID(ctx)
			}
			return err
		}

		base.Logger.WithField("job_id", jobID).WithField("status", status).Info("任务确认成功")

		if source == "" {
			return nil
		}

		payloadJSON := resultJSON
		needCallback := false
		switch status {
		case model.JobStatusSucceeded:
			needCallback = true
		case model.JobStatusFailed:
			// 重试耗尽转死信时，Workflow 需要收到回调以走 error 边。
			// 必须在同事务内回读——DAO 刚写入的状态尚未提交，走 base.DB 读不到。
			after, aErr := s.dao.GetByID(txCtx, jobID)
			if aErr == nil && after != nil && after.Status == model.JobStatusDead {
				needCallback = true
				b, marshalErr := json.Marshal(map[string]string{"error_msg": errorMsg})
				if marshalErr != nil {
					return s.err.New("序列化失败回调载荷失败", marshalErr).WithTraceID(ctx)
				}
				payloadJSON = string(b)
			}
		}
		if !needCallback {
			return nil
		}

		if !callbackViaOutbox() {
			// 过渡开关：退回改造前的同步回调。稳定后连同本分支一并删除。
			s.mu.RLock()
			handler := s.handlers[source]
			s.mu.RUnlock()
			if handler == nil {
				return nil
			}
			if cbErr := handler.OnJobCompleted(ctx, jobID, callbackData, payloadJSON); cbErr != nil {
				base.Logger.WithErr(cbErr).WithField("job_id", jobID).WithField("source", source).
					Error("同步回调失败（sync 模式无重试，工作流可能卡死）")
			}
			return nil
		}

		return s.submitCallbackOutbox(txCtx, env, source, jobID, callbackData, payloadJSON)
	})
}

// submitCallbackOutbox 在当前事务内提交一条承载完成回调的 outbox 任务。
//
// 注意：Source 必须留空。若非空，这条回调任务自身 Ack 时会再次生成回调任务，
// 形成无限递归。
func (s *ExecutorJobService) submitCallbackOutbox(ctx context.Context, env, source string,
	jobID uint64, callbackData, resultJSON string) error {
	payload := callback.CallbackPayload{
		JobID:        jobID,
		Source:       source,
		CallbackData: callbackData,
		ResultJSON:   resultJSON,
	}
	argsJSON, err := json.Marshal(payload)
	if err != nil {
		return s.err.New("序列化回调载荷失败", err).WithTraceID(ctx)
	}

	dedupKey := callback.OutboxDedupKeyPrefix + strconv.FormatUint(jobID, 10)
	if _, err := s.SubmitJob(ctx, &dto.SubmitJobInput{
		Env:              env,
		TargetService:    callback.InternalTargetService,
		Method:           callback.MethodJobCompletedCallback,
		ArgsJSON:         string(argsJSON),
		MaxAttempts:      callback.OutboxMaxAttempts,
		Priority:         callback.OutboxPriority,
		DedupKey:         dedupKey,
		RetryBackoffType: dto.RetryBackoffExponential,
		Source:           "",
	}); err != nil {
		base.Logger.WithErr(err).WithField("job_id", jobID).WithField("source", source).
			Error("提交回调 outbox 任务失败，本次 Ack 将整体回滚")
		return err
	}

	base.Logger.WithField("job_id", jobID).
		WithField("source", source).
		WithField("dedup_key", dedupKey).
		Info("回调 outbox 任务已入队")
	return nil
}

// callbackViaOutbox 判定是否走 outbox 回调。
//
// 注意：配置缺失（含测试环境 base.Configures 为 nil）时返回 true——
// 未配置即新行为，退回同步回调必须显式配置 workflow.callback-mode: sync。
func callbackViaOutbox() bool {
	if base.Configures == nil {
		return true
	}
	return base.Configures.Config.Workflow.CallbackMode != "sync"
}

// DispatchCompletionCallback 按 source 路由到已注册的完成处理器。
//
// 参数：source 为提交任务时指定的来源标识；jobID 为原始任务 ID。
//
// 返回：未注册对应 source 时返回错误（让 outbox 任务重试，
// 避免注册时序问题导致回调被静默丢弃）。
func (s *ExecutorJobService) DispatchCompletionCallback(ctx context.Context, source string,
	jobID uint64, callbackData, resultJSON string) error {
	s.mu.RLock()
	handler := s.handlers[source]
	s.mu.RUnlock()
	if handler == nil {
		return s.err.New("未注册 source 对应的完成处理器: "+source, nil).WithTraceID(ctx)
	}
	return handler.OnJobCompleted(ctx, jobID, callbackData, resultJSON)
}

// GetJob 获取任务详情
func (s *ExecutorJobService) GetJob(ctx context.Context, jobID uint64) (*model.ExecutorJobModel, error) {
	job, err := s.dao.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, s.err.New("任务不存在", err).WithTraceID(ctx)
		}
		return nil, err
	}
	return job, nil
}

// GetJobByDedupKey 根据环境+幂等键获取任务（供 Workflow 等组件按 dedupKey 查找并取消任务）
func (s *ExecutorJobService) GetJobByDedupKey(ctx context.Context, env, dedupKey string) (*model.ExecutorJobModel, error) {
	e, err := requireEnv(env)
	if err != nil {
		return nil, err
	}
	job, err := s.dao.GetByDedupKey(ctx, e, dedupKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// CancelActiveJobsByDedupKeyPrefix 取消 env 下 dedup_key 以 prefix 开头的 pending/running 任务（与带后缀的派发幂等键配套）
func (s *ExecutorJobService) CancelActiveJobsByDedupKeyPrefix(ctx context.Context, env, dedupKeyPrefix string) error {
	p := strings.TrimSpace(dedupKeyPrefix)
	if p == "" {
		return errors.New("dedupKeyPrefix 不能为空")
	}
	e, err := requireEnv(env)
	if err != nil {
		return err
	}
	jobs, err := s.dao.ListActiveByDedupKeyPrefix(ctx, e, p)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if err := s.CancelJob(ctx, uint64(j.ID)); err != nil {
			base.Logger.Infof("按前缀取消任务跳过: job_id=%d err=%v", j.ID, err)
		}
	}
	return nil
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
			return s.err.New("任务不存在", err).WithTraceID(ctx)
		}
		return err
	}

	// 只有 pending 或 running（租约过期）的任务才能取消
	if job.Status != model.JobStatusPending && job.Status != model.JobStatusRunning {
		return s.err.New("只有待执行或执行中的任务才能取消", nil).WithTraceID(ctx)
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
			return s.err.New("任务不存在", err).WithTraceID(ctx)
		}
		return err
	}

	// 只有 failed、canceled、dead 状态的任务才能重新入队
	if job.Status != model.JobStatusFailed &&
		job.Status != model.JobStatusCanceled &&
		job.Status != model.JobStatusDead {
		return s.err.New("只有失败、已取消或死信状态的任务才能重新入队", nil).WithTraceID(ctx)
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
			return s.err.New("任务不存在", err).WithTraceID(ctx)
		}
		return err
	}

	// 只有非 running 状态的任务才能修改参数
	if job.Status == model.JobStatusRunning {
		return s.err.New("running 任务不允许修改参数", nil).WithTraceID(ctx)
	}

	// 更新参数
	err = s.dao.UpdateArgsJSON(ctx, jobID, argsJSON)
	if err != nil {
		return err
	}

	base.Logger.Info("任务参数更新成功")

	return nil
}
