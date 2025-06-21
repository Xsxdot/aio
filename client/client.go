package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/lock"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/scheduler"
	"go.uber.org/zap"
)

type Client struct {
	mu          sync.RWMutex
	log         *zap.Logger
	serviceInfo *registry.ServiceInstance
	once        sync.Once
	cfg         *AioConfig
	ctx         context.Context
	cancel      context.CancelFunc

	// 统一的 gRPC 客户端管理器
	grpcManager *GRPCClientManager

	// 各种服务客户端
	EtcdClient     *etcd.EtcdClient
	Scheduler      *scheduler.Scheduler
	LockManager    lock.LockManager
	ConfigClient   *ConfigClient
	RegistryClient *RegistryClient
	MonitorClient  *MonitorClient
}

type AioConfig struct {
	Endpoints []string
	ClientId  string
	Secret    string
}

func New(config *AioConfig, serviceInfo *registry.ServiceInstance) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		log:         common.GetLogger().GetZapLogger("aio-client"),
		cfg:         config,
		serviceInfo: serviceInfo,
		ctx:         ctx,
		cancel:      cancel,
	}
	return client
}

// Start 启动客户端，建立连接并初始化所有服务
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()

	// 验证配置
	if len(c.cfg.Endpoints) == 0 {
		return fmt.Errorf("端点列表不能为空")
	}
	if c.cfg.ClientId == "" || c.cfg.Secret == "" {
		return fmt.Errorf("客户端ID和密钥不能为空")
	}

	// 创建 gRPC 客户端管理器
	c.grpcManager = NewGRPCClientManager(c.cfg.Endpoints, c.cfg.ClientId, c.cfg.Secret, c.log)

	// 启动 gRPC 客户端管理器
	if err := c.grpcManager.Start(ctx); err != nil {
		return fmt.Errorf("启动 gRPC 客户端管理器失败: %v", err)
	}

	// 初始化各种服务客户端
	c.ConfigClient = NewConfigClient(c.grpcManager)
	c.RegistryClient = NewRegistryClient(c.grpcManager)
	c.mu.Unlock()

	// 等待客户端完全就绪，包括认证 token 获取成功
	maxWaitTime := 30 * time.Second
	waitInterval := 500 * time.Millisecond
	totalWait := time.Duration(0)

	for !c.IsReady() && totalWait < maxWaitTime {
		time.Sleep(waitInterval)
		totalWait += waitInterval
		c.log.Debug("等待客户端就绪", zap.Duration("已等待", totalWait))
	}

	if !c.IsReady() {
		return fmt.Errorf("客户端在 %v 内未能就绪", maxWaitTime)
	}

	// 1. 先刷新端点列表
	if err := c.RefreshEndpoints(ctx); err != nil {
		c.log.Warn("刷新端点列表失败，使用默认端点", zap.Error(err))
	}

	etcdCfg := new(config.EtcdConfig)
	err := c.ConfigClient.GetConfigJSONWithParse(ctx, "etcd", etcdCfg)
	if err != nil {
		return fmt.Errorf("获取 etcd 配置失败: %v", err)
	}
	c.EtcdClient, err = etcd.NewClient(etcdCfg)
	if err != nil {
		return err
	}

	c.log.Info("初始化 etcd 客户端成功")

	c.LockManager, err = lock.NewEtcdLockManager(c.EtcdClient, fmt.Sprintf("aio-lock-manager-%s-%s", c.serviceInfo.Name, c.serviceInfo.Env), lock.DefaultLockManagerOptions())
	if err != nil {
		return err
	}

	// 初始化 Scheduler
	c.Scheduler = scheduler.NewScheduler(c.LockManager, scheduler.DefaultSchedulerConfig())

	// 启动 Scheduler
	if err := c.Scheduler.Start(); err != nil {
		return fmt.Errorf("启动调度器失败: %v", err)
	}

	// 2. 注册自身到服务注册中心
	if c.serviceInfo != nil {
		_, err := c.RegisterSelf(ctx)
		if err != nil {
			return fmt.Errorf("注册服务失败: %v", err)
		}
	}

	// 3. 使用 Scheduler 设置自动续约
	if c.serviceInfo != nil && c.serviceInfo.ID != "" {
		if err := c.setupAutoRenewal(ctx); err != nil {
			c.log.Error("设置自动续约失败", zap.Error(err))
			// 不返回错误，让客户端继续运行
		}
	}

	c.MonitorClient = NewMonitorClient(c.serviceInfo, c.grpcManager, c.Scheduler)
	if err := c.MonitorClient.Start(); err != nil {
		return fmt.Errorf("启动监控客户端失败: %v", err)
	}

	c.log.Info("AIO 客户端启动成功",
		zap.String("client_id", c.cfg.ClientId),
		zap.Strings("endpoints", c.cfg.Endpoints),
		zap.String("current_endpoint", c.grpcManager.GetCurrentEndpoint()))

	return nil
}

