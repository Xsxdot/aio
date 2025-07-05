// Package server 服务器管理服务
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/monitoring/collector"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/scheduler"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/server/credential"
	"github.com/xsxdot/aio/pkg/server/executor"
	"github.com/xsxdot/aio/pkg/server/nginx"
	"github.com/xsxdot/aio/pkg/server/systemd"
	"go.uber.org/zap"
)

// Storage 服务器存储接口
type Storage interface {
	// 服务器管理
	CreateServer(ctx context.Context, server *server.Server) error
	GetServer(ctx context.Context, id string) (*server.Server, error)
	UpdateServer(ctx context.Context, server *server.Server) error
	DeleteServer(ctx context.Context, id string) error
	ListServers(ctx context.Context, req *server.ServerListRequest) ([]*server.Server, int, error)

	// 服务器查询
	GetServersByIDs(ctx context.Context, ids []string) ([]*server.Server, error)
	GetServersByTags(ctx context.Context, tags []string) ([]*server.Server, error)
	UpdateServerStatus(ctx context.Context, id string, status server.ServerStatus) error
}

// ServiceImpl 服务器管理服务实现
type ServiceImpl struct {
	storage           Storage
	credentialService credential.Service
	monitorManager    *MonitorManager
	executor          server.Executor
	executorHelper    *server.Helper
	systemdManager    server.SystemdServiceManager
	logger            *zap.Logger
	scheduler         *scheduler.Scheduler
	collector         *collector.ServerCollector
	nginxManager      server.NginxServiceManager
}

// Config 服务配置
type Config struct {
	EtcdClient *etcd.EtcdClient
	Collector  *collector.ServerCollector
	Logger     *zap.Logger
	Registry   registry.Registry
	Scheduler  *scheduler.Scheduler
}

// NewService 创建服务器管理服务
func NewService(config Config) (server.Service, error) {
	if config.Logger == nil {
		config.Logger, _ = zap.NewProduction()
	}

	credentialService, err := credential.NewService(credential.Config{
		EtcdClient: config.EtcdClient,
		Logger:     config.Logger,
	})
	if err != nil {
		return nil, err
	}

	// 创建服务实例（循环依赖问题的解决）
	serviceImpl := &ServiceImpl{
		storage: NewETCDStorage(ETCDStorageConfig{
			Client: config.EtcdClient,
			Logger: config.Logger,
		}),
		credentialService: credentialService,
		logger:            config.Logger,
		scheduler:         config.Scheduler,
	}

	// 创建executor
	executorConfig := executor.Config{
		ServerService:     serviceImpl,
		CredentialService: serviceImpl.credentialService,
		Storage:           nil,
		Logger:            config.Logger,
	}
	serviceImpl.executor = executor.NewExecutor(executorConfig)
	serviceImpl.executorHelper = server.NewHelper(serviceImpl.executor)
	monitorManager := NewMonitorManager(serviceImpl, serviceImpl.executor, config.Registry, config.EtcdClient, config.Collector, config.Scheduler)
	serviceImpl.monitorManager = monitorManager
	serviceImpl.systemdManager = systemd.NewManager(serviceImpl, serviceImpl.executor)
	serviceImpl.nginxManager = nginx.NewManager(config.EtcdClient, serviceImpl.executor)
	return serviceImpl, nil
}

// serverServiceAdapter 适配器结构
type serverServiceAdapter struct {
	service *ServiceImpl
}

// GetCredentialService 获取凭证服务
func (s *ServiceImpl) GetCredentialService() credential.Service {
	return s.credentialService
}

// GetSystemdManager 获取systemd管理器
func (s *ServiceImpl) GetSystemdManager() server.SystemdServiceManager {
	return s.systemdManager
}

// GetNginxManager 获取nginx管理器
func (s *ServiceImpl) GetNginxManager() server.NginxServiceManager {
	return s.nginxManager
}

// GetExecutor 获取执行器
func (s *ServiceImpl) GetExecutor() server.Executor {
	return s.executor
}

