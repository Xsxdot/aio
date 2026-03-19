package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// Scheduler 分布式任务调度器
type Scheduler struct {
	// 配置
	nodeID        string
	lockKey       string
	lockTTL       time.Duration
	checkInterval time.Duration
	maxWorkers    int

	// 运行时状态
	isRunning atomic.Bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// 任务管理
	taskHeap *TaskHeap

	// 工作者池
	workerSemaphore chan struct{}

	// 定时器
	timer   *time.Timer
	timerMu sync.Mutex

	// 日志
	logger *logrus.Logger

	// 统计信息
	stats *SchedulerStats

	// Redis 分布式锁（可选）
	rdb    *redis.Client
	locker *redislock.Client
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
	NodeID            string        `json:"node_id"`
	LockKey           string        `json:"lock_key"`
	LockTTL           time.Duration `json:"lock_ttl"`
	LockRetryInterval time.Duration `json:"lock_retry_interval"`
	MaxWorkers        int           `json:"max_workers"`
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		NodeID:            fmt.Sprintf("scheduler-%d", time.Now().UnixNano()),
		LockKey:           "aio/scheduler/leader",
		LockTTL:           30 * time.Second,
		LockRetryInterval: 5 * time.Second,
		MaxWorkers:        30,
	}
}

// NewScheduler 创建新的调度器（不支持分布式任务）
func NewScheduler(config *SchedulerConfig) *Scheduler {
	return newSchedulerInternal(config, nil)
}

// NewSchedulerWithRedis 创建支持分布式任务的调度器
func NewSchedulerWithRedis(config *SchedulerConfig, rdb *redis.Client) *Scheduler {
	return newSchedulerInternal(config, rdb)
}

// newSchedulerInternal 内部构造函数
func newSchedulerInternal(config *SchedulerConfig, rdb *redis.Client) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	var locker *redislock.Client
	if rdb != nil {
		locker = redislock.New(rdb)
	}

	s := &Scheduler{
		nodeID:          config.NodeID,
		lockKey:         config.LockKey,
		lockTTL:         config.LockTTL,
		checkInterval:   config.LockRetryInterval,
		maxWorkers:      config.MaxWorkers,
		ctx:             ctx,
		cancel:          cancel,
		taskHeap:        NewTaskHeap(),
		workerSemaphore: make(chan struct{}, config.MaxWorkers),
		logger:          logrus.New(),
		stats:           &SchedulerStats{},
		rdb:             rdb,
		locker:          locker,
	}

	return s
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	if s.isRunning.Load() {
		return fmt.Errorf("调度器已经在运行")
	}

	s.logger.Infof("启动调度器，节点ID: %s", s.nodeID)
	if s.locker != nil {
		s.logger.Info("Redis 分布式锁已启用")
	} else {
		s.logger.Warn("Redis 分布式锁未启用，分布式任务将不会执行")
	}
	s.isRunning.Store(true)

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

	// 根据任务类型选择合适的日志级别
	if task.GetType() == TaskTypeInterval {
		s.logger.Debugf("添加固定间隔任务: %s [%s]", task.GetName(), task.GetID())
	} else {
		s.logger.Infof("添加任务: %s [%s]", task.GetName(), task.GetID())
	}

	// 重新设置定时器
	s.resetTimer()

	return nil
}

