package lock

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/distributed/common"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

// LockInfo 锁信息
type LockInfo struct {
	// Key 锁键
	Key string `json:"key"`
	// IsLocked 是否已锁定
	IsLocked bool `json:"isLocked"`
	// Status 锁状态
	Status common.ComponentStatus `json:"status"`
	// TTL 锁的TTL秒数
	TTL int `json:"ttl"`
	// LastLockTime 最后获取锁时间
	LastLockTime time.Time `json:"lastLockTime,omitempty"`
	// CreateTime 创建时间
	CreateTime string `json:"createTime"`
}

// Lock 锁接口
type Lock interface {
	// Lock 获取锁
	Lock(ctx context.Context) error
	// TryLock 尝试获取锁，不阻塞
	TryLock(ctx context.Context) (bool, error)
	// Unlock 释放锁
	Unlock(ctx context.Context) error
	// IsLocked 是否已锁定
	IsLocked() bool
	// GetInfo 获取锁信息
	GetInfo() LockInfo
}

// LockOption 锁配置选项函数类型
type LockOption func(*lockImpl)

// WithLockTTL 设置锁TTL
func WithLockTTL(ttl int) LockOption {
	return func(l *lockImpl) {
		l.ttl = ttl
	}
}

// WithLockMaxRetries 设置最大重试次数
func WithLockMaxRetries(maxRetries int) LockOption {
	return func(l *lockImpl) {
		l.maxRetries = maxRetries
	}
}

// LockService 锁服务接口
type LockService interface {
	common.Component

	// Create 创建锁实例
	Create(key string, options ...LockOption) (Lock, error)
	// Get 获取锁实例
	Get(key string) (Lock, bool)
	// List 列出所有锁
	List() []LockInfo
	// Delete 删除锁实例
	Delete(key string) error
}

// 锁服务实现
type lockServiceImpl struct {
	etcdClient *clientv3.Client
	logger     *zap.Logger
	locks      map[string]Lock
	mutex      sync.RWMutex
	isRunning  bool
}

// 锁实现
type lockImpl struct {
	key          string
	ttl          int
	maxRetries   int
	etcdClient   *clientv3.Client
	logger       *zap.Logger
	session      *concurrency.Session
	mutex        *concurrency.Mutex
	isLocked     bool
	status       common.ComponentStatus
	createTime   time.Time
	lastLockTime time.Time
}

// NewLockService 创建锁服务
func NewLockService(etcdClient *clientv3.Client, logger *zap.Logger) (LockService, error) {
	return &lockServiceImpl{
		etcdClient: etcdClient,
		logger:     logger,
		locks:      make(map[string]Lock),
		isRunning:  false,
	}, nil
}

// Start 启动锁服务
func (s *lockServiceImpl) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		return nil
	}

	s.logger.Info("Starting lock service")

	// 从etcd恢复锁配置
	err := s.restoreLocksFromEtcd(ctx)
	if err != nil {
		s.logger.Error("Failed to restore locks from etcd", zap.Error(err))
		return err
	}

	s.isRunning = true
	return nil
}

// Stop 停止锁服务
func (s *lockServiceImpl) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping lock service")

	// 释放所有锁
	for key, lock := range s.locks {
		lockImpl, ok := lock.(*lockImpl)
		if ok && lockImpl.isLocked {
			s.logger.Debug("Unlocking lock", zap.String("key", key))
			if err := lockImpl.Unlock(ctx); err != nil {
				s.logger.Error("Failed to unlock", zap.String("key", key), zap.Error(err))
			}
		}
	}

	s.isRunning = false
	return nil
}

