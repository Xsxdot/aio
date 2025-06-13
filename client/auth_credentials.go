package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	authv1 "github.com/xsxdot/aio/api/proto/auth/v1"
)

// AIOCredentials 实现 gRPC 的 PerRPCCredentials 接口
// 这是 gRPC 官方推荐的认证方式
type AIOCredentials struct {
	mu           sync.RWMutex
	clientID     string
	clientSecret string
	accessToken  string
	tokenExpiry  time.Time
	authClient   authv1.AuthServiceClient
	requireTLS   bool
}

// NewAIOCredentials 创建新的 AIO 认证凭据
func NewAIOCredentials(clientID, clientSecret string, authClient authv1.AuthServiceClient) *AIOCredentials {
	creds := &AIOCredentials{
		clientID:     clientID,
		clientSecret: clientSecret,
		authClient:   authClient,
		requireTLS:   false, // 根据实际需求配置
	}

	// 立即尝试获取一次 token，确保认证凭据可用
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := creds.ensureValidToken(ctx); err != nil {
		// 如果获取失败，记录错误但不阻止创建（后续调用时会重试）
		// 注意：这里不能返回错误，因为可能是暂时性的网络问题
		_ = err
	}

	return creds
}

// GetRequestMetadata 实现 PerRPCCredentials 接口
// 这个方法会在每个 gRPC 请求时自动调用
func (c *AIOCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	// 检查并刷新 token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, fmt.Errorf("获取有效 token 失败: %v", err)
	}

	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	return map[string]string{
		"authorization": "Bearer " + token,
	}, nil
}

// RequireTransportSecurity 实现 PerRPCCredentials 接口
func (c *AIOCredentials) RequireTransportSecurity() bool {
	return c.requireTLS
}

// SetRequireTLS 设置是否需要 TLS
func (c *AIOCredentials) SetRequireTLS(require bool) {
	c.requireTLS = require
}

// ensureValidToken 确保 token 有效，如果过期则自动刷新
func (c *AIOCredentials) ensureValidToken(ctx context.Context) error {
	c.mu.RLock()
	needRefresh := c.accessToken == "" || time.Now().Add(time.Minute).After(c.tokenExpiry)
	c.mu.RUnlock()

	if !needRefresh {
		return nil
	}

	// 获取写锁进行 token 刷新
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查，避免并发刷新
	if c.accessToken != "" && time.Now().Add(time.Minute).Before(c.tokenExpiry) {
		return nil
	}

	// 调用认证服务获取新 token
	resp, err := c.authClient.ClientAuth(ctx, &authv1.ClientAuthRequest{
		ClientId:     c.clientID,
		ClientSecret: c.clientSecret,
	})
	if err != nil {
		return fmt.Errorf("客户端认证失败: %v", err)
	}

	// 更新 token 信息
	c.accessToken = resp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)

	return nil
}

// IsTokenValid 检查 token 是否有效
func (c *AIOCredentials) IsTokenValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken != "" && time.Now().Before(c.tokenExpiry)
}

// UpdateAuthClient 更新认证客户端，解决连接重建后的引用问题
func (c *AIOCredentials) UpdateAuthClient(authClient authv1.AuthServiceClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authClient = authClient
}