// CreateServer 创建服务器
func (s *ServiceImpl) CreateServer(ctx context.Context, req *server.ServerCreateRequest) (*server.Server, error) {
	// 验证必填字段
	if req.Name == "" {
		return nil, fmt.Errorf("服务器名称不能为空")
	}
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
	if req.Port <= 0 {
		req.Port = 22
	}

	// 检查服务器名称是否重复
	listReq := &server.ServerListRequest{Limit: 100, Offset: 0}
	existingServers, _, err := s.storage.ListServers(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("检查服务器名称失败: %w", err)
	}

	for _, server := range existingServers {
		if server.Name == req.Name {
			return nil, fmt.Errorf("服务器名称 '%s' 已存在", req.Name)
		}
	}

	// 创建服务器对象
	now := time.Now()
	server := &server.Server{
		ID:           s.generateServerID(),
		Name:         req.Name,
		Host:         req.Host,
		Port:         req.Port,
		Username:     req.Username,
		InstallAIO:   req.InstallAIO,
		InstallNginx: req.InstallNginx,
		CredentialID: req.CredentialID,
		Description:  req.Description,
		Status:       server.ServerStatusUnknown,
		Tags:         req.Tags,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 保存到存储
	if err := s.storage.CreateServer(ctx, server); err != nil {
		s.logger.Error("创建服务器失败",
			zap.String("name", req.Name),
			zap.Error(err))
		return nil, fmt.Errorf("创建服务器失败: %w", err)
	}

	// 创建监控节点
	s.monitorManager.AddServer(server)

	s.logger.Info("创建服务器成功",
		zap.String("id", server.ID),
		zap.String("name", server.Name))

	return server, nil
}

// GetServer 获取服务器
func (s *ServiceImpl) GetServer(ctx context.Context, id string) (*server.Server, error) {
	if id == "" {
		return nil, fmt.Errorf("服务器ID不能为空")
	}

	server, err := s.storage.GetServer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取服务器失败: %w", err)
	}

	return server, nil
}

// UpdateServer 更新服务器
func (s *ServiceImpl) UpdateServer(ctx context.Context, id string, req *server.ServerUpdateRequest) (*server.Server, error) {
	if id == "" {
		return nil, fmt.Errorf("服务器ID不能为空")
	}

	// 获取现有服务器
	serverEntity, err := s.storage.GetServer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("服务器不存在: %w", err)
	}

	// 更新字段
	if req.Name != nil {
		// 检查名称是否重复
		listReq := &server.ServerListRequest{Limit: 100, Offset: 0}
		existingServers, _, err := s.storage.ListServers(ctx, listReq)
		if err != nil {
			return nil, fmt.Errorf("检查服务器名称失败: %w", err)
		}

		for _, existingServer := range existingServers {
			if existingServer.ID != id && existingServer.Name == *req.Name {
				return nil, fmt.Errorf("服务器名称 '%s' 已存在", *req.Name)
			}
		}
		serverEntity.Name = *req.Name
	}

	if req.Host != nil {
		serverEntity.Host = *req.Host
	}

	if req.Port != nil {
		serverEntity.Port = *req.Port
	}

	if req.Username != nil {
		serverEntity.Username = *req.Username
	}

	if req.InstallAIO != nil {
		serverEntity.InstallAIO = *req.InstallAIO
	}

	if req.InstallNginx != nil {
		serverEntity.InstallNginx = *req.InstallNginx
	}

	if req.CredentialID != nil {
		serverEntity.CredentialID = *req.CredentialID
	}

	if req.Description != nil {
		serverEntity.Description = *req.Description
	}

	if req.Tags != nil {
		serverEntity.Tags = *req.Tags
	}

	serverEntity.UpdatedAt = time.Now()

	// 保存更新
	if err := s.storage.UpdateServer(ctx, serverEntity); err != nil {
		s.logger.Error("更新服务器失败",
			zap.String("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("更新服务器失败: %w", err)
	}

	// 更新监控节点
	s.monitorManager.EditServer(serverEntity)

	s.logger.Info("更新服务器成功",
		zap.String("id", id),
		zap.String("name", serverEntity.Name))

	return serverEntity, nil
}

// DeleteServer 删除服务器
func (s *ServiceImpl) DeleteServer(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("服务器ID不能为空")
	}

	// 检查服务器是否存在
	server, err := s.storage.GetServer(ctx, id)
	if err != nil {
		return fmt.Errorf("服务器不存在: %w", err)
	}

	// 删除服务器
	if err := s.storage.DeleteServer(ctx, id); err != nil {
		s.logger.Error("删除服务器失败",
			zap.String("id", id),
			zap.Error(err))
		return fmt.Errorf("删除服务器失败: %w", err)
	}

	// 删除监控节点
	s.monitorManager.RemoveServer(server)

	s.logger.Info("删除服务器成功",
		zap.String("id", id),
		zap.String("name", server.Name))

	return nil
}

