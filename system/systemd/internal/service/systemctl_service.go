package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/systemd/internal/model/dto"
)

const (
	// DefaultCommandTimeout 默认命令超时时间
	DefaultCommandTimeout = 30 * time.Second
)

// SystemctlService systemctl 命令服务
type SystemctlService struct {
	timeout time.Duration
	log     *logger.Log
	err     *errorc.ErrorBuilder
}

// NewSystemctlService 创建 systemctl 服务
func NewSystemctlService(timeout time.Duration, log *logger.Log) *SystemctlService {
	if timeout == 0 {
		timeout = DefaultCommandTimeout
	}
	return &SystemctlService{
		timeout: timeout,
		log:     log.WithEntryName("SystemctlService"),
		err:     errorc.NewErrorBuilder("SystemctlService"),
	}
}

// checkPlatform 检查平台是否为 Linux
func (s *SystemctlService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("systemctl 仅支持 Linux 平台，当前平台: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

// runCommand 执行命令并返回输出
func (s *SystemctlService) runCommand(ctx context.Context, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "systemctl", args...)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	s.log.WithFields(map[string]interface{}{
		"command": fmt.Sprintf("systemctl %s", strings.Join(args, " ")),
		"output":  outputStr,
	}).Debug("执行 systemctl 命令")

	if err != nil {
		// 检查是否是超时
		if cmdCtx.Err() == context.DeadlineExceeded {
			return outputStr, s.err.New("命令执行超时", err)
		}
		return outputStr, s.err.New(fmt.Sprintf("命令执行失败: %s", outputStr), err)
	}

	return outputStr, nil
}

// DaemonReload 执行 daemon-reload
func (s *SystemctlService) DaemonReload(ctx context.Context) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "daemon-reload")
	if err != nil {
		return s.err.New("daemon-reload 失败", err)
	}

	s.log.Info("daemon-reload 执行成功")
	return nil
}

// Start 启动服务
func (s *SystemctlService) Start(ctx context.Context, name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "start", name)
	if err != nil {
		return s.err.New(fmt.Sprintf("启动服务 %s 失败", name), err)
	}

	s.log.WithField("name", name).Info("服务启动成功")
	return nil
}

// Stop 停止服务
func (s *SystemctlService) Stop(ctx context.Context, name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "stop", name)
	if err != nil {
		return s.err.New(fmt.Sprintf("停止服务 %s 失败", name), err)
	}

	s.log.WithField("name", name).Info("服务停止成功")
	return nil
}

// Restart 重启服务
func (s *SystemctlService) Restart(ctx context.Context, name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "restart", name)
	if err != nil {
		return s.err.New(fmt.Sprintf("重启服务 %s 失败", name), err)
	}

	s.log.WithField("name", name).Info("服务重启成功")
	return nil
}

// Reload 重载服务配置
func (s *SystemctlService) Reload(ctx context.Context, name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "reload", name)
	if err != nil {
		return s.err.New(fmt.Sprintf("重载服务 %s 失败", name), err)
	}

	s.log.WithField("name", name).Info("服务重载成功")
	return nil
}

// Enable 启用服务（开机自启）
func (s *SystemctlService) Enable(ctx context.Context, name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "enable", name)
	if err != nil {
		return s.err.New(fmt.Sprintf("启用服务 %s 失败", name), err)
	}

	s.log.WithField("name", name).Info("服务启用成功")
	return nil
}

// Disable 禁用服务（取消开机自启）
func (s *SystemctlService) Disable(ctx context.Context, name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	_, err := s.runCommand(ctx, "disable", name)
	if err != nil {
		return s.err.New(fmt.Sprintf("禁用服务 %s 失败", name), err)
	}

	s.log.WithField("name", name).Info("服务禁用成功")
	return nil
}

// StopAndDisable 停止并禁用服务
func (s *SystemctlService) StopAndDisable(ctx context.Context, name string) error {
	// 先尝试停止，忽略错误（可能本来就没运行）
	s.Stop(ctx, name)

	// 禁用
	return s.Disable(ctx, name)
}

// Show 获取服务状态（结构化）
func (s *SystemctlService) Show(ctx context.Context, name string) (*dto.ServiceStatus, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	// 获取多个属性
	properties := []string{
		"Description",
		"LoadState",
		"ActiveState",
		"SubState",
		"UnitFileState",
		"MainPID",
		"ExecMainStartTimestamp",
		"MemoryCurrent",
		"Result",
	}

	output, err := s.runCommand(ctx, "show", name, "--no-page", "--property="+strings.Join(properties, ","))
	if err != nil {
		return nil, s.err.New(fmt.Sprintf("获取服务 %s 状态失败", name), err)
	}

	// 解析输出
	status := &dto.ServiceStatus{
		Name: name,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "Description":
			status.Description = value
		case "LoadState":
			status.LoadState = value
		case "ActiveState":
			status.ActiveState = value
		case "SubState":
			status.SubState = value
		case "UnitFileState":
			status.UnitFileState = value
		case "MainPID":
			if pid, err := strconv.Atoi(value); err == nil {
				status.MainPID = pid
			}
		case "ExecMainStartTimestamp":
			status.ExecMainStartAt = value
		case "MemoryCurrent":
			// 可能是 [not set] 或数字
			if value != "[not set]" {
				if mem, err := strconv.ParseUint(value, 10, 64); err == nil {
					status.MemoryCurrent = mem
				}
			}
		case "Result":
			status.Result = value
		}
	}

	return status, nil
}

// IsEnabled 检查服务是否启用
func (s *SystemctlService) IsEnabled(ctx context.Context, name string) (bool, error) {
	if err := s.checkPlatform(); err != nil {
		return false, err
	}

	output, _ := s.runCommand(ctx, "is-enabled", name)
	return output == "enabled", nil
}

// IsActive 检查服务是否活动
func (s *SystemctlService) IsActive(ctx context.Context, name string) (bool, error) {
	if err := s.checkPlatform(); err != nil {
		return false, err
	}

	output, _ := s.runCommand(ctx, "is-active", name)
	return output == "active", nil
}

// GetActiveState 获取活动状态
func (s *SystemctlService) GetActiveState(ctx context.Context, name string) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}

	output, err := s.runCommand(ctx, "show", name, "--no-page", "--property=ActiveState")
	if err != nil {
		return "", err
	}

	// 解析 ActiveState=xxx
	parts := strings.SplitN(output, "=", 2)
	if len(parts) == 2 {
		return parts[1], nil
	}
	return "", nil
}

// GetUnitFileState 获取 unit 文件状态
func (s *SystemctlService) GetUnitFileState(ctx context.Context, name string) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}

	output, err := s.runCommand(ctx, "show", name, "--no-page", "--property=UnitFileState")
	if err != nil {
		return "", err
	}

	// 解析 UnitFileState=xxx
	parts := strings.SplitN(output, "=", 2)
	if len(parts) == 2 {
		return parts[1], nil
	}
	return "", nil
}

// GetSubState 获取子状态
func (s *SystemctlService) GetSubState(ctx context.Context, name string) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}

	output, err := s.runCommand(ctx, "show", name, "--no-page", "--property=SubState")
	if err != nil {
		return "", err
	}

	// 解析 SubState=xxx
	parts := strings.SplitN(output, "=", 2)
	if len(parts) == 2 {
		return parts[1], nil
	}
	return "", nil
}

