package app

import (
	"context"
	"fmt"

	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/systemd/internal/facade"
	"xiaozhizhang/system/systemd/internal/model/dto"
	"xiaozhizhang/system/systemd/internal/service"
)

// AppAgentified Systemd Agent化的应用层
type AppAgentified struct {
	UnitGeneratorService *service.UnitGeneratorService
	serverFacade         facade.IServerFacade
	log                  *logger.Log
	err                  *errorc.ErrorBuilder
}

// NewAppAgentified 创建 Systemd Agent化应用实例
func NewAppAgentified(serverFacade facade.IServerFacade) *AppAgentified {
	log := base.Logger.WithEntryName("SystemdApp")
	unitGeneratorSvc := service.NewUnitGeneratorService(log)

	return &AppAgentified{
		UnitGeneratorService: unitGeneratorSvc,
		serverFacade:         serverFacade,
		log:                  log,
		err:                  errorc.NewErrorBuilder("SystemdApp"),
	}
}

// CreateService 创建服务（通过 agent）
func (a *AppAgentified) CreateService(ctx context.Context, serverID int64, req *dto.CreateServiceRequest) error {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	resp, err := base.AgentClient.PutSystemdUnit(ctx, agentAddr, req.Name, req.Content, true)
	if err != nil {
		return a.err.New("调用 agent 创建服务失败", err)
	}

	if !resp.Success {
		return a.err.New(resp.Message, nil)
	}

	a.log.WithFields(map[string]interface{}{
		"server_id": serverID,
		"name":      req.Name,
	}).Info("服务创建成功")
	return nil
}

// UpdateService 更新服务（通过 agent）
func (a *AppAgentified) UpdateService(ctx context.Context, serverID int64, name string, req *dto.UpdateServiceRequest) error {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	resp, err := base.AgentClient.PutSystemdUnit(ctx, agentAddr, name, req.Content, true)
	if err != nil {
		return a.err.New("调用 agent 更新服务失败", err)
	}

	if !resp.Success {
		return a.err.New(resp.Message, nil)
	}

	a.log.WithFields(map[string]interface{}{
		"server_id": serverID,
		"name":      name,
	}).Info("服务更新成功")
	return nil
}

// DeleteService 删除服务（通过 agent）
func (a *AppAgentified) DeleteService(ctx context.Context, serverID int64, name string, force bool) error {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	resp, err := base.AgentClient.DeleteSystemdUnit(ctx, agentAddr, name, !force, !force, true)
	if err != nil && !force {
		return a.err.New("调用 agent 删除服务失败", err)
	}

	if resp != nil && !resp.Success && !force {
		return a.err.New(resp.Message, nil)
	}

	a.log.WithFields(map[string]interface{}{
		"server_id": serverID,
		"name":      name,
	}).Info("服务删除成功")
	return nil
}

// GetService 获取服务信息（通过 agent）
func (a *AppAgentified) GetService(ctx context.Context, serverID int64, name string) (*dto.ServiceInfo, error) {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return nil, err
	}

	resp, err := base.AgentClient.GetSystemdUnit(ctx, agentAddr, name)
	if err != nil {
		return nil, a.err.New("调用 agent 获取服务失败", err)
	}

	return &dto.ServiceInfo{
		Name:        resp.Name,
		Content:     resp.Content,
		Description: resp.Description,
		ModTime:     fmt.Sprintf("%d", resp.ModTime),
	}, nil
}

// ListServices 列出服务（通过 agent）
func (a *AppAgentified) ListServices(ctx context.Context, serverID int64, req *dto.QueryServiceRequest) ([]*dto.ServiceListItem, int64, error) {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return nil, 0, err
	}

	resp, err := base.AgentClient.ListSystemdUnits(ctx, agentAddr, req.Keyword)
	if err != nil {
		return nil, 0, a.err.New("调用 agent 列出服务失败", err)
	}

	result := make([]*dto.ServiceListItem, len(resp.Units))
	for i, unit := range resp.Units {
		item := &dto.ServiceListItem{
			Name:        unit.Name,
			Description: unit.Description,
			ModTime:     fmt.Sprintf("%d", unit.ModTime),
		}

		// 如果需要包含状态信息
		if req.IncludeStatus {
			status, err := base.AgentClient.GetSystemdServiceStatus(ctx, agentAddr, unit.Name)
			if err == nil {
				item.ActiveState = status.ActiveState
				item.SubState = status.SubState
				item.UnitFileState = status.UnitFileState
			}
		}

		result[i] = item
	}

	total := int64(len(result))

	// 简单的内存分页
	pageNum := req.PageNum
	if pageNum <= 0 {
		pageNum = 1
	}
	size := req.Size
	if size <= 0 {
		size = 20
	}

	start := (pageNum - 1) * size
	end := start + size
	if start >= int(total) {
		return []*dto.ServiceListItem{}, total, nil
	}
	if end > int(total) {
		end = int(total)
	}

	return result[start:end], total, nil
}

