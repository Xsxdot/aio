package grpc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/workflow/api/client"
	pb "github.com/xsxdot/aio/system/workflow/api/proto"
	"github.com/xsxdot/aio/system/workflow/internal/app"

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

// GetDef 查询定义，version=0 表示最新版本
func (s *WorkflowService) GetDef(ctx context.Context, req *pb.GetDefRequest) (*pb.GetDefResponse, error) {
	if strings.TrimSpace(req.Code) == "" {
		return nil, status.Error(codes.InvalidArgument, "code 不能为空")
	}
	def, err := s.client.GetDef(ctx, req.Code, req.Version)
	if err != nil {
		s.log.WithErr(err).Error("查询工作流定义失败")
		return nil, status.Error(codes.Internal, err.Error())
	}
	if def == nil {
		return &pb.GetDefResponse{NotFound: true}, nil
	}
	return &pb.GetDefResponse{
		DefId:   def.ID,
		Code:    def.Code,
		Version: def.Version,
		Name:    def.Name,
		DagJson: def.DAGJSON,
	}, nil
}

// ListDefs 分页列出定义
func (s *WorkflowService) ListDefs(ctx context.Context, req *pb.ListDefsRequest) (*pb.ListDefsResponse, error) {
	pageNum, pageSize := req.PageNum, req.PageSize
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.client.ListDefs(ctx, req.CodeLike, pageNum, pageSize)
	if err != nil {
		s.log.WithErr(err).Error("列出工作流定义失败")
		return nil, status.Error(codes.Internal, err.Error())
	}
	pbItems := make([]*pb.GetDefResponse, len(items))
	for i, d := range items {
		pbItems[i] = &pb.GetDefResponse{
			DefId:   d.ID,
			Code:    d.Code,
			Version: d.Version,
			Name:    d.Name,
			DagJson: d.DAGJSON,
		}
	}
	return &pb.ListDefsResponse{Items: pbItems, Total: total}, nil
}

// CreateIfNotExists 幂等创建定义
func (s *WorkflowService) CreateIfNotExists(ctx context.Context, req *pb.CreateIfNotExistsRequest) (*pb.CreateIfNotExistsResponse, error) {
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
	defID, created, err := s.client.CreateIfNotExists(ctx, req.Code, req.Name, req.DagJson, version)
	if err != nil {
		s.log.WithErr(err).Error("幂等创建工作流定义失败")
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateIfNotExistsResponse{DefId: defID, Created: created}, nil
}

// GetInstance 获取实例详情
func (s *WorkflowService) GetInstance(ctx context.Context, req *pb.GetInstanceRequest) (*pb.GetInstanceResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}
	inst, defCode, err := s.client.GetInstance(ctx, req.InstanceId)
	if err != nil {
		s.log.WithErr(err).Error("获取实例失败")
		return nil, status.Error(codes.Internal, err.Error())
	}
	if inst == nil {
		return &pb.GetInstanceResponse{NotFound: true}, nil
	}
	createdAt := ""
	if !inst.CreatedAt.IsZero() {
		createdAt = inst.CreatedAt.Format("2006-01-02 15:04:05")
	}
	return &pb.GetInstanceResponse{
		InstanceId:    inst.ID,
		DefId:         inst.DefID,
		DefCode:       defCode,
		DefVersion:    inst.DefVersion,
		Env:           inst.Env,
		Status:        string(inst.Status),
		InitialState:  inst.InitialState,
		CurrentState:  inst.CurrentState,
		ActiveNodeIds: inst.ActiveNodeIDs,
		CreatedAt:     createdAt,
	}, nil
}

// GetInstanceStatus 轻量查询实例状态
func (s *WorkflowService) GetInstanceStatus(ctx context.Context, req *pb.GetInstanceStatusRequest) (*pb.GetInstanceStatusResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}
	statusStr, err := s.client.GetInstanceStatus(ctx, req.InstanceId)
	if err != nil {
		s.log.WithErr(err).Error("获取实例状态失败")
		return nil, status.Error(codes.Internal, err.Error())
	}
	if statusStr == "" {
		return &pb.GetInstanceStatusResponse{NotFound: true}, nil
	}
	return &pb.GetInstanceStatusResponse{Status: statusStr}, nil
}

// ListInstances 分页列出实例
func (s *WorkflowService) ListInstances(ctx context.Context, req *pb.ListInstancesRequest) (*pb.ListInstancesResponse, error) {
	pageNum, pageSize := req.PageNum, req.PageSize
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	filter := &app.ListInstancesFilter{
		DefCode:       req.DefCode,
		Status:        req.Status,
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
	}
	items, total, err := s.client.ListInstances(ctx, filter, pageNum, pageSize)
	if err != nil {
		s.log.WithErr(err).Error("列出实例失败")
		return nil, status.Error(codes.Internal, err.Error())
	}
	pbItems := make([]*pb.GetInstanceResponse, len(items))
	for i, inst := range items {
		_, defCode, _ := s.client.GetInstance(ctx, inst.ID)
		createdAt := ""
		if !inst.CreatedAt.IsZero() {
			createdAt = inst.CreatedAt.Format("2006-01-02 15:04:05")
		}
		pbItems[i] = &pb.GetInstanceResponse{
			InstanceId:    inst.ID,
			DefId:         inst.DefID,
			DefCode:       defCode,
			DefVersion:    inst.DefVersion,
			Env:           inst.Env,
			Status:        string(inst.Status),
			InitialState:  inst.InitialState,
			CurrentState:  inst.CurrentState,
			ActiveNodeIds: inst.ActiveNodeIDs,
			CreatedAt:     createdAt,
		}
	}
	return &pb.ListInstancesResponse{Items: pbItems, Total: total}, nil
}

// CancelInstance 取消实例
func (s *WorkflowService) CancelInstance(ctx context.Context, req *pb.CancelInstanceRequest) (*pb.CancelInstanceResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}
	err := s.client.CancelInstance(ctx, req.InstanceId)
	if err != nil {
		s.log.WithErr(err).Error("取消实例失败")
		return &pb.CancelInstanceResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.CancelInstanceResponse{Success: true, Message: "取消成功"}, nil
}

// RetryNode 重试失败节点
func (s *WorkflowService) RetryNode(ctx context.Context, req *pb.RetryNodeRequest) (*pb.RetryNodeResponse, error) {
	if req.InstanceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "instance_id 不能为空")
	}
	if strings.TrimSpace(req.NodeId) == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id 不能为空")
	}
	err := s.client.RetryNode(ctx, req.InstanceId, req.NodeId)
	if err != nil {
		s.log.WithErr(err).Error("重试节点失败")
		return &pb.RetryNodeResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.RetryNodeResponse{Success: true, Message: "重试成功"}, nil
}
