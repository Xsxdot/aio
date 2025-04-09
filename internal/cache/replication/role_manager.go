package replication

import (
	"github.com/xsxdot/aio/pkg/common"
	"sync"
	"time"
)

// RoleManager 角色管理器接口
type RoleManager interface {
	// GetRole 获取当前角色
	GetRole() ReplicationRole
	// IsMaster 判断是否为主节点
	IsMaster() bool
	// RegisterListener 注册角色变更监听器
	RegisterListener(listener RoleChangeListener)
	// Start 启动角色管理
	Start() error
	// Stop 停止角色管理
	Stop() error
	// SetRole 手动设置角色
	SetRole(role ReplicationRole) error
}

// ElectionBasedRoleManager 基于选举的角色管理器
type ElectionBasedRoleManager struct {
	election  RoleClient
	role      ReplicationRole
	listeners []RoleChangeListener
	stopChan  chan struct{}
	logger    *common.Logger
	mutex     sync.RWMutex
}

// NewRoleManager 创建新的角色管理器
func NewRoleManager(election RoleClient) RoleManager {
	return &ElectionBasedRoleManager{
		election:  election,
		role:      RoleNone,
		listeners: make([]RoleChangeListener, 0),
		stopChan:  make(chan struct{}),
		logger:    common.GetLogger(),
	}
}

// GetRole 获取当前角色
func (rm *ElectionBasedRoleManager) GetRole() ReplicationRole {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.role
}

// IsMaster 判断是否为主节点
func (rm *ElectionBasedRoleManager) IsMaster() bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.role == RoleMaster
}

// RegisterListener 注册角色变更监听器
func (rm *ElectionBasedRoleManager) RegisterListener(listener RoleChangeListener) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.listeners = append(rm.listeners, listener)
}

// Start 启动角色管理
func (rm *ElectionBasedRoleManager) Start() error {
	// 启动选举客户端
	if err := rm.election.Start(); err != nil {
		return err
	}

	// 监听角色变更
	rm.election.WatchRoleChange(rm.handleRoleChange)

	// 进行初始角色检查
	rm.checkRole()

	// 启动定期检查
	go rm.periodicCheck()

	return nil
}

// Stop 停止角色管理
func (rm *ElectionBasedRoleManager) Stop() error {
	close(rm.stopChan)
	return rm.election.Stop()
}

// handleRoleChange 处理角色变更
func (rm *ElectionBasedRoleManager) handleRoleChange(isMaster bool) {
	rm.mutex.Lock()
	oldRole := rm.role
	newRole := RoleNone
	if isMaster {
		newRole = RoleMaster
	} else {
		newRole = RoleSlave
	}

	// 只有角色变更时才通知
	if oldRole != newRole {
		rm.role = newRole
		// 复制监听器列表，避免在锁外操作原始列表
		listeners := make([]RoleChangeListener, len(rm.listeners))
		copy(listeners, rm.listeners)
		rm.mutex.Unlock()

		// 记录角色变更
		rm.logger.Infof("节点角色从 %s 变更为 %s", oldRole, newRole)

		// 通知所有监听器
		for _, listener := range listeners {
			go listener.OnRoleChange(oldRole, newRole)
		}
	} else {
		rm.mutex.Unlock()
		rm.logger.Debugf("角色未变化，保持为: %s", oldRole)
	}
}

// checkRole 检查当前角色
func (rm *ElectionBasedRoleManager) checkRole() {
	isMaster := rm.election.IsMaster()

	// 将布尔值转换为角色类型
	newRole := RoleSlave
	if isMaster {
		newRole = RoleMaster
	}

	// 获取当前角色
	rm.mutex.RLock()
	oldRole := rm.role
	rm.mutex.RUnlock()

	// 只有角色真正变更时才通知，避免重复触发
	if oldRole != newRole {
		rm.logger.Infof("检测到节点角色变更: %s -> %s (通过定期检查)", oldRole, newRole)
		rm.handleRoleChange(isMaster)
	} else {
		rm.logger.Debugf("角色未变化，保持为: %s", oldRole)
	}
}

// periodicCheck 定期检查角色
func (rm *ElectionBasedRoleManager) periodicCheck() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.checkRole()
		case <-rm.stopChan:
			return
		}
	}
}

// SetRole 手动设置角色
func (rm *ElectionBasedRoleManager) SetRole(role ReplicationRole) error {
	rm.mutex.Lock()
	oldRole := rm.role

	// 如果角色没有变化，直接返回，避免重复通知
	if oldRole == role {
		rm.mutex.Unlock()
		rm.logger.Debugf("跳过相同角色的设置: %s", role)
		return nil
	}

	rm.role = role
	// 复制监听器列表，避免在锁外操作原始列表
	listeners := make([]RoleChangeListener, len(rm.listeners))
	copy(listeners, rm.listeners)
	rm.mutex.Unlock()

	// 记录角色变更
	rm.logger.Infof("节点角色从 %s 变更为 %s", oldRole, role)

	// 通知所有监听器
	for _, listener := range listeners {
		listener.OnRoleChange(oldRole, role)
	}

	return nil
}
