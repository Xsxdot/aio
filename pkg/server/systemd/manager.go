package systemd

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/xsxdot/aio/pkg/server"
)

// ServiceNotFoundError 服务不存在错误
type ServiceNotFoundError struct {
	ServiceName string
}

func (e *ServiceNotFoundError) Error() string {
	return fmt.Sprintf("服务 '%s' 不存在", e.ServiceName)
}

// IsServiceNotFound 检查错误是否为服务不存在错误
func IsServiceNotFound(err error) bool {
	_, ok := err.(*ServiceNotFoundError)
	return ok
}

// StartService 启动服务
func (m *Manager) StartService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl start %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("start-%s", serviceName),
			Name:    fmt.Sprintf("启动服务 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  fmt.Sprintf("服务 %s 启动操作完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// StopService 停止服务
func (m *Manager) StopService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl stop %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("stop-%s", serviceName),
			Name:    fmt.Sprintf("停止服务 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  fmt.Sprintf("服务 %s 停止操作完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// RestartService 重启服务
func (m *Manager) RestartService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl restart %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("restart-%s", serviceName),
			Name:    fmt.Sprintf("重启服务 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  fmt.Sprintf("服务 %s 重启操作完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// ReloadService 重载服务
func (m *Manager) ReloadService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl reload %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("reload-%s", serviceName),
			Name:    fmt.Sprintf("重载服务 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  fmt.Sprintf("服务 %s 重载操作完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// GetService 获取单个服务信息
func (m *Manager) GetService(ctx context.Context, serverID, serviceName string) (*server.SystemdService, error) {
	// 首先检查服务是否存在（通过检查服务文件或在systemctl列表中查找）
	exists, err := m.checkServiceExists(ctx, serverID, serviceName)
	if err != nil {
		return nil, fmt.Errorf("检查服务是否存在失败: %w", err)
	}

	if !exists {
		return nil, &ServiceNotFoundError{ServiceName: serviceName}
	}

	// 获取服务状态信息
	statusCommand := fmt.Sprintf("sudo systemctl show %s --property=ActiveState,UnitFileState,Description,Type,ExecStart,ExecReload,ExecStop,WorkingDirectory,User,Group,PIDFile,Restart", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("show-%s", serviceName),
			Name:    fmt.Sprintf("获取服务信息 %s", serviceName),
			Command: statusCommand,
		},
	}

	statusResult, err := m.executor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("获取服务状态失败: %w", err)
	}

	if statusResult.CommandResult.Status != server.CommandStatusSuccess {
		return nil, fmt.Errorf("获取服务状态失败: %s", statusResult.CommandResult.Error)
	}

	service := &server.SystemdService{
		Name:        serviceName,
		Environment: make(map[string]string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 解析服务状态信息
	lines := strings.Split(statusResult.CommandResult.Stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "ActiveState":
					service.Status = server.ServiceState(value)
				case "UnitFileState":
					service.Enabled = value == "enabled"
				case "Description":
					service.Description = value
				case "Type":
					service.Type = server.ServiceType(value)
				case "ExecStart":
					service.ExecStart = value
				case "ExecReload":
					service.ExecReload = value
				case "ExecStop":
					service.ExecStop = value
				case "WorkingDirectory":
					service.WorkingDir = value
				case "User":
					service.User = value
				case "Group":
					service.Group = value
				case "PIDFile":
					service.PIDFile = value
				case "Restart":
					service.Restart = value
				}
			}
		}
	}

	// 获取环境变量
	envCommand := fmt.Sprintf("sudo systemctl show %s --property=Environment", serviceName)

	envReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("env-%s", serviceName),
			Name:    fmt.Sprintf("获取服务环境变量 %s", serviceName),
			Command: envCommand,
		},
	}

	envResult, err := m.executor.Execute(ctx, envReq)
	if err == nil && envResult.CommandResult.Status == server.CommandStatusSuccess {
		envLine := strings.TrimSpace(envResult.CommandResult.Stdout)
		if strings.HasPrefix(envLine, "Environment=") {
			envVars := strings.TrimPrefix(envLine, "Environment=")
			service.Environment = parseEnvironmentVariables(envVars)
		}
	}

	return service, nil
}

