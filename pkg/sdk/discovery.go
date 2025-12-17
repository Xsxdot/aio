package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DiscoveryClient 服务发现客户端
type DiscoveryClient struct {
	client *Client

	// 实例缓存
	mu       sync.RWMutex
	services map[string]*serviceCache // key: project/name

	// 故障实例追踪
	failedInstances map[string]*instanceFailure // key: endpoint
}

// serviceCache 服务缓存
type serviceCache struct {
	descriptor  *ServiceDescriptor
	lastRefresh time.Time
	nextIndex   int // round-robin 索引
}

// instanceFailure 实例故障记录
type instanceFailure struct {
	endpoint      string
	failureCount  int
	lastFailTime  time.Time
	cooldownUntil time.Time
}

// newDiscoveryClient 创建服务发现客户端
func newDiscoveryClient(client *Client) *DiscoveryClient {
	return &DiscoveryClient{
		client:          client,
		services:        make(map[string]*serviceCache),
		failedInstances: make(map[string]*instanceFailure),
	}
}

// Resolve 解析服务实例列表
func (dc *DiscoveryClient) Resolve(ctx context.Context, project, serviceName, env string) ([]InstanceEndpoint, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("%s/%s", project, serviceName)

	dc.mu.RLock()
	cache, exists := dc.services[cacheKey]
	if exists && time.Since(cache.lastRefresh) < 30*time.Second {
		// 缓存有效
		instances := cache.descriptor.Instances
		dc.mu.RUnlock()
		return instances, nil
	}
	dc.mu.RUnlock()

	// 从注册中心刷新
	services, err := dc.client.Registry.ListServices(ctx, project, env)
	if err != nil {
		return nil, err
	}

	// 查找匹配的服务
	var targetService *ServiceDescriptor
	for i := range services {
		if services[i].Name == serviceName {
			targetService = &services[i]
			break
		}
	}

	if targetService == nil {
		return nil, fmt.Errorf("service %s/%s not found", project, serviceName)
	}

	// 更新缓存
	dc.mu.Lock()
	dc.services[cacheKey] = &serviceCache{
		descriptor:  targetService,
		lastRefresh: time.Now(),
		nextIndex:   0,
	}
	dc.mu.Unlock()

	return targetService.Instances, nil
}

// Pick 选择一个健康的实例（round-robin + 故障转移）
func (dc *DiscoveryClient) Pick(project, serviceName, env string) (*InstanceEndpoint, func(error), error) {
	ctx, cancel := dc.client.DefaultContext()
	defer cancel()

	// 获取实例列表
	instances, err := dc.Resolve(ctx, project, serviceName, env)
	if err != nil {
		return nil, nil, err
	}

	if len(instances) == 0 {
		return nil, nil, fmt.Errorf("no instances available for %s/%s", project, serviceName)
	}

	// 获取缓存
	cacheKey := fmt.Sprintf("%s/%s", project, serviceName)
	dc.mu.Lock()
	cache := dc.services[cacheKey]

	// 过滤健康的实例
	healthyInstances := make([]int, 0, len(instances))
	now := time.Now()
	for i := range instances {
		endpoint := instances[i].Endpoint
		if failure, exists := dc.failedInstances[endpoint]; exists {
			// 检查是否在冷却期
			if now.Before(failure.cooldownUntil) {
				continue
			}
		}
		healthyInstances = append(healthyInstances, i)
	}

	if len(healthyInstances) == 0 {
		dc.mu.Unlock()
		return nil, nil, fmt.Errorf("no healthy instances available for %s/%s", project, serviceName)
	}

	// Round-robin 选择
	selectedIndex := healthyInstances[cache.nextIndex%len(healthyInstances)]
	cache.nextIndex++
	selectedInstance := &instances[selectedIndex]
	endpoint := selectedInstance.Endpoint

	dc.mu.Unlock()

	// 返回实例和错误报告函数
	reportErr := func(err error) {
		if err != nil {
			dc.markInstanceFailed(endpoint)
		}
	}

	return selectedInstance, reportErr, nil
}

// markInstanceFailed 标记实例失败
func (dc *DiscoveryClient) markInstanceFailed(endpoint string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	failure, exists := dc.failedInstances[endpoint]
	if !exists {
		failure = &instanceFailure{
			endpoint: endpoint,
		}
		dc.failedInstances[endpoint] = failure
	}

	failure.failureCount++
	failure.lastFailTime = time.Now()

	// 短期熔断：30 秒
	failure.cooldownUntil = time.Now().Add(30 * time.Second)
}

// RefreshService 刷新服务缓存
func (dc *DiscoveryClient) RefreshService(ctx context.Context, project, serviceName, env string) error {
	services, err := dc.client.Registry.ListServices(ctx, project, env)
	if err != nil {
		return err
	}

	var targetService *ServiceDescriptor
	for i := range services {
		if services[i].Name == serviceName {
			targetService = &services[i]
			break
		}
	}

	if targetService == nil {
		return fmt.Errorf("service %s/%s not found", project, serviceName)
	}

	cacheKey := fmt.Sprintf("%s/%s", project, serviceName)
	dc.mu.Lock()
	if cache, exists := dc.services[cacheKey]; exists {
		cache.descriptor = targetService
		cache.lastRefresh = time.Now()
	} else {
		dc.services[cacheKey] = &serviceCache{
			descriptor:  targetService,
			lastRefresh: time.Now(),
			nextIndex:   0,
		}
	}
	dc.mu.Unlock()

	return nil
}
