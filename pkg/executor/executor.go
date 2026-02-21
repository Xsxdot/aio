package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/core/logger"
)

// Executor 后台任务执行器（单 worker 顺序执行）
type Executor struct {
	config *Config
	log    *logger.Log

	// 任务队列（有界 channel，保证顺序执行）
	queue chan *Job

	// 任务历史记录（jobID -> Job）
	history   map[string]*Job
	historyMu sync.RWMutex

	// 历史记录的有序列表（用于 LRU 淘汰）
	historyOrder []string

	// 运行状态
	isRunning bool
	runningMu sync.Mutex

	// worker 停止信号
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewExecutor 创建新的 Executor 实例
func NewExecutor(config *Config) *Executor {
	if config == nil {
		config = DefaultConfig()
	}

	return &Executor{
		config:       config,
		log:          logger.GetLogger().WithEntryName("Executor"),
		queue:        make(chan *Job, config.QueueSize),
		history:      make(map[string]*Job),
		historyOrder: make([]string, 0, config.MaxHistorySize),
		stopChan:     make(chan struct{}),
	}
}

// Start 启动 Executor（启动 worker goroutine）
func (e *Executor) Start() error {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()

	if e.isRunning {
		return fmt.Errorf("executor already running")
	}

	e.isRunning = true
	e.log.Info("Executor 启动成功，单 worker 顺序执行模式")

	// 启动单个 worker goroutine
	e.wg.Add(1)
	go e.worker()

	return nil
}

// Stop 停止 Executor（优雅关闭）
func (e *Executor) Stop() error {
	e.runningMu.Lock()
	if !e.isRunning {
		e.runningMu.Unlock()
		return nil
	}
	e.isRunning = false
	e.runningMu.Unlock()

	e.log.Info("Executor 正在停止...")

	// 发送停止信号
	close(e.stopChan)

	// 等待 worker 退出
	e.wg.Wait()

	// 将队列中剩余的任务标记为 canceled
	e.cancelPendingJobs()

	e.log.Info("Executor 已停止")
	return nil
}

// Submit 提交任务到队列（返回 jobID）
func (e *Executor) Submit(ctx context.Context, name string, timeout time.Duration, fn JobFunc) (string, error) {
	e.runningMu.Lock()
	if !e.isRunning {
		e.runningMu.Unlock()
		return "", fmt.Errorf("executor not running")
	}
	e.runningMu.Unlock()

	// 如果未指定超时，使用默认超时
	if timeout <= 0 {
		timeout = e.config.DefaultTimeout
	}

	// 创建任务
	job := NewJob(ctx, name, timeout, fn)

	// 尝试放入队列（非阻塞）
	select {
	case e.queue <- job:
		// 加入历史记录
		e.addToHistory(job)
		e.log.WithField("jobId", job.ID).
			WithField("name", job.Name).
			WithField("traceId", job.TraceID).
			Info("任务已提交到队列")
		return job.ID, nil
	default:
		// 队列已满
		return "", fmt.Errorf("executor queue is full")
	}
}

// GetJob 查询任务状态
func (e *Executor) GetJob(jobID string) (*Job, error) {
	e.historyMu.RLock()
	defer e.historyMu.RUnlock()

	job, exists := e.history[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// 返回副本，避免外部修改
	jobCopy := &Job{
		ID:         job.ID,
		Name:       job.Name,
		Status:     job.Status,
		CreatedAt:  job.CreatedAt,
		StartedAt:  job.StartedAt,
		FinishedAt: job.FinishedAt,
		Error:      job.Error,
		TraceID:    job.TraceID,
		Timeout:    job.Timeout,
	}

	return jobCopy, nil
}

// ListJobs 列出所有任务（可选状态过滤）
func (e *Executor) ListJobs(status JobStatus) []*Job {
	e.historyMu.RLock()
	defer e.historyMu.RUnlock()

	jobs := make([]*Job, 0)
	for _, jobID := range e.historyOrder {
		job, exists := e.history[jobID]
		if !exists {
			continue
		}
		// 状态过滤
		if status != "" && job.Status != status {
			continue
		}
		// 返回副本
		jobCopy := &Job{
			ID:         job.ID,
			Name:       job.Name,
			Status:     job.Status,
			CreatedAt:  job.CreatedAt,
			StartedAt:  job.StartedAt,
			FinishedAt: job.FinishedAt,
			Error:      job.Error,
			TraceID:    job.TraceID,
			Timeout:    job.Timeout,
		}
		jobs = append(jobs, jobCopy)
	}
	return jobs
}

// GetStats 获取统计信息
func (e *Executor) GetStats() map[string]interface{} {
	e.historyMu.RLock()
	defer e.historyMu.RUnlock()

	stats := map[string]interface{}{
		"queue_size":    len(e.queue),
		"queue_cap":     cap(e.queue),
		"history_count": len(e.history),
	}

	// 统计各状态任务数
	statusCount := make(map[JobStatus]int)
	for _, job := range e.history {
		statusCount[job.Status]++
	}
	stats["status_count"] = statusCount

	return stats
}

// worker 单个 worker goroutine，顺序执行任务
func (e *Executor) worker() {
	defer e.wg.Done()

	for {
		select {
		case <-e.stopChan:
			// 收到停止信号
			e.log.Info("Worker 收到停止信号，退出")
			return
		case job := <-e.queue:
			// 执行任务
			e.executeJob(job)
		}
	}
}

// executeJob 执行单个任务
func (e *Executor) executeJob(job *Job) {
	// 更新状态为 running
	now := time.Now()
	job.StartedAt = &now
	job.Status = JobStatusRunning
	e.updateHistory(job)

	e.log.WithField("jobId", job.ID).
		WithField("name", job.Name).
		WithField("traceId", job.TraceID).
		Info("开始执行任务")

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(job.ctx, job.Timeout)
	job.cancel = cancel
	defer cancel()

	// 执行任务函数
	err := job.fn(ctx)

	// 更新任务状态
	finishedAt := time.Now()
	job.FinishedAt = &finishedAt

	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
		e.log.WithField("jobId", job.ID).
			WithField("name", job.Name).
			WithField("traceId", job.TraceID).
			WithField("duration", job.Duration()).
			WithErr(err).
			Error("任务执行失败")
	} else {
		job.Status = JobStatusSucceeded
		e.log.WithField("jobId", job.ID).
			WithField("name", job.Name).
			WithField("traceId", job.TraceID).
			WithField("duration", job.Duration()).
			Info("任务执行成功")
	}

	e.updateHistory(job)
}

// addToHistory 将任务加入历史记录
func (e *Executor) addToHistory(job *Job) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	// 如果历史已满，淘汰最旧的已完成/失败/取消任务
	if len(e.history) >= e.config.MaxHistorySize {
		e.evictOldestFinished()
	}

	e.history[job.ID] = job
	e.historyOrder = append(e.historyOrder, job.ID)
}