// IsReady 检查客户端是否就绪
func (c *Client) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.grpcManager != nil && c.grpcManager.IsTokenValid()
}

// GetCurrentEndpoint 获取当前连接的端点
func (c *Client) GetCurrentEndpoint() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.grpcManager != nil {
		return c.grpcManager.GetCurrentEndpoint()
	}
	return ""
}

// GetEndpoints 获取所有可用端点
func (c *Client) GetEndpoints() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.grpcManager != nil {
		return c.grpcManager.GetEndpoints()
	}
	return c.cfg.Endpoints
}

// RefreshEndpoints 从注册中心刷新端点列表
func (c *Client) RefreshEndpoints(ctx context.Context) error {
	if !c.IsReady() {
		return fmt.Errorf("客户端未就绪")
	}

	// 获取 aio-service 的所有实例
	instances, err := c.RegistryClient.Discover(ctx, "aio-service", "", registry.StatusUp, "")
	if err != nil {
		return fmt.Errorf("从注册中心获取服务实例失败: %v", err)
	}

	var newEndpoints []string
	for _, instance := range instances {
		newEndpoints = append(newEndpoints, instance.Address)
	}

	if len(newEndpoints) > 0 {
		c.mu.Lock()
		c.cfg.Endpoints = newEndpoints
		c.mu.Unlock()

		c.log.Info("端点列表已更新", zap.Strings("endpoints", newEndpoints))
	}

	return nil
}

// RegisterSelf 将自身注册到注册中心
func (c *Client) RegisterSelf(ctx context.Context) (*registry.ServiceInstance, error) {
	if !c.IsReady() {
		return nil, fmt.Errorf("客户端未就绪")
	}

	if c.serviceInfo == nil {
		return nil, fmt.Errorf("服务信息未设置")
	}

	// 使用构建器注册服务
	builder := NewServiceInstanceBuilder(c.serviceInfo.Name, c.serviceInfo.Address).
		WithProtocol(c.serviceInfo.Protocol).
		WithEnv(string(c.serviceInfo.Env)).
		WithWeight(int32(c.serviceInfo.Weight)).
		WithStatus(c.serviceInfo.Status)

	// 添加元数据
	if c.serviceInfo.Metadata != nil {
		for k, v := range c.serviceInfo.Metadata {
			builder.WithMetadata(k, v)
		}
	}

	protoInstance, err := builder.Register(ctx, c.RegistryClient)
	if err != nil {
		return nil, fmt.Errorf("注册服务失败: %v", err)
	}

	// 更新服务信息中的 ID
	c.serviceInfo.ID = protoInstance.Id

	c.log.Info("服务注册成功",
		zap.String("service_id", protoInstance.Id),
		zap.String("service_name", protoInstance.Name),
		zap.String("address", protoInstance.Address))

	return c.serviceInfo, nil
}

// UnregisterSelf 从注册中心注销自身
func (c *Client) UnregisterSelf(ctx context.Context) error {
	if !c.IsReady() {
		return fmt.Errorf("客户端未就绪")
	}

	if c.serviceInfo == nil || c.serviceInfo.ID == "" {
		return fmt.Errorf("服务未注册或服务ID为空")
	}

	_, err := c.RegistryClient.Offline(ctx, c.serviceInfo.ID)
	if err != nil {
		return fmt.Errorf("注销服务失败: %v", err)
	}

	c.log.Info("服务下线成功",
		zap.String("service_id", c.serviceInfo.ID),
		zap.String("message", "下线成功"))

	c.serviceInfo.ID = ""
	return nil
}

