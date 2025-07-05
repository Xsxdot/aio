package systemd

import (
	"github.com/xsxdot/aio/pkg/server"
)

// Manager systemd服务管理器
type Manager struct {
	serverService server.Service
	executor      server.Executor
}

// NewManager 创建systemd服务管理器
func NewManager(serverService server.Service, executor server.Executor) *Manager {
	return &Manager{
		serverService: serverService,
		executor:      executor,
	}
}
