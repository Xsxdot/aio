package client

import (
	"context"
	"fmt"

	registryv1 "github.com/xsxdot/aio/api/proto/registry/v1"
)

// RegistryClient 注册服务客户端
type RegistryClient struct {
	manager *GRPCClientManager
}

// NewRegistryClient 创建新的注册服务客户端
func NewRegistryClient(manager *GRPCClientManager) *RegistryClient {
	return &RegistryClient{
		manager: manager,
	}
}

// Register 注册服务实例
func (r *RegistryClient) Register(ctx context.Context, name, address, protocol, env string, metadata map[string]string, weight int32, status string) (*registryv1.ServiceInstance, error) {
	var result *registryv1.ServiceInstance
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.Register(authCtx, &registryv1.RegisterRequest{
			Name:     name,
			Address:  address,
			Protocol: protocol,
			Env:      env,
			Metadata: metadata,
			Weight:   weight,
			Status:   status,
		})
		if err != nil {
			return err
		}
		result = resp.Instance
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("注册服务失败: %v", err)
	}
	return result, nil
}

// Unregister 注销服务实例
func (r *RegistryClient) Unregister(ctx context.Context, serviceID string) (string, error) {
	var result string
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.Unregister(authCtx, &registryv1.UnregisterRequest{
			ServiceId: serviceID,
		})
		if err != nil {
			return err
		}
		result = resp.Message
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("注销服务失败: %v", err)
	}
	return result, nil
}

// Renew 续约服务实例
func (r *RegistryClient) Renew(ctx context.Context, serviceID string) (string, error) {
	var result string
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.Renew(authCtx, &registryv1.RenewRequest{
			ServiceId: serviceID,
		})
		if err != nil {
			return err
		}
		result = resp.Message
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("续约服务失败: %v", err)
	}
	return result, nil
}

// GetService 获取单个服务实例
func (r *RegistryClient) GetService(ctx context.Context, serviceID string) (*registryv1.ServiceInstance, error) {
	var result *registryv1.ServiceInstance
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.GetService(authCtx, &registryv1.GetServiceRequest{
			ServiceId: serviceID,
		})
		if err != nil {
			return err
		}
		result = resp.Instance
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取服务失败: %v", err)
	}
	return result, nil
}

// ListServices 列出所有服务名称
func (r *RegistryClient) ListServices(ctx context.Context) ([]string, error) {
	var result []string
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.ListServices(authCtx, &registryv1.ListServicesRequest{})
		if err != nil {
			return err
		}
		result = resp.Services
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("列出服务失败: %v", err)
	}
	return result, nil
}

// Discover 发现服务实例列表
func (r *RegistryClient) Discover(ctx context.Context, serviceName, env, status, protocol string) ([]*registryv1.ServiceInstance, error) {
	var result []*registryv1.ServiceInstance
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.Discover(authCtx, &registryv1.DiscoverRequest{
			ServiceName: serviceName,
			Env:         env,
			Status:      status,
			Protocol:    protocol,
		})
		if err != nil {
			return err
		}
		result = resp.Instances
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("发现服务失败: %v", err)
	}
	return result, nil
}

// CheckHealth 检查服务健康状态
func (r *RegistryClient) CheckHealth(ctx context.Context, serviceID string) (*registryv1.CheckHealthResponse, error) {
	var result *registryv1.CheckHealthResponse
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.CheckHealth(authCtx, &registryv1.CheckHealthRequest{
			ServiceId: serviceID,
		})
		if err != nil {
			return err
		}
		result = resp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("检查服务健康状态失败: %v", err)
	}
	return result, nil
}

// GetStats 获取注册中心统计信息
func (r *RegistryClient) GetStats(ctx context.Context) (*registryv1.GetStatsResponse, error) {
	var result *registryv1.GetStatsResponse
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.GetStats(authCtx, &registryv1.GetStatsRequest{})
		if err != nil {
			return err
		}
		result = resp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取统计信息失败: %v", err)
	}
	return result, nil
}

