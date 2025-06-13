package registry

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	registryv1 "github.com/xsxdot/aio/api/proto/registry/v1"
)

// GRPCService Registry gRPC 服务实现
type GRPCService struct {
	registryv1.UnimplementedRegistryServiceServer
	registry Registry
}

// NewGRPCService 创建新的 Registry gRPC 服务
func NewGRPCService(reg Registry) *GRPCService {
	return &GRPCService{
		registry: reg,
	}
}

// RegisterService 注册服务到 gRPC 服务器
func (s *GRPCService) RegisterService(server *grpc.Server) error {
	registryv1.RegisterRegistryServiceServer(server, s)
	return nil
}

// ServiceName 返回服务名称
func (s *GRPCService) ServiceName() string {
	return "registry.v1.RegistryService"
}

// ServiceVersion 返回服务版本
func (s *GRPCService) ServiceVersion() string {
	return "v1.0.0"
}

// Register 注册服务实例
func (s *GRPCService) Register(ctx context.Context, req *registryv1.RegisterRequest) (*registryv1.RegisterResponse, error) {
	// 参数验证
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "服务名称不能为空")
	}
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "服务地址不能为空")
	}

	// 构建服务实例
	instance := &ServiceInstance{
		Name:     req.Name,
		Address:  req.Address,
		Protocol: req.Protocol,
		Env:      ParseEnvironment(req.Env),
		Metadata: req.Metadata,
		Weight:   int(req.Weight),
		Status:   req.Status,
	}

	// 设置默认值
	if instance.Protocol == "" {
		instance.Protocol = "http"
	}
	if instance.Weight == 0 {
		instance.Weight = 100
	}
	if instance.Status == "" {
		instance.Status = "active"
	}
	// env的默认值将在注册中心的Register方法中处理

	// 注册服务实例
	err := s.registry.Register(ctx, instance)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "注册服务失败: %v", err)
	}

	// 返回注册结果
	return &registryv1.RegisterResponse{
		Instance: s.serviceInstanceToProto(instance),
	}, nil
}

// Unregister 注销服务实例
func (s *GRPCService) Unregister(ctx context.Context, req *registryv1.UnregisterRequest) (*registryv1.UnregisterResponse, error) {
	if req.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "服务实例ID不能为空")
	}

	err := s.registry.Unregister(ctx, req.ServiceId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "服务实例不存在")
		}
		return nil, status.Errorf(codes.Internal, "注销服务失败: %v", err)
	}

	return &registryv1.UnregisterResponse{
		Message: "服务注销成功",
	}, nil
}

// Renew 续约服务实例
func (s *GRPCService) Renew(ctx context.Context, req *registryv1.RenewRequest) (*registryv1.RenewResponse, error) {
	if req.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "服务实例ID不能为空")
	}

	err := s.registry.Renew(ctx, req.ServiceId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "服务实例不存在")
		}
		return nil, status.Errorf(codes.Internal, "续约服务失败: %v", err)
	}

	return &registryv1.RenewResponse{
		Message: "服务续约成功",
	}, nil
}

// GetService 获取单个服务实例
func (s *GRPCService) GetService(ctx context.Context, req *registryv1.GetServiceRequest) (*registryv1.GetServiceResponse, error) {
	if req.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "服务实例ID不能为空")
	}

	instance, err := s.registry.GetService(ctx, req.ServiceId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "服务实例不存在")
		}
		return nil, status.Errorf(codes.Internal, "获取服务失败: %v", err)
	}

	return &registryv1.GetServiceResponse{
		Instance: s.serviceInstanceToProto(instance),
	}, nil
}

// ListServices 列出所有服务名称
func (s *GRPCService) ListServices(ctx context.Context, req *registryv1.ListServicesRequest) (*registryv1.ListServicesResponse, error) {
	services, err := s.registry.ListServices(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "列出服务失败: %v", err)
	}

	return &registryv1.ListServicesResponse{
		Services: services,
	}, nil
}

