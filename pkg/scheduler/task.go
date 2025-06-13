package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// TaskType 任务类型
type TaskType int

const (
	// TaskTypeOnce 一次性任务
	TaskTypeOnce TaskType = iota
	// TaskTypeInterval 固定间隔任务
	TaskTypeInterval
	// TaskTypeCron 基于Cron表达式的任务
	TaskTypeCron
)

// TaskStatus 任务状态
type TaskStatus int

const (
	// TaskStatusWaiting 等待执行
	TaskStatusWaiting TaskStatus = iota
	// TaskStatusRunning 正在执行
	TaskStatusRunning
	// TaskStatusCompleted 已完成
	TaskStatusCompleted
	// TaskStatusFailed 执行失败
	TaskStatusFailed
	// TaskStatusCanceled 已取消
	TaskStatusCanceled
)

// TaskExecuteMode 任务执行模式
type TaskExecuteMode int

const (
	// TaskExecuteModeDistributed 分布式执行（需要获取锁）
	TaskExecuteModeDistributed TaskExecuteMode = iota
	// TaskExecuteModeLocal 本地执行
	TaskExecuteModeLocal
)

// TaskFunc 任务执行函数
type TaskFunc func(ctx context.Context) error

// Task 任务接口
type Task interface {
	// GetID 获取任务ID
	GetID() string

	// GetName 获取任务名称
	GetName() string

	// GetType 获取任务类型
	GetType() TaskType

	// GetExecuteMode 获取执行模式
	GetExecuteMode() TaskExecuteMode

	// GetNextTime 获取下次执行时间
	GetNextTime() time.Time

	// GetTimeout 获取任务超时时间
	GetTimeout() time.Duration

	// Execute 执行任务
	Execute(ctx context.Context) error

	// UpdateNextTime 更新下次执行时间
	UpdateNextTime(currentTime time.Time) time.Time

	// CanExecute 检查是否可以执行
	CanExecute(currentTime time.Time) bool

	// IsCompleted 检查任务是否已完成
	IsCompleted() bool

	// GetStatus 获取任务状态
	GetStatus() TaskStatus

	// SetStatus 设置任务状态
	SetStatus(status TaskStatus)
}

// BaseTask 基础任务实现
type BaseTask struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        TaskType        `json:"type"`
	ExecuteMode TaskExecuteMode `json:"execute_mode"`
	Status      TaskStatus      `json:"status"`
	NextTime    time.Time       `json:"next_time"`
	Timeout     time.Duration   `json:"timeout"`
	Func        TaskFunc        `json:"-"`
	CreateTime  time.Time       `json:"create_time"`
	UpdateTime  time.Time       `json:"update_time"`
}

// GetID 获取任务ID
func (t *BaseTask) GetID() string {
	return t.ID
}

// GetName 获取任务名称
func (t *BaseTask) GetName() string {
	return t.Name
}

// GetType 获取任务类型
func (t *BaseTask) GetType() TaskType {
	return t.Type
}

// GetExecuteMode 获取执行模式
func (t *BaseTask) GetExecuteMode() TaskExecuteMode {
	return t.ExecuteMode
}

// GetNextTime 获取下次执行时间
func (t *BaseTask) GetNextTime() time.Time {
	return t.NextTime
}

// GetTimeout 获取任务超时时间
func (t *BaseTask) GetTimeout() time.Duration {
	if t.Timeout <= 0 {
		return 30 * time.Second // 默认超时时间
	}
	return t.Timeout
}

// Execute 执行任务
func (t *BaseTask) Execute(ctx context.Context) error {
	if t.Func == nil {
		return nil
	}

	t.SetStatus(TaskStatusRunning)
	err := t.Func(ctx)

	if err != nil {
		t.SetStatus(TaskStatusFailed)
	} else {
		if t.Type == TaskTypeOnce {
			t.SetStatus(TaskStatusCompleted)
		} else {
			t.SetStatus(TaskStatusWaiting)
		}
	}

	return err
}

