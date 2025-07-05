// Package server 服务器管理存储实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/server"
	"go.uber.org/zap"
)

const (
	// ETCD存储路径前缀
	serverPrefix = "/aio/servers/"
)

// ETCDStorage ETCD存储实现
type ETCDStorage struct {
	client *etcd.EtcdClient
	logger *zap.Logger
}

// ETCDStorageConfig ETCD存储配置
type ETCDStorageConfig struct {
	Client *etcd.EtcdClient
	Logger *zap.Logger
}

// NewETCDStorage 创建ETCD存储实例
func NewETCDStorage(config ETCDStorageConfig) Storage {
	if config.Logger == nil {
		config.Logger, _ = zap.NewProduction()
	}
	return &ETCDStorage{
		client: config.Client,
		logger: config.Logger,
	}
}

// CreateServer 创建服务器
func (s *ETCDStorage) CreateServer(ctx context.Context, server *server.Server) error {
	key := s.buildServerKey(server.ID)

	// 序列化服务器对象
	data, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("序列化服务器失败: %w", err)
	}

	// 保存到ETCD
	if err := s.client.Put(ctx, key, string(data)); err != nil {
		s.logger.Error("保存服务器到ETCD失败",
			zap.String("id", server.ID),
			zap.Error(err))
		return fmt.Errorf("保存服务器失败: %w", err)
	}

	s.logger.Info("服务器创建成功",
		zap.String("id", server.ID),
		zap.String("name", server.Name))

	return nil
}

// GetServer 获取服务器
func (s *ETCDStorage) GetServer(ctx context.Context, id string) (*server.Server, error) {
	key := s.buildServerKey(id)

	value, err := s.client.Get(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "键不存在") {
			return nil, fmt.Errorf("服务器不存在: %s", id)
		}
		return nil, fmt.Errorf("从ETCD获取服务器失败: %w", err)
	}

	var server server.Server
	if err := json.Unmarshal([]byte(value), &server); err != nil {
		return nil, fmt.Errorf("解析服务器数据失败: %w", err)
	}

	return &server, nil
}

// UpdateServer 更新服务器
func (s *ETCDStorage) UpdateServer(ctx context.Context, server *server.Server) error {
	key := s.buildServerKey(server.ID)

	// 序列化服务器对象
	data, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("序列化服务器失败: %w", err)
	}

	// 更新ETCD中的数据
	if err := s.client.Put(ctx, key, string(data)); err != nil {
		s.logger.Error("更新ETCD中的服务器失败",
			zap.String("id", server.ID),
			zap.Error(err))
		return fmt.Errorf("更新服务器失败: %w", err)
	}

	s.logger.Info("服务器更新成功",
		zap.String("id", server.ID),
		zap.String("name", server.Name))

	return nil
}

// DeleteServer 删除服务器
func (s *ETCDStorage) DeleteServer(ctx context.Context, id string) error {
	key := s.buildServerKey(id)

	if err := s.client.Delete(ctx, key); err != nil {
		s.logger.Error("从ETCD删除服务器失败",
			zap.String("id", id),
			zap.Error(err))
		return fmt.Errorf("删除服务器失败: %w", err)
	}

	s.logger.Info("服务器删除成功", zap.String("id", id))
	return nil
}

// ListServers 获取服务器列表
func (s *ETCDStorage) ListServers(ctx context.Context, req *server.ServerListRequest) ([]*server.Server, int, error) {
	// 获取所有服务器
	values, err := s.client.GetWithPrefix(ctx, serverPrefix)
	if err != nil {
		return nil, 0, fmt.Errorf("从ETCD获取服务器列表失败: %w", err)
	}

	var allServers []*server.Server
	for _, value := range values {
		var server server.Server
		if err := json.Unmarshal([]byte(value), &server); err != nil {
			s.logger.Warn("解析服务器数据失败", zap.Error(err))
			continue
		}

		// 状态过滤
		if req.Status != "" && string(server.Status) != req.Status {
			continue
		}

		// 标签过滤
		if len(req.Tags) > 0 {
			match := true
			for key, value := range req.Tags {
				if server.Tags[key] != value {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		allServers = append(allServers, &server)
	}

	total := len(allServers)

	// 分页处理
	start := req.Offset
	if start > total {
		start = total
	}

	end := start + req.Limit
	if end > total {
		end = total
	}

	result := allServers[start:end]
	return result, total, nil
}

// GetServersByIDs 根据ID列表获取服务器
func (s *ETCDStorage) GetServersByIDs(ctx context.Context, ids []string) ([]*server.Server, error) {
	var servers []*server.Server

	for _, id := range ids {
		server, err := s.GetServer(ctx, id)
		if err != nil {
			s.logger.Warn("获取服务器失败",
				zap.String("id", id),
				zap.Error(err))
			continue
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// GetServersByTags 根据标签获取服务器
func (s *ETCDStorage) GetServersByTags(ctx context.Context, tags []string) ([]*server.Server, error) {
	// 获取所有服务器
	values, err := s.client.GetWithPrefix(ctx, serverPrefix)
	if err != nil {
		return nil, fmt.Errorf("从ETCD获取服务器列表失败: %w", err)
	}

	var servers []*server.Server
	for _, value := range values {
		var server server.Server
		if err := json.Unmarshal([]byte(value), &server); err != nil {
			s.logger.Warn("解析服务器数据失败", zap.Error(err))
			continue
		}

		// 检查是否包含指定标签
		hasAllTags := true
		for _, tag := range tags {
			found := false
			for key := range server.Tags {
				if key == tag {
					found = true
					break
				}
			}
			if !found {
				hasAllTags = false
				break
			}
		}

		if hasAllTags {
			servers = append(servers, &server)
		}
	}

	return servers, nil
}

// UpdateServerStatus 更新服务器状态
func (s *ETCDStorage) UpdateServerStatus(ctx context.Context, id string, status server.ServerStatus) error {
	// 获取现有服务器
	server, err := s.GetServer(ctx, id)
	if err != nil {
		return err
	}

	// 更新状态
	server.Status = status
	server.UpdatedAt = time.Now()

	return s.UpdateServer(ctx, server)
}

// buildServerKey 构建服务器存储键
func (s *ETCDStorage) buildServerKey(id string) string {
	return fmt.Sprintf("%s%s", serverPrefix, id)
}
