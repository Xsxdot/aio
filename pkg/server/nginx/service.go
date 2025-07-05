package nginx

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/xsxdot/aio/pkg/server"
)

// Service nginx服务层实现
type Service struct {
	storage  Storage
	executor server.Executor
}

// NewService 创建nginx服务
func NewService(storage Storage, executor server.Executor) *Service {
	return &Service{
		storage:  storage,
		executor: executor,
	}
}

// AddNginxServer 添加nginx服务器
func (s *Service) AddNginxServer(ctx context.Context, req *server.NginxServerCreateRequest) (*server.NginxServer, error) {
	// 检测nginx安装和版本
	version, err := s.detectNginxVersion(ctx, req.ServerID, req.NginxPath)
	if err != nil {
		return nil, fmt.Errorf("检测nginx版本失败: %w", err)
	}

	// 验证配置路径
	if err := s.validateConfigPath(ctx, req.ServerID, req.ConfigPath); err != nil {
		return nil, fmt.Errorf("验证配置路径失败: %w", err)
	}

	nginxServer := &server.NginxServer{
		ServerID:       req.ServerID,
		NginxPath:      req.NginxPath,
		ConfigPath:     req.ConfigPath,
		SitesEnabled:   req.SitesEnabled,
		SitesAvailable: req.SitesAvailable,
		LogPath:        req.LogPath,
		Version:        version,
		Status:         "unknown",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 设置默认值
	if nginxServer.SitesEnabled == "" {
		nginxServer.SitesEnabled = filepath.Join(req.ConfigPath, "sites-enabled")
	}
	if nginxServer.SitesAvailable == "" {
		nginxServer.SitesAvailable = filepath.Join(req.ConfigPath, "sites-available")
	}
	if nginxServer.LogPath == "" {
		nginxServer.LogPath = "/var/log/nginx"
	}

	return s.storage.CreateNginxServer(ctx, nginxServer)
}

// GetNginxServer 获取nginx服务器
func (s *Service) GetNginxServer(ctx context.Context, serverID string) (*server.NginxServer, error) {
	return s.storage.GetNginxServer(ctx, serverID)
}

// UpdateNginxServer 更新nginx服务器
func (s *Service) UpdateNginxServer(ctx context.Context, serverID string, req *server.NginxServerUpdateRequest) (*server.NginxServer, error) {
	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.NginxPath != nil {
		nginxServer.NginxPath = *req.NginxPath
	}
	if req.ConfigPath != nil {
		nginxServer.ConfigPath = *req.ConfigPath
	}
	if req.SitesEnabled != nil {
		nginxServer.SitesEnabled = *req.SitesEnabled
	}
	if req.SitesAvailable != nil {
		nginxServer.SitesAvailable = *req.SitesAvailable
	}
	if req.LogPath != nil {
		nginxServer.LogPath = *req.LogPath
	}

	nginxServer.UpdatedAt = time.Now()

	return s.storage.UpdateNginxServer(ctx, nginxServer)
}

// DeleteNginxServer 删除nginx服务器
func (s *Service) DeleteNginxServer(ctx context.Context, serverID string) error {
	return s.storage.DeleteNginxServer(ctx, serverID)
}

// ListNginxServers 列出nginx服务器
func (s *Service) ListNginxServers(ctx context.Context, req *server.NginxServerListRequest) ([]*server.NginxServer, int, error) {
	return s.storage.ListNginxServers(ctx, req)
}

// ListConfigs 列出配置文件
func (s *Service) ListConfigs(ctx context.Context, serverID string, req *server.NginxConfigListRequest) ([]*server.NginxConfig, int, error) {
	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, 0, err
	}

	// 确定查询路径
	queryPath := nginxServer.ConfigPath
	if req.Path != "" {
		queryPath = filepath.Join(nginxServer.ConfigPath, req.Path)
	}

	// 执行ls命令获取文件列表
	cmd := &server.Command{
		ID:      "list_configs",
		Command: fmt.Sprintf(`find "%s" -maxdepth 1 -type f -name "*.conf" -o -name "nginx.conf" | head -n %d | tail -n +%d`, queryPath, req.Limit, req.Offset+1),
		Timeout: 30 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, 0, fmt.Errorf("执行命令失败: %w", err)
	}

	if result.CommandResult.ExitCode != 0 {
		return nil, 0, fmt.Errorf("命令执行失败: %s", result.CommandResult.Stderr)
	}

	// 解析结果
	configs := make([]*server.NginxConfig, 0)
	lines := strings.Split(strings.TrimSpace(result.CommandResult.Stdout), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		config := &server.NginxConfig{
			ServerID:    serverID,
			Path:        line,
			Name:        filepath.Base(line),
			Type:        s.detectConfigType(line),
			IsDirectory: false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		configs = append(configs, config)
	}

	return configs, len(configs), nil
}

// GetConfig 获取配置文件内容
func (s *Service) GetConfig(ctx context.Context, serverID, configPath string) (*server.NginxConfig, error) {
	// 读取文件内容
	cmd := &server.Command{
		ID:      "get_config",
		Command: fmt.Sprintf(`cat "%s"`, configPath),
		Timeout: 30 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}

	if result.CommandResult.ExitCode != 0 {
		return nil, fmt.Errorf("读取配置文件失败: %s", result.CommandResult.Stderr)
	}

	config := &server.NginxConfig{
		ServerID:    serverID,
		Path:        configPath,
		Name:        filepath.Base(configPath),
		Type:        s.detectConfigType(configPath),
		Content:     result.CommandResult.Stdout,
		Size:        int64(len(result.CommandResult.Stdout)),
		IsDirectory: false,
		ModTime:     time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return config, nil
}

// CreateConfig 创建配置文件
func (s *Service) CreateConfig(ctx context.Context, serverID string, req *server.NginxConfigCreateRequest) (*server.NginxConfig, error) {
	// 构建完整路径
	fullPath := filepath.Join(req.Path, req.Name)

	// 创建文件
	cmd := &server.Command{
		ID:      "create_config",
		Command: fmt.Sprintf("cat > \"%s\" << 'EOF'\n%s\nEOF", fullPath, req.Content),
		Timeout: 30 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}

	if result.CommandResult.ExitCode != 0 {
		return nil, fmt.Errorf("创建配置文件失败: %s", result.CommandResult.Stderr)
	}

	config := &server.NginxConfig{
		ServerID:    serverID,
		Path:        fullPath,
		Name:        req.Name,
		Type:        req.Type,
		Content:     req.Content,
		Size:        int64(len(req.Content)),
		IsDirectory: false,
		ModTime:     time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 先测试配置文件语法
	testResult, err := s.TestConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("测试配置失败: %w", err)
	}

	if !testResult.Success {
		return nil, fmt.Errorf("配置文件语法错误: %s", testResult.Error)
	}

	// 配置测试通过，执行reload
	_, err = s.ReloadConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("重载配置失败: %w", err)
	}

	return config, nil
}

// UpdateConfig 更新配置文件
func (s *Service) UpdateConfig(ctx context.Context, serverID, configPath string, req *server.NginxConfigUpdateRequest) (*server.NginxConfig, error) {
	if req.Content == nil {
		return nil, fmt.Errorf("配置内容不能为空")
	}

	// 更新文件内容
	cmd := &server.Command{
		ID:      "update_config",
		Command: fmt.Sprintf("cat > \"%s\" << 'EOF'\n%s\nEOF", configPath, *req.Content),
		Timeout: 30 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}

	if result.CommandResult.ExitCode != 0 {
		return nil, fmt.Errorf("更新配置文件失败: %s", result.CommandResult.Stderr)
	}

	// 获取更新后的配置
	config, err := s.GetConfig(ctx, serverID, configPath)
	if err != nil {
		return nil, err
	}

	// 先测试配置文件语法
	testResult, err := s.TestConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("测试配置失败: %w", err)
	}

	if !testResult.Success {
		return nil, fmt.Errorf("配置文件语法错误: %s", testResult.Error)
	}

	// 配置测试通过，执行reload
	_, err = s.ReloadConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("重载配置失败: %w", err)
	}

	return config, nil
}

