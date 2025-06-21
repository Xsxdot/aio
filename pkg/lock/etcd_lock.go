package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/common"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
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

	// 共享会话
	session *concurrency.Session
	// 用于通知所有锁会话已失效
	ctx    context.Context
	cancel context.CancelFunc

	// manager options to re-create session
	opts *LockManagerOptions
	// closed flag to distinguish between active close and temporary failure
	closed bool
}

// NewEtcdLockManager 创建ETCD锁管理器
func NewEtcdLockManager(client *etcd.EtcdClient, prefix string, opts *LockManagerOptions) (*EtcdLockManager, error) {
	if client == nil {
		return nil, NewLockError(ErrCodeInvalidKey, "etcd客户端不能为空", nil)
	}

	if opts == nil {
		opts = DefaultLockManagerOptions()
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &EtcdLockManager{
		client:   client,
		logger:   common.GetLogger().GetZapLogger("aio-lock-manager"),
		prefix:   strings.TrimSuffix(prefix, "/") + "/locks/",
		hostname: hostname,
		pid:      os.Getpid(),
		locks:    make(map[string]*EtcdLock),
		ctx:      ctx,
		cancel:   cancel,
		opts:     opts,
		closed:   false,
	}

	// 创建共享会话
	session, err := concurrency.NewSession(client.Client,
		concurrency.WithTTL(int(opts.TTL.Seconds())),
		concurrency.WithContext(manager.ctx))
	if err != nil {
		cancel()
		return nil, NewLockError(ErrCodeInvalidKey, "创建共享session失败", err)
	}
	manager.session = session

	// 启动会话监控
	go manager.monitorSession()

	return manager, nil
}

// monitorSession 监控并自愈共享会话
func (m *EtcdLockManager) monitorSession() {
	for {
		// 当session的Done()通道关闭时，意味着会话已失效
		select {
		case <-m.ctx.Done():
			// 主动关闭
			m.logger.Info("锁管理器上下文关闭，会话监控退出")
			return
		case <-m.session.Done():
			// 被动失效
			m.mu.RLock()
			if m.closed {
				m.mu.RUnlock()
				m.logger.Info("锁管理器已主动关闭，会话监控退出")
				return
			}
			m.mu.RUnlock()

			m.logger.Warn("共享etcd会话已失效，将尝试自动重建...")

			// 关键：取消与旧会话关联的上下文，这将导致所有基于旧会话的锁失效。
			m.cancel()

			// 进入无限重试循环以创建新会话
			reconnectInterval := time.Second
			for {
				m.mu.Lock()
				if m.closed {
					m.mu.Unlock()
					m.logger.Info("在会话重建期间，锁管理器被主动关闭，退出重试")
					return
				}

				newCtx, newCancel := context.WithCancel(context.Background())
				newSession, err := concurrency.NewSession(m.client.Client,
					concurrency.WithTTL(int(m.opts.TTL.Seconds())),
					concurrency.WithContext(newCtx))

				if err == nil {
					// 成功重建会话
					m.session = newSession
					m.ctx = newCtx
					m.cancel = newCancel
					m.logger.Info("共享etcd会话已成功重建")
					m.mu.Unlock()
					break // 跳出重试循环
				}

				// 重建失败
				m.mu.Unlock()
				m.logger.Error("重建etcd会话失败，将稍后重试", zap.Error(err), zap.Duration("retry_after", reconnectInterval))
				time.Sleep(reconnectInterval)
				// 指数退避策略
				if reconnectInterval < 30*time.Second {
					reconnectInterval *= 2
				}
			}
		}
	}
}

// NewLock 创建新的分布式锁 (工厂模式)
func (m *EtcdLockManager) NewLock(key string, opts *LockOptions) DistributedLock {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 工厂模式：如果已存在，直接返回，确保对同一个key返回同一个锁实例
	if lock, ok := m.locks[key]; ok {
		return lock
	}

	if opts == nil {
		opts = DefaultLockOptions()
	}

	fullKey := m.prefix + key
	// owner现在用于调试和LockInfo，但重入判断基于单例
	owner := fmt.Sprintf("%s-%d", m.hostname, m.pid)

	lock := &EtcdLock{
		manager:   m,
		key:       fullKey,
		rawKey:    key,
		owner:     owner,
		options:   opts,
		logger:    m.logger.With(zap.String("lock_key", key), zap.String("owner", owner)),
		reentrant: NewReentrantCounter(),
	}

	m.locks[key] = lock
	return lock
}

// GetLockInfo 获取锁信息
func (m *EtcdLockManager) GetLockInfo(ctx context.Context, key string) (*LockInfo, error) {
	infoKey := m.prefix + key + "_info"

	resp, err := m.client.Get(ctx, infoKey)
	if err != nil {
		return nil, NewLockError(ErrCodeInvalidKey, "获取锁信息失败", err)
	}

	if resp == "" {
		return nil, NewLockError(ErrCodeLockNotHeld, "锁不存在或信息已丢失", nil)
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
	for key, value := range kvs {
		// 我们只关心锁信息，它们以后缀 "_info" 存储
		if !strings.HasSuffix(key, "_info") {
			continue
		}
		var info LockInfo
		if err := json.Unmarshal([]byte(value), &info); err != nil {
			m.logger.Warn("解析锁信息失败", zap.String("key", key), zap.Error(err))
			continue
		}
		locks = append(locks, &info)
	}

	return locks, nil
}

// ForceUnlock 强制释放锁
func (m *EtcdLockManager) ForceUnlock(ctx context.Context, key string) error {
	info, err := m.GetLockInfo(ctx, key)
	if err != nil {
		// 如果锁信息不存在，可能锁已经被释放了
		if e, ok := err.(*LockError); ok && e.Code == ErrCodeLockNotHeld {
			m.logger.Warn("尝试强制解锁一个不存在或无信息的锁", zap.String("key", key))
			return nil
		}
		return NewLockError(ErrCodeInvalidKey, "强制释放锁失败：无法获取锁信息", err)
	}

	if info.Version == 0 {
		return NewLockError(ErrCodeInvalidKey, "强制释放锁失败：无效的租约ID", nil)
	}

	leaseID := clientv3.LeaseID(info.Version)

	// 吊销租约，这将自动删除与该租约关联的所有键（即锁本身）
	_, err = m.client.Client.Revoke(ctx, leaseID)
	if err != nil {
		// 如果租约未找到，说明它可能已经过期了，这也是一种成功
		if err == rpctypes.ErrLeaseNotFound {
			m.logger.Warn("尝试吊销一个不存在的租约，可能已经过期", zap.Int64("leaseID", int64(leaseID)))
		} else {
			return NewLockError(ErrCodeInvalidKey, "强制释放锁失败：吊销租约失败", err)
		}
	}

	// 删除我们手动创建的锁信息键
	infoKey := m.prefix + key + "_info"
	if err := m.client.Delete(ctx, infoKey); err != nil {
		// 记录这个错误，但由于租约已吊销，锁已被释放，所以不返回失败
		m.logger.Error("删除锁信息失败，但锁本身已通过租约吊销被释放", zap.String("key", infoKey), zap.Error(err))
	}

	m.logger.Info("强制释放锁成功", zap.String("key", key))
	return nil
}

// Close 关闭锁管理器
func (m *EtcdLockManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil // 幂等关闭
	}

	m.closed = true

	// 这会触发 manager.ctx 的取消，从而通过 concurrency.WithContext 关闭会话
	// 并且会通知 monitorSession 协程彻底退出
	m.cancel()

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

	mu     sync.RWMutex
	mutex  *concurrency.Mutex
	locked bool

	// 锁丢失事件通知
	done   chan struct{}
	cancel context.CancelFunc
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
	tryCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	err := l.acquireLock(tryCtx)
	if err != nil {
		l.reentrant.Exit(l.owner) // 获取失败，恢复重入计数
		// 使用errors.Is可以优雅地处理包括被包装过的超时错误
		if errors.Is(err, context.DeadlineExceeded) || err == concurrency.ErrLocked {
			return false, nil
		}
		// 其他错误是真正的执行错误，需要返回
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

	// 如果获取失败，需要撤销 TryEnter 的效果
	defer func() {
		if !l.locked {
			l.reentrant.Exit(l.owner)
		}
	}()

	var lockCtx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		lockCtx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		lockCtx = ctx
	}
	if cancel != nil {
		defer cancel()
	}

	// 重试获取锁
	var lastErr error
	retries := 0

	for {
		// 优先检查顶层或超时上下文
		select {
		case <-lockCtx.Done():
			return NewLockError(ErrCodeLockTimeout, "获取锁超时", lockCtx.Err())
		case <-l.manager.ctx.Done():
			return NewLockError(ErrCodeLockExpired, "获取锁失败，因管理器会话已关闭", l.manager.ctx.Err())
		default:
		}

		err := l.acquireLock(lockCtx)
		if err == nil {
			return nil // 成功获取锁
		}

		lastErr = err
		retries++

		if l.options.MaxRetries > 0 && retries >= l.options.MaxRetries {
			return NewLockError(ErrCodeLockTimeout, "达到最大重试次数", lastErr)
		}

		// 等待重试间隔
		select {
		case <-lockCtx.Done():
			return NewLockError(ErrCodeLockTimeout, "获取锁超时", lockCtx.Err())
		case <-l.manager.ctx.Done():
			return NewLockError(ErrCodeLockExpired, "获取锁失败，因管理器会话已关闭", l.manager.ctx.Err())
		case <-time.After(l.options.RetryInterval):
		}
	}
}

// acquireLock 实际获取锁
func (l *EtcdLock) acquireLock(ctx context.Context) error {
	// 在尝试获取锁前，检查会话是否有效
	if l.manager.ctx.Err() != nil {
		return NewLockError(ErrCodeLockExpired, "管理器会话已关闭", l.manager.ctx.Err())
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 直接使用共享会话创建Mutex
	mutex := concurrency.NewMutex(l.manager.session, l.key)

	// 尝试获取锁
	err := mutex.Lock(ctx)
	if err != nil {
		return err // 返回原始错误，让上层判断
	}

	// 创建锁专属的上下文和取消函数，用于Done()通知
	lockCtx, cancel := context.WithCancel(l.manager.ctx)
	doneChan := make(chan struct{})
	l.done = doneChan
	l.cancel = cancel

	// 启动一个goroutine来监听lockCtx.Done()并关闭捕获的channel
	go func() {
		<-lockCtx.Done()
		close(doneChan)
	}()

	// 保存锁信息，用于查询和强制解锁
	lockInfo := &LockInfo{
		Key:        l.rawKey,
		Owner:      l.owner,
		CreateTime: time.Now(),
		Version:    int64(l.manager.session.Lease()),
	}
	infoData, _ := json.Marshal(lockInfo)

	// 将锁信息写入ETCD，使用一个独立的key
	// 必须使用与锁相同的租约，确保原子性释放
	infoKey := l.key + "_info"
	_, err = l.manager.client.Client.Put(ctx, infoKey, string(infoData), clientv3.WithLease(l.manager.session.Lease()))
	if err != nil {
		// 如果写入信息失败，必须释放刚获取的锁，避免状态不一致
		mutex.Unlock(context.Background()) // 使用新的后台context确保能执行
		cancel()                           // 取消Done()通知
		return NewLockError(ErrCodeInvalidKey, "保存锁信息失败", err)
	}

	l.mutex = mutex
	l.locked = true

	l.logger.Info("成功获取锁", zap.Int64("lease", int64(l.manager.session.Lease())))
	return nil
}

// Unlock 释放锁
func (l *EtcdLock) Unlock(ctx context.Context) error {
	// 处理可重入
	if !l.reentrant.Exit(l.owner) {
		return NewLockError(ErrCodeLockNotHeld, "当前实例未持有锁或重入次数不匹配", nil)
	}

	if l.reentrant.GetCount() > 0 {
		l.logger.Debug("可重入锁计数减少", zap.Int("count", l.reentrant.GetCount()))
		return nil
	}

	// 计数为0，实际释放锁
	return l.releaseLock(ctx)
}

// releaseLock 实际释放锁
func (l *EtcdLock) releaseLock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.locked {
		// 这种情况可能发生在：锁已因会话过期而释放，但用户代码依然调用了Unlock
		return NewLockError(ErrCodeLockNotHeld, "锁未被持有或已自动释放", nil)
	}

	// 优先删除锁信息
	// 注意：由于info key和lock key绑定了同一个lease，当lease过期时它们都会被删除。
	// 手动删除是为了在正常Unlock时，让GetLockInfo()能立刻反映锁已释放的状态。
	infoKey := l.key + "_info"
	// 使用后台context删除，以防传入的ctx已超时
	if err := l.manager.client.Delete(context.Background(), infoKey); err != nil {
		// 只记录错误，因为即使信息删除失败，我们仍需尝试释放锁
		l.logger.Warn("删除锁信息失败", zap.Error(err))
	}

	// 释放mutex
	if l.mutex != nil {
		// 使用后台context确保能执行
		if err := l.mutex.Unlock(context.Background()); err != nil {
			// 如果解锁失败（例如会话已过期），我们只记录日志，因为锁实际上已经释放了
			l.logger.Warn("释放mutex失败，可能锁已因会话过期而释放", zap.Error(err))
		}
		l.mutex = nil
	}

	// 关闭Done()通知 - 通过取消上下文来触发goroutine关闭channel
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
		l.done = nil
	}

	l.locked = false

	// 从管理器中移除锁实例，防止内存泄漏
	l.manager.mu.Lock()
	delete(l.manager.locks, l.rawKey)
	l.manager.mu.Unlock()

	l.logger.Info("成功释放锁")
	return nil
}

// IsLocked 检查锁是否被当前实例持有
func (l *EtcdLock) IsLocked() bool {
	// 首先检查会话是否存活
	if l.manager.ctx.Err() != nil {
		return false
	}
	// 然后检查本地重入状态
	return l.reentrant.IsHeld(l.owner)
}

// GetLockKey 获取锁的键
func (l *EtcdLock) GetLockKey() string {
	return l.rawKey
}

// Done 返回一个channel，当锁被释放或丢失时会被关闭
func (l *EtcdLock) Done() <-chan struct{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.done == nil {
		// 如果锁还未获取，返回一个永远不会关闭的channel
		return make(chan struct{})
	}
	return l.done
}
