package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	authv1 "github.com/xsxdot/aio/api/proto/auth/v1"
	configv1 "github.com/xsxdot/aio/api/proto/config/v1"
	monitoringv1 "github.com/xsxdot/aio/api/proto/monitoring/v1"
	registryv1 "github.com/xsxdot/aio/api/proto/registry/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClientManager 统一的 gRPC 客户端管理器
type GRPCClientManager struct {
	mu           sync.RWMutex
	endpoints    []string
	clientID     string
	clientSecret string
	logger       *zap.Logger

	// 连接管理
	connection      *grpc.ClientConn
	currentEndpoint string

	// 认证相关
	credentials *AIOCredentials

	// 服务客户端
	authClient       authv1.AuthServiceClient
	configClient     configv1.ConfigServiceClient
	registryClient   registryv1.RegistryServiceClient
	monitoringClient monitoringv1.MetricStorageServiceClient

	// 配置
	maxRetries int
	retryDelay time.Duration
}

// NewGRPCClientManager 创建新的 gRPC 客户端管理器
func NewGRPCClientManager(endpoints []string, clientID, clientSecret string, logger *zap.Logger) *GRPCClientManager {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &GRPCClientManager{
		endpoints:    endpoints,
		clientID:     clientID,
		clientSecret: clientSecret,
		logger:       logger,
		maxRetries:   3,
		retryDelay:   time.Second * 2,
	}
}

// Start 启动客户端管理器
func (m *GRPCClientManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 建立初始连接进行认证测试
	if err := m.connectToEndpoint(); err != nil {
		return fmt.Errorf("连接到端点失败: %v", err)
	}

	// 创建认证凭据
	m.credentials = NewAIOCredentials(m.clientID, m.clientSecret, m.authClient)

	// 重新创建带认证的连接
	if err := m.recreateConnectionWithAuth(); err != nil {
		return fmt.Errorf("创建认证连接失败: %v", err)
	}

	// 启动后台服务发现任务
	go m.startServiceDiscovery(ctx)

	m.logger.Info("gRPC 客户端管理器启动成功",
		zap.String("current_endpoint", m.currentEndpoint),
		zap.Strings("endpoints", m.endpoints))

	return nil
}

// connectToEndpoint 连接到一个可用的端点
func (m *GRPCClientManager) connectToEndpoint() error {
	// 尝试连接所有端点
	for _, endpoint := range m.endpoints {
		conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			m.logger.Warn("连接端点失败", zap.String("endpoint", endpoint), zap.Error(err))
			continue
		}

		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		authClient := authv1.NewAuthServiceClient(conn)
		_, err = authClient.ClientAuth(ctx, &authv1.ClientAuthRequest{
			ClientId:     m.clientID,
			ClientSecret: m.clientSecret,
		})
		cancel()

		if err != nil {
			conn.Close()
			m.logger.Warn("端点认证测试失败", zap.String("endpoint", endpoint), zap.Error(err))
			continue
		}

		// 连接成功
		m.connection = conn
		m.currentEndpoint = endpoint
		m.initClients(conn)

		m.logger.Info("成功连接到端点", zap.String("endpoint", endpoint))
		return nil
	}

	return fmt.Errorf("无法连接到任何端点")
}

// recreateConnectionWithAuth 重新创建带认证的连接
func (m *GRPCClientManager) recreateConnectionWithAuth() error {
	if m.connection != nil {
		m.connection.Close()
	}

	// 创建带认证的连接
	conn, err := grpc.NewClient(m.currentEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(m.credentials), // 使用 PerRPCCredentials
	)
	if err != nil {
		return fmt.Errorf("创建认证连接失败: %v", err)
	}

	m.connection = conn
	m.initClients(conn)

	// 更新 AIOCredentials 中的 authClient，使用新连接
	if m.credentials != nil {
		m.credentials.UpdateAuthClient(m.authClient)
	}

	return nil
}

// initClients 初始化各种服务客户端
func (m *GRPCClientManager) initClients(conn *grpc.ClientConn) {
	m.authClient = authv1.NewAuthServiceClient(conn)
	m.configClient = configv1.NewConfigServiceClient(conn)
	m.registryClient = registryv1.NewRegistryServiceClient(conn)
	m.monitoringClient = monitoringv1.NewMetricStorageServiceClient(conn)
}