// ListServers 获取服务器列表
func (s *ServiceImpl) ListServers(ctx context.Context, req *server.ServerListRequest) ([]*server.Server, int, error) {
	// 设置默认分页
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	return s.storage.ListServers(ctx, req)
}

// TestConnection 测试服务器连接
func (s *ServiceImpl) TestConnection(ctx context.Context, req *server.ServerTestConnectionRequest) (*server.ServerTestConnectionResult, error) {
	// 转换为执行器的请求格式
	execReq := &server.TestConnectionRequest{
		Host:         req.Host,
		Port:         req.Port,
		Username:     req.Username,
		CredentialID: req.CredentialID,
	}

	// 调用执行器的TestConnection方法
	result, err := s.executor.TestConnection(ctx, execReq)
	if err != nil {
		return nil, err
	}

	// 转换为服务层的结果格式
	return &server.ServerTestConnectionResult{
		Success: result.Success,
		Message: result.Message,
		Latency: result.Latency,
		Error:   result.Error,
	}, nil
}

// PerformHealthCheck 执行服务器健康检查
func (s *ServiceImpl) PerformHealthCheck(ctx context.Context, serverID string) (*server.ServerHealthCheck, error) {
	// 获取服务器信息
	serverEntity, err := s.storage.GetServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("获取服务器信息失败: %w", err)
	}

	start := time.Now()
	healthCheck := &server.ServerHealthCheck{
		ServerID:  serverID,
		CheckTime: start,
		Status:    server.ServerStatusOnline,
	}

	// 测试连接
	testReq := &server.ServerTestConnectionRequest{
		Host:         serverEntity.Host,
		Port:         serverEntity.Port,
		Username:     serverEntity.Username,
		CredentialID: serverEntity.CredentialID,
	}

	result, err := s.TestConnection(ctx, testReq)
	if err != nil {
		healthCheck.Error = err.Error()
	} else if result.Success {
		healthCheck.Status = server.ServerStatusOnline
		healthCheck.Latency = result.Latency

	} else {
		healthCheck.Error = result.Error
	}

	// 更新服务器状态
	if err := s.storage.UpdateServerStatus(ctx, serverID, healthCheck.Status); err != nil {
		s.logger.Warn("更新服务器状态失败",
			zap.String("serverID", serverID),
			zap.Error(err))
	}

	return healthCheck, nil
}

// BatchHealthCheck 批量健康检查
func (s *ServiceImpl) BatchHealthCheck(ctx context.Context, serverIDs []string) ([]*server.ServerHealthCheck, error) {
	results := make([]*server.ServerHealthCheck, 0, len(serverIDs))

	for _, serverID := range serverIDs {
		healthCheck, err := s.PerformHealthCheck(ctx, serverID)
		if err != nil {
			s.logger.Error("服务器健康检查失败",
				zap.String("serverID", serverID),
				zap.Error(err))
			continue
		}
		results = append(results, healthCheck)
	}

	return results, nil
}

// generateServerID 生成服务器ID
func (s *ServiceImpl) generateServerID() string {
	// 使用UUID生成唯一ID，格式为 server- + uuid前8位
	serverUUID := uuid.New()
	shortUUID := fmt.Sprintf("%x", serverUUID[:4])
	return fmt.Sprintf("server-%s", shortUUID)
}

// getDefaultInstallPath 获取默认安装路径
func (s *ServiceImpl) getDefaultInstallPath(username string) string {
	if username == "root" {
		return "/opt/aio"
	}
	return fmt.Sprintf("/home/%s/aio", username)
}

// 命令执行相关方法

