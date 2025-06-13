package registry

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xsxdot/aio/internal/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// Registry 服务注册中心接口
type Registry interface {
	// Register 注册服务实例
	Register(ctx context.Context, instance *ServiceInstance) error

	// Unregister 注销服务实例
	Unregister(ctx context.Context, serviceID string) error

	// Renew 续约服务实例
	Renew(ctx context.Context, serviceID string) error

	// Discover 发现服务实例列表
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	// DiscoverByEnv 根据环境发现服务实例列表
	DiscoverByEnv(ctx context.Context, serviceName string, env Environment) ([]*ServiceInstance, error)

	// Watch 监听服务变更
	Watch(ctx context.Context, serviceName string) (Watcher, error)

	// GetService 获取单个服务实例
	GetService(ctx context.Context, serviceID string) (*ServiceInstance, error)

	// ListServices 列出所有服务名称
	ListServices(ctx context.Context) ([]string, error)

	// Close 关闭注册中心
	Close() error
}

// Watcher 服务监听器接口
type Watcher interface {
	// Next 获取下一个事件
	Next() ([]*ServiceInstance, error)

	// Stop 停止监听
	Stop()
}

// etcdRegistry ETCD实现的注册中心
type etcdRegistry struct {
	client   *etcd.EtcdClient
	options  *Options
	logger   *zap.Logger
	leases   map[string]clientv3.LeaseID // 保存每个服务实例的租约ID
	leaseMux sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewRegistry 创建新的注册中心
func NewRegistry(etcdClient *etcd.EtcdClient, opts ...Option) (Registry, error) {
	if etcdClient == nil {
		return nil, NewRegistryError(ErrCodeInvalidConfig, "etcd client cannot be nil")
	}

	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	registry := &etcdRegistry{
		client:  etcdClient,
		options: options,
		logger:  zap.NewNop(), // 默认使用无操作的logger，可以后续设置
		leases:  make(map[string]clientv3.LeaseID),
		ctx:     ctx,
		cancel:  cancel,
	}

	return registry, nil
}

// SetLogger 设置日志器
func (r *etcdRegistry) SetLogger(logger *zap.Logger) {
	r.logger = logger
}

// Register 注册服务实例
func (r *etcdRegistry) Register(ctx context.Context, instance *ServiceInstance) error {
	if instance == nil {
		return ErrInvalidServiceInstance
	}

	if err := r.validateInstance(instance); err != nil {
		return err
	}

	// 设置注册时间
	if instance.RegisterTime.IsZero() {
		instance.RegisterTime = time.Now()
	}

	// 如果没有设置启动时间，使用注册时间
	if instance.StartTime.IsZero() {
		instance.StartTime = instance.RegisterTime
	}

	// 如果没有设置ID，生成一个
	if instance.ID == "" {
		instance.ID = r.generateInstanceID(instance.Name)
	}

	// 如果没有设置环境，默认为all（适用所有环境）
	if instance.Env == "" {
		instance.Env = EnvAll
	}

	// 如果没有设置状态，默认为active
	if instance.Status == "" {
		instance.Status = "active"
	}

	// 创建租约
	leaseResp, err := r.client.Client.Grant(ctx, r.options.LeaseTTL)
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeLeaseError, "failed to create lease", err)
	}

	// 构建key和value
	key := r.buildServiceKey(instance.Name, instance.ID)
	value, err := instance.ToJSON()
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to serialize instance", err)
	}

	// 注册服务，使用底层client并传递租约
	_, err = r.client.Client.Put(ctx, key, value, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to register service", err)
	}

	// 保存租约ID
	r.leaseMux.Lock()
	r.leases[instance.ID] = leaseResp.ID
	r.leaseMux.Unlock()

	r.logger.Info("Service registered successfully",
		zap.String("service_id", instance.ID),
		zap.String("service_name", instance.Name),
		zap.String("address", instance.Address))

	return nil
}

