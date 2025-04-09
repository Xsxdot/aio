package state

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/distributed/common"
	"path"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// StateEvent 状态变更事件
type StateEvent struct {
	// Key 状态键
	Key string `json:"key"`
	// Value 状态值
	Value string `json:"value"`
	// Type 事件类型
	Type string `json:"type"`
	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`
}

// StateManagerInfo 状态管理器信息
type StateManagerInfo struct {
	// Prefix 前缀
	Prefix string `json:"prefix"`
	// Keys 键数量
	Keys int `json:"keys"`
	// Status 状态
	Status common.ComponentStatus `json:"status"`
	// CreateTime 创建时间
	CreateTime string `json:"createTime"`
}

// StateEventType 状态事件类型
type StateEventType string

const (
	// StateEventPut 状态设置事件
	StateEventPut StateEventType = "put"
	// StateEventDelete 状态删除事件
	StateEventDelete StateEventType = "delete"
)

// StateManager 状态管理器接口
type StateManager interface {
	// Put 设置状态
	Put(ctx context.Context, key string, value interface{}) error
	// Get 获取状态
	Get(ctx context.Context, key string, value interface{}) (bool, error)
	// Delete 删除状态
	Delete(ctx context.Context, key string) error
	// Watch 监听状态变更
	Watch(ctx context.Context, key string) (<-chan StateEvent, error)
	// ListKeys 列出所有键
	ListKeys(ctx context.Context, prefix string) ([]string, error)
	// GetInfo 获取状态管理器信息
	GetInfo() StateManagerInfo
}

// StateOption 状态管理器配置选项函数类型
type StateOption func(*stateManagerImpl)

// WithStateTTL 设置状态TTL
func WithStateTTL(ttl int64) StateOption {
	return func(s *stateManagerImpl) {
		s.ttl = ttl
	}
}

// StateManagerService 状态管理服务接口
type StateManagerService interface {
	common.Component

	// Create 创建状态管理器
	Create(prefix string, options ...StateOption) (StateManager, error)
	// Get 获取状态管理器
	Get(prefix string) (StateManager, bool)
	// List 列出所有状态管理器
	List() []StateManagerInfo
	// Delete 删除状态管理器
	Delete(prefix string) error
}

// 状态管理服务实现
type stateManagerServiceImpl struct {
	etcdClient *clientv3.Client
	logger     *zap.Logger
	managers   map[string]StateManager
	mutex      sync.RWMutex
	isRunning  bool
}

// 状态管理器实现
type stateManagerImpl struct {
	prefix       string
	ttl          int64
	etcdClient   *clientv3.Client
	logger       *zap.Logger
	watchCancels map[string]context.CancelFunc
	mutex        sync.RWMutex
	status       common.ComponentStatus
	createTime   time.Time
}

// NewStateManagerService 创建状态管理服务
func NewStateManagerService(etcdClient *clientv3.Client, logger *zap.Logger) (StateManagerService, error) {
	return &stateManagerServiceImpl{
		etcdClient: etcdClient,
		logger:     logger,
		managers:   make(map[string]StateManager),
		isRunning:  false,
	}, nil
}

// Start 启动状态管理服务
func (s *stateManagerServiceImpl) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		return nil
	}

	s.logger.Info("Starting state manager service")

	// 从etcd恢复状态管理器配置
	err := s.restoreManagersFromEtcd(ctx)
	if err != nil {
		s.logger.Error("Failed to restore state managers from etcd", zap.Error(err))
		return err
	}

	s.isRunning = true
	return nil
}

// Stop 停止状态管理服务
func (s *stateManagerServiceImpl) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping state manager service")

	// 停止所有状态管理器
	for prefix, manager := range s.managers {
		managerImpl, ok := manager.(*stateManagerImpl)
		if ok {
			s.logger.Debug("Stopping state manager", zap.String("prefix", prefix))
			managerImpl.stopAllWatchers()
		}
	}

	s.isRunning = false
	return nil
}

// Create 创建状态管理器
func (s *stateManagerServiceImpl) Create(prefix string, options ...StateOption) (StateManager, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 标准化前缀
	prefix = normalizePath(prefix)

	// 检查是否已存在
	if manager, exists := s.managers[prefix]; exists {
		return manager, nil
	}

	// 创建状态管理器实现
	manager := &stateManagerImpl{
		prefix:       prefix,
		ttl:          0, // 默认无TTL
		etcdClient:   s.etcdClient,
		logger:       s.logger.With(zap.String("statePrefix", prefix)),
		watchCancels: make(map[string]context.CancelFunc),
		status:       common.StatusCreated,
		createTime:   time.Now(),
	}

	// 应用配置选项
	for _, option := range options {
		option(manager)
	}

	s.logger.Info("Created state manager",
		zap.String("prefix", prefix),
		zap.Int64("ttl", manager.ttl))

	// 保存管理器配置到etcd
	if err := s.saveManagerConfig(context.Background(), prefix, manager); err != nil {
		s.logger.Error("Failed to save manager config", zap.Error(err))
		return nil, err
	}

	// 保存到内存
	s.managers[prefix] = manager

	return manager, nil
}

// Get 获取状态管理器
func (s *stateManagerServiceImpl) Get(prefix string) (StateManager, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	prefix = normalizePath(prefix)
	manager, exists := s.managers[prefix]
	return manager, exists
}

// List 列出所有状态管理器
func (s *stateManagerServiceImpl) List() []StateManagerInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]StateManagerInfo, 0, len(s.managers))
	for _, manager := range s.managers {
		result = append(result, manager.GetInfo())
	}

	return result
}

// Delete 删除状态管理器
func (s *stateManagerServiceImpl) Delete(prefix string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	prefix = normalizePath(prefix)
	manager, exists := s.managers[prefix]
	if !exists {
		return nil
	}

	// 停止所有监听
	managerImpl, ok := manager.(*stateManagerImpl)
	if ok {
		managerImpl.stopAllWatchers()
	}

	// 从etcd删除管理器配置
	configKey := fmt.Sprintf("/distributed/components/states/%s/config", prefix)
	_, err := s.etcdClient.Delete(context.Background(), configKey)
	if err != nil {
		s.logger.Error("Failed to delete manager config from etcd", zap.Error(err))
		return err
	}

	// 从内存中删除
	delete(s.managers, prefix)
	s.logger.Info("Deleted state manager", zap.String("prefix", prefix))

	return nil
}

// 保存管理器配置到etcd
func (s *stateManagerServiceImpl) saveManagerConfig(ctx context.Context, prefix string, manager *stateManagerImpl) error {
	config := map[string]interface{}{
		"ttl":        manager.ttl,
		"createTime": manager.createTime.Format(time.RFC3339),
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("/distributed/components/states/%s/config", prefix)
	_, err = s.etcdClient.Put(ctx, key, string(data))
	return err
}

// 修改恢复状态管理器的方法，避免锁嵌套
func (s *stateManagerServiceImpl) restoreManagersFromEtcd(ctx context.Context) error {
	prefix := "/distributed/components/states/"
	resp, err := s.etcdClient.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	restoreMap := make(map[string]map[string]interface{})

	// 解析所有管理器配置
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if len(key) <= len(prefix) {
			continue
		}

		// 解析管理器名称
		parts := []rune(key[len(prefix):])
		nameEnd := 0
		for i, c := range parts {
			if c == '/' {
				nameEnd = i
				break
			}
		}

		if nameEnd == 0 {
			continue
		}

		name := string(parts[:nameEnd])
		configType := string(parts[nameEnd+1:])

		if configType != "config" {
			continue
		}

		// 解析配置
		var config map[string]interface{}
		if err := json.Unmarshal(kv.Value, &config); err != nil {
			s.logger.Error("Failed to unmarshal state manager config",
				zap.String("name", name),
				zap.Error(err))
			continue
		}

		restoreMap[name] = config
	}

	// 恢复管理器，使用内部方法创建状态管理器实例
	for name, config := range restoreMap {
		ttl, _ := config["ttl"].(float64)

		options := []StateOption{
			WithStateTTL(int64(ttl)),
		}

		// 使用内部方法创建管理器，避免加锁导致的死锁
		manager := s.createManagerInternal(name, options...)
		s.managers[name] = manager

		s.logger.Info("Restored state manager from etcd",
			zap.String("prefix", name))
	}

	return nil
}

// 添加内部方法，不获取互斥锁
func (s *stateManagerServiceImpl) createManagerInternal(prefix string, options ...StateOption) *stateManagerImpl {
	// 标准化前缀
	prefix = normalizePath(prefix)

	// 创建状态管理器实现
	manager := &stateManagerImpl{
		prefix:       prefix,
		ttl:          0, // 默认无TTL
		etcdClient:   s.etcdClient,
		logger:       s.logger.With(zap.String("statePrefix", prefix)),
		watchCancels: make(map[string]context.CancelFunc),
		status:       common.StatusCreated,
		createTime:   time.Now(),
	}

	// 应用配置选项
	for _, option := range options {
		option(manager)
	}

	s.logger.Info("Internally created state manager",
		zap.String("prefix", prefix),
		zap.Int64("ttl", manager.ttl))

	// 这里不需要保存配置到etcd，因为配置已存在

	return manager
}

// Put 设置状态
func (m *stateManagerImpl) Put(ctx context.Context, key string, value interface{}) error {
	fullKey := m.getFullKey(key)

	var data []byte
	var err error

	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
	}

	opts := []clientv3.OpOption{}
	if m.ttl > 0 {
		// 创建租约
		lease, err := m.etcdClient.Grant(ctx, m.ttl)
		if err != nil {
			return fmt.Errorf("failed to create lease: %w", err)
		}
		opts = append(opts, clientv3.WithLease(lease.ID))
	}

	_, err = m.etcdClient.Put(ctx, fullKey, string(data), opts...)
	if err != nil {
		return fmt.Errorf("failed to put value: %w", err)
	}

	return nil
}

// Get 获取状态
func (m *stateManagerImpl) Get(ctx context.Context, key string, value interface{}) (bool, error) {
	fullKey := m.getFullKey(key)

	resp, err := m.etcdClient.Get(ctx, fullKey)
	if err != nil {
		return false, fmt.Errorf("failed to get value: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return false, nil
	}

	data := resp.Kvs[0].Value

	// 根据值类型进行解析
	switch v := value.(type) {
	case *string:
		*v = string(data)
	case *[]byte:
		*v = data
	default:
		if err := json.Unmarshal(data, value); err != nil {
			return true, fmt.Errorf("failed to unmarshal value: %w", err)
		}
	}

	return true, nil
}

// Delete 删除状态
func (m *stateManagerImpl) Delete(ctx context.Context, key string) error {
	fullKey := m.getFullKey(key)

	_, err := m.etcdClient.Delete(ctx, fullKey)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

// Watch 监听状态变更
func (m *stateManagerImpl) Watch(ctx context.Context, key string) (<-chan StateEvent, error) {
	fullKey := m.getFullKey(key)

	// 创建结果通道
	resultCh := make(chan StateEvent, 100)

	// 创建可取消的上下文
	watchCtx, cancel := context.WithCancel(context.Background())

	// 保存取消函数
	m.mutex.Lock()
	m.watchCancels[key] = cancel
	m.mutex.Unlock()

	// 开始监听
	watchCh := m.etcdClient.Watch(watchCtx, fullKey, clientv3.WithPrefix())

	// 启动处理协程
	go func() {
		defer close(resultCh)

		for {
			select {
			case <-ctx.Done():
				m.mutex.Lock()
				delete(m.watchCancels, key)
				m.mutex.Unlock()
				cancel()
				return

			case watchResp, ok := <-watchCh:
				if !ok {
					m.logger.Warn("Watch channel closed", zap.String("key", key))
					m.mutex.Lock()
					delete(m.watchCancels, key)
					m.mutex.Unlock()
					cancel()
					return
				}

				if watchResp.Err() != nil {
					m.logger.Error("Watch error",
						zap.String("key", key),
						zap.Error(watchResp.Err()))
					continue
				}

				// 处理事件
				for _, ev := range watchResp.Events {
					event := StateEvent{
						Key:       strings.TrimPrefix(string(ev.Kv.Key), m.getStatePrefix()),
						Value:     string(ev.Kv.Value),
						Timestamp: time.Now(),
					}

					switch ev.Type {
					case clientv3.EventTypePut:
						event.Type = string(StateEventPut)
					case clientv3.EventTypeDelete:
						event.Type = string(StateEventDelete)
					}

					select {
					case resultCh <- event:
					case <-watchCtx.Done():
						return
					default:
						// 如果通道已满，记录日志
						m.logger.Warn("Event channel full, dropping event",
							zap.String("key", event.Key))
					}
				}
			}
		}
	}()

	return resultCh, nil
}

// ListKeys 列出所有键
func (m *stateManagerImpl) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := m.getFullKey(prefix)

	resp, err := m.etcdClient.Get(ctx, fullPrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	statePrefix := m.getStatePrefix()
	keys := make([]string, 0, len(resp.Kvs))

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		// 去除前缀
		if strings.HasPrefix(key, statePrefix) {
			keys = append(keys, strings.TrimPrefix(key, statePrefix))
		}
	}

	return keys, nil
}

// GetInfo 获取状态管理器信息
func (m *stateManagerImpl) GetInfo() StateManagerInfo {
	keys, _ := m.ListKeys(context.Background(), "")

	return StateManagerInfo{
		Prefix:     m.prefix,
		Keys:       len(keys),
		Status:     m.status,
		CreateTime: m.createTime.Format(time.RFC3339),
	}
}

// 获取完整键
func (m *stateManagerImpl) getFullKey(key string) string {
	return path.Join(m.getStatePrefix(), key)
}

// 获取状态前缀
func (m *stateManagerImpl) getStatePrefix() string {
	return path.Join("/distributed/state", m.prefix)
}

// 停止所有监听
func (m *stateManagerImpl) stopAllWatchers() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for key, cancel := range m.watchCancels {
		m.logger.Debug("Stopping watcher", zap.String("key", key))
		cancel()
	}

	m.watchCancels = make(map[string]context.CancelFunc)
}

// 标准化路径
func normalizePath(p string) string {
	// 确保路径以/开头，不以/结尾
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	return strings.TrimSuffix(p, "/")
}