// checkServiceExists 检查服务是否存在
func (m *Manager) checkServiceExists(ctx context.Context, serverID, serviceName string) (bool, error) {
	// 方法1: 检查服务文件是否存在
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	checkFileCommand := fmt.Sprintf("test -f %s && echo 'file_exists' || echo 'file_not_exists'", serviceFilePath)

	fileReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("check-file-%s", serviceName),
			Name:    fmt.Sprintf("检查服务文件 %s", serviceName),
			Command: checkFileCommand,
		},
	}

	fileResult, err := m.executor.Execute(ctx, fileReq)
	if err != nil {
		return false, fmt.Errorf("检查服务文件失败: %w", err)
	}

	if fileResult.CommandResult.Status == server.CommandStatusSuccess {
		fileExists := strings.TrimSpace(fileResult.CommandResult.Stdout) == "file_exists"
		if fileExists {
			return true, nil
		}
	}

	// 方法2: 检查系统中是否有该服务单元（包括系统服务）
	// 使用systemctl list-unit-files检查是否有该服务
	listCommand := fmt.Sprintf("sudo systemctl list-unit-files --type=service --no-pager | grep '^%s.service' | wc -l", serviceName)

	listReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("check-unit-%s", serviceName),
			Name:    fmt.Sprintf("检查服务单元 %s", serviceName),
			Command: listCommand,
		},
	}

	listResult, err := m.executor.Execute(ctx, listReq)
	if err != nil {
		return false, fmt.Errorf("检查服务单元失败: %w", err)
	}

	if listResult.CommandResult.Status == server.CommandStatusSuccess {
		countStr := strings.TrimSpace(listResult.CommandResult.Stdout)
		if countStr == "1" {
			return true, nil
		}
	}

	// 方法3: 使用systemctl status检查服务是否被systemd识别
	statusCommand := fmt.Sprintf("sudo systemctl status %s --no-pager -l 2>&1", serviceName)

	statusReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("check-status-%s", serviceName),
			Name:    fmt.Sprintf("检查服务状态 %s", serviceName),
			Command: statusCommand,
		},
	}

	statusResult, err := m.executor.Execute(ctx, statusReq)
	if err != nil {
		// 如果命令执行失败，也可能是服务不存在
		return false, nil
	}

	// 检查输出中是否包含"could not be found"或类似的不存在标识
	output := strings.ToLower(statusResult.CommandResult.Stdout + " " + statusResult.CommandResult.Error)
	notFoundIndicators := []string{
		"could not be found",
		"not found",
		"no such file or directory",
		"unit " + serviceName + ".service could not be found",
		"failed to get properties",
	}

	for _, indicator := range notFoundIndicators {
		if strings.Contains(output, indicator) {
			return false, nil
		}
	}

	// 如果没有明确的不存在标识，认为服务存在
	return true, nil
}

// getUserCreatedServices 获取用户创建的服务列表
func (m *Manager) getUserCreatedServices(ctx context.Context, serverID, pattern string) []string {
	var services []string

	// 获取 /etc/systemd/system/ 目录下的 .service 文件
	command := "sudo find /etc/systemd/system/ -name '*.service' -type f"

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      "list-user-services",
			Name:    "获取用户创建的服务",
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil || result.CommandResult.Status != server.CommandStatusSuccess {
		return services
	}

	// 解析每个service文件
	serviceFiles := strings.Split(strings.TrimSpace(result.CommandResult.Stdout), "\n")
	for _, filePath := range serviceFiles {
		if strings.TrimSpace(filePath) == "" {
			continue
		}

		// 从文件路径提取服务名称
		fileName := strings.TrimSuffix(strings.Split(filePath, "/")[len(strings.Split(filePath, "/"))-1], ".service")

		// 应用模式匹配过滤
		if pattern != "" && !strings.Contains(fileName, pattern) {
			continue
		}

		services = append(services, fileName)
	}

	return services
}

// ListServices 获取服务列表
func (m *Manager) ListServices(ctx context.Context, serverID string, req *server.ServiceListRequest) ([]*server.SystemdService, int, error) {
	var services []*server.SystemdService

	// 获取所有服务文件（包括已禁用的）
	unitFilesCommand := "sudo systemctl list-unit-files --type=service --no-pager"
	unitsCommand := "sudo systemctl list-units --type=service --all --no-pager"

	// 并行执行两个命令
	unitFilesReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      "list-unit-files",
			Name:    "获取所有服务文件",
			Command: unitFilesCommand,
		},
	}

	unitsReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      "list-units",
			Name:    "获取运行中的服务",
			Command: unitsCommand,
		},
	}

	// 执行获取服务文件命令
	unitFilesResult, err := m.executor.Execute(ctx, unitFilesReq)
	if err != nil {
		return nil, 0, fmt.Errorf("获取服务文件列表失败: %w", err)
	}

	if unitFilesResult.CommandResult.Status != server.CommandStatusSuccess {
		return nil, 0, fmt.Errorf("获取服务文件列表失败: %s", unitFilesResult.CommandResult.Error)
	}

	// 执行获取运行中服务命令
	unitsResult, err := m.executor.Execute(ctx, unitsReq)
	if err != nil {
		return nil, 0, fmt.Errorf("获取运行中服务列表失败: %w", err)
	}

	if unitsResult.CommandResult.Status != server.CommandStatusSuccess {
		return nil, 0, fmt.Errorf("获取运行中服务列表失败: %s", unitsResult.CommandResult.Error)
	}

	// 解析服务文件列表和运行状态，合并信息
	services = parseServiceListCombined(unitFilesResult.CommandResult.Stdout, unitsResult.CommandResult.Stdout)

	// 应用模式匹配过滤
	if req.Pattern != "" {
		var filteredByPattern []*server.SystemdService
		for _, service := range services {
			if strings.Contains(service.Name, req.Pattern) {
				filteredByPattern = append(filteredByPattern, service)
			}
		}
		services = filteredByPattern
	}

	if req.UserOnly {
		onlyUser := m.getUserCreatedServices(ctx, serverID, req.Pattern)
		var filteredByUser []*server.SystemdService
		for _, userService := range onlyUser {
			for _, service := range services {
				if service.Name == userService {
					filteredByUser = append(filteredByUser, service)
				}
			}
		}
		services = filteredByUser
	}

	// 过滤服务
	var filteredServices []*server.SystemdService
	for _, service := range services {
		// 状态过滤
		if req.Status != "" && service.Status != req.Status {
			continue
		}

		// 启用状态过滤
		if req.Enabled != nil && service.Enabled != *req.Enabled {
			continue
		}

		filteredServices = append(filteredServices, service)
	}

	total := len(filteredServices)

	// 分页处理
	if req.Limit > 0 {
		start := req.Offset
		end := start + req.Limit

		if start > total {
			return []*server.SystemdService{}, total, nil
		}

		if end > total {
			end = total
		}

		filteredServices = filteredServices[start:end]
	}

	return filteredServices, total, nil
}