// Create 创建锁实例
func (s *lockServiceImpl) Create(key string, options ...LockOption) (Lock, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否已存在
	if _, exists := s.locks[key]; exists {
		return s.locks[key], nil
	}

	// 创建锁实现
	lockImpl := &lockImpl{
		key:        key,
		ttl:        30, // 默认TTL 30秒
		maxRetries: 3,  // 默认最大重试3次
		etcdClient: s.etcdClient,
		logger:     s.logger.With(zap.String("lock", key)),
		isLocked:   false,
		status:     common.StatusCreated,
		createTime: time.Now(),
	}

	// 应用配置选项
	for _, option := range options {
		option(lockImpl)
	}

	s.logger.Info("Created lock",
		zap.String("key", key),
		zap.Int("ttl", lockImpl.ttl),
		zap.Int("maxRetries", lockImpl.maxRetries))

	// 保存锁配置到etcd
	if err := s.saveLockConfig(context.Background(), key, lockImpl); err != nil {
		s.logger.Error("Failed to save lock config", zap.Error(err))
		return nil, err
	}

	// 保存到内存
	s.locks[key] = lockImpl

	return lockImpl, nil
}

// Get 获取锁实例
func (s *lockServiceImpl) Get(key string) (Lock, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	lock, exists := s.locks[key]
	return lock, exists
}

// List 列出所有锁
func (s *lockServiceImpl) List() []LockInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]LockInfo, 0, len(s.locks))
	for _, lock := range s.locks {
		result = append(result, lock.GetInfo())
	}

	return result
}

// Delete 删除锁实例
func (s *lockServiceImpl) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	lock, exists := s.locks[key]
	if !exists {
		return nil
	}

	// 如果已锁定，先解锁
	if lock.IsLocked() {
		if err := lock.Unlock(context.Background()); err != nil {
			s.logger.Error("Failed to unlock before delete", zap.String("key", key), zap.Error(err))
			// 继续删除，不返回错误
		}
	}

	// 从etcd删除锁配置
	configKey := fmt.Sprintf("/distributed/components/locks/%s/config", key)
	_, err := s.etcdClient.Delete(context.Background(), configKey)
	if err != nil {
		s.logger.Error("Failed to delete lock config from etcd", zap.Error(err))
		return err
	}

	// 从内存中删除
	delete(s.locks, key)
	s.logger.Info("Deleted lock", zap.String("key", key))

	return nil
}

// 保存锁配置到etcd
func (s *lockServiceImpl) saveLockConfig(ctx context.Context, key string, lock *lockImpl) error {
	config := map[string]interface{}{
		"ttl":        lock.ttl,
		"maxRetries": lock.maxRetries,
		"createTime": lock.createTime.Format(time.RFC3339),
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	configKey := fmt.Sprintf("/distributed/components/locks/%s/config", key)
	_, err = s.etcdClient.Put(ctx, configKey, string(data))
	return err
}

// 修改恢复锁的方法，避免锁嵌套
func (s *lockServiceImpl) restoreLocksFromEtcd(ctx context.Context) error {
	prefix := "/distributed/components/locks/"
	resp, err := s.etcdClient.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	restoreMap := make(map[string]map[string]interface{})

	// 解析所有锁配置
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if len(key) <= len(prefix) {
			continue
		}

		// 解析锁名称
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
			s.logger.Error("Failed to unmarshal lock config",
				zap.String("key", name),
				zap.Error(err))
			continue
		}

		restoreMap[name] = config
	}

	// 恢复锁，使用内部方法创建锁实例，避免互斥锁嵌套
	for key, config := range restoreMap {
		ttl, _ := config["ttl"].(float64)
		maxRetries, _ := config["maxRetries"].(float64)

		options := []LockOption{
			WithLockTTL(int(ttl)),
			WithLockMaxRetries(int(maxRetries)),
		}

		// 使用内部方法创建锁，而不是调用 s.Create
		lockImpl := s.createLockInternal(key, options...)
		s.locks[key] = lockImpl
		s.logger.Info("Restored lock from etcd", zap.String("key", key))
	}

	return nil
}