// Unregister 注销服务实例
func (r *etcdRegistry) Unregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return NewRegistryError(ErrCodeInvalidInstance, "service ID cannot be empty")
	}

	// 获取租约ID
	r.leaseMux.RLock()
	leaseID, exists := r.leases[serviceID]
	r.leaseMux.RUnlock()

	if exists {
		// 撤销租约（这会自动删除相关的key）
		_, err := r.client.Client.Revoke(ctx, leaseID)
		if err != nil {
			r.logger.Warn("Failed to revoke lease", zap.String("service_id", serviceID), zap.Error(err))
		}

		// 删除租约记录
		r.leaseMux.Lock()
		delete(r.leases, serviceID)
		r.leaseMux.Unlock()
	}

	// 确保删除key（兜底操作）
	resp, err := r.client.Client.Get(ctx, r.options.Prefix, clientv3.WithPrefix())
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeUnregisterFailed, "failed to list services", err)
	}

	for _, kv := range resp.Kvs {
		if strings.Contains(string(kv.Key), serviceID) {
			_, err := r.client.Client.Delete(ctx, string(kv.Key))
			if err != nil {
				r.logger.Warn("Failed to delete service key", zap.String("key", string(kv.Key)), zap.Error(err))
			}
		}
	}

	r.logger.Info("Service unregistered successfully", zap.String("service_id", serviceID))
	return nil
}

