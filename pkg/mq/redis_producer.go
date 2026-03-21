package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultDelayPollInterval = 500 * time.Millisecond
	delayZSetSuffix          = ":mq:delay"
)

// redisProducer Redis Stream 生产者实现
type redisProducer struct {
	client       *redis.Client
	pollInterval time.Duration
	closed       bool
	managed      bool // true 表示 client 是由 mq 创建的，Close 时需要释放
	mu           sync.Mutex
	stopCh       chan struct{}
	wg           sync.WaitGroup
	// delayTopics 记录已开启延时轮询的 topic，避免重复启动协程
	delayTopics map[string]struct{}
}

// 编译时检查接口实现
var _ Producer = (*redisProducer)(nil)

func newRedisProducer(cfg *RedisConfig) (*redisProducer, error) {
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

	pollInterval := defaultDelayPollInterval
	if cfg.DelayPollIntervalMs > 0 {
		pollInterval = time.Duration(cfg.DelayPollIntervalMs) * time.Millisecond
	}

	return &redisProducer{
		client:       client,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
		delayTopics:  make(map[string]struct{}),
		managed:      managed,
	}, nil
}

// SendMessage 普通消息 — XADD 到 stream
func (p *redisProducer) SendMessage(ctx context.Context, msg *Message) (*SendResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrAlreadyClosed
	}
	p.mu.Unlock()

	values := buildRedisValues(msg)
	id, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: msg.Topic,
		ID:     "*",
		Values: values,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("mq: redis XADD failed: %w", err)
	}
	return &SendResult{MessageID: id}, nil
}

// SendDelayMessage 延时消息 — 写入 sorted set，后台轮询到期后 XADD
func (p *redisProducer) SendDelayMessage(ctx context.Context, msg *Message, delay time.Duration) (*SendResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrAlreadyClosed
	}
	p.mu.Unlock()

	// 将消息序列化后写入 sorted set，score 为触发时间戳
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("mq: marshal delay message failed: %w", err)
	}

	msgID := uuid.New().String()
	member := msgID + "|" + string(payload)
	score := float64(time.Now().Add(delay).UnixMilli())

	zsetKey := msg.Topic + delayZSetSuffix
	err = p.client.ZAdd(ctx, zsetKey, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
	if err != nil {
		return nil, fmt.Errorf("mq: redis ZADD failed: %w", err)
	}

	// 确保该 topic 的延时轮询协程已启动
	p.ensureDelayPoller(msg.Topic)

	return &SendResult{MessageID: msgID}, nil
}

// SendOrderMessage 顺序消息 — Redis Stream 单 stream 天然保序，直接 XADD
func (p *redisProducer) SendOrderMessage(ctx context.Context, msg *Message, shardingKey string) (*SendResult, error) {
	// Redis Stream 天然保证单 stream 内消息有序
	// 为保证相同 shardingKey 的消息在同一 stream 中，将 shardingKey 追加到 topic 作为后缀
	originalTopic := msg.Topic
	msg.Topic = msg.Topic + ":order:" + shardingKey
	result, err := p.SendMessage(ctx, msg)
	msg.Topic = originalTopic // 还原
	return result, err
}

// Close 关闭生产者
func (p *redisProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrAlreadyClosed
	}
	p.closed = true
	close(p.stopCh)
	p.wg.Wait()

	if p.managed {
		return p.client.Close()
	}
	return nil
}

// ensureDelayPoller 确保指定 topic 的延时轮询协程已启动
func (p *redisProducer) ensureDelayPoller(topic string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.delayTopics[topic]; ok {
		return
	}
	p.delayTopics[topic] = struct{}{}

	p.wg.Add(1)
	go p.pollDelayMessages(topic)
}

// pollDelayMessages 轮询 sorted set 中到期的延时消息并 XADD 到 stream
func (p *redisProducer) pollDelayMessages(topic string) {
	defer p.wg.Done()

	zsetKey := topic + delayZSetSuffix
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.transferDueMessages(zsetKey, topic)
		}
	}
}

// transferDueMessages 将到期的延时消息从 sorted set 转移到 stream
func (p *redisProducer) transferDueMessages(zsetKey, topic string) {
	ctx := context.Background()
	now := float64(time.Now().UnixMilli())

	// 获取所有到期消息
	results, err := p.client.ZRangeByScore(ctx, zsetKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatFloat(now, 'f', 0, 64),
	}).Result()
	if err != nil || len(results) == 0 {
		return
	}

	for _, member := range results {
		// 尝试原子性地移除该成员（ZREM 返回移除数量，0 表示已被其他实例处理）
		removed, err := p.client.ZRem(ctx, zsetKey, member).Result()
		if err != nil || removed == 0 {
			continue
		}

		// 解析消息
		// 格式: msgID|jsonPayload
		idx := -1
		for i, c := range member {
			if c == '|' {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(member[idx+1:]), &msg); err != nil {
			continue
		}

		// XADD 到目标 stream
		values := buildRedisValues(&msg)
		p.client.XAdd(ctx, &redis.XAddArgs{
			Stream: topic,
			ID:     "*",
			Values: values,
		})
	}
}

// buildRedisValues 将 Message 转换为 Redis Stream 的 key-value 对
func buildRedisValues(msg *Message) map[string]interface{} {
	values := map[string]interface{}{
		"body": string(msg.Body),
	}
	if msg.Key != "" {
		values["key"] = msg.Key
	}
	if len(msg.Properties) > 0 {
		if data, err := json.Marshal(msg.Properties); err == nil {
			values["properties"] = string(data)
		}
	}
	return values
}