// DeleteConfig 删除配置文件
func (s *Service) DeleteConfig(ctx context.Context, serverID, configPath string) (*server.NginxOperationResult, error) {
	cmd := &server.Command{
		ID:      "delete_config",
		Command: fmt.Sprintf(`rm -f "%s"`, configPath),
		Timeout: 10 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}

	operationResult := &server.NginxOperationResult{
		Success:  result.CommandResult.ExitCode == 0,
		Message:  "配置文件删除成功",
		Output:   result.CommandResult.Stdout,
		ExitCode: result.CommandResult.ExitCode,
		Error:    result.CommandResult.Stderr,
	}

	// 如果删除成功，执行reload
	if operationResult.Success {
		// 先测试配置文件语法
		testResult, err := s.TestConfig(ctx, serverID)
		if err != nil {
			operationResult.Message += "，但测试配置失败: " + err.Error()
		} else if !testResult.Success {
			operationResult.Message += "，但配置文件语法错误: " + testResult.Error
		} else {
			// 配置测试通过，执行reload
			_, err = s.ReloadConfig(ctx, serverID)
			if err != nil {
				operationResult.Message += "，但重载配置失败: " + err.Error()
			} else {
				operationResult.Message += "，配置已重载"
			}
		}
	}

	return operationResult, nil
}

// TestConfig 测试nginx配置
func (s *Service) TestConfig(ctx context.Context, serverID string) (*server.NginxOperationResult, error) {
	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := &server.Command{
		ID:      "test_config",
		Command: fmt.Sprintf(`%s -t`, nginxServer.NginxPath),
		Timeout: 10 * time.Second,
	}

	return s.executeNginxCommand(ctx, serverID, cmd)
}

// ReloadConfig 重载nginx配置
func (s *Service) ReloadConfig(ctx context.Context, serverID string) (*server.NginxOperationResult, error) {
	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	cmd := &server.Command{
		ID:      "reload_config",
		Command: fmt.Sprintf(`%s -s reload`, nginxServer.NginxPath),
		Timeout: 10 * time.Second,
	}

	return s.executeNginxCommand(ctx, serverID, cmd)
}

