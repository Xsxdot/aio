package mq

import (
	"context"
	"errors"
	"time"
)

// 标准错误
var (
	// ErrNotSupported 当前驱动不支持该操作
	ErrNotSupported = errors.New("mq: operation not supported by current driver")
	// ErrAlreadyClosed 客户端已关闭
	ErrAlreadyClosed = errors.New("mq: client already closed")
	// ErrAlreadyStarted 消费者已启动
	ErrAlreadyStarted = errors.New("mq: consumer already started")
	// ErrNoSubscription 消费者未订阅任何主题
	ErrNoSubscription = errors.New("mq: no subscription registered")
)

// DriverType 驱动类型
type DriverType string

const (
	DriverRedis    DriverType = "redis"
	DriverRocketMQ DriverType = "rocketmq"
)

// Message 消息
type Message struct {
	Topic      string            // 主题
	Body       []byte            // 消息体
	Key        string            // 业务Key（可选）
	Properties map[string]string // 自定义属性（可选）
	ID         string            // 消息ID（消费时由框架填充）
}

// SendResult 发送结果
type SendResult struct {
	MessageID string
}

// ConsumeResult 消费结果
type ConsumeResult int

const (
	ConsumeSuccess ConsumeResult = iota // 消费成功
	ConsumeRetry                        // 稍后重试
)

// ConsumeMode 消费模式
type ConsumeMode int

const (
	ConsumeModeCluster   ConsumeMode = iota // 集群消费（默认）：同一组内消费者分担消息
	ConsumeModeBroadcast                    // 广播消费：组内每个消费者都收到全量消息
)

// Handler 消费处理函数
type Handler func(ctx context.Context, msg *Message) ConsumeResult

// Producer 生产者接口
type Producer interface {
	// SendMessage 发送普通消息
	SendMessage(ctx context.Context, msg *Message) (*SendResult, error)
	// SendDelayMessage 发送延时消息，delay 为延迟时长
	SendDelayMessage(ctx context.Context, msg *Message, delay time.Duration) (*SendResult, error)
	// SendOrderMessage 发送顺序消息，shardingKey 用于保证相同 key 的消息顺序消费
	SendOrderMessage(ctx context.Context, msg *Message, shardingKey string) (*SendResult, error)
	// Close 关闭生产者客户端，释放资源
	Close() error
}

// Consumer 消费者接口
type Consumer interface {
	// Subscribe 订阅主题。group 为消费组名称，mode 为消费模式
	Subscribe(topic string, group string, mode ConsumeMode, handler Handler) error
	// Start 启动消费者，开始拉取消息。调用前需先 Subscribe
	Start() error
	// Close 关闭消费者，停止消费并释放资源
	Close() error
}

// SubscribeCluster 便捷函数：以集群模式订阅主题
func SubscribeCluster(c Consumer, topic string, group string, handler Handler) error {
	return c.Subscribe(topic, group, ConsumeModeCluster, handler)
}

// SubscribeBroadcast 便捷函数：以广播模式订阅主题
func SubscribeBroadcast(c Consumer, topic string, group string, handler Handler) error {
	return c.Subscribe(topic, group, ConsumeModeBroadcast, handler)
}