// CanExecute 检查是否可以执行
func (t *BaseTask) CanExecute(currentTime time.Time) bool {
	return t.Status == TaskStatusWaiting && !currentTime.Before(t.NextTime)
}

// IsCompleted 检查任务是否已完成
func (t *BaseTask) IsCompleted() bool {
	return t.Status == TaskStatusCompleted || t.Status == TaskStatusCanceled
}

// GetStatus 获取任务状态
func (t *BaseTask) GetStatus() TaskStatus {
	return t.Status
}

// SetStatus 设置任务状态
func (t *BaseTask) SetStatus(status TaskStatus) {
	t.Status = status
	t.UpdateTime = time.Now()
}

// OnceTask 一次性任务
type OnceTask struct {
	*BaseTask
}

// NewOnceTask 创建一次性任务
func NewOnceTask(name string, executeTime time.Time, executeMode TaskExecuteMode, timeout time.Duration, fn TaskFunc) *OnceTask {
	return &OnceTask{
		BaseTask: &BaseTask{
			ID:          uuid.New().String(),
			Name:        name,
			Type:        TaskTypeOnce,
			ExecuteMode: executeMode,
			Status:      TaskStatusWaiting,
			NextTime:    executeTime,
			Timeout:     timeout,
			Func:        fn,
			CreateTime:  time.Now(),
			UpdateTime:  time.Now(),
		},
	}
}

// UpdateNextTime 更新下次执行时间（一次性任务不更新）
func (t *OnceTask) UpdateNextTime(currentTime time.Time) time.Time {
	return t.NextTime
}

// IntervalTask 固定间隔任务
type IntervalTask struct {
	*BaseTask
	Interval time.Duration `json:"interval"`
}

// NewIntervalTask 创建固定间隔任务
func NewIntervalTask(name string, startTime time.Time, interval time.Duration, executeMode TaskExecuteMode, timeout time.Duration, fn TaskFunc) *IntervalTask {
	return &IntervalTask{
		BaseTask: &BaseTask{
			ID:          uuid.New().String(),
			Name:        name,
			Type:        TaskTypeInterval,
			ExecuteMode: executeMode,
			Status:      TaskStatusWaiting,
			NextTime:    startTime,
			Timeout:     timeout,
			Func:        fn,
			CreateTime:  time.Now(),
			UpdateTime:  time.Now(),
		},
		Interval: interval,
	}
}

// UpdateNextTime 更新下次执行时间
func (t *IntervalTask) UpdateNextTime(currentTime time.Time) time.Time {
	t.NextTime = currentTime.Add(t.Interval)
	t.UpdateTime = time.Now()
	return t.NextTime
}

// CronTask 基于Cron表达式的任务
type CronTask struct {
	*BaseTask
	CronExpr   string        `json:"cron_expr"`
	cronParser cron.Parser   `json:"-"`
	schedule   cron.Schedule `json:"-"`
}

// NewCronTask 创建Cron任务
func NewCronTask(name string, cronExpr string, executeMode TaskExecuteMode, timeout time.Duration, fn TaskFunc) (*CronTask, error) {
	parser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)

	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &CronTask{
		BaseTask: &BaseTask{
			ID:          uuid.New().String(),
			Name:        name,
			Type:        TaskTypeCron,
			ExecuteMode: executeMode,
			Status:      TaskStatusWaiting,
			NextTime:    schedule.Next(now),
			Timeout:     timeout,
			Func:        fn,
			CreateTime:  now,
			UpdateTime:  now,
		},
		CronExpr:   cronExpr,
		cronParser: parser,
		schedule:   schedule,
	}, nil
}

// UpdateNextTime 更新下次执行时间
func (t *CronTask) UpdateNextTime(currentTime time.Time) time.Time {
	t.NextTime = t.schedule.Next(currentTime)
	t.UpdateTime = time.Now()
	return t.NextTime
}
