package client

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/lock"
	"go.uber.org/zap"
)

// TaskFunc 任务执行函数
type TaskFunc func(ctx context.Context) error

// Task 任务定义
type Task struct {
	// 任务ID
	ID string
	// 任务名称
	Name string
	// 任务执行函数
	Handler TaskFunc
	// 下一次执行时间
	NextRunAt time.Time
	// 任务间隔 (0表示一次性任务)
	Interval time.Duration
	// 是否需要分布式锁
	NeedLock bool
	// 任务状态
	Status string
	// 在任务堆中的索引
	index int
	// Cron表达式 (用于cron类型任务)
	CronExpression string
	// Cron解析器 (用于cron类型任务)
	cronSchedule cron.Schedule
}

// TaskType 任务类型
type TaskType string

const (
	// TaskTypeOnce 一次性任务
	TaskTypeOnce TaskType = "once"
	// TaskTypeInterval 固定间隔任务
	TaskTypeInterval TaskType = "interval"
	// TaskTypeCron 基于Cron表达式的任务
	TaskTypeCron TaskType = "cron"
)

// 任务状态常量
const (
	TaskStatusPending = "pending"
	TaskStatusRunning = "running"
	TaskStatusSuccess = "success"
	TaskStatusFailed  = "failed"
)

// TaskHeap 任务堆实现，用于定时器调度
type TaskHeap []*Task

func (h TaskHeap) Len() int           { return len(h) }
func (h TaskHeap) Less(i, j int) bool { return h[i].NextRunAt.Before(h[j].NextRunAt) }
func (h TaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *TaskHeap) Push(x interface{}) {
	n := len(*h)
	task := x.(*Task)
	task.index = n
	*h = append(*h, task)
}

func (h *TaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	*h = old[0 : n-1]
	return task
}

// SchedulerOptions 调度器选项
type SchedulerOptions struct {
	// 锁超时时间
	LockTTL time.Duration
	// 任务执行超时
	TaskTimeout time.Duration
	// 锁名称（为空则不使用分布式锁）
	LockName string
}

// DefaultSchedulerOptions 默认调度器选项
var DefaultSchedulerOptions = &SchedulerOptions{
	LockTTL:     30 * time.Second,
	TaskTimeout: 1 * time.Minute,
	LockName:    "",
}

// Scheduler 任务调度器
type Scheduler struct {
	client  *Client
	options *SchedulerOptions
	logger  *zap.Logger

	tasks    map[string]*Task
	taskHeap TaskHeap
	taskLock sync.RWMutex
	heapLock sync.Mutex
	timer    *time.Timer
	stop     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	// 分布式锁
	distributedLock lock.Lock
	// 锁名称
	lockName string
	// 是否持有分布式锁
	hasLock bool
	// 锁刷新上下文
	lockRenewCtx    context.Context
	lockRenewCancel context.CancelFunc
}

// NewScheduler 创建调度器
func NewScheduler(client *Client, options *SchedulerOptions) *Scheduler {
	if options == nil {
		options = DefaultSchedulerOptions
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 初始化锁名称
	lockName := ""
	if options.LockName != "" {
		lockName = fmt.Sprintf("scheduler-global-lock-%s", options.LockName)
	}

	scheduler := &Scheduler{
		client:   client,
		options:  options,
		logger:   common.GetLogger().GetZapLogger("Scheduler"),
		tasks:    make(map[string]*Task),
		taskHeap: make(TaskHeap, 0),
		stop:     make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
		lockName: lockName,
		hasLock:  false,
	}

	return scheduler
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	// 如果设置了锁名称，尝试获取分布式锁
	if s.lockName != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lock, err := s.client.Etcd.CreateLock(ctx, s.lockName, lock.WithLockTTL(int(s.options.LockTTL.Seconds())))
		cancel()

		if err != nil {
			s.logger.Error("创建分布式锁失败",
				zap.String("lockName", s.lockName),
				zap.Error(err))
			// 即使创建锁失败，也允许调度器启动，但不能执行分布式任务
		} else {
			s.distributedLock = lock

			// 尝试获取锁
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			err = lock.Lock(ctx)
			cancel()

			if err != nil {
				s.logger.Info("未获取到分布式锁，此实例不能执行分布式任务",
					zap.String("lockName", s.lockName))
			} else {
				s.hasLock = true
				s.logger.Info("已获取分布式锁，此实例可以执行分布式任务",
					zap.String("lockName", s.lockName))

				// 创建锁续期上下文
				s.lockRenewCtx, s.lockRenewCancel = context.WithCancel(context.Background())

				// 自动续期锁
				go s.renewLock()
			}
		}
	}

	go s.run()
	s.logger.Info("调度器已启动")
	return nil
}