// RenewSelf 续约自身服务
func (c *Client) RenewSelf(ctx context.Context) error {
	if !c.IsReady() {
		return fmt.Errorf("客户端未就绪")
	}

	if c.serviceInfo == nil || c.serviceInfo.ID == "" {
		return fmt.Errorf("服务未注册或服务ID为空")
	}

	_, err := c.RegistryClient.Renew(ctx, c.serviceInfo.ID)
	if err != nil {
		return fmt.Errorf("续约服务失败: %v", err)
	}

	c.log.Debug("服务续约成功", zap.String("service_id", c.serviceInfo.ID))
	return nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	if c.MonitorClient != nil {
		if err := c.MonitorClient.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("停止监控客户端失败: %v", err))
		}
	}

	// 注销服务（如果已注册）
	if c.serviceInfo != nil && c.serviceInfo.ID != "" {
		if err := c.UnregisterSelf(c.ctx); err != nil {
			errs = append(errs, fmt.Errorf("注销服务失败: %v", err))
		}
	}

	// 停止调度器
	if c.Scheduler != nil {
		if err := c.Scheduler.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("停止调度器失败: %v", err))
		}
	}

	// 关闭 ETCD 客户端
	if c.EtcdClient != nil {
		c.EtcdClient.Close()
	}

	// 关闭 gRPC 客户端管理器
	if c.grpcManager != nil {
		if err := c.grpcManager.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭 gRPC 客户端管理器失败: %v", err))
		}
	}

	// 取消上下文
	if c.cancel != nil {
		c.cancel()
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭客户端时发生错误: %v", errs)
	}

	c.log.Info("AIO 客户端已关闭")
	return nil
}

// GetClientID 获取客户端 ID
func (c *Client) GetClientID() string {
	return c.cfg.ClientId
}

// GetServiceInfo 获取服务信息
func (c *Client) GetServiceInfo() *registry.ServiceInstance {
	return c.serviceInfo
}

// UpdateServiceInfo 更新服务信息
func (c *Client) UpdateServiceInfo(serviceInfo *registry.ServiceInstance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serviceInfo = serviceInfo
}

// setupAutoRenewal 设置自动续约任务
func (c *Client) setupAutoRenewal(ctx context.Context) error {
	if c.serviceInfo == nil || c.serviceInfo.ID == "" {
		return fmt.Errorf("服务信息未设置或服务ID为空")
	}

	// 创建自动续约任务（每10秒续约一次）
	renewalTask := scheduler.NewIntervalTask(
		fmt.Sprintf("auto-renewal-%s", c.serviceInfo.ID),
		time.Now().Add(10*time.Second), // 10秒后开始执行
		10*time.Second,                 // 每10秒执行一次
		scheduler.TaskExecuteModeLocal, // 本地任务
		5*time.Second,                  // 5秒超时
		func(ctx context.Context) error {
			// 先尝试续约
			err := c.RenewSelf(ctx)
			if err != nil {
				c.log.Warn("服务续约失败，尝试重新注册",
					zap.String("service_id", c.serviceInfo.ID),
					zap.Error(err))

				// 续约失败，尝试重新注册
				_, registerErr := c.RegisterSelf(ctx)
				if registerErr != nil {
					c.log.Error("重新注册服务失败",
						zap.String("service_id", c.serviceInfo.ID),
						zap.Error(registerErr))
					return fmt.Errorf("续约失败且重新注册也失败: 续约错误=%v, 注册错误=%v", err, registerErr)
				}

				c.log.Info("服务重新注册成功",
					zap.String("service_id", c.serviceInfo.ID))
				return nil
			}

			// 续约成功
			return nil
		},
	)

	// 添加任务到调度器
	if err := c.Scheduler.AddTask(renewalTask); err != nil {
		return fmt.Errorf("添加自动续约任务失败: %v", err)
	}

	c.log.Info("自动续约任务已设置",
		zap.String("service_id", c.serviceInfo.ID),
		zap.String("task_id", renewalTask.GetID()))

	return nil
}
