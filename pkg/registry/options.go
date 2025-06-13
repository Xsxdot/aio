package registry

import (
	"time"
)

// Options 注册中心配置选项
type Options struct {
	// 注册中心配置
	Prefix        string        `json:"prefix"`         // ETCD Key 前缀
	LeaseTTL      int64         `json:"lease_ttl"`      // 租约TTL(秒)
	KeepAlive     bool          `json:"keep_alive"`     // 是否启用租约续期
	RenewInterval time.Duration `json:"renew_interval"` // 续约间隔时间
	RetryTimes    int           `json:"retry_times"`    // 重试次数
	RetryDelay    time.Duration `json:"retry_delay"`    // 重试延迟
}

// DefaultOptions 返回默认配置
func DefaultOptions() *Options {
	return &Options{
		Prefix:        "/aio/registry",
		LeaseTTL:      30, // 30秒
		KeepAlive:     true,
		RenewInterval: 10 * time.Second, // 每10秒续约一次
		RetryTimes:    3,
		RetryDelay:    1 * time.Second,
	}
}

// Option 配置选项函数类型
type Option func(*Options)

// WithPrefix 设置Key前缀
func WithPrefix(prefix string) Option {
	return func(o *Options) {
		o.Prefix = prefix
	}
}

// WithLeaseTTL 设置租约TTL
func WithLeaseTTL(ttl int64) Option {
	return func(o *Options) {
		o.LeaseTTL = ttl
	}
}

// WithKeepAlive 设置是否启用租约续期
func WithKeepAlive(keepAlive bool) Option {
	return func(o *Options) {
		o.KeepAlive = keepAlive
	}
}

// WithRenewInterval 设置续约间隔
func WithRenewInterval(interval time.Duration) Option {
	return func(o *Options) {
		o.RenewInterval = interval
	}
}

// WithRetry 设置重试配置
func WithRetry(times int, delay time.Duration) Option {
	return func(o *Options) {
		o.RetryTimes = times
		o.RetryDelay = delay
	}
}

// Validate 验证配置有效性
func (o *Options) Validate() error {
	if o.Prefix == "" {
		return NewRegistryError(ErrCodeInvalidConfig, "prefix cannot be empty")
	}

	if o.LeaseTTL <= 0 {
		return NewRegistryError(ErrCodeInvalidConfig, "lease TTL must be positive")
	}

	if o.RenewInterval <= 0 {
		return NewRegistryError(ErrCodeInvalidConfig, "renew interval must be positive")
	}

	if o.RenewInterval >= time.Duration(o.LeaseTTL)*time.Second {
		return NewRegistryError(ErrCodeInvalidConfig, "renew interval must be less than lease TTL")
	}

	if o.RetryTimes < 0 {
		return NewRegistryError(ErrCodeInvalidConfig, "retry times cannot be negative")
	}

	if o.RetryDelay < 0 {
		return NewRegistryError(ErrCodeInvalidConfig, "retry delay cannot be negative")
	}

	return nil
}
