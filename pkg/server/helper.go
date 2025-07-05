package server

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Helper 命令执行辅助工具
type Helper struct {
	executor Executor
}

// NewHelper 创建命令执行辅助工具
func NewHelper(executor Executor) *Helper {
	return &Helper{
		executor: executor,
	}
}

// CommandBuilder 命令构建器
type CommandBuilder struct {
	command *Command
}

// BatchCommandBuilder 批量命令构建器
type BatchCommandBuilder struct {
	batchCommand *BatchCommand
}

// NewCommand 创建单个命令构建器
func NewCommand(name, command string) *CommandBuilder {
	return &CommandBuilder{
		command: &Command{
			ID:      uuid.New().String(),
			Name:    name,
			Command: command,
			Timeout: 5 * time.Minute, // 默认5分钟超时
		},
	}
}

// NewBatchCommand 创建批量命令构建器
func NewBatchCommand(name string) *BatchCommandBuilder {
	return &BatchCommandBuilder{
		batchCommand: &BatchCommand{
			ID:      uuid.New().String(),
			Name:    name,
			Mode:    BatchModeSequential, // 默认顺序执行
			Timeout: 30 * time.Minute,    // 默认30分钟超时
		},
	}
}

// CommandBuilder 方法
func (cb *CommandBuilder) WorkDir(dir string) *CommandBuilder {
	cb.command.WorkDir = dir
	return cb
}

func (cb *CommandBuilder) Environment(env map[string]string) *CommandBuilder {
	cb.command.Environment = env
	return cb
}

func (cb *CommandBuilder) Env(key, value string) *CommandBuilder {
	if cb.command.Environment == nil {
		cb.command.Environment = make(map[string]string)
	}
	cb.command.Environment[key] = value
	return cb
}

func (cb *CommandBuilder) Timeout(timeout time.Duration) *CommandBuilder {
	cb.command.Timeout = timeout
	return cb
}

func (cb *CommandBuilder) IgnoreError() *CommandBuilder {
	cb.command.IgnoreError = true
	return cb
}

func (cb *CommandBuilder) ContinueOnError() *CommandBuilder {
	cb.command.ContinueOnError = true
	return cb
}

func (cb *CommandBuilder) Condition(condition string) *CommandBuilder {
	cb.command.Condition = condition
	return cb
}

func (cb *CommandBuilder) Retry(times int, interval time.Duration) *CommandBuilder {
	cb.command.RetryTimes = times
	cb.command.RetryInterval = interval
	return cb
}

func (cb *CommandBuilder) Build() *Command {
	return cb.command
}

// BatchCommandBuilder 方法
func (bcb *BatchCommandBuilder) Mode(mode BatchMode) *BatchCommandBuilder {
	bcb.batchCommand.Mode = mode
	return bcb
}

func (bcb *BatchCommandBuilder) Parallel() *BatchCommandBuilder {
	bcb.batchCommand.Mode = BatchModeParallel
	return bcb
}

func (bcb *BatchCommandBuilder) Sequential() *BatchCommandBuilder {
	bcb.batchCommand.Mode = BatchModeSequential
	return bcb
}

func (bcb *BatchCommandBuilder) Timeout(timeout time.Duration) *BatchCommandBuilder {
	bcb.batchCommand.Timeout = timeout
	return bcb
}

func (bcb *BatchCommandBuilder) StopOnError() *BatchCommandBuilder {
	bcb.batchCommand.StopOnError = true
	return bcb
}

func (bcb *BatchCommandBuilder) ContinueOnFailed() *BatchCommandBuilder {
	bcb.batchCommand.ContinueOnFailed = true
	return bcb
}

func (bcb *BatchCommandBuilder) Try(commands ...*Command) *BatchCommandBuilder {
	bcb.batchCommand.TryCommands = commands
	return bcb
}

func (bcb *BatchCommandBuilder) Catch(commands ...*Command) *BatchCommandBuilder {
	bcb.batchCommand.CatchCommands = commands
	return bcb
}

func (bcb *BatchCommandBuilder) Finally(commands ...*Command) *BatchCommandBuilder {
	bcb.batchCommand.FinallyCommands = commands
	return bcb
}

func (bcb *BatchCommandBuilder) Build() *BatchCommand {
	return bcb.batchCommand
}

// Helper 便捷方法

// ExecuteCommand 执行单个命令
func (h *Helper) ExecuteCommand(ctx context.Context, serverID string, command *Command) (*CommandResult, error) {
	req := &ExecuteRequest{
		ServerID: serverID,
		Type:     CommandTypeSingle,
		Command:  command,
		SaveLog:  true,
	}

	result, err := h.executor.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.CommandResult, nil
}

// ExecuteBatchCommand 执行批量命令
func (h *Helper) ExecuteBatchCommand(ctx context.Context, serverID string, batchCommand *BatchCommand) (*BatchResult, error) {
	req := &ExecuteRequest{
		ServerID:     serverID,
		Type:         CommandTypeBatch,
		BatchCommand: batchCommand,
		SaveLog:      true,
	}

	result, err := h.executor.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.BatchResult, nil
}

// ExecuteSimple 执行简单命令（快捷方法）
func (h *Helper) ExecuteSimple(ctx context.Context, serverID, commandName, commandText string) (*CommandResult, error) {
	command := NewCommand(commandName, commandText).Build()
	return h.ExecuteCommand(ctx, serverID, command)
}

// ExecuteScript 执行脚本文件
func (h *Helper) ExecuteScript(ctx context.Context, serverID, scriptPath string, args ...string) (*CommandResult, error) {
	commandText := scriptPath
	if len(args) > 0 {
		for _, arg := range args {
			commandText += " " + arg
		}
	}

	command := NewCommand("执行脚本: "+scriptPath, commandText).
		Condition(fmt.Sprintf("test -f %s && test -x %s", scriptPath, scriptPath)).
		Build()

	return h.ExecuteCommand(ctx, serverID, command)
}

