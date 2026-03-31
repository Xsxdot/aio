package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	registrypb "github.com/xsxdot/aio/system/registry/api/proto"

	"google.golang.org/grpc"
)

// RegistryClient 注册中心客户端
type RegistryClient struct {
	env     string
	client  *Client
	service registrypb.RegistryServiceClient
}

// newRegistryClient 创建注册中心客户端
func newRegistryClient(client *Client, conn *grpc.ClientConn, env string) (*RegistryClient, error) {
	if strings.TrimSpace(env) == "" {
		env = "dev"
	}
	return &RegistryClient{
		env:     env,
		client:  client,
		service: registrypb.NewRegistryServiceClient(conn),
	}, nil
}

// ServiceDescriptor 服务描述
type ServiceDescriptor struct {
	ID          int64
	Project     string
	Name        string
	Owner       string
	Description string
	Instances   []InstanceEndpoint
}

// InstanceEndpoint 实例端点
type InstanceEndpoint struct {
	ID            int64
	InstanceKey   string
	Env           string
	Host          string
	Endpoint      string              // 默认访问地址（向后兼容）
	Endpoints     []EndpointConfig    // 多端点列表
	HTTPPort      int64               // HTTP 端口（全局）
	GRPCPort      int64               // gRPC 端口（全局）
	Meta          map[string]interface{}
	TTLSeconds    int64
	LastHeartbeat int64
}

// EndpointConfig 单个端点配置
type EndpointConfig struct {
	Host     string `json:"host"`     // 主机地址（不含协议前缀）
	Network  string `json:"network"`  // 网络类型标识：local/internal/external/tailscale
	Priority int    `json:"priority"` // 优先级（数值越小优先级越高）
}

// GetEndpointByNetwork 根据网络类型选择端点
func (ie *InstanceEndpoint) GetEndpointByNetwork(network string) string {
	for _, ep := range ie.Endpoints {
		if ep.Network == network {
			if ie.HTTPPort > 0 {
				return fmt.Sprintf("%s:%d", ep.Host, ie.HTTPPort)
			}
			return ep.Host
		}
	}
	// 回退到默认 Endpoint 或 Host
	if ie.Endpoint != "" {
		return ie.Endpoint
	}
	if ie.HTTPPort > 0 {
		return fmt.Sprintf("%s:%d", ie.Host, ie.HTTPPort)
	}
	return ie.Host
}

// GetGRPCAddress 根据 network 获取 gRPC 地址
func (ie *InstanceEndpoint) GetGRPCAddress(network string) string {
	for _, ep := range ie.Endpoints {
		if ep.Network == network {
			if ie.GRPCPort > 0 {
				return fmt.Sprintf("%s:%d", ep.Host, ie.GRPCPort)
			}
			return ""
		}
	}
	// 无匹配时使用默认 Host
	if ie.GRPCPort > 0 {
		return fmt.Sprintf("%s:%d", ie.Host, ie.GRPCPort)
	}
	return ""
}

// GetAllEndpoints 获取所有端点（用于调试或遍历）
func (ie *InstanceEndpoint) GetAllEndpoints() []string {
	result := make([]string, 0, len(ie.Endpoints))
	for _, ep := range ie.Endpoints {
		if ie.HTTPPort > 0 {
			result = append(result, fmt.Sprintf("%s:%d", ep.Host, ie.HTTPPort))
		} else {
			result = append(result, ep.Host)
		}
	}
	return result
}

// parseInstanceEndpoint 从 proto Instance 解析为 InstanceEndpoint
func parseInstanceEndpoint(inst *registrypb.Instance) InstanceEndpoint {
	ep := InstanceEndpoint{
		ID:            inst.Id,
		InstanceKey:   inst.InstanceKey,
		Env:           inst.Env,
		Host:          inst.Host,
		Endpoint:      inst.Endpoint,
		HTTPPort:      inst.HttpPort,
		GRPCPort:      inst.GrpcPort,
		TTLSeconds:    inst.TtlSeconds,
		LastHeartbeat: inst.LastHeartbeatAt,
		Meta:          make(map[string]interface{}),
	}

	// 解析 endpoints_json
	if inst.EndpointsJson != "" {
		if err := json.Unmarshal([]byte(inst.EndpointsJson), &ep.Endpoints); err != nil {
			// 解析失败不影响整体，忽略
		}
	}

	return ep
}

// ListServices 获取服务列表
func (rc *RegistryClient) ListServices(ctx context.Context, project string) ([]ServiceDescriptor, error) {
	req := &registrypb.ListServicesRequest{
		Project: project,
		Env:     rc.env,
	}

	resp, err := rc.service.ListServices(ctx, req)
	if err != nil {
		return nil, WrapError(err, "list services failed")
	}

	// 转换为 SDK 内部结构
	services := make([]ServiceDescriptor, 0, len(resp.Services))
	for _, svc := range resp.Services {
		desc := ServiceDescriptor{
			ID:          svc.Service.Id,
			Project:     svc.Service.Project,
			Name:        svc.Service.Name,
			Owner:       svc.Service.Owner,
			Description: svc.Service.Description,
			Instances:   make([]InstanceEndpoint, 0, len(svc.Instances)),
		}

		for _, inst := range svc.Instances {
			desc.Instances = append(desc.Instances, parseInstanceEndpoint(inst))
		}

		services = append(services, desc)
	}

	return services, nil
}

