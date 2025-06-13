package lock

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/common"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

// EtcdLockManager ETCD分布式锁管理器
type EtcdLockManager struct {
	client   *etcd.EtcdClient
	logger   *zap.Logger
	prefix   string
	hostname string
	pid      int
	mu       sync.RWMutex
	locks    map[string]*EtcdLock
}

// NewEtcdLockManager 创建ETCD锁管理器
func NewEtcdLockManager(client *etcd.EtcdClient, prefix string) (*EtcdLockManager, error) {
	if client == nil {
		return nil, NewLockError(ErrCodeInvalidKey, "etcd客户端不能为空", nil)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &EtcdLockManager{
		client:   client,
		logger:   common.GetLogger().GetZapLogger("aio-lock-manager"),
		prefix:   strings.TrimSuffix(prefix, "/") + "/locks/",
		hostname: hostname,
		pid:      os.Getpid(),
		locks:    make(map[string]*EtcdLock),
	}, nil
}

// NewLock 创建新的分布式锁
func (m *EtcdLockManager) NewLock(key string, opts *LockOptions) DistributedLock {
	if opts == nil {
		opts = DefaultLockOptions()
	}

	fullKey := m.prefix + key
	owner := fmt.Sprintf("%s-%d-%d", m.hostname, m.pid, time.Now().UnixNano())

	lock := &EtcdLock{
		manager:   m,
		key:       fullKey,
		rawKey:    key,
		owner:     owner,
		options:   opts,
		logger:    m.logger.With(zap.String("lock_key", key), zap.String("owner", owner)),
		reentrant: NewReentrantCounter(),
		stopCh:    make(chan struct{}),
	}

	m.mu.Lock()
	m.locks[key] = lock
	m.mu.Unlock()

	return lock
}

// GetLockInfo 获取锁信息
func (m *EtcdLockManager) GetLockInfo(ctx context.Context, key string) (*LockInfo, error) {
	fullKey := m.prefix + key

	resp, err := m.client.Get(ctx, fullKey)
	if err != nil {
		return nil, NewLockError(ErrCodeInvalidKey, "获取锁信息失败", err)
	}

	if resp == "" {
		return nil, NewLockError(ErrCodeLockNotHeld, "锁不存在", nil)
	}

	var info LockInfo
	if err := json.Unmarshal([]byte(resp), &info); err != nil {
		return nil, NewLockError(ErrCodeInvalidKey, "解析锁信息失败", err)
	}

	return &info, nil
}

// ListLocks 列出所有锁
func (m *EtcdLockManager) ListLocks(ctx context.Context, prefix string) ([]*LockInfo, error) {
	searchPrefix := m.prefix
	if prefix != "" {
		searchPrefix = m.prefix + prefix
	}

	kvs, err := m.client.GetWithPrefix(ctx, searchPrefix)
	if err != nil {
		return nil, NewLockError(ErrCodeInvalidKey, "列出锁失败", err)
	}

	var locks []*LockInfo
	for _, value := range kvs {
		var info LockInfo
		if err := json.Unmarshal([]byte(value), &info); err != nil {
			m.logger.Warn("解析锁信息失败", zap.Error(err))
			continue
		}
		locks = append(locks, &info)
	}

	return locks, nil
}

// ForceUnlock 强制释放锁
func (m *EtcdLockManager) ForceUnlock(ctx context.Context, key string) error {
	fullKey := m.prefix + key

	err := m.client.Delete(ctx, fullKey)
	if err != nil {
		return NewLockError(ErrCodeInvalidKey, "强制释放锁失败", err)
	}

	m.logger.Info("强制释放锁成功", zap.String("key", key))
	return nil
}

// Close 关闭锁管理器
func (m *EtcdLockManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭所有锁
	for _, lock := range m.locks {
		lock.forceStop()
	}

	m.locks = make(map[string]*EtcdLock)
	m.logger.Info("锁管理器已关闭")
	return nil
}

// EtcdLock ETCD分布式锁实现
type EtcdLock struct {
	manager   *EtcdLockManager
	key       string
	rawKey    string
	owner     string
	options   *LockOptions
	logger    *zap.Logger
	reentrant *ReentrantCounter

	mu        sync.RWMutex
	session   *concurrency.Session
	mutex     *concurrency.Mutex
	locked    bool
	stopCh    chan struct{}
	renewDone chan struct{}
}

// Lock 获取锁
func (l *EtcdLock) Lock(ctx context.Context) error {
	return l.LockWithTimeout(ctx, 0)
}

// TryLock 尝试获取锁，不阻塞
func (l *EtcdLock) TryLock(ctx context.Context) (bool, error) {
	// 检查可重入
	if l.reentrant.TryEnter(l.owner) {
		if l.reentrant.GetCount() > 1 {
			l.logger.Debug("可重入锁计数增加", zap.Int("count", l.reentrant.GetCount()))
			return true, nil
		}
	}

	// 创建短期超时的上下文
	tryCtx, cancel := context.WithTimeout(ctx, l.options.RetryInterval)
	defer cancel()

	err := l.acquireLock(tryCtx)
	if err != nil {
		l.reentrant.Exit(l.owner)
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// LockWithTimeout 带超时的获取锁
func (l *EtcdLock) LockWithTimeout(ctx context.Context, timeout time.Duration) error {
	// 检查可重入
	if l.reentrant.TryEnter(l.owner) {
		if l.reentrant.GetCount() > 1 {
			l.logger.Debug("可重入锁计数增加", zap.Int("count", l.reentrant.GetCount()))
			return nil
		}
	}

	var lockCtx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		lockCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	} else {
		lockCtx = ctx
	}

	// 重试获取锁
	var lastErr error
	retries := 0

	for {
		select {
		case <-lockCtx.Done():
			l.reentrant.Exit(l.owner)
			return NewLockError(ErrCodeLockTimeout, "获取锁超时", lockCtx.Err())
		default:
		}

		err := l.acquireLock(lockCtx)
		if err == nil {
			return nil
		}

		lastErr = err
		retries++

		// 检查是否达到最大重试次数
		if l.options.MaxRetries > 0 && retries >= l.options.MaxRetries {
			l.reentrant.Exit(l.owner)
			return NewLockError(ErrCodeLockTimeout, "达到最大重试次数", lastErr)
		}

		// 等待重试间隔
		select {
		case <-lockCtx.Done():
			l.reentrant.Exit(l.owner)
			return NewLockError(ErrCodeLockTimeout, "获取锁超时", lockCtx.Err())
		case <-time.After(l.options.RetryInterval):
		}
	}
}

// acquireLock 实际获取锁
func (l *EtcdLock) acquireLock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 创建session
	session, err := concurrency.NewSession(l.manager.client.Client,
		concurrency.WithTTL(int(l.options.TTL.Seconds())))
	if err != nil {
		return NewLockError(ErrCodeInvalidKey, "创建session失败", err)
	}

	// 创建mutex
	mutex := concurrency.NewMutex(session, l.key)

	// 尝试获取锁
	err = mutex.Lock(ctx)
	if err != nil {
		session.Close()
		return NewLockError(ErrCodeLockTimeout, "获取锁失败", err)
	}

	// 保存锁信息
	lockInfo := &LockInfo{
		Key:        l.rawKey,
		Owner:      l.owner,
		CreateTime: time.Now(),
		ExpireTime: time.Now().Add(l.options.TTL),
		Version:    int64(session.Lease()),
	}

	infoData, _ := json.Marshal(lockInfo)

	// 将锁信息写入ETCD
	err = l.manager.client.Put(ctx, l.key+"_info", string(infoData))
	if err != nil {
		mutex.Unlock(ctx)
		session.Close()
		return NewLockError(ErrCodeInvalidKey, "保存锁信息失败", err)
	}

	l.session = session
	l.mutex = mutex
	l.locked = true
	l.renewDone = make(chan struct{})

	// 启动自动续期
	if l.options.AutoRenew {
		go l.autoRenew()
	}

	l.logger.Info("成功获取锁",
		zap.Duration("ttl", l.options.TTL),
		zap.Bool("auto_renew", l.options.AutoRenew))

	return nil
}

// Unlock 释放锁
func (l *EtcdLock) Unlock(ctx context.Context) error {
	// 处理可重入
	if !l.reentrant.Exit(l.owner) {
		return NewLockError(ErrCodeLockNotHeld, "当前实例未持有锁", nil)
	}

	if l.reentrant.GetCount() > 0 {
		l.logger.Debug("可重入锁计数减少", zap.Int("count", l.reentrant.GetCount()))
		return nil
	}

	return l.releaseLock(ctx)
}

// releaseLock 实际释放锁
func (l *EtcdLock) releaseLock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.locked {
		return NewLockError(ErrCodeLockNotHeld, "锁未被持有", nil)
	}

	// 停止自动续期
	if l.renewDone != nil {
		close(l.renewDone)
		l.renewDone = nil
	}

	// 删除锁信息
	l.manager.client.Delete(ctx, l.key+"_info")

	// 释放mutex
	if l.mutex != nil {
		err := l.mutex.Unlock(ctx)
		if err != nil {
			l.logger.Warn("释放mutex失败", zap.Error(err))
		}
		l.mutex = nil
	}

	// 关闭session
	if l.session != nil {
		l.session.Close()
		l.session = nil
	}

	l.locked = false

	l.logger.Info("成功释放锁")
	return nil
}

