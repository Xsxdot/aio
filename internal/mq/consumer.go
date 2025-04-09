package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// MsgHandler 是处理消息的回调函数类型
type MsgHandler func(msg *Message) error

// Message 代表一个消息
type Message struct {
	// 消息主题
	Subject string
	// 消息回复主题（如果有）
	Reply string
	// 消息数据
	Data []byte
	// 原始NATS消息
	Msg *nats.Msg
}

// Consumer 是消息消费者接口
type Consumer interface {
	// Subscribe 订阅指定主题
	Subscribe(subject string, handler MsgHandler) (string, error)
	// QueueSubscribe 创建队列订阅
	QueueSubscribe(subject, queue string, handler MsgHandler) (string, error)
	// JetStreamSubscribe 创建JetStream订阅（仅JetStream支持）
	JetStreamSubscribe(subject string, handler MsgHandler, opts ...nats.SubOpt) (string, error)
	// JetStreamQueueSubscribe 创建JetStream队列订阅（仅JetStream支持）
	JetStreamQueueSubscribe(subject, queue string, handler MsgHandler, opts ...nats.SubOpt) (string, error)
	// Unsubscribe 取消订阅
	Unsubscribe(subscriptionID string) error
	// Close 关闭消费者
	Close() error
}

// DefaultConsumer 默认的消息消费者实现
type DefaultConsumer struct {
	client        *NatsClient
	logger        *zap.Logger
	subscriptions map[string]*nats.Subscription
	mu            sync.RWMutex
}

// NewConsumer 创建一个新的消息消费者
func NewConsumer(client *NatsClient, logger *zap.Logger) (Consumer, error) {
	if client == nil {
		return nil, errors.New("NATS客户端不能为空")
	}

	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("创建日志记录器失败: %v", err)
		}
	}

	return &DefaultConsumer{
		client:        client,
		logger:        logger,
		subscriptions: make(map[string]*nats.Subscription),
	}, nil
}

// Subscribe 订阅指定主题
func (c *DefaultConsumer) Subscribe(subject string, handler MsgHandler) (string, error) {
	if c.client.conn == nil {
		return "", ErrNotConnected
	}

	// 创建NATS消息处理函数
	natsHandler := c.createMessageHandler(handler)

	// 订阅主题
	sub, err := c.client.conn.Subscribe(subject, natsHandler)
	if err != nil {
		return "", fmt.Errorf("订阅主题失败: %v", err)
	}

	// 保存订阅信息
	subscriptionID := fmt.Sprintf("%s-%p", subject, sub)
	c.saveSubscription(subscriptionID, sub)

	c.logger.Info("已订阅主题",
		zap.String("subject", subject),
		zap.String("subscription_id", subscriptionID),
	)

	return subscriptionID, nil
}

// QueueSubscribe 创建队列订阅
func (c *DefaultConsumer) QueueSubscribe(subject, queue string, handler MsgHandler) (string, error) {
	if c.client.conn == nil {
		return "", ErrNotConnected
	}

	// 创建NATS消息处理函数
	natsHandler := c.createMessageHandler(handler)

	// 创建队列订阅
	sub, err := c.client.conn.QueueSubscribe(subject, queue, natsHandler)
	if err != nil {
		return "", fmt.Errorf("创建队列订阅失败: %v", err)
	}

	// 保存订阅信息
	subscriptionID := fmt.Sprintf("%s-%s-%p", subject, queue, sub)
	c.saveSubscription(subscriptionID, sub)

	c.logger.Info("已创建队列订阅",
		zap.String("subject", subject),
		zap.String("queue", queue),
		zap.String("subscription_id", subscriptionID),
	)

	return subscriptionID, nil
}

// JetStreamSubscribe 创建JetStream订阅
func (c *DefaultConsumer) JetStreamSubscribe(subject string, handler MsgHandler, opts ...nats.SubOpt) (string, error) {
	if c.client.js == nil {
		return "", errors.New("JetStream未启用")
	}

	// 创建NATS消息处理函数
	natsHandler := c.createMessageHandler(handler)

	// 创建JetStream订阅
	sub, err := c.client.js.Subscribe(subject, natsHandler, opts...)
	if err != nil {
		return "", fmt.Errorf("创建JetStream订阅失败: %v", err)
	}

	// 保存订阅信息
	consumerInfo, _ := sub.ConsumerInfo()
	var consumerName string
	if consumerInfo != nil {
		consumerName = consumerInfo.Name
	} else {
		consumerName = fmt.Sprintf("consumer-%p", sub)
	}

	subscriptionID := fmt.Sprintf("js-%s-%s", subject, consumerName)
	c.saveSubscription(subscriptionID, sub)

	c.logger.Info("已创建JetStream订阅",
		zap.String("subject", subject),
		zap.String("consumer", consumerName),
		zap.String("subscription_id", subscriptionID),
	)

	return subscriptionID, nil
}

