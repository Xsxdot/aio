// Package sdk 提供 ConcurrentExecutorWorker，在 ExecutorWorker 之上实现单轮询 + ConsumerID 池，
// 支持「任务等待期间继续拉取新任务」，同时空闲时仅调用 1 次 AcquireJob/轮询。
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ConcurrentWorkerConfig 并发 Worker 配置
// 在 WorkerConfig 基础上增加 MaxConcurrent，用于控制每个 worker 最多同时执行的任务数。
type ConcurrentWorkerConfig struct {
	WorkerConfig
	// MaxConcurrent 每个 worker 最多同时持有的任务数（默认 10）
	// 适合 I/O 等待型任务：任务在等待时，其他 slot 可继续拉取新任务
	MaxConcurrent int
}

// DefaultConcurrentWorkerConfig 默认并发 Worker 配置
func DefaultConcurrentWorkerConfig(targetService string) *ConcurrentWorkerConfig {
	return &ConcurrentWorkerConfig{
		WorkerConfig:   *DefaultWorkerConfig(targetService),
		MaxConcurrent: 10,
	}
}

// concurrentMethodHandler 方法处理器（内部）
type concurrentMethodHandler struct {
	method  string
	handler JobHandler
}

// ConcurrentExecutorWorker 并发 Executor Worker
// 单一轮询循环 + ConsumerID 池，每轮最多 1 次 AcquireJob 调用，支持多任务并发执行。
type ConcurrentExecutorWorker struct {
	client *ExecutorClient
	config *ConcurrentWorkerConfig

	handlers   map[string]*concurrentMethodHandler
	handlersMu sync.RWMutex

	// 每个 method 一个轮询 goroutine，共享同一个 freeSlots 池
	freeSlots chan string

	isRunning  bool
	runningMu  sync.Mutex
	stopCtx    context.Context
	stopCancel context.CancelFunc
	wg         sync.WaitGroup
}

// NewConcurrentWorker 创建并发 Worker
func (c *ExecutorClient) NewConcurrentWorker(config *ConcurrentWorkerConfig) (*ConcurrentExecutorWorker, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.TargetService == "" {
		return nil, fmt.Errorf("TargetService is required")
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	// 复用 WorkerConfig 默认值逻辑
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

	// 初始化 freeSlots 池，预填 MaxConcurrent 个 ConsumerID
	freeSlots := make(chan string, config.MaxConcurrent)
	baseID := config.ConsumerID
	for i := 0; i < config.MaxConcurrent; i++ {
		freeSlots <- fmt.Sprintf("%s-slot-%d", baseID, i)
	}

	return &ConcurrentExecutorWorker{
		client:     c,
		config:     config,
		handlers:   make(map[string]*concurrentMethodHandler),
		freeSlots:  freeSlots,
		stopCtx:    ctx,
		stopCancel: cancel,
	}, nil
}

// Register 注册方法处理器
func (w *ConcurrentExecutorWorker) Register(method string, handler JobHandler) error {
	if method == "" {
		return fmt.Errorf("method is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	w.handlersMu.Lock()
	defer w.handlersMu.Unlock()

	if _, exists := w.handlers[method]; exists {
		return fmt.Errorf("method %s already registered", method)
	}

	w.handlers[method] = &concurrentMethodHandler{
		method:  method,
		handler: handler,
	}
	return nil
}

// Start 启动 Worker
func (w *ConcurrentExecutorWorker) Start() error {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()

	if w.isRunning {
		return fmt.Errorf("worker already started")
	}

	w.handlersMu.RLock()
	methods := make([]string, 0, len(w.handlers))
	for m := range w.handlers {
		methods = append(methods, m)
	}
	w.handlersMu.RUnlock()

	for _, method := range methods {
		m := method
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.pollLoop(m)
		}()
	}

	w.isRunning = true
	return nil
}

// Stop 停止 Worker
func (w *ConcurrentExecutorWorker) Stop() error {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()

	if !w.isRunning {
		return nil
	}

	w.stopCancel()
	w.wg.Wait()

	w.isRunning = false
	return nil
}

// IsRunning 检查 Worker 是否正在运行
func (w *ConcurrentExecutorWorker) IsRunning() bool {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()
	return w.isRunning
}

// pollLoop 单一轮询循环：每轮最多 1 次 AcquireJob，使用池中的空闲 ConsumerID
func (w *ConcurrentExecutorWorker) pollLoop(method string) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCtx.Done():
			return
		case <-ticker.C:
			w.tryAcquireAndExecute(method)
		}
	}
}