// ControlService 控制服务（通过 agent）
func (a *AppAgentified) ControlService(ctx context.Context, serverID int64, name, action string) error {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	resp, err := base.AgentClient.SystemdServiceControl(ctx, agentAddr, name, action)
	if err != nil {
		return a.err.New(fmt.Sprintf("调用 agent %s 服务失败", action), err)
	}

	if !resp.Success {
		return a.err.New(fmt.Sprintf("%s 服务失败: %s", action, resp.Output), nil)
	}

	return nil
}

// SetServiceEnabled 设置服务启用/禁用状态（通过 agent）
func (a *AppAgentified) SetServiceEnabled(ctx context.Context, serverID int64, name string, enable bool) error {
	action := "disable"
	if enable {
		action = "enable"
	}
	return a.ControlService(ctx, serverID, name, action)
}

// GetServiceStatus 获取服务状态（通过 agent）
func (a *AppAgentified) GetServiceStatus(ctx context.Context, serverID int64, name string) (*dto.ServiceStatus, error) {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return nil, err
	}

	resp, err := base.AgentClient.GetSystemdServiceStatus(ctx, agentAddr, name)
	if err != nil {
		return nil, a.err.New("调用 agent 获取服务状态失败", err)
	}

	return &dto.ServiceStatus{
		Name:              resp.Name,
		Description:       resp.Description,
		LoadState:         resp.LoadState,
		ActiveState:       resp.ActiveState,
		SubState:          resp.SubState,
		UnitFileState:     resp.UnitFileState,
		MainPID:           int(resp.MainPid),
		ExecMainStartAt:   resp.ExecMainStartAt,
		MemoryCurrent:     resp.MemoryCurrent,
		Result:            resp.Result,
	}, nil
}

// GetServiceLogs 获取服务日志（通过 agent）
func (a *AppAgentified) GetServiceLogs(ctx context.Context, serverID int64, name string, req *dto.LogsRequest) (*dto.ServiceLogs, error) {
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return nil, err
	}

	lines := req.Lines
	if lines <= 0 {
		lines = 200
	}

	resp, err := base.AgentClient.GetSystemdServiceLogs(ctx, agentAddr, name, lines, req.Since, req.Until)
	if err != nil {
		return nil, a.err.New("调用 agent 获取服务日志失败", err)
	}

	return &dto.ServiceLogs{
		Name:  name,
		Lines: resp.LogLines,
	}, nil
}

// GenerateService 生成 service unit 内容（本地操作）
func (a *AppAgentified) GenerateService(_ context.Context, req *dto.GenerateServiceRequest) (*dto.GenerateServiceResponse, error) {
	content, err := a.UnitGeneratorService.Generate(&req.Params)
	if err != nil {
		return nil, err
	}

	return &dto.GenerateServiceResponse{
		Content: content,
	}, nil
}

// CreateServiceFromParams 按参数创建 service
func (a *AppAgentified) CreateServiceFromParams(ctx context.Context, serverID int64, req *dto.CreateServiceFromParamsRequest) error {
	content, err := a.UnitGeneratorService.Generate(&req.Params)
	if err != nil {
		return err
	}

	createReq := &dto.CreateServiceRequest{
		Name:    req.Name,
		Content: content,
	}
	return a.CreateService(ctx, serverID, createReq)
}

// UpdateServiceFromParams 按参数更新 service
func (a *AppAgentified) UpdateServiceFromParams(ctx context.Context, serverID int64, name string, req *dto.UpdateServiceFromParamsRequest) error {
	content, err := a.UnitGeneratorService.Generate(&req.Params)
	if err != nil {
		return err
	}

	updateReq := &dto.UpdateServiceRequest{
		Content: content,
	}
	return a.UpdateService(ctx, serverID, name, updateReq)
}

// getAgentAddress 获取服务器的 agent 地址
func (a *AppAgentified) getAgentAddress(ctx context.Context, serverID int64) (string, error) {
	serverInfo, err := a.serverFacade.GetServerAgentInfo(ctx, serverID)
	if err != nil {
		return "", a.err.New("获取服务器信息失败", err)
	}

	if serverInfo.AgentGrpcAddress == "" {
		return "", a.err.New(fmt.Sprintf("服务器 %d 未配置 Agent 地址", serverID), nil).ValidWithCtx()
	}

	return serverInfo.AgentGrpcAddress, nil
}