// GetServiceByID 根据 ID 获取服务详情
func (rc *RegistryClient) GetServiceByID(ctx context.Context, serviceID int64) (*ServiceDescriptor, error) {
	req := &registrypb.GetServiceByIDRequest{
		Id: serviceID,
	}

	resp, err := rc.service.GetServiceByID(ctx, req)
	if err != nil {
		return nil, WrapError(err, "get service by id failed")
	}

	if resp.Service == nil || resp.Service.Service == nil {
		return nil, fmt.Errorf("service not found")
	}

	svc := resp.Service
	desc := &ServiceDescriptor{
		ID:          svc.Service.Id,
		Project:     svc.Service.Project,
		Name:        svc.Service.Name,
		Owner:       svc.Service.Owner,
		Description: svc.Service.Description,
		Instances:   make([]InstanceEndpoint, 0, len(svc.Instances)),
	}

	for _, inst := range svc.Instances {
		desc.Instances = append(desc.Instances, parseInstanceEndpoint(inst))
	}

	return desc, nil
}

// RegisterInstance 注册实例
func (rc *RegistryClient) RegisterInstance(ctx context.Context, req *RegisterInstanceRequest) (*RegisterInstanceResponse, error) {
	grpcReq := &registrypb.RegisterInstanceRequest{
		ServiceId:     req.ServiceID,
		InstanceKey:   req.InstanceKey,
		Env:           req.Env,
		Host:          req.Host,
		Endpoint:      req.Endpoint,
		EndpointsJson: req.EndpointsJSON,
		HttpPort:      req.HTTPPort,
		GrpcPort:      req.GRPCPort,
		MetaJson:      req.MetaJSON,
		TtlSeconds:    req.TTLSeconds,
	}

	resp, err := rc.service.RegisterInstance(ctx, grpcReq)
	if err != nil {
		return nil, WrapError(err, "register instance failed")
	}

	return &RegisterInstanceResponse{
		InstanceKey: resp.InstanceKey,
		ExpiresAt:   resp.ExpiresAt,
	}, nil
}

// DeregisterInstance 注销实例
func (rc *RegistryClient) DeregisterInstance(ctx context.Context, serviceID int64, instanceKey string) error {
	req := &registrypb.DeregisterInstanceRequest{
		ServiceId:   serviceID,
		InstanceKey: instanceKey,
	}

	_, err := rc.service.DeregisterInstance(ctx, req)
	if err != nil {
		return WrapError(err, "deregister instance failed")
	}

	return nil
}

// RegisterInstanceRequest 注册实例请求
type RegisterInstanceRequest struct {
	ServiceID     int64
	InstanceKey   string
	Env           string
	Host          string
	Endpoint      string
	EndpointsJSON string
	HTTPPort      int64
	GRPCPort      int64
	MetaJSON      string
	TTLSeconds    int64
}

// RegisterInstanceResponse 注册实例响应
type RegisterInstanceResponse struct {
	InstanceKey string
	ExpiresAt   int64
}

// EnsureServiceRequest 确保服务定义存在请求
type EnsureServiceRequest struct {
	Project     string
	Name        string
	Owner       string
	Description string
	SpecJSON    string
}

// EnsureServiceResponse 确保服务定义存在响应
type EnsureServiceResponse struct {
	Service ServiceDescriptor
	Created bool
}

// EnsureService 确保服务定义存在（不存在则创建，存在则返回）
func (rc *RegistryClient) EnsureService(ctx context.Context, req *EnsureServiceRequest) (*EnsureServiceResponse, error) {
	grpcReq := &registrypb.EnsureServiceRequest{
		Project:     req.Project,
		Name:        req.Name,
		Owner:       req.Owner,
		Description: req.Description,
		SpecJson:    req.SpecJSON,
	}

	resp, err := rc.service.EnsureService(ctx, grpcReq)
	if err != nil {
		return nil, WrapError(err, "ensure service failed")
	}

	if resp.Service == nil {
		return nil, fmt.Errorf("service is nil in response")
	}

	svc := resp.Service
	desc := ServiceDescriptor{
		ID:          svc.Id,
		Project:     svc.Project,
		Name:        svc.Name,
		Owner:       svc.Owner,
		Description: svc.Description,
		Instances:   []InstanceEndpoint{}, // EnsureService 不返回实例列表
	}

	return &EnsureServiceResponse{
		Service: desc,
		Created: resp.Created,
	}, nil
}

// RegisterSelfWithEnsureService 一键注册：先确保服务存在，再注册实例并启动心跳
// 这是完整的自注册闭环，调用方无需提前知道 service_id
func (rc *RegistryClient) RegisterSelfWithEnsureService(ctx context.Context, svcReq *EnsureServiceRequest, instReq *RegisterInstanceRequest) (*RegistrationHandle, error) {
	// 1. 确保服务存在
	ensureResp, err := rc.EnsureService(ctx, svcReq)
	if err != nil {
		return nil, fmt.Errorf("ensure service failed: %w", err)
	}

	// 2. 将服务 ID 写入实例注册请求
	instReq.ServiceID = ensureResp.Service.ID

	// 3. 调用现有的 RegisterSelf（会自动启动心跳）
	return rc.RegisterSelf(ctx, instReq)
}