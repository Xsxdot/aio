package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/server/credential"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// ExecutorImpl 命令执行器实现
type ExecutorImpl struct {
	serverService     server.Service
	credentialService credential.Service
	storage           server.ExecutorStorage
	logger            *zap.Logger

	// 异步执行管理
	asyncResults sync.Map // requestID -> *ExecuteResult
	cancelFuncs  sync.Map // requestID -> context.CancelFunc
}

// Config 执行器配置
type Config struct {
	ServerService     server.Service
	CredentialService credential.Service
	Storage           server.ExecutorStorage
	Logger            *zap.Logger
}

// NewExecutor 创建命令执行器
func NewExecutor(config Config) server.Executor {
	if config.Logger == nil {
		config.Logger, _ = zap.NewProduction()
	}

	return &ExecutorImpl{
		serverService:     config.ServerService,
		credentialService: config.CredentialService,
		storage:           config.Storage,
		logger:            config.Logger,
	}
}

// Execute 执行命令
func (e *ExecutorImpl) Execute(ctx context.Context, req *server.ExecuteRequest) (*server.ExecuteResult, error) {
	if req.ServerID == "" {
		return nil, fmt.Errorf("服务器ID不能为空")
	}

	// 生成请求ID
	requestID := uuid.New().String()
	startTime := time.Now()

	result := &server.ExecuteResult{
		RequestID: requestID,
		Type:      req.Type,
		ServerID:  req.ServerID,
		Async:     false,
		StartTime: startTime,
	}

	// 获取服务器信息
	serverInfo, err := e.serverService.GetServer(ctx, req.ServerID)
	if err != nil {
		return nil, fmt.Errorf("获取服务器信息失败: %w", err)
	}

	// 建立SSH连接
	sshClient, err := e.createSSHClient(ctx, serverInfo)
	if err != nil {
		return nil, fmt.Errorf("建立SSH连接失败: %w", err)
	}
	defer sshClient.Close()

	// 根据命令类型执行
	switch req.Type {
	case server.CommandTypeSingle:
		if req.Command == nil {
			return nil, fmt.Errorf("单个命令不能为空")
		}
		commandResult, err := e.executeSingleCommand(ctx, sshClient, req.Command)
		if err != nil {
			return nil, err
		}
		result.CommandResult = commandResult

	case server.CommandTypeBatch:
		if req.BatchCommand == nil {
			return nil, fmt.Errorf("批量命令不能为空")
		}
		batchResult, err := e.executeBatchCommand(ctx, sshClient, req.BatchCommand)
		if err != nil {
			return nil, err
		}
		result.BatchResult = batchResult

	default:
		return nil, fmt.Errorf("不支持的命令类型: %s", req.Type)
	}

	result.EndTime = time.Now()

	// 保存执行记录
	if req.SaveLog && e.storage != nil {
		if err := e.storage.SaveExecuteResult(ctx, result); err != nil {
			e.logger.Warn("保存执行记录失败",
				zap.String("requestID", requestID),
				zap.Error(err))
		}
	}

	return result, nil
}

// ExecuteAsync 异步执行命令
func (e *ExecutorImpl) ExecuteAsync(ctx context.Context, req *server.ExecuteRequest) (string, error) {
	requestID := uuid.New().String()

	// 创建可取消的上下文
	asyncCtx, cancel := context.WithCancel(context.Background())
	e.cancelFuncs.Store(requestID, cancel)

	// 启动异步执行
	go func() {
		defer func() {
			e.cancelFuncs.Delete(requestID)
			cancel()
		}()

		// 复制请求并设置为异步
		asyncReq := *req
		asyncReq.Async = true

		result, err := e.Execute(asyncCtx, &asyncReq)
		if err != nil {
			// 创建错误结果
			result = &server.ExecuteResult{
				RequestID: requestID,
				Type:      req.Type,
				ServerID:  req.ServerID,
				Async:     true,
				StartTime: time.Now(),
				EndTime:   time.Now(),
			}

			if req.Type == server.CommandTypeSingle {
				result.CommandResult = &server.CommandResult{
					Status: server.CommandStatusFailed,
					Error:  err.Error(),
				}
			} else {
				result.BatchResult = &server.BatchResult{
					Status: server.CommandStatusFailed,
					Error:  err.Error(),
				}
			}
		}

		e.asyncResults.Store(requestID, result)
	}()

	return requestID, nil
}

