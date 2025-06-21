package registry

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/scheduler"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// Registry 服务注册中心接口
type Registry interface {
	// Register 注册服务实例
	Register(ctx context.Context, instance *ServiceInstance) error

	// Unregister 注销服务实例（物理删除）
	Unregister(ctx context.Context, serviceID string) error

	// Offline 下线服务实例（逻辑删除，保留记录）
	Offline(ctx context.Context, serviceID string) (*ServiceInstance, error)

	// Renew 续约服务实例
	Renew(ctx context.Context, serviceID string) error

	// Discover 发现服务实例列表（只返回在线服务）
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	// DiscoverByEnv 根据环境发现服务实例列表（只返回在线服务）
	DiscoverByEnv(ctx context.Context, serviceName string, env Environment) ([]*ServiceInstance, error)

	// DiscoverAll 发现所有服务实例列表（包括下线服务）
	DiscoverAll(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	// Watch 监听服务变更
	Watch(ctx context.Context, serviceName string) (Watcher, error)

	// GetService 获取单个服务实例
	GetService(ctx context.Context, serviceID string) (*ServiceInstance, error)

	// ListServices 列出所有服务名称
	ListServices(ctx context.Context) ([]string, error)

	// GetExpiredServices 获取过期的服务实例列表
	GetExpiredServices(ctx context.Context, expireThreshold time.Duration) ([]*ServiceInstance, error)

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
	client      *etcd.EtcdClient
	options     *Options
	logger      *zap.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	monitorOnce sync.Once // 确保监控任务只启动一次
	scheduler   *scheduler.Scheduler
}

// NewRegistry 创建新的注册中心
// etcdClient: ETCD客户端，用于服务数据存储
// scheduler: 定时任务调度器，用于定期检查过期服务（可以为nil，但将不支持自动监控过期服务）
func NewRegistry(etcdClient *etcd.EtcdClient, scheduler *scheduler.Scheduler, opts ...Option) (Registry, error) {
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
		client:    etcdClient,
		options:   options,
		logger:    zap.NewNop(), // 默认使用无操作的logger，可以后续设置
		ctx:       ctx,
		cancel:    cancel,
		scheduler: scheduler,
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
		generatedID, err := r.generateInstanceIDWithCollisionCheck(ctx, instance.Name, instance.Address, instance.Env)
		if err != nil {
			return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to generate instance ID", err)
		}
		instance.ID = generatedID
	}

	// 如果没有设置环境，默认为all（适用所有环境）
	if instance.Env == "" {
		instance.Env = EnvAll
	}

	instance.Status = StatusUp
	// 设置初始续约时间为注册时间
	instance.LastRenewTime = instance.RegisterTime

	// 构建key和value
	key := r.buildServiceKey(instance.Name, instance.ID)
	value, err := instance.ToJSON()
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to serialize instance", err)
	}

	// 注册服务，直接永久保存（不使用租约）
	_, err = r.client.Client.Put(ctx, key, value)
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to register service", err)
	}

	r.logger.Info("Service registered successfully",
		zap.String("service_id", instance.ID),
		zap.String("service_name", instance.Name),
		zap.String("address", instance.Address))

	// 启动监控任务（只启动一次）
	r.startMonitorOnce()

	return nil
}

// Unregister 注销服务实例
func (r *etcdRegistry) Unregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return NewRegistryError(ErrCodeInvalidInstance, "service ID cannot be empty")
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

// Offline 下线服务实例（逻辑删除，保留记录）
func (r *etcdRegistry) Offline(ctx context.Context, serviceID string) (*ServiceInstance, error) {
	if serviceID == "" {
		return nil, NewRegistryError(ErrCodeInvalidInstance, "service ID cannot be empty")
	}

	// 获取现有的服务实例
	instance, err := r.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	// 如果已经是下线状态，直接返回
	if instance.IsOffline() {
		return instance, nil
	}

	// 更新实例状态为下线
	instance.Status = StatusDown
	instance.OfflineTime = time.Now()

	// 构建key和value
	key := r.buildServiceKey(instance.Name, instance.ID)
	value, err := instance.ToJSON()
	if err != nil {
		return nil, NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to serialize instance", err)
	}

	// 更新服务实例（不使用租约，永久保存）
	_, err = r.client.Client.Put(ctx, key, value)
	if err != nil {
		return nil, NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to update service status", err)
	}

	r.logger.Info("Service offlined successfully",
		zap.String("service_id", serviceID),
		zap.String("service_name", instance.Name),
		zap.Time("offline_time", instance.OfflineTime))

	// 启动监控任务（只启动一次）
	r.startMonitorOnce()

	return instance, nil
}

