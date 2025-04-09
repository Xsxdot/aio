package manager

import (
	"context"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/distributed/idgen"
	"github.com/xsxdot/aio/pkg/distributed/lock"
	"github.com/xsxdot/aio/pkg/distributed/state"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// ManagerStatus 管理器状态类型
type ManagerStatus string

const (
	// StatusStopped 已停止状态
	StatusStopped ManagerStatus = "stopped"
	// StatusRunning 运行中状态
	StatusRunning ManagerStatus = "running"
	// StatusStarting 启动中状态
	StatusStarting ManagerStatus = "starting"
	// StatusStopping 停止中状态
	StatusStopping ManagerStatus = "stopping"
	// StatusError 错误状态
	StatusError ManagerStatus = "error"
)

// DistributedManager 分布式管理器接口
type DistributedManager interface {
	// 生命周期方法
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error

	// 组件服务访问
	Election() election.ElectionService
	Lock() lock.LockService
	IDGenerator() idgen.IDGeneratorService
	StateManager() state.StateManagerService

	// 状态管理
	GetStatus() ManagerStatus
	IsHealthy() bool
}

// Manager 分布式管理器实现
type Manager struct {
	etcdClient *clientv3.Client
	logger     *zap.Logger
	status     ManagerStatus

	electionService election.ElectionService
	lockService     lock.LockService
	idgenService    idgen.IDGeneratorService
	stateService    state.StateManagerService
}

// ManagerOption 管理器配置选项函数类型
type ManagerOption func(*Manager)

// WithLogger 设置日志记录器选项
func WithLogger(logger *zap.Logger) ManagerOption {
	return func(m *Manager) {
		m.logger = logger
	}
}

// NewManager 创建新的分布式管理器实例
func NewManager(etcdClient *clientv3.Client, options ...ManagerOption) DistributedManager {
	manager := &Manager{
		etcdClient: etcdClient,
		logger:     zap.NewNop(),
		status:     StatusStopped,
	}

	// 应用选项
	for _, option := range options {
		option(manager)
	}

	return manager
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.status = StatusStarting
	m.logger.Info("Starting distributed manager")

	// 初始化各服务组件
	var err error

	// 初始化选举服务
	m.electionService, err = election.NewElectionService(m.etcdClient, m.logger)
	if err != nil {
		m.status = StatusError
		return err
	}

	// 初始化锁服务
	m.lockService, err = lock.NewLockService(m.etcdClient, m.logger)
	if err != nil {
		m.status = StatusError
		return err
	}

	// 初始化ID生成器服务
	m.idgenService, err = idgen.NewIDGeneratorService(m.etcdClient, m.logger)
	if err != nil {
		m.status = StatusError
		return err
	}

	// 初始化状态管理服务
	m.stateService, err = state.NewStateManagerService(m.etcdClient, m.logger)
	if err != nil {
		m.status = StatusError
		return err
	}

	// 启动各服务组件
	if err := m.electionService.Start(ctx); err != nil {
		m.status = StatusError
		return err
	}

	if err := m.lockService.Start(ctx); err != nil {
		m.status = StatusError
		return err
	}

	if err := m.idgenService.Start(ctx); err != nil {
		m.status = StatusError
		return err
	}

	if err := m.stateService.Start(ctx); err != nil {
		m.status = StatusError
		return err
	}

	m.status = StatusRunning
	m.logger.Info("Distributed manager started successfully")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop(ctx context.Context) error {
	m.status = StatusStopping
	m.logger.Info("Stopping distributed manager")

	// 按相反顺序停止各服务组件
	if err := m.stateService.Stop(ctx); err != nil {
		m.logger.Error("Failed to stop state manager service", zap.Error(err))
	}

	if err := m.idgenService.Stop(ctx); err != nil {
		m.logger.Error("Failed to stop ID generator service", zap.Error(err))
	}

	if err := m.lockService.Stop(ctx); err != nil {
		m.logger.Error("Failed to stop lock service", zap.Error(err))
	}

	if err := m.electionService.Stop(ctx); err != nil {
		m.logger.Error("Failed to stop election service", zap.Error(err))
	}

	m.status = StatusStopped
	m.logger.Info("Distributed manager stopped successfully")
	return nil
}

// Restart 重启管理器
func (m *Manager) Restart(ctx context.Context) error {
	m.logger.Info("Restarting distributed manager")

	if err := m.Stop(ctx); err != nil {
		return err
	}

	if err := m.Start(ctx); err != nil {
		return err
	}

	return nil
}

// Election 获取选举服务
func (m *Manager) Election() election.ElectionService {
	return m.electionService
}

// Lock 获取锁服务
func (m *Manager) Lock() lock.LockService {
	return m.lockService
}

// IDGenerator 获取ID生成器服务
func (m *Manager) IDGenerator() idgen.IDGeneratorService {
	return m.idgenService
}

// StateManager 获取状态管理服务
func (m *Manager) StateManager() state.StateManagerService {
	return m.stateService
}

// GetStatus 获取管理器状态
func (m *Manager) GetStatus() ManagerStatus {
	return m.status
}

// IsHealthy 检查管理器是否健康
func (m *Manager) IsHealthy() bool {
	return m.status == StatusRunning
}