// RemoveTask 移除任务
func (s *Scheduler) RemoveTask(taskID string) bool {
	removed := s.taskHeap.SafeRemove(taskID)
	if removed {
		s.logger.Debugf("移除任务: %s", taskID)
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

// IsLeader 检查是否为领导者（已废弃，按任务粒度加锁）
// Deprecated: 不再使用 leader 选举模式，改为按任务粒度加锁
func (s *Scheduler) IsLeader() bool {
	return false
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

	// 立即重置定时器，指向堆里下一个到期任务
	// 这样可以避免长耗时任务导致后续任务延迟触发
	s.resetTimer()
}

// executeTask 执行任务
func (s *Scheduler) executeTask(task Task) {
	// 检查任务执行模式
	if task.GetExecuteMode() == TaskExecuteModeDistributed {
		// 分布式任务需要 Redis 锁
		if s.locker == nil {
			// 稳定性优先：没有 locker 则不执行分布式任务
			s.logger.Warnf("分布式任务 %s [%s] 无法执行：Redis locker 未配置", task.GetName(), task.GetID())
			// 重新加入堆等待下次调度
			nextTime := task.UpdateNextTime(time.Now())
			if !task.IsCompleted() && !nextTime.IsZero() {
				task.SetStatus(TaskStatusWaiting)
				s.taskHeap.SafePush(task)
				s.resetTimer()
			}
			return
		}
		s.stats.IncrementDistributedTasks()
	} else {
		// 本地任务总是执行
		s.stats.IncrementLocalTasks()
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

	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("任务执行发生panic: %s [%s], panic: %v",
				task.GetName(), task.GetID(), r)

			// 将任务状态设置为已取消，防止再次执行
			task.SetStatus(TaskStatusCanceled)
			s.stats.IncrementFailedTasks()

			// 任务已取消，重置定时器以便调度其他任务
			s.resetTimer()
		}
	}()

	// 根据任务类型选择合适的日志级别
	isIntervalTask := task.GetType() == TaskTypeInterval

	// 如果是分布式任务，尝试获取锁
	var lock *redislock.Lock
	if task.GetExecuteMode() == TaskExecuteModeDistributed {
		var err error
		lock, err = s.obtainTaskLock(task)
		if err != nil {
			if err == redislock.ErrNotObtained {
				// 其他节点正在执行，本节点跳过
				if !isIntervalTask {
					s.logger.Infof("任务 %s [%s] 已被其他节点执行，跳过", task.GetName(), task.GetID())
				}
				// 对于一次性任务，标记为完成；对于周期任务，等待下次调度
				if task.GetType() == TaskTypeOnce {
					task.SetStatus(TaskStatusCompleted)
				} else {
					// 重新加入堆等待下次调度
					nextTime := task.UpdateNextTime(time.Now())
					if !task.IsCompleted() && !nextTime.IsZero() {
						task.SetStatus(TaskStatusWaiting)
						s.taskHeap.SafePush(task)
					}
				}
				s.resetTimer()
				return
			}
			// 其他错误（Redis 连接失败等）
			s.logger.Errorf("获取任务锁失败: %s [%s], 错误: %v", task.GetName(), task.GetID(), err)
			s.stats.IncrementFailedTasks()
			// 重新加入堆等待下次调度
			nextTime := task.UpdateNextTime(time.Now())
			if !task.IsCompleted() && !nextTime.IsZero() {
				task.SetStatus(TaskStatusWaiting)
				s.taskHeap.SafePush(task)
			}
			s.resetTimer()
			return
		}
		// 成功获取锁，确保执行完释放
		defer func() {
			if err := lock.Release(context.Background()); err != nil {
				s.logger.Warnf("释放任务锁失败: %s [%s], 错误: %v", task.GetName(), task.GetID(), err)
			}
		}()
	}

	if isIntervalTask {
		s.logger.Debugf("开始执行固定间隔任务: %s [%s]", task.GetName(), task.GetID())
	} else {
		s.logger.Infof("开始执行任务: %s [%s]", task.GetName(), task.GetID())
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(s.ctx, task.GetTimeout())
	defer cancel()

	// 如果是分布式任务且获得了锁，启动续租
	if lock != nil {
		refreshCtx, refreshCancel := context.WithCancel(ctx)
		defer refreshCancel()
		go s.refreshLock(refreshCtx, lock, task)
	}

	// 执行任务
	err := task.Execute(ctx)

	duration := time.Since(start)
	s.stats.SetLastExecuteTime(start)

	if err != nil {
		s.logger.Errorf("任务执行失败: %s [%s], 耗时: %v, 错误: %v",
			task.GetName(), task.GetID(), duration, err)
		s.stats.IncrementFailedTasks()
	} else {
		if isIntervalTask {
			s.logger.Debugf("固定间隔任务执行成功: %s [%s], 耗时: %v",
				task.GetName(), task.GetID(), duration)
		} else {
			s.logger.Infof("任务执行成功: %s [%s], 耗时: %v",
				task.GetName(), task.GetID(), duration)
		}
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

// obtainTaskLock 获取任务的分布式锁
func (s *Scheduler) obtainTaskLock(task Task) (*redislock.Lock, error) {
	lockKey := fmt.Sprintf("%s/%s", s.lockKey, task.GetKey())

	// 计算锁的 TTL：任务超时 + 10s grace period
	lockTTL := task.GetTimeout() + 10*time.Second
	if lockTTL < s.lockTTL {
		lockTTL = s.lockTTL
	}

	// 使用短超时上下文尝试获取锁（500ms，最多重试 3 次）
	obtainCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(100*time.Millisecond), 3),
	}

	lock, err := s.locker.Obtain(obtainCtx, lockKey, lockTTL, opts)
	return lock, err
}

// refreshLock 持续续租锁，直到任务完成或上下文取消
func (s *Scheduler) refreshLock(ctx context.Context, lock *redislock.Lock, task Task) {
	lockTTL := task.GetTimeout() + 10*time.Second
	if lockTTL < s.lockTTL {
		lockTTL = s.lockTTL
	}

	// 续租间隔为 TTL 的 1/3
	refreshInterval := lockTTL / 3
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 任务完成或取消
			return
		case <-ticker.C:
			// 续租
			if err := lock.Refresh(ctx, lockTTL, nil); err != nil {
				if err == redislock.ErrNotObtained {
					s.logger.Errorf("任务锁已丢失: %s [%s]，任务可能被其他节点接管", task.GetName(), task.GetID())
				} else {
					s.logger.Warnf("续租任务锁失败: %s [%s], 错误: %v", task.GetName(), task.GetID(), err)
				}
				return
			}
		}
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
