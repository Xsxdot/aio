package sdk

import (
	"context"
	"fmt"
	consts "github.com/xsxdot/aio/app/const"
	"time"

	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/distributed/idgen"
	"github.com/xsxdot/aio/pkg/distributed/lock"
	"github.com/xsxdot/aio/pkg/distributed/manager"
	"github.com/xsxdot/aio/pkg/distributed/state"

	"github.com/xsxdot/aio/internal/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdServiceOptions ETCD 服务选项
type EtcdServiceOptions struct {
	// 连接超时时间
	ConnectTimeout time.Duration
	// 安全连接配置
	TLS *etcd.TLSConfig
	// 认证信息
	Username string
	Password string
	// 自动同步端点
	AutoSyncEndpoints bool
}

// 默认的 ETCD 服务选项
var DefaultEtcdServiceOptions = &EtcdServiceOptions{
	ConnectTimeout:    5 * time.Second,
	AutoSyncEndpoints: true,
}

// EtcdService ETCD 服务
type EtcdService struct {
	// 客户端引用
	client *Client
	// 选项
	options *EtcdServiceOptions
	// ETCD 客户端实例
	etcdClient *etcd.EtcdClient
	// 日志记录器
	logger *zap.Logger
	// 分布式服务管理器
	distributedManager manager.DistributedManager
}

// NewEtcdService 创建 ETCD 服务
func NewEtcdService(client *Client, options *EtcdServiceOptions) *EtcdService {
	if options == nil {
		options = DefaultEtcdServiceOptions
	}

	return &EtcdService{
		client:  client,
		options: options,
		logger:  zap.NewExample(), // 使用示例日志记录器，实际可从外部传入
	}
}

// Connect 连接到 ETCD 服务
func (e *EtcdService) Connect(ctx context.Context) error {
	// 获取 ETCD 服务节点信息
	services, err := e.client.Discovery.DiscoverServices(ctx, consts.ComponentEtcd)
	if err != nil {
		return fmt.Errorf("发现 ETCD 服务失败: %w", err)
	}

	// 检查是否有可用的 ETCD 服务节点
	if len(services) == 0 {
		return fmt.Errorf("没有可用的 ETCD 服务节点")
	}

	e.logger.Info("发现 ETCD 服务节点", zap.Int("count", len(services)))

	// 构建端点列表
	endpoints := make([]string, 0, len(services))
	for _, service := range services {
		// 检查是否有指定的 ETCD 端口
		port := service.Port
		if etcdPort, ok := service.Metadata["etcd_port"]; ok {
			fmt.Sscanf(etcdPort, "%d", &port)
		}

		// 检查是否有 TLS 配置
		scheme := "http"
		if _, ok := service.Metadata["tls_enabled"]; ok {
			scheme = "https"
		}

		// 构建端点地址
		endpoint := fmt.Sprintf("%s://%s:%d", scheme, service.Address, port)
		endpoints = append(endpoints, endpoint)
		e.logger.Info("添加 ETCD 端点", zap.String("endpoint", endpoint))
	}

	// 创建 ETCD 客户端配置
	clientConfig := &etcd.ClientConfig{
		Endpoints:         endpoints,
		DialTimeout:       e.options.ConnectTimeout,
		Username:          e.options.Username,
		Password:          e.options.Password,
		AutoSyncEndpoints: e.options.AutoSyncEndpoints,
		TLS:               e.options.TLS,
	}

	// 创建 ETCD 客户端
	etcdClient, err := etcd.NewEtcdClient(clientConfig, e.logger)
	if err != nil {
		return fmt.Errorf("创建 ETCD 客户端失败: %w", err)
	}

	e.etcdClient = etcdClient

	// 初始化分布式服务管理器
	rawClient := etcdClient.GetClient()
	if rawClient != nil {
		e.distributedManager = manager.NewManager(rawClient, manager.WithLogger(e.logger))

		// 启动分布式服务管理器
		startCtx, cancel := context.WithTimeout(ctx, e.options.ConnectTimeout)
		defer cancel()

		if err := e.distributedManager.Start(startCtx); err != nil {
			e.logger.Warn("启动分布式服务管理器失败", zap.Error(err))
			// 继续执行，不返回错误，因为基础的 ETCD 客户端已经创建成功
		} else {
			e.logger.Info("分布式服务管理器启动成功")
		}
	}

	return nil
}

// GetClient 获取 ETCD 客户端实例
func (e *EtcdService) GetClient() *etcd.EtcdClient {
	return e.etcdClient
}

// GetRawClient 获取原始的 ETCD 客户端
func (e *EtcdService) GetRawClient() *clientv3.Client {
	if e.etcdClient == nil {
		return nil
	}
	return e.etcdClient.GetClient()
}

// GetDistributedManager 获取分布式服务管理器
func (e *EtcdService) GetDistributedManager() manager.DistributedManager {
	return e.distributedManager
}

// Close 关闭 ETCD 客户端
func (e *EtcdService) Close() {
	// 先停止分布式服务管理器
	if e.distributedManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.distributedManager.Stop(ctx); err != nil {
			e.logger.Warn("停止分布式服务管理器失败", zap.Error(err))
		}
		e.distributedManager = nil
	}

	// 再关闭 ETCD 客户端
	if e.etcdClient != nil {
		e.etcdClient.Close()
		e.etcdClient = nil
	}
}

