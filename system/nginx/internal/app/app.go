package app

import (
	"context"

	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/nginx/internal/model/dto"
	"xiaozhizhang/system/nginx/internal/service"
)

// App Nginx 管理应用组合根
type App struct {
	FileService     *service.NginxFileService
	CommandService  *service.NginxCommandService
	GenerateService *service.NginxConfigGenerateService
	log             *logger.Log
	err             *errorc.ErrorBuilder
}

// NewApp 创建 Nginx 管理应用实例
func NewApp() *App {
	log := base.Logger.WithEntryName("NginxApp")

	// 从配置获取参数
	cfg := base.Configures.Config.Nginx
	rootDir := cfg.RootDir
	if rootDir == "" {
		rootDir = "/etc/nginx/conf.d"
	}
	validateCommand := cfg.ValidateCommand
	if validateCommand == "" {
		validateCommand = "nginx -t"
	}
	reloadCommand := cfg.ReloadCommand
	if reloadCommand == "" {
		reloadCommand = "nginx -s reload"
	}
	timeout := cfg.CommandTimeout
	if timeout == 0 {
		timeout = 30 * 1e9 // 30 seconds in nanoseconds
	}
	fileMode := cfg.FileMode
	if fileMode == "" {
		fileMode = "0644"
	}

	// 创建 Service
	fileSvc := service.NewNginxFileService(rootDir, fileMode, log)
	cmdSvc := service.NewNginxCommandService(validateCommand, reloadCommand, timeout, log)
	genSvc := service.NewNginxConfigGenerateService(log)

	return &App{
		FileService:     fileSvc,
		CommandService:  cmdSvc,
		GenerateService: genSvc,
		log:             log,
		err:             errorc.NewErrorBuilder("NginxApp"),
	}
}

// CreateConfig 创建配置文件
// 1. 校验名称
// 2. 创建配置文件
// 3. 执行 nginx -t 校验
// 4. 执行 reload
// 失败时回滚（删除新创建的文件）
func (a *App) CreateConfig(ctx context.Context, req *dto.CreateConfigRequest) error {
	// 校验名称
	if err := a.FileService.ValidateName(req.Name); err != nil {
		return err
	}

	// 创建配置文件
	if err := a.FileService.Create(req.Name, req.Content); err != nil {
		return err
	}

	// 校验并重载
	_, _, err := a.CommandService.ValidateAndReload(ctx)
	if err != nil {
		// 回滚：删除刚创建的文件
		a.FileService.Delete(req.Name)
		return a.err.New("配置校验/重载失败，已回滚", err)
	}

	a.log.WithField("name", req.Name).Info("配置文件创建成功")
	return nil
}

// UpdateConfig 更新配置文件
// 1. 校验名称
// 2. 备份旧内容
// 3. 更新配置文件
// 4. 执行 nginx -t 校验
// 5. 执行 reload
// 失败时回滚（恢复旧内容）
func (a *App) UpdateConfig(ctx context.Context, name string, req *dto.UpdateConfigRequest) error {
	// 校验名称
	if err := a.FileService.ValidateName(name); err != nil {
		return err
	}

	// 备份旧内容
	oldInfo, err := a.FileService.Read(name)
	if err != nil {
		return err
	}

	// 更新配置文件
	if err := a.FileService.Update(name, req.Content); err != nil {
		return err
	}

	// 校验并重载
	_, _, err = a.CommandService.ValidateAndReload(ctx)
	if err != nil {
		// 回滚：恢复旧内容
		a.FileService.Update(name, oldInfo.Content)
		return a.err.New("配置校验/重载失败，已回滚", err)
	}

	a.log.WithField("name", name).Info("配置文件更新成功")
	return nil
}

// DeleteConfig 删除配置文件
// 1. 校验名称
// 2. 备份旧内容
// 3. 删除配置文件
// 4. 执行 nginx -t 校验
// 5. 执行 reload
// 失败时回滚（恢复旧内容）
func (a *App) DeleteConfig(ctx context.Context, name string) error {
	// 校验名称
	if err := a.FileService.ValidateName(name); err != nil {
		return err
	}

	// 检查文件是否存在
	exists, err := a.FileService.Exists(name)
	if err != nil {
		return err
	}
	if !exists {
		return a.err.New("配置文件不存在", nil).ValidWithCtx()
	}

	// 备份旧内容
	oldInfo, err := a.FileService.Read(name)
	if err != nil {
		return err
	}

	// 删除配置文件
	if err := a.FileService.Delete(name); err != nil {
		return err
	}

	// 校验并重载
	_, _, err = a.CommandService.ValidateAndReload(ctx)
	if err != nil {
		// 回滚：恢复旧内容
		a.FileService.Create(name, oldInfo.Content)
		return a.err.New("配置校验/重载失败，已回滚", err)
	}

	a.log.WithField("name", name).Info("配置文件删除成功")
	return nil
}

// GetConfig 获取配置文件信息
func (a *App) GetConfig(ctx context.Context, name string) (*dto.ConfigInfo, error) {
	// 校验名称
	if err := a.FileService.ValidateName(name); err != nil {
		return nil, err
	}

	info, err := a.FileService.Read(name)
	if err != nil {
		return nil, err
	}

	return &dto.ConfigInfo{
		Name:        info.Name,
		Content:     info.Content,
		Description: info.Description,
		ModTime:     info.ModTime.Format("2006-01-02 15:04:05"),
	}, nil
}

// ListConfigs 列出配置文件
func (a *App) ListConfigs(ctx context.Context, req *dto.QueryConfigRequest) ([]*dto.ConfigListItem, int64, error) {
	// 获取所有配置文件
	infos, err := a.FileService.List(req.Keyword)
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
		return []*dto.ConfigListItem{}, total, nil
	}
	if end > int(total) {
		end = int(total)
	}

	pageInfos := infos[start:end]
	result := make([]*dto.ConfigListItem, len(pageInfos))

	for i, info := range pageInfos {
		result[i] = &dto.ConfigListItem{
			Name:        info.Name,
			Description: info.Description,
			ModTime:     info.ModTime.Format("2006-01-02 15:04:05"),
		}
	}

	return result, total, nil
}

// CreateConfigByParams 按参数生成并创建配置文件
// 1. 使用 GenerateService 生成配置内容
// 2. 复用 CreateConfig 流程（写入 + 校验 + reload + 失败回滚）
func (a *App) CreateConfigByParams(ctx context.Context, req *dto.CreateConfigByParamsRequest) error {
	// 生成配置内容
	content, err := a.GenerateService.Generate(&req.Spec)
	if err != nil {
		return err
	}

	// 复用现有创建流程
	createReq := &dto.CreateConfigRequest{
		Name:    req.Name,
		Content: content,
	}
	return a.CreateConfig(ctx, createReq)
}

// UpdateConfigByParams 按参数生成并更新配置文件
// 1. 使用 GenerateService 生成配置内容
// 2. 复用 UpdateConfig 流程（备份 + 写入 + 校验 + reload + 失败回滚）
func (a *App) UpdateConfigByParams(ctx context.Context, name string, req *dto.UpdateConfigByParamsRequest) error {
	// 生成配置内容
	content, err := a.GenerateService.Generate(&req.Spec)
	if err != nil {
		return err
	}

	// 复用现有更新流程
	updateReq := &dto.UpdateConfigRequest{
		Content: content,
	}
	return a.UpdateConfig(ctx, name, updateReq)
}
