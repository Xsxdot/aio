package nginx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/server"
)

// Storage nginx存储接口
type Storage interface {
	// nginx服务器管理
	CreateNginxServer(ctx context.Context, nginxServer *server.NginxServer) (*server.NginxServer, error)
	GetNginxServer(ctx context.Context, serverID string) (*server.NginxServer, error)
	UpdateNginxServer(ctx context.Context, nginxServer *server.NginxServer) (*server.NginxServer, error)
	DeleteNginxServer(ctx context.Context, serverID string) error
	ListNginxServers(ctx context.Context, req *server.NginxServerListRequest) ([]*server.NginxServer, int, error)

	// 站点管理
	CreateSite(ctx context.Context, site *server.NginxSite) (*server.NginxSite, error)
	GetSite(ctx context.Context, serverID, siteName string) (*server.NginxSite, error)
	UpdateSite(ctx context.Context, site *server.NginxSite) (*server.NginxSite, error)
	DeleteSite(ctx context.Context, serverID, siteName string) error
	ListSites(ctx context.Context, serverID string, req *server.NginxSiteListRequest) ([]*server.NginxSite, int, error)
}

// EtcdStorage etcd存储实现
type EtcdStorage struct {
	client *etcd.EtcdClient
	prefix string
}

// NewEtcdStorage 创建etcd存储
func NewEtcdStorage(client *etcd.EtcdClient) Storage {
	return &EtcdStorage{
		client: client,
		prefix: "/aio/nginx",
	}
}

// CreateNginxServer 创建nginx服务器
func (s *EtcdStorage) CreateNginxServer(ctx context.Context, nginxServer *server.NginxServer) (*server.NginxServer, error) {
	key := fmt.Sprintf("%s/servers/%s", s.prefix, nginxServer.ServerID)

	data, err := json.Marshal(nginxServer)
	if err != nil {
		return nil, fmt.Errorf("序列化nginx服务器失败: %w", err)
	}

	err = s.client.Put(ctx, key, string(data))
	if err != nil {
		return nil, fmt.Errorf("保存nginx服务器失败: %w", err)
	}

	return nginxServer, nil
}

// GetNginxServer 获取nginx服务器
func (s *EtcdStorage) GetNginxServer(ctx context.Context, serverID string) (*server.NginxServer, error) {
	key := fmt.Sprintf("%s/servers/%s", s.prefix, serverID)

	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取nginx服务器失败: %w", err)
	}

	var nginxServer server.NginxServer
	if err := json.Unmarshal([]byte(resp), &nginxServer); err != nil {
		return nil, fmt.Errorf("反序列化nginx服务器失败: %w", err)
	}

	return &nginxServer, nil
}

// UpdateNginxServer 更新nginx服务器
func (s *EtcdStorage) UpdateNginxServer(ctx context.Context, nginxServer *server.NginxServer) (*server.NginxServer, error) {
	nginxServer.UpdatedAt = time.Now()
	return s.CreateNginxServer(ctx, nginxServer)
}

// DeleteNginxServer 删除nginx服务器
func (s *EtcdStorage) DeleteNginxServer(ctx context.Context, serverID string) error {
	// 删除服务器配置
	serverKey := fmt.Sprintf("%s/servers/%s", s.prefix, serverID)

	// 删除关联的站点
	sitesKey := fmt.Sprintf("%s/sites/%s/", s.prefix, serverID)

	err := s.client.Delete(ctx, serverKey)
	if err != nil {
		return fmt.Errorf("删除nginx服务器失败: %w", err)
	}

	err = s.client.DeleteWithPrefix(ctx, sitesKey)
	if err != nil {
		return fmt.Errorf("删除nginx站点失败: %w", err)
	}

	return nil
}

