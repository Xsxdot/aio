package server

import (
	"context"
	"time"

	"github.com/xsxdot/aio/pkg/server/credential"
)

// MonitorAssignment 监控分配信息
type MonitorAssignment struct {
	ServerID     string    `json:"server_id"`
	ServerName   string    `json:"server_name"`
	AssignedNode string    `json:"assigned_node"`
	AssignTime   time.Time `json:"assign_time"`
}

// Service 服务器管理服务接口
type Service interface {
	GetCredentialService() credential.Service
	GetSystemdManager() SystemdServiceManager
	GetNginxManager() NginxServiceManager
	GetExecutor() Executor
	// 服务器管理
	CreateServer(ctx context.Context, req *ServerCreateRequest) (*Server, error)
	GetServer(ctx context.Context, id string) (*Server, error)
	UpdateServer(ctx context.Context, id string, req *ServerUpdateRequest) (*Server, error)
	DeleteServer(ctx context.Context, id string) error
	ListServers(ctx context.Context, req *ServerListRequest) ([]*Server, int, error)

	// 连接测试
	TestConnection(ctx context.Context, req *ServerTestConnectionRequest) (*ServerTestConnectionResult, error)

	// 健康检查
	PerformHealthCheck(ctx context.Context, serverID string) (*ServerHealthCheck, error)
	BatchHealthCheck(ctx context.Context, serverIDs []string) ([]*ServerHealthCheck, error)

	// 系统管理
	GetSystemInfo(ctx context.Context, serverID string) (*BatchResult, error)

	// 监控管理
	GetMonitorNodeIP(ctx context.Context, serverID string) (string, string, error)
	GetMonitorAssignment(ctx context.Context, serverID string) (*MonitorAssignment, error)
	ReassignMonitorNode(ctx context.Context, serverID, nodeID string) error
}