// ExecuteCommand 执行单个命令
func (s *ServiceImpl) ExecuteCommand(ctx context.Context, serverID string, command *server.Command) (*server.CommandResult, error) {
	return s.executorHelper.ExecuteCommand(ctx, serverID, command)
}

// ExecuteBatchCommand 执行批量命令
func (s *ServiceImpl) ExecuteBatchCommand(ctx context.Context, serverID string, batchCommand *server.BatchCommand) (*server.BatchResult, error) {
	return s.executorHelper.ExecuteBatchCommand(ctx, serverID, batchCommand)
}

// ExecuteSimpleCommand 执行简单命令
func (s *ServiceImpl) ExecuteSimpleCommand(ctx context.Context, serverID, commandName, commandText string) (*server.CommandResult, error) {
	return s.executorHelper.ExecuteSimple(ctx, serverID, commandName, commandText)
}

// ExecuteAsync 异步执行命令
func (s *ServiceImpl) ExecuteAsync(ctx context.Context, req *server.ExecuteRequest) (string, error) {
	return s.executor.ExecuteAsync(ctx, req)
}

// GetAsyncResult 获取异步执行结果
func (s *ServiceImpl) GetAsyncResult(ctx context.Context, requestID string) (*server.ExecuteResult, error) {
	return s.executor.GetAsyncResult(ctx, requestID)
}

// CancelExecution 取消执行
func (s *ServiceImpl) CancelExecution(ctx context.Context, requestID string) error {
	return s.executor.CancelExecution(ctx, requestID)
}

// GetExecuteHistory 获取执行历史
func (s *ServiceImpl) GetExecuteHistory(ctx context.Context, serverID string, limit int, offset int) ([]*server.ExecuteResult, int, error) {
	return s.executor.GetExecuteHistory(ctx, serverID, limit, offset)
}

// 服务管理方法

// CheckService 检查服务状态
func (s *ServiceImpl) CheckService(ctx context.Context, serverID, serviceName string) (*server.CommandResult, error) {
	return s.executorHelper.CheckService(ctx, serverID, serviceName)
}

// StartService 启动服务
func (s *ServiceImpl) StartService(ctx context.Context, serverID, serviceName string) (*server.CommandResult, error) {
	return s.executorHelper.StartService(ctx, serverID, serviceName)
}

// StopService 停止服务
func (s *ServiceImpl) StopService(ctx context.Context, serverID, serviceName string) (*server.CommandResult, error) {
	return s.executorHelper.StopService(ctx, serverID, serviceName)
}

// RestartService 重启服务
func (s *ServiceImpl) RestartService(ctx context.Context, serverID, serviceName string) (*server.CommandResult, error) {
	return s.executorHelper.RestartService(ctx, serverID, serviceName)
}

// 系统管理方法

// GetSystemInfo 获取系统信息
func (s *ServiceImpl) GetSystemInfo(ctx context.Context, serverID string) (*server.BatchResult, error) {
	return s.executorHelper.GetSystemInfo(ctx, serverID)
}

// InstallPackage 安装软件包
func (s *ServiceImpl) InstallPackage(ctx context.Context, serverID, packageName string) (*server.BatchResult, error) {
	return s.executorHelper.InstallPackage(ctx, serverID, packageName)
}

// GetMonitorNodeIP 获取监控节点IP
func (s *ServiceImpl) GetMonitorNodeIP(ctx context.Context, serverID string) (string, string, error) {
	return s.monitorManager.GetMonitorNodeIP(serverID)
}

// GetMonitorAssignment 获取监控分配信息
func (s *ServiceImpl) GetMonitorAssignment(ctx context.Context, serverID string) (*server.MonitorAssignment, error) {
	assignment, err := s.monitorManager.GetMonitorAssignment(serverID)
	if err != nil {
		return nil, err
	}

	// 类型转换
	result := &server.MonitorAssignment{
		ServerID:     assignment.ServerID,
		ServerName:   assignment.ServerName,
		AssignedNode: assignment.AssignedNode,
		AssignTime:   assignment.AssignTime,
	}

	return result, nil
}

// ReassignMonitorNode 重新分配监控节点
func (s *ServiceImpl) ReassignMonitorNode(ctx context.Context, serverID, nodeID string) error {
	return s.monitorManager.ReassignMonitorNode(serverID, nodeID)
}
