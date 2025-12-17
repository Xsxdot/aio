package app

import (
	"context"
	"fmt"

	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/nginx/internal/facade"
	"xiaozhizhang/system/nginx/internal/model/dto"
	"xiaozhizhang/system/nginx/internal/service"
)

// AppAgentified Nginx Agent化的应用层
// 只保留配置生成逻辑，文件操作全部通过 AgentClient 调用 agent
type AppAgentified struct {
	GenerateService *service.NginxConfigGenerateService
	serverFacade    facade.IServerFacade
	log             *logger.Log
	err             *errorc.ErrorBuilder
}

// NewAppAgentified 创建 Nginx Agent化应用实例
func NewAppAgentified(serverFacade facade.IServerFacade) *AppAgentified {
	log := base.Logger.WithEntryName("NginxApp")
	genSvc := service.NewNginxConfigGenerateService(log)

	return &AppAgentified{
		GenerateService: genSvc,
		serverFacade:    serverFacade,
		log:             log,
		err:             errorc.NewErrorBuilder("NginxApp"),
	}
}

// CreateConfig 创建配置文件（通过 agent）
func (a *AppAgentified) CreateConfig(ctx context.Context, serverID int64, req *dto.CreateConfigRequest) error {
	// 获取 agent 地址
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	// 调用 agent 创建配置
	resp, err := base.AgentClient.PutNginxConfig(ctx, agentAddr, req.Name, req.Content, true, true)
	if err != nil {
		return a.err.New("调用 agent 创建配置失败", err)
	}

	if !resp.Success {
		return a.err.New(resp.Message, nil)
	}

	a.log.WithFields(map[string]interface{}{
		"server_id": serverID,
		"name":      req.Name,
	}).Info("配置文件创建成功")
	return nil
}

// UpdateConfig 更新配置文件（通过 agent）
func (a *AppAgentified) UpdateConfig(ctx context.Context, serverID int64, name string, req *dto.UpdateConfigRequest) error {
	// 获取 agent 地址
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	// 调用 agent 更新配置
	resp, err := base.AgentClient.PutNginxConfig(ctx, agentAddr, name, req.Content, true, true)
	if err != nil {
		return a.err.New("调用 agent 更新配置失败", err)
	}

	if !resp.Success {
		return a.err.New(resp.Message, nil)
	}

	a.log.WithFields(map[string]interface{}{
		"server_id": serverID,
		"name":      name,
	}).Info("配置文件更新成功")
	return nil
}

// DeleteConfig 删除配置文件（通过 agent）
func (a *AppAgentified) DeleteConfig(ctx context.Context, serverID int64, name string) error {
	// 获取 agent 地址
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return err
	}

	// 调用 agent 删除配置
	resp, err := base.AgentClient.DeleteNginxConfig(ctx, agentAddr, name, true, true)
	if err != nil {
		return a.err.New("调用 agent 删除配置失败", err)
	}

	if !resp.Success {
		return a.err.New(resp.Message, nil)
	}

	a.log.WithFields(map[string]interface{}{
		"server_id": serverID,
		"name":      name,
	}).Info("配置文件删除成功")
	return nil
}

// GetConfig 获取配置文件（通过 agent）
func (a *AppAgentified) GetConfig(ctx context.Context, serverID int64, name string) (*dto.ConfigInfo, error) {
	// 获取 agent 地址
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 调用 agent 获取配置
	resp, err := base.AgentClient.GetNginxConfig(ctx, agentAddr, name)
	if err != nil {
		return nil, a.err.New("调用 agent 获取配置失败", err)
	}

	return &dto.ConfigInfo{
		Name:        resp.Name,
		Content:     resp.Content,
		Description: resp.Description,
		ModTime:     fmt.Sprintf("%d", resp.ModTime),
	}, nil
}

// ListConfigs 列出配置文件（通过 agent）
func (a *AppAgentified) ListConfigs(ctx context.Context, serverID int64, req *dto.QueryConfigRequest) ([]*dto.ConfigListItem, int64, error) {
	// 获取 agent 地址
	agentAddr, err := a.getAgentAddress(ctx, serverID)
	if err != nil {
		return nil, 0, err
	}

	// 调用 agent 列出配置
	resp, err := base.AgentClient.ListNginxConfigs(ctx, agentAddr, req.Keyword)
	if err != nil {
		return nil, 0, a.err.New("调用 agent 列出配置失败", err)
	}

	result := make([]*dto.ConfigListItem, len(resp.Configs))
	for i, cfg := range resp.Configs {
		result[i] = &dto.ConfigListItem{
			Name:        cfg.Name,
			Description: cfg.Description,
			ModTime:     fmt.Sprintf("%d", cfg.ModTime),
		}
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
		return []*dto.ConfigListItem{}, total, nil
	}
	if end > int(total) {
		end = int(total)
	}

	return result[start:end], total, nil
}

// CreateConfigByParams 按参数生成并创建配置文件
func (a *AppAgentified) CreateConfigByParams(ctx context.Context, serverID int64, req *dto.CreateConfigByParamsRequest) error {
	// 生成配置内容
	content, err := a.GenerateService.Generate(&req.Spec)
	if err != nil {
		return err
	}

	// 复用创建接口
	createReq := &dto.CreateConfigRequest{
		Name:    req.Name,
		Content: content,
	}
	return a.CreateConfig(ctx, serverID, createReq)
}

// UpdateConfigByParams 按参数生成并更新配置文件
func (a *AppAgentified) UpdateConfigByParams(ctx context.Context, serverID int64, name string, req *dto.UpdateConfigByParamsRequest) error {
	// 生成配置内容
	content, err := a.GenerateService.Generate(&req.Spec)
	if err != nil {
		return err
	}

	// 复用更新接口
	updateReq := &dto.UpdateConfigRequest{
		Content: content,
	}
	return a.UpdateConfig(ctx, serverID, name, updateReq)
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

