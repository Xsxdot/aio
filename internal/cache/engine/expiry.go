// Package engine 提供缓存引擎实现
package engine

import (
	"sync"
	"time"
)

// ExpiryPolicy 过期策略接口
type ExpiryPolicy interface {
	// SetExpiry 设置键的过期时间
	SetExpiry(key string, expiration time.Duration) bool

	// GetExpiry 获取键的过期时间
	GetExpiry(key string) (time.Time, bool)

	// RemoveExpiry 移除键的过期时间
	RemoveExpiry(key string)

	// IsExpired 检查键是否过期
	IsExpired(key string) bool

	// CleanExpiredKeys 清理过期的键
	CleanExpiredKeys(deleteCallback func(string))

	// GetExpiryMap 获取过期时间映射（用于持久化）
	GetExpiryMap() map[string]time.Time

	// LoadExpiryMap 加载过期时间映射（用于持久化）
	LoadExpiryMap(expires map[string]time.Time)
}

// DefaultExpiryPolicy 默认过期策略实现
type DefaultExpiryPolicy struct {
	expires     map[string]time.Time
	mutex       sync.RWMutex
	lastCleanup time.Time

	// 配置项
	cleanupInterval time.Duration // 清理间隔
	sampleSize      int           // 随机采样大小
	maxCleanup      int           // 单次最大清理数量
}

// NewDefaultExpiryPolicy 创建默认过期策略
func NewDefaultExpiryPolicy() *DefaultExpiryPolicy {
	return &DefaultExpiryPolicy{
		expires:         make(map[string]time.Time),
		lastCleanup:     time.Now(),
		cleanupInterval: 100 * time.Millisecond, // 默认100ms清理一次
		sampleSize:      20,                     // 默认随机采样20个键
		maxCleanup:      200,                    // 默认单次最多清理200个键
	}
}

// SetExpiry 设置键的过期时间
func (p *DefaultExpiryPolicy) SetExpiry(key string, expiration time.Duration) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if expiration > 0 {
		p.expires[key] = time.Now().Add(expiration)
		return true
	} else if expiration == 0 {
		// 如果过期时间为0，则移除过期时间
		delete(p.expires, key)
	}
	return false
}

// GetExpiry 获取键的过期时间
func (p *DefaultExpiryPolicy) GetExpiry(key string) (time.Time, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	expireTime, hasExpiry := p.expires[key]
	return expireTime, hasExpiry
}

// RemoveExpiry 移除键的过期时间
func (p *DefaultExpiryPolicy) RemoveExpiry(key string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	delete(p.expires, key)
}

// IsExpired 检查键是否过期
func (p *DefaultExpiryPolicy) IsExpired(key string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	expireTime, hasExpiry := p.expires[key]
	return hasExpiry && time.Now().After(expireTime)
}

// CleanExpiredKeys 清理过期的键
func (p *DefaultExpiryPolicy) CleanExpiredKeys(deleteCallback func(string)) {
	now := time.Now()

	// 检查是否需要清理
	if now.Sub(p.lastCleanup) < p.cleanupInterval {
		return
	}

	p.lastCleanup = now

	// 随机采样策略，避免一次性扫描所有键
	p.mutex.RLock()
	keysCount := len(p.expires)
	if keysCount == 0 {
		p.mutex.RUnlock()
		return
	}

	// 确定采样大小
	sampleSize := p.sampleSize
	if keysCount < sampleSize {
		sampleSize = keysCount
	}

	// 随机选择键进行检查
	keys := make([]string, 0, sampleSize)
	i := 0
	for key := range p.expires {
		if i >= sampleSize {
			break
		}
		keys = append(keys, key)
		i++
	}
	p.mutex.RUnlock()

	// 检查采样的键是否过期
	expiredKeys := make([]string, 0, sampleSize)
	for _, key := range keys {
		if p.IsExpired(key) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 删除过期的键
	if len(expiredKeys) > 0 {
		p.mutex.Lock()
		for _, key := range expiredKeys {
			// 再次检查，避免竞态条件
			if expireTime, hasExpiry := p.expires[key]; hasExpiry && now.After(expireTime) {
				delete(p.expires, key)
				// 调用回调函数删除实际数据
				if deleteCallback != nil {
					deleteCallback(key)
				}
			}
		}
		p.mutex.Unlock()
	}

	// 如果过期键比例较高，进行更大规模的清理
	if len(expiredKeys) > sampleSize/2 {
		p.activeCleanup(now, deleteCallback)
	}
}

// activeCleanup 主动清理过期键
func (p *DefaultExpiryPolicy) activeCleanup(now time.Time, deleteCallback func(string)) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 限制单次清理数量
	count := 0
	for key, expireTime := range p.expires {
		if count >= p.maxCleanup {
			break
		}

		if now.After(expireTime) {
			delete(p.expires, key)
			// 调用回调函数删除实际数据
			if deleteCallback != nil {
				deleteCallback(key)
			}
			count++
		}
	}
}

// GetExpiryMap 获取过期时间映射
func (p *DefaultExpiryPolicy) GetExpiryMap() map[string]time.Time {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 创建副本
	expires := make(map[string]time.Time, len(p.expires))
	for k, v := range p.expires {
		expires[k] = v
	}

	return expires
}

// LoadExpiryMap 加载过期时间映射
func (p *DefaultExpiryPolicy) LoadExpiryMap(expires map[string]time.Time) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 清空现有数据
	p.expires = make(map[string]time.Time, len(expires))

	// 加载新数据
	for k, v := range expires {
		p.expires[k] = v
	}
}

// LRUExpiryPolicy LRU过期策略实现
// 可以在此基础上实现更复杂的LRU过期策略
type LRUExpiryPolicy struct {
	*DefaultExpiryPolicy
	// 可以添加LRU相关的字段
}

// TTLExpiryPolicy TTL过期策略实现
// 可以在此基础上实现更复杂的TTL过期策略
type TTLExpiryPolicy struct {
	*DefaultExpiryPolicy
	// 可以添加TTL相关的字段
}