// GetAsyncResult 获取异步执行结果
func (e *ExecutorImpl) GetAsyncResult(ctx context.Context, requestID string) (*server.ExecuteResult, error) {
	if requestID == "" {
		return nil, fmt.Errorf("请求ID不能为空")
	}

	// 先从内存中查找
	if result, ok := e.asyncResults.Load(requestID); ok {
		return result.(*server.ExecuteResult), nil
	}

	// 从存储中查找
	if e.storage != nil {
		return e.storage.GetExecuteResult(ctx, requestID)
	}

	return nil, fmt.Errorf("未找到请求ID为 %s 的执行结果", requestID)
}

// CancelExecution 取消执行
func (e *ExecutorImpl) CancelExecution(ctx context.Context, requestID string) error {
	if requestID == "" {
		return fmt.Errorf("请求ID不能为空")
	}

	if cancel, ok := e.cancelFuncs.Load(requestID); ok {
		cancel.(context.CancelFunc)()
		e.cancelFuncs.Delete(requestID)
		return nil
	}

	return fmt.Errorf("未找到请求ID为 %s 的执行任务", requestID)
}

// GetExecuteHistory 获取执行历史
func (e *ExecutorImpl) GetExecuteHistory(ctx context.Context, serverID string, limit int, offset int) ([]*server.ExecuteResult, int, error) {
	if e.storage == nil {
		return nil, 0, fmt.Errorf("存储服务不可用")
	}

	return e.storage.GetServerExecuteHistory(ctx, serverID, limit, offset)
}

