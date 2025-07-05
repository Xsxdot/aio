// Package server 提供独立的服务器管理功能
package server

import (
	"context"
	"time"
)

// AuthType 认证类型
type AuthType string

const (
	AuthTypePassword   AuthType = "password"    // 密码认证
	AuthTypeSSHKey     AuthType = "ssh_key"     // SSH密钥认证
	AuthTypeSSHKeyFile AuthType = "ssh_keyfile" // SSH密钥文件认证
)

// ServerStatus 服务器状态
type ServerStatus string

const (
	ServerStatusOnline  ServerStatus = "online"  // 在线
	ServerStatusOffline ServerStatus = "offline" // 离线
	ServerStatusUnknown ServerStatus = "unknown" // 未知
)

// Server 服务器配置
type Server struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	InstallAIO   bool              `json:"installAIO"`   //是否安装了AIO
	InstallNginx bool              `json:"installNginx"` //是否安装了Nginx
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Username     string            `json:"username"`
	CredentialID string            `json:"credentialId"` // 密钥ID或密码ID
	Tags         map[string]string `json:"tags"`         // 服务器标签
	Status       ServerStatus      `json:"status"`       // 服务器状态
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ServerCreateRequest 创建服务器请求
type ServerCreateRequest struct {
	Name         string            `json:"name" validate:"required"`        // 服务器名称
	Host         string            `json:"host" validate:"required"`        // 主机地址
	Port         int               `json:"port" validate:"min=1,max=65535"` // 端口
	Username     string            `json:"username" validate:"required"`    // 用户名
	InstallAIO   bool              `json:"installAIO"`
	InstallNginx bool              `json:"installNginx"`
	CredentialID string            `json:"credentialId" validate:"required"` // 密钥ID
	Description  string            `json:"description"`                      // 描述
	Tags         map[string]string `json:"tags"`                             // 标签
}

// ServerUpdateRequest 更新服务器请求
type ServerUpdateRequest struct {
	Name         *string            `json:"name,omitempty"`         // 服务器名称
	Host         *string            `json:"host,omitempty"`         // 主机地址
	Port         *int               `json:"port,omitempty"`         // 端口
	Username     *string            `json:"username,omitempty"`     // 用户名
	InstallAIO   *bool              `json:"installAIO,omitempty"`   // 是否安装AIO
	InstallNginx *bool              `json:"installNginx,omitempty"` // 是否安装Nginx
	CredentialID *string            `json:"credentialId,omitempty"` // 密钥ID
	Description  *string            `json:"description,omitempty"`  // 描述
	Tags         *map[string]string `json:"tags,omitempty"`         // 标签
}

// ServerListRequest 服务器列表查询请求
type ServerListRequest struct {
	Limit  int               `json:"limit"`  // 分页大小
	Offset int               `json:"offset"` // 分页偏移
	Status string            `json:"status"` // 状态过滤
	Tags   map[string]string `json:"tags"`   // 标签过滤
}

// ServerTestConnectionRequest 测试服务器连接请求
type ServerTestConnectionRequest struct {
	Host         string `json:"host" validate:"required"`         // 主机地址
	Port         int    `json:"port" validate:"min=1,max=65535"`  // 端口
	Username     string `json:"username" validate:"required"`     // 用户名
	InstallAIO   bool   `json:"installAIO"`                       // 是否安装AIO
	CredentialID string `json:"credentialId" validate:"required"` // 密钥ID
}

// ServerTestConnectionResult 测试连接结果
type ServerTestConnectionResult struct {
	Success bool   `json:"success"`         // 是否成功
	Message string `json:"message"`         // 结果消息
	Latency int64  `json:"latency"`         // 延迟（毫秒）
	Error   string `json:"error,omitempty"` // 错误信息
}

// ServerHealthCheck 服务器健康检查结果
type ServerHealthCheck struct {
	ServerID  string       `json:"serverId"`
	Status    ServerStatus `json:"status"`
	Latency   int64        `json:"latency"`
	CPUUsage  float64      `json:"cpuUsage"`
	MemUsage  float64      `json:"memUsage"`
	DiskUsage float64      `json:"diskUsage"`
	CheckTime time.Time    `json:"checkTime"`
	Error     string       `json:"error,omitempty"`
}

// CredentialProvider 密钥提供者接口，用于解耦密钥管理依赖
type CredentialProvider interface {
	// GetCredentialContent 获取密钥内容
	GetCredentialContent(ctx context.Context, id string) (string, error)
	// TestCredential 测试密钥连接
	TestCredential(ctx context.Context, id string, host string, port int, username string) error
}

// GetID 获取服务器ID
func (s *Server) GetID() string {
	return s.ID
}

// GetHost 获取主机地址
func (s *Server) GetHost() string {
	return s.Host
}

// GetPort 获取端口
func (s *Server) GetPort() int {
	return s.Port
}

// GetUsername 获取用户名
func (s *Server) GetUsername() string {
	return s.Username
}

// GetCredentialID 获取凭证ID
func (s *Server) GetCredentialID() string {
	return s.CredentialID
}
