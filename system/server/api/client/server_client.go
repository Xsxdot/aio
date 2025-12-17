package client

import (
	"context"
	"xiaozhizhang/system/server/api/dto"
	"xiaozhizhang/system/server/internal/app"
	internaldto "xiaozhizhang/system/server/internal/model/dto"
)

// ServerClient 服务器组件对外客户端
type ServerClient struct {
	app *app.App
}

// NewServerClient 创建服务器客户端
func NewServerClient(app *app.App) *ServerClient {
	return &ServerClient{
		app: app,
	}
}

// GetAllServerStatus 获取所有服务器状态
func (c *ServerClient) GetAllServerStatus(ctx context.Context) ([]*internaldto.ServerStatusInfo, error) {
	return c.app.GetAllServerStatus(ctx)
}

// GetServerStatusByID 获取单个服务器状态
func (c *ServerClient) GetServerStatusByID(ctx context.Context, serverID int64) (*internaldto.ServerStatusInfo, error) {
	return c.app.GetServerStatusByID(ctx, serverID)
}

// GetServerSSHConfigByID 获取服务器的 SSH 连接配置（已解密）
// 返回 host + SSH 凭证信息，供 SSL 组件部署证书时使用
func (c *ServerClient) GetServerSSHConfigByID(ctx context.Context, serverID int64) (*dto.ServerSSHConfig, error) {
	// 获取服务器基本信息
	server, err := c.app.GetServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 获取解密后的 SSH 凭证
	credential, err := c.app.GetDecryptedServerSSHCredential(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 组装返回
	return &dto.ServerSSHConfig{
		Host:       server.Host,
		Port:       credential.Port,
		Username:   credential.Username,
		AuthMethod: credential.AuthMethod,
		Password:   credential.Password,
		PrivateKey: credential.PrivateKey,
	}, nil
}

// GetServerAgentInfo 获取服务器 Agent 信息（用于路由 agent 请求）
func (c *ServerClient) GetServerAgentInfo(ctx context.Context, serverID int64) (*dto.ServerAgentInfo, error) {
	server, err := c.app.GetServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	return &dto.ServerAgentInfo{
		ID:               int64(server.ID),
		Name:             server.Name,
		Host:             server.Host,
		AgentGrpcAddress: server.AgentGrpcAddress,
	}, nil
}
