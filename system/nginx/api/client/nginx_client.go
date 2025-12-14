package client

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/nginx/api/dto"
	internalapp "xiaozhizhang/system/nginx/internal/app"
	internaldto "xiaozhizhang/system/nginx/internal/model/dto"
)

// NginxClient Nginx 管理组件对外客户端（进程内调用）
// 供其他组件调用 Nginx 配置管理能力
type NginxClient struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
}

// NewNginxClient 创建 Nginx 客户端实例
func NewNginxClient(app *internalapp.App) *NginxClient {
	return &NginxClient{
		app: app,
		err: errorc.NewErrorBuilder("NginxClient"),
	}
}

// CreateConfig 创建配置文件
func (c *NginxClient) CreateConfig(ctx context.Context, req *dto.CreateConfigReq) error {
	internalReq := &internaldto.CreateConfigRequest{
		Name:    req.Name,
		Content: req.Content,
	}
	return c.app.CreateConfig(ctx, internalReq)
}

// UpdateConfig 更新配置文件
func (c *NginxClient) UpdateConfig(ctx context.Context, name string, req *dto.UpdateConfigReq) error {
	internalReq := &internaldto.UpdateConfigRequest{
		Content: req.Content,
	}
	return c.app.UpdateConfig(ctx, name, internalReq)
}

// DeleteConfig 删除配置文件
func (c *NginxClient) DeleteConfig(ctx context.Context, name string) error {
	return c.app.DeleteConfig(ctx, name)
}

// GetConfig 获取配置文件信息
func (c *NginxClient) GetConfig(ctx context.Context, name string) (*dto.ConfigDTO, error) {
	info, err := c.app.GetConfig(ctx, name)
	if err != nil {
		return nil, err
	}
	return &dto.ConfigDTO{
		Name:        info.Name,
		Content:     info.Content,
		Description: info.Description,
		ModTime:     info.ModTime,
	}, nil
}

// ListConfigs 列出配置文件
func (c *NginxClient) ListConfigs(ctx context.Context, req *dto.QueryConfigReq) ([]*dto.ConfigListItemDTO, int64, error) {
	internalReq := &internaldto.QueryConfigRequest{
		Keyword: req.Keyword,
		PageNum: req.PageNum,
		Size:    req.Size,
	}

	items, total, err := c.app.ListConfigs(ctx, internalReq)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*dto.ConfigListItemDTO, len(items))
	for i, item := range items {
		result[i] = &dto.ConfigListItemDTO{
			Name:        item.Name,
			Description: item.Description,
			ModTime:     item.ModTime,
		}
	}

	return result, total, nil
}

// CreateConfigByParams 按参数生成并创建配置文件
func (c *NginxClient) CreateConfigByParams(ctx context.Context, req *dto.CreateConfigByParamsReq) error {
	internalReq := &internaldto.CreateConfigByParamsRequest{
		Name: req.Name,
		Spec: req.Spec,
	}
	return c.app.CreateConfigByParams(ctx, internalReq)
}

// UpdateConfigByParams 按参数生成并更新配置文件
func (c *NginxClient) UpdateConfigByParams(ctx context.Context, name string, req *dto.UpdateConfigByParamsReq) error {
	internalReq := &internaldto.UpdateConfigByParamsRequest{
		Spec: req.Spec,
	}
	return c.app.UpdateConfigByParams(ctx, name, internalReq)
}
