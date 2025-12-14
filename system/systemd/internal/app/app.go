package app

import (
	"context"
	"time"

	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/systemd/internal/model/dto"
	"xiaozhizhang/system/systemd/internal/service"
)

// App Systemd 管理应用组合根
type App struct {
	UnitFileService      *service.UnitFileService
	SystemctlService     *service.SystemctlService
	JournalService       *service.JournalService
	UnitGeneratorService *service.UnitGeneratorService
	log                  *logger.Log
	err                  *errorc.ErrorBuilder
}

// NewApp 创建 Systemd 管理应用实例
func NewApp() *App {
	log := base.Logger.WithEntryName("SystemdApp")

	// 从配置获取参数
	cfg := base.Configures.Config.Systemd
	rootDir := cfg.RootDir
	if rootDir == "" {
		rootDir = "/etc/systemd/system"
	}
	timeout := cfg.CommandTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 创建 Service
	unitFileSvc := service.NewUnitFileService(rootDir, log)
	systemctlSvc := service.NewSystemctlService(timeout, log)
	journalSvc := service.NewJournalService(timeout, log)
	unitGeneratorSvc := service.NewUnitGeneratorService(log)

	return &App{
		UnitFileService:      unitFileSvc,
		SystemctlService:     systemctlSvc,
		JournalService:       journalSvc,
		UnitGeneratorService: unitGeneratorSvc,
		log:                  log,
		err:                  errorc.NewErrorBuilder("SystemdApp"),
	}
}

// CreateService 创建服务（unit 文件）
// 1. 校验名称
// 2. 创建 unit 文件
// 3. 执行 daemon-reload
func (a *App) CreateService(ctx context.Context, req *dto.CreateServiceRequest) error {
	// 校验名称
	if err := a.UnitFileService.ValidateName(req.Name); err != nil {
		return err
	}

	// 创建 unit 文件
	if err := a.UnitFileService.Create(req.Name, req.Content); err != nil {
		return err
	}

	// daemon-reload
	if err := a.SystemctlService.DaemonReload(ctx); err != nil {
		// 回滚：删除刚创建的文件
		a.UnitFileService.Delete(req.Name)
		return a.err.New("daemon-reload 失败，已回滚", err)
	}

	a.log.WithField("name", req.Name).Info("服务创建成功")
	return nil
}

// UpdateService 更新服务（unit 文件）
// 1. 校验名称
// 2. 更新 unit 文件
// 3. 执行 daemon-reload
func (a *App) UpdateService(ctx context.Context, name string, req *dto.UpdateServiceRequest) error {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return err
	}

	// 备份旧内容
	oldInfo, err := a.UnitFileService.Read(name)
	if err != nil {
		return err
	}

	// 更新 unit 文件
	if err := a.UnitFileService.Update(name, req.Content); err != nil {
		return err
	}

	// daemon-reload
	if err := a.SystemctlService.DaemonReload(ctx); err != nil {
		// 回滚：恢复旧内容
		a.UnitFileService.Update(name, oldInfo.Content)
		return a.err.New("daemon-reload 失败，已回滚", err)
	}

	a.log.WithField("name", name).Info("服务更新成功")
	return nil
}

// DeleteService 删除服务
// 1. 校验名称
// 2. 停止并禁用服务（可选 force 模式忽略错误）
// 3. 删除 unit 文件
// 4. 执行 daemon-reload
func (a *App) DeleteService(ctx context.Context, name string, force bool) error {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return err
	}

	// 尝试停止并禁用
	err := a.SystemctlService.StopAndDisable(ctx, name)
	if err != nil && !force {
		return a.err.New("停止/禁用服务失败", err)
	}

	// 删除 unit 文件
	if err := a.UnitFileService.Delete(name); err != nil {
		return err
	}

	// daemon-reload
	if err := a.SystemctlService.DaemonReload(ctx); err != nil {
		a.log.WithErr(err).Warn("daemon-reload 失败（unit 文件已删除）")
	}

	a.log.WithField("name", name).Info("服务删除成功")
	return nil
}

// GetService 获取服务信息
func (a *App) GetService(ctx context.Context, name string) (*dto.ServiceInfo, error) {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return nil, err
	}

	info, err := a.UnitFileService.Read(name)
	if err != nil {
		return nil, err
	}

	return &dto.ServiceInfo{
		Name:        info.Name,
		Content:     info.Content,
		Description: info.Description,
		ModTime:     info.ModTime.Format("2006-01-02 15:04:05"),
	}, nil
}

// ListServices 列出服务
func (a *App) ListServices(ctx context.Context, req *dto.QueryServiceRequest) ([]*dto.ServiceListItem, int64, error) {
	// 获取所有 unit 文件
	infos, err := a.UnitFileService.List(req.Keyword)
	if err != nil {
		return nil, 0, err
	}

	total := int64(len(infos))

	// 分页
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

	pageInfos := infos[start:end]
	result := make([]*dto.ServiceListItem, len(pageInfos))

	for i, info := range pageInfos {
		item := &dto.ServiceListItem{
			Name:        info.Name,
			Description: info.Description,
			ModTime:     info.ModTime.Format("2006-01-02 15:04:05"),
		}

		// 如果需要包含状态信息
		if req.IncludeStatus {
			activeState, _ := a.SystemctlService.GetActiveState(ctx, info.Name)
			subState, _ := a.SystemctlService.GetSubState(ctx, info.Name)
			unitFileState, _ := a.SystemctlService.GetUnitFileState(ctx, info.Name)

			item.ActiveState = activeState
			item.SubState = subState
			item.UnitFileState = unitFileState
		}

		result[i] = item
	}

	return result, total, nil
}