// Discover 发现服务实例列表
func (r *etcdRegistry) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if serviceName == "" {
		return nil, NewRegistryError(ErrCodeInvalidInstance, "service name cannot be empty")
	}

	key := r.buildServicePrefix(serviceName)
	// 使用GetWithPrefix方法
	kvMap, err := r.client.GetWithPrefix(ctx, key)
	if err != nil {
		return nil, NewRegistryErrorWithCause(ErrCodeDiscoveryFailed, "failed to discover services", err)
	}

	var instances []*ServiceInstance
	for k, v := range kvMap {
		instance, err := FromJSON(v)
		if err != nil {
			r.logger.Warn("Failed to parse service instance",
				zap.String("key", k),
				zap.Error(err))
			continue
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// DiscoverByEnv 根据环境发现服务实例列表
func (r *etcdRegistry) DiscoverByEnv(ctx context.Context, serviceName string, env Environment) ([]*ServiceInstance, error) {
	if serviceName == "" {
		return nil, NewRegistryError(ErrCodeInvalidInstance, "service name cannot be empty")
	}

	// 获取所有该服务的实例
	allInstances, err := r.Discover(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 如果没有指定环境，返回所有实例
	if env == "" {
		return allInstances, nil
	}

	// 过滤匹配环境的实例
	var filteredInstances []*ServiceInstance
	for _, instance := range allInstances {
		// 匹配指定环境或者是适用于所有环境的实例(env=EnvAll)
		if instance.Env == env || instance.Env == EnvAll {
			filteredInstances = append(filteredInstances, instance)
		}
	}

	return filteredInstances, nil
}

// Watch 监听服务变更
func (r *etcdRegistry) Watch(ctx context.Context, serviceName string) (Watcher, error) {
	if serviceName == "" {
		return nil, NewRegistryError(ErrCodeInvalidInstance, "service name cannot be empty")
	}

	key := r.buildServicePrefix(serviceName)
	// 使用WatchWithPrefix方法
	watchChan := r.client.WatchWithPrefix(ctx, key)

	watcher := &etcdWatcher{
		serviceName: serviceName,
		watchChan:   watchChan,
		registry:    r,
		ctx:         ctx,
	}

	return watcher, nil
}

// GetService 获取单个服务实例
func (r *etcdRegistry) GetService(ctx context.Context, serviceID string) (*ServiceInstance, error) {
	if serviceID == "" {
		return nil, NewRegistryError(ErrCodeInvalidInstance, "service ID cannot be empty")
	}

	// 使用GetWithPrefix获取所有服务
	kvMap, err := r.client.GetWithPrefix(ctx, r.options.Prefix)
	if err != nil {
		return nil, NewRegistryErrorWithCause(ErrCodeDiscoveryFailed, "failed to get service", err)
	}

	for k, v := range kvMap {
		if strings.Contains(k, serviceID) {
			instance, err := FromJSON(v)
			if err != nil {
				return nil, NewRegistryErrorWithCause(ErrCodeDiscoveryFailed, "failed to parse service instance", err)
			}
			return instance, nil
		}
	}

	return nil, ErrServiceNotFound
}

// ListServices 列出所有服务名称
func (r *etcdRegistry) ListServices(ctx context.Context) ([]string, error) {
	// 使用GetWithPrefix获取所有服务
	kvMap, err := r.client.GetWithPrefix(ctx, r.options.Prefix)
	if err != nil {
		return nil, NewRegistryErrorWithCause(ErrCodeDiscoveryFailed, "failed to list services", err)
	}

	serviceNames := make(map[string]bool)
	for _, v := range kvMap {
		instance, err := FromJSON(v)
		if err != nil {
			continue
		}
		serviceNames[instance.Name] = true
	}

	var names []string
	for name := range serviceNames {
		names = append(names, name)
	}

	return names, nil
}

// Close 关闭注册中心
func (r *etcdRegistry) Close() error {
	r.cancel()
	// 注意：不关闭传入的EtcdClient，因为它可能被其他组件使用
	return nil
}

// validateInstance 验证服务实例
func (r *etcdRegistry) validateInstance(instance *ServiceInstance) error {
	if instance.Name == "" {
		return NewRegistryError(ErrCodeInvalidInstance, "service name cannot be empty")
	}

	if instance.Address == "" {
		return NewRegistryError(ErrCodeInvalidInstance, "service address cannot be empty")
	}

	return nil
}

// generateInstanceID 生成服务实例ID
func (r *etcdRegistry) generateInstanceID(serviceName string) string {
	return fmt.Sprintf("%s-%s", serviceName, uuid.New().String()[:8])
}

// buildServiceKey 构建服务key
func (r *etcdRegistry) buildServiceKey(serviceName, serviceID string) string {
	return path.Join(r.options.Prefix, serviceName, serviceID)
}

// buildServicePrefix 构建服务前缀
func (r *etcdRegistry) buildServicePrefix(serviceName string) string {
	return path.Join(r.options.Prefix, serviceName) + "/"
}

// buildServiceKeyPattern 构建服务key模式
func (r *etcdRegistry) buildServiceKeyPattern(serviceID string) string {
	return r.options.Prefix + "/" + "*/" + serviceID
}

// etcdWatcher ETCD监听器实现
type etcdWatcher struct {
	serviceName string
	watchChan   clientv3.WatchChan
	registry    *etcdRegistry
	ctx         context.Context
}

// Next 获取下一个事件
func (w *etcdWatcher) Next() ([]*ServiceInstance, error) {
	select {
	case <-w.ctx.Done():
		return nil, NewRegistryError(ErrCodeWatchFailed, "watcher context cancelled")
	case resp, ok := <-w.watchChan:
		if !ok {
			return nil, NewRegistryError(ErrCodeWatchFailed, "watch channel closed")
		}

		if resp.Err() != nil {
			return nil, NewRegistryErrorWithCause(ErrCodeWatchFailed, "watch error", resp.Err())
		}

		// 获取当前所有实例
		return w.registry.Discover(w.ctx, w.serviceName)
	}
}

// Stop 停止监听
func (w *etcdWatcher) Stop() {
	// Watch channel会在context取消时自动关闭
}

// Renew 续约服务实例
func (r *etcdRegistry) Renew(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return NewRegistryError(ErrCodeInvalidInstance, "service ID cannot be empty")
	}

	// 获取租约ID
	r.leaseMux.RLock()
	leaseID, exists := r.leases[serviceID]
	r.leaseMux.RUnlock()

	if !exists {
		return NewRegistryError(ErrCodeLeaseError, "lease not found for service")
	}

	// 续约
	_, err := r.client.Client.KeepAliveOnce(ctx, leaseID)
	if err != nil {
		r.logger.Error("Failed to renew lease",
			zap.String("service_id", serviceID),
			zap.Error(err))
		return NewRegistryErrorWithCause(ErrCodeLeaseError, "failed to renew lease", err)
	}

	r.logger.Debug("Service lease renewed successfully",
		zap.String("service_id", serviceID))

	return nil
}