// Discover 发现服务实例列表（只返回在线服务）
func (r *etcdRegistry) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	// 获取所有实例
	allInstances, err := r.DiscoverAll(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 过滤只返回在线服务
	var onlineInstances []*ServiceInstance
	for _, instance := range allInstances {
		if instance.IsOnline() {
			onlineInstances = append(onlineInstances, instance)
		}
	}

	return onlineInstances, nil
}

// DiscoverAll 发现所有服务实例列表（包括下线服务）
func (r *etcdRegistry) DiscoverAll(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
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

// GetExpiredServices 获取过期的服务实例列表
func (r *etcdRegistry) GetExpiredServices(ctx context.Context, expireThreshold time.Duration) ([]*ServiceInstance, error) {
	// 使用GetWithPrefix获取所有服务
	kvMap, err := r.client.GetWithPrefix(ctx, r.options.Prefix)
	if err != nil {
		return nil, NewRegistryErrorWithCause(ErrCodeDiscoveryFailed, "failed to get services", err)
	}

	var expiredInstances []*ServiceInstance
	for k, v := range kvMap {
		instance, err := FromJSON(v)
		if err != nil {
			r.logger.Warn("Failed to parse service instance",
				zap.String("key", k),
				zap.Error(err))
			continue
		}

		// 检查是否过期
		if instance.IsExpired(expireThreshold) {
			expiredInstances = append(expiredInstances, instance)
		}
	}

	return expiredInstances, nil
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
// 使用服务名称、地址和环境的哈希值生成固定ID，确保同一应用在相同环境下生成相同的ID
func (r *etcdRegistry) generateInstanceID(serviceName string, address string, env Environment) string {
	// 使用服务名称、地址和环境组合生成哈希
	data := fmt.Sprintf("%s-%s-%s", serviceName, address, env)
	hash := sha256.Sum256([]byte(data))
	// 取前16位十六进制字符作为ID后缀，降低碰撞概率
	hashStr := fmt.Sprintf("%x", hash)[:16]
	return fmt.Sprintf("%s-%s", serviceName, hashStr)
}

// generateInstanceIDWithCollisionCheck 生成服务实例ID并检查冲突
func (r *etcdRegistry) generateInstanceIDWithCollisionCheck(ctx context.Context, serviceName string, address string, env Environment) (string, error) {
	// 生成基础ID
	baseID := r.generateInstanceID(serviceName, address, env)

	// 检查是否已存在
	existing, err := r.GetService(ctx, baseID)
	if err != nil && err != ErrServiceNotFound {
		return "", err
	}

	// 如果不存在，直接返回
	if existing == nil {
		return baseID, nil
	}

	// 如果存在的服务实例与当前请求的完全匹配，说明是同一个服务实例
	if existing.Name == serviceName && existing.Address == address && existing.Env == env {
		return baseID, nil
	}

	// 如果存在但不匹配，说明发生了哈希冲突，使用更长的哈希值
	r.logger.Warn("检测到服务ID哈希冲突，使用扩展哈希",
		zap.String("service_name", serviceName),
		zap.String("address", address),
		zap.String("env", string(env)),
		zap.String("conflicting_id", baseID))

	// 使用完整的哈希值（64位）来避免冲突
	data := fmt.Sprintf("%s-%s-%s", serviceName, address, env)
	hash := sha256.Sum256([]byte(data))
	fullHashStr := fmt.Sprintf("%x", hash)
	return fmt.Sprintf("%s-%s", serviceName, fullHashStr), nil
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
		return w.registry.DiscoverAll(w.ctx, w.serviceName)
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

	// 获取现有的服务实例
	instance, err := r.GetService(ctx, serviceID)
	if err != nil {
		if err == ErrServiceNotFound {
			return NewRegistryError(ErrCodeInvalidInstance, "service not found")
		}
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to get service for renewal", err)
	}

	// 更新续约时间
	instance.LastRenewTime = time.Now()

	// 如果状态不是up，改为up
	statusChanged := false
	if instance.Status != StatusUp {
		instance.Status = StatusUp
		instance.OfflineTime = time.Time{} // 清空下线时间
		statusChanged = true
	}

	// 构建key和value
	key := r.buildServiceKey(instance.Name, instance.ID)
	value, err := instance.ToJSON()
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to serialize instance", err)
	}

	// 更新服务实例
	_, err = r.client.Client.Put(ctx, key, value)
	if err != nil {
		return NewRegistryErrorWithCause(ErrCodeRegistryFailed, "failed to renew service", err)
	}

	r.logger.Debug("Service renewed successfully",
		zap.String("service_id", serviceID),
		zap.Bool("status_changed", statusChanged),
		zap.String("status", instance.Status))

	// 启动监控任务（只启动一次）
	r.startMonitorOnce()

	return nil
}

// startMonitorOnce 启动监控任务（只启动一次）
func (r *etcdRegistry) startMonitorOnce() {
	r.monitorOnce.Do(func() {
		r.scheduleMonitorTask()
	})
}

// scheduleMonitorTask 创建并调度监控任务
func (r *etcdRegistry) scheduleMonitorTask() {
	if r.scheduler == nil {
		r.logger.Warn("Scheduler is nil, cannot schedule monitor task")
		return
	}

	// 创建定时任务，检查间隔为TTL的一半
	checkInterval := time.Duration(r.options.LeaseTTL) * time.Second / 2

	// 使用IntervalTask创建周期性任务
	task := scheduler.NewIntervalTask(
		"registry-monitor-expired-services",
		time.Now().Add(checkInterval),  // 首次执行时间
		checkInterval,                  // 执行间隔
		scheduler.TaskExecuteModeLocal, // 本地执行模式（每个节点都需要检查自己的服务）
		30*time.Second,                 // 任务超时时间
		r.monitorTaskFunc,              // 执行函数
	)

	// 添加任务到调度器
	if err := r.scheduler.AddTask(task); err != nil {
		r.logger.Error("Failed to add monitor task to scheduler", zap.Error(err))
	} else {
		r.logger.Info("Monitor task scheduled successfully",
			zap.Duration("interval", checkInterval))
	}
}

// monitorTaskFunc 监控任务执行函数
func (r *etcdRegistry) monitorTaskFunc(ctx context.Context) error {
	r.checkAndOfflineExpiredServices()
	return nil
}

// checkAndOfflineExpiredServices 检查并将过期服务设为离线状态
// 此方法由调度器定时调用，检查所有在线服务是否已过期
func (r *etcdRegistry) checkAndOfflineExpiredServices() {
	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()

	// 获取所有服务实例
	kvMap, err := r.client.GetWithPrefix(ctx, r.options.Prefix)
	if err != nil {
		r.logger.Error("Failed to get services for expiration check", zap.Error(err))
		return
	}

	expiredCount := 0
	for k, v := range kvMap {
		instance, err := FromJSON(v)
		if err != nil {
			r.logger.Warn("Failed to parse service instance during expiration check",
				zap.String("key", k),
				zap.Error(err))
			continue
		}

		// 只检查在线服务
		if !instance.IsOnline() {
			continue
		}

		// 检查是否过期（续约间隔的3倍作为过期时间）
		expireThreshold := time.Duration(r.options.LeaseTTL) * time.Second
		if instance.IsExpired(expireThreshold) {
			// 将服务标记为离线
			_, err := r.Offline(ctx, instance.ID)
			if err != nil {
				r.logger.Error("Failed to offline expired service",
					zap.String("service_id", instance.ID),
					zap.String("service_name", instance.Name),
					zap.Error(err))
			} else {
				r.logger.Info("Service marked as offline due to expiration",
					zap.String("service_id", instance.ID),
					zap.String("service_name", instance.Name),
					zap.Duration("last_renew_duration", instance.GetLastRenewDuration()))
				expiredCount++
			}
		}
	}

	if expiredCount > 0 {
		r.logger.Info("Expired services check completed",
			zap.Int("expired_count", expiredCount),
			zap.Int("total_checked", len(kvMap)))
	}
}