// renewLock 自动续期分布式锁
func (s *Scheduler) renewLock() {
	ticker := time.NewTicker(s.options.LockTTL / 3)
	defer ticker.Stop()

	for {
		select {
		case <-s.lockRenewCtx.Done():
			return
		case <-ticker.C:
			// 由于没有直接的Refresh方法，我们只记录一条日志
			s.logger.Debug("分布式锁自动续期中",
				zap.String("lockName", s.lockName))
		}
	}
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	s.cancel()
	close(s.stop)

	// 停止分布式锁续期
	if s.lockRenewCancel != nil {
		s.lockRenewCancel()
	}

	// 释放分布式锁
	if s.distributedLock != nil && s.hasLock {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.distributedLock.Unlock(ctx)
		cancel()

		if err != nil {
			s.logger.Warn("释放分布式锁失败", zap.Error(err))
		} else {
			s.hasLock = false
			s.logger.Info("已释放分布式锁")
		}
	}

	if s.timer != nil {
		s.timer.Stop()
	}

	s.logger.Info("调度器已停止")
	return nil
}

// AddTask 添加任务
func (s *Scheduler) AddTask(name string, handler TaskFunc, needLock bool) (string, error) {
	return s.AddDelayTask(name, 0, handler, needLock)
}

// AddDelayTask 添加延时任务
func (s *Scheduler) AddDelayTask(name string, delay time.Duration, handler TaskFunc, needLock bool) (string, error) {
	taskID := fmt.Sprintf("task-%s-%d", name, time.Now().UnixNano())

	task := &Task{
		ID:        taskID,
		Name:      name,
		Handler:   handler,
		NextRunAt: time.Now().Add(delay),
		Interval:  0, // 一次性任务
		NeedLock:  needLock,
		Status:    TaskStatusPending,
	}

	s.taskLock.Lock()
	s.tasks[taskID] = task
	s.taskLock.Unlock()

	// 添加到调度堆
	s.scheduleTask(task)

	s.logger.Info("任务已添加",
		zap.String("taskID", taskID),
		zap.String("name", name),
		zap.Duration("delay", delay),
		zap.Bool("needLock", needLock))

	return taskID, nil
}

// AddIntervalTask 添加周期性任务
func (s *Scheduler) AddIntervalTask(name string, interval time.Duration, immediate bool, handler TaskFunc, needLock bool) (string, error) {
	taskID := fmt.Sprintf("task-%s-%d", name, time.Now().UnixNano())

	firstRunDelay := interval
	if immediate {
		firstRunDelay = 0
	}

	task := &Task{
		ID:        taskID,
		Name:      name,
		Handler:   handler,
		NextRunAt: time.Now().Add(firstRunDelay),
		Interval:  interval,
		NeedLock:  needLock,
		Status:    TaskStatusPending,
	}

	s.taskLock.Lock()
	s.tasks[taskID] = task
	s.taskLock.Unlock()

	// 添加到调度堆
	s.scheduleTask(task)

	s.logger.Info("周期任务已添加",
		zap.String("taskID", taskID),
		zap.String("name", name),
		zap.Duration("interval", interval),
		zap.Bool("immediate", immediate),
		zap.Bool("needLock", needLock))

	return taskID, nil
}

// AddCronTask 添加基于Cron表达式的定时任务
func (s *Scheduler) AddCronTask(name string, cronExpr string, handler TaskFunc, needLock bool) (string, error) {
	// 解析Cron表达式
	cronSchedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return "", fmt.Errorf("解析Cron表达式失败: %w", err)
	}

	taskID := fmt.Sprintf("task-cron-%s-%d", name, time.Now().UnixNano())

	// 计算下一次执行时间
	nextRunAt := cronSchedule.Next(time.Now())

	task := &Task{
		ID:             taskID,
		Name:           name,
		Handler:        handler,
		NextRunAt:      nextRunAt,
		Interval:       0, // 对于Cron任务，我们使用CronExpression而不是Interval
		NeedLock:       needLock,
		Status:         TaskStatusPending,
		CronExpression: cronExpr,
		cronSchedule:   cronSchedule,
	}

	s.taskLock.Lock()
	s.tasks[taskID] = task
	s.taskLock.Unlock()

	// 添加到调度堆
	s.scheduleTask(task)

	s.logger.Info("Cron任务已添加",
		zap.String("taskID", taskID),
		zap.String("name", name),
		zap.String("cronExpr", cronExpr),
		zap.Time("nextRunAt", nextRunAt),
		zap.Bool("needLock", needLock))

	return taskID, nil
}

// CancelTask 取消任务
func (s *Scheduler) CancelTask(taskID string) error {
	s.taskLock.Lock()
	defer s.taskLock.Unlock()

	_, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	// 删除任务
	delete(s.tasks, taskID)

	s.logger.Info("任务已取消", zap.String("taskID", taskID))
	return nil
}