// Discover 发现服务实例列表
func (s *GRPCService) Discover(ctx context.Context, req *registryv1.DiscoverRequest) (*registryv1.DiscoverResponse, error) {
	if req.ServiceName == "" {
		return nil, status.Error(codes.InvalidArgument, "服务名称不能为空")
	}

	var instances []*ServiceInstance
	var err error

	// 根据是否指定环境使用不同的发现方法
	if req.Env != "" {
		instances, err = s.registry.DiscoverByEnv(ctx, req.ServiceName, ParseEnvironment(req.Env))
	} else {
		instances, err = s.registry.Discover(ctx, req.ServiceName)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "发现服务失败: %v", err)
	}

	// 过滤实例（按状态和协议）
	filteredInstances := s.filterInstances(instances, req.Status, req.Protocol)

	// 转换为 proto 格式
	protoInstances := make([]*registryv1.ServiceInstance, len(filteredInstances))
	for i, instance := range filteredInstances {
		protoInstances[i] = s.serviceInstanceToProto(instance)
	}

	return &registryv1.DiscoverResponse{
		Instances: protoInstances,
	}, nil
}

// CheckHealth 检查服务健康状态
func (s *GRPCService) CheckHealth(ctx context.Context, req *registryv1.CheckHealthRequest) (*registryv1.CheckHealthResponse, error) {
	if req.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "服务实例ID不能为空")
	}

	instance, err := s.registry.GetService(ctx, req.ServiceId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "服务实例不存在")
		}
		return nil, status.Errorf(codes.Internal, "获取服务失败: %v", err)
	}

	// 计算运行时长和注册时长
	uptime := instance.GetUptime()
	registerDuration := instance.GetRegisterDuration()

	return &registryv1.CheckHealthResponse{
		ServiceId:        instance.ID,
		ServiceName:      instance.Name,
		Status:           instance.Status,
		Healthy:          instance.IsHealthy(),
		Uptime:           uptime.String(),
		RegisterDuration: registerDuration.String(),
		LastCheck:        time.Now().Unix(),
	}, nil
}

// GetStats 获取注册中心统计信息
func (s *GRPCService) GetStats(ctx context.Context, req *registryv1.GetStatsRequest) (*registryv1.GetStatsResponse, error) {
	// 获取所有服务
	serviceNames, err := s.registry.ListServices(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取服务列表失败: %v", err)
	}

	// 统计信息
	totalServices := int32(len(serviceNames))
	totalInstances := int32(0)
	healthyInstances := int32(0)
	unhealthyInstances := int32(0)
	serviceStats := make(map[string]int32)

	// 遍历每个服务，统计实例数量
	for _, serviceName := range serviceNames {
		instances, err := s.registry.Discover(ctx, serviceName)
		if err != nil {
			continue
		}

		serviceStats[serviceName] = int32(len(instances))
		totalInstances += int32(len(instances))

		// 统计健康状态
		for _, instance := range instances {
			if instance.IsHealthy() {
				healthyInstances++
			} else {
				unhealthyInstances++
			}
		}
	}

	return &registryv1.GetStatsResponse{
		TotalServices:      totalServices,
		TotalInstances:     totalInstances,
		HealthyInstances:   healthyInstances,
		UnhealthyInstances: unhealthyInstances,
		ServiceStats:       serviceStats,
		Timestamp:          time.Now().Unix(),
	}, nil
}

