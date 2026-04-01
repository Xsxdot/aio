package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/gokit/scheduler"
)

// JobHandler 任务处理函数
// 参数：
// - ctx: 上下文（包含超时控制）
// - job: 已领取的任务信息
// 返回：
// - result: 任务执行结果（会被序列化为 JSON）
// - err: 错误信息（如果返回 JobFailedError，会使用其 RetryAfter）
type JobHandler func(ctx context.Context, job *AcquiredJob) (result interface{}, err error)

// JobFailedError 任务失败错误（带重试选项）
type JobFailedError struct {
	Message        string // 错误信息
	RetryAfter     int32  // 重试延迟（秒），0 表示使用默认退避策略
	ErrorType      string // 错误类型（如 TimeoutError）
	StopRetry      bool   // true 表示立即标记为 dead，不再重试
	AddMaxAttempts int32  // 增加的最大重试次数
}

// Error 实现 error 接口
func (e *JobFailedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s (retry after %ds)", e.Message, e.RetryAfter)
	}
	return e.Message
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
	// TargetService 目标服务名（必填）
	TargetService string
	// ConsumerID 消费者 ID（默认：worker-{timestamp}）
	ConsumerID string
	// LeaseDuration 租约时长（秒，默认 30）
	LeaseDuration int32
	// EnableAutoRenew 是否启用自动续租（默认 true）
	EnableAutoRenew bool
	// RenewInterval 续租间隔（默认 LeaseDuration/3）
	RenewInterval time.Duration
	// ExtendDuration 续租延长时长（秒，默认使用 LeaseDuration）
	ExtendDuration int32
	// TaskTimeout 任务超时时间（默认 25s，应小于 LeaseDuration）
	TaskTimeout time.Duration
	// PollInterval 轮询间隔（默认 1s）
	PollInterval time.Duration
	// OnResultSerializeError 结果序列化失败回调（可选）
	// 默认行为：记录错误但仍然上报成功（避免任务重复执行）
	OnResultSerializeError func(job *AcquiredJob, result interface{}, err error)
	// OnRenewLeaseError 续租失败回调（可选）
	OnRenewLeaseError func(job *AcquiredJob, err error)
}

// DefaultWorkerConfig 默认 Worker 配置
func DefaultWorkerConfig(targetService string) *WorkerConfig {
	return &WorkerConfig{
		TargetService:   targetService,
		ConsumerID:      fmt.Sprintf("worker-%d", time.Now().UnixNano()),
		LeaseDuration:   30,
		EnableAutoRenew: true,
		TaskTimeout:     25 * time.Second,
		PollInterval:    1 * time.Second,
	}
}

// methodHandler 方法处理器（内部）
type methodHandler struct {
	method  string
	handler JobHandler
	taskID  string // scheduler 任务 ID
}

// ExecutorWorker Executor Worker（开箱即用的任务消费者）
type ExecutorWorker struct {
	client    *ExecutorClient
	config    *WorkerConfig
	scheduler *scheduler.Scheduler

	// 方法注册表
	handlers   map[string]*methodHandler
	handlersMu sync.RWMutex

	// 生命周期控制
	isRunning  bool
	runningMu  sync.Mutex
	stopCtx    context.Context
	stopCancel context.CancelFunc
	wg         sync.WaitGroup

	// 是否内部创建 scheduler（需要负责其生命周期）
	ownScheduler bool
}

// NewWorker 创建 Worker（内部自建 scheduler）
func (c *ExecutorClient) NewWorker(config *WorkerConfig) (*ExecutorWorker, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.TargetService == "" {
		return nil, fmt.Errorf("TargetService is required")
	}

	// 创建内部 scheduler
	schedulerConfig := scheduler.DefaultSchedulerConfig()
	schedulerConfig.MaxWorkers = 10 // Worker 内部并发数
	s := scheduler.NewScheduler(schedulerConfig)

	return c.newWorkerWithScheduler(s, config, true)
}

// NewWorkerWithScheduler 创建 Worker（使用外部 scheduler）
func (c *ExecutorClient) NewWorkerWithScheduler(s *scheduler.Scheduler, config *WorkerConfig) (*ExecutorWorker, error) {
	return c.newWorkerWithScheduler(s, config, false)
}

