package client

import (
	"context"
	"time"

	"github.com/xsxdot/aio/pkg/scheduler"
)

// Scheduler 客户端调度器适配器
type Scheduler struct {
	client    *Client
	scheduler *scheduler.Scheduler
}

// NewScheduler 创建客户端调度器
func NewScheduler(client *Client, options *scheduler.SchedulerOptions) *Scheduler {
	scheduler, err := scheduler.NewSchedulerWithEtcd(client.Etcd.GetClient(), options)
	if err != nil {
		panic(err)
	}
	return &Scheduler{
		client:    client,
		scheduler: scheduler,
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	return s.scheduler.Start(context.Background())
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	return s.scheduler.Stop(context.Background())
}

// AddTask 添加任务
func (s *Scheduler) AddTask(name string, handler scheduler.TaskFunc, needLock bool) (string, error) {
	return s.scheduler.AddTask(name, handler, needLock)
}

// AddDelayTask 添加延时任务
func (s *Scheduler) AddDelayTask(name string, delay time.Duration, handler scheduler.TaskFunc, needLock bool) (string, error) {
	return s.scheduler.AddDelayTask(name, delay, handler, needLock)
}

// AddIntervalTask 添加周期性任务
func (s *Scheduler) AddIntervalTask(name string, interval time.Duration, immediate bool, handler scheduler.TaskFunc, needLock bool) (string, error) {
	return s.scheduler.AddIntervalTask(name, interval, immediate, handler, needLock)
}

// AddCronTask 添加基于Cron表达式的定时任务
func (s *Scheduler) AddCronTask(name string, cronExpr string, handler scheduler.TaskFunc, needLock bool) (string, error) {
	return s.scheduler.AddCronTask(name, cronExpr, handler, needLock)
}

// CancelTask 取消任务
func (s *Scheduler) CancelTask(taskID string) error {
	return s.scheduler.CancelTask(taskID)
}

// GetTask 获取任务信息
func (s *Scheduler) GetTask(taskID string) (*scheduler.Task, error) {
	return s.scheduler.GetTask(taskID)
}

// GetAllTasks 获取所有任务
func (s *Scheduler) GetAllTasks() []*scheduler.Task {
	return s.scheduler.GetAllTasks()
}