// Renew 续期锁
func (l *EtcdLock) Renew(ctx context.Context) error {
	l.mu.RLock()
	session := l.session
	l.mu.RUnlock()

	if session == nil {
		return NewLockError(ErrCodeLockNotHeld, "锁未被持有", nil)
	}

	// 续期session
	_, err := session.Client().KeepAliveOnce(ctx, session.Lease())
	if err != nil {
		return NewLockError(ErrCodeLockExpired, "续期失败", err)
	}

	l.logger.Debug("锁续期成功")
	return nil
}

// autoRenew 自动续期
func (l *EtcdLock) autoRenew() {
	ticker := time.NewTicker(l.options.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.renewDone:
			return
		case <-l.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := l.Renew(ctx)
			cancel()

			if err != nil {
				l.logger.Error("自动续期失败", zap.Error(err))
				return
			}
		}
	}
}

// IsLocked 检查锁是否被当前实例持有
func (l *EtcdLock) IsLocked() bool {
	return l.reentrant.IsHeld(l.owner)
}

// GetLockKey 获取锁的键
func (l *EtcdLock) GetLockKey() string {
	return l.rawKey
}

// forceStop 强制停止锁
func (l *EtcdLock) forceStop() {
	close(l.stopCh)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.renewDone != nil {
		close(l.renewDone)
		l.renewDone = nil
	}

	if l.session != nil {
		l.session.Close()
		l.session = nil
	}

	l.locked = false
}
