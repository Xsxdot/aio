package grpc

import (
	"context"
	"encoding/json"
	"io"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/registry/api/client"
	"xiaozhizhang/system/registry/api/dto"
	pb "xiaozhizhang/system/registry/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegistryService gRPC 注册中心服务实现
type RegistryService struct {
	pb.UnimplementedRegistryServiceServer
	client *client.RegistryClient
	err    *errorc.ErrorBuilder
	log    *logger.Log
}

// NewRegistryService 创建注册中心服务实例
func NewRegistryService(registryClient *client.RegistryClient, log *logger.Log) *RegistryService {
	return &RegistryService{
		client: registryClient,
		err:    errorc.NewErrorBuilder("RegistryGRPCService"),
		log:    log.WithEntryName("RegistryGRPCService"),
	}
}

// ServiceName 返回服务名称
func (s *RegistryService) ServiceName() string {
	return "registry.v1.RegistryService"
}

// ServiceVersion 返回服务版本
func (s *RegistryService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册服务到 gRPC 服务器
func (s *RegistryService) RegisterService(server *grpc.Server) error {
	pb.RegisterRegistryServiceServer(server, s)
	return nil
}

// =============== 实例注册/注销 ===============

// RegisterInstance 注册实例（上线）
func (s *RegistryService) RegisterInstance(ctx context.Context, req *pb.RegisterInstanceRequest) (*pb.RegisterInstanceResponse, error) {
	// 参数校验
	if req.ServiceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "service_id 不能为空")
	}
	if req.Host == "" {
		return nil, status.Error(codes.InvalidArgument, "host 不能为空")
	}
	if req.Endpoint == "" {
		return nil, status.Error(codes.InvalidArgument, "endpoint 不能为空")
	}

	// 转换为 DTO
	dtoReq := &dto.RegisterInstanceReq{
		ServiceID:   req.ServiceId,
		InstanceKey: req.InstanceKey,
		Env:         req.Env,
		Host:        req.Host,
		Endpoint:    req.Endpoint,
		Meta:        parseMetaJSON(req.MetaJson),
		TTLSeconds:  req.TtlSeconds,
	}

	// 调用 client
	resp, err := s.client.RegisterInstance(ctx, dtoReq)
	if err != nil {
		s.log.WithErr(err).WithField("service_id", req.ServiceId).Error("注册实例失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.RegisterInstanceResponse{
		InstanceKey: resp.InstanceKey,
		ExpiresAt:   resp.ExpiresAt.Unix(),
	}, nil
}

// DeregisterInstance 注销实例（下线）
func (s *RegistryService) DeregisterInstance(ctx context.Context, req *pb.DeregisterInstanceRequest) (*pb.DeregisterInstanceResponse, error) {
	// 参数校验
	if req.ServiceId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "service_id 不能为空")
	}
	if req.InstanceKey == "" {
		return nil, status.Error(codes.InvalidArgument, "instance_key 不能为空")
	}

	// 转换为 DTO
	dtoReq := &dto.DeregisterInstanceReq{
		ServiceID:   req.ServiceId,
		InstanceKey: req.InstanceKey,
	}

	// 调用 client
	err := s.client.DeregisterInstance(ctx, dtoReq)
	if err != nil {
		s.log.WithErr(err).WithField("service_id", req.ServiceId).WithField("instance_key", req.InstanceKey).Error("注销实例失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.DeregisterInstanceResponse{
		Success: true,
		Message: "实例已下线",
	}, nil
}

// =============== 心跳流 ===============

// HeartbeatStream 心跳流（双向流）
func (s *RegistryService) HeartbeatStream(stream grpc.BidiStreamingServer[pb.HeartbeatRequest, pb.HeartbeatResponse]) error {
	ctx := stream.Context()

	for {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			s.log.Debug("心跳流：上下文已取消")
			return ctx.Err()
		default:
		}

		// 接收心跳请求
		req, err := stream.Recv()
		if err == io.EOF {
			s.log.Debug("心跳流：客户端已关闭流")
			return nil
		}
		if err != nil {
			s.log.WithErr(err).Error("心跳流：接收消息失败")
			return status.Error(codes.Internal, "接收心跳消息失败")
		}

		// 参数校验
		if req.ServiceId <= 0 {
			if err := stream.Send(&pb.HeartbeatResponse{ExpiresAt: 0}); err != nil {
				return err
			}
			continue
		}
		if req.InstanceKey == "" {
			if err := stream.Send(&pb.HeartbeatResponse{ExpiresAt: 0}); err != nil {
				return err
			}
			continue
		}

		// 转换为 DTO
		dtoReq := &dto.HeartbeatReq{
			ServiceID:   req.ServiceId,
			InstanceKey: req.InstanceKey,
		}

		// 调用 client
		resp, err := s.client.HeartbeatInstance(ctx, dtoReq)
		if err != nil {
			s.log.WithErr(err).WithField("service_id", req.ServiceId).WithField("instance_key", req.InstanceKey).Warn("心跳失败")
			// 心跳失败时仍然返回响应，但 expires_at 为 0
			if err := stream.Send(&pb.HeartbeatResponse{ExpiresAt: 0}); err != nil {
				return err
			}
			continue
		}

		// 发送响应
		if err := stream.Send(&pb.HeartbeatResponse{
			ExpiresAt: resp.ExpiresAt.Unix(),
		}); err != nil {
			s.log.WithErr(err).Error("心跳流：发送响应失败")
			return err
		}
	}
}

// =============== 服务查询 ===============