// 添加内部方法，不获取互斥锁
func (s *lockServiceImpl) createLockInternal(key string, options ...LockOption) *lockImpl {
	// 创建锁实现
	lockImpl := &lockImpl{
		key:        key,
		ttl:        30, // 默认TTL 30秒
		maxRetries: 3,  // 默认最大重试3次
		etcdClient: s.etcdClient,
		logger:     s.logger.With(zap.String("lock", key)),
		isLocked:   false,
		status:     common.StatusCreated,
		createTime: time.Now(),
	}

	// 应用配置选项
	for _, option := range options {
		option(lockImpl)
	}

	s.logger.Info("Internally created lock",
		zap.String("key", key),
		zap.Int("ttl", lockImpl.ttl),
		zap.Int("maxRetries", lockImpl.maxRetries))

	// 保存锁配置到etcd (可选，因为配置已经存在于etcd中)
	// 这里省略保存配置的步骤，因为配置应该已经存在

	return lockImpl
}

// Lock 获取锁
func (l *lockImpl) Lock(ctx context.Context) error {
	if l.isLocked {
		return nil
	}

	var err error
	var retry int

	for retry = 0; retry <= l.maxRetries; retry++ {
		if retry > 0 {
			l.logger.Debug("Retrying to acquire lock",
				zap.String("key", l.key),
				zap.Int("attempt", retry),
				zap.Int("maxRetries", l.maxRetries))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(retry*500) * time.Millisecond):
				// 指数退避
			}
		}

		if err = l.acquireLock(ctx); err == nil {
			return nil
		}

		l.logger.Warn("Failed to acquire lock",
			zap.String("key", l.key),
			zap.Int("attempt", retry+1),
			zap.Error(err))
	}

	return fmt.Errorf("failed to acquire lock after %d attempts: %w", retry, err)
}

// TryLock 尝试获取锁
func (l *lockImpl) TryLock(ctx context.Context) (bool, error) {
	if l.isLocked {
		return true, nil
	}

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err := l.acquireLock(timeoutCtx)
	if err == nil {
		return true, nil
	}

	if err == context.DeadlineExceeded {
		return false, nil
	}

	return false, err
}

// 获取锁的内部实现
func (l *lockImpl) acquireLock(ctx context.Context) error {
	// 如果已经获取了锁，直接返回
	if l.isLocked {
		return nil
	}

	// 创建etcd会话
	var err error
	l.session, err = concurrency.NewSession(l.etcdClient, concurrency.WithTTL(l.ttl))
	if err != nil {
		return err
	}

	lockKey := fmt.Sprintf("/distributed/locks/%s", l.key)
	l.mutex = concurrency.NewMutex(l.session, lockKey)

	// 获取锁
	if err := l.mutex.Lock(ctx); err != nil {
		l.session.Close()
		l.session = nil
		l.mutex = nil
		return err
	}

	l.isLocked = true
	l.lastLockTime = time.Now()
	l.status = common.StatusRunning

	l.logger.Debug("Acquired lock", zap.String("key", l.key))

	return nil
}

// Unlock 释放锁
func (l *lockImpl) Unlock(ctx context.Context) error {
	if !l.isLocked || l.mutex == nil {
		return nil
	}

	// 释放锁
	if err := l.mutex.Unlock(ctx); err != nil {
		l.logger.Error("Failed to unlock",
			zap.String("key", l.key),
			zap.Error(err))
		return err
	}

	// 关闭会话
	if l.session != nil {
		l.session.Close()
	}

	l.isLocked = false
	l.mutex = nil
	l.session = nil
	l.status = common.StatusStopped

	l.logger.Debug("Released lock", zap.String("key", l.key))

	return nil
}

// IsLocked 是否已锁定
func (l *lockImpl) IsLocked() bool {
	return l.isLocked
}

// GetInfo 获取锁信息
func (l *lockImpl) GetInfo() LockInfo {
	return LockInfo{
		Key:          l.key,
		IsLocked:     l.isLocked,
		Status:       l.status,
		TTL:          l.ttl,
		LastLockTime: l.lastLockTime,
		CreateTime:   l.createTime.Format(time.RFC3339),
	}
}