// GetServiceStatus 获取服务状态
func (m *Manager) GetServiceStatus(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl is-active %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("status-%s", serviceName),
			Name:    fmt.Sprintf("获取服务状态 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	status := strings.TrimSpace(result.CommandResult.Stdout)
	message := fmt.Sprintf("服务 %s 当前状态: %s", serviceName, status)

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  message,
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// CreateService 创建服务
func (m *Manager) CreateService(ctx context.Context, serverID string, req *server.ServiceCreateRequest) (*server.SystemdService, error) {
	// 验证请求参数
	if err := validateServiceCreateRequest(req); err != nil {
		return nil, fmt.Errorf("服务创建请求验证失败: %w", err)
	}

	// 检查服务是否已存在
	existingService, err := m.GetService(ctx, serverID, req.Name)
	if err != nil {
		// 如果不是服务不存在错误，则返回错误
		if !IsServiceNotFound(err) {
			return nil, fmt.Errorf("检查服务是否存在失败: %w", err)
		}
		// 服务不存在，可以继续创建
	} else if existingService != nil {
		// 服务已存在
		return nil, fmt.Errorf("服务 '%s' 已存在", req.Name)
	}

	// 生成service文件内容
	serviceContent := generateServiceFileContent(req)

	// 创建服务文件
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", req.Name)
	createCommand := fmt.Sprintf("sudo tee %s > /dev/null << 'EOF'\n%s\nEOF", servicePath, serviceContent)

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("create-%s", req.Name),
			Name:    fmt.Sprintf("创建服务 %s", req.Name),
			Command: createCommand,
		},
	}

	result, err := m.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("创建服务文件失败: %w", err)
	}

	if result.CommandResult.Status != server.CommandStatusSuccess {
		return nil, fmt.Errorf("创建服务文件失败: %s", result.CommandResult.Error)
	}

	// 重载systemd配置
	reloadResult, err := m.DaemonReload(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("重载systemd配置失败: %w", err)
	}

	if !reloadResult.Success {
		return nil, fmt.Errorf("重载systemd配置失败: %s", reloadResult.Error)
	}

	// 如果需要启用服务
	if req.Enabled {
		_, err = m.EnableService(ctx, serverID, req.Name)
		if err != nil {
			return nil, fmt.Errorf("启用服务失败: %w", err)
		}
	}

	// 返回创建的服务信息
	return m.GetService(ctx, serverID, req.Name)
}

// UpdateService 更新服务
func (m *Manager) UpdateService(ctx context.Context, serverID, serviceName string, req *server.ServiceUpdateRequest) (*server.SystemdService, error) {
	// 先获取当前服务信息
	currentService, err := m.GetService(ctx, serverID, serviceName)
	if err != nil {
		return nil, fmt.Errorf("获取当前服务信息失败: %w", err)
	}

	// 应用更新
	updateRequest := applyServiceUpdate(currentService, req)

	// 生成新的服务文件内容
	serviceContent := generateServiceFileContent(updateRequest)

	// 更新服务文件
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	updateCommand := fmt.Sprintf("sudo tee %s > /dev/null << 'EOF'\n%s\nEOF", servicePath, serviceContent)

	executeReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("update-%s", serviceName),
			Name:    fmt.Sprintf("更新服务 %s", serviceName),
			Command: updateCommand,
		},
	}

	result, err := m.executor.Execute(ctx, executeReq)
	if err != nil {
		return nil, fmt.Errorf("更新服务文件失败: %w", err)
	}

	if result.CommandResult.Status != server.CommandStatusSuccess {
		return nil, fmt.Errorf("更新服务文件失败: %s", result.CommandResult.Error)
	}

	// 重载systemd配置
	reloadResult, err := m.DaemonReload(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("重载systemd配置失败: %w", err)
	}

	if !reloadResult.Success {
		return nil, fmt.Errorf("重载systemd配置失败: %s", reloadResult.Error)
	}

	// 处理启用状态变更
	if req.Enabled != nil {
		if *req.Enabled {
			_, err = m.EnableService(ctx, serverID, serviceName)
		} else {
			_, err = m.DisableService(ctx, serverID, serviceName)
		}
		if err != nil {
			return nil, fmt.Errorf("更新服务启用状态失败: %w", err)
		}
	}

	// 返回更新后的服务信息
	return m.GetService(ctx, serverID, serviceName)
}

