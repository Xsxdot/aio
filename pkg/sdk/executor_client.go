package sdk

import (
	"context"
	"encoding/json"
	"strings"

	executorpb "github.com/xsxdot/aio/system/executor/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecutorClient 任务执行器客户端
type ExecutorClient struct {
	env     string
	service executorpb.ExecutorServiceClient
}

// newExecutorClient 创建任务执行器客户端
func newExecutorClient(conn *grpc.ClientConn, env string) *ExecutorClient {
	if strings.TrimSpace(env) == "" {
		env = "dev"
	}
	return &ExecutorClient{
		env:     env,
		service: executorpb.NewExecutorServiceClient(conn),
	}
}

// AckStatus 任务确认状态
type AckStatus string

const (
	// AckStatusSucceeded 任务执行成功
	AckStatusSucceeded AckStatus = "SUCCEEDED"
	// AckStatusFailed 任务执行失败（可重试）
	AckStatusFailed AckStatus = "FAILED"
)

// SubmitJobRequest 提交任务请求（SDK 友好版）
type SubmitJobRequest struct {
	TargetService string // 目标服务名
	Method        string // 方法名
	ArgsJSON      string // 参数 JSON
	RunAt         int64  // 执行时间（Unix 时间戳秒），0表示立即执行
	MaxAttempts   int32  // 最大重试次数，默认3次
	Priority      int32  // 优先级，数字越大优先级越高，默认0
	DedupKey      string // 幂等键（必填）
}

// SubmitJob 提交任务
// req.Env 和 req.DedupKey 为必填项，若为空会在客户端直接返回错误。
func (c *ExecutorClient) SubmitJob(ctx context.Context, req *SubmitJobRequest) (int64, error) {
	if strings.TrimSpace(req.DedupKey) == "" {
		return 0, WrapError(
			status.Error(codes.InvalidArgument, "dedup_key 不能为空"),
			"submit job failed",
		)
	}

	pbReq := &executorpb.SubmitJobRequest{
		Env:           c.env,
		TargetService: req.TargetService,
		Method:        req.Method,
		ArgsJson:      req.ArgsJSON,
		RunAt:         req.RunAt,
		MaxAttempts:   req.MaxAttempts,
		Priority:      req.Priority,
		DedupKey:      req.DedupKey,
	}

	resp, err := c.service.SubmitJob(ctx, pbReq)
	if err != nil {
		return 0, WrapError(err, "submit job failed")
	}

	return resp.JobId, nil
}

// SubmitJobWithArgs 提交任务（自动序列化参数）
// env、dedupKey 为必填项，不能为空；args 会被自动序列化为 JSON。
func (c *ExecutorClient) SubmitJobWithArgs(ctx context.Context, targetService, method, dedupKey string, args interface{}, opts ...SubmitJobOption) (int64, error) {
	// 序列化参数
	argsJSON := ""
	if args != nil {
		data, err := json.Marshal(args)
		if err != nil {
			return 0, WrapError(err, "marshal args failed")
		}
		argsJSON = string(data)
	}

	// 构建请求
	req := &SubmitJobRequest{
		TargetService: targetService,
		Method:        method,
		ArgsJSON:      argsJSON,
		DedupKey:      dedupKey,
	}

	// 应用选项
	for _, opt := range opts {
		opt(req)
	}

	return c.SubmitJob(ctx, req)
}

// SubmitJobOption 提交任务选项
type SubmitJobOption func(*SubmitJobRequest)

// WithRunAt 设置执行时间
func WithRunAt(runAt int64) SubmitJobOption {
	return func(req *SubmitJobRequest) {
		req.RunAt = runAt
	}
}

// WithMaxAttempts 设置最大重试次数
func WithMaxAttempts(maxAttempts int32) SubmitJobOption {
	return func(req *SubmitJobRequest) {
		req.MaxAttempts = maxAttempts
	}
}

// WithPriority 设置优先级
func WithPriority(priority int32) SubmitJobOption {
	return func(req *SubmitJobRequest) {
		req.Priority = priority
	}
}

// AcquireJobRequest 领取任务请求（SDK 友好版）
type AcquireJobRequest struct {
	TargetService string // 目标服务名（只能领取指定服务的任务）
	Method        string // 可选：方法名过滤，空表示不过滤
	ConsumerID    string // 消费者ID（实例唯一标识）
	LeaseDuration int32  // 租约时长（秒），默认30秒
}

// AcquiredJob 已领取的任务信息
type AcquiredJob struct {
	JobID         int64  // 任务ID
	AttemptNo     int32  // 当前尝试次数
	Env           string // 环境标识
	TargetService string // 目标服务名
	Method        string // 方法名
	ArgsJSON      string // 参数 JSON
	LeaseUntil    int64  // 租约到期时间（Unix 时间戳秒）
}

// AcquireJob 领取任务
// req.Env 为必填项；返回 (nil, nil) 表示没有可领取的任务
func (c *ExecutorClient) AcquireJob(ctx context.Context, req *AcquireJobRequest) (*AcquiredJob, error) {
	pbReq := &executorpb.AcquireJobRequest{
		Env:           c.env,
		TargetService: req.TargetService,
		Method:        req.Method,
		ConsumerId:    req.ConsumerID,
		LeaseDuration: req.LeaseDuration,
	}

	resp, err := c.service.AcquireJob(ctx, pbReq)
	if err != nil {
		return nil, WrapError(err, "acquire job failed")
	}

	// job_id=0 表示没有可领取的任务
	if resp.JobId == 0 {
		return nil, nil
	}

	return &AcquiredJob{
		JobID:         resp.JobId,
		AttemptNo:     resp.AttemptNo,
		Env:           resp.Env,
		TargetService: resp.TargetService,
		Method:        resp.Method,
		ArgsJSON:      resp.ArgsJson,
		LeaseUntil:    resp.LeaseUntil,
	}, nil
}

// RenewLease 续租
// 返回新的租约到期时间（Unix 时间戳秒）
func (c *ExecutorClient) RenewLease(ctx context.Context, jobID int64, attemptNo int32, consumerID string, extendDuration int32) (int64, error) {
	pbReq := &executorpb.RenewLeaseRequest{
		JobId:          jobID,
		AttemptNo:      attemptNo,
		ConsumerId:     consumerID,
		ExtendDuration: extendDuration,
	}

	resp, err := c.service.RenewLease(ctx, pbReq)
	if err != nil {
		return 0, WrapError(err, "renew lease failed")
	}

	// 服务端返回 success=false 表示续租失败（可能是租约已过期或被其他 worker 领取）
	if !resp.Success {
		return 0, WrapError(
			status.Error(codes.FailedPrecondition, resp.Message),
			"renew lease rejected",
		)
	}

	return resp.LeaseUntil, nil
}

// AckJobRequest 确认任务请求（SDK 友好版）
type AckJobRequest struct {
	JobID      int64     // 任务ID
	AttemptNo  int32     // 尝试次数（用于校验）
	ConsumerID string    // 消费者ID
	Status     AckStatus // 执行结果状态（SUCCEEDED/FAILED）
	Error      string    // 错误信息（失败时）
	ResultJSON string    // 结果 JSON（可选）
	RetryAfter int32     // 重试延迟（秒），仅当status=FAILED时有效，0表示使用默认退避策略
}

// AckJob 确认任务执行结果
func (c *ExecutorClient) AckJob(ctx context.Context, req *AckJobRequest) error {
	// 转换状态
	var pbStatus executorpb.JobStatus
	switch req.Status {
	case AckStatusSucceeded:
		pbStatus = executorpb.JobStatus_JOB_STATUS_SUCCEEDED
	case AckStatusFailed:
		pbStatus = executorpb.JobStatus_JOB_STATUS_FAILED
	default:
		return WrapError(
			status.Error(codes.InvalidArgument, "invalid ack status"),
			"invalid ack status",
		)
	}

	pbReq := &executorpb.AckJobRequest{
		JobId:      req.JobID,
		AttemptNo:  req.AttemptNo,
		ConsumerId: req.ConsumerID,
		Status:     pbStatus,
		Error:      req.Error,
		ResultJson: req.ResultJSON,
		RetryAfter: req.RetryAfter,
	}

	resp, err := c.service.AckJob(ctx, pbReq)
	if err != nil {
		return WrapError(err, "ack job failed")
	}

	// 服务端返回 success=false 表示确认失败（可能是租约已过期或attempt_no不匹配）
	if !resp.Success {
		return WrapError(
			status.Error(codes.FailedPrecondition, resp.Message),
			"ack job rejected",
		)
	}

	return nil
}

// UpdateJobArgs 更新任务参数
func (c *ExecutorClient) UpdateJobArgs(ctx context.Context, jobID int64, argsJSON string) error {
	pbReq := &executorpb.UpdateJobArgsRequest{
		JobId:    jobID,
		ArgsJson: argsJSON,
	}

	resp, err := c.service.UpdateJobArgs(ctx, pbReq)
	if err != nil {
		return WrapError(err, "update job args failed")
	}

	// 服务端返回 success=false 表示更新失败
	if !resp.Success {
		return WrapError(
			status.Error(codes.FailedPrecondition, resp.Message),
			"update job args rejected",
		)
	}

	return nil
}
