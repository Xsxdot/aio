package model

import (
	"time"

	"github.com/xsxdot/aio/pkg/core/model/common"
)

// RetryBackoffType 重试退避类型
type RetryBackoffType string

const (
	RetryBackoffExponential RetryBackoffType = "exponential" // 指数退避（默认）
	RetryBackoffFixed       RetryBackoffType = "fixed"       // 固定间隔
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
	// 环境标识（(env, dedup_key) 联合唯一，隔离不同环境）
	Env string `gorm:"column:env;size:50;not null;default:'dev';uniqueIndex:idx_env_dedup_key;index:idx_env_target_status_next;index:idx_env_target_method_status_next" json:"env" comment:"环境标识（dev/prod/test）"`

	// 路由信息
	TargetService string `gorm:"column:target_service;size:100;not null;index:idx_env_target_status_next;index:idx_env_target_method_status_next" json:"target_service" comment:"目标服务名"`
	Method        string `gorm:"column:method;size:100;not null;index:idx_env_target_method_status_next" json:"method" comment:"方法名"`
	ArgsJSON      string `gorm:"column:args_json;type:text" json:"args_json" comment:"参数JSON"`

	// 调度信息
	Status    JobStatus  `gorm:"column:status;size:20;not null;index:idx_env_target_status_next;index:idx_status;index:idx_env_target_method_status_next" json:"status" comment:"任务状态"`
	Priority  int32      `gorm:"column:priority;default:0;not null;index:idx_priority" json:"priority" comment:"优先级，数字越大优先级越高"`
	NextRunAt *time.Time `gorm:"column:next_run_at;index:idx_env_target_status_next;index:idx_next_run;index:idx_env_target_method_status_next" json:"next_run_at" comment:"下次执行时间"`

	// 重试信息
	MaxAttempts       int32            `gorm:"column:max_attempts;default:3;not null" json:"max_attempts" comment:"最大重试次数"`
	Attempts          int32            `gorm:"column:attempts;default:0;not null" json:"attempts" comment:"已尝试次数"`
	RetryBackoffType  RetryBackoffType `gorm:"column:retry_backoff_type;size:20;default:exponential" json:"retry_backoff_type" comment:"重试退避类型"`
	RetryIntervalSec  int32            `gorm:"column:retry_interval_sec;default:0" json:"retry_interval_sec" comment:"固定间隔秒数，仅 fixed 时有效"`

	// 租约信息
	LeaseOwner string     `gorm:"column:lease_owner;size:100;index:idx_lease_owner" json:"lease_owner" comment:"租约持有者（consumer_id）"`
	LeaseUntil *time.Time `gorm:"column:lease_until;index:idx_lease_until" json:"lease_until" comment:"租约到期时间"`

	// 幂等信息（与 Env 联合唯一，隔离不同环境）
	DedupKey string `gorm:"column:dedup_key;size:255;uniqueIndex:idx_env_dedup_key" json:"dedup_key" comment:"幂等键"`

	// 顺序执行（同 key 任务串行）
	SequenceKey string `gorm:"column:sequence_key;size:255;index:idx_sequence_key_status" json:"sequence_key" comment:"顺序键，非空时同 key 任务串行执行"`

	// 回调信息（由调用方提交时指定）
	Source       string `gorm:"column:source;size:64;index:idx_source" json:"source" comment:"任务来源标识（如 workflow），非空表示需要触发完成回调"`
	CallbackData string `gorm:"column:callback_data;type:text" json:"callback_data" comment:"回调透传数据（JSON），由调用方自行约定格式"`

	// 结果信息
	LastError     string `gorm:"column:last_error;type:text" json:"last_error" comment:"最后错误信息"`
	LastErrorType string `gorm:"column:last_error_type;size:64" json:"last_error_type" comment:"最后错误类型"`
	ResultJSON    string `gorm:"column:result_json;type:text" json:"result_json" comment:"结果JSON"`
}

// TableName 指定表名
func (ExecutorJobModel) TableName() string {
	return "aio_executor_jobs"
}

// ExecutorJobAttemptModel 任务尝试记录表（审计用）
type ExecutorJobAttemptModel struct {
	common.Model
	JobID      uint64     `gorm:"column:job_id;not null;index:idx_job_id" json:"job_id" comment:"任务ID"`
	AttemptNo  int32      `gorm:"column:attempt_no;not null" json:"attempt_no" comment:"尝试次数"`
	WorkerID   string     `gorm:"column:worker_id;size:100" json:"worker_id" comment:"执行者ID"`
	Status     JobStatus  `gorm:"column:status;size:20;not null" json:"status" comment:"执行状态"`
	Error      string     `gorm:"column:error;type:text" json:"error" comment:"错误信息"`
	ErrorType  string     `gorm:"column:error_type;size:64" json:"error_type" comment:"错误类型"`
	StartedAt  *time.Time `gorm:"column:started_at" json:"started_at" comment:"开始时间"`
	FinishedAt *time.Time `gorm:"column:finished_at" json:"finished_at" comment:"完成时间"`
}

// TableName 指定表名
func (ExecutorJobAttemptModel) TableName() string {
	return "executor_job_attempts"
}