// GetServiceStats 获取指定服务的统计信息
func (r *RegistryClient) GetServiceStats(ctx context.Context, serviceName string) (*registryv1.GetServiceStatsResponse, error) {
	var result *registryv1.GetServiceStatsResponse
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.GetServiceStats(authCtx, &registryv1.GetServiceStatsRequest{
			ServiceName: serviceName,
		})
		if err != nil {
			return err
		}
		result = resp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取服务统计信息失败: %v", err)
	}
	return result, nil
}

// GetAllServices 管理员获取所有服务详细信息
func (r *RegistryClient) GetAllServices(ctx context.Context) (*registryv1.GetAllServicesResponse, error) {
	var result *registryv1.GetAllServicesResponse
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.GetAllServices(authCtx, &registryv1.GetAllServicesRequest{})
		if err != nil {
			return err
		}
		result = resp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取所有服务失败: %v", err)
	}
	return result, nil
}

// RemoveAllServiceInstances 管理员删除指定服务的所有实例
func (r *RegistryClient) RemoveAllServiceInstances(ctx context.Context, serviceName string) (*registryv1.RemoveAllServiceInstancesResponse, error) {
	var result *registryv1.RemoveAllServiceInstancesResponse
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.RemoveAllServiceInstances(authCtx, &registryv1.RemoveAllServiceInstancesRequest{
			ServiceName: serviceName,
		})
		if err != nil {
			return err
		}
		result = resp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("删除服务实例失败: %v", err)
	}
	return result, nil
}

// ServiceInstanceBuilder 服务实例构建器
type ServiceInstanceBuilder struct {
	instance *registryv1.RegisterRequest
}

// NewServiceInstanceBuilder 创建服务实例构建器
func NewServiceInstanceBuilder(name, address string) *ServiceInstanceBuilder {
	return &ServiceInstanceBuilder{
		instance: &registryv1.RegisterRequest{
			Name:     name,
			Address:  address,
			Protocol: "http",   // 默认协议
			Env:      "all",    // 默认环境
			Weight:   100,      // 默认权重
			Status:   "active", // 默认状态
			Metadata: make(map[string]string),
		},
	}
}

// WithProtocol 设置协议
func (b *ServiceInstanceBuilder) WithProtocol(protocol string) *ServiceInstanceBuilder {
	b.instance.Protocol = protocol
	return b
}

// WithEnv 设置环境
func (b *ServiceInstanceBuilder) WithEnv(env string) *ServiceInstanceBuilder {
	b.instance.Env = env
	return b
}

// WithWeight 设置权重
func (b *ServiceInstanceBuilder) WithWeight(weight int32) *ServiceInstanceBuilder {
	b.instance.Weight = weight
	return b
}

// WithStatus 设置状态
func (b *ServiceInstanceBuilder) WithStatus(status string) *ServiceInstanceBuilder {
	b.instance.Status = status
	return b
}

// WithMetadata 设置元数据
func (b *ServiceInstanceBuilder) WithMetadata(key, value string) *ServiceInstanceBuilder {
	b.instance.Metadata[key] = value
	return b
}

// WithMetadataMap 设置元数据映射
func (b *ServiceInstanceBuilder) WithMetadataMap(metadata map[string]string) *ServiceInstanceBuilder {
	b.instance.Metadata = metadata
	return b
}

// Build 构建服务实例
func (b *ServiceInstanceBuilder) Build() *registryv1.RegisterRequest {
	return b.instance
}

// Register 注册服务实例
func (b *ServiceInstanceBuilder) Register(ctx context.Context, client *RegistryClient) (*registryv1.ServiceInstance, error) {
	return client.Register(ctx, b.instance.Name, b.instance.Address, b.instance.Protocol,
		b.instance.Env, b.instance.Metadata, b.instance.Weight, b.instance.Status)
}
