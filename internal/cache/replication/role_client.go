package replication

// RoleClient 选举客户端接口
type RoleClient interface {
	// IsMaster 判断当前节点是否为主节点
	IsMaster() bool
	// WatchRoleChange 监听角色变更
	WatchRoleChange(callback func(bool))
	// Start 启动选举客户端
	Start() error
	// Stop 停止选举客户端
	Stop() error
}