// RestartNginx 重启nginx
func (s *Service) RestartNginx(ctx context.Context, serverID string) (*server.NginxOperationResult, error) {
	cmd := &server.Command{
		ID:      "restart_nginx",
		Command: "systemctl restart nginx",
		Timeout: 30 * time.Second,
	}

	return s.executeNginxCommand(ctx, serverID, cmd)
}

// GetNginxStatus 获取nginx状态
func (s *Service) GetNginxStatus(ctx context.Context, serverID string) (*server.NginxStatusResult, error) {
	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 检查nginx进程状态
	cmd := &server.Command{
		ID:      "nginx_status",
		Command: `systemctl is-active nginx && pgrep -f nginx | head -1`,
		Timeout: 10 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return &server.NginxStatusResult{
			Status: server.NginxStatusError,
			Error:  err.Error(),
		}, nil
	}

	status := server.NginxStatusStopped
	if result.CommandResult.ExitCode == 0 {
		status = server.NginxStatusRunning
	}

	statusResult := &server.NginxStatusResult{
		Status:  status,
		Version: nginxServer.Version,
	}

	// 解析进程信息
	lines := strings.Split(strings.TrimSpace(result.CommandResult.Stdout), "\n")
	if len(lines) >= 2 && lines[0] == "active" {
		statusResult.PID = 0 // 简化处理
	}

	return statusResult, nil
}

// ListSites 列出站点
func (s *Service) ListSites(ctx context.Context, serverID string, req *server.NginxSiteListRequest) ([]*server.NginxSite, int, error) {
	return s.storage.ListSites(ctx, serverID, req)
}

// GetSite 获取站点
func (s *Service) GetSite(ctx context.Context, serverID, siteName string) (*server.NginxSite, error) {
	return s.storage.GetSite(ctx, serverID, siteName)
}

// CreateSite 创建站点
func (s *Service) CreateSite(ctx context.Context, serverID string, req *server.NginxSiteCreateRequest) (*server.NginxSite, error) {
	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	site := &server.NginxSite{
		ServerID:    serverID,
		Name:        req.Name,
		Type:        req.Type,
		ServerName:  req.ServerName,
		Listen:      req.Listen,
		Root:        req.Root,
		Index:       req.Index,
		AccessLog:   req.AccessLog,
		ErrorLog:    req.ErrorLog,
		SSL:         req.SSL,
		SSLCert:     req.SSLCert,
		SSLKey:      req.SSLKey,
		Enabled:     req.Enabled,
		ConfigMode:  req.ConfigMode,
		ExtraConfig: req.ExtraConfig,
		Upstream:    req.Upstream,
		Locations:   req.Locations,
		GlobalProxy: req.GlobalProxy,
		ConfigPath:  filepath.Join(nginxServer.SitesAvailable, req.Name+".conf"),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置默认值
	if site.Type == "" {
		site.Type = server.NginxSiteTypeStatic
	}
	if len(site.Listen) == 0 {
		if site.SSL {
			site.Listen = []string{"443 ssl"}
		} else {
			site.Listen = []string{"80"}
		}
	}
	if site.Type == server.NginxSiteTypeStatic && len(site.Index) == 0 {
		site.Index = []string{"index.html", "index.htm", "index.php"}
	}

	// 验证配置
	if err := ValidateSiteConfig(site); err != nil {
		return nil, fmt.Errorf("站点配置验证失败: %w", err)
	}

	// 生成nginx配置
	configContent := s.generateSiteConfig(site)

	// 创建配置文件
	createCmd := &server.Command{
		ID:      "create_site_config",
		Command: fmt.Sprintf("cat > \"%s\" << 'EOF'\n%s\nEOF", site.ConfigPath, configContent),
		Timeout: 30 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  createCmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("创建站点配置文件失败: %w", err)
	}

	if result.CommandResult.ExitCode != 0 {
		return nil, fmt.Errorf("创建站点配置文件失败: %s", result.CommandResult.Stderr)
	}

	// 如果启用站点，创建软链接
	if site.Enabled {
		enableCmd := &server.Command{
			ID:      "enable_site",
			Command: fmt.Sprintf(`ln -sf "%s" "%s"`, site.ConfigPath, filepath.Join(nginxServer.SitesEnabled, req.Name+".conf")),
			Timeout: 10 * time.Second,
		}

		enableReq := &server.ExecuteRequest{
			ServerID: serverID,
			Type:     server.CommandTypeSingle,
			Command:  enableCmd,
		}

		s.executor.Execute(ctx, enableReq)
	}

	// 创建站点记录
	createdSite, err := s.storage.CreateSite(ctx, site)
	if err != nil {
		return nil, err
	}

	// 先测试配置文件语法
	testResult, err := s.TestConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("测试配置失败: %w", err)
	}

	if !testResult.Success {
		return nil, fmt.Errorf("配置文件语法错误: %s", testResult.Error)
	}

	// 配置测试通过，执行reload
	_, err = s.ReloadConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("重载配置失败: %w", err)
	}

	return createdSite, nil
}