// tryAcquireAndExecute 尝试领取并执行（每轮最多 1 次 API 调用）
func (w *ConcurrentExecutorWorker) tryAcquireAndExecute(method string) {
	select {
	case consumerID := <-w.freeSlots:
		acquireCtx, cancel := context.WithTimeout(w.stopCtx, 5*time.Second)
		job, err := w.client.AcquireJob(acquireCtx, &AcquireJobRequest{
			TargetService: w.config.TargetService,
			Method:        method,
			ConsumerID:    consumerID,
			LeaseDuration: w.config.LeaseDuration,
		})
		cancel()

		if err != nil || job == nil {
			w.freeSlots <- consumerID
			return
		}

		w.handlersMu.RLock()
		h, exists := w.handlers[method]
		w.handlersMu.RUnlock()

		if !exists {
			w.freeSlots <- consumerID
			return
		}

		w.wg.Add(1)
		go w.runHandlerAndRelease(job, consumerID, h.handler)
	default:
		// 无空闲 slot，本轮不调用 API
	}
}

// runHandlerAndRelease 执行 handler 并在完成后将 slot 归还池
func (w *ConcurrentExecutorWorker) runHandlerAndRelease(job *AcquiredJob, consumerID string, handler JobHandler) {
	defer w.wg.Done()
	defer func() { w.freeSlots <- consumerID }()

	w.executeJobWithConsumerID(w.stopCtx, job, consumerID, handler)
}

// executeJobWithConsumerID 执行任务（使用指定 consumerID，复用 ExecutorWorker 逻辑）
func (w *ConcurrentExecutorWorker) executeJobWithConsumerID(ctx context.Context, job *AcquiredJob, consumerID string, handler JobHandler) {
	execCtx, execCancel := context.WithTimeout(ctx, w.config.TaskTimeout)
	defer execCancel()

	var renewCancel context.CancelFunc
	if w.config.EnableAutoRenew {
		renewCtx, cancel := context.WithCancel(ctx)
		renewCancel = cancel
		defer renewCancel()
		go w.autoRenewLeaseWithConsumerID(renewCtx, job, consumerID)
	}

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

	if renewCancel != nil {
		renewCancel()
	}

	ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ackCancel()

	w.ackJobWithConsumerID(ackCtx, job, consumerID, result, handlerErr)
}

// ackJobWithConsumerID 确认任务结果
func (w *ConcurrentExecutorWorker) ackJobWithConsumerID(ctx context.Context, job *AcquiredJob, consumerID string, result interface{}, handlerErr error) {
	req := &AckJobRequest{
		JobID:      job.JobID,
		AttemptNo:  job.AttemptNo,
		ConsumerID: consumerID,
	}

	if handlerErr != nil {
		req.Status = AckStatusFailed
		req.Error = handlerErr.Error()
		if jfe, ok := handlerErr.(*JobFailedError); ok {
			req.RetryAfter = jfe.RetryAfter
			req.ErrorType = jfe.ErrorType
			req.StopRetry = jfe.StopRetry
			req.AddMaxAttempts = jfe.AddMaxAttempts
		}
	} else {
		req.Status = AckStatusSucceeded
		if result != nil {
			resultJSON, err := json.Marshal(result)
			if err != nil {
				if w.config.OnResultSerializeError != nil {
					w.config.OnResultSerializeError(job, result, err)
				}
			} else {
				req.ResultJSON = string(resultJSON)
			}
		}
	}

	_ = w.client.AckJob(ctx, req)
}

// autoRenewLeaseWithConsumerID 自动续租
func (w *ConcurrentExecutorWorker) autoRenewLeaseWithConsumerID(ctx context.Context, job *AcquiredJob, consumerID string) {
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
				consumerID,
				w.config.ExtendDuration,
			)
			renewCancel()

			if err != nil {
				if w.config.OnRenewLeaseError != nil {
					w.config.OnRenewLeaseError(job, err)
				}
				return
			}
		}
	}
}
