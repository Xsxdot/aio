package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

type rocketMQSubscription struct {
	topic   string
	group   string
	handler Handler
}

// rocketMQConsumer RocketMQ 消费者实现
type rocketMQConsumer struct {
	cfg       *RocketConfig
	subs      []*rocketMQSubscription
	consumers []rocketmq.PushConsumer
	started   bool
	closed    bool
	mu        sync.Mutex
}

// 编译时检查接口实现
var _ Consumer = (*rocketMQConsumer)(nil)

func newRocketMQConsumer(cfg *RocketConfig) (*rocketMQConsumer, error) {
	return &rocketMQConsumer{
		cfg: cfg,
	}, nil
}

// Subscribe 注册主题订阅，需在 Start 前调用
func (c *rocketMQConsumer) Subscribe(topic string, group string, handler Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return ErrAlreadyStarted
	}
	c.subs = append(c.subs, &rocketMQSubscription{
		topic:   topic,
		group:   group,
		handler: handler,
	})
	return nil
}

// Start 启动所有 push consumer
func (c *rocketMQConsumer) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return ErrAlreadyStarted
	}
	if len(c.subs) == 0 {
		return ErrNoSubscription
	}
	c.started = true

	retryTimes := c.cfg.RetryTimes
	if retryTimes <= 0 {
		retryTimes = 3
	}

	for _, sub := range c.subs {
		opts := []consumer.Option{
			consumer.WithNameServer(strings.Split(c.cfg.NameServer, ",")),
			consumer.WithGroupName(sub.group),
			consumer.WithConsumerModel(consumer.Clustering),
			consumer.WithRetry(retryTimes),
		}

		if c.cfg.AccessKey != "" && c.cfg.SecretKey != "" {
			opts = append(opts, consumer.WithCredentials(primitive.Credentials{
				AccessKey: c.cfg.AccessKey,
				SecretKey: c.cfg.SecretKey,
			}))
		}
		if c.cfg.Namespace != "" {
			opts = append(opts, consumer.WithNamespace(c.cfg.Namespace))
		}

		pc, err := rocketmq.NewPushConsumer(opts...)
		if err != nil {
			c.shutdownAll()
			return fmt.Errorf("mq: create rocketmq consumer for topic %s failed: %w", sub.topic, err)
		}

		handler := sub.handler
		err = pc.Subscribe(sub.topic, consumer.MessageSelector{
			Type:       consumer.TAG,
			Expression: "*",
		}, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
			for _, ext := range msgs {
				msg := parseRocketMQMessage(ext)
				result := handler(ctx, msg)
				if result == ConsumeRetry {
					return consumer.ConsumeRetryLater, nil
				}
			}
			return consumer.ConsumeSuccess, nil
		})
		if err != nil {
			c.shutdownAll()
			return fmt.Errorf("mq: subscribe topic %s failed: %w", sub.topic, err)
		}

		if err = pc.Start(); err != nil {
			c.shutdownAll()
			return fmt.Errorf("mq: start rocketmq consumer for topic %s failed: %w", sub.topic, err)
		}

		c.consumers = append(c.consumers, pc)
	}

	return nil
}

// Close 关闭所有消费者
func (c *rocketMQConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrAlreadyClosed
	}
	c.closed = true
	return c.shutdownAll()
}

func (c *rocketMQConsumer) shutdownAll() error {
	var lastErr error
	for _, pc := range c.consumers {
		if err := pc.Shutdown(); err != nil {
			lastErr = err
		}
	}
	c.consumers = nil
	return lastErr
}

// parseRocketMQMessage 将 RocketMQ 消息解析为统一 Message
func parseRocketMQMessage(ext *primitive.MessageExt) *Message {
	msg := &Message{
		Topic: ext.Topic,
		Body:  ext.Body,
		ID:    ext.MsgId,
	}
	if keys := ext.GetKeys(); keys != "" {
		msg.Key = keys
	}

	// 尝试解析自定义属性
	if propsStr := ext.GetProperty("mq_properties"); propsStr != "" {
		var props map[string]string
		if err := json.Unmarshal([]byte(propsStr), &props); err == nil {
			msg.Properties = props
		}
	}

	return msg
}
