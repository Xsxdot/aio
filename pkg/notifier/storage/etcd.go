// Package storage 提供通知器配置存储实现
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/notifier"
)

// EtcdStorage 基于ETCD的通知器配置存储
type EtcdStorage struct {
	client *etcd.EtcdClient
	prefix string
	logger *zap.Logger
}

// EtcdStorageConfig ETCD存储配置
type EtcdStorageConfig struct {
	// ETCD客户端
	Client *etcd.EtcdClient
	// 存储前缀
	Prefix string
	// 日志器
	Logger *zap.Logger
}

// NewEtcdStorage 创建新的ETCD存储实例
func NewEtcdStorage(config EtcdStorageConfig) (notifier.Storage, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("ETCD客户端不能为空")
	}

	if config.Logger == nil {
		logger, _ := zap.NewProduction()
		config.Logger = logger
	}

	if config.Prefix == "" {
		config.Prefix = "/aio/notifiers"
	}

	return &EtcdStorage{
		client: config.Client,
		prefix: config.Prefix,
		logger: config.Logger,
	}, nil
}

// GetNotifier 获取单个通知器配置
func (s *EtcdStorage) GetNotifier(ctx context.Context, id string) (*notifier.NotifierConfig, error) {
	key := s.buildKey(id)
	value, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("从ETCD获取通知器配置失败: %w", err)
	}

	if value == "" {
		return nil, fmt.Errorf("通知器不存在: %s", id)
	}

	var config notifier.NotifierConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, fmt.Errorf("解析通知器配置失败: %w", err)
	}

	return &config, nil
}

// GetNotifiers 获取所有通知器配置
func (s *EtcdStorage) GetNotifiers(ctx context.Context) ([]*notifier.NotifierConfig, error) {
	kvs, err := s.client.GetWithPrefix(ctx, s.prefix)
	if err != nil {
		return nil, fmt.Errorf("从ETCD获取通知器配置列表失败: %w", err)
	}

	configs := make([]*notifier.NotifierConfig, 0, len(kvs))
	for key, value := range kvs {
		var config notifier.NotifierConfig
		if err := json.Unmarshal([]byte(value), &config); err != nil {
			s.logger.Error("解析通知器配置失败",
				zap.String("key", key),
				zap.Error(err))
			continue
		}
		configs = append(configs, &config)
	}

	return configs, nil
}

// SaveNotifier 保存通知器配置
func (s *EtcdStorage) SaveNotifier(ctx context.Context, config *notifier.NotifierConfig) error {
	if config == nil {
		return fmt.Errorf("通知器配置不能为空")
	}

	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化通知器配置失败: %w", err)
	}

	key := s.buildKey(config.ID)
	if err := s.client.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("保存通知器配置到ETCD失败: %w", err)
	}

	s.logger.Debug("保存通知器配置",
		zap.String("id", config.ID),
		zap.String("name", config.Name))

	return nil
}

// DeleteNotifier 删除通知器配置
func (s *EtcdStorage) DeleteNotifier(ctx context.Context, id string) error {
	key := s.buildKey(id)
	if err := s.client.Delete(ctx, key); err != nil {
		return fmt.Errorf("从ETCD删除通知器配置失败: %w", err)
	}

	s.logger.Debug("删除通知器配置", zap.String("id", id))
	return nil
}

// WatchNotifiers 监听通知器配置变更
func (s *EtcdStorage) WatchNotifiers(ctx context.Context) (<-chan notifier.NotifierEvent, error) {
	watchChan := s.client.WatchWithPrefix(ctx, s.prefix)
	eventChan := make(chan notifier.NotifierEvent)

	go s.processWatchEvents(ctx, watchChan, eventChan)

	return eventChan, nil
}

// processWatchEvents 处理ETCD监听事件
func (s *EtcdStorage) processWatchEvents(ctx context.Context, watchChan <-chan clientv3.WatchResponse, eventChan chan<- notifier.NotifierEvent) {
	defer close(eventChan)

	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				s.logger.Warn("ETCD监听通道已关闭")
				return
			}

			for _, event := range resp.Events {
				notifierEvent := s.convertEtcdEvent(event)
				if notifierEvent != nil {
					select {
					case eventChan <- *notifierEvent:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// convertEtcdEvent 转换ETCD事件为通知器事件
func (s *EtcdStorage) convertEtcdEvent(event *clientv3.Event) *notifier.NotifierEvent {
	switch event.Type {
	case clientv3.EventTypePut:
		var config notifier.NotifierConfig
		if err := json.Unmarshal(event.Kv.Value, &config); err != nil {
			s.logger.Error("解析通知器配置失败",
				zap.String("key", string(event.Kv.Key)),
				zap.Error(err))
			return nil
		}

		// 判断是新增还是更新
		eventType := notifier.NotifierEventAdd
		if event.Kv.Version > 1 {
			eventType = notifier.NotifierEventUpdate
		}

		return &notifier.NotifierEvent{
			Type:   eventType,
			Config: &config,
		}

	case clientv3.EventTypeDelete:
		// 删除事件，从key中提取ID
		id := s.extractIDFromKey(string(event.Kv.Key))
		return &notifier.NotifierEvent{
			Type: notifier.NotifierEventDelete,
			Config: &notifier.NotifierConfig{
				ID: id,
			},
		}

	default:
		return nil
	}
}

// buildKey 构建存储键
func (s *EtcdStorage) buildKey(id string) string {
	return path.Join(s.prefix, id)
}

// extractIDFromKey 从存储键中提取ID
func (s *EtcdStorage) extractIDFromKey(key string) string {
	return path.Base(key)
}