// ControlService 控制服务（启动/停止/重启/重载）
func (a *App) ControlService(ctx context.Context, name, action string) error {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return err
	}

	// 检查服务是否存在
	exists, err := a.UnitFileService.Exists(name)
	if err != nil {
		return err
	}
	if !exists {
		return a.err.New("服务不存在", nil).ValidWithCtx()
	}

	switch action {
	case "start":
		return a.SystemctlService.Start(ctx, name)
	case "stop":
		return a.SystemctlService.Stop(ctx, name)
	case "restart":
		return a.SystemctlService.Restart(ctx, name)
	case "reload":
		return a.SystemctlService.Reload(ctx, name)
	default:
		return a.err.New("不支持的操作: "+action, nil).ValidWithCtx()
	}
}

// SetServiceEnabled 设置服务启用/禁用状态
func (a *App) SetServiceEnabled(ctx context.Context, name string, enable bool) error {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return err
	}

	// 检查服务是否存在
	exists, err := a.UnitFileService.Exists(name)
	if err != nil {
		return err
	}
	if !exists {
		return a.err.New("服务不存在", nil).ValidWithCtx()
	}

	if enable {
		return a.SystemctlService.Enable(ctx, name)
	}
	return a.SystemctlService.Disable(ctx, name)
}

// GetServiceStatus 获取服务状态
func (a *App) GetServiceStatus(ctx context.Context, name string) (*dto.ServiceStatus, error) {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return nil, err
	}

	// 检查服务是否存在
	exists, err := a.UnitFileService.Exists(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, a.err.New("服务不存在", nil).ValidWithCtx()
	}

	return a.SystemctlService.Show(ctx, name)
}

// GetServiceLogs 获取服务日志
func (a *App) GetServiceLogs(ctx context.Context, name string, req *dto.LogsRequest) (*dto.ServiceLogs, error) {
	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return nil, err
	}

	lines := req.Lines
	if lines <= 0 {
		lines = 200
	}

	logLines, err := a.JournalService.GetLogs(ctx, name, lines, req.Since, req.Until)
	if err != nil {
		return nil, err
	}

	return &dto.ServiceLogs{
		Name:  name,
		Lines: logLines,
	}, nil
}

// ------------------- Unit 生成相关方法 -------------------

// GenerateService 生成 service unit 内容（仅预览，不落盘）
func (a *App) GenerateService(_ context.Context, req *dto.GenerateServiceRequest) (*dto.GenerateServiceResponse, error) {
	content, err := a.UnitGeneratorService.Generate(&req.Params)
	if err != nil {
		return nil, err
	}

	return &dto.GenerateServiceResponse{
		Content: content,
	}, nil
}

// CreateServiceFromParams 按参数创建 service
// 1. 根据参数生成 unit 内容
// 2. 校验名称
// 3. 创建 unit 文件
// 4. 执行 daemon-reload
func (a *App) CreateServiceFromParams(ctx context.Context, req *dto.CreateServiceFromParamsRequest) error {
	// 生成 unit 内容
	content, err := a.UnitGeneratorService.Generate(&req.Params)
	if err != nil {
		return err
	}

	// 校验名称
	if err := a.UnitFileService.ValidateName(req.Name); err != nil {
		return err
	}

	// 创建 unit 文件
	if err := a.UnitFileService.Create(req.Name, content); err != nil {
		return err
	}

	// daemon-reload
	if err := a.SystemctlService.DaemonReload(ctx); err != nil {
		// 回滚：删除刚创建的文件
		a.UnitFileService.Delete(req.Name)
		return a.err.New("daemon-reload 失败，已回滚", err)
	}

	a.log.WithField("name", req.Name).Info("服务创建成功（按参数生成）")
	return nil
}

// UpdateServiceFromParams 按参数更新 service
// 1. 根据参数生成 unit 内容
// 2. 校验名称
// 3. 备份旧内容
// 4. 更新 unit 文件
// 5. 执行 daemon-reload
func (a *App) UpdateServiceFromParams(ctx context.Context, name string, req *dto.UpdateServiceFromParamsRequest) error {
	// 生成 unit 内容
	content, err := a.UnitGeneratorService.Generate(&req.Params)
	if err != nil {
		return err
	}

	// 校验名称
	if err := a.UnitFileService.ValidateName(name); err != nil {
		return err
	}

	// 备份旧内容
	oldInfo, err := a.UnitFileService.Read(name)
	if err != nil {
		return err
	}

	// 更新 unit 文件
	if err := a.UnitFileService.Update(name, content); err != nil {
		return err
	}

	// daemon-reload
	if err := a.SystemctlService.DaemonReload(ctx); err != nil {
		// 回滚：恢复旧内容
		a.UnitFileService.Update(name, oldInfo.Content)
		return a.err.New("daemon-reload 失败，已回滚", err)
	}

	a.log.WithField("name", name).Info("服务更新成功（按参数生成）")
	return nil
}