// GetServiceStats 获取指定服务的统计信息
func (s *GRPCService) GetServiceStats(ctx context.Context, req *registryv1.GetServiceStatsRequest) (*registryv1.GetServiceStatsResponse, error) {
	if req.ServiceName == "" {
		return nil, status.Error(codes.InvalidArgument, "服务名称不能为空")
	}

	instances, err := s.registry.Discover(ctx, req.ServiceName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取服务实例失败: %v", err)
	}

	// 统计信息
	totalInstances := int32(len(instances))
	healthyInstances := int32(0)
	unhealthyInstances := int32(0)
	protocols := make(map[string]int32)
	statuses := make(map[string]int32)

	// 转换为 proto 格式并统计
	protoInstances := make([]*registryv1.ServiceInstance, len(instances))
	for i, instance := range instances {
		protoInstances[i] = s.serviceInstanceToProto(instance)

		// 统计健康状态
		if instance.IsHealthy() {
			healthyInstances++
		} else {
			unhealthyInstances++
		}

		// 统计协议分布
		protocols[instance.Protocol]++

		// 统计状态分布
		statuses[instance.Status]++
	}

	return &registryv1.GetServiceStatsResponse{
		ServiceName:        req.ServiceName,
		TotalInstances:     totalInstances,
		HealthyInstances:   healthyInstances,
		UnhealthyInstances: unhealthyInstances,
		Protocols:          protocols,
		Statuses:           statuses,
		Instances:          protoInstances,
		Timestamp:          time.Now().Unix(),
	}, nil
}

// GetAllServices 管理员获取所有服务详细信息
func (s *GRPCService) GetAllServices(ctx context.Context, req *registryv1.GetAllServicesRequest) (*registryv1.GetAllServicesResponse, error) {
	// 获取所有服务名称
	serviceNames, err := s.registry.ListServices(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取服务列表失败: %v", err)
	}

	services := make(map[string]*registryv1.ServiceInstanceList)

	// 获取每个服务的实例列表
	for _, serviceName := range serviceNames {
		instances, err := s.registry.Discover(ctx, serviceName)
		if err != nil {
			continue
		}

		// 转换为 proto 格式
		protoInstances := make([]*registryv1.ServiceInstance, len(instances))
		for i, instance := range instances {
			protoInstances[i] = s.serviceInstanceToProto(instance)
		}

		services[serviceName] = &registryv1.ServiceInstanceList{
			Instances: protoInstances,
		}
	}

	return &registryv1.GetAllServicesResponse{
		Services: services,
	}, nil
}

// RemoveAllServiceInstances 管理员删除指定服务的所有实例
func (s *GRPCService) RemoveAllServiceInstances(ctx context.Context, req *registryv1.RemoveAllServiceInstancesRequest) (*registryv1.RemoveAllServiceInstancesResponse, error) {
	if req.ServiceName == "" {
		return nil, status.Error(codes.InvalidArgument, "服务名称不能为空")
	}

	// 获取服务的所有实例
	instances, err := s.registry.Discover(ctx, req.ServiceName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取服务实例失败: %v", err)
	}

	totalInstances := int32(len(instances))
	removedCount := int32(0)
	var errors []string

	// 删除每个实例
	for _, instance := range instances {
		err := s.registry.Unregister(ctx, instance.ID)
		if err != nil {
			errors = append(errors, err.Error())
		} else {
			removedCount++
		}
	}

	return &registryv1.RemoveAllServiceInstancesResponse{
		ServiceName:    req.ServiceName,
		TotalInstances: totalInstances,
		RemovedCount:   removedCount,
		Errors:         errors,
	}, nil
}

// serviceInstanceToProto 将 ServiceInstance 转换为 proto 格式
func (s *GRPCService) serviceInstanceToProto(instance *ServiceInstance) *registryv1.ServiceInstance {
	return &registryv1.ServiceInstance{
		Id:           instance.ID,
		Name:         instance.Name,
		Address:      instance.Address,
		Protocol:     instance.Protocol,
		Env:          instance.Env.String(),
		RegisterTime: instance.RegisterTime.Unix(),
		StartTime:    instance.StartTime.Unix(),
		Metadata:     instance.Metadata,
		Weight:       int32(instance.Weight),
		Status:       instance.Status,
	}
}

// filterInstances 过滤服务实例
func (s *GRPCService) filterInstances(instances []*ServiceInstance, status, protocol string) []*ServiceInstance {
	if status == "" && protocol == "" {
		return instances
	}

	var filtered []*ServiceInstance
	for _, instance := range instances {
		// 状态过滤
		if status != "" && instance.Status != status {
			continue
		}

		// 协议过滤
		if protocol != "" && instance.Protocol != protocol {
			continue
		}

		filtered = append(filtered, instance)
	}

	return filtered
}
