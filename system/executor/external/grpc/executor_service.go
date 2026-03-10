package grpc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/executor/api/client"
	pb "github.com/xsxdot/aio/system/executor/api/proto"
	"github.com/xsxdot/aio/system/executor/internal/app"
	"github.com/xsxdot/aio/system/executor/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecutorService gRPC 服务实现
type ExecutorService struct {
	pb.UnimplementedExecutorServiceServer
	client *client.ExecutorClient
	app    *app.App
	log    *logger.Log
}

// NewExecutorService 创建 gRPC 服务实例
func NewExecutorService(client *client.ExecutorClient, app *app.App, log *logger.Log) *ExecutorService {
	return &ExecutorService{
		client: client,
		app:    app,
		log:    log.WithEntryName("ExecutorService"),
	}
}

// ServiceName 返回服务名称
func (s *ExecutorService) ServiceName() string {
	return "executor.v1.ExecutorService"
}

// ServiceVersion 返回服务版本
func (s *ExecutorService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册 gRPC 服务
func (s *ExecutorService) RegisterService(server *grpc.Server) error {
	pb.RegisterExecutorServiceServer(server, s)
	s.log.Info("Executor gRPC 服务注册成功")
	return nil
}

// SubmitJob 提交任务
func (s *ExecutorService) SubmitJob(ctx context.Context, req *pb.SubmitJobRequest) (*pb.SubmitJobResponse, error) {
	if strings.TrimSpace(req.Env) == "" {
		return nil, status.Error(codes.InvalidArgument, "env 不能为空")
	}
	if strings.TrimSpace(req.DedupKey) == "" {
		return nil, status.Error(codes.InvalidArgument, "dedup_key 不能为空")
	}

	jobID, err := s.app.JobService.SubmitJob(
		ctx,
		req.Env,
		req.TargetService,
		req.Method,
		req.ArgsJson,
		req.RunAt,
		req.MaxAttempts,
		req.Priority,
		req.DedupKey,
	)
	if err != nil {
		s.log.WithErr(err).Error("提交任务失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.SubmitJobResponse{
		JobId: int64(jobID),
	}, nil
}

// AcquireJob 领取任务
func (s *ExecutorService) AcquireJob(ctx context.Context, req *pb.AcquireJobRequest) (*pb.AcquireJobResponse, error) {
	if strings.TrimSpace(req.Env) == "" {
		return nil, status.Error(codes.InvalidArgument, "env 不能为空")
	}

	job, err := s.app.JobService.AcquireJob(
		ctx,
		req.Env,
		req.TargetService,
		req.Method,
		req.ConsumerId,
		req.LeaseDuration,
	)
	if err != nil {
		s.log.WithErr(err).Error("领取任务失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 没有可领取的任务
	if job == nil {
		return &pb.AcquireJobResponse{
			JobId: 0,
		}, nil
	}

	// 返回任务信息
	return &pb.AcquireJobResponse{
		JobId:         int64(job.ID),
		AttemptNo:     job.Attempts,
		TargetService: job.TargetService,
		Method:        job.Method,
		ArgsJson:      job.ArgsJSON,
		LeaseUntil:    job.LeaseUntil.Unix(),
		Env:           job.Env,
	}, nil
}

// RenewLease 续租
func (s *ExecutorService) RenewLease(ctx context.Context, req *pb.RenewLeaseRequest) (*pb.RenewLeaseResponse, error) {
	job, err := s.app.JobService.RenewLease(
		ctx,
		uint64(req.JobId),
		req.AttemptNo,
		req.ConsumerId,
		req.ExtendDuration,
	)
	if err != nil {
		s.log.WithErr(err).Error("续租失败")
		return &pb.RenewLeaseResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.RenewLeaseResponse{
		Success:    true,
		Message:    "续租成功",
		LeaseUntil: job.LeaseUntil.Unix(),
	}, nil
}

// AckJob 确认任务
func (s *ExecutorService) AckJob(ctx context.Context, req *pb.AckJobRequest) (*pb.AckJobResponse, error) {
	// 转换状态
	var status model.JobStatus
	switch req.Status {
	case pb.JobStatus_JOB_STATUS_SUCCEEDED:
		status = model.JobStatusSucceeded
	case pb.JobStatus_JOB_STATUS_FAILED:
		status = model.JobStatusFailed
	default:
		return &pb.AckJobResponse{
			Success: false,
			Message: "无效的任务状态",
		}, nil
	}

	err := s.app.JobService.AckJob(
		ctx,
		uint64(req.JobId),
		req.AttemptNo,
		req.ConsumerId,
		status,
		req.Error,
		req.ResultJson,
		req.RetryAfter,
	)
	if err != nil {
		s.log.WithErr(err).Error("确认任务失败")
		return &pb.AckJobResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.AckJobResponse{
		Success: true,
		Message: "确认成功",
	}, nil
}

// GetJob 获取任务详情
func (s *ExecutorService) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.JobResponse, error) {
	job, err := s.app.JobService.GetJob(ctx, uint64(req.JobId))
	if err != nil {
		s.log.WithErr(err).Error("获取任务失败")
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return s.modelToProto(job), nil
}

// ListJobs 列出任务
func (s *ExecutorService) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	if strings.TrimSpace(req.Env) == "" {
		return nil, status.Error(codes.InvalidArgument, "env 不能为空")
	}

	// 转换状态
	var jobStatus model.JobStatus
	if req.Status != pb.JobStatus_JOB_STATUS_UNSPECIFIED {
		jobStatus = s.protoStatusToModel(req.Status)
	}

	jobs, total, err := s.app.JobService.ListJobs(
		ctx,
		req.Env,
		req.TargetService,
		jobStatus,
		req.PageNum,
		req.PageSize,
	)
	if err != nil {
		s.log.WithErr(err).Error("列出任务失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 转换为 proto
	pbJobs := make([]*pb.JobResponse, len(jobs))
	for i, job := range jobs {
		pbJobs[i] = s.modelToProto(job)
	}

	return &pb.ListJobsResponse{
		Jobs:  pbJobs,
		Total: total,
	}, nil
}

// CancelJob 取消任务
func (s *ExecutorService) CancelJob(ctx context.Context, req *pb.CancelJobRequest) (*pb.CancelJobResponse, error) {
	err := s.app.JobService.CancelJob(ctx, uint64(req.JobId))
	if err != nil {
		s.log.WithErr(err).Error("取消任务失败")
		return &pb.CancelJobResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.CancelJobResponse{
		Success: true,
		Message: "取消成功",
	}, nil
}

// RequeueJob 重新入队
func (s *ExecutorService) RequeueJob(ctx context.Context, req *pb.RequeueJobRequest) (*pb.RequeueJobResponse, error) {
	err := s.app.JobService.RequeueJob(ctx, uint64(req.JobId), req.RunAt)
	if err != nil {
		s.log.WithErr(err).Error("重新入队失败")
		return &pb.RequeueJobResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.RequeueJobResponse{
		Success: true,
		Message: "重新入队成功",
	}, nil
}

// UpdateJobArgs 更新任务参数
func (s *ExecutorService) UpdateJobArgs(ctx context.Context, req *pb.UpdateJobArgsRequest) (*pb.UpdateJobArgsResponse, error) {
	// 校验 JSON 合法性（非空时）
	if req.ArgsJson != "" {
		if !json.Valid([]byte(req.ArgsJson)) {
			s.log.Error("参数 JSON 格式不合法")
			return nil, status.Error(codes.InvalidArgument, "参数 JSON 格式不合法")
		}
	}

	err := s.app.JobService.UpdateJobArgsJSON(ctx, uint64(req.JobId), req.ArgsJson)
	if err != nil {
		s.log.WithErr(err).Error("更新任务参数失败")
		// 根据错误类型返回不同的状态码
		if err.Error() == "任务不存在" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err.Error() == "running 任务不允许修改参数" {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return &pb.UpdateJobArgsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.UpdateJobArgsResponse{
		Success: true,
		Message: "参数更新成功",
	}, nil
}

// modelToProto 转换模型为 proto
func (s *ExecutorService) modelToProto(job *model.ExecutorJobModel) *pb.JobResponse {
	resp := &pb.JobResponse{
		Id:            int64(job.ID),
		Env:           job.Env,
		TargetService: job.TargetService,
		Method:        job.Method,
		ArgsJson:      job.ArgsJSON,
		Status:        s.modelStatusToProto(job.Status),
		Priority:      job.Priority,
		MaxAttempts:   job.MaxAttempts,
		Attempts:      job.Attempts,
		LeaseOwner:    job.LeaseOwner,
		DedupKey:      job.DedupKey,
		LastError:     job.LastError,
		ResultJson:    job.ResultJSON,
		CreatedAt:     job.CreatedAt.Unix(),
		UpdatedAt:     job.UpdatedAt.Unix(),
	}

	if job.NextRunAt != nil {
		resp.NextRunAt = job.NextRunAt.Unix()
	}

	if job.LeaseUntil != nil {
		resp.LeaseUntil = job.LeaseUntil.Unix()
	}

	return resp
}

// modelStatusToProto 转换模型状态为 proto 状态
func (s *ExecutorService) modelStatusToProto(status model.JobStatus) pb.JobStatus {
	switch status {
	case model.JobStatusPending:
		return pb.JobStatus_JOB_STATUS_PENDING
	case model.JobStatusRunning:
		return pb.JobStatus_JOB_STATUS_RUNNING
	case model.JobStatusSucceeded:
		return pb.JobStatus_JOB_STATUS_SUCCEEDED
	case model.JobStatusFailed:
		return pb.JobStatus_JOB_STATUS_FAILED
	case model.JobStatusCanceled:
		return pb.JobStatus_JOB_STATUS_CANCELED
	case model.JobStatusDead:
		return pb.JobStatus_JOB_STATUS_DEAD
	default:
		return pb.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

// protoStatusToModel 转换 proto 状态为模型状态
func (s *ExecutorService) protoStatusToModel(status pb.JobStatus) model.JobStatus {
	switch status {
	case pb.JobStatus_JOB_STATUS_PENDING:
		return model.JobStatusPending
	case pb.JobStatus_JOB_STATUS_RUNNING:
		return model.JobStatusRunning
	case pb.JobStatus_JOB_STATUS_SUCCEEDED:
		return model.JobStatusSucceeded
	case pb.JobStatus_JOB_STATUS_FAILED:
		return model.JobStatusFailed
	case pb.JobStatus_JOB_STATUS_CANCELED:
		return model.JobStatusCanceled
	case pb.JobStatus_JOB_STATUS_DEAD:
		return model.JobStatusDead
	default:
		return ""
	}
}
