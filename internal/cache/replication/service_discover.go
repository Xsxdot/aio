package replication

// ServiceInfo 服务信息
type ServiceInfo struct {
	// ID 服务实例ID
	ID string
	// Host 服务地址
	Host string
	// Port 服务端口
	Port int
	// ProtocolPort 协议服务端口，用于复制通信
	ProtocolPort int
	// Role 服务角色
	Role ReplicationRole
	// MasterID 主节点ID（如果是从节点）
	MasterID string
	// NodeID 节点ID
	NodeID string
}

// ServiceDiscover 服务发现接口
type ServiceDiscover interface {
	// Register 注册服务
	Register(info ServiceInfo) error
	// Deregister 注销服务
	Deregister(serviceID string) error
	// FindMaster 查找主节点
	FindMaster() (ServiceInfo, error)
	// WatchMasterChange 监听主节点变更
	WatchMasterChange(handler func(ServiceInfo))
	// Close 关闭资源
	Close() error
}
