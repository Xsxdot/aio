package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "xiaozhizhang/system/agent/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// AgentClient Agent gRPC 客户端
// 管理到不同 agent 地址的连接池
type AgentClient struct {
	conns          map[string]*grpc.ClientConn
	mu             sync.RWMutex
	defaultTimeout time.Duration
	tokenProvider  func() string // 获取当前 JWT token 的函数
}

// NewAgentClient 创建 Agent 客户端
func NewAgentClient(tokenProvider func() string, timeout time.Duration) *AgentClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &AgentClient{
		conns:          make(map[string]*grpc.ClientConn),
		defaultTimeout: timeout,
		tokenProvider:  tokenProvider,
	}
}

// getConn 获取或创建到指定地址的连接
func (c *AgentClient) getConn(address string) (*grpc.ClientConn, error) {
	c.mu.RLock()
	conn, exists := c.conns[address]
	c.mu.RUnlock()

	if exists && conn.GetState() != connectivity.TransientFailure {
		return conn, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 双检锁
	if conn, exists := c.conns[address]; exists {
		if conn.GetState() != connectivity.TransientFailure {
			return conn, nil
		}
		// 旧连接失效，关闭
		conn.Close()
	}

	// 创建新连接（使用 context 控制超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("连接 agent %s 失败: %w", address, err)
	}

	c.conns[address] = conn
	return conn, nil
}

// withAuth 添加鉴权 metadata
func (c *AgentClient) withAuth(ctx context.Context) context.Context {
	if c.tokenProvider != nil {
		token := c.tokenProvider()
		if token != "" {
			md := metadata.Pairs("authorization", "Bearer "+token)
			return metadata.NewOutgoingContext(ctx, md)
		}
	}
	return ctx
}