// createSSHClient 创建SSH客户端
func (e *ExecutorImpl) createSSHClient(ctx context.Context, serverInfo *server.Server) (*ssh.Client, error) {
	// 获取认证信息
	credentialContent, credentialType, err := e.credentialService.GetCredentialContent(ctx, serverInfo.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("获取认证信息失败: %w", err)
	}

	var authMethods []ssh.AuthMethod

	// 根据认证类型配置认证方法
	switch credentialType {
	case credential.CredentialTypeSSHKey:
		signer, err := ssh.ParsePrivateKey([]byte(credentialContent))
		if err != nil {
			return nil, fmt.Errorf("解析SSH私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	case credential.CredentialTypePassword:
		authMethods = append(authMethods, ssh.Password(credentialContent))

	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", credentialType)
	}

	// 配置SSH客户端
	config := &ssh.ClientConfig{
		User:            serverInfo.GetUsername(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 连接SSH服务器
	addr := fmt.Sprintf("%s:%d", serverInfo.GetHost(), serverInfo.GetPort())
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}

	return client, nil
}

// executeSingleCommand 执行单个命令
func (e *ExecutorImpl) executeSingleCommand(ctx context.Context, client *ssh.Client, cmd *server.Command) (*server.CommandResult, error) {
	result := &server.CommandResult{
		CommandID:   cmd.ID,
		CommandName: cmd.Name,
		Command:     cmd.Command,
		Status:      server.CommandStatusPending,
		StartTime:   time.Now(),
	}

	// 检查执行条件
	if cmd.Condition != "" {
		conditionMet, err := e.checkCondition(ctx, client, cmd.Condition)
		if err != nil {
			result.Status = server.CommandStatusFailed
			result.Error = fmt.Sprintf("检查执行条件失败: %v", err)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, nil
		}

		if !conditionMet {
			result.Status = server.CommandStatusSuccess
			result.Stdout = "条件不满足，跳过执行"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, nil
		}
	}

	// 执行命令（带重试）
	for attempt := 0; attempt <= cmd.RetryTimes; attempt++ {
		if attempt > 0 {
			result.RetryCount = attempt
			if cmd.RetryInterval > 0 {
				time.Sleep(cmd.RetryInterval)
			}
		}

		err := e.runCommand(ctx, client, cmd, result)
		if err == nil && result.ExitCode == 0 {
			result.Status = server.CommandStatusSuccess
			break
		}

		if attempt == cmd.RetryTimes {
			if cmd.IgnoreError {
				result.Status = server.CommandStatusSuccess
			} else {
				result.Status = server.CommandStatusFailed
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// executeBatchCommand 执行批量命令
func (e *ExecutorImpl) executeBatchCommand(ctx context.Context, client *ssh.Client, batchCmd *server.BatchCommand) (*server.BatchResult, error) {
	result := &server.BatchResult{
		BatchID:   batchCmd.ID,
		BatchName: batchCmd.Name,
		Status:    server.CommandStatusRunning,
		StartTime: time.Now(),
	}

	// 执行Try阶段
	trySuccess := true
	if len(batchCmd.TryCommands) > 0 {
		e.logger.Info("开始执行Try阶段", zap.String("batchID", batchCmd.ID))
		tryResults, err := e.executeCommandList(ctx, client, batchCmd.TryCommands, batchCmd.Mode, batchCmd.StopOnError)
		if err != nil {
			result.Error = fmt.Sprintf("Try阶段执行失败: %v", err)
			trySuccess = false
		}
		result.TryResults = tryResults

		// 检查Try阶段是否有失败的命令
		for _, cmdResult := range tryResults {
			result.TotalCommands++
			if cmdResult.Status == server.CommandStatusSuccess {
				result.SuccessCommands++
			} else {
				result.FailedCommands++
				trySuccess = false
			}
		}
	}

	// 如果Try阶段失败，执行Catch阶段
	if !trySuccess && len(batchCmd.CatchCommands) > 0 {
		e.logger.Info("Try阶段失败，开始执行Catch阶段", zap.String("batchID", batchCmd.ID))
		catchResults, err := e.executeCommandList(ctx, client, batchCmd.CatchCommands, batchCmd.Mode, false)
		if err != nil {
			e.logger.Error("Catch阶段执行失败", zap.Error(err))
		}
		result.CatchResults = catchResults

		for _, cmdResult := range catchResults {
			result.TotalCommands++
			if cmdResult.Status == server.CommandStatusSuccess {
				result.SuccessCommands++
			} else {
				result.FailedCommands++
			}
		}
	}

	// 执行Finally阶段
	if len(batchCmd.FinallyCommands) > 0 {
		e.logger.Info("开始执行Finally阶段", zap.String("batchID", batchCmd.ID))
		finallyResults, err := e.executeCommandList(ctx, client, batchCmd.FinallyCommands, batchCmd.Mode, false)
		if err != nil {
			e.logger.Error("Finally阶段执行失败", zap.Error(err))
		}
		result.FinallyResults = finallyResults

		for _, cmdResult := range finallyResults {
			result.TotalCommands++
			if cmdResult.Status == server.CommandStatusSuccess {
				result.SuccessCommands++
			} else {
				result.FailedCommands++
			}
		}
	}

	// 确定整体状态
	if result.FailedCommands == 0 {
		result.Status = server.CommandStatusSuccess
	} else if trySuccess || batchCmd.ContinueOnFailed {
		result.Status = server.CommandStatusSuccess // 部分成功
	} else {
		result.Status = server.CommandStatusFailed
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// executeCommandList 执行命令列表
func (e *ExecutorImpl) executeCommandList(ctx context.Context, client *ssh.Client, commands []*server.Command, mode server.BatchMode, stopOnError bool) ([]*server.CommandResult, error) {
	results := make([]*server.CommandResult, 0, len(commands))

	if mode == server.BatchModeParallel {
		// 并行执行
		resultsChan := make(chan *server.CommandResult, len(commands))
		var wg sync.WaitGroup

		for _, cmd := range commands {
			wg.Add(1)
			go func(command *server.Command) {
				defer wg.Done()
				result, err := e.executeSingleCommand(ctx, client, command)
				if err != nil {
					result = &server.CommandResult{
						CommandID:   command.ID,
						CommandName: command.Name,
						Command:     command.Command,
						Status:      server.CommandStatusFailed,
						Error:       err.Error(),
						StartTime:   time.Now(),
						EndTime:     time.Now(),
					}
				}
				resultsChan <- result
			}(cmd)
		}

		go func() {
			wg.Wait()
			close(resultsChan)
		}()

		for result := range resultsChan {
			results = append(results, result)
		}

	} else {
		// 顺序执行
		for _, cmd := range commands {
			result, err := e.executeSingleCommand(ctx, client, cmd)
			if err != nil {
				result = &server.CommandResult{
					CommandID:   cmd.ID,
					CommandName: cmd.Name,
					Command:     cmd.Command,
					Status:      server.CommandStatusFailed,
					Error:       err.Error(),
					StartTime:   time.Now(),
					EndTime:     time.Now(),
				}
			}

			results = append(results, result)

			// 如果设置了遇错停止且当前命令失败
			if stopOnError && result.Status == server.CommandStatusFailed && !cmd.ContinueOnError {
				break
			}
		}
	}

	return results, nil
}

// checkCondition 检查执行条件
func (e *ExecutorImpl) checkCondition(ctx context.Context, client *ssh.Client, condition string) (bool, error) {
	session, err := client.NewSession()
	if err != nil {
		return false, err
	}
	defer session.Close()

	// 执行条件检查命令
	err = session.Run(condition)
	return err == nil, nil
}

// runCommand 运行单个命令
func (e *ExecutorImpl) runCommand(ctx context.Context, client *ssh.Client, cmd *server.Command, result *server.CommandResult) error {
	result.Status = server.CommandStatusRunning

	session, err := client.NewSession()
	if err != nil {
		result.Error = fmt.Sprintf("创建SSH会话失败: %v", err)
		return err
	}
	defer session.Close()

	// 设置工作目录和环境变量
	var command strings.Builder

	if cmd.WorkDir != "" {
		command.WriteString(fmt.Sprintf("cd %s && ", cmd.WorkDir))
	}

	if len(cmd.Environment) > 0 {
		for key, value := range cmd.Environment {
			command.WriteString(fmt.Sprintf("export %s=%s && ", key, value))
		}
	}

	command.WriteString(cmd.Command)

	// 创建管道获取输出
	stdout, err := session.StdoutPipe()
	if err != nil {
		result.Error = fmt.Sprintf("创建stdout管道失败: %v", err)
		return err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		result.Error = fmt.Sprintf("创建stderr管道失败: %v", err)
		return err
	}

	// 启动命令
	err = session.Start(command.String())
	if err != nil {
		result.Error = fmt.Sprintf("启动命令失败: %v", err)
		return err
	}

	// 读取输出
	stdoutData := make([]byte, 0)
	stderrData := make([]byte, 0)

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	// 设置超时
	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute // 默认5分钟超时
	}

	var waitErr error
	select {
	case waitErr = <-done:
		// 命令正常结束
	case <-time.After(timeout):
		// 超时
		session.Signal(ssh.SIGTERM)
		result.Status = server.CommandStatusTimeout
		result.Error = "命令执行超时"
		return fmt.Errorf("命令执行超时")
	case <-ctx.Done():
		// 上下文取消
		session.Signal(ssh.SIGTERM)
		result.Status = server.CommandStatusCancelled
		result.Error = "命令执行被取消"
		return ctx.Err()
	}

	// 读取所有输出
	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			stdoutData = append(stdoutData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			stderrData = append(stderrData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	result.Stdout = string(stdoutData)
	result.Stderr = string(stderrData)

	// 获取退出码
	if waitErr != nil {
		if exitError, ok := waitErr.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = waitErr.Error()
	} else {
		result.ExitCode = 0
	}

	return nil
}

// TestConnection 测试服务器连接
func (e *ExecutorImpl) TestConnection(ctx context.Context, req *server.TestConnectionRequest) (*server.TestConnectionResult, error) {
	if req.Host == "" {
		return nil, fmt.Errorf("主机地址不能为空")
	}
	if req.Username == "" {
		return nil, fmt.Errorf("用户名不能为空")
	}
	if req.CredentialID == "" {
		return nil, fmt.Errorf("密钥ID不能为空")
	}

	// 设置默认端口
	port := req.Port
	if port <= 0 {
		port = 22
	}

	// 获取密钥内容
	credentialContent, credentialType, err := e.credentialService.GetCredentialContent(ctx, req.CredentialID)
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "获取密钥失败",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()

	// 根据认证类型进行连接测试
	switch credentialType {
	case credential.CredentialTypeSSHKey:
		return e.testSSHKeyConnection(req.Host, port, req.Username, credentialContent)
	case credential.CredentialTypePassword:
		return e.testPasswordConnection(req.Host, port, req.Username, credentialContent)
	default:
		return &server.TestConnectionResult{
			Success: false,
			Message: "不支持的认证类型",
			Error:   "unsupported auth type",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
}

// testSSHKeyConnection 测试SSH密钥连接
func (e *ExecutorImpl) testSSHKeyConnection(host string, port int, username, privateKey string) (*server.TestConnectionResult, error) {
	start := time.Now()

	// 解析私钥
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "解析SSH私钥失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	// 配置SSH客户端
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 连接SSH服务器
	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "SSH连接失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer client.Close()

	// 执行简单命令测试
	session, err := client.NewSession()
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "创建SSH会话失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer session.Close()

	err = session.Run("echo 'test'")
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "执行测试命令失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	return &server.TestConnectionResult{
		Success: true,
		Message: "SSH连接测试成功",
		Latency: time.Since(start).Milliseconds(),
	}, nil
}

// testPasswordConnection 测试密码连接
func (e *ExecutorImpl) testPasswordConnection(host string, port int, username, password string) (*server.TestConnectionResult, error) {
	start := time.Now()

	// 配置SSH客户端
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 连接SSH服务器
	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "SSH连接失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer client.Close()

	// 执行简单命令测试
	session, err := client.NewSession()
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "创建SSH会话失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer session.Close()

	err = session.Run("echo 'test'")
	if err != nil {
		return &server.TestConnectionResult{
			Success: false,
			Message: "执行测试命令失败",
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	return &server.TestConnectionResult{
		Success: true,
		Message: "SSH连接测试成功",
		Latency: time.Since(start).Milliseconds(),
	}, nil
}

// CloneGitRepository 克隆Git仓库到指定目录
// 克隆完成后，cd到req.TargetDir即可进入项目根目录
func (e *ExecutorImpl) CloneGitRepository(ctx context.Context, req *server.GitCloneRequest) (*server.GitCloneResult, error) {
	if req.ServerID == "" {
		return nil, fmt.Errorf("服务器ID不能为空")
	}
	if req.RepoURL == "" {
		return nil, fmt.Errorf("仓库URL不能为空")
	}
	if req.TargetDir == "" {
		return nil, fmt.Errorf("目标目录不能为空")
	}

	// 生成请求ID
	requestID := uuid.New().String()
	startTime := time.Now()

	result := &server.GitCloneResult{
		RequestID: requestID,
		ServerID:  req.ServerID,
		RepoURL:   req.RepoURL,
		TargetDir: req.TargetDir,
		StartTime: startTime,
		Status:    server.CommandStatusRunning,
	}

	// 获取服务器信息
	serverInfo, err := e.serverService.GetServer(ctx, req.ServerID)
	if err != nil {
		result.Status = server.CommandStatusFailed
		result.Error = fmt.Sprintf("获取服务器信息失败: %v", err)
		result.EndTime = time.Now()
		return result, err
	}

	// 建立SSH连接
	sshClient, err := e.createSSHClient(ctx, serverInfo)
	if err != nil {
		result.Status = server.CommandStatusFailed
		result.Error = fmt.Sprintf("建立SSH连接失败: %v", err)
		result.EndTime = time.Now()
		return result, err
	}
	defer sshClient.Close()

	// 根据是否有Git认证信息选择不同的克隆方式
	if req.GitCredentialID != "" {
		err = e.cloneWithCredential(ctx, sshClient, req, result)
	} else {
		err = e.cloneDirectly(ctx, sshClient, req, result)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = server.CommandStatusFailed
		if result.Error == "" {
			result.Error = err.Error()
		}
		return result, err
	}

	result.Status = server.CommandStatusSuccess
	return result, nil
}

// cloneWithCredential 使用认证信息克隆Git仓库
func (e *ExecutorImpl) cloneWithCredential(ctx context.Context, client *ssh.Client, req *server.GitCloneRequest, result *server.GitCloneResult) error {
	// 获取Git认证信息
	gitCredContent, gitCredType, err := e.credentialService.GetCredentialContent(ctx, req.GitCredentialID)
	if err != nil {
		return fmt.Errorf("获取Git认证信息失败: %w", err)
	}

	// 根据认证类型选择不同的克隆方式
	switch gitCredType {
	case credential.CredentialTypeSSHKey:
		return e.cloneWithSSHKey(ctx, client, req, result, gitCredContent)
	case credential.CredentialTypePassword:
		return e.cloneWithPassword(ctx, client, req, result, gitCredContent)
	default:
		return fmt.Errorf("不支持的Git认证类型: %s", gitCredType)
	}
}

// cloneWithSSHKey 使用SSH密钥克隆Git仓库
func (e *ExecutorImpl) cloneWithSSHKey(ctx context.Context, client *ssh.Client, req *server.GitCloneRequest, result *server.GitCloneResult, keyContent string) error {
	// 1. 创建临时SSH密钥文件
	tempKeyFile := fmt.Sprintf("/tmp/git_key_%s", uuid.New().String()[:8])

	// 上传SSH密钥到服务器
	if err := e.uploadSSHKey(client, keyContent, tempKeyFile); err != nil {
		return fmt.Errorf("上传SSH密钥失败: %w", err)
	}

	// 确保在函数结束时清理临时文件
	defer func() {
		e.cleanupTempFile(client, tempKeyFile)
	}()

	// 2. 设置SSH配置以使用指定的密钥
	sshConfigContent := fmt.Sprintf(`
Host git-clone-host
    HostName %s
    User git
    IdentityFile %s
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
`, e.extractGitHost(req.RepoURL), tempKeyFile)

	tempSSHConfig := fmt.Sprintf("/tmp/ssh_config_%s", uuid.New().String()[:8])

	// 上传SSH配置文件
	if err := e.uploadTextFile(client, sshConfigContent, tempSSHConfig); err != nil {
		return fmt.Errorf("上传SSH配置失败: %w", err)
	}

	defer func() {
		e.cleanupTempFile(client, tempSSHConfig)
	}()

	// 3. 修改Git URL以使用自定义的SSH配置
	modifiedURL := e.modifyGitURLForSSHConfig(req.RepoURL)

	// 4. 构建Git克隆命令
	commands := []string{
		// 创建目标目录
		fmt.Sprintf("mkdir -p %s", req.TargetDir),
		// 进入目标目录并克隆仓库到当前目录
		fmt.Sprintf("cd %s && GIT_SSH_COMMAND='ssh -F %s' git clone %s .", req.TargetDir, tempSSHConfig, modifiedURL),
	}

	// 如果指定了分支
	if req.Branch != "" {
		commands[1] += fmt.Sprintf(" --branch %s", req.Branch)
	}

	// 如果需要浅克隆
	if req.Depth > 0 {
		commands[1] += fmt.Sprintf(" --depth %d", req.Depth)
	}

	// 执行命令序列
	for i, cmd := range commands {
		cmdResult := &server.CommandResult{
			CommandID:   fmt.Sprintf("git-clone-step-%d", i+1),
			CommandName: fmt.Sprintf("Git克隆步骤%d", i+1),
			Command:     cmd,
			StartTime:   time.Now(),
		}

		if err := e.runGitCommand(ctx, client, cmd, cmdResult, req.Timeout); err != nil {
			result.CommandResults = append(result.CommandResults, cmdResult)
			result.Error = fmt.Sprintf("执行命令失败: %s", err.Error())
			result.Stdout += cmdResult.Stdout
			result.Stderr += cmdResult.Stderr
			return err
		}

		result.CommandResults = append(result.CommandResults, cmdResult)
		result.Stdout += cmdResult.Stdout
		result.Stderr += cmdResult.Stderr

		// 如果命令失败
		if cmdResult.ExitCode != 0 {
			result.Error = fmt.Sprintf("命令执行失败，退出码: %d", cmdResult.ExitCode)
			return fmt.Errorf("命令执行失败: %s", cmdResult.Error)
		}
	}

	return nil
}

// cloneDirectly 直接克隆Git仓库（无SSH密钥）
func (e *ExecutorImpl) cloneDirectly(ctx context.Context, client *ssh.Client, req *server.GitCloneRequest, result *server.GitCloneResult) error {
	// 构建基本的Git克隆命令
	cmd := fmt.Sprintf("git clone %s .", req.RepoURL)

	// 添加可选参数
	if req.Branch != "" {
		cmd += fmt.Sprintf(" --branch %s", req.Branch)
	}
	if req.Depth > 0 {
		cmd += fmt.Sprintf(" --depth %d", req.Depth)
	}

	// 创建目标目录并进入目录执行克隆
	commands := []string{
		fmt.Sprintf("mkdir -p %s", req.TargetDir),
		fmt.Sprintf("cd %s && %s", req.TargetDir, cmd),
	}

	// 执行命令序列
	for i, command := range commands {
		cmdResult := &server.CommandResult{
			CommandID:   fmt.Sprintf("git-clone-step-%d", i+1),
			CommandName: fmt.Sprintf("Git克隆步骤%d", i+1),
			Command:     command,
			StartTime:   time.Now(),
		}

		if err := e.runGitCommand(ctx, client, command, cmdResult, req.Timeout); err != nil {
			result.CommandResults = append(result.CommandResults, cmdResult)
			result.Error = fmt.Sprintf("执行命令失败: %s", err.Error())
			result.Stdout += cmdResult.Stdout
			result.Stderr += cmdResult.Stderr
			return err
		}

		result.CommandResults = append(result.CommandResults, cmdResult)
		result.Stdout += cmdResult.Stdout
		result.Stderr += cmdResult.Stderr

		if cmdResult.ExitCode != 0 {
			result.Error = fmt.Sprintf("命令执行失败，退出码: %d", cmdResult.ExitCode)
			return fmt.Errorf("命令执行失败: %s", cmdResult.Error)
		}
	}

	return nil
}

// cloneWithPassword 使用用户名密码克隆Git仓库
func (e *ExecutorImpl) cloneWithPassword(ctx context.Context, client *ssh.Client, req *server.GitCloneRequest, result *server.GitCloneResult, password string) error {
	// 检查URL格式是否支持用户名密码认证
	if !strings.HasPrefix(req.RepoURL, "https://") && !strings.HasPrefix(req.RepoURL, "http://") {
		return fmt.Errorf("用户名密码认证只支持HTTPS/HTTP格式的Git URL")
	}

	// 从GitCloneRequest中获取用户名，如果没有设置则使用默认值
	username := req.Username
	if username == "" {
		// 如果是GitHub等Git服务，可以使用git作为默认用户名
		if strings.Contains(req.RepoURL, "github.com") {
			username = "git"
		} else {
			return fmt.Errorf("使用密码认证时必须指定用户名")
		}
	}

	// 构建包含认证信息的Git URL
	modifiedURL, err := e.buildAuthenticatedURL(req.RepoURL, username, password)
	if err != nil {
		return fmt.Errorf("构建认证URL失败: %w", err)
	}

	// 构建Git克隆命令
	cmd := fmt.Sprintf("git clone %s .", modifiedURL)

	// 添加可选参数
	if req.Branch != "" {
		cmd += fmt.Sprintf(" --branch %s", req.Branch)
	}
	if req.Depth > 0 {
		cmd += fmt.Sprintf(" --depth %d", req.Depth)
	}

	// 创建目标目录并进入目录执行克隆
	commands := []string{
		fmt.Sprintf("mkdir -p %s", req.TargetDir),
		fmt.Sprintf("cd %s && %s", req.TargetDir, cmd),
	}

	// 执行命令序列
	for i, command := range commands {
		cmdResult := &server.CommandResult{
			CommandID:   fmt.Sprintf("git-clone-step-%d", i+1),
			CommandName: fmt.Sprintf("Git密码克隆步骤%d", i+1),
			Command:     command,
			StartTime:   time.Now(),
		}

		// 为了安全，在日志中隐藏密码信息
		logCommand := command
		if strings.Contains(command, password) {
			logCommand = strings.ReplaceAll(command, password, "***")
		}
		cmdResult.Command = logCommand

		if err := e.runGitCommand(ctx, client, command, cmdResult, req.Timeout); err != nil {
			result.CommandResults = append(result.CommandResults, cmdResult)
			result.Error = fmt.Sprintf("执行命令失败: %s", err.Error())
			result.Stdout += cmdResult.Stdout
			result.Stderr += cmdResult.Stderr
			return err
		}

		result.CommandResults = append(result.CommandResults, cmdResult)
		result.Stdout += cmdResult.Stdout
		result.Stderr += cmdResult.Stderr

		if cmdResult.ExitCode != 0 {
			result.Error = fmt.Sprintf("命令执行失败，退出码: %d", cmdResult.ExitCode)
			return fmt.Errorf("命令执行失败: %s", cmdResult.Error)
		}
	}

	return nil
}

// buildAuthenticatedURL 构建包含认证信息的Git URL
func (e *ExecutorImpl) buildAuthenticatedURL(repoURL, username, password string) (string, error) {
	// 解析URL
	if strings.HasPrefix(repoURL, "https://") {
		// 移除协议前缀
		urlWithoutProtocol := strings.TrimPrefix(repoURL, "https://")
		// 构建包含认证信息的URL
		return fmt.Sprintf("https://%s:%s@%s", username, password, urlWithoutProtocol), nil
	} else if strings.HasPrefix(repoURL, "http://") {
		// 移除协议前缀
		urlWithoutProtocol := strings.TrimPrefix(repoURL, "http://")
		// 构建包含认证信息的URL
		return fmt.Sprintf("http://%s:%s@%s", username, password, urlWithoutProtocol), nil
	}

	return "", fmt.Errorf("不支持的URL格式: %s", repoURL)
}

// uploadSSHKey 上传SSH密钥到服务器
func (e *ExecutorImpl) uploadSSHKey(client *ssh.Client, keyContent, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %w", err)
	}
	defer session.Close()

	// 使用cat命令写入SSH密钥
	cmd := fmt.Sprintf("cat > %s && chmod 600 %s", remotePath, remotePath)

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建stdin管道失败: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("启动命令失败: %w", err)
	}

	// 写入密钥内容
	if _, err := stdin.Write([]byte(keyContent)); err != nil {
		return fmt.Errorf("写入密钥内容失败: %w", err)
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("上传SSH密钥失败: %w", err)
	}

	return nil
}

// uploadTextFile 上传文本文件到服务器
func (e *ExecutorImpl) uploadTextFile(client *ssh.Client, content, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("cat > %s", remotePath)

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建stdin管道失败: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("启动命令失败: %w", err)
	}

	if _, err := stdin.Write([]byte(content)); err != nil {
		return fmt.Errorf("写入文件内容失败: %w", err)
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	return nil
}

// cleanupTempFile 清理临时文件
func (e *ExecutorImpl) cleanupTempFile(client *ssh.Client, filePath string) {
	session, err := client.NewSession()
	if err != nil {
		e.logger.Warn("创建清理会话失败", zap.String("file", filePath), zap.Error(err))
		return
	}
	defer session.Close()

	cmd := fmt.Sprintf("rm -f %s", filePath)
	if err := session.Run(cmd); err != nil {
		e.logger.Warn("清理临时文件失败", zap.String("file", filePath), zap.Error(err))
	}
}

// extractGitHost 从Git URL中提取主机名
func (e *ExecutorImpl) extractGitHost(repoURL string) string {
	// 处理SSH格式: git@github.com:user/repo.git
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, "@")
		if len(parts) >= 2 {
			hostAndPath := parts[1]
			colonIndex := strings.Index(hostAndPath, ":")
			if colonIndex > 0 {
				return hostAndPath[:colonIndex]
			}
		}
	}

	// 处理HTTPS格式: https://github.com/user/repo.git
	if strings.HasPrefix(repoURL, "https://") {
		repoURL = strings.TrimPrefix(repoURL, "https://")
		slashIndex := strings.Index(repoURL, "/")
		if slashIndex > 0 {
			return repoURL[:slashIndex]
		}
	}

	// 默认返回github.com
	return "github.com"
}

// modifyGitURLForSSHConfig 修改Git URL以使用SSH配置
func (e *ExecutorImpl) modifyGitURLForSSHConfig(repoURL string) string {
	// 如果是SSH格式，替换主机名为配置中的别名
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, "@")
		if len(parts) >= 2 {
			hostAndPath := parts[1]
			colonIndex := strings.Index(hostAndPath, ":")
			if colonIndex > 0 {
				path := hostAndPath[colonIndex:]
				return "git@git-clone-host" + path
			}
		}
	}

	return repoURL
}

// runGitCommand 运行Git相关命令
func (e *ExecutorImpl) runGitCommand(ctx context.Context, client *ssh.Client, command string, result *server.CommandResult, timeout time.Duration) error {
	result.Status = server.CommandStatusRunning

	session, err := client.NewSession()
	if err != nil {
		result.Error = fmt.Sprintf("创建SSH会话失败: %v", err)
		result.Status = server.CommandStatusFailed
		return err
	}
	defer session.Close()

	// 创建管道获取输出
	stdout, err := session.StdoutPipe()
	if err != nil {
		result.Error = fmt.Sprintf("创建stdout管道失败: %v", err)
		result.Status = server.CommandStatusFailed
		return err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		result.Error = fmt.Sprintf("创建stderr管道失败: %v", err)
		result.Status = server.CommandStatusFailed
		return err
	}

	// 启动命令
	err = session.Start(command)
	if err != nil {
		result.Error = fmt.Sprintf("启动命令失败: %v", err)
		result.Status = server.CommandStatusFailed
		return err
	}

	// 读取输出
	stdoutData := make([]byte, 0)
	stderrData := make([]byte, 0)

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	// 设置超时
	if timeout <= 0 {
		timeout = 10 * time.Minute // 默认10分钟超时
	}

	var waitErr error
	select {
	case waitErr = <-done:
		// 命令正常结束
	case <-time.After(timeout):
		// 超时
		session.Signal(ssh.SIGTERM)
		result.Status = server.CommandStatusTimeout
		result.Error = "Git命令执行超时"
		return fmt.Errorf("Git命令执行超时")
	case <-ctx.Done():
		// 上下文取消
		session.Signal(ssh.SIGTERM)
		result.Status = server.CommandStatusCancelled
		result.Error = "Git命令执行被取消"
		return ctx.Err()
	}

	// 读取所有输出
	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			stdoutData = append(stdoutData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			stderrData = append(stderrData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	result.Stdout = string(stdoutData)
	result.Stderr = string(stderrData)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 获取退出码
	if waitErr != nil {
		if exitError, ok := waitErr.(*ssh.ExitError); ok {
			result.ExitCode = exitError.ExitStatus()
		} else {
			result.ExitCode = 1
		}
		result.Error = waitErr.Error()
		result.Status = server.CommandStatusFailed
	} else {
		result.ExitCode = 0
		result.Status = server.CommandStatusSuccess
	}

	return nil
}