// ListServices 获取服务列表（包含在线实例）
func (s *RegistryService) ListServices(ctx context.Context, req *pb.ListServicesRequest) (*pb.ListServicesResponse, error) {
	// 调用 client
	services, err := s.client.ListServices(ctx, req.Project, req.Env)
	if err != nil {
		s.log.WithErr(err).Error("获取服务列表失败")
		return nil, convertToGRPCError(err)
	}

	// 转换为 proto
	pbServices := make([]*pb.ServiceWithInstances, 0, len(services))
	for _, svc := range services {
		pbServices = append(pbServices, convertToProtoServiceWithInstances(svc))
	}

	return &pb.ListServicesResponse{
		Services: pbServices,
		Total:    int32(len(pbServices)),
	}, nil
}

// GetServiceByID 根据 ID 获取服务详情（包含在线实例）
func (s *RegistryService) GetServiceByID(ctx context.Context, req *pb.GetServiceByIDRequest) (*pb.GetServiceByIDResponse, error) {
	// 参数校验
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id 不能为空")
	}

	// 调用 client
	svc, err := s.client.GetServiceByID(ctx, req.Id)
	if err != nil {
		s.log.WithErr(err).WithField("id", req.Id).Error("获取服务详情失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.GetServiceByIDResponse{
		Service: convertToProtoServiceWithInstances(svc),
	}, nil
}

// EnsureService 确保服务定义存在（不存在则创建，存在则返回）
func (s *RegistryService) EnsureService(ctx context.Context, req *pb.EnsureServiceRequest) (*pb.EnsureServiceResponse, error) {
	// 参数校验
	if req.Project == "" {
		return nil, status.Error(codes.InvalidArgument, "project 不能为空")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	// 解析 spec_json
	spec := parseMetaJSON(req.SpecJson)

	// 转换为 DTO
	dtoReq := &dto.CreateServiceReq{
		Project:     req.Project,
		Name:        req.Name,
		Owner:       req.Owner,
		Description: req.Description,
		Spec:        spec,
	}

	// 调用 client.EnsureService
	svcDTO, created, err := s.client.EnsureService(ctx, dtoReq)
	if err != nil {
		s.log.WithErr(err).
			WithField("project", req.Project).
			WithField("name", req.Name).
			Error("确保服务存在失败")
		return nil, convertToGRPCError(err)
	}

	// 转换为 proto Service
	pbService := &pb.Service{
		Id:          svcDTO.ID,
		Project:     svcDTO.Project,
		Name:        svcDTO.Name,
		Owner:       svcDTO.Owner,
		Description: svcDTO.Description,
		SpecJson:    toJSONString(svcDTO.Spec),
		CreatedAt:   svcDTO.CreatedAt.Unix(),
		UpdatedAt:   svcDTO.UpdatedAt.Unix(),
	}

	return &pb.EnsureServiceResponse{
		Service: pbService,
		Created: created,
	}, nil
}

// =============== 辅助函数 ===============

// parseMetaJSON 解析 meta JSON 字符串
func parseMetaJSON(metaJSON string) map[string]interface{} {
	if metaJSON == "" {
		return nil
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil
	}
	return meta
}

// toJSONString 将 map 转为 JSON 字符串
func toJSONString(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// convertToProtoServiceWithInstances 转换 DTO 为 proto ServiceWithInstances
func convertToProtoServiceWithInstances(svc *dto.ServiceWithInstancesDTO) *pb.ServiceWithInstances {
	if svc == nil {
		return nil
	}

	result := &pb.ServiceWithInstances{}

	// 转换 Service
	if svc.Service != nil {
		result.Service = &pb.Service{
			Id:          svc.Service.ID,
			Project:     svc.Service.Project,
			Name:        svc.Service.Name,
			Owner:       svc.Service.Owner,
			Description: svc.Service.Description,
			SpecJson:    toJSONString(svc.Service.Spec),
			CreatedAt:   svc.Service.CreatedAt.Unix(),
			UpdatedAt:   svc.Service.UpdatedAt.Unix(),
		}
	}

	// 转换 Instances
	if svc.Instances != nil {
		result.Instances = make([]*pb.Instance, 0, len(svc.Instances))
		for _, inst := range svc.Instances {
			if inst == nil {
				continue
			}
			result.Instances = append(result.Instances, &pb.Instance{
				Id:              inst.ID,
				ServiceId:       inst.ServiceID,
				InstanceKey:     inst.InstanceKey,
				Env:             inst.Env,
				Host:            inst.Host,
				Endpoint:        inst.Endpoint,
				MetaJson:        toJSONString(inst.Meta),
				TtlSeconds:      inst.TTLSeconds,
				LastHeartbeatAt: inst.LastHeartbeatAt.Unix(),
				CreatedAt:       inst.CreatedAt.Unix(),
				UpdatedAt:       inst.UpdatedAt.Unix(),
			})
		}
	}

	return result
}

// convertToGRPCError 转换业务错误为 gRPC 错误
func convertToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// 检查是否是 NotFound 错误
	if errorc.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}

	// 尝试解析为自定义 Error 类型，检查 ErrorCode
	customErr := errorc.ParseError(err)
	if customErr != nil && customErr.ErrorCode != nil {
		switch customErr.ErrorCode {
		case errorc.ErrorCodeValid:
			return status.Error(codes.InvalidArgument, err.Error())
		case errorc.ErrorCodeNoAuth:
			return status.Error(codes.Unauthenticated, err.Error())
		case errorc.ErrorCodeForbidden:
			return status.Error(codes.PermissionDenied, err.Error())
		case errorc.ErrorCodeNotFound:
			return status.Error(codes.NotFound, err.Error())
		case errorc.ErrorCodeDB, errorc.ErrorCodeThird:
			return status.Error(codes.Internal, err.Error())
		}
	}

	// 默认返回内部错误
	return status.Error(codes.Internal, err.Error())
}