// UpdateSite 更新站点
func (s *Service) UpdateSite(ctx context.Context, serverID, siteName string, req *server.NginxSiteUpdateRequest) (*server.NginxSite, error) {
	site, err := s.storage.GetSite(ctx, serverID, siteName)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.Type != nil {
		site.Type = *req.Type
	}
	if req.ServerName != nil {
		site.ServerName = *req.ServerName
	}
	if req.Listen != nil {
		site.Listen = *req.Listen
	}
	if req.Root != nil {
		site.Root = *req.Root
	}
	if req.Index != nil {
		site.Index = *req.Index
	}
	if req.AccessLog != nil {
		site.AccessLog = *req.AccessLog
	}
	if req.ErrorLog != nil {
		site.ErrorLog = *req.ErrorLog
	}
	if req.SSL != nil {
		site.SSL = *req.SSL
	}
	if req.SSLCert != nil {
		site.SSLCert = *req.SSLCert
	}
	if req.SSLKey != nil {
		site.SSLKey = *req.SSLKey
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}
	if req.ConfigMode != nil {
		site.ConfigMode = *req.ConfigMode
	}
	if req.ExtraConfig != nil {
		site.ExtraConfig = *req.ExtraConfig
	}
	if req.Upstream != nil {
		site.Upstream = req.Upstream
	}
	if req.Locations != nil {
		site.Locations = *req.Locations
	}
	if req.GlobalProxy != nil {
		site.GlobalProxy = req.GlobalProxy
	}

	site.UpdatedAt = time.Now()

	// 重新生成配置文件
	configContent := s.generateSiteConfig(site)

	updateCmd := &server.Command{
		ID:      "update_site_config",
		Command: fmt.Sprintf("cat > \"%s\" << 'EOF'\n%s\nEOF", site.ConfigPath, configContent),
		Timeout: 30 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  updateCmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("更新站点配置文件失败: %w", err)
	}

	if result.CommandResult.ExitCode != 0 {
		return nil, fmt.Errorf("更新站点配置文件失败: %s", result.CommandResult.Stderr)
	}

	// 更新存储中的站点信息
	updatedSite, err := s.storage.UpdateSite(ctx, site)
	if err != nil {
		return nil, err
	}

	// 先测试配置文件语法
	testResult, err := s.TestConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("测试配置失败: %w", err)
	}

	if !testResult.Success {
		return nil, fmt.Errorf("配置文件语法错误: %s", testResult.Error)
	}

	// 配置测试通过，执行reload
	_, err = s.ReloadConfig(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("重载配置失败: %w", err)
	}

	return updatedSite, nil
}

// DeleteSite 删除站点
func (s *Service) DeleteSite(ctx context.Context, serverID, siteName string) (*server.NginxOperationResult, error) {
	site, err := s.storage.GetSite(ctx, serverID, siteName)
	if err != nil {
		return nil, err
	}

	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 删除配置文件和软链接
	deleteCmd := &server.Command{
		ID:      "delete_site",
		Command: fmt.Sprintf(`rm -f "%s" "%s"`, site.ConfigPath, filepath.Join(nginxServer.SitesEnabled, siteName+".conf")),
		Timeout: 10 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  deleteCmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("删除站点配置文件失败: %w", err)
	}

	// 从存储中删除
	if err := s.storage.DeleteSite(ctx, serverID, siteName); err != nil {
		return nil, err
	}

	operationResult := &server.NginxOperationResult{
		Success:  result.CommandResult.ExitCode == 0,
		Message:  "站点删除成功",
		Output:   result.CommandResult.Stdout,
		ExitCode: result.CommandResult.ExitCode,
		Error:    result.CommandResult.Stderr,
	}

	// 如果删除成功，执行reload
	if operationResult.Success {
		// 先测试配置文件语法
		testResult, err := s.TestConfig(ctx, serverID)
		if err != nil {
			operationResult.Message += "，但测试配置失败: " + err.Error()
		} else if !testResult.Success {
			operationResult.Message += "，但配置文件语法错误: " + testResult.Error
		} else {
			// 配置测试通过，执行reload
			_, err = s.ReloadConfig(ctx, serverID)
			if err != nil {
				operationResult.Message += "，但重载配置失败: " + err.Error()
			} else {
				operationResult.Message += "，配置已重载"
			}
		}
	}

	return operationResult, nil
}

// EnableSite 启用站点
func (s *Service) EnableSite(ctx context.Context, serverID, siteName string) (*server.NginxOperationResult, error) {
	site, err := s.storage.GetSite(ctx, serverID, siteName)
	if err != nil {
		return nil, err
	}

	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 创建软链接
	enableCmd := &server.Command{
		ID:      "enable_site",
		Command: fmt.Sprintf(`ln -sf "%s" "%s"`, site.ConfigPath, filepath.Join(nginxServer.SitesEnabled, siteName+".conf")),
		Timeout: 10 * time.Second,
	}

	result, err := s.executeNginxCommand(ctx, serverID, enableCmd)
	if err != nil {
		return nil, err
	}

	// 更新数据库状态
	site.Enabled = true
	site.UpdatedAt = time.Now()
	s.storage.UpdateSite(ctx, site)

	// 如果启用成功，执行reload
	if result.Success {
		// 先测试配置文件语法
		testResult, err := s.TestConfig(ctx, serverID)
		if err != nil {
			result.Message += "，但测试配置失败: " + err.Error()
		} else if !testResult.Success {
			result.Message += "，但配置文件语法错误: " + testResult.Error
		} else {
			// 配置测试通过，执行reload
			_, err = s.ReloadConfig(ctx, serverID)
			if err != nil {
				result.Message += "，但重载配置失败: " + err.Error()
			} else {
				result.Message = "站点启用成功，配置已重载"
			}
		}
	}

	return result, nil
}

