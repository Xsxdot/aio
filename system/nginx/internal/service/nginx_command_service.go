package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
)

const (
	// DefaultNginxValidateCommand 默认配置校验命令
	DefaultNginxValidateCommand = "nginx -t"
	// DefaultNginxReloadCommand 默认配置重载命令
	DefaultNginxReloadCommand = "nginx -s reload"
	// DefaultNginxCommandTimeout 默认命令超时时间
	DefaultNginxCommandTimeout = 30 * time.Second
)

// NginxCommandService nginx 命令服务
// 负责执行 nginx -t 校验和 reload 命令
type NginxCommandService struct {
	validateCommand string
	reloadCommand   string
	timeout         time.Duration
	log             *logger.Log
	err             *errorc.ErrorBuilder
}

// NewNginxCommandService 创建 nginx 命令服务
func NewNginxCommandService(validateCommand, reloadCommand string, timeout time.Duration, log *logger.Log) *NginxCommandService {
	if validateCommand == "" {
		validateCommand = DefaultNginxValidateCommand
	}
	if reloadCommand == "" {
		reloadCommand = DefaultNginxReloadCommand
	}
	if timeout == 0 {
		timeout = DefaultNginxCommandTimeout
	}

	return &NginxCommandService{
		validateCommand: validateCommand,
		reloadCommand:   reloadCommand,
		timeout:         timeout,
		log:             log.WithEntryName("NginxCommandService"),
		err:             errorc.NewErrorBuilder("NginxCommandService"),
	}
}

// checkPlatform 检查平台是否为 Linux
func (s *NginxCommandService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("nginx 命令仅支持 Linux 平台，当前平台: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

// runCommand 执行命令并返回输出
func (s *NginxCommandService) runCommand(ctx context.Context, command string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// 使用 sh -c 执行命令，支持 sudo 等场景
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	s.log.WithFields(map[string]interface{}{
		"command": command,
		"output":  outputStr,
	}).Debug("执行 nginx 命令")

	if err != nil {
		// 检查是否是超时
		if cmdCtx.Err() == context.DeadlineExceeded {
			return outputStr, s.err.New("命令执行超时", err)
		}
		return outputStr, s.err.New(fmt.Sprintf("命令执行失败: %s", outputStr), err)
	}

	return outputStr, nil
}

// Validate 执行配置校验命令（nginx -t）
func (s *NginxCommandService) Validate(ctx context.Context) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}

	output, err := s.runCommand(ctx, s.validateCommand)
	if err != nil {
		return output, s.err.New("nginx 配置校验失败", err)
	}

	s.log.Info("nginx 配置校验成功")
	return output, nil
}

// Reload 执行配置重载命令（nginx -s reload）
func (s *NginxCommandService) Reload(ctx context.Context) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}

	output, err := s.runCommand(ctx, s.reloadCommand)
	if err != nil {
		return output, s.err.New("nginx 配置重载失败", err)
	}

	s.log.Info("nginx 配置重载成功")
	return output, nil
}

// ValidateAndReload 校验并重载配置
// 先执行 validate，成功后再 reload
func (s *NginxCommandService) ValidateAndReload(ctx context.Context) (validateOutput, reloadOutput string, err error) {
	// 先校验
	validateOutput, err = s.Validate(ctx)
	if err != nil {
		return validateOutput, "", err
	}

	// 再重载
	reloadOutput, err = s.Reload(ctx)
	if err != nil {
		return validateOutput, reloadOutput, err
	}

	return validateOutput, reloadOutput, nil
}