// updateHistory 更新历史记录中的任务状态
func (e *Executor) updateHistory(job *Job) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	e.history[job.ID] = job
}

// evictOldestFinished 淘汰最旧的已完成任务
func (e *Executor) evictOldestFinished() {
	// 找到第一个已完成/失败/取消的任务
	for i, jobID := range e.historyOrder {
		job, exists := e.history[jobID]
		if !exists {
			continue
		}
		// 只淘汰已终止的任务（不淘汰 pending/running）
		if job.Status == JobStatusSucceeded || job.Status == JobStatusFailed || job.Status == JobStatusCanceled {
			delete(e.history, jobID)
			e.historyOrder = append(e.historyOrder[:i], e.historyOrder[i+1:]...)
			e.log.WithField("jobId", jobID).Debug("淘汰历史任务")
			return
		}
	}

	// 如果没有可淘汰的（全是 pending/running），强制淘汰最旧的
	if len(e.historyOrder) > 0 {
		oldestID := e.historyOrder[0]
		delete(e.history, oldestID)
		e.historyOrder = e.historyOrder[1:]
		e.log.WithField("jobId", oldestID).Warn("强制淘汰历史任务（pending/running）")
	}
}

// cancelPendingJobs 取消队列中所有待执行的任务
func (e *Executor) cancelPendingJobs() {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	canceledCount := 0
	for {
		select {
		case job := <-e.queue:
			// 标记为 canceled
			job.Status = JobStatusCanceled
			job.Error = "executor stopped before execution"
			now := time.Now()
			job.FinishedAt = &now
			e.history[job.ID] = job
			canceledCount++
		default:
			// 队列已空
			if canceledCount > 0 {
				e.log.WithField("count", canceledCount).Info("已取消队列中未执行的任务")
			}
			return
		}
	}
}

