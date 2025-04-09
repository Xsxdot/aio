package server

import (
	"sync"
)

// ConnectionManager 管理缓存服务器的客户端连接
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]Connection
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]Connection),
	}
}

// AddConnection 添加新连接
func (m *ConnectionManager) AddConnection(conn Connection) {
	m.mu.Lock()
	m.connections[conn.ID()] = conn
	m.mu.Unlock()

}

// RemoveConnection 移除连接
func (m *ConnectionManager) RemoveConnection(id string) {
	m.mu.Lock()
	delete(m.connections, id)
	m.mu.Unlock()

}

// GetConnection 获取连接
func (m *ConnectionManager) GetConnection(id string) (Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, ok := m.connections[id]
	return conn, ok
}

// ConnectionCount 获取当前连接数
func (m *ConnectionManager) ConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// Connection 表示缓存客户端连接
type Connection interface {
	// ID 返回连接唯一标识
	ID() string
	// Close 关闭连接
	Close() error
	// RemoteAddr 返回远程地址
	RemoteAddr() string
}