// DeleteService 删除服务
func (m *Manager) DeleteService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	// 先停止服务
	stopResult, err := m.StopService(ctx, serverID, serviceName)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   fmt.Sprintf("停止服务失败: %s", err.Error()),
		}, err
	}

	// 禁用服务
	disableResult, err := m.DisableService(ctx, serverID, serviceName)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   fmt.Sprintf("禁用服务失败: %s", err.Error()),
		}, err
	}

	// 删除服务文件
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	deleteCommand := fmt.Sprintf("sudo rm -f %s", servicePath)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("delete-%s", serviceName),
			Name:    fmt.Sprintf("删除服务 %s", serviceName),
			Command: deleteCommand,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// 重载systemd配置
	reloadResult, err := m.DaemonReload(ctx, serverID)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   fmt.Sprintf("重载systemd配置失败: %s", err.Error()),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess && stopResult.Success && disableResult.Success && reloadResult.Success,
		Message:  fmt.Sprintf("服务 %s 删除完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// EnableService 启用服务
func (m *Manager) EnableService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl enable %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("enable-%s", serviceName),
			Name:    fmt.Sprintf("启用服务 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  fmt.Sprintf("服务 %s 启用操作完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// DisableService 禁用服务
func (m *Manager) DisableService(ctx context.Context, serverID, serviceName string) (*server.ServiceOperationResult, error) {
	command := fmt.Sprintf("sudo systemctl disable %s", serviceName)

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("disable-%s", serviceName),
			Name:    fmt.Sprintf("禁用服务 %s", serviceName),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  fmt.Sprintf("服务 %s 禁用操作完成", serviceName),
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// GetServiceLogs 获取服务日志
func (m *Manager) GetServiceLogs(ctx context.Context, req *server.ServiceLogRequest) (*server.ServiceLogResult, error) {
	lines := req.Lines
	if lines <= 0 {
		lines = 100
	}

	command := fmt.Sprintf("sudo journalctl -u %s -n %d --no-pager", req.Name, lines)
	if req.Follow {
		command += " -f"
	}

	executeReq := &server.ExecuteRequest{
		ServerID: req.ServerID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("logs-%s", req.Name),
			Name:    fmt.Sprintf("获取服务日志 %s", req.Name),
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, executeReq)
	if err != nil {
		return &server.ServiceLogResult{
			Error: err.Error(),
		}, err
	}

	if result.CommandResult.Status != server.CommandStatusSuccess {
		return &server.ServiceLogResult{
			Error: result.CommandResult.Error,
		}, nil
	}

	logs := strings.Split(result.CommandResult.Stdout, "\n")
	return &server.ServiceLogResult{
		Logs: logs,
	}, nil
}

// ReloadSystemd 重载systemd
func (m *Manager) ReloadSystemd(ctx context.Context, serverID string) (*server.ServiceOperationResult, error) {
	return m.DaemonReload(ctx, serverID)
}

// DaemonReload 重载systemd守护进程
func (m *Manager) DaemonReload(ctx context.Context, serverID string) (*server.ServiceOperationResult, error) {
	command := "sudo systemctl daemon-reload"

	req := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      "daemon-reload",
			Name:    "重载systemd守护进程",
			Command: command,
		},
	}

	result, err := m.executor.Execute(ctx, req)
	if err != nil {
		return &server.ServiceOperationResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &server.ServiceOperationResult{
		Success:  result.CommandResult.Status == server.CommandStatusSuccess,
		Message:  "systemd守护进程重载完成",
		Output:   result.CommandResult.Stdout,
		Error:    result.CommandResult.Error,
		ExitCode: result.CommandResult.ExitCode,
	}, nil
}

// GetServiceFileContent 获取服务文件内容
func (m *Manager) GetServiceFileContent(ctx context.Context, serverID, serviceName string) (*server.ServiceFileResult, error) {
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	// 先检查文件是否存在
	checkCommand := fmt.Sprintf("test -f %s && echo 'exists' || echo 'not_exists'", servicePath)

	checkReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("check-%s", serviceName),
			Name:    fmt.Sprintf("检查服务文件 %s", serviceName),
			Command: checkCommand,
		},
	}

	checkResult, err := m.executor.Execute(ctx, checkReq)
	if err != nil {
		return &server.ServiceFileResult{
			ServiceName: serviceName,
			FilePath:    servicePath,
			Content:     "",
			Exists:      false,
			Error:       fmt.Sprintf("检查文件失败: %s", err.Error()),
		}, err
	}

	exists := strings.TrimSpace(checkResult.CommandResult.Stdout) == "exists"

	if !exists {
		return &server.ServiceFileResult{
			ServiceName: serviceName,
			FilePath:    servicePath,
			Content:     "",
			Exists:      false,
			Error:       "",
		}, nil
	}

	// 读取文件内容
	readCommand := fmt.Sprintf("sudo cat %s", servicePath)

	readReq := &server.ExecuteRequest{
		ServerID: serverID,
		Type:     server.CommandTypeSingle,
		Command: &server.Command{
			ID:      fmt.Sprintf("read-%s", serviceName),
			Name:    fmt.Sprintf("读取服务文件 %s", serviceName),
			Command: readCommand,
		},
	}

	readResult, err := m.executor.Execute(ctx, readReq)
	if err != nil {
		return &server.ServiceFileResult{
			ServiceName: serviceName,
			FilePath:    servicePath,
			Content:     "",
			Exists:      true,
			Error:       fmt.Sprintf("读取文件失败: %s", err.Error()),
		}, err
	}

	if readResult.CommandResult.Status != server.CommandStatusSuccess {
		return &server.ServiceFileResult{
			ServiceName: serviceName,
			FilePath:    servicePath,
			Content:     "",
			Exists:      true,
			Error:       fmt.Sprintf("读取文件失败: %s", readResult.CommandResult.Error),
		}, nil
	}

	return &server.ServiceFileResult{
		ServiceName: serviceName,
		FilePath:    servicePath,
		Content:     readResult.CommandResult.Stdout,
		Exists:      true,
		Error:       "",
	}, nil
}

