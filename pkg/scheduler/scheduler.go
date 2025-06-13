package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xsxdot/aio/pkg/lock"
)

// Scheduler 分布式任务调度器
type Scheduler struct {
	// 配置
	nodeID        string
	lockManager   lock.LockManager
	lockKey       string
	lockTTL       time.Duration
	checkInterval time.Duration
	maxWorkers    int

	// 运行时状态
	isRunning atomic.Bool
	isLeader  atomic.Bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// 任务管理
	taskHeap        *TaskHeap
	distributedLock lock.DistributedLock

	// 工作者池
	workerSemaphore chan struct{}

	// 定时器
	timer   *time.Timer
	timerMu sync.Mutex

	// 日志
	logger *logrus.Logger

	// 统计信息
	stats *SchedulerStats
}

// SchedulerStats 调度器统计信息
type SchedulerStats struct {
	mu               sync.RWMutex
	TotalTasks       int64     `json:"total_tasks"`
	CompletedTasks   int64     `json:"completed_tasks"`
	FailedTasks      int64     `json:"failed_tasks"`
	DistributedTasks int64     `json:"distributed_tasks"`
	LocalTasks       int64     `json:"local_tasks"`
	LeaderElections  int64     `json:"leader_elections"`
	LastExecuteTime  time.Time `json:"last_execute_time"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	NodeID        string        `json:"node_id"`
	LockKey       string        `json:"lock_key"`
	LockTTL       time.Duration `json:"lock_ttl"`
	CheckInterval time.Duration `json:"check_interval"`
	MaxWorkers    int           `json:"max_workers"`
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		NodeID:        fmt.Sprintf("scheduler-%d", time.Now().UnixNano()),
		LockKey:       "aio/scheduler/leader",
		LockTTL:       30 * time.Second,
		CheckInterval: 1 * time.Second,
		MaxWorkers:    10,
	}
}

// NewScheduler 创建新的调度器
func NewScheduler(lockManager lock.LockManager, config *SchedulerConfig) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		nodeID:          config.NodeID,
		lockManager:     lockManager,
		lockKey:         config.LockKey,
		lockTTL:         config.LockTTL,
		checkInterval:   config.CheckInterval,
		maxWorkers:      config.MaxWorkers,
		ctx:             ctx,
		cancel:          cancel,
		taskHeap:        NewTaskHeap(),
		workerSemaphore: make(chan struct{}, config.MaxWorkers),
		logger:          logrus.New(),
		stats:           &SchedulerStats{},
	}

	// 创建分布式锁
	lockOpts := &lock.LockOptions{
		TTL:           config.LockTTL,
		AutoRenew:     true,
		RenewInterval: config.LockTTL / 3,
		RetryInterval: 1 * time.Second,
		MaxRetries:    0,
	}
	s.distributedLock = lockManager.NewLock(config.LockKey, lockOpts)

	return s
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	if s.isRunning.Load() {
		return fmt.Errorf("调度器已经在运行")
	}

	s.logger.Infof("启动调度器，节点ID: %s", s.nodeID)
	s.isRunning.Store(true)

	// 启动主循环
	s.wg.Add(1)
	go s.mainLoop()

	// 如果有任务需要执行，立即设置定时器
	s.resetTimer()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	if !s.isRunning.Load() {
		return nil
	}

	s.logger.Info("停止调度器")
	s.isRunning.Store(false)
	s.cancel()

	// 释放分布式锁
	if s.distributedLock != nil && s.distributedLock.IsLocked() {
		if err := s.distributedLock.Unlock(context.Background()); err != nil {
			s.logger.Errorf("释放分布式锁失败: %v", err)
		}
	}

	// 停止定时器
	s.stopTimer()

	// 等待所有goroutine完成
	s.wg.Wait()

	s.logger.Info("调度器已停止")
	return nil
}

// AddTask 添加任务
func (s *Scheduler) AddTask(task Task) error {
	if !s.isRunning.Load() {
		return fmt.Errorf("调度器未运行")
	}

	s.taskHeap.SafePush(task)
	s.stats.IncrementTotalTasks()

	s.logger.Infof("添加任务: %s [%s]", task.GetName(), task.GetID())

	// 重新设置定时器
	s.resetTimer()

	return nil
}

// RemoveTask 移除任务
func (s *Scheduler) RemoveTask(taskID string) bool {
	removed := s.taskHeap.SafeRemove(taskID)
	if removed {
		s.logger.Infof("移除任务: %s", taskID)
		s.resetTimer()
	}
	return removed
}

// GetTask 获取任务信息
func (s *Scheduler) GetTask(taskID string) Task {
	tasks := s.taskHeap.SafeList()
	for _, task := range tasks {
		if task.GetID() == taskID {
			return task
		}
	}
	return nil
}

// ListTasks 列出所有任务
func (s *Scheduler) ListTasks() []Task {
	return s.taskHeap.SafeList()
}

// GetStats 获取统计信息
func (s *Scheduler) GetStats() *SchedulerStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	// 创建副本返回
	return &SchedulerStats{
		TotalTasks:       s.stats.TotalTasks,
		CompletedTasks:   s.stats.CompletedTasks,
		FailedTasks:      s.stats.FailedTasks,
		DistributedTasks: s.stats.DistributedTasks,
		LocalTasks:       s.stats.LocalTasks,
		LeaderElections:  s.stats.LeaderElections,
		LastExecuteTime:  s.stats.LastExecuteTime,
	}
}

// IsLeader 检查是否为领导者
func (s *Scheduler) IsLeader() bool {
	return s.isLeader.Load()
}

// hasLocalTasks 检查是否有本地任务
func (s *Scheduler) hasLocalTasks() bool {
	tasks := s.taskHeap.SafeList()
	for _, task := range tasks {
		if task.GetExecuteMode() == TaskExecuteModeLocal && !task.IsCompleted() {
			return true
		}
	}
	return false
}

// mainLoop 主循环
func (s *Scheduler) mainLoop() {
	defer s.wg.Done()

	leaderCheckTicker := time.NewTicker(s.checkInterval)
	defer leaderCheckTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-leaderCheckTicker.C:
			s.tryBecomeLeader()
		}
	}
}

// tryBecomeLeader 尝试成为领导者
func (s *Scheduler) tryBecomeLeader() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	// 尝试获取分布式锁
	locked, err := s.distributedLock.TryLock(ctx)
	if err != nil {
		s.logger.Errorf("尝试获取分布式锁失败: %v", err)
		s.becomeFollower()
		return
	}

	if locked {
		if !s.isLeader.Load() {
			s.logger.Info("成为领导者")
			s.isLeader.Store(true)
			s.stats.IncrementLeaderElections()
			s.resetTimer()
		}
	} else {
		s.becomeFollower()
		// 即使不是领导者，如果有本地任务也需要启动定时器
		if s.hasLocalTasks() {
			s.resetTimer()
		}
	}
}

// becomeFollower 成为跟随者
func (s *Scheduler) becomeFollower() {
	if s.isLeader.Load() {
		s.logger.Info("失去领导者身份")
		s.isLeader.Store(false)
		// 检查是否还有本地任务需要执行，如果有则不停止定时器
		if !s.hasLocalTasks() {
			s.stopTimer()
		}
	}
}

// resetTimer 重置定时器
func (s *Scheduler) resetTimer() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	// 停止现有定时器
	if s.timer != nil {
		s.timer.Stop()
	}

	// 获取下次执行时间
	nextTime := s.taskHeap.GetNextExecuteTime()
	if nextTime == nil {
		return
	}

	// 计算等待时间
	waitDuration := time.Until(*nextTime)
	if waitDuration < 0 {
		waitDuration = 0
	}

	// 创建新定时器
	s.timer = time.AfterFunc(waitDuration, s.onTimerFired)
}

// stopTimer 停止定时器
func (s *Scheduler) stopTimer() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

// onTimerFired 定时器触发
func (s *Scheduler) onTimerFired() {
	if !s.isRunning.Load() {
		return
	}

	now := time.Now()
	readyTasks := s.taskHeap.PopReadyTasks(now)

	// 如果没有就绪任务，直接重置定时器
	if len(readyTasks) == 0 {
		s.resetTimer()
		return
	}

	// 执行就绪的任务
	for _, task := range readyTasks {
		s.executeTask(task)
	}

	// 注意：不在此处立即重置定时器，而是在任务执行完成后由 runTask 触发重置
}

// executeTask 执行任务
func (s *Scheduler) executeTask(task Task) {
	// 检查任务执行模式
	shouldExecute := false
	if task.GetExecuteMode() == TaskExecuteModeDistributed {
		// 分布式任务需要领导者身份
		if s.isLeader.Load() {
			shouldExecute = true
			s.stats.IncrementDistributedTasks()
		}
	} else {
		// 本地任务总是执行
		shouldExecute = true
		s.stats.IncrementLocalTasks()
	}

	if !shouldExecute {
		// 如果不应该执行，重新加入堆（等待下次调度）
		nextTime := task.UpdateNextTime(time.Now())
		if !task.IsCompleted() && !nextTime.IsZero() {
			// 重置任务状态为等待，以便下次执行
			task.SetStatus(TaskStatusWaiting)
			s.taskHeap.SafePush(task)
			// 任务重新加入堆后，重置定时器
			s.resetTimer()
		}
		return
	}

	// 获取工作者资源
	select {
	case s.workerSemaphore <- struct{}{}:
		// 异步执行任务
		s.wg.Add(1)
		go func(t Task) {
			defer s.wg.Done()
			defer func() { <-s.workerSemaphore }()

			s.runTask(t)
		}(task)
	default:
		// 工作者池满，重新调度
		s.logger.Warnf("工作者池已满，任务重新调度: %s", task.GetID())
		nextTime := task.UpdateNextTime(time.Now().Add(1 * time.Second))
		if !task.IsCompleted() && !nextTime.IsZero() {
			// 重置任务状态为等待，以便下次执行
			task.SetStatus(TaskStatusWaiting)
			s.taskHeap.SafePush(task)
			// 任务重新加入堆后，重置定时器
			s.resetTimer()
		}
	}
}

// runTask 运行任务
func (s *Scheduler) runTask(task Task) {
	start := time.Now()
	s.logger.Infof("开始执行任务: %s [%s]", task.GetName(), task.GetID())

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(s.ctx, task.GetTimeout())
	defer cancel()

	// 执行任务
	err := task.Execute(ctx)

	duration := time.Since(start)
	s.stats.SetLastExecuteTime(start)

	if err != nil {
		s.logger.Errorf("任务执行失败: %s [%s], 耗时: %v, 错误: %v",
			task.GetName(), task.GetID(), duration, err)
		s.stats.IncrementFailedTasks()
	} else {
		s.logger.Infof("任务执行成功: %s [%s], 耗时: %v",
			task.GetName(), task.GetID(), duration)
		s.stats.IncrementCompletedTasks()
	}

	// 更新下次执行时间并重新加入堆
	if !task.IsCompleted() {
		nextTime := task.UpdateNextTime(time.Now())
		if !nextTime.IsZero() {
			// 重置任务状态为等待，以便下次执行
			task.SetStatus(TaskStatusWaiting)
			s.taskHeap.SafePush(task)
			// 任务重新加入堆后，重置定时器以便调度下一个任务
			s.resetTimer()
		}
	} else {
		// 任务已完成，重置定时器以便调度其他任务
		s.resetTimer()
	}
}

// 统计方法
func (s *SchedulerStats) IncrementTotalTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalTasks++
}

func (s *SchedulerStats) IncrementCompletedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CompletedTasks++
}

func (s *SchedulerStats) IncrementFailedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailedTasks++
}

func (s *SchedulerStats) IncrementDistributedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DistributedTasks++
}

func (s *SchedulerStats) IncrementLocalTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LocalTasks++
}

func (s *SchedulerStats) IncrementLeaderElections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LeaderElections++
}

func (s *SchedulerStats) SetLastExecuteTime(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastExecuteTime = t
}
