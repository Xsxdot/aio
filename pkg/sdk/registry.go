package sdk

import (
	"context"
	"fmt"

	registrypb "xiaozhizhang/system/registry/api/proto"

	"google.golang.org/grpc"
)

// RegistryClient 注册中心客户端
type RegistryClient struct {
	client  *Client
	service registrypb.RegistryServiceClient
}

// newRegistryClient 创建注册中心客户端
func newRegistryClient(client *Client, conn *grpc.ClientConn) (*RegistryClient, error) {
	return &RegistryClient{
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
	Endpoint      string
	Meta          map[string]interface{}
	TTLSeconds    int64
	LastHeartbeat int64
}

// ListServices 获取服务列表
func (rc *RegistryClient) ListServices(ctx context.Context, project, env string) ([]ServiceDescriptor, error) {
	req := &registrypb.ListServicesRequest{
		Project: project,
		Env:     env,
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
			endpoint := InstanceEndpoint{
				ID:            inst.Id,
				InstanceKey:   inst.InstanceKey,
				Env:           inst.Env,
				Host:          inst.Host,
				Endpoint:      inst.Endpoint,
				TTLSeconds:    inst.TtlSeconds,
				LastHeartbeat: inst.LastHeartbeatAt,
			}

			// 解析 meta（简单处理，实际可能需要 JSON 解析）
			endpoint.Meta = make(map[string]interface{})

			desc.Instances = append(desc.Instances, endpoint)
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
		endpoint := InstanceEndpoint{
			ID:            inst.Id,
			InstanceKey:   inst.InstanceKey,
			Env:           inst.Env,
			Host:          inst.Host,
			Endpoint:      inst.Endpoint,
			TTLSeconds:    inst.TtlSeconds,
			LastHeartbeat: inst.LastHeartbeatAt,
		}
		endpoint.Meta = make(map[string]interface{})
		desc.Instances = append(desc.Instances, endpoint)
	}

	return desc, nil
}

// RegisterInstance 注册实例
func (rc *RegistryClient) RegisterInstance(ctx context.Context, req *RegisterInstanceRequest) (*RegisterInstanceResponse, error) {
	grpcReq := &registrypb.RegisterInstanceRequest{
		ServiceId:   req.ServiceID,
		InstanceKey: req.InstanceKey,
		Env:         req.Env,
		Host:        req.Host,
		Endpoint:    req.Endpoint,
		MetaJson:    req.MetaJSON,
		TtlSeconds:  req.TTLSeconds,
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
	ServiceID   int64
	InstanceKey string
	Env         string
	Host        string
	Endpoint    string
	MetaJSON    string
	TTLSeconds  int64
}

// RegisterInstanceResponse 注册实例响应
type RegisterInstanceResponse struct {
	InstanceKey string
	ExpiresAt   int64
}
