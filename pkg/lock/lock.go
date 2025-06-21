package lock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DistributedLock 分布式锁接口
type DistributedLock interface {
	// Lock 获取锁
	Lock(ctx context.Context) error

	// TryLock 尝试获取锁，不阻塞
	TryLock(ctx context.Context) (bool, error)

	// LockWithTimeout 带超时的获取锁
	LockWithTimeout(ctx context.Context, timeout time.Duration) error

	// Unlock 释放锁
	Unlock(ctx context.Context) error

	// IsLocked 检查锁是否被当前实例持有
	IsLocked() bool

	// GetLockKey 获取锁的键
	GetLockKey() string

	// Done 返回一个channel，当锁被释放或丢失时会被关闭
	Done() <-chan struct{}
}

// LockOptions 锁配置选项
type LockOptions struct {
	// RetryInterval 重试间隔
	RetryInterval time.Duration

	// MaxRetries 最大重试次数，0表示无限重试
	MaxRetries int
}

// DefaultLockOptions 默认锁配置
func DefaultLockOptions() *LockOptions {
	return &LockOptions{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    0,
	}
}

// LockManagerOptions 锁管理器配置
type LockManagerOptions struct {
	// TTL 锁的生存时间
	TTL time.Duration
}

// DefaultLockManagerOptions 默认管理器配置
func DefaultLockManagerOptions() *LockManagerOptions {
	return &LockManagerOptions{
		TTL: 30 * time.Second,
	}
}

// LockInfo 锁信息
type LockInfo struct {
	Key        string    `json:"key"`
	Owner      string    `json:"owner"`
	CreateTime time.Time `json:"create_time"`
	Version    int64     `json:"version"`
}

// LockManager 锁管理器接口
type LockManager interface {
	// NewLock 创建新的分布式锁
	NewLock(key string, opts *LockOptions) DistributedLock

	// GetLockInfo 获取锁信息
	GetLockInfo(ctx context.Context, key string) (*LockInfo, error)

	// ListLocks 列出所有锁
	ListLocks(ctx context.Context, prefix string) ([]*LockInfo, error)

	// ForceUnlock 强制释放锁（管理员操作）
	ForceUnlock(ctx context.Context, key string) error

	// Close 关闭锁管理器
	Close() error
}

// LockError 锁相关错误
type LockError struct {
	Code    string
	Message string
	Cause   error
}

func (e *LockError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("锁错误 [%s]: %s, 原因: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("锁错误 [%s]: %s", e.Code, e.Message)
}

// Unwrap 支持Go 1.13+的错误包装
func (e *LockError) Unwrap() error {
	return e.Cause
}

// 预定义错误代码
const (
	ErrCodeLockTimeout     = "LOCK_TIMEOUT"
	ErrCodeLockNotHeld     = "LOCK_NOT_HELD"
	ErrCodeLockAlreadyHeld = "LOCK_ALREADY_HELD"
	ErrCodeLockExpired     = "LOCK_EXPIRED"
	ErrCodeInvalidKey      = "INVALID_KEY"
)

// NewLockError 创建锁错误
func NewLockError(code, message string, cause error) *LockError {
	return &LockError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// ReentrantCounter 可重入计数器
type ReentrantCounter struct {
	mu    sync.RWMutex
	count int
	owner string
}

// NewReentrantCounter 创建可重入计数器
func NewReentrantCounter() *ReentrantCounter {
	return &ReentrantCounter{}
}

// TryEnter 尝试进入
func (rc *ReentrantCounter) TryEnter(owner string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.count == 0 || rc.owner == owner {
		rc.count++
		rc.owner = owner
		return true
	}
	return false
}

// Exit 退出
func (rc *ReentrantCounter) Exit(owner string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.owner == owner && rc.count > 0 {
		rc.count--
		if rc.count == 0 {
			rc.owner = ""
		}
		return true
	}
	return false
}

// IsHeld 是否被持有
func (rc *ReentrantCounter) IsHeld(owner string) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.count > 0 && rc.owner == owner
}

// GetCount 获取计数
func (rc *ReentrantCounter) GetCount() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.count
}

// GetOwner 获取拥有者
func (rc *ReentrantCounter) GetOwner() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.owner
}
