package grpc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/workflow/api/client"
	pb "github.com/xsxdot/aio/system/workflow/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkflowService gRPC 服务实现
type WorkflowService struct {
	pb.UnimplementedWorkflowServiceServer
	client *client.WorkflowClient
	log    *logger.Log
}

// NewWorkflowService 创建 gRPC 服务实例
func NewWorkflowService(wfClient *client.WorkflowClient, log *logger.Log) *WorkflowService {
	return &WorkflowService{
		client: wfClient,
		log:    log.WithEntryName("WorkflowService"),
	}
}

// ServiceName 返回服务名称
func (s *WorkflowService) ServiceName() string {
	return "workflow.v1.WorkflowService"
}

// ServiceVersion 返回服务版本
func (s *WorkflowService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册 gRPC 服务
func (s *WorkflowService) RegisterService(server *grpc.Server) error {
	pb.RegisterWorkflowServiceServer(server, s)
	s.log.Info("Workflow gRPC 服务注册成功")
	return nil
}

// CreateDef 创建工作流定义
func (s *WorkflowService) CreateDef(ctx context.Context, req *pb.CreateDefRequest) (*pb.CreateDefResponse, error) {
	if strings.TrimSpace(req.Code) == "" {
		return nil, status.Error(codes.InvalidArgument, "code 不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}
	if strings.TrimSpace(req.DagJson) == "" {
		return nil, status.Error(codes.InvalidArgument, "dag_json 不能为空")
	}

	version := req.Version
	if version <= 0 {
		version = 1
	}

	defID, err := s.client.CreateDef(ctx, req.Code, req.Name, req.DagJson, version)
	if err != nil {
		s.log.WithErr(err).Error("创建工作流定义失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateDefResponse{
		DefId: defID,
	}, nil
}

// StartWorkflow 启动工作流
func (s *WorkflowService) StartWorkflow(ctx context.Context, req *pb.StartWorkflowRequest) (*pb.StartWorkflowResponse, error) {
	if strings.TrimSpace(req.DefCode) == "" {
		return nil, status.Error(codes.InvalidArgument, "def_code 不能为空")
	}

	var initialData map[string]interface{}
	if req.InitialDataJson != "" {
		if !json.Valid([]byte(req.InitialDataJson)) {
			return nil, status.Error(codes.InvalidArgument, "initial_data_json 格式不合法")
		}
		if err := json.Unmarshal([]byte(req.InitialDataJson), &initialData); err != nil {
			return nil, status.Error(codes.InvalidArgument, "initial_data_json 解析失败")
		}
	}
	if initialData == nil {
		initialData = make(map[string]interface{})
	}

	env := req.Env
	if strings.TrimSpace(env) == "" {
		env = base.ENV
	}
	instanceID, err := s.client.StartWorkflow(ctx, req.DefCode, initialData, env)
	if err != nil {
		s.log.WithErr(err).Error("启动工作流失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.StartWorkflowResponse{
		InstanceId: instanceID,
	}, nil
}

// ReportNodeCompleted 报告节点完成
func (s *WorkflowService) ReportNodeCompleted(ctx context.Context, req *pb.ReportNodeCompletedRequest) (*pb.ReportNodeCompletedResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}
	if strings.TrimSpace(req.NodeId) == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id 不能为空")
	}

	var output map[string]interface{}
	if req.OutputJson != "" {
		if !json.Valid([]byte(req.OutputJson)) {
			return &pb.ReportNodeCompletedResponse{
				Success: false,
				Message: "output_json 格式不合法",
			}, nil
		}
		if err := json.Unmarshal([]byte(req.OutputJson), &output); err != nil {
			return &pb.ReportNodeCompletedResponse{
				Success: false,
				Message: "output_json 解析失败",
			}, nil
		}
	}
	if output == nil {
		output = make(map[string]interface{})
	}

	env := req.Env
	if strings.TrimSpace(env) == "" {
		env = base.ENV
	}
	err := s.client.ReportNodeCompleted(ctx, req.InstanceId, req.NodeId, output, env)
	if err != nil {
		s.log.WithErr(err).Error("报告节点完成失败")
		return &pb.ReportNodeCompletedResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ReportNodeCompletedResponse{
		Success: true,
		Message: "成功",
	}, nil
}

// RollbackToNode 回滚到指定节点
func (s *WorkflowService) RollbackToNode(ctx context.Context, req *pb.RollbackToNodeRequest) (*pb.RollbackToNodeResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}
	if strings.TrimSpace(req.TargetNodeId) == "" {
		return nil, status.Error(codes.InvalidArgument, "target_node_id 不能为空")
	}

	env := req.Env
	if strings.TrimSpace(env) == "" {
		env = base.ENV
	}
	err := s.client.RollbackToNode(ctx, req.InstanceId, req.TargetNodeId, env)
	if err != nil {
		s.log.WithErr(err).Error("回滚失败")
		return &pb.RollbackToNodeResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.RollbackToNodeResponse{
		Success: true,
		Message: "回滚成功",
	}, nil
}

// GetExecutionTrail 获取执行轨迹
func (s *WorkflowService) GetExecutionTrail(ctx context.Context, req *pb.GetExecutionTrailRequest) (*pb.GetExecutionTrailResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}

	trail, err := s.client.GetExecutionTrail(ctx, req.InstanceId)
	if err != nil {
		s.log.WithErr(err).Error("获取执行轨迹失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	checkpoints := make([]*pb.ExecutionTrailCheckpoint, len(trail.Checkpoints))
	for i, cp := range trail.Checkpoints {
		nodeOutputJSON := "{}"
		stateAfterJSON := "{}"
		if cp.NodeOutput != nil {
			if b, err := json.Marshal(cp.NodeOutput); err == nil {
				nodeOutputJSON = string(b)
			}
		}
		if cp.StateAfter != nil {
			if b, err := json.Marshal(cp.StateAfter); err == nil {
				stateAfterJSON = string(b)
			}
		}
		checkpoints[i] = &pb.ExecutionTrailCheckpoint{
			NodeId:         cp.NodeID,
			NodeOutputJson: nodeOutputJSON,
			StateAfterJson: stateAfterJSON,
			CreatedAt:      cp.CreatedAt,
		}
	}

	return &pb.GetExecutionTrailResponse{
		InstanceId:     trail.InstanceID,
		Status:         trail.Status,
		CurrentState:   trail.CurrentState,
		ActiveNodeIds:  trail.ActiveNodeIDs,
		Checkpoints:    checkpoints,
	}, nil
}