// Put 放置键值对
func (e *EtcdService) Put(ctx context.Context, key, value string) error {
	if e.etcdClient == nil {
		return fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.Put(ctx, key, value)
}

// Get 获取键对应的值
func (e *EtcdService) Get(ctx context.Context, key string) (string, error) {
	if e.etcdClient == nil {
		return "", fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.Get(ctx, key)
}

// GetWithPrefix 获取具有相同前缀的所有键值对
func (e *EtcdService) GetWithPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	if e.etcdClient == nil {
		return nil, fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.GetWithPrefix(ctx, prefix)
}

// Delete 删除键
func (e *EtcdService) Delete(ctx context.Context, key string) error {
	if e.etcdClient == nil {
		return fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.Delete(ctx, key)
}

// DeleteWithPrefix 删除具有相同前缀的所有键
func (e *EtcdService) DeleteWithPrefix(ctx context.Context, prefix string) error {
	if e.etcdClient == nil {
		return fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.DeleteWithPrefix(ctx, prefix)
}

// Watch 监视键的变化
func (e *EtcdService) Watch(ctx context.Context, key string) (clientv3.WatchChan, error) {
	if e.etcdClient == nil {
		return nil, fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.Watch(ctx, key), nil
}

// WatchWithPrefix 监视具有相同前缀的所有键的变化
func (e *EtcdService) WatchWithPrefix(ctx context.Context, prefix string) (clientv3.WatchChan, error) {
	if e.etcdClient == nil {
		return nil, fmt.Errorf("ETCD 客户端未初始化")
	}
	return e.etcdClient.WatchWithPrefix(ctx, prefix), nil
}

// AutoConnect 根据服务发现自动连接到 ETCD 服务
func (e *EtcdService) AutoConnect(ctx context.Context) error {
	// 先尝试连接
	err := e.Connect(ctx)
	if err == nil {
		return nil
	}

	// 如果连接失败，监听服务变更
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 添加服务发现处理函数
	e.client.Discovery.OnServiceEvent(func(event *DiscoveryServiceEvent) {
		if event.Service.Name != consts.ComponentEtcd {
			return
		}

		// 如果有 ETCD 服务变更，尝试重新连接
		if event.Type == "created" || event.Type == "updated" {
			// 关闭之前的连接
			if e.etcdClient != nil {
				e.etcdClient.Close()
				e.etcdClient = nil
			}

			// 停止之前的分布式服务管理器
			if e.distributedManager != nil {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer stopCancel()
				e.distributedManager.Stop(stopCtx)
				e.distributedManager = nil
			}

			// 尝试重新连接
			connectCtx, connectCancel := context.WithTimeout(context.Background(), e.options.ConnectTimeout)
			defer connectCancel()

			err := e.Connect(connectCtx)
			if err != nil {
				e.logger.Error("重新连接 ETCD 服务失败", zap.Error(err))
			} else {
				e.logger.Info("已重新连接到 ETCD 服务")
				cancel() // 连接成功，取消监听
			}
		}
	})

	// 开始监听 ETCD 服务
	err = e.client.Discovery.WatchService(watchCtx, consts.ComponentEtcd)
	if err != nil {
		return fmt.Errorf("监听 ETCD 服务变更失败: %w", err)
	}

	// 等待连接成功或超时
	select {
	case <-watchCtx.Done():
		if watchCtx.Err() == context.Canceled {
			// 主动取消，说明连接成功
			return nil
		}
		return fmt.Errorf("等待 ETCD 服务连接超时")
	case <-ctx.Done():
		// 外部上下文取消
		return ctx.Err()
	}
}

// 以下是分布式功能的快捷访问方法 //

// GetElectionService 获取选举服务
func (e *EtcdService) GetElectionService() (election.ElectionService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.Election(), nil
}

// GetLockService 获取锁服务
func (e *EtcdService) GetLockService() (lock.LockService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.Lock(), nil
}

// GetIDGeneratorService 获取ID生成器服务
func (e *EtcdService) GetIDGeneratorService() (idgen.IDGeneratorService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.IDGenerator(), nil
}

// GetStateManagerService 获取状态管理服务
func (e *EtcdService) GetStateManagerService() (state.StateManagerService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.StateManager(), nil
}

// CreateLock 创建一个命名的分布式锁
func (e *EtcdService) CreateLock(ctx context.Context, name string, options ...lock.LockOption) (lock.Lock, error) {
	lockService, err := e.GetLockService()
	if err != nil {
		return nil, err
	}
	return lockService.Create(name, options...)
}

// CreateElection 创建一个命名的分布式选举
func (e *EtcdService) CreateElection(ctx context.Context, name string, options ...election.ElectionOption) (election.Election, error) {
	electionService, err := e.GetElectionService()
	if err != nil {
		return nil, err
	}
	return electionService.Create(name, options...)
}

// CreateIDGenerator 创建一个命名的ID生成器
func (e *EtcdService) CreateIDGenerator(ctx context.Context, name string, options ...idgen.IDGenOption) (idgen.IDGenerator, error) {
	idGenService, err := e.GetIDGeneratorService()
	if err != nil {
		return nil, err
	}
	return idGenService.Create(name, options...)
}

// RegisterEtcdServiceChangeHandler 注册 ETCD 服务变更处理函数
func (e *EtcdService) RegisterEtcdServiceChangeHandler(ctx context.Context) error {
	// 监听 ETCD 服务变更
	return e.client.Discovery.WatchService(ctx, consts.ComponentEtcd)
}

// 确保服务发现能提供 ETCD 信息的工具方法
func (e *EtcdService) ensureEtcdServiceDiscoverable(ctx context.Context, serviceName string) error {
	// 检查服务是否可发现
	services, err := e.client.Discovery.DiscoverServices(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("发现 %s 服务失败: %w", serviceName, err)
	}

	if len(services) == 0 {
		return fmt.Errorf("没有可用的 %s 服务节点", serviceName)
	}

	return nil
}
