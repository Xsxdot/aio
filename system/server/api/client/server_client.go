package client

import (
	"context"
	"xiaozhizhang/system/server/internal/app"
	"xiaozhizhang/system/server/internal/model/dto"
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
func (c *ServerClient) GetAllServerStatus(ctx context.Context) ([]*dto.ServerStatusInfo, error) {
	return c.app.GetAllServerStatus(ctx)
}

// GetServerStatusByID 获取单个服务器状态
func (c *ServerClient) GetServerStatusByID(ctx context.Context, serverID int64) (*dto.ServerStatusInfo, error) {
	return c.app.GetServerStatusByID(ctx, serverID)
}