// startServiceDiscovery 启动后台服务发现任务
func (m *GRPCClientManager) startServiceDiscovery(ctx context.Context) {
	// 服务发现定时器
	discoveryTicker := time.NewTicker(time.Minute * 5)
	defer discoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-discoveryTicker.C:
			m.updateEndpointsFromRegistry(ctx)
		}
	}
}

// updateEndpointsFromRegistry 从注册中心更新端点列表
func (m *GRPCClientManager) updateEndpointsFromRegistry(ctx context.Context) {
	// 发现 aio-service 服务实例
	resp, err := m.registryClient.Discover(ctx, &registryv1.DiscoverRequest{
		ServiceName: "aio-service",
		Status:      "active",
	})
	if err != nil {
		m.logger.Warn("从注册中心获取服务列表失败", zap.Error(err))
		return
	}

	var newEndpoints []string
	for _, instance := range resp.Instances {
		newEndpoints = append(newEndpoints, instance.Address)
	}

	if len(newEndpoints) > 0 {
		m.mu.Lock()
		m.endpoints = newEndpoints
		m.mu.Unlock()

		m.logger.Info("从注册中心更新端点列表", zap.Strings("endpoints", newEndpoints))
	}
}

// executeWithRetry 执行带重试的操作
func (m *GRPCClientManager) executeWithRetry(ctx context.Context, operation func(context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		err := operation(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		m.logger.Warn("操作失败，准备重试",
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", m.maxRetries+1),
			zap.Error(err))

		if attempt < m.maxRetries {
			time.Sleep(m.retryDelay)
			// 尝试切换端点
			m.switchEndpoint()
		}
	}

	return fmt.Errorf("操作失败，已重试 %d 次: %v", m.maxRetries+1, lastErr)
}

// switchEndpoint 切换到另一个端点
func (m *GRPCClientManager) switchEndpoint() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.endpoints) <= 1 {
		return
	}

	// 选择一个不同的端点
	var newEndpoint string
	for _, endpoint := range m.endpoints {
		if endpoint != m.currentEndpoint {
			newEndpoint = endpoint
			break
		}
	}

	if newEndpoint == "" {
		return
	}

	// 关闭当前连接
	if m.connection != nil {
		m.connection.Close()
	}

	// 连接到新端点
	conn, err := grpc.NewClient(newEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(m.credentials),
	)
	if err != nil {
		m.logger.Error("切换端点失败", zap.String("endpoint", newEndpoint), zap.Error(err))
		return
	}

	m.connection = conn
	m.currentEndpoint = newEndpoint
	m.initClients(conn)

	// 更新 AIOCredentials 中的 authClient，使用新连接
	if m.credentials != nil {
		m.credentials.UpdateAuthClient(m.authClient)
	}

	m.logger.Info("切换到新端点", zap.String("endpoint", newEndpoint))
}

// GetConfigClient 获取配置服务客户端
func (m *GRPCClientManager) GetConfigClient() configv1.ConfigServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configClient
}

// GetRegistryClient 获取注册服务客户端
func (m *GRPCClientManager) GetRegistryClient() registryv1.RegistryServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registryClient
}

// GetMonitoringClient 获取监控存储服务客户端
func (m *GRPCClientManager) GetMonitoringClient() monitoringv1.MetricStorageServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.monitoringClient
}

// ExecuteWithRetry 暴露给外部使用的重试执行方法
func (m *GRPCClientManager) ExecuteWithRetry(ctx context.Context, operation func(context.Context) error) error {
	return m.executeWithRetry(ctx, operation)
}

// Close 关闭客户端管理器
func (m *GRPCClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connection != nil {
		if err := m.connection.Close(); err != nil {
			return fmt.Errorf("关闭连接失败: %v", err)
		}
	}

	m.logger.Info("gRPC 客户端管理器已关闭")
	return nil
}

// GetCurrentEndpoint 获取当前端点
func (m *GRPCClientManager) GetCurrentEndpoint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentEndpoint
}

// GetEndpoints 获取所有端点
func (m *GRPCClientManager) GetEndpoints() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	endpoints := make([]string, len(m.endpoints))
	copy(endpoints, m.endpoints)
	return endpoints
}

// IsTokenValid 检查 token 是否有效
func (m *GRPCClientManager) IsTokenValid() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.credentials != nil && m.credentials.IsTokenValid()
}

// GetCredentials 获取认证凭据
func (m *GRPCClientManager) GetCredentials() *AIOCredentials {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.credentials
}