// DisableSite 禁用站点
func (s *Service) DisableSite(ctx context.Context, serverID, siteName string) (*server.NginxOperationResult, error) {
	site, err := s.storage.GetSite(ctx, serverID, siteName)
	if err != nil {
		return nil, err
	}

	nginxServer, err := s.storage.GetNginxServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 删除软链接
	disableCmd := &server.Command{
		ID:      "disable_site",
		Command: fmt.Sprintf(`rm -f "%s"`, filepath.Join(nginxServer.SitesEnabled, siteName+".conf")),
		Timeout: 10 * time.Second,
	}

	result, err := s.executeNginxCommand(ctx, serverID, disableCmd)
	if err != nil {
		return nil, err
	}

	// 更新数据库状态
	site.Enabled = false
	site.UpdatedAt = time.Now()
	s.storage.UpdateSite(ctx, site)

	// 如果禁用成功，执行reload
	if result.Success {
		// 先测试配置文件语法
		testResult, err := s.TestConfig(ctx, serverID)
		if err != nil {
			result.Message += "，但测试配置失败: " + err.Error()
		} else if !testResult.Success {
			result.Message += "，但配置文件语法错误: " + testResult.Error
		} else {
			// 配置测试通过，执行reload
			_, err = s.ReloadConfig(ctx, serverID)
			if err != nil {
				result.Message += "，但重载配置失败: " + err.Error()
			} else {
				result.Message = "站点禁用成功，配置已重载"
			}
		}
	}

	return result, nil
}

// detectNginxVersion 检测nginx版本
func (s *Service) detectNginxVersion(ctx context.Context, serverID, nginxPath string) (string, error) {
	cmd := &server.Command{
		ID:      "detect_version",
		Command: fmt.Sprintf(`%s -v 2>&1 | grep -o "nginx/[0-9.]*"`, nginxPath),
		Timeout: 10 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return "", err
	}

	if result.CommandResult.ExitCode != 0 {
		return "", fmt.Errorf("nginx版本检测失败: %s", result.CommandResult.Stderr)
	}

	return strings.TrimSpace(result.CommandResult.Stdout), nil
}

// validateConfigPath 验证配置路径
func (s *Service) validateConfigPath(ctx context.Context, serverID, configPath string) error {
	cmd := &server.Command{
		ID:      "validate_path",
		Command: fmt.Sprintf(`test -d "%s" || mkdir -p "%s"`, configPath, configPath),
		Timeout: 10 * time.Second,
	}

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return err
	}

	if result.CommandResult.ExitCode != 0 {
		return fmt.Errorf("配置路径验证失败: %s", result.CommandResult.Stderr)
	}

	return nil
}

// detectConfigType 检测配置文件类型
func (s *Service) detectConfigType(configPath string) server.NginxConfigType {
	name := filepath.Base(configPath)

	if name == "nginx.conf" {
		return server.NginxConfigTypeMain
	}

	if strings.Contains(configPath, "sites-") {
		return server.NginxConfigTypeSite
	}

	if strings.Contains(configPath, "modules") {
		return server.NginxConfigTypeModule
	}

	return server.NginxConfigTypeCustom
}

// executeNginxCommand 执行nginx命令
func (s *Service) executeNginxCommand(ctx context.Context, serverID string, cmd *server.Command) (*server.NginxOperationResult, error) {
	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command:  cmd,
	}

	result, err := s.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}

	return &server.NginxOperationResult{
		Success:  result.CommandResult.ExitCode == 0,
		Message:  "操作完成",
		Output:   result.CommandResult.Stdout,
		ExitCode: result.CommandResult.ExitCode,
		Error:    result.CommandResult.Stderr,
	}, nil
}

