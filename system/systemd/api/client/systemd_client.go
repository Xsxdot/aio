package client

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/systemd/api/dto"
	internalapp "xiaozhizhang/system/systemd/internal/app"
	internaldto "xiaozhizhang/system/systemd/internal/model/dto"
)

// SystemdClient Systemd 管理组件对外客户端（进程内调用）
// 对外只暴露 api/dto，禁止泄漏 internal/model。
type SystemdClient struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
}

// NewSystemdClient 创建 Systemd 客户端实例
func NewSystemdClient(app *internalapp.App) *SystemdClient {
	return &SystemdClient{
		app: app,
		err: errorc.NewErrorBuilder("SystemdClient"),
	}
}

// ------------------- CRUD 操作 -------------------

// CreateService 创建服务（unit 文件）
func (c *SystemdClient) CreateService(ctx context.Context, req *dto.CreateServiceReq) error {
	if req == nil {
		return c.err.New("请求参数不能为空", nil).ValidWithCtx()
	}
	if req.Name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}

	internalReq := &internaldto.CreateServiceRequest{
		Name:    req.Name,
		Content: req.Content,
	}

	return c.app.CreateService(ctx, internalReq)
}

// UpdateService 更新服务（unit 文件）
func (c *SystemdClient) UpdateService(ctx context.Context, name string, req *dto.UpdateServiceReq) error {
	if name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}
	if req == nil {
		return c.err.New("请求参数不能为空", nil).ValidWithCtx()
	}

	internalReq := &internaldto.UpdateServiceRequest{
		Content: req.Content,
	}

	return c.app.UpdateService(ctx, name, internalReq)
}

// DeleteService 删除服务
func (c *SystemdClient) DeleteService(ctx context.Context, name string, force bool) error {
	if name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}
	return c.app.DeleteService(ctx, name, force)
}

// GetService 获取服务信息
func (c *SystemdClient) GetService(ctx context.Context, name string) (*dto.ServiceInfoDTO, error) {
	if name == "" {
		return nil, c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}

	info, err := c.app.GetService(ctx, name)
	if err != nil {
		return nil, err
	}

	return &dto.ServiceInfoDTO{
		Name:        info.Name,
		Content:     info.Content,
		Description: info.Description,
		ModTime:     info.ModTime,
	}, nil
}

// ListServices 列出服务
func (c *SystemdClient) ListServices(ctx context.Context, req *dto.QueryServiceReq) ([]*dto.ServiceListItemDTO, int64, error) {
	if req == nil {
		return nil, 0, c.err.New("请求参数不能为空", nil).ValidWithCtx()
	}

	internalReq := &internaldto.QueryServiceRequest{
		Keyword:       req.Keyword,
		IncludeStatus: req.IncludeStatus,
		PageNum:       req.PageNum,
		Size:          req.Size,
	}

	items, total, err := c.app.ListServices(ctx, internalReq)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*dto.ServiceListItemDTO, len(items))
	for i, item := range items {
		result[i] = &dto.ServiceListItemDTO{
			Name:          item.Name,
			Description:   item.Description,
			ModTime:       item.ModTime,
			ActiveState:   item.ActiveState,
			SubState:      item.SubState,
			UnitFileState: item.UnitFileState,
		}
	}

	return result, total, nil
}

// ------------------- 控制操作（预留扩展） -------------------

// ControlService 控制服务（启动/停止/重启/重载）
func (c *SystemdClient) ControlService(ctx context.Context, name, action string) error {
	if name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}
	return c.app.ControlService(ctx, name, action)
}

// SetServiceEnabled 设置服务启用/禁用状态
func (c *SystemdClient) SetServiceEnabled(ctx context.Context, name string, enable bool) error {
	if name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}
	return c.app.SetServiceEnabled(ctx, name, enable)
}

// GetServiceStatus 获取服务状态
func (c *SystemdClient) GetServiceStatus(ctx context.Context, name string) (*dto.ServiceStatusDTO, error) {
	if name == "" {
		return nil, c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}

	status, err := c.app.GetServiceStatus(ctx, name)
	if err != nil {
		return nil, err
	}

	return &dto.ServiceStatusDTO{
		Name:            status.Name,
		Description:     status.Description,
		LoadState:       status.LoadState,
		ActiveState:     status.ActiveState,
		SubState:        status.SubState,
		UnitFileState:   status.UnitFileState,
		MainPID:         status.MainPID,
		ExecMainStartAt: status.ExecMainStartAt,
		MemoryCurrent:   status.MemoryCurrent,
		Result:          status.Result,
	}, nil
}

