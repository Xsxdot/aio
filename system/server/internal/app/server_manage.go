package app

import (
	"context"
	"xiaozhizhang/system/server/internal/model"
	"xiaozhizhang/system/server/internal/model/dto"
)

// CreateServer 创建服务器
func (a *App) CreateServer(ctx context.Context, req *dto.CreateServerRequest) (*model.ServerModel, error) {
	return a.ServerService.Create(ctx, req)
}

// UpdateServer 更新服务器
func (a *App) UpdateServer(ctx context.Context, id int64, req *dto.UpdateServerRequest) error {
	return a.ServerService.Update(ctx, id, req)
}

// DeleteServer 删除服务器
func (a *App) DeleteServer(ctx context.Context, id int64) error {
	// 删除服务器时也删除对应的状态记录
	// 注意：这里简化处理，直接删除。实际可考虑软删除或保留历史状态
	
	// 先尝试删除状态（可能不存在）
	status, err := a.ServerStatusService.FindByServerID(ctx, id)
	if err == nil && status != nil {
		_ = a.ServerStatusService.DeleteById(ctx, status.ID)
	}
	
	// 删除服务器
	return a.ServerService.DeleteById(ctx, id)
}

// GetServer 获取服务器详情
func (a *App) GetServer(ctx context.Context, id int64) (*model.ServerModel, error) {
	return a.ServerService.FindById(ctx, id)
}

// QueryServers 分页查询服务器
func (a *App) QueryServers(ctx context.Context, req *dto.QueryServerRequest) ([]*model.ServerModel, int64, error) {
	return a.ServerService.QueryWithPage(ctx, req)
}

