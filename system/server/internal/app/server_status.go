package app

import (
	"context"
	"time"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/server/internal/model"
	"xiaozhizhang/system/server/internal/model/dto"
)

// ReportServerStatus 上报服务器状态
func (a *App) ReportServerStatus(ctx context.Context, req *dto.ReportServerStatusRequest) error {
	// 验证服务器是否存在
	server, err := a.ServerService.FindById(ctx, req.ServerID)
	if err != nil {
		return err
	}
	
	if server == nil {
		return errorc.NewErrorBuilder("ServerApp").New("服务器不存在", nil).NotFound()
	}

	// 如果 CollectedAt 为零值，使用当前时间
	if req.CollectedAt.IsZero() {
		req.CollectedAt = time.Now()
	}

	// 上报状态
	return a.ServerStatusService.ReportStatus(ctx, req)
}

// GetAllServerStatus 获取所有服务器状态（主页聚合查询）
func (a *App) GetAllServerStatus(ctx context.Context) ([]*dto.ServerStatusInfo, error) {
	// 查询所有服务器
	servers, err := a.ServerService.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return []*dto.ServerStatusInfo{}, nil
	}

	// 提取所有服务器 ID
	serverIDs := make([]int64, 0, len(servers))
	for _, server := range servers {
		serverIDs = append(serverIDs, server.ID)
	}

	// 批量查询状态
	statusMap, err := a.ServerStatusService.ListByServerIDs(ctx, serverIDs)
	if err != nil {
		return nil, err
	}

	// 组装结果
	result := make([]*dto.ServerStatusInfo, 0, len(servers))
	for _, server := range servers {
		info := a.buildServerStatusInfo(server, statusMap[server.ID])
		result = append(result, info)
	}

	return result, nil
}

// GetServerStatusByID 获取单个服务器状态
func (a *App) GetServerStatusByID(ctx context.Context, serverID int64) (*dto.ServerStatusInfo, error) {
	// 查询服务器
	server, err := a.ServerService.FindById(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 查询状态
	status, err := a.ServerStatusService.FindByServerID(ctx, serverID)
	if err != nil && !errorc.IsNotFound(err) {
		return nil, err
	}

	return a.buildServerStatusInfo(server, status), nil
}

// buildServerStatusInfo 构建服务器状态信息
func (a *App) buildServerStatusInfo(server *model.ServerModel, status *model.ServerStatusModel) *dto.ServerStatusInfo {
	info := &dto.ServerStatusInfo{
		ID:               server.ID,
		Name:             server.Name,
		Host:             server.Host,
		AgentGrpcAddress: server.AgentGrpcAddress,
		Enabled:          server.Enabled,
		Comment:          server.Comment,
	}

	// 解析标签
	if len(server.Tags) > 0 {
		tags := make(map[string]string)
		for k, v := range server.Tags {
			if strVal, ok := v.(string); ok {
				tags[k] = strVal
			}
		}
		info.Tags = tags
	}

	// 如果没有状态，标记为 unknown
	if status == nil {
		info.StatusSummary = "unknown"
		return info
	}

	// 填充状态信息
	info.CPUPercent = &status.CPUPercent
	info.MemUsed = &status.MemUsed
	info.MemTotal = &status.MemTotal
	info.Load1 = &status.Load1
	info.Load5 = &status.Load5
	info.Load15 = &status.Load15
	info.CollectedAt = &status.CollectedAt
	info.ReportedAt = &status.ReportedAt
	info.ErrorMessage = status.ErrorMessage

	// 解析磁盘信息
	if len(status.DiskItems) > 0 {
		info.DiskItems = make([]dto.DiskItemDTO, 0, len(status.DiskItems))
		for _, item := range status.DiskItems {
			info.DiskItems = append(info.DiskItems, dto.DiskItemDTO{
				MountPoint: item.MountPoint,
				Used:       item.Used,
				Total:      item.Total,
				Percent:    item.Percent,
			})
		}
	}

	// 判断状态：如果最后上报时间距今超过 5 分钟，视为 offline
	if time.Since(status.ReportedAt) > 5*time.Minute {
		info.StatusSummary = "offline"
	} else if status.ErrorMessage != "" {
		info.StatusSummary = "error"
	} else {
		info.StatusSummary = "online"
	}

	return info
}

