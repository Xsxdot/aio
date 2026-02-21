package executor

import "time"

// Config Executor 配置
type Config struct {
	// QueueSize 队列大小（待执行任务数上限）
	QueueSize int `json:"queue_size"`

	// MaxHistorySize 历史记录保留上限（超过后会丢弃最旧的已完成/失败任务）
	MaxHistorySize int `json:"max_history_size"`

	// DefaultTimeout 任务默认超时时间（如果 Submit 时未指定）
	DefaultTimeout time.Duration `json:"default_timeout"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		QueueSize:      100,              // 队列最多 100 个待执行任务
		MaxHistorySize: 1000,             // 最多保留 1000 个历史记录
		DefaultTimeout: 10 * time.Minute, // 默认超时 10 分钟
	}
}