// parseServiceListCombined 解析合并的服务列表输出
func parseServiceListCombined(unitFilesOutput, unitsOutput string) []*server.SystemdService {
	services := make(map[string]*server.SystemdService)

	// 首先解析服务文件列表（包含所有服务文件及其启用状态）
	unitFilesLines := strings.Split(unitFilesOutput, "\n")
	for i, line := range unitFilesLines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// 解析格式：UNIT FILE                                  STATE     VENDOR PRESET
		re := regexp.MustCompile(`^\s*(\S+\.service)\s+(\S+)`)
		matches := re.FindStringSubmatch(line)

		if len(matches) >= 3 {
			serviceName := strings.TrimSuffix(matches[1], ".service")
			enabledState := matches[2]

			service := &server.SystemdService{
				Name:        serviceName,
				Status:      server.ServiceStateInactive, // 默认为inactive
				Enabled:     enabledState == "enabled",
				Description: "",
				Environment: make(map[string]string),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			services[serviceName] = service
		}
	}

	// 然后解析运行状态（包含活动状态和描述）
	unitsLines := strings.Split(unitsOutput, "\n")
	for i, line := range unitsLines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// 使用正则表达式解析服务信息
		re := regexp.MustCompile(`^\s*(\S+\.service)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)$`)
		matches := re.FindStringSubmatch(line)

		if len(matches) >= 4 {
			serviceName := strings.TrimSuffix(matches[1], ".service")
			loadState := matches[2]
			activeState := matches[3]
			subState := matches[4]
			description := ""
			if len(matches) > 5 {
				description = matches[5]
			}

			// 如果服务已存在于map中，更新其状态
			if service, exists := services[serviceName]; exists {
				service.Status = server.ServiceState(activeState)
				service.Description = description

				// 根据子状态进一步判断
				if subState == "running" {
					service.Status = server.ServiceStateActive
				} else if subState == "dead" {
					service.Status = server.ServiceStateInactive
				} else if subState == "failed" {
					service.Status = server.ServiceStateFailed
				}

				// 更新启用状态（以实际加载状态为准）
				if loadState == "loaded" {
					// 保持从unit-files获取的enabled状态
				}
			} else {
				// 如果服务不在unit-files中，可能是运行时创建的
				service := &server.SystemdService{
					Name:        serviceName,
					Status:      server.ServiceState(activeState),
					Enabled:     loadState == "loaded",
					Description: description,
					Environment: make(map[string]string),
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}

				// 根据子状态进一步判断
				if subState == "running" {
					service.Status = server.ServiceStateActive
				} else if subState == "dead" {
					service.Status = server.ServiceStateInactive
				} else if subState == "failed" {
					service.Status = server.ServiceStateFailed
				}

				services[serviceName] = service
			}
		}
	}

	// 转换map为slice
	var result []*server.SystemdService
	for _, service := range services {
		result = append(result, service)
	}

	return result
}

