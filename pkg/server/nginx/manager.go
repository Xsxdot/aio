package nginx

import (
	"context"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/server"
)

// Manager nginx管理器实现
type Manager struct {
	service *Service
}

// NewManager 创建nginx管理器
func NewManager(etcdClient *etcd.EtcdClient, executor server.Executor) server.NginxServiceManager {
	storage := NewEtcdStorage(etcdClient)
	service := NewService(storage, executor)

	return &Manager{
		service: service,
	}
}

// AddNginxServer 添加nginx服务器
func (m *Manager) AddNginxServer(ctx context.Context, req *server.NginxServerCreateRequest) (*server.NginxServer, error) {
	return m.service.AddNginxServer(ctx, req)
}

// GetNginxServer 获取nginx服务器
func (m *Manager) GetNginxServer(ctx context.Context, serverID string) (*server.NginxServer, error) {
	return m.service.GetNginxServer(ctx, serverID)
}

// UpdateNginxServer 更新nginx服务器
func (m *Manager) UpdateNginxServer(ctx context.Context, serverID string, req *server.NginxServerUpdateRequest) (*server.NginxServer, error) {
	return m.service.UpdateNginxServer(ctx, serverID, req)
}

// DeleteNginxServer 删除nginx服务器
func (m *Manager) DeleteNginxServer(ctx context.Context, serverID string) error {
	return m.service.DeleteNginxServer(ctx, serverID)
}

// ListNginxServers 列出nginx服务器
func (m *Manager) ListNginxServers(ctx context.Context, req *server.NginxServerListRequest) ([]*server.NginxServer, int, error) {
	return m.service.ListNginxServers(ctx, req)
}

// ListConfigs 列出配置文件
func (m *Manager) ListConfigs(ctx context.Context, serverID string, req *server.NginxConfigListRequest) ([]*server.NginxConfig, int, error) {
	return m.service.ListConfigs(ctx, serverID, req)
}

// GetConfig 获取配置文件
func (m *Manager) GetConfig(ctx context.Context, serverID, configPath string) (*server.NginxConfig, error) {
	return m.service.GetConfig(ctx, serverID, configPath)
}

// CreateConfig 创建配置文件
func (m *Manager) CreateConfig(ctx context.Context, serverID string, req *server.NginxConfigCreateRequest) (*server.NginxConfig, error) {
	return m.service.CreateConfig(ctx, serverID, req)
}

// UpdateConfig 更新配置文件
func (m *Manager) UpdateConfig(ctx context.Context, serverID, configPath string, req *server.NginxConfigUpdateRequest) (*server.NginxConfig, error) {
	return m.service.UpdateConfig(ctx, serverID, configPath, req)
}

// DeleteConfig 删除配置文件
func (m *Manager) DeleteConfig(ctx context.Context, serverID, configPath string) (*server.NginxOperationResult, error) {
	return m.service.DeleteConfig(ctx, serverID, configPath)
}

// TestConfig 测试nginx配置
func (m *Manager) TestConfig(ctx context.Context, serverID string) (*server.NginxOperationResult, error) {
	return m.service.TestConfig(ctx, serverID)
}

// ReloadConfig 重载nginx配置
func (m *Manager) ReloadConfig(ctx context.Context, serverID string) (*server.NginxOperationResult, error) {
	return m.service.ReloadConfig(ctx, serverID)
}

// RestartNginx 重启nginx
func (m *Manager) RestartNginx(ctx context.Context, serverID string) (*server.NginxOperationResult, error) {
	return m.service.RestartNginx(ctx, serverID)
}

// GetNginxStatus 获取nginx状态
func (m *Manager) GetNginxStatus(ctx context.Context, serverID string) (*server.NginxStatusResult, error) {
	return m.service.GetNginxStatus(ctx, serverID)
}

// ListSites 列出站点
func (m *Manager) ListSites(ctx context.Context, serverID string, req *server.NginxSiteListRequest) ([]*server.NginxSite, int, error) {
	return m.service.ListSites(ctx, serverID, req)
}

// GetSite 获取站点
func (m *Manager) GetSite(ctx context.Context, serverID, siteName string) (*server.NginxSite, error) {
	return m.service.GetSite(ctx, serverID, siteName)
}

// CreateSite 创建站点
func (m *Manager) CreateSite(ctx context.Context, serverID string, req *server.NginxSiteCreateRequest) (*server.NginxSite, error) {
	return m.service.CreateSite(ctx, serverID, req)
}

// UpdateSite 更新站点
func (m *Manager) UpdateSite(ctx context.Context, serverID, siteName string, req *server.NginxSiteUpdateRequest) (*server.NginxSite, error) {
	return m.service.UpdateSite(ctx, serverID, siteName, req)
}

// DeleteSite 删除站点
func (m *Manager) DeleteSite(ctx context.Context, serverID, siteName string) (*server.NginxOperationResult, error) {
	return m.service.DeleteSite(ctx, serverID, siteName)
}

// EnableSite 启用站点
func (m *Manager) EnableSite(ctx context.Context, serverID, siteName string) (*server.NginxOperationResult, error) {
	return m.service.EnableSite(ctx, serverID, siteName)
}

// DisableSite 禁用站点
func (m *Manager) DisableSite(ctx context.Context, serverID, siteName string) (*server.NginxOperationResult, error) {
	return m.service.DisableSite(ctx, serverID, siteName)
}
