package mq

import "fmt"

// NewProducer 根据配置创建对应驱动的生产者
func NewProducer(cfg *Config) (Producer, error) {
	switch cfg.Driver {
	case DriverRedis:
		return newRedisProducer(&cfg.Redis)
	case DriverRocketMQ:
		return newRocketMQProducer(&cfg.RocketMQ)
	default:
		return nil, fmt.Errorf("mq: unsupported driver %q", cfg.Driver)
	}
}

// NewConsumer 根据配置创建对应驱动的消费者
func NewConsumer(cfg *Config) (Consumer, error) {
	switch cfg.Driver {
	case DriverRedis:
		return newRedisConsumer(&cfg.Redis)
	case DriverRocketMQ:
		return newRocketMQConsumer(&cfg.RocketMQ)
	default:
		return nil, fmt.Errorf("mq: unsupported driver %q", cfg.Driver)
	}
}
