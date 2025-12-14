package app

import (
	"context"

	"xiaozhizhang/system/application/internal/model"
	"xiaozhizhang/system/application/internal/model/dto"
)

// CreateApplication 创建应用
func (a *App) CreateApplication(ctx context.Context, req *dto.CreateApplicationRequest) (*model.Application, error) {
	app := &model.Application{
		Name:        req.Name,
		Project:     req.Project,
		Env:         req.Env,
		Type:        req.Type,
		Domain:      req.Domain,
		Port:        req.Port,
		SSL:         req.SSL,
		InstallPath: req.InstallPath,
		Owner:       req.Owner,
		Description: req.Description,
		Status:      1,
	}

	if err := a.ApplicationSvc.Create(ctx, app); err != nil {
		return nil, err
	}

	return app, nil
}

// GetApplication 获取应用
func (a *App) GetApplication(ctx context.Context, id int64) (*model.Application, error) {
	return a.ApplicationSvc.FindByID(ctx, id)
}

// GetApplicationByKey 根据唯一键获取应用
func (a *App) GetApplicationByKey(ctx context.Context, project, name, env string) (*model.Application, error) {
	return a.ApplicationSvc.FindByKey(ctx, project, name, env)
}

// ListApplications 列出应用
func (a *App) ListApplications(ctx context.Context, req *dto.QueryApplicationRequest) ([]*model.Application, error) {
	return a.ApplicationSvc.ListByFilter(ctx, req.Project, req.Env, req.Type, req.Keyword)
}

// UpdateApplication 更新应用
func (a *App) UpdateApplication(ctx context.Context, id int64, req *dto.UpdateApplicationRequest) (*model.Application, error) {
	updates := &model.Application{}

	if req.Name != "" {
		updates.Name = req.Name
	}
	if req.Domain != "" {
		updates.Domain = req.Domain
	}
	if req.Port > 0 {
		updates.Port = req.Port
	}
	if req.SSL != nil {
		updates.SSL = *req.SSL
	}
	if req.InstallPath != "" {
		updates.InstallPath = req.InstallPath
	}
	if req.Owner != "" {
		updates.Owner = req.Owner
	}
	if req.Description != "" {
		updates.Description = req.Description
	}
	if req.Status != nil {
		updates.Status = *req.Status
	}

	return a.ApplicationSvc.Update(ctx, id, updates)
}

// DeleteApplication 删除应用
func (a *App) DeleteApplication(ctx context.Context, id int64) error {
	return a.ApplicationSvc.Delete(ctx, id)
}

// ListReleases 列出应用的版本
func (a *App) ListReleases(ctx context.Context, applicationID int64, limit int) ([]*model.Release, error) {
	return a.ReleaseSvc.ListByApplicationID(ctx, applicationID, limit)
}

// ListDeployments 列出应用的部署记录
func (a *App) ListDeployments(ctx context.Context, applicationID int64, limit int) ([]*model.Deployment, error) {
	return a.DeploymentSvc.ListByApplicationID(ctx, applicationID, limit)
}

// GetDeployment 获取部署详情
func (a *App) GetDeployment(ctx context.Context, id int64) (*model.Deployment, error) {
	return a.DeploymentSvc.FindByID(ctx, id)
}

