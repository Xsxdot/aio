package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	registryv1 "github.com/xsxdot/aio/api/proto/registry/v1"
)

// ServiceCache 服务缓存信息
type ServiceCache struct {
	Instances []*registryv1.ServiceInstance `json:"instances"`
	UpdatedAt time.Time                     `json:"updated_at"`
	TTL       time.Duration                 `json:"ttl"` // 缓存生存时间
}

// RegistryClient 注册服务客户端
type RegistryClient struct {
	manager      *GRPCClientManager
	serviceCache map[string]map[string]*ServiceCache // [serviceName][env] -> ServiceCache
	mutex        sync.RWMutex                        // 保护服务缓存的读写锁
	watchCtx     context.Context                     // Watch上下文
	watchCancel  context.CancelFunc                  // 取消Watch
	cacheTTL     time.Duration                       // 默认缓存生存时间
}

// NewRegistryClient 创建新的注册服务客户端
func NewRegistryClient(manager *GRPCClientManager) *RegistryClient {
	return &RegistryClient{
		manager:      manager,
		serviceCache: make(map[string]map[string]*ServiceCache),
		cacheTTL:     5 * time.Minute, // 默认5分钟缓存时间
	}
}

// NewRegistryClientWithTTL 创建新的注册服务客户端，指定缓存TTL
func NewRegistryClientWithTTL(manager *GRPCClientManager, ttl time.Duration) *RegistryClient {
	return &RegistryClient{
		manager:      manager,
		serviceCache: make(map[string]map[string]*ServiceCache),
		cacheTTL:     ttl,
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

// Unregister 注销服务实例（物理删除）
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

// Offline 下线服务实例（逻辑删除，保留记录）
func (r *RegistryClient) Offline(ctx context.Context, serviceID string) (*registryv1.ServiceInstance, error) {
	var result *registryv1.ServiceInstance
	err := r.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		resp, err := client.Offline(authCtx, &registryv1.OfflineRequest{
			ServiceId: serviceID,
		})
		if err != nil {
			return err
		}
		result = resp.Instance
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("下线服务失败: %v", err)
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

// Discover 发现服务实例列表，支持智能缓存
// forceRefresh: 是否强制刷新缓存，忽略TTL
func (r *RegistryClient) Discover(ctx context.Context, serviceName, env, status, protocol string) ([]*registryv1.ServiceInstance, error) {
	return r.DiscoverWithOptions(ctx, serviceName, env, status, protocol, false)
}

// DiscoverWithOptions 发现服务实例列表，支持更多选项
func (r *RegistryClient) DiscoverWithOptions(ctx context.Context, serviceName, env, status, protocol string, forceRefresh bool) ([]*registryv1.ServiceInstance, error) {
	// 1. 检查缓存（如果不是强制刷新且没有额外过滤条件）
	if !forceRefresh && status == "" && protocol == "" {
		if cachedInstances, valid := r.getCachedServicesIfValid(serviceName, env); valid {
			// 应用过滤条件（如果有）
			return r.filterInstances(cachedInstances, status, protocol), nil
		}
	}

	// 2. 从远程获取数据
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

	// 3. 如果没有额外的过滤条件，将完整结果保存到缓存
	if status == "" && protocol == "" && len(result) > 0 {
		r.updateServiceCacheWithTTL(serviceName, env, result, r.cacheTTL)
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
			Protocol: "http", // 默认协议
			Env:      "all",  // 默认环境
			Weight:   100,    // 默认权重
			Status:   "up",   // 默认状态（在线）
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

// updateServiceCache 更新服务缓存（使用默认TTL）
func (r *RegistryClient) updateServiceCache(serviceName, env string, instances []*registryv1.ServiceInstance) {
	r.updateServiceCacheWithTTL(serviceName, env, instances, r.cacheTTL)
}

// updateServiceCacheWithTTL 更新服务缓存，指定TTL
func (r *RegistryClient) updateServiceCacheWithTTL(serviceName, env string, instances []*registryv1.ServiceInstance, ttl time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.serviceCache[serviceName] == nil {
		r.serviceCache[serviceName] = make(map[string]*ServiceCache)
	}

	r.serviceCache[serviceName][env] = &ServiceCache{
		Instances: instances,
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}
}

// GetCachedServices 获取缓存的服务实例
func (r *RegistryClient) GetCachedServices(serviceName, env string) ([]*registryv1.ServiceInstance, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if envMap, exists := r.serviceCache[serviceName]; exists {
		if cache, exists := envMap[env]; exists {
			return cache.Instances, true
		}
	}
	return nil, false
}

// GetAllCachedServices 获取所有缓存的服务
func (r *RegistryClient) GetAllCachedServices() map[string]map[string]*ServiceCache {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 深拷贝缓存数据
	result := make(map[string]map[string]*ServiceCache)
	for serviceName, envMap := range r.serviceCache {
		result[serviceName] = make(map[string]*ServiceCache)
		for env, cache := range envMap {
			// 拷贝实例列表
			instances := make([]*registryv1.ServiceInstance, len(cache.Instances))
			copy(instances, cache.Instances)
			result[serviceName][env] = &ServiceCache{
				Instances: instances,
				UpdatedAt: cache.UpdatedAt,
			}
		}
	}
	return result
}

// ClearCache 清除指定服务的缓存
func (r *RegistryClient) ClearCache(serviceName, env string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if env == "" {
		// 清除指定服务的所有环境缓存
		delete(r.serviceCache, serviceName)
	} else if envMap, exists := r.serviceCache[serviceName]; exists {
		// 清除指定环境的缓存
		delete(envMap, env)
		// 如果该服务没有任何环境缓存了，删除整个服务条目
		if len(envMap) == 0 {
			delete(r.serviceCache, serviceName)
		}
	}
}

// Watch 监听服务变化
func (r *RegistryClient) Watch(ctx context.Context, serviceName, env string, onUpdate func(*registryv1.WatchResponse)) error {
	// 如果有正在运行的Watch，先取消
	if r.watchCancel != nil {
		r.watchCancel()
	}

	// 创建新的Watch上下文
	r.watchCtx, r.watchCancel = context.WithCancel(ctx)

	return r.manager.ExecuteWithRetry(r.watchCtx, func(authCtx context.Context) error {
		client := r.manager.GetRegistryClient()
		stream, err := client.Watch(authCtx, &registryv1.WatchRequest{
			ServiceName: serviceName,
			Env:         env,
		})
		if err != nil {
			return fmt.Errorf("启动Watch失败: %v", err)
		}

		// 在goroutine中处理流式响应
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Watch goroutine panic: %v\n", r)
				}
			}()

			for {
				select {
				case <-r.watchCtx.Done():
					return
				default:
					resp, err := stream.Recv()
					if err != nil {
						fmt.Printf("Watch接收错误: %v\n", err)
						return
					}

					// 更新缓存
					r.handleWatchEvent(resp)

					// 调用用户回调
					if onUpdate != nil {
						onUpdate(resp)
					}
				}
			}
		}()

		return nil
	})
}

// handleWatchEvent 处理Watch事件，更新缓存
func (r *RegistryClient) handleWatchEvent(resp *registryv1.WatchResponse) {
	if resp.Instance == nil {
		return
	}

	instance := resp.Instance
	serviceName := instance.Name
	env := instance.Env

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 确保缓存map存在
	if r.serviceCache[serviceName] == nil {
		r.serviceCache[serviceName] = make(map[string]*ServiceCache)
	}
	if r.serviceCache[serviceName][env] == nil {
		r.serviceCache[serviceName][env] = &ServiceCache{
			Instances: []*registryv1.ServiceInstance{},
			UpdatedAt: time.Now(),
		}
	}

	cache := r.serviceCache[serviceName][env]

	switch resp.EventType {
	case registryv1.WatchResponse_ADDED:
		// 添加新实例
		cache.Instances = append(cache.Instances, instance)
	case registryv1.WatchResponse_MODIFIED:
		// 修改现有实例
		for i, existing := range cache.Instances {
			if existing.Id == instance.Id {
				cache.Instances[i] = instance
				break
			}
		}
	case registryv1.WatchResponse_DELETED:
		// 删除实例
		for i, existing := range cache.Instances {
			if existing.Id == instance.Id {
				cache.Instances = append(cache.Instances[:i], cache.Instances[i+1:]...)
				break
			}
		}
	}

	cache.UpdatedAt = time.Now()
}

// StopWatch 停止监听
func (r *RegistryClient) StopWatch() {
	if r.watchCancel != nil {
		r.watchCancel()
		r.watchCancel = nil
	}
}

// getCachedServicesIfValid 获取有效的缓存服务实例
func (r *RegistryClient) getCachedServicesIfValid(serviceName, env string) ([]*registryv1.ServiceInstance, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if envMap, exists := r.serviceCache[serviceName]; exists {
		if cache, exists := envMap[env]; exists {
			// 检查缓存是否过期
			if time.Since(cache.UpdatedAt) < cache.TTL {
				// 返回副本，避免数据竞争
				instances := make([]*registryv1.ServiceInstance, len(cache.Instances))
				copy(instances, cache.Instances)
				return instances, true
			}
		}
	}
	return nil, false
}

// filterInstances 根据条件过滤服务实例
func (r *RegistryClient) filterInstances(instances []*registryv1.ServiceInstance, status, protocol string) []*registryv1.ServiceInstance {
	if status == "" && protocol == "" {
		return instances
	}

	var filtered []*registryv1.ServiceInstance
	for _, instance := range instances {
		// 检查状态过滤条件
		if status != "" && instance.Status != status {
			continue
		}
		// 检查协议过滤条件
		if protocol != "" && instance.Protocol != protocol {
			continue
		}
		filtered = append(filtered, instance)
	}
	return filtered
}

// ForceRefreshCache 强制刷新指定服务的缓存
func (r *RegistryClient) ForceRefreshCache(ctx context.Context, serviceName, env string) ([]*registryv1.ServiceInstance, error) {
	return r.DiscoverWithOptions(ctx, serviceName, env, "", "", true)
}

// IsCacheValid 检查缓存是否有效
func (r *RegistryClient) IsCacheValid(serviceName, env string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if envMap, exists := r.serviceCache[serviceName]; exists {
		if cache, exists := envMap[env]; exists {
			return time.Since(cache.UpdatedAt) < cache.TTL
		}
	}
	return false
}

// GetCacheInfo 获取缓存信息
func (r *RegistryClient) GetCacheInfo(serviceName, env string) (int, time.Time, time.Duration, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if envMap, exists := r.serviceCache[serviceName]; exists {
		if cache, exists := envMap[env]; exists {
			return len(cache.Instances), cache.UpdatedAt, cache.TTL, true
		}
	}
	return 0, time.Time{}, 0, false
}

// SetCacheTTL 设置缓存TTL
func (r *RegistryClient) SetCacheTTL(ttl time.Duration) {
	r.cacheTTL = ttl
}