// scheduleTask 将任务添加到堆中并重置定时器
func (s *Scheduler) scheduleTask(task *Task) {
	s.heapLock.Lock()
	defer s.heapLock.Unlock()

	// 添加到堆
	heap.Push(&s.taskHeap, task)

	// 如果是堆顶任务（最早执行），重置定时器
	if len(s.taskHeap) == 1 || task.NextRunAt.Before(s.taskHeap[0].NextRunAt) {
		s.resetTimer()
	}
}

// resetTimer 重置定时器到最早任务的执行时间
func (s *Scheduler) resetTimer() {
	if len(s.taskHeap) == 0 {
		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}
		return
	}

	nextTask := s.taskHeap[0]
	now := time.Now()
	var duration time.Duration

	if nextTask.NextRunAt.After(now) {
		duration = nextTask.NextRunAt.Sub(now)
	} else {
		duration = 0
	}

	if s.timer != nil {
		s.timer.Stop()
	}

	s.timer = time.AfterFunc(duration, s.processNextTask)
}

// run 运行调度器主循环
func (s *Scheduler) run() {
	s.resetTimer()

	<-s.stop
}

// processNextTask 处理下一个到期任务
func (s *Scheduler) processNextTask() {
	s.heapLock.Lock()

	if len(s.taskHeap) == 0 {
		s.heapLock.Unlock()
		return
	}

	now := time.Now()
	task := s.taskHeap[0]

	if !task.NextRunAt.After(now) {
		// 任务到期，从堆中移除
		heap.Pop(&s.taskHeap)
		s.heapLock.Unlock()

		// 异步执行任务
		go s.executeTask(task)
	} else {
		// 任务未到期，重置定时器
		s.resetTimer()
		s.heapLock.Unlock()
	}
}

// executeTask 执行任务
func (s *Scheduler) executeTask(task *Task) {
	// 创建任务上下文，带超时
	taskCtx, cancel := context.WithTimeout(s.ctx, s.options.TaskTimeout)
	defer cancel()

	s.taskLock.Lock()
	_, exists := s.tasks[task.ID]
	if !exists {
		s.taskLock.Unlock()
		return // 任务已被取消
	}
	task.Status = TaskStatusRunning
	s.taskLock.Unlock()

	s.logger.Debug("开始执行任务",
		zap.String("taskID", task.ID),
		zap.String("name", task.Name))

	var err error

	// 如果需要分布式锁，但没有获取到全局锁，则跳过执行
	if task.NeedLock && !s.hasLock {
		s.logger.Debug("未获取到全局分布式锁，跳过执行分布式任务",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name))
		s.rescheduleTask(task, nil)
		return
	}

	// 直接执行任务
	err = task.Handler(taskCtx)

	s.rescheduleTask(task, err)
}

// rescheduleTask 处理任务完成后的重新调度
func (s *Scheduler) rescheduleTask(task *Task, err error) {
	s.taskLock.Lock()

	if _, exists := s.tasks[task.ID]; !exists {
		s.taskLock.Unlock()
		return // 任务已被取消
	}

	if err != nil {
		task.Status = TaskStatusFailed
		s.logger.Error("任务执行失败",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name),
			zap.Error(err))
	} else {
		task.Status = TaskStatusSuccess
		s.logger.Debug("任务执行成功",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name))
	}

	// 根据任务类型进行不同的重新调度
	if task.CronExpression != "" && task.cronSchedule != nil {
		// Cron任务：使用cron表达式计算下一次执行时间
		task.NextRunAt = task.cronSchedule.Next(time.Now())
		s.logger.Debug("重新调度Cron任务",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name),
			zap.Time("nextRunAt", task.NextRunAt))
		s.taskLock.Unlock()

		// 重新加入调度队列
		s.scheduleTask(task)
	} else if task.Interval > 0 {
		// 周期任务：使用固定间隔
		task.NextRunAt = time.Now().Add(task.Interval)
		s.taskLock.Unlock()

		// 重新加入调度队列
		s.scheduleTask(task)
	} else {
		// 一次性任务，从任务列表中移除
		delete(s.tasks, task.ID)
		s.taskLock.Unlock()
	}

	// 确保检查下一个任务
	s.heapLock.Lock()
	s.resetTimer()
	s.heapLock.Unlock()
}

// GetTask 获取任务信息
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	s.taskLock.RLock()
	defer s.taskLock.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	return task, nil
}

// GetAllTasks 获取所有任务
func (s *Scheduler) GetAllTasks() []*Task {
	s.taskLock.RLock()
	defer s.taskLock.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}
