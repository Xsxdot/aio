package sdk

import (
	"container/heap"
	"context"
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// SchedulerServiceOptions 定时任务服务选项
type SchedulerServiceOptions struct {
	// 服务名称，默认为 "scheduler-service"
	ServiceName string
	// 选举TTL（生存时间），用于领导选举
	ElectionTTL int
	// 是否开启分布式协调（启用领导选举）
	EnableDistributed bool
	// 任务执行超时，默认为1分钟
	TaskTimeout time.Duration
	// 任务重试次数，默认为3次
	TaskRetryCount int
	// 任务重试间隔，默认为5秒
	TaskRetryInterval time.Duration
}

// 默认的定时任务服务选项
var DefaultSchedulerServiceOptions = &SchedulerServiceOptions{
	ServiceName:       "scheduler-service",
	ElectionTTL:       10,
	EnableDistributed: true,
	TaskTimeout:       1 * time.Minute,
	TaskRetryCount:    3,
	TaskRetryInterval: 5 * time.Second,
}

// TaskStatus 任务状态
type TaskStatus string

const (
	// TaskStatusPending 等待执行
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 正在执行
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusSuccess 执行成功
	TaskStatusSuccess TaskStatus = "success"
	// TaskStatusFailed 执行失败
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskType 任务类型
type TaskType string

const (
	// TaskTypeCron 基于Cron表达式的定时任务
	TaskTypeCron TaskType = "cron"
	// TaskTypeDelay 延时任务
	TaskTypeDelay TaskType = "delay"
	// TaskTypeInterval 固定周期任务
	TaskTypeInterval TaskType = "interval"
)

// TaskFunc 任务执行函数
type TaskFunc func(ctx context.Context, params map[string]interface{}) error

// Task 任务定义
type Task struct {
	// 任务ID
	ID string
	// 任务名称
	Name string
	// 任务类型
	Type TaskType
	// 任务执行函数
	Handler TaskFunc
	// 任务参数
	Params map[string]interface{}
	// 对于Cron任务，指定Cron表达式
	CronExpression string
	// 对于延时任务，指定延时时间
	Delay time.Duration
	// 对于固定周期任务，指定执行间隔
	Interval time.Duration
	// 任务创建时间
	CreatedAt time.Time
	// 下一次执行时间
	NextRunAt time.Time
	// 最后一次执行时间
	LastRunAt time.Time
	// 任务状态
	Status TaskStatus
	// 最大重试次数
	MaxRetries int
	// 当前重试次数
	CurrentRetry int
	// 错误信息
	LastError error
	// 是否只由leader执行（需要启用分布式协调）
	LeaderOnly bool
	// 记录在任务队列中的索引
	heapIndex int
}

// TaskHeap 任务优先队列实现
type TaskHeap []*Task

func (h TaskHeap) Len() int { return len(h) }

func (h TaskHeap) Less(i, j int) bool {
	// 按照下一次执行时间排序
	return h[i].NextRunAt.Before(h[j].NextRunAt)
}

func (h TaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *TaskHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*Task)
	item.heapIndex = n
	*h = append(*h, item)
}

func (h *TaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.heapIndex = -1
	*h = old[0 : n-1]
	return item
}

// SchedulerService 定时任务服务
type SchedulerService struct {
	// 客户端引用
	client *Client
	// 选项
	options *SchedulerServiceOptions
	// 日志记录器
	logger *zap.Logger
	// Cron调度器
	cronScheduler *cron.Cron
	// 任务互斥锁
	taskLock sync.RWMutex
	// 任务映射 (taskID -> Task)
	tasks map[string]*Task
	// 任务执行状态
	taskStatus map[string]TaskStatus
	// 选举实例（用于分布式协调）
	election election.Election
	// 是否是leader
	isLeader bool
	// leader状态互斥锁
	leaderLock sync.RWMutex
	// 上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc

	// 任务优先队列
	taskHeap TaskHeap
	// 优先队列互斥锁
	heapLock sync.Mutex
	// 单一计时器
	timer *time.Timer
	// 定时器通知通道
	timerChan chan struct{}
}

// NewSchedulerService 创建定时任务服务
func NewSchedulerService(client *Client, options *SchedulerServiceOptions) *SchedulerService {
	if options == nil {
		options = DefaultSchedulerServiceOptions
	}

	// 如果未指定服务名称，使用默认值
	if client.serviceInfo != nil {
		options.ServiceName = client.serviceInfo.Name
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建Cron调度器
	cronOptions := cron.WithSeconds()
	cronScheduler := cron.New(cronOptions)

	service := &SchedulerService{
		client:        client,
		options:       options,
		logger:        common.GetLogger().GetZapLogger("SchedulerService-" + options.ServiceName), // 使用示例日志记录器，实际可从外部传入
		cronScheduler: cronScheduler,
		tasks:         make(map[string]*Task),
		taskStatus:    make(map[string]TaskStatus),
		isLeader:      false,
		ctx:           ctx,
		cancel:        cancel,
		taskHeap:      make(TaskHeap, 0),
		timerChan:     make(chan struct{}, 1),
	}

	// 初始化任务堆
	heap.Init(&service.taskHeap)

	return service
}

// Start 启动定时任务服务
func (s *SchedulerService) Start(ctx context.Context) error {
	// 启动Cron调度器
	s.cronScheduler.Start()

	// 启动任务处理协程
	go s.processTasksLoop()

	// 如果启用分布式协调，则创建选举实例
	if s.options.EnableDistributed {
		if err := s.setupElection(ctx); err != nil {
			return fmt.Errorf("设置选举实例失败: %w", err)
		}
	} else {
		// 如果不启用分布式协调，则默认为leader
		s.setLeader(true)
	}

	s.logger.Info("定时任务服务已启动",
		zap.String("service", s.options.ServiceName),
		zap.Bool("distributed", s.options.EnableDistributed))

	return nil
}

// setupElection 设置选举实例
func (s *SchedulerService) setupElection(ctx context.Context) error {
	// 创建选举实例
	electionOptions := []election.ElectionOption{
		election.WithElectionTTL(s.options.ElectionTTL),
	}

	electionName := fmt.Sprintf("%s-election", s.options.ServiceName)
	electionInstance, err := s.client.CreateElection(ctx, electionName, electionOptions...)
	if err != nil {
		return fmt.Errorf("创建选举实例失败: %w", err)
	}

	s.election = electionInstance

	// 使用选举事件处理函数
	err = electionInstance.Campaign(ctx, func(event election.ElectionEvent) {
		switch event.Type {
		case election.EventBecomeLeader:
			s.logger.Info("当前节点被选为leader",
				zap.String("service", s.options.ServiceName),
				zap.String("leaderID", event.Leader))
			s.setLeader(true)
		case election.EventBecomeFollower:
			s.logger.Info("当前节点成为follower",
				zap.String("service", s.options.ServiceName),
				zap.String("leaderID", event.Leader))
			s.setLeader(false)
		case election.EventLeaderChanged:
			s.logger.Info("leader已变更",
				zap.String("service", s.options.ServiceName),
				zap.String("leaderID", event.Leader))

			// 检查当前节点是否是leader
			if event.Leader == electionInstance.GetInfo().NodeID {
				s.setLeader(true)
			} else {
				s.setLeader(false)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("启动选举失败: %w", err)
	}

	return nil
}

// setLeader 设置leader状态
func (s *SchedulerService) setLeader(isLeader bool) {
	s.leaderLock.Lock()
	defer s.leaderLock.Unlock()
	s.isLeader = isLeader
}

// IsLeader 返回当前节点是否是leader
func (s *SchedulerService) IsLeader() bool {
	s.leaderLock.RLock()
	defer s.leaderLock.RUnlock()
	return s.isLeader
}

// AddCronTask 添加基于Cron表达式的定时任务
func (s *SchedulerService) AddCronTask(name, cronExpr string, handler TaskFunc, params map[string]interface{}, leaderOnly bool) (string, error) {
	taskID := fmt.Sprintf("cron-%s-%d", name, time.Now().UnixNano())

	task := &Task{
		ID:             taskID,
		Name:           name,
		Type:           TaskTypeCron,
		Handler:        handler,
		Params:         params,
		CronExpression: cronExpr,
		CreatedAt:      time.Now(),
		Status:         TaskStatusPending,
		MaxRetries:     s.options.TaskRetryCount,
		LeaderOnly:     leaderOnly,
	}

	// 注册Cron任务
	cronID, err := s.cronScheduler.AddFunc(cronExpr, func() {
		s.executeTask(task)
	})

	if err != nil {
		return "", fmt.Errorf("添加Cron任务失败: %w", err)
	}

	// 保存任务
	s.taskLock.Lock()
	task.ID = fmt.Sprintf("%s-%d", taskID, cronID)
	s.tasks[task.ID] = task
	s.taskStatus[task.ID] = TaskStatusPending
	s.taskLock.Unlock()

	s.logger.Info("添加Cron任务成功",
		zap.String("taskID", task.ID),
		zap.String("name", name),
		zap.String("cronExpr", cronExpr),
		zap.Bool("leaderOnly", leaderOnly))

	return task.ID, nil
}

// AddDelayTask 添加延时任务
func (s *SchedulerService) AddDelayTask(name string, delay time.Duration, handler TaskFunc, params map[string]interface{}, leaderOnly bool) (string, error) {
	taskID := fmt.Sprintf("delay-%s-%d", name, time.Now().UnixNano())

	task := &Task{
		ID:         taskID,
		Name:       name,
		Type:       TaskTypeDelay,
		Handler:    handler,
		Params:     params,
		Delay:      delay,
		CreatedAt:  time.Now(),
		NextRunAt:  time.Now().Add(delay),
		Status:     TaskStatusPending,
		MaxRetries: s.options.TaskRetryCount,
		LeaderOnly: leaderOnly,
	}

	// 保存任务并添加到优先队列
	s.taskLock.Lock()
	s.tasks[taskID] = task
	s.taskStatus[taskID] = TaskStatusPending
	s.taskLock.Unlock()

	// 添加到任务优先队列
	s.scheduleTask(task)

	s.logger.Info("添加延时任务成功",
		zap.String("taskID", taskID),
		zap.String("name", name),
		zap.Duration("delay", delay),
		zap.Time("nextRunAt", task.NextRunAt),
		zap.Bool("leaderOnly", leaderOnly))

	return taskID, nil
}

// AddIntervalTask 添加固定周期任务
func (s *SchedulerService) AddIntervalTask(name string, interval time.Duration, immediate bool, handler TaskFunc, params map[string]interface{}, leaderOnly bool) (string, error) {
	taskID := fmt.Sprintf("interval-%s-%d", name, time.Now().UnixNano())

	firstRunDelay := interval
	if immediate {
		firstRunDelay = 0
	}

	task := &Task{
		ID:         taskID,
		Name:       name,
		Type:       TaskTypeInterval,
		Handler:    handler,
		Params:     params,
		Interval:   interval,
		CreatedAt:  time.Now(),
		NextRunAt:  time.Now().Add(firstRunDelay),
		Status:     TaskStatusPending,
		MaxRetries: s.options.TaskRetryCount,
		LeaderOnly: leaderOnly,
	}

	// 保存任务
	s.taskLock.Lock()
	s.tasks[taskID] = task
	s.taskStatus[taskID] = TaskStatusPending
	s.taskLock.Unlock()

	// 添加到任务优先队列
	s.scheduleTask(task)

	s.logger.Info("添加固定周期任务成功",
		zap.String("taskID", taskID),
		zap.String("name", name),
		zap.Duration("interval", interval),
		zap.Bool("immediate", immediate),
		zap.Time("nextRunAt", task.NextRunAt),
		zap.Bool("leaderOnly", leaderOnly))

	return taskID, nil
}

// scheduleTask 将任务添加到优先队列并重置计时器
func (s *SchedulerService) scheduleTask(task *Task) {
	s.heapLock.Lock()
	defer s.heapLock.Unlock()

	// 添加到任务优先队列
	heap.Push(&s.taskHeap, task)

	// 如果是队列中第一个任务或任务应该最先执行，则重置计时器
	if len(s.taskHeap) == 1 || task.NextRunAt.Before(s.taskHeap[0].NextRunAt) {
		s.resetTimer()
	}
}

// resetTimer 重置计时器到下一个任务的执行时间
func (s *SchedulerService) resetTimer() {
	// 如果队列为空，直接返回
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
		// 如果任务应该立即执行
		duration = 0
	}

	// 停止现有计时器
	if s.timer != nil {
		s.timer.Stop()
	}

	// 创建新计时器
	s.timer = time.AfterFunc(duration, func() {
		// 当计时器触发时，向处理循环发送信号
		select {
		case s.timerChan <- struct{}{}:
		default:
			// 通道已满，不阻塞
		}
	})
}

// processTasksLoop 处理任务队列的主循环
func (s *SchedulerService) processTasksLoop() {
	for {
		select {
		case <-s.ctx.Done():
			// 服务已停止
			return
		case <-s.timerChan:
			// 计时器触发，执行到期任务
			s.processTasksDue()
		}
	}
}

// processTasksDue 处理所有到期的任务
func (s *SchedulerService) processTasksDue() {
	now := time.Now()
	tasksToExecute := make([]*Task, 0)

	// 获取所有到期任务
	s.heapLock.Lock()
	for len(s.taskHeap) > 0 && !s.taskHeap[0].NextRunAt.After(now) {
		task := heap.Pop(&s.taskHeap).(*Task)

		// 检查任务是否存在且未被取消
		s.taskLock.RLock()
		_, exists := s.tasks[task.ID]
		status := s.taskStatus[task.ID]
		s.taskLock.RUnlock()

		if exists && status != TaskStatusCancelled {
			tasksToExecute = append(tasksToExecute, task)
		}
	}

	// 重置计时器到下一个任务的执行时间
	s.resetTimer()
	s.heapLock.Unlock()

	// 执行到期任务
	for _, task := range tasksToExecute {
		go s.executeTask(task)
	}
}

// executeTask 执行任务
func (s *SchedulerService) executeTask(task *Task) {
	// 如果任务指定只由leader执行，且当前节点不是leader，则跳过
	if task.LeaderOnly && !s.IsLeader() {
		s.logger.Debug("跳过任务执行，当前节点不是leader",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name))

		// 对于周期任务，重新调度下一次执行
		if task.Type == TaskTypeInterval {
			s.rescheduleIntervalTask(task)
		}
		return
	}

	// 创建任务上下文
	taskCtx, cancel := context.WithTimeout(s.ctx, s.options.TaskTimeout)
	defer cancel()

	// 更新任务状态为运行中
	s.taskLock.Lock()
	if _, exists := s.tasks[task.ID]; exists {
		s.tasks[task.ID].Status = TaskStatusRunning
		s.tasks[task.ID].LastRunAt = time.Now()
	}
	s.taskStatus[task.ID] = TaskStatusRunning
	s.taskLock.Unlock()

	s.logger.Debug("开始执行任务",
		zap.String("taskID", task.ID),
		zap.String("name", task.Name),
		zap.String("type", string(task.Type)))

	// 执行任务
	err := task.Handler(taskCtx, task.Params)

	// 更新任务状态
	s.taskLock.Lock()
	if _, exists := s.tasks[task.ID]; !exists {
		// 任务可能已被删除
		s.taskLock.Unlock()
		return
	}

	if err != nil {
		// 任务执行失败
		s.tasks[task.ID].Status = TaskStatusFailed
		s.tasks[task.ID].LastError = err
		s.taskStatus[task.ID] = TaskStatusFailed

		s.logger.Error("任务执行失败",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name),
			zap.Error(err))

		// 检查是否需要重试
		if s.tasks[task.ID].CurrentRetry < s.tasks[task.ID].MaxRetries {
			s.tasks[task.ID].CurrentRetry++
			retryDelay := time.Duration(s.tasks[task.ID].CurrentRetry) * s.options.TaskRetryInterval
			s.tasks[task.ID].NextRunAt = time.Now().Add(retryDelay)

			s.logger.Info("任务将重试",
				zap.String("taskID", task.ID),
				zap.String("name", task.Name),
				zap.Int("retryCount", s.tasks[task.ID].CurrentRetry),
				zap.Duration("retryDelay", retryDelay))

			taskCopy := s.tasks[task.ID]
			s.taskLock.Unlock()

			// 重新调度任务
			s.scheduleTask(taskCopy)
		} else {
			s.taskLock.Unlock()

			// 如果是周期任务，则重新调度
			if task.Type == TaskTypeInterval {
				s.rescheduleIntervalTask(task)
			}
		}
	} else {
		// 任务执行成功
		s.tasks[task.ID].Status = TaskStatusSuccess
		s.tasks[task.ID].CurrentRetry = 0
		s.tasks[task.ID].LastError = nil
		s.taskStatus[task.ID] = TaskStatusSuccess

		s.logger.Debug("任务执行成功",
			zap.String("taskID", task.ID),
			zap.String("name", task.Name))

		s.taskLock.Unlock()

		// 如果是周期任务，则重新调度
		if task.Type == TaskTypeInterval {
			s.rescheduleIntervalTask(task)
		}
	}
}

// rescheduleIntervalTask 重新调度周期任务
func (s *SchedulerService) rescheduleIntervalTask(task *Task) {
	s.taskLock.Lock()
	t, exists := s.tasks[task.ID]
	if !exists || s.taskStatus[task.ID] == TaskStatusCancelled {
		s.taskLock.Unlock()
		return
	}

	// 计算下一次执行时间
	t.NextRunAt = time.Now().Add(t.Interval)
	taskCopy := t
	s.taskLock.Unlock()

	// 重新调度任务
	s.scheduleTask(taskCopy)
}

// CancelTask 取消任务
func (s *SchedulerService) CancelTask(taskID string) error {
	s.taskLock.Lock()

	task, exists := s.tasks[taskID]
	if !exists {
		s.taskLock.Unlock()
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	// 根据任务类型取消
	if task.Type == TaskTypeCron {
		// 从Cron调度器中移除
		entryIDStr := ""
		var entryID cron.EntryID
		if _, err := fmt.Sscanf(taskID, "cron-%s-%d", &entryIDStr, &entryID); err == nil {
			s.cronScheduler.Remove(entryID)
		}
	}

	// 更新任务状态
	task.Status = TaskStatusCancelled
	s.taskStatus[taskID] = TaskStatusCancelled
	s.taskLock.Unlock()

	s.logger.Info("任务已取消",
		zap.String("taskID", taskID),
		zap.String("name", task.Name))

	return nil
}

// GetTask 获取任务信息
func (s *SchedulerService) GetTask(taskID string) (*Task, error) {
	s.taskLock.RLock()
	defer s.taskLock.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	return task, nil
}

// GetAllTasks 获取所有任务
func (s *SchedulerService) GetAllTasks() []*Task {
	s.taskLock.RLock()
	defer s.taskLock.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// Stop 停止定时任务服务
func (s *SchedulerService) Stop() error {
	// 停止上下文
	s.cancel()

	// 停止Cron调度器
	s.cronScheduler.Stop()

	// 停止计时器
	s.heapLock.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.heapLock.Unlock()

	// 如果存在选举实例，停止选举
	if s.election != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.election.Resign(ctx); err != nil {
			s.logger.Warn("停止选举失败", zap.Error(err))
		}
	}

	s.logger.Info("定时任务服务已停止",
		zap.String("service", s.options.ServiceName))

	return nil
}
