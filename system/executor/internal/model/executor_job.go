package model

import (
	"time"

	"github.com/xsxdot/aio/pkg/core/model/common"
)

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"   // 待执行
	JobStatusRunning   JobStatus = "running"   // 执行中
	JobStatusSucceeded JobStatus = "succeeded" // 成功
	JobStatusFailed    JobStatus = "failed"    // 失败（可重试）
	JobStatusCanceled  JobStatus = "canceled"  // 已取消
	JobStatusDead      JobStatus = "dead"      // 死信（超过最大重试次数）
)

// ExecutorJobModel 任务主表
type ExecutorJobModel struct {
	common.Model
	// 路由信息
	TargetService string `gorm:"column:target_service;size:100;not null;index:idx_target_status_next;index:idx_target_method_status_next" json:"target_service" comment:"目标服务名"`
	Method        string `gorm:"column:method;size:100;not null;index:idx_target_method_status_next" json:"method" comment:"方法名"`
	ArgsJSON      string `gorm:"column:args_json;type:text" json:"args_json" comment:"参数JSON"`

	// 调度信息
	Status    JobStatus  `gorm:"column:status;size:20;not null;index:idx_target_status_next;index:idx_status;index:idx_target_method_status_next" json:"status" comment:"任务状态"`
	Priority  int32      `gorm:"column:priority;default:0;not null;index:idx_priority" json:"priority" comment:"优先级，数字越大优先级越高"`
	NextRunAt *time.Time `gorm:"column:next_run_at;index:idx_target_status_next;index:idx_next_run;index:idx_target_method_status_next" json:"next_run_at" comment:"下次执行时间"`

	// 重试信息
	MaxAttempts int32 `gorm:"column:max_attempts;default:3;not null" json:"max_attempts" comment:"最大重试次数"`
	Attempts    int32 `gorm:"column:attempts;default:0;not null" json:"attempts" comment:"已尝试次数"`

	// 租约信息
	LeaseOwner string     `gorm:"column:lease_owner;size:100;index:idx_lease_owner" json:"lease_owner" comment:"租约持有者（consumer_id）"`
	LeaseUntil *time.Time `gorm:"column:lease_until;index:idx_lease_until" json:"lease_until" comment:"租约到期时间"`

	// 幂等信息
	DedupKey string `gorm:"column:dedup_key;size:255;uniqueIndex:idx_dedup_key" json:"dedup_key" comment:"幂等键"`

	// 结果信息
	LastError  string `gorm:"column:last_error;type:text" json:"last_error" comment:"最后错误信息"`
	ResultJSON string `gorm:"column:result_json;type:text" json:"result_json" comment:"结果JSON"`
}

// TableName 指定表名
func (ExecutorJobModel) TableName() string {
	return "executor_jobs"
}

// ExecutorJobAttemptModel 任务尝试记录表（审计用）
type ExecutorJobAttemptModel struct {
	common.Model
	JobID      uint64     `gorm:"column:job_id;not null;index:idx_job_id" json:"job_id" comment:"任务ID"`
	AttemptNo  int32      `gorm:"column:attempt_no;not null" json:"attempt_no" comment:"尝试次数"`
	WorkerID   string     `gorm:"column:worker_id;size:100" json:"worker_id" comment:"执行者ID"`
	Status     JobStatus  `gorm:"column:status;size:20;not null" json:"status" comment:"执行状态"`
	Error      string     `gorm:"column:error;type:text" json:"error" comment:"错误信息"`
	StartedAt  *time.Time `gorm:"column:started_at" json:"started_at" comment:"开始时间"`
	FinishedAt *time.Time `gorm:"column:finished_at" json:"finished_at" comment:"完成时间"`
}

// TableName 指定表名
func (ExecutorJobAttemptModel) TableName() string {
	return "executor_job_attempts"
}