// GetServiceLogs 获取服务日志
func (c *SystemdClient) GetServiceLogs(ctx context.Context, name string, req *dto.LogsReq) (*dto.ServiceLogsDTO, error) {
	if name == "" {
		return nil, c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}
	if req == nil {
		req = &dto.LogsReq{Lines: 200}
	}

	internalReq := &internaldto.LogsRequest{
		Lines: req.Lines,
		Since: req.Since,
		Until: req.Until,
	}

	logs, err := c.app.GetServiceLogs(ctx, name, internalReq)
	if err != nil {
		return nil, err
	}

	return &dto.ServiceLogsDTO{
		Name:  logs.Name,
		Lines: logs.Lines,
	}, nil
}

// ------------------- Unit 生成操作 -------------------

// GenerateService 生成服务 unit 内容（仅预览，不落盘）
func (c *SystemdClient) GenerateService(ctx context.Context, req *dto.GenerateServiceReq) (*dto.GenerateServiceResp, error) {
	if req == nil {
		return nil, c.err.New("请求参数不能为空", nil).ValidWithCtx()
	}

	internalReq := &internaldto.GenerateServiceRequest{
		Params: c.convertParamsToInternal(&req.Params),
	}

	resp, err := c.app.GenerateService(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return &dto.GenerateServiceResp{
		Content: resp.Content,
	}, nil
}

// CreateServiceFromParams 按参数创建服务
func (c *SystemdClient) CreateServiceFromParams(ctx context.Context, req *dto.CreateServiceFromParamsReq) error {
	if req == nil {
		return c.err.New("请求参数不能为空", nil).ValidWithCtx()
	}
	if req.Name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}

	internalReq := &internaldto.CreateServiceFromParamsRequest{
		Name:   req.Name,
		Params: c.convertParamsToInternal(&req.Params),
	}

	return c.app.CreateServiceFromParams(ctx, internalReq)
}

// UpdateServiceFromParams 按参数更新服务
func (c *SystemdClient) UpdateServiceFromParams(ctx context.Context, name string, req *dto.UpdateServiceFromParamsReq) error {
	if name == "" {
		return c.err.New("服务名称不能为空", nil).ValidWithCtx()
	}
	if req == nil {
		return c.err.New("请求参数不能为空", nil).ValidWithCtx()
	}

	internalReq := &internaldto.UpdateServiceFromParamsRequest{
		Params: c.convertParamsToInternal(&req.Params),
	}

	return c.app.UpdateServiceFromParams(ctx, name, internalReq)
}

// convertParamsToInternal 将 api dto 转换为 internal dto
func (c *SystemdClient) convertParamsToInternal(params *dto.ServiceUnitParamsDTO) internaldto.ServiceUnitParams {
	return internaldto.ServiceUnitParams{
		// [Unit] 段
		Description:   params.Description,
		Documentation: params.Documentation,
		After:         params.After,
		Wants:         params.Wants,
		Requires:      params.Requires,

		// [Service] 段
		Type:             params.Type,
		ExecStart:        params.ExecStart,
		ExecStartPre:     params.ExecStartPre,
		ExecStartPost:    params.ExecStartPost,
		ExecStop:         params.ExecStop,
		ExecReload:       params.ExecReload,
		WorkingDirectory: params.WorkingDirectory,
		User:             params.User,
		Group:            params.Group,
		Environment:      params.Environment,
		EnvironmentFile:  params.EnvironmentFile,
		Restart:          params.Restart,
		RestartSec:       params.RestartSec,
		TimeoutStartSec:  params.TimeoutStartSec,
		TimeoutStopSec:   params.TimeoutStopSec,
		LimitNOFILE:      params.LimitNOFILE,
		LimitNPROC:       params.LimitNPROC,

		// [Install] 段
		WantedBy:   params.WantedBy,
		RequiredBy: params.RequiredBy,
		Alias:      params.Alias,

		// 扩展行
		ExtraUnitLines:    params.ExtraUnitLines,
		ExtraServiceLines: params.ExtraServiceLines,
		ExtraInstallLines: params.ExtraInstallLines,
	}
}

