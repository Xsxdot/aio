package mq

import "github.com/redis/go-redis/v9"

// Config 消息队列统一配置
type Config struct {
	Driver   DriverType    `yaml:"driver" json:"driver"`     // 驱动类型: redis / rocketmq
	Redis    RedisConfig   `yaml:"redis" json:"redis"`       // Redis Stream 配置
	RocketMQ RocketConfig  `yaml:"rocketmq" json:"rocketmq"` // RocketMQ 配置
}

// RedisConfig Redis Stream 配置
type RedisConfig struct {
	Addr     string `yaml:"addr" json:"addr"`         // Redis 地址，如 127.0.0.1:6379
	Password string `yaml:"password" json:"password"` // 密码
	DB       int    `yaml:"db" json:"db"`             // 数据库编号
	Client   *redis.Client `yaml:"-" json:"-"`        // 支持外部直接传入 redis 客户端，若传入则优先使用且 Close 时不关闭

	// 延时消息轮询间隔（毫秒），默认 500
	DelayPollIntervalMs int `yaml:"delay-poll-interval-ms" json:"delay-poll-interval-ms"`
	// 消费者拉取超时（毫秒），默认 5000
	BlockTimeoutMs int `yaml:"block-timeout-ms" json:"block-timeout-ms"`
	// Pending 消息重试间隔（秒），默认 30
	PendingRetryIntervalSec int `yaml:"pending-retry-interval-sec" json:"pending-retry-interval-sec"`
}

// RocketConfig RocketMQ 配置
type RocketConfig struct {
	NameServer  string `yaml:"name-server" json:"name-server"`   // NameServer 地址，多个用逗号分隔
	AccessKey   string `yaml:"access-key" json:"access-key"`     // AccessKey
	SecretKey   string `yaml:"secret-key" json:"secret-key"`     // SecretKey
	Namespace   string `yaml:"namespace" json:"namespace"`       // 命名空间
	GroupName   string `yaml:"group-name" json:"group-name"`     // 生产者/消费者 组名
	RetryTimes  int    `yaml:"retry-times" json:"retry-times"`   // 重试次数，默认 3
}
