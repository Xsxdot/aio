package client

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/application/api/dto"
	internalapp "xiaozhizhang/system/application/internal/app"
	internalmodel "xiaozhizhang/system/application/internal/model"
	internaldto "xiaozhizhang/system/application/internal/model/dto"
)

// ApplicationClient Application 组件对外客户端（进程内调用）
// 对外只暴露 api/dto，禁止泄漏 internal/model。
type ApplicationClient struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
}

// NewApplicationClient 创建 Application 客户端实例
func NewApplicationClient(app *internalapp.App) *ApplicationClient {
	return &ApplicationClient{
		app: app,
		err: errorc.NewErrorBuilder("ApplicationClient"),
	}
}

// CreateApplication 创建应用
func (c *ApplicationClient) CreateApplication(ctx context.Context, req *dto.CreateApplicationReq) (*dto.ApplicationDTO, error) {
	internalReq := &internaldto.CreateApplicationRequest{
		Name:        req.Name,
		Project:     req.Project,
		Env:         req.Env,
		Type:        internalmodel.ApplicationType(req.Type),
		Domain:      req.Domain,
		Port:        req.Port,
		SSL:         req.SSL,
		InstallPath: req.InstallPath,
		Owner:       req.Owner,
		Description: req.Description,
	}

	app, err := c.app.CreateApplication(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return c.toDTO(app), nil
}

// GetApplication 获取应用
func (c *ApplicationClient) GetApplication(ctx context.Context, id int64) (*dto.ApplicationDTO, error) {
	app, err := c.app.GetApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.toDTO(app), nil
}

// GetApplicationByKey 根据唯一键获取应用
func (c *ApplicationClient) GetApplicationByKey(ctx context.Context, project, name, env string) (*dto.ApplicationDTO, error) {
	app, err := c.app.GetApplicationByKey(ctx, project, name, env)
	if err != nil {
		return nil, err
	}
	return c.toDTO(app), nil
}

// toDTO 转换为对外 DTO
func (c *ApplicationClient) toDTO(app *internalmodel.Application) *dto.ApplicationDTO {
	if app == nil {
		return nil
	}
	return &dto.ApplicationDTO{
		ID:               app.ID,
		Name:             app.Name,
		Project:          app.Project,
		Env:              app.Env,
		Type:             string(app.Type),
		Domain:           app.Domain,
		Port:             app.Port,
		SSL:              app.SSL,
		InstallPath:      app.InstallPath,
		Owner:            app.Owner,
		Description:      app.Description,
		Status:           app.Status,
		CurrentReleaseID: app.CurrentReleaseID,
		CreatedAt:        app.CreatedAt,
		UpdatedAt:        app.UpdatedAt,
	}
}

