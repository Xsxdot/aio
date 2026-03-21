package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultBlockTimeout       = 5 * time.Second
	defaultPendingRetryInterval = 30 * time.Second
)

type subscription struct {
	topic   string
	group   string
	handler Handler
}

// redisConsumer Redis Stream 消费者实现
type redisConsumer struct {
	client              *redis.Client
	blockTimeout        time.Duration
	pendingRetryInterval time.Duration
	managed             bool // true 表示 client 是由 mq 创建的，Close 时需要释放
	subs                []*subscription
	started             bool
	closed              bool
	mu                  sync.Mutex
	stopCh              chan struct{}
	wg                  sync.WaitGroup
}

// 编译时检查接口实现
var _ Consumer = (*redisConsumer)(nil)

func newRedisConsumer(cfg *RedisConfig) (*redisConsumer, error) {
	var client *redis.Client
	var managed bool

	if cfg.Client != nil {
		client = cfg.Client
		managed = false
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		})
		managed = true
	}

	blockTimeout := defaultBlockTimeout
	if cfg.BlockTimeoutMs > 0 {
		blockTimeout = time.Duration(cfg.BlockTimeoutMs) * time.Millisecond
	}

	pendingRetry := defaultPendingRetryInterval
	if cfg.PendingRetryIntervalSec > 0 {
		pendingRetry = time.Duration(cfg.PendingRetryIntervalSec) * time.Second
	}

	return &redisConsumer{
		client:              client,
		blockTimeout:        blockTimeout,
		pendingRetryInterval: pendingRetry,
		managed:             managed,
		stopCh:              make(chan struct{}),
	}, nil
}

// Subscribe 注册主题订阅，需在 Start 前调用
func (c *redisConsumer) Subscribe(topic string, group string, handler Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return ErrAlreadyStarted
	}
	c.subs = append(c.subs, &subscription{
		topic:   topic,
		group:   group,
		handler: handler,
	})
	return nil
}

// Start 启动消费者
func (c *redisConsumer) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return ErrAlreadyStarted
	}
	if len(c.subs) == 0 {
		return ErrNoSubscription
	}
	c.started = true

	// 为每个订阅创建 consumer group 并启动消费协程
	ctx := context.Background()
	for _, sub := range c.subs {
		// 创建 consumer group（如已存在则忽略）
		c.client.XGroupCreateMkStream(ctx, sub.topic, sub.group, "0").Err()

		c.wg.Add(2)
		go c.consumeLoop(sub)
		go c.reclaimPendingLoop(sub)
	}

	return nil
}

// Close 关闭消费者
func (c *redisConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrAlreadyClosed
	}
	c.closed = true
	close(c.stopCh)
	c.wg.Wait()

	if c.managed {
		return c.client.Close()
	}
	return nil
}

// consumeLoop 消费循环：XREADGROUP 拉取新消息
func (c *redisConsumer) consumeLoop(sub *subscription) {
	defer c.wg.Done()

	consumerName := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		streams, err := c.client.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Group:    sub.group,
			Consumer: consumerName,
			Streams:  []string{sub.topic, ">"},
			Count:    10,
			Block:    c.blockTimeout,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			// 短暂等待后重试
			select {
			case <-c.stopCh:
				return
			case <-time.After(time.Second):
				continue
			}
		}

		for _, stream := range streams {
			for _, xMsg := range stream.Messages {
				msg := parseRedisMessage(sub.topic, xMsg)
				result := sub.handler(context.Background(), msg)
				if result == ConsumeSuccess {
					c.client.XAck(context.Background(), sub.topic, sub.group, xMsg.ID)
				}
				// ConsumeRetry: 不 ACK，消息将在 pending 列表中等待重试
			}
		}
	}
}

// reclaimPendingLoop 定期处理 pending 中的超时消息
func (c *redisConsumer) reclaimPendingLoop(sub *subscription) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.pendingRetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.processPending(sub)
		}
	}
}

// processPending 处理超时的 pending 消息
func (c *redisConsumer) processPending(sub *subscription) {
	ctx := context.Background()

	// 获取 pending 列表中的消息
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: sub.topic,
		Group:  sub.group,
		Start:  "-",
		End:    "+",
		Count:  50,
		Idle:   c.pendingRetryInterval,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	consumerName := fmt.Sprintf("reclaimer-%d", time.Now().UnixNano())

	// 收集需要 claim 的消息 ID
	ids := make([]string, 0, len(pending))
	for _, p := range pending {
		ids = append(ids, p.ID)
	}

	// XCLAIM 这些消息
	messages, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   sub.topic,
		Group:    sub.group,
		Consumer: consumerName,
		MinIdle:  c.pendingRetryInterval,
		Messages: ids,
	}).Result()
	if err != nil {
		return
	}

	for _, xMsg := range messages {
		msg := parseRedisMessage(sub.topic, xMsg)
		result := sub.handler(ctx, msg)
		if result == ConsumeSuccess {
			c.client.XAck(ctx, sub.topic, sub.group, xMsg.ID)
		}
	}
}

// parseRedisMessage 将 Redis Stream 消息解析为 Message
func parseRedisMessage(topic string, xMsg redis.XMessage) *Message {
	msg := &Message{
		Topic: topic,
		ID:    xMsg.ID,
	}

	if body, ok := xMsg.Values["body"]; ok {
		if s, ok := body.(string); ok {
			msg.Body = []byte(s)
		}
	}
	if key, ok := xMsg.Values["key"]; ok {
		if s, ok := key.(string); ok {
			msg.Key = s
		}
	}
	if props, ok := xMsg.Values["properties"]; ok {
		if s, ok := props.(string); ok {
			var properties map[string]string
			if err := json.Unmarshal([]byte(s), &properties); err == nil {
				msg.Properties = properties
			}
		}
	}

	return msg
}
