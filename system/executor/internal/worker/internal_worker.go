// Package worker 提供 aio 进程内的回调消费者。
//
// 职责：消费 executor 中 target_service=aio、method=internal.job_completed_callback
// 的 outbox 任务，按 source 路由到已注册的 JobCompletionHandler。
//
// 边界：不承载任何业务逻辑，不处理其他 method；只消费本进程 env 的任务——
// 跨 env 的回调需由对应 env 的 aio 实例消费。直调 service 而非绕 gRPC：同进程
// 无需网络层，也避免自连自身的启动顺序依赖。
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/aio/system/executor/internal/service"
	"github.com/xsxdot/gokit/logger"
)

const (
	pollInterval  = time.Second
	leaseDuration = 60
	maxConcurrent = 5
)

// JobRunner 是内部 worker 依赖的最小任务接口，由 ExecutorJobService 实现。
type JobRunner interface {
	AcquireJobs(ctx context.Context, req service.AcquireJobsRequest) ([]*service.AcquiredJobResult, error)
	AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
		status model.JobStatus, errorMsg, resultJSON string, retryAfter int32,
		stopRetry bool, addMaxAttempts int32, errorType string) error
	DispatchCompletionCallback(ctx context.Context, source string, jobID uint64,
		callbackData, resultJSON string) error
}

// InternalCallbackWorker 消费 outbox 回调任务的进程内 worker。
type InternalCallbackWorker struct {
	runner     JobRunner
	env        string
	consumerID string
	log        *logger.Log

	stopCtx    context.Context
	stopCancel context.CancelFunc
	wg         sync.WaitGroup
	startOnce  sync.Once
	stopOnce   sync.Once
}

// NewInternalCallbackWorker 创建内部回调 worker。
//
// 参数：
//   - runner: 任务领取/确认/派发能力，生产环境传 *service.ExecutorJobService
//   - env: 消费的环境标识，通常为 base.ENV
//   - consumerID: 实例标识，多实例下必须互不相同
func NewInternalCallbackWorker(runner JobRunner, env, consumerID string, log *logger.Log) *InternalCallbackWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &InternalCallbackWorker{
		runner:     runner,
		env:        env,
		consumerID: consumerID,
		log:        log,
		stopCtx:    ctx,
		stopCancel: cancel,
	}
}

// Start 启动轮询循环。重复调用无副作用。
func (w *InternalCallbackWorker) Start() {
	w.startOnce.Do(func() {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.log.WithField("env", w.env).WithField("consumer_id", w.consumerID).
				Info("内部回调 worker 已启动")
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-w.stopCtx.Done():
					w.log.Info("内部回调 worker 已停止")
					return
				case <-ticker.C:
					w.pollOnce(w.stopCtx)
				}
			}
		}()
	})
}

// Stop 停止轮询并等待在途回调结束。重复调用无副作用。
func (w *InternalCallbackWorker) Stop() {
	w.stopOnce.Do(func() {
		w.stopCancel()
		w.wg.Wait()
	})
}

// pollOnce 领取并处理一轮 outbox 回调任务。
// 单独保留为方法便于测试直接驱动一轮，不必等待 ticker。
func (w *InternalCallbackWorker) pollOnce(ctx context.Context) {
	consumerIDs := make([]string, 0, maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		consumerIDs = append(consumerIDs, fmt.Sprintf("%s-slot-%d", w.consumerID, i))
	}

	jobs, err := w.runner.AcquireJobs(ctx, service.AcquireJobsRequest{
		Env:           w.env,
		TargetService: callback.InternalTargetService,
		Methods:       []string{callback.MethodJobCompletedCallback},
		ConsumerIDs:   consumerIDs,
		LeaseDuration: leaseDuration,
		Mode:          dao.AcquireJobsModeFillSlots,
	})
	if err != nil {
		w.log.WithErr(err).WithField("env", w.env).Error("内部回调 worker 领取任务失败")
		return
	}
	if len(jobs) == 0 {
		return
	}
	w.log.WithField("count", len(jobs)).Debug("内部回调 worker 领到任务")

	for _, j := range jobs {
		w.handle(ctx, j)
	}
}

func (w *InternalCallbackWorker) handle(ctx context.Context, j *service.AcquiredJobResult) {
	jobID := uint64(j.Job.ID)
	started := time.Now()

	var payload callback.CallbackPayload
	if err := json.Unmarshal([]byte(j.Job.ArgsJSON), &payload); err != nil {
		w.log.WithErr(err).WithField("outbox_job_id", jobID).
			Error("解析回调载荷失败，标记失败等待重试")
		w.ack(ctx, jobID, j, model.JobStatusFailed, "解析回调载荷失败: "+err.Error(), "")
		return
	}

	w.log.WithField("outbox_job_id", jobID).
		WithField("origin_job_id", payload.JobID).
		WithField("source", payload.Source).
		Debug("开始派发完成回调")

	if err := w.runner.DispatchCompletionCallback(ctx, payload.Source, payload.JobID,
		payload.CallbackData, payload.ResultJSON); err != nil {
		w.log.WithErr(err).
			WithField("outbox_job_id", jobID).
			WithField("origin_job_id", payload.JobID).
			WithField("source", payload.Source).
			Error("派发完成回调失败，标记失败等待重试")
		w.ack(ctx, jobID, j, model.JobStatusFailed, err.Error(), "")
		return
	}

	w.log.WithField("outbox_job_id", jobID).
		WithField("origin_job_id", payload.JobID).
		WithField("source", payload.Source).
		WithField("cost_ms", time.Since(started).Milliseconds()).
		Info("完成回调派发成功")
	w.ack(ctx, jobID, j, model.JobStatusSucceeded, "", "")
}

func (w *InternalCallbackWorker) ack(ctx context.Context, jobID uint64,
	j *service.AcquiredJobResult, status model.JobStatus, errMsg, resultJSON string) {
	if err := w.runner.AckJob(ctx, jobID, j.AttemptNo, j.ConsumerID,
		status, errMsg, resultJSON, 0, false, 0, ""); err != nil {
		// 确认失败不重试：租约到期后 executor 会重投，重复派发由 workflow 侧幂等表拦截。
		w.log.WithErr(err).WithField("outbox_job_id", jobID).WithField("status", status).
			Error("内部回调 worker 确认任务失败，等待租约到期重投")
	}
}