// parseServiceList 解析服务列表输出（保留作为向后兼容）
func parseServiceList(output string) []*server.SystemdService {
	var services []*server.SystemdService
	lines := strings.Split(output, "\n")

	// 跳过标题行
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// 使用正则表达式解析服务信息
		re := regexp.MustCompile(`^\s*(\S+\.service)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)$`)
		matches := re.FindStringSubmatch(line)

		if len(matches) >= 4 {
			serviceName := strings.TrimSuffix(matches[1], ".service")
			loadState := matches[2]
			activeState := matches[3]
			subState := matches[4]
			description := ""
			if len(matches) > 5 {
				description = matches[5]
			}

			service := &server.SystemdService{
				Name:        serviceName,
				Status:      server.ServiceState(activeState),
				Enabled:     loadState == "loaded",
				Description: description,
				Environment: make(map[string]string),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			// 根据子状态进一步判断
			if subState == "running" {
				service.Status = server.ServiceStateActive
			} else if subState == "dead" {
				service.Status = server.ServiceStateInactive
			} else if subState == "failed" {
				service.Status = server.ServiceStateFailed
			}

			services = append(services, service)
		}
	}

	return services
}

// parseEnvironmentVariables 解析环境变量
func parseEnvironmentVariables(envString string) map[string]string {
	env := make(map[string]string)

	// 简单的环境变量解析，支持 KEY=VALUE 格式
	vars := strings.Fields(envString)
	for _, v := range vars {
		if strings.Contains(v, "=") {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}
	}

	return env
}

// generateServiceFileContent 生成服务文件内容
func generateServiceFileContent(req *server.ServiceCreateRequest) string {
	var content strings.Builder

	content.WriteString("[Unit]\n")
	if req.Description != "" {
		content.WriteString(fmt.Sprintf("Description=%s\n", req.Description))
	}
	content.WriteString("After=network.target\n\n")

	content.WriteString("[Service]\n")

	// 服务类型
	serviceType := req.Type
	if serviceType == "" {
		serviceType = server.ServiceTypeSimple
	}
	content.WriteString(fmt.Sprintf("Type=%s\n", serviceType))

	// 执行命令
	content.WriteString(fmt.Sprintf("ExecStart=%s\n", req.ExecStart))

	if req.ExecReload != "" {
		content.WriteString(fmt.Sprintf("ExecReload=%s\n", req.ExecReload))
	}

	if req.ExecStop != "" {
		content.WriteString(fmt.Sprintf("ExecStop=%s\n", req.ExecStop))
	}

	// 工作目录
	if req.WorkingDir != "" {
		content.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", req.WorkingDir))
	}

	// 运行用户和组
	if req.User != "" {
		content.WriteString(fmt.Sprintf("User=%s\n", req.User))
	}

	if req.Group != "" {
		content.WriteString(fmt.Sprintf("Group=%s\n", req.Group))
	}

	// PID文件
	if req.PIDFile != "" {
		content.WriteString(fmt.Sprintf("PIDFile=%s\n", req.PIDFile))
	}

	// 重启策略
	restart := req.Restart
	if restart == "" {
		restart = "on-failure"
	}
	content.WriteString(fmt.Sprintf("Restart=%s\n", restart))

	// 环境变量
	if len(req.Environment) > 0 {
		var envVars []string
		for key, value := range req.Environment {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		}
		content.WriteString(fmt.Sprintf("Environment=%s\n", strings.Join(envVars, " ")))
	}

	content.WriteString("\n[Install]\n")
	content.WriteString("WantedBy=multi-user.target\n")

	return content.String()
}

// applyServiceUpdate 应用服务更新
func applyServiceUpdate(current *server.SystemdService, req *server.ServiceUpdateRequest) *server.ServiceCreateRequest {
	updateReq := &server.ServiceCreateRequest{
		Name:        current.Name,
		Description: current.Description,
		Type:        current.Type,
		ExecStart:   current.ExecStart,
		ExecReload:  current.ExecReload,
		ExecStop:    current.ExecStop,
		WorkingDir:  current.WorkingDir,
		User:        current.User,
		Group:       current.Group,
		Environment: current.Environment,
		PIDFile:     current.PIDFile,
		Restart:     current.Restart,
		Enabled:     current.Enabled,
	}

	// 应用更新字段
	if req.Description != nil {
		updateReq.Description = *req.Description
	}
	if req.Type != nil {
		updateReq.Type = *req.Type
	}
	if req.ExecStart != nil {
		updateReq.ExecStart = *req.ExecStart
	}
	if req.ExecReload != nil {
		updateReq.ExecReload = *req.ExecReload
	}
	if req.ExecStop != nil {
		updateReq.ExecStop = *req.ExecStop
	}
	if req.WorkingDir != nil {
		updateReq.WorkingDir = *req.WorkingDir
	}
	if req.User != nil {
		updateReq.User = *req.User
	}
	if req.Group != nil {
		updateReq.Group = *req.Group
	}
	if req.Environment != nil {
		updateReq.Environment = *req.Environment
	}
	if req.PIDFile != nil {
		updateReq.PIDFile = *req.PIDFile
	}
	if req.Restart != nil {
		updateReq.Restart = *req.Restart
	}
	if req.Enabled != nil {
		updateReq.Enabled = *req.Enabled
	}

	return updateReq
}

