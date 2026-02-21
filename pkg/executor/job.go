package executor

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// JobStatus 任务状态
type JobStatus string

const (
	// JobStatusPending 等待执行
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning 正在执行
	JobStatusRunning JobStatus = "running"
	// JobStatusSucceeded 执行成功
	JobStatusSucceeded JobStatus = "succeeded"
	// JobStatusFailed 执行失败
	JobStatusFailed JobStatus = "failed"
	// JobStatusCanceled 已取消（通常是 executor 停止时队列中还未执行的任务）
	JobStatusCanceled JobStatus = "canceled"
)

// JobFunc 任务执行函数签名
type JobFunc func(ctx context.Context) error

// Job 任务结构体
type Job struct {
	// ID 任务唯一标识
	ID string `json:"id"`

	// Name 任务名称（便于日志和监控）
	Name string `json:"name"`

	// Status 当前状态
	Status JobStatus `json:"status"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`

	// StartedAt 开始执行时间（nil 表示尚未开始）
	StartedAt *time.Time `json:"started_at,omitempty"`

	// FinishedAt 结束时间（nil 表示尚未结束）
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Error 错误摘要（只有 failed 状态才有值）
	Error string `json:"error,omitempty"`

	// TraceID 用于日志追踪（从提交时的 context 中复制）
	TraceID string `json:"trace_id,omitempty"`

	// Timeout 超时时间
	Timeout time.Duration `json:"timeout"`

	// fn 实际执行函数（内部字段，不对外暴露）
	fn JobFunc `json:"-"`

	// ctx 执行上下文（内部字段）
	ctx context.Context `json:"-"`

	// cancel 取消函数（内部字段）
	cancel context.CancelFunc `json:"-"`
}

// NewJob 创建新任务
func NewJob(ctx context.Context, name string, timeout time.Duration, fn JobFunc) *Job {
	jobID := uuid.New().String()

	// 从原始 context 中提取 traceId（如果有）
	var traceID string
	if ctx != nil {
		if val := ctx.Value("traceId"); val != nil {
			if tid, ok := val.(string); ok {
				traceID = tid
			}
		}
	}

	// 创建一个新的后台 context（不依赖原始请求的生命周期）
	bgCtx := context.Background()
	if traceID != "" {
		bgCtx = context.WithValue(bgCtx, "traceId", traceID)
	}

	return &Job{
		ID:        jobID,
		Name:      name,
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
		Timeout:   timeout,
		TraceID:   traceID,
		fn:        fn,
		ctx:       bgCtx,
	}
}

// Duration 返回任务执行耗时（如果尚未开始或正在执行，返回已等待/已执行时间）
func (j *Job) Duration() time.Duration {
	if j.StartedAt == nil {
		// 尚未开始，返回等待时间
		return time.Since(j.CreatedAt)
	}
	if j.FinishedAt == nil {
		// 正在执行，返回已执行时间
		return time.Since(*j.StartedAt)
	}
	// 已结束，返回总执行时间
	return j.FinishedAt.Sub(*j.StartedAt)
}

