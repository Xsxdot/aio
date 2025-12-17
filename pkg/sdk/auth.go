package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	userpb "xiaozhizhang/system/user/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthClient 鉴权客户端
type AuthClient struct {
	client      *Client
	authConn    *grpc.ClientConn
	authService userpb.ClientAuthServiceClient

	// Token 缓存
	mu          sync.RWMutex
	token       string
	expiresAt   int64
	refreshing  bool
	refreshCond *sync.Cond
}

// newAuthClient 创建鉴权客户端
func newAuthClient(client *Client) (*AuthClient, error) {
	// 建立不带鉴权的连接（用于获取 token）
	conn, err := grpc.Dial(
		client.config.RegistryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial for auth: %w", err)
	}

	ac := &AuthClient{
		client:      client,
		authConn:    conn,
		authService: userpb.NewClientAuthServiceClient(conn),
	}
	ac.refreshCond = sync.NewCond(&ac.mu)

	return ac, nil
}

// Token 获取有效的 token（自动续期）
func (ac *AuthClient) Token(ctx context.Context) (string, error) {
	ac.mu.RLock()

	// 如果 token 有效且未过期（提前 5 分钟刷新）
	now := time.Now().Unix()
	if ac.token != "" && ac.expiresAt > now+300 {
		token := ac.token
		ac.mu.RUnlock()
		return token, nil
	}

	// 如果正在刷新，等待刷新完成
	if ac.refreshing {
		ac.mu.RUnlock()
		ac.mu.Lock()
		for ac.refreshing {
			ac.refreshCond.Wait()
		}
		token := ac.token
		expiresAt := ac.expiresAt
		ac.mu.Unlock()

		if token != "" && expiresAt > now {
			return token, nil
		}
		return "", fmt.Errorf("token refresh failed")
	}

	ac.mu.RUnlock()

	// 需要刷新
	return ac.refreshToken(ctx)
}

// refreshToken 刷新 token
func (ac *AuthClient) refreshToken(ctx context.Context) (string, error) {
	ac.mu.Lock()

	// 双重检查
	now := time.Now().Unix()
	if ac.token != "" && ac.expiresAt > now+300 {
		token := ac.token
		ac.mu.Unlock()
		return token, nil
	}

	// 标记正在刷新
	ac.refreshing = true
	ac.mu.Unlock()

	defer func() {
		ac.mu.Lock()
		ac.refreshing = false
		ac.refreshCond.Broadcast()
		ac.mu.Unlock()
	}()

	// 优先尝试续期
	if ac.token != "" && ac.expiresAt > now {
		token, expiresAt, err := ac.renewToken(ctx)
		if err == nil {
			ac.mu.Lock()
			ac.token = token
			ac.expiresAt = expiresAt
			ac.mu.Unlock()
			return token, nil
		}
		// 续期失败，fallback 到重新认证
	}

	// 重新认证
	token, expiresAt, err := ac.authenticate(ctx)
	if err != nil {
		return "", err
	}

	ac.mu.Lock()
	ac.token = token
	ac.expiresAt = expiresAt
	ac.mu.Unlock()

	return token, nil
}

// authenticate 认证获取 token
func (ac *AuthClient) authenticate(ctx context.Context) (string, int64, error) {
	req := &userpb.AuthenticateClientRequest{
		ClientKey:    ac.client.config.ClientKey,
		ClientSecret: ac.client.config.ClientSecret,
	}

	resp, err := ac.authService.AuthenticateClient(ctx, req)
	if err != nil {
		return "", 0, WrapError(err, "authenticate failed")
	}

	return resp.AccessToken, resp.ExpiresAt, nil
}

// renewToken 续期 token
func (ac *AuthClient) renewToken(ctx context.Context) (string, int64, error) {
	// 注入当前 token 到 metadata
	ctx = ac.client.injectToken(ctx, ac.token)

	req := &userpb.RenewTokenRequest{}

	resp, err := ac.authService.RenewToken(ctx, req)
	if err != nil {
		return "", 0, WrapError(err, "renew token failed")
	}

	return resp.AccessToken, resp.ExpiresAt, nil
}

// Close 关闭鉴权客户端
func (ac *AuthClient) Close() error {
	if ac.authConn != nil {
		return ac.authConn.Close()
	}
	return nil
}