// generateSiteConfig 生成站点配置
func (s *Service) generateSiteConfig(site *server.NginxSite) string {
	var config strings.Builder

	// 如果是反向代理且配置了upstream，先生成upstream配置
	if site.Type == server.NginxSiteTypeProxy && site.Upstream != nil {
		config.WriteString(s.generateUpstreamConfig(site.Upstream))
		config.WriteString("\n")
	}

	config.WriteString("server {\n")

	// Listen指令
	for _, listen := range site.Listen {
		config.WriteString(fmt.Sprintf("    listen %s;\n", listen))
	}

	// Server name
	config.WriteString(fmt.Sprintf("    server_name %s;\n\n", site.ServerName))

	// SSL配置
	if site.SSL {
		if site.SSLCert != "" {
			config.WriteString(fmt.Sprintf("    ssl_certificate %s;\n", site.SSLCert))
		}
		if site.SSLKey != "" {
			config.WriteString(fmt.Sprintf("    ssl_certificate_key %s;\n", site.SSLKey))
		}
		config.WriteString("\n")
	}

	// 日志配置
	if site.AccessLog != "" {
		config.WriteString(fmt.Sprintf("    access_log %s;\n", site.AccessLog))
	}
	if site.ErrorLog != "" {
		config.WriteString(fmt.Sprintf("    error_log %s;\n", site.ErrorLog))
	}
	config.WriteString("\n")

	// 根据站点类型生成不同的配置
	switch site.Type {
	case server.NginxSiteTypeStatic:
		s.generateStaticSiteConfig(&config, site)
	case server.NginxSiteTypeProxy:
		s.generateProxySiteConfig(&config, site)
	default:
		s.generateStaticSiteConfig(&config, site)
	}

	// 额外配置
	if site.ExtraConfig != "" {
		config.WriteString("    # 额外配置\n")
		for _, line := range strings.Split(site.ExtraConfig, "\n") {
			if strings.TrimSpace(line) != "" {
				config.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
		config.WriteString("\n")
	}

	config.WriteString("}\n")

	return config.String()
}

// generateUpstreamConfig 生成upstream配置
func (s *Service) generateUpstreamConfig(upstream *server.NginxUpstream) string {
	var config strings.Builder

	config.WriteString(fmt.Sprintf("upstream %s {\n", upstream.Name))

	// 负载均衡方法
	switch upstream.LoadBalance {
	case server.NginxLoadBalanceLeastConn:
		config.WriteString("    least_conn;\n")
	case server.NginxLoadBalanceIPHash:
		config.WriteString("    ip_hash;\n")
	case server.NginxLoadBalanceHash:
		if upstream.HashKey != "" {
			config.WriteString(fmt.Sprintf("    hash %s;\n", upstream.HashKey))
		}
	case server.NginxLoadBalanceRandom:
		config.WriteString("    random;\n")
	}

	// 服务器列表
	for _, server := range upstream.Servers {
		var serverConfig strings.Builder
		serverConfig.WriteString(fmt.Sprintf("    server %s", server.Address))

		if server.Weight > 0 && server.Weight != 1 {
			serverConfig.WriteString(fmt.Sprintf(" weight=%d", server.Weight))
		}
		if server.MaxFails > 0 && server.MaxFails != 1 {
			serverConfig.WriteString(fmt.Sprintf(" max_fails=%d", server.MaxFails))
		}
		if server.FailTimeout != "" && server.FailTimeout != "10s" {
			serverConfig.WriteString(fmt.Sprintf(" fail_timeout=%s", server.FailTimeout))
		}
		if server.Backup {
			serverConfig.WriteString(" backup")
		}
		if server.Down {
			serverConfig.WriteString(" down")
		}
		if server.SlowStart != "" {
			serverConfig.WriteString(fmt.Sprintf(" slow_start=%s", server.SlowStart))
		}

		serverConfig.WriteString(";\n")
		config.WriteString(serverConfig.String())
	}

	// 连接保持
	if upstream.KeepAlive > 0 {
		config.WriteString(fmt.Sprintf("    keepalive %d;\n", upstream.KeepAlive))
	}
	if upstream.KeepaliveTime != "" {
		config.WriteString(fmt.Sprintf("    keepalive_time %s;\n", upstream.KeepaliveTime))
	}
	if upstream.KeepaliveTimeout != "" {
		config.WriteString(fmt.Sprintf("    keepalive_timeout %s;\n", upstream.KeepaliveTimeout))
	}

	config.WriteString("}\n")

	return config.String()
}

// generateStaticSiteConfig 生成静态站点配置
func (s *Service) generateStaticSiteConfig(config *strings.Builder, site *server.NginxSite) {
	// Root directory
	if site.Root != "" {
		config.WriteString(fmt.Sprintf("    root %s;\n", site.Root))
	}

	// Index files
	if len(site.Index) > 0 {
		config.WriteString(fmt.Sprintf("    index %s;\n\n", strings.Join(site.Index, " ")))
	}

	// 如果有自定义location配置
	if len(site.Locations) > 0 {
		for _, location := range site.Locations {
			s.generateLocationConfig(config, &location)
		}
	} else {
		// 默认location配置
		config.WriteString("    location / {\n")
		config.WriteString("        try_files $uri $uri/ =404;\n")
		config.WriteString("    }\n\n")
	}
}

// generateProxySiteConfig 生成反向代理站点配置
func (s *Service) generateProxySiteConfig(config *strings.Builder, site *server.NginxSite) {
	// 全局代理配置
	if site.GlobalProxy != nil {
		s.generateGlobalProxyConfig(config, site.GlobalProxy)
	}

	// location配置
	if len(site.Locations) > 0 {
		for _, location := range site.Locations {
			s.generateLocationConfig(config, &location)
		}
	} else {
		// 默认代理所有请求
		config.WriteString("    location / {\n")
		if site.Upstream != nil {
			config.WriteString(fmt.Sprintf("        proxy_pass http://%s;\n", site.Upstream.Name))
		} else if site.GlobalProxy != nil && site.GlobalProxy.ProxyPass != "" {
			config.WriteString(fmt.Sprintf("        proxy_pass %s;\n", site.GlobalProxy.ProxyPass))
		}

		// 默认代理头部
		config.WriteString("        proxy_set_header Host $host;\n")
		config.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		config.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		config.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
		config.WriteString("    }\n\n")
	}
}

// generateGlobalProxyConfig 生成全局代理配置
func (s *Service) generateGlobalProxyConfig(config *strings.Builder, proxyConfig *server.NginxProxyConfig) {
	if proxyConfig.ProxyTimeout != "" {
		config.WriteString(fmt.Sprintf("    proxy_timeout %s;\n", proxyConfig.ProxyTimeout))
	}
	if proxyConfig.ProxyConnectTimeout != "" {
		config.WriteString(fmt.Sprintf("    proxy_connect_timeout %s;\n", proxyConfig.ProxyConnectTimeout))
	}
	if proxyConfig.ProxyReadTimeout != "" {
		config.WriteString(fmt.Sprintf("    proxy_read_timeout %s;\n", proxyConfig.ProxyReadTimeout))
	}
	if proxyConfig.ProxyBuffering != nil {
		if *proxyConfig.ProxyBuffering {
			config.WriteString("    proxy_buffering on;\n")
		} else {
			config.WriteString("    proxy_buffering off;\n")
		}
	}
	if proxyConfig.ProxyBufferSize != "" {
		config.WriteString(fmt.Sprintf("    proxy_buffer_size %s;\n", proxyConfig.ProxyBufferSize))
	}
	if proxyConfig.ProxyBuffers != "" {
		config.WriteString(fmt.Sprintf("    proxy_buffers %s;\n", proxyConfig.ProxyBuffers))
	}
	if proxyConfig.ProxyRedirect != "" {
		config.WriteString(fmt.Sprintf("    proxy_redirect %s;\n", proxyConfig.ProxyRedirect))
	}
	config.WriteString("\n")
}

// generateLocationConfig 生成location配置
func (s *Service) generateLocationConfig(config *strings.Builder, location *server.NginxLocation) {
	config.WriteString(fmt.Sprintf("    location %s {\n", location.Path))

	// 代理配置
	if location.ProxyConfig != nil {
		if location.ProxyConfig.ProxyPass != "" {
			config.WriteString(fmt.Sprintf("        proxy_pass %s;\n", location.ProxyConfig.ProxyPass))
		}

		// 设置头部
		for key, value := range location.ProxyConfig.ProxySetHeader {
			config.WriteString(fmt.Sprintf("        proxy_set_header %s %s;\n", key, value))
		}

		// 其他代理配置
		if location.ProxyConfig.ProxyTimeout != "" {
			config.WriteString(fmt.Sprintf("        proxy_timeout %s;\n", location.ProxyConfig.ProxyTimeout))
		}
		if location.ProxyConfig.ProxyConnectTimeout != "" {
			config.WriteString(fmt.Sprintf("        proxy_connect_timeout %s;\n", location.ProxyConfig.ProxyConnectTimeout))
		}
		if location.ProxyConfig.ProxyReadTimeout != "" {
			config.WriteString(fmt.Sprintf("        proxy_read_timeout %s;\n", location.ProxyConfig.ProxyReadTimeout))
		}
		if location.ProxyConfig.ProxyBuffering != nil {
			if *location.ProxyConfig.ProxyBuffering {
				config.WriteString("        proxy_buffering on;\n")
			} else {
				config.WriteString("        proxy_buffering off;\n")
			}
		}
		if location.ProxyConfig.ProxyBufferSize != "" {
			config.WriteString(fmt.Sprintf("        proxy_buffer_size %s;\n", location.ProxyConfig.ProxyBufferSize))
		}
		if location.ProxyConfig.ProxyBuffers != "" {
			config.WriteString(fmt.Sprintf("        proxy_buffers %s;\n", location.ProxyConfig.ProxyBuffers))
		}
		if location.ProxyConfig.ProxyRedirect != "" {
			config.WriteString(fmt.Sprintf("        proxy_redirect %s;\n", location.ProxyConfig.ProxyRedirect))
		}
	}

	// try_files配置
	if len(location.TryFiles) > 0 {
		config.WriteString(fmt.Sprintf("        try_files %s;\n", strings.Join(location.TryFiles, " ")))
	}

	// 自定义头部
	for key, value := range location.Headers {
		config.WriteString(fmt.Sprintf("        add_header %s %s;\n", key, value))
	}

	// 速率限制
	if location.RateLimit != nil {
		config.WriteString(fmt.Sprintf("        limit_req zone=%s", location.RateLimit.Zone))
		if location.RateLimit.Burst > 0 {
			config.WriteString(fmt.Sprintf(" burst=%d", location.RateLimit.Burst))
		}
		if location.RateLimit.NoDelay {
			config.WriteString(" nodelay")
		}
		config.WriteString(";\n")
	}

	// 额外配置
	if location.ExtraConfig != "" {
		for _, line := range strings.Split(location.ExtraConfig, "\n") {
			if strings.TrimSpace(line) != "" {
				config.WriteString(fmt.Sprintf("        %s\n", line))
			}
		}
	}

	config.WriteString("    }\n\n")
}

// CreateDefaultProxyHeaders 创建默认代理头部配置
func CreateDefaultProxyHeaders() map[string]string {
	return map[string]string{
		"Host":              "$host",
		"X-Real-IP":         "$remote_addr",
		"X-Forwarded-For":   "$proxy_add_x_forwarded_for",
		"X-Forwarded-Proto": "$scheme",
	}
}

// CreateWebSocketProxyHeaders 创建WebSocket代理头部配置
func CreateWebSocketProxyHeaders() map[string]string {
	headers := CreateDefaultProxyHeaders()
	headers["Upgrade"] = "$http_upgrade"
	headers["Connection"] = "$connection_upgrade"
	return headers
}

// CreateDefaultProxyConfig 创建默认代理配置
func CreateDefaultProxyConfig(proxyPass string) *server.NginxProxyConfig {
	return &server.NginxProxyConfig{
		ProxyPass:           proxyPass,
		ProxySetHeader:      CreateDefaultProxyHeaders(),
		ProxyConnectTimeout: "5s",
		ProxyReadTimeout:    "60s",
	}
}

// CreateLoadBalancedUpstream 创建负载均衡upstream配置
func CreateLoadBalancedUpstream(name string, servers []string, method server.NginxLoadBalanceMethod) *server.NginxUpstream {
	upstreamServers := make([]server.NginxUpstreamServer, len(servers))
	for i, addr := range servers {
		upstreamServers[i] = server.NginxUpstreamServer{
			Address:     addr,
			Weight:      1,
			MaxFails:    3,
			FailTimeout: "30s",
		}
	}

	return &server.NginxUpstream{
		Name:        name,
		Servers:     upstreamServers,
		LoadBalance: method,
		KeepAlive:   32,
	}
}

// CreateAPIGatewayLocation 创建API网关location配置
func CreateAPIGatewayLocation(path, upstreamName string) server.NginxLocation {
	return server.NginxLocation{
		Path: path,
		ProxyConfig: &server.NginxProxyConfig{
			ProxyPass:           fmt.Sprintf("http://%s", upstreamName),
			ProxySetHeader:      CreateDefaultProxyHeaders(),
			ProxyConnectTimeout: "5s",
			ProxyReadTimeout:    "60s",
			ProxyBuffering:      &[]bool{false}[0], // 禁用缓冲用于实时API
		},
		Headers: map[string]string{
			"X-API-Gateway": "nginx",
		},
	}
}

// CreateStaticAssetsLocation 创建静态资源location配置
func CreateStaticAssetsLocation(path, root string) server.NginxLocation {
	return server.NginxLocation{
		Path:     path,
		TryFiles: []string{"$uri", "$uri/", "=404"},
		Headers: map[string]string{
			"Cache-Control": "public, max-age=31536000",
			"Expires":       "1y",
		},
		ExtraConfig: fmt.Sprintf("root %s;", root),
	}
}

// ValidateUpstreamConfig 验证upstream配置
func ValidateUpstreamConfig(upstream *server.NginxUpstream) error {
	if upstream == nil {
		return fmt.Errorf("upstream配置不能为空")
	}

	if upstream.Name == "" {
		return fmt.Errorf("upstream名称不能为空")
	}

	if len(upstream.Servers) == 0 {
		return fmt.Errorf("upstream必须至少包含一个服务器")
	}

	for i, server := range upstream.Servers {
		if server.Address == "" {
			return fmt.Errorf("服务器%d的地址不能为空", i+1)
		}

		// 简单的地址格式验证
		if !strings.Contains(server.Address, ":") {
			return fmt.Errorf("服务器%d的地址格式无效，应为 ip:port 或 domain:port", i+1)
		}
	}

	return nil
}

// ValidateSiteConfig 验证站点配置
func ValidateSiteConfig(site *server.NginxSite) error {
	if site.Name == "" {
		return fmt.Errorf("站点名称不能为空")
	}

	if site.ServerName == "" {
		return fmt.Errorf("域名不能为空")
	}

	// 根据站点类型验证不同的字段
	switch site.Type {
	case server.NginxSiteTypeStatic:
		if site.Root == "" {
			return fmt.Errorf("静态站点必须指定根目录")
		}
	case server.NginxSiteTypeProxy:
		if site.Upstream == nil && site.GlobalProxy == nil && len(site.Locations) == 0 {
			return fmt.Errorf("代理站点必须配置upstream、全局代理或location配置")
		}

		if site.Upstream != nil {
			if err := ValidateUpstreamConfig(site.Upstream); err != nil {
				return fmt.Errorf("upstream配置错误: %w", err)
			}
		}
	}

	// 验证SSL配置
	if site.SSL {
		if site.SSLCert == "" || site.SSLKey == "" {
			return fmt.Errorf("启用SSL时必须提供证书文件和私钥文件路径")
		}
	}

	return nil
}