// ExecuteWithRetry 执行带重试的命令
func (h *Helper) ExecuteWithRetry(ctx context.Context, serverID, commandName, commandText string, retryTimes int, retryInterval time.Duration) (*CommandResult, error) {
	command := NewCommand(commandName, commandText).
		Retry(retryTimes, retryInterval).
		Build()

	return h.ExecuteCommand(ctx, serverID, command)
}

// CheckService 检查服务状态
func (h *Helper) CheckService(ctx context.Context, serverID, serviceName string) (*CommandResult, error) {
	command := NewCommand("检查服务: "+serviceName, fmt.Sprintf("systemctl status %s", serviceName)).
		IgnoreError().
		Build()

	return h.ExecuteCommand(ctx, serverID, command)
}

// StartService 启动服务
func (h *Helper) StartService(ctx context.Context, serverID, serviceName string) (*CommandResult, error) {
	command := NewCommand("启动服务: "+serviceName, fmt.Sprintf("systemctl start %s", serviceName)).
		Build()

	return h.ExecuteCommand(ctx, serverID, command)
}

// StopService 停止服务
func (h *Helper) StopService(ctx context.Context, serverID, serviceName string) (*CommandResult, error) {
	command := NewCommand("停止服务: "+serviceName, fmt.Sprintf("systemctl stop %s", serviceName)).
		Build()

	return h.ExecuteCommand(ctx, serverID, command)
}

// RestartService 重启服务
func (h *Helper) RestartService(ctx context.Context, serverID, serviceName string) (*CommandResult, error) {
	command := NewCommand("重启服务: "+serviceName, fmt.Sprintf("systemctl restart %s", serviceName)).
		Build()

	return h.ExecuteCommand(ctx, serverID, command)
}

// InstallPackage 安装软件包
func (h *Helper) InstallPackage(ctx context.Context, serverID, packageName string) (*BatchResult, error) {
	// 创建安装软件包的批量命令
	batch := NewBatchCommand("安装软件包: "+packageName).
		Sequential().
		StopOnError().
		Try(
			NewCommand("更新包列表", "apt-get update -y || yum update -y").IgnoreError().Build(),
			NewCommand("安装软件包", fmt.Sprintf("apt-get install -y %s || yum install -y %s", packageName, packageName)).Build(),
		).
		Catch(
			NewCommand("清理缓存", "apt-get clean || yum clean all").IgnoreError().Build(),
		).
		Finally(
			NewCommand("验证安装", fmt.Sprintf("which %s || rpm -qa | grep %s || dpkg -l | grep %s", packageName, packageName, packageName)).IgnoreError().Build(),
		).
		Build()

	return h.ExecuteBatchCommand(ctx, serverID, batch)
}

// DeployApplication 部署应用程序（示例）
func (h *Helper) DeployApplication(ctx context.Context, serverID, appName, appPath string) (*BatchResult, error) {
	batch := NewBatchCommand("部署应用: "+appName).
		Sequential().
		Try(
			NewCommand("创建备份", fmt.Sprintf("if [ -d %s ]; then cp -r %s %s.backup.$(date +%%Y%%m%%d%%H%%M%%S); fi", appPath, appPath, appPath)).IgnoreError().Build(),
			NewCommand("停止服务", fmt.Sprintf("systemctl stop %s", appName)).IgnoreError().Build(),
			NewCommand("部署文件", fmt.Sprintf("mkdir -p %s && cd %s", appPath, appPath)).Build(),
			NewCommand("设置权限", fmt.Sprintf("chown -R app:app %s", appPath)).Build(),
			NewCommand("启动服务", fmt.Sprintf("systemctl start %s", appName)).Build(),
			NewCommand("验证服务", fmt.Sprintf("systemctl is-active %s", appName)).Build(),
		).
		Catch(
			NewCommand("回滚部署", fmt.Sprintf("if [ -d %s.backup.* ]; then BACKUP=$(ls -t %s.backup.* | head -1); rm -rf %s; mv $BACKUP %s; fi", appPath, appPath, appPath, appPath)).IgnoreError().Build(),
			NewCommand("启动回滚服务", fmt.Sprintf("systemctl start %s", appName)).IgnoreError().Build(),
		).
		Finally(
			NewCommand("清理旧备份", fmt.Sprintf("find %s.backup.* -type d -mtime +7 -exec rm -rf {} \\; 2>/dev/null", appPath)).IgnoreError().Build(),
			NewCommand("检查最终状态", fmt.Sprintf("systemctl status %s", appName)).IgnoreError().Build(),
		).
		Build()

	return h.ExecuteBatchCommand(ctx, serverID, batch)
}

// GetSystemInfo 获取系统信息
func (h *Helper) GetSystemInfo(ctx context.Context, serverID string) (*BatchResult, error) {
	batch := NewBatchCommand("获取系统信息").
		Parallel().
		Try(
			NewCommand("系统版本", "cat /etc/os-release").Build(),
			NewCommand("内核版本", "uname -r").Build(),
			NewCommand("内存信息", "free -h").Build(),
			NewCommand("磁盘信息", "df -h").Build(),
			NewCommand("CPU信息", "lscpu").Build(),
			NewCommand("网络接口", "ip addr show").Build(),
			NewCommand("运行进程", "ps aux --sort=-%cpu | head -10").Build(),
			NewCommand("系统负载", "uptime").Build(),
		).
		Build()

	return h.ExecuteBatchCommand(ctx, serverID, batch)
}