// validateServiceCreateRequest 验证服务创建请求
func validateServiceCreateRequest(req *server.ServiceCreateRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}

	// 验证服务名称
	if err := validateServiceName(req.Name); err != nil {
		return fmt.Errorf("服务名称验证失败: %w", err)
	}

	// 验证ExecStart (必填)
	if strings.TrimSpace(req.ExecStart) == "" {
		return fmt.Errorf("ExecStart 不能为空")
	}

	// 验证ExecStart路径格式
	if err := validateExecutablePath(req.ExecStart); err != nil {
		return fmt.Errorf("ExecStart 路径验证失败: %w", err)
	}

	// 验证服务类型
	if req.Type != "" {
		if err := validateServiceType(req.Type); err != nil {
			return fmt.Errorf("服务类型验证失败: %w", err)
		}
	}

	// 验证用户名
	if req.User != "" {
		if err := validateUsername(req.User); err != nil {
			return fmt.Errorf("用户名验证失败: %w", err)
		}
	}

	// 验证组名
	if req.Group != "" {
		if err := validateGroupname(req.Group); err != nil {
			return fmt.Errorf("组名验证失败: %w", err)
		}
	}

	// 验证工作目录
	if req.WorkingDir != "" {
		if err := validateWorkingDirectory(req.WorkingDir); err != nil {
			return fmt.Errorf("工作目录验证失败: %w", err)
		}
	}

	// 验证PID文件路径
	if req.PIDFile != "" {
		if err := validatePIDFilePath(req.PIDFile); err != nil {
			return fmt.Errorf("PID文件路径验证失败: %w", err)
		}
	}

	// 验证重启策略
	if req.Restart != "" {
		if err := validateRestartPolicy(req.Restart); err != nil {
			return fmt.Errorf("重启策略验证失败: %w", err)
		}
	}

	// 验证环境变量
	if err := validateEnvironmentVariables(req.Environment); err != nil {
		return fmt.Errorf("环境变量验证失败: %w", err)
	}

	// 验证可选的执行命令
	if req.ExecReload != "" {
		if err := validateExecutablePath(req.ExecReload); err != nil {
			return fmt.Errorf("ExecReload 路径验证失败: %w", err)
		}
	}

	if req.ExecStop != "" {
		if err := validateExecutablePath(req.ExecStop); err != nil {
			return fmt.Errorf("ExecStop 路径验证失败: %w", err)
		}
	}

	return nil
}

// validateServiceName 验证服务名称
func validateServiceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	// 服务名称长度限制
	if len(name) > 253 {
		return fmt.Errorf("服务名称长度不能超过253个字符")
	}

	// systemd服务名称规则: 只能包含字母、数字、连字符、下划线和点
	// 不能以点开头，不能包含两个连续的点
	serviceNamePattern := `^[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`
	matched, err := regexp.MatchString(serviceNamePattern, name)
	if err != nil {
		return fmt.Errorf("正则表达式验证失败: %w", err)
	}
	if !matched {
		return fmt.Errorf("服务名称格式不正确，只能包含字母、数字、连字符、下划线和点，且不能以点开头")
	}

	// 检查是否包含连续的点
	if strings.Contains(name, "..") {
		return fmt.Errorf("服务名称不能包含连续的点")
	}

	// 检查保留名称
	reservedNames := []string{"systemd", "kernel", "kthreadd", "migration"}
	for _, reserved := range reservedNames {
		if strings.EqualFold(name, reserved) {
			return fmt.Errorf("服务名称 '%s' 是保留名称，不能使用", name)
		}
	}

	return nil
}

// validateExecutablePath 验证可执行文件路径
func validateExecutablePath(execPath string) error {
	execPath = strings.TrimSpace(execPath)
	if execPath == "" {
		return fmt.Errorf("可执行文件路径不能为空")
	}

	// 分割命令和参数
	parts := strings.Fields(execPath)
	if len(parts) == 0 {
		return fmt.Errorf("无效的执行命令格式")
	}

	executable := parts[0]

	// 检查是否是绝对路径
	if !filepath.IsAbs(executable) {
		return fmt.Errorf("可执行文件必须使用绝对路径: %s", executable)
	}

	// 检查路径是否包含危险字符
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "[", "]"}
	for _, char := range dangerousChars {
		if strings.Contains(executable, char) {
			return fmt.Errorf("可执行文件路径包含危险字符 '%s'", char)
		}
	}

	// 基本路径长度检查
	if len(executable) > 4096 {
		return fmt.Errorf("可执行文件路径过长")
	}

	return nil
}

// validateServiceType 验证服务类型
func validateServiceType(serviceType server.ServiceType) error {
	validTypes := []server.ServiceType{
		server.ServiceTypeSimple,
		server.ServiceTypeForking,
		server.ServiceTypeOneshot,
		server.ServiceTypeNotify,
		server.ServiceTypeDbus,
	}

	for _, validType := range validTypes {
		if serviceType == validType {
			return nil
		}
	}

	return fmt.Errorf("无效的服务类型: %s", serviceType)
}

