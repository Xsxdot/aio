package client

import (
	"context"

	"xiaozhizhang/system/nginx/internal/app"
	"xiaozhizhang/system/nginx/internal/model/dto"
)

// NginxClientAgentified Nginx Agent化客户端（对外门面）
type NginxClientAgentified struct {
	app *app.AppAgentified
}

// NewNginxClientAgentified 创建 Nginx Agent化客户端
func NewNginxClientAgentified(app *app.AppAgentified) *NginxClientAgentified {
	return &NginxClientAgentified{
		app: app,
	}
}

// CreateConfig 创建配置文件
func (c *NginxClientAgentified) CreateConfig(ctx context.Context, serverID int64, req *dto.CreateConfigRequest) error {
	return c.app.CreateConfig(ctx, serverID, req)
}

// UpdateConfig 更新配置文件
func (c *NginxClientAgentified) UpdateConfig(ctx context.Context, serverID int64, name string, req *dto.UpdateConfigRequest) error {
	return c.app.UpdateConfig(ctx, serverID, name, req)
}

// DeleteConfig 删除配置文件
func (c *NginxClientAgentified) DeleteConfig(ctx context.Context, serverID int64, name string) error {
	return c.app.DeleteConfig(ctx, serverID, name)
}

// GetConfig 获取配置文件
func (c *NginxClientAgentified) GetConfig(ctx context.Context, serverID int64, name string) (*dto.ConfigInfo, error) {
	return c.app.GetConfig(ctx, serverID, name)
}

// ListConfigs 列出配置文件
func (c *NginxClientAgentified) ListConfigs(ctx context.Context, serverID int64, req *dto.QueryConfigRequest) ([]*dto.ConfigListItem, int64, error) {
	return c.app.ListConfigs(ctx, serverID, req)
}

// CreateConfigByParams 按参数生成并创建配置文件
func (c *NginxClientAgentified) CreateConfigByParams(ctx context.Context, serverID int64, req *dto.CreateConfigByParamsRequest) error {
	return c.app.CreateConfigByParams(ctx, serverID, req)
}

// UpdateConfigByParams 按参数生成并更新配置文件
func (c *NginxClientAgentified) UpdateConfigByParams(ctx context.Context, serverID int64, name string, req *dto.UpdateConfigByParamsRequest) error {
	return c.app.UpdateConfigByParams(ctx, serverID, name, req)
}