// JetStreamQueueSubscribe 创建JetStream队列订阅
func (c *DefaultConsumer) JetStreamQueueSubscribe(subject, queue string, handler MsgHandler, opts ...nats.SubOpt) (string, error) {
	if c.client.js == nil {
		return "", errors.New("JetStream未启用")
	}

	// 创建NATS消息处理函数
	natsHandler := c.createMessageHandler(handler)

	// 创建JetStream队列订阅
	sub, err := c.client.js.QueueSubscribe(subject, queue, natsHandler, opts...)
	if err != nil {
		return "", fmt.Errorf("创建JetStream队列订阅失败: %v", err)
	}

	// 保存订阅信息
	consumerInfo, _ := sub.ConsumerInfo()
	var consumerName string
	if consumerInfo != nil {
		consumerName = consumerInfo.Name
	} else {
		consumerName = fmt.Sprintf("queue-%s-%p", queue, sub)
	}

	subscriptionID := fmt.Sprintf("js-%s-%s-%s", subject, queue, consumerName)
	c.saveSubscription(subscriptionID, sub)

	c.logger.Info("已创建JetStream队列订阅",
		zap.String("subject", subject),
		zap.String("queue", queue),
		zap.String("consumer", consumerName),
		zap.String("subscription_id", subscriptionID),
	)

	return subscriptionID, nil
}

// Unsubscribe 取消订阅
func (c *DefaultConsumer) Unsubscribe(subscriptionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sub, exists := c.subscriptions[subscriptionID]
	if !exists {
		return fmt.Errorf("订阅 %s 不存在", subscriptionID)
	}

	// 取消订阅
	err := sub.Unsubscribe()
	if err != nil {
		c.logger.Warn("取消订阅失败",
			zap.String("subscription_id", subscriptionID),
			zap.Error(err),
		)
		// 继续处理，将订阅从映射中移除
	}

	// 移除订阅信息
	delete(c.subscriptions, subscriptionID)

	c.logger.Info("已取消订阅", zap.String("subscription_id", subscriptionID))
	return nil
}

// Close 关闭消费者
func (c *DefaultConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 取消所有订阅
	for id, sub := range c.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			c.logger.Warn("取消订阅失败",
				zap.String("subscription_id", id),
				zap.Error(err),
			)
			// 继续取消其他订阅
		}
	}

	// 清空订阅映射
	c.subscriptions = make(map[string]*nats.Subscription)

	c.logger.Info("消费者已关闭")
	return nil
}

// 创建消息处理函数
func (c *DefaultConsumer) createMessageHandler(handler MsgHandler) nats.MsgHandler {
	return func(msg *nats.Msg) {
		// 创建消息对象
		message := &Message{
			Subject: msg.Subject,
			Reply:   msg.Reply,
			Data:    msg.Data,
			Msg:     msg,
		}

		// 处理消息
		start := time.Now()

		// 执行用户处理函数
		err := handler(message)

		// 记录处理结果
		duration := time.Since(start)
		if err != nil {
			c.logger.Warn("处理消息失败",
				zap.String("subject", msg.Subject),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		} else {
			c.logger.Debug("处理消息成功",
				zap.String("subject", msg.Subject),
				zap.Duration("duration", duration),
			)

			// 尝试确认消息（如果是JetStream消息）
			// 由于NATS库限制，我们无法直接判断消息是否来自JetStream
			// 因此我们尝试确认所有消息，忽略可能的错误
			_ = msg.Ack()
		}
	}
}

// 保存订阅信息
func (c *DefaultConsumer) saveSubscription(id string, sub *nats.Subscription) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[id] = sub
}

// UnmarshalJson 解析JSON格式的消息
func UnmarshalJson(msg *Message, v interface{}) error {
	return json.Unmarshal(msg.Data, v)
}

// ReplyJson 回复JSON格式的消息
func ReplyJson(msg *Message, data interface{}) error {
	if msg.Reply == "" {
		return errors.New("消息没有回复主题")
	}

	// 序列化数据
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化回复数据失败: %v", err)
	}

	// 发送回复
	return msg.Msg.Respond(payload)
}

// WithDurable 创建持久化订阅的选项
func WithDurable(name string) nats.SubOpt {
	return nats.Durable(name)
}

// WithDeliverNew 从最新消息开始订阅的选项
func WithDeliverNew() nats.SubOpt {
	return nats.DeliverNew()
}

// WithDeliverAll 从所有存储的消息开始订阅的选项
func WithDeliverAll() nats.SubOpt {
	return nats.DeliverAll()
}

// WithStartSequence 从指定序列号开始订阅的选项
func WithStartSequence(seq uint64) nats.SubOpt {
	return nats.StartSequence(seq)
}

// WithStartTime 从指定时间开始订阅的选项
func WithStartTime(startTime time.Time) nats.SubOpt {
	return nats.StartTime(startTime)
}

// WithAckWait 设置确认等待时间的选项
func WithAckWait(wait time.Duration) nats.SubOpt {
	return nats.AckWait(wait)
}

// WithMaxDeliver 设置最大重试次数的选项
func WithMaxDeliver(maxDeliver int) nats.SubOpt {
	return nats.MaxDeliver(maxDeliver)
}

// WithManualAck 设置手动确认的选项
func WithManualAck() nats.SubOpt {
	return nats.ManualAck()
}