// withTimeout 创建带超时的 context
func (c *AgentClient) withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout == 0 {
		timeout = c.defaultTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// Close 关闭所有连接
func (c *AgentClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for addr, conn := range c.conns {
		if err := conn.Close(); err != nil {
			// 记录但不阻塞
			fmt.Printf("关闭连接 %s 失败: %v\n", addr, err)
		}
	}
	c.conns = make(map[string]*grpc.ClientConn)
	return nil
}

// ==================== Nginx 方法 ====================

// PutNginxConfig 创建或更新 nginx 配置
func (c *AgentClient) PutNginxConfig(ctx context.Context, address, name, content string, validate, reload bool) (*pb.PutNginxConfigResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.PutNginxConfig(ctx, &pb.PutNginxConfigRequest{
		Name:     name,
		Content:  content,
		Validate: validate,
		Reload:   reload,
	})
}

// GetNginxConfig 读取 nginx 配置
func (c *AgentClient) GetNginxConfig(ctx context.Context, address, name string) (*pb.GetNginxConfigResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.GetNginxConfig(ctx, &pb.GetNginxConfigRequest{
		Name: name,
	})
}

// DeleteNginxConfig 删除 nginx 配置
func (c *AgentClient) DeleteNginxConfig(ctx context.Context, address, name string, validate, reload bool) (*pb.DeleteNginxConfigResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.DeleteNginxConfig(ctx, &pb.DeleteNginxConfigRequest{
		Name:     name,
		Validate: validate,
		Reload:   reload,
	})
}

// ListNginxConfigs 列出 nginx 配置
func (c *AgentClient) ListNginxConfigs(ctx context.Context, address, keyword string) (*pb.ListNginxConfigsResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.ListNginxConfigs(ctx, &pb.ListNginxConfigsRequest{
		Keyword: keyword,
	})
}

// ValidateNginxConfig 校验 nginx 配置
func (c *AgentClient) ValidateNginxConfig(ctx context.Context, address string) (*pb.ValidateNginxConfigResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.ValidateNginxConfig(ctx, &pb.ValidateNginxConfigRequest{})
}

// ReloadNginx 重载 nginx
func (c *AgentClient) ReloadNginx(ctx context.Context, address string) (*pb.ReloadNginxResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.ReloadNginx(ctx, &pb.ReloadNginxRequest{})
}

// ==================== Systemd 方法 ====================

// PutSystemdUnit 创建或更新 systemd unit
func (c *AgentClient) PutSystemdUnit(ctx context.Context, address, name, content string, daemonReload bool) (*pb.PutSystemdUnitResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.PutSystemdUnit(ctx, &pb.PutSystemdUnitRequest{
		Name:         name,
		Content:      content,
		DaemonReload: daemonReload,
	})
}

// GetSystemdUnit 读取 systemd unit
func (c *AgentClient) GetSystemdUnit(ctx context.Context, address, name string) (*pb.GetSystemdUnitResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.GetSystemdUnit(ctx, &pb.GetSystemdUnitRequest{
		Name: name,
	})
}

// DeleteSystemdUnit 删除 systemd unit
func (c *AgentClient) DeleteSystemdUnit(ctx context.Context, address, name string, stopService, disableService, daemonReload bool) (*pb.DeleteSystemdUnitResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.DeleteSystemdUnit(ctx, &pb.DeleteSystemdUnitRequest{
		Name:           name,
		StopService:    stopService,
		DisableService: disableService,
		DaemonReload:   daemonReload,
	})
}

// ListSystemdUnits 列出 systemd units
func (c *AgentClient) ListSystemdUnits(ctx context.Context, address, keyword string) (*pb.ListSystemdUnitsResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.ListSystemdUnits(ctx, &pb.ListSystemdUnitsRequest{
		Keyword: keyword,
	})
}

// SystemdDaemonReload 执行 daemon-reload
func (c *AgentClient) SystemdDaemonReload(ctx context.Context, address string) (*pb.SystemdDaemonReloadResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.SystemdDaemonReload(ctx, &pb.SystemdDaemonReloadRequest{})
}

// SystemdServiceControl 控制服务
func (c *AgentClient) SystemdServiceControl(ctx context.Context, address, name, action string) (*pb.SystemdServiceControlResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.SystemdServiceControl(ctx, &pb.SystemdServiceControlRequest{
		Name:   name,
		Action: action,
	})
}

// GetSystemdServiceStatus 获取服务状态
func (c *AgentClient) GetSystemdServiceStatus(ctx context.Context, address, name string) (*pb.GetSystemdServiceStatusResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.GetSystemdServiceStatus(ctx, &pb.GetSystemdServiceStatusRequest{
		Name: name,
	})
}

// GetSystemdServiceLogs 获取服务日志
func (c *AgentClient) GetSystemdServiceLogs(ctx context.Context, address, name string, lines int, since, until string) (*pb.GetSystemdServiceLogsResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.GetSystemdServiceLogs(ctx, &pb.GetSystemdServiceLogsRequest{
		Name:  name,
		Lines: int32(lines),
		Since: since,
		Until: until,
	})
}

// ==================== SSL 方法 ====================

// DeploySSLCertificate 部署 SSL 证书
func (c *AgentClient) DeploySSLCertificate(ctx context.Context, address, basePath, fullchainName, privkeyName, fullchainPem, privkeyPem, fileMode string) (*pb.DeploySSLCertificateResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.DeploySSLCertificate(ctx, &pb.DeploySSLCertificateRequest{
		BasePath:      basePath,
		FullchainName: fullchainName,
		PrivkeyName:   privkeyName,
		FullchainPem:  fullchainPem,
		PrivkeyPem:    privkeyPem,
		FileMode:      fileMode,
	})
}

// ReloadService 重载服务
func (c *AgentClient) ReloadService(ctx context.Context, address, serviceType, serviceName string) (*pb.ReloadServiceResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.ReloadService(ctx, &pb.ReloadServiceRequest{
		ServiceType: serviceType,
		ServiceName: serviceName,
	})
}

// HealthCheck 健康检查
func (c *AgentClient) HealthCheck(ctx context.Context, address string) (*pb.HealthCheckResponse, error) {
	conn, err := c.getConn(address)
	if err != nil {
		return nil, err
	}

	client := pb.NewAgentServiceClient(conn)
	ctx = c.withAuth(ctx)
	ctx, cancel := c.withTimeout(ctx, 0)
	defer cancel()

	return client.HealthCheck(ctx, &pb.HealthCheckRequest{})
}