// NewWorkerWithScheduler 创建 Worker（使用外部 scheduler）
func (c *ExecutorClient) newWorkerWithScheduler(s *scheduler.Scheduler, config *WorkerConfig, ownScheduler bool) (*ExecutorWorker, error) {
	if s == nil {
		return nil, fmt.Errorf("scheduler is required")
	}
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.TargetService == "" {
		return nil, fmt.Errorf("TargetService is required")
	}

	// 设置默认值
	if config.ConsumerID == "" {
		config.ConsumerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 30
	}
	if config.TaskTimeout == 0 {
		config.TaskTimeout = 25 * time.Second
	}
	if config.PollInterval == 0 {
		config.PollInterval = 1 * time.Second
	}
	if config.EnableAutoRenew {
		if config.RenewInterval == 0 {
			config.RenewInterval = time.Duration(config.LeaseDuration) * time.Second / 3
		}
		if config.ExtendDuration == 0 {
			config.ExtendDuration = config.LeaseDuration
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ExecutorWorker{
		client:       c,
		config:       config,
		scheduler:    s,
		handlers:     make(map[string]*methodHandler),
		stopCtx:      ctx,
		stopCancel:   cancel,
		ownScheduler: ownScheduler,
	}, nil
}

// Register 注册方法处理器
func (w *ExecutorWorker) Register(method string, handler JobHandler) error {
	if method == "" {
		return fmt.Errorf("method is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	w.handlersMu.Lock()
	defer w.handlersMu.Unlock()

	// 检查是否已注册
	if _, exists := w.handlers[method]; exists {
		return fmt.Errorf("method %s already registered", method)
	}

	// 创建 scheduler 任务
	taskName := fmt.Sprintf("executor-worker-%s-%s", w.config.TargetService, method)
	task := scheduler.NewIntervalTask(
		taskName,
		time.Now(), // 立即开始
		w.config.PollInterval,
		scheduler.TaskExecuteModeLocal,     // 本地执行，不需要分布式锁
		w.config.TaskTimeout+5*time.Second, // 任务超时略长于实际执行超时
		func(ctx context.Context) error {
			return w.processMethod(ctx, method)
		},
	)

	// 保存处理器
	w.handlers[method] = &methodHandler{
		method:  method,
		handler: handler,
		taskID:  task.GetID(),
	}

	// 如果 worker 已启动，立即添加任务到 scheduler
	w.runningMu.Lock()
	isRunning := w.isRunning
	w.runningMu.Unlock()

	if isRunning {
		if err := w.scheduler.AddTask(task); err != nil {
			delete(w.handlers, method)
			return fmt.Errorf("failed to add task to scheduler: %w", err)
		}
	}

	return nil
}

// Unregister 注销方法处理器
func (w *ExecutorWorker) Unregister(method string) error {
	w.handlersMu.Lock()
	defer w.handlersMu.Unlock()

	handler, exists := w.handlers[method]
	if !exists {
		return fmt.Errorf("method %s not registered", method)
	}

	// 从 scheduler 移除任务
	w.scheduler.RemoveTask(handler.taskID)

	// 删除处理器
	delete(w.handlers, method)

	return nil
}

// Start 启动 Worker
func (w *ExecutorWorker) Start() error {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()

	if w.isRunning {
		return fmt.Errorf("worker already started")
	}

	// 如果 scheduler 是内部创建的，先启动 scheduler
	if w.ownScheduler {
		if err := w.scheduler.Start(); err != nil {
			return fmt.Errorf("failed to start scheduler: %w", err)
		}
	}

	// 将所有已注册的方法添加到 scheduler
	w.handlersMu.RLock()
	handlers := make([]*methodHandler, 0, len(w.handlers))
	for _, h := range w.handlers {
		handlers = append(handlers, h)
	}
	w.handlersMu.RUnlock()

	for _, h := range handlers {
		taskName := fmt.Sprintf("executor-worker-%s-%s", w.config.TargetService, h.method)
		task := scheduler.NewIntervalTask(
			taskName,
			time.Now(),
			w.config.PollInterval,
			scheduler.TaskExecuteModeLocal,
			w.config.TaskTimeout+5*time.Second,
			func(ctx context.Context) error {
				return w.processMethod(ctx, h.method)
			},
		)

		// 更新 taskID
		w.handlersMu.Lock()
		if handler, exists := w.handlers[h.method]; exists {
			handler.taskID = task.GetID()
		}
		w.handlersMu.Unlock()

		if err := w.scheduler.AddTask(task); err != nil {
			return fmt.Errorf("failed to add task for method %s: %w", h.method, err)
		}
	}

	w.isRunning = true
	return nil
}

// Stop 停止 Worker
func (w *ExecutorWorker) Stop() error {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()

	if !w.isRunning {
		return nil
	}

	// 取消所有正在执行的任务
	w.stopCancel()

	// 从 scheduler 移除所有任务
	w.handlersMu.RLock()
	taskIDs := make([]string, 0, len(w.handlers))
	for _, h := range w.handlers {
		taskIDs = append(taskIDs, h.taskID)
	}
	w.handlersMu.RUnlock()

	for _, taskID := range taskIDs {
		w.scheduler.RemoveTask(taskID)
	}

	// 如果 scheduler 是内部创建的，停止 scheduler
	if w.ownScheduler {
		if err := w.scheduler.Stop(); err != nil {
			return fmt.Errorf("failed to stop scheduler: %w", err)
		}
	}

	// 等待所有任务完成
	w.wg.Wait()

	w.isRunning = false
	return nil
}

// IsRunning 检查 Worker 是否正在运行
func (w *ExecutorWorker) IsRunning() bool {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()
	return w.isRunning
}

// GetScheduler 获取 Scheduler（用于观测）
func (w *ExecutorWorker) GetScheduler() *scheduler.Scheduler {
	return w.scheduler
}

// processMethod 处理指定方法的任务（由 scheduler 周期调用）
func (w *ExecutorWorker) processMethod(ctx context.Context, method string) error {
	// 获取处理器
	w.handlersMu.RLock()
	handler, exists := w.handlers[method]
	w.handlersMu.RUnlock()

	if !exists {
		return fmt.Errorf("method %s not registered", method)
	}

	// 领取任务
	acquireCtx, acquireCancel := context.WithTimeout(ctx, 5*time.Second)
	defer acquireCancel()

	job, err := w.client.AcquireJob(acquireCtx, &AcquireJobRequest{
		TargetService: w.config.TargetService,
		Method:        method,
		ConsumerID:    w.config.ConsumerID,
		LeaseDuration: w.config.LeaseDuration,
	})
	if err != nil {
		return fmt.Errorf("acquire job failed: %w", err)
	}

	// 没有任务，返回 nil（不算错误）
	if job == nil {
		return nil
	}

	// 执行任务
	w.wg.Add(1)
	defer w.wg.Done()

	return w.executeJob(ctx, job, handler.handler)
}

// executeJob 执行任务
func (w *ExecutorWorker) executeJob(ctx context.Context, job *AcquiredJob, handler JobHandler) error {
	// 创建执行上下文（带超时）
	execCtx, execCancel := context.WithTimeout(ctx, w.config.TaskTimeout)
	defer execCancel()

	// 如果启用自动续租，启动续租 goroutine
	var renewCtx context.Context
	var renewCancel context.CancelFunc
	if w.config.EnableAutoRenew {
		renewCtx, renewCancel = context.WithCancel(ctx)
		defer renewCancel()
		go w.autoRenewLease(renewCtx, job)
	}

	// 执行处理器（捕获 panic）
	var result interface{}
	var handlerErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				handlerErr = fmt.Errorf("handler panic: %v", r)
			}
		}()
		result, handlerErr = handler(execCtx, job)
	}()

	// 停止续租
	if renewCancel != nil {
		renewCancel()
	}

	// 确认任务结果（使用较长超时，因为服务端回调可能触发下游节点提交）
	// 原 5 秒超时在并行节点场景下可能导致竞态问题
	ackCtx, ackCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ackCancel()

	return w.ackJob(ackCtx, job, result, handlerErr)
}

// ackJob 确认任务结果
func (w *ExecutorWorker) ackJob(ctx context.Context, job *AcquiredJob, result interface{}, handlerErr error) error {
	req := &AckJobRequest{
		JobID:      job.JobID,
		AttemptNo:  job.AttemptNo,
		ConsumerID: w.config.ConsumerID,
	}

	if handlerErr != nil {
		// 任务失败
		req.Status = AckStatusFailed
		req.Error = handlerErr.Error()

		// 检查是否是 JobFailedError（带重试选项）
		if jfe, ok := handlerErr.(*JobFailedError); ok {
			req.RetryAfter = jfe.RetryAfter
			req.ErrorType = jfe.ErrorType
			req.StopRetry = jfe.StopRetry
			req.AddMaxAttempts = jfe.AddMaxAttempts
		}
	} else {
		// 任务成功
		req.Status = AckStatusSucceeded

		// 序列化结果
		if result != nil {
			resultJSON, err := json.Marshal(result)
			if err != nil {
				// 结果序列化失败
				if w.config.OnResultSerializeError != nil {
					w.config.OnResultSerializeError(job, result, err)
				}
				// 默认行为：仍然上报成功（避免任务重复执行）
				// 如果需要改变此行为，可以在回调中设置不同的策略
			} else {
				req.ResultJSON = string(resultJSON)
			}
		}
	}

	return w.client.AckJob(ctx, req)
}

// autoRenewLease 自动续租
func (w *ExecutorWorker) autoRenewLease(ctx context.Context, job *AcquiredJob) {
	ticker := time.NewTicker(w.config.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewCtx, renewCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, err := w.client.RenewLease(
				renewCtx,
				job.JobID,
				job.AttemptNo,
				w.config.ConsumerID,
				w.config.ExtendDuration,
			)
			renewCancel()

			if err != nil {
				if w.config.OnRenewLeaseError != nil {
					w.config.OnRenewLeaseError(job, err)
				}
				// 续租失败，停止续租（任务可能已被其他 worker 接管或租约过期）
				return
			}
		}
	}
}