// validateUsername 验证用户名
func validateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名不能为空")
	}

	// 用户名长度限制
	if len(username) > 32 {
		return fmt.Errorf("用户名长度不能超过32个字符")
	}

	// Linux用户名规则
	usernamePattern := `^[a-z_][a-z0-9_-]*\$?$`
	matched, err := regexp.MatchString(usernamePattern, username)
	if err != nil {
		return fmt.Errorf("用户名正则验证失败: %w", err)
	}
	if !matched {
		return fmt.Errorf("用户名格式不正确，必须以字母或下划线开头，只能包含小写字母、数字、下划线和连字符")
	}

	return nil
}

// validateGroupname 验证组名
func validateGroupname(groupname string) error {
	groupname = strings.TrimSpace(groupname)
	if groupname == "" {
		return fmt.Errorf("组名不能为空")
	}

	// 组名长度限制
	if len(groupname) > 32 {
		return fmt.Errorf("组名长度不能超过32个字符")
	}

	// Linux组名规则（与用户名相同）
	groupnamePattern := `^[a-z_][a-z0-9_-]*\$?$`
	matched, err := regexp.MatchString(groupnamePattern, groupname)
	if err != nil {
		return fmt.Errorf("组名正则验证失败: %w", err)
	}
	if !matched {
		return fmt.Errorf("组名格式不正确，必须以字母或下划线开头，只能包含小写字母、数字、下划线和连字符")
	}

	return nil
}

// validateWorkingDirectory 验证工作目录
func validateWorkingDirectory(workingDir string) error {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir == "" {
		return fmt.Errorf("工作目录不能为空")
	}

	// 检查是否是绝对路径
	if !filepath.IsAbs(workingDir) {
		return fmt.Errorf("工作目录必须是绝对路径: %s", workingDir)
	}

	// 清理路径
	cleanPath := filepath.Clean(workingDir)
	if workingDir != cleanPath {
		return fmt.Errorf("工作目录路径格式不正确: %s", workingDir)
	}

	// 路径长度检查
	if len(workingDir) > 4096 {
		return fmt.Errorf("工作目录路径过长")
	}

	return nil
}

// validatePIDFilePath 验证PID文件路径
func validatePIDFilePath(pidFile string) error {
	pidFile = strings.TrimSpace(pidFile)
	if pidFile == "" {
		return fmt.Errorf("PID文件路径不能为空")
	}

	// 检查是否是绝对路径
	if !filepath.IsAbs(pidFile) {
		return fmt.Errorf("PID文件必须使用绝对路径: %s", pidFile)
	}

	// 检查文件扩展名
	if ext := filepath.Ext(pidFile); ext != ".pid" {
		return fmt.Errorf("PID文件必须以.pid结尾: %s", pidFile)
	}

	// 路径长度检查
	if len(pidFile) > 4096 {
		return fmt.Errorf("PID文件路径过长")
	}

	return nil
}

// validateRestartPolicy 验证重启策略
func validateRestartPolicy(restart string) error {
	validPolicies := []string{
		"no",
		"on-success",
		"on-failure",
		"on-abnormal",
		"on-watchdog",
		"on-abort",
		"always",
	}

	restart = strings.TrimSpace(restart)
	for _, policy := range validPolicies {
		if restart == policy {
			return nil
		}
	}

	return fmt.Errorf("无效的重启策略: %s，有效值为: %s", restart, strings.Join(validPolicies, ", "))
}

// validateEnvironmentVariables 验证环境变量
func validateEnvironmentVariables(env map[string]string) error {
	if env == nil {
		return nil
	}

	for key, value := range env {
		// 验证环境变量名
		if err := validateEnvironmentVariableName(key); err != nil {
			return fmt.Errorf("环境变量名 '%s' 验证失败: %w", key, err)
		}

		// 验证环境变量值
		if err := validateEnvironmentVariableValue(value); err != nil {
			return fmt.Errorf("环境变量值 '%s=%s' 验证失败: %w", key, value, err)
		}
	}

	return nil
}

// validateEnvironmentVariableName 验证环境变量名
func validateEnvironmentVariableName(name string) error {
	if name == "" {
		return fmt.Errorf("环境变量名不能为空")
	}

	// 环境变量名长度限制
	if len(name) > 255 {
		return fmt.Errorf("环境变量名长度不能超过255个字符")
	}

	// 环境变量名规则：以字母或下划线开头，后跟字母、数字或下划线
	envNamePattern := `^[a-zA-Z_][a-zA-Z0-9_]*$`
	matched, err := regexp.MatchString(envNamePattern, name)
	if err != nil {
		return fmt.Errorf("环境变量名正则验证失败: %w", err)
	}
	if !matched {
		return fmt.Errorf("环境变量名格式不正确，必须以字母或下划线开头，只能包含字母、数字和下划线")
	}

	return nil
}

// validateEnvironmentVariableValue 验证环境变量值
func validateEnvironmentVariableValue(value string) error {
	// 环境变量值长度限制
	if len(value) > 32768 {
		return fmt.Errorf("环境变量值长度不能超过32768个字符")
	}

	// 检查是否包含NULL字符
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("环境变量值不能包含NULL字符")
	}

	return nil
}
