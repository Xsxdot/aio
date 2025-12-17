package sdk

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config SDK 配置
type Config struct {
	// RegistryAddr 注册中心地址（必填）
	RegistryAddr string
	// ClientKey 客户端认证密钥（必填）
	ClientKey string
	// ClientSecret 客户端认证密文（必填）
	ClientSecret string
	// DefaultTimeout 默认超时时间
	DefaultTimeout time.Duration
	// DisableAuth 禁用自动鉴权（用于特殊场景）
	DisableAuth bool
}

// Client SDK 客户端
type Client struct {
	config Config
	conn   *grpc.ClientConn

	// Auth 鉴权客户端
	Auth *AuthClient
	// Registry 注册中心客户端
	Registry *RegistryClient
	// Discovery 服务发现客户端
	Discovery *DiscoveryClient

	// ConfigClient 配置中心客户端
	ConfigClient *ConfigClient
	// ShortURL 短网址客户端
	ShortURL *ShortURLClient
	// Application 应用部署客户端
	Application *ApplicationClient
}

// New 创建 SDK 客户端
func New(config Config) (*Client, error) {
	// 验证配置
	if config.RegistryAddr == "" {
		return nil, fmt.Errorf("RegistryAddr is required")
	}
	if config.ClientKey == "" {
		return nil, fmt.Errorf("ClientKey is required")
	}
	if config.ClientSecret == "" {
		return nil, fmt.Errorf("ClientSecret is required")
	}

	// 设置默认值
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	client := &Client{
		config: config,
	}

	// 初始化 Auth 客户端（不需要 token）
	authClient, err := newAuthClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}
	client.Auth = authClient

	// 建立连接（带 token provider）
	conn, err := client.dialWithAuth(config.RegistryAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial registry: %w", err)
	}
	client.conn = conn

	// 初始化 Registry 客户端
	registryClient, err := newRegistryClient(client, conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}
	client.Registry = registryClient

	// 初始化 Discovery 客户端
	discoveryClient := newDiscoveryClient(client)
	client.Discovery = discoveryClient

	// 初始化 Config 客户端
	configClient := newConfigClient(conn)
	client.ConfigClient = configClient

	// 初始化 ShortURL 客户端
	shortURLClient := newShortURLClient(conn)
	client.ShortURL = shortURLClient

	// 初始化 Application 客户端
	applicationClient := newApplicationClient(conn)
	client.Application = applicationClient

	return client, nil
}

// dialWithAuth 建立带鉴权的 gRPC 连接
func (c *Client) dialWithAuth(target string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	// 使用不安全连接（生产环境应该使用 TLS）
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	// 添加拦截器
	if !c.config.DisableAuth {
		opts = append(opts,
			grpc.WithUnaryInterceptor(c.unaryAuthInterceptor()),
			grpc.WithStreamInterceptor(c.streamAuthInterceptor()),
		)
	}

	// 建立连接
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", target, err)
	}

	return conn, nil
}

// Close 关闭 SDK 客户端
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// DefaultContext 创建带默认超时的 context
func (c *Client) DefaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.config.DefaultTimeout)
}