// ListNginxServers 列出nginx服务器
func (s *EtcdStorage) ListNginxServers(ctx context.Context, req *server.NginxServerListRequest) ([]*server.NginxServer, int, error) {
	key := fmt.Sprintf("%s/servers/", s.prefix)

	resp, err := s.client.GetWithPrefix(ctx, key)
	if err != nil {
		return nil, 0, fmt.Errorf("获取nginx服务器列表失败: %w", err)
	}

	servers := make([]*server.NginxServer, 0)

	for _, kv := range resp {
		var nginxServer server.NginxServer
		if err := json.Unmarshal([]byte(kv), &nginxServer); err != nil {
			continue // 跳过无效数据
		}

		// 状态过滤
		if req.Status != "" && nginxServer.Status != req.Status {
			continue
		}

		servers = append(servers, &nginxServer)
	}

	total := len(servers)

	// 分页处理
	if req.Limit > 0 {
		start := req.Offset
		end := start + req.Limit

		if start >= total {
			return []*server.NginxServer{}, total, nil
		}

		if end > total {
			end = total
		}

		servers = servers[start:end]
	}

	return servers, total, nil
}

// CreateSite 创建站点
func (s *EtcdStorage) CreateSite(ctx context.Context, site *server.NginxSite) (*server.NginxSite, error) {
	key := fmt.Sprintf("%s/sites/%s/%s", s.prefix, site.ServerID, site.Name)

	data, err := json.Marshal(site)
	if err != nil {
		return nil, fmt.Errorf("序列化站点失败: %w", err)
	}

	err = s.client.Put(ctx, key, string(data))
	if err != nil {
		return nil, fmt.Errorf("保存站点失败: %w", err)
	}

	return site, nil
}

// GetSite 获取站点
func (s *EtcdStorage) GetSite(ctx context.Context, serverID, siteName string) (*server.NginxSite, error) {
	key := fmt.Sprintf("%s/sites/%s/%s", s.prefix, serverID, siteName)

	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取站点失败: %w", err)
	}

	var site server.NginxSite
	if err := json.Unmarshal([]byte(resp), &site); err != nil {
		return nil, fmt.Errorf("反序列化站点失败: %w", err)
	}

	return &site, nil
}

// UpdateSite 更新站点
func (s *EtcdStorage) UpdateSite(ctx context.Context, site *server.NginxSite) (*server.NginxSite, error) {
	site.UpdatedAt = time.Now()
	return s.CreateSite(ctx, site)
}

// DeleteSite 删除站点
func (s *EtcdStorage) DeleteSite(ctx context.Context, serverID, siteName string) error {
	key := fmt.Sprintf("%s/sites/%s/%s", s.prefix, serverID, siteName)

	err := s.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("删除站点失败: %w", err)
	}

	return nil
}

// ListSites 列出站点
func (s *EtcdStorage) ListSites(ctx context.Context, serverID string, req *server.NginxSiteListRequest) ([]*server.NginxSite, int, error) {
	key := fmt.Sprintf("%s/sites/%s/", s.prefix, serverID)

	resp, err := s.client.GetWithPrefix(ctx, key)
	if err != nil {
		return nil, 0, fmt.Errorf("获取站点列表失败: %w", err)
	}

	sites := make([]*server.NginxSite, 0)

	for _, kv := range resp {
		var site server.NginxSite
		if err := json.Unmarshal([]byte(kv), &site); err != nil {
			continue // 跳过无效数据
		}

		// 过滤条件
		if req.Enabled != nil && site.Enabled != *req.Enabled {
			continue
		}

		if req.SSL != nil && site.SSL != *req.SSL {
			continue
		}

		if req.Pattern != "" && !strings.Contains(site.Name, req.Pattern) {
			continue
		}

		sites = append(sites, &site)
	}

	total := len(sites)

	// 分页处理
	if req.Limit > 0 {
		start := req.Offset
		end := start + req.Limit

		if start >= total {
			return []*server.NginxSite{}, total, nil
		}

		if end > total {
			end = total
		}

		sites = sites[start:end]
	}

	return sites, total, nil
}
