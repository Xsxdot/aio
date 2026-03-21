package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// delayLevelMap RocketMQ 延时级别对应的时长
// 1s 5s 10s 30s 1m 2m 3m 4m 5m 6m 7m 8m 9m 10m 20m 30m 1h 2h
var delayLevelDurations = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
	3 * time.Minute,
	4 * time.Minute,
	5 * time.Minute,
	6 * time.Minute,
	7 * time.Minute,
	8 * time.Minute,
	9 * time.Minute,
	10 * time.Minute,
	20 * time.Minute,
	30 * time.Minute,
	1 * time.Hour,
	2 * time.Hour,
}

// rocketMQProducer RocketMQ 生产者实现
type rocketMQProducer struct {
	producer rocketmq.Producer
	closed   bool
	mu       sync.Mutex
}

// 编译时检查接口实现
var _ Producer = (*rocketMQProducer)(nil)

func newRocketMQProducer(cfg *RocketConfig) (*rocketMQProducer, error) {
	retryTimes := cfg.RetryTimes
	if retryTimes <= 0 {
		retryTimes = 3
	}

	groupName := cfg.GroupName
	if groupName == "" {
		groupName = "DEFAULT_PRODUCER"
	}

	opts := []producer.Option{
		producer.WithNameServer(strings.Split(cfg.NameServer, ",")),
		producer.WithGroupName(groupName),
		producer.WithRetry(retryTimes),
		producer.WithQueueSelector(producer.NewHashQueueSelector()),
	}

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, producer.WithCredentials(primitive.Credentials{
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		}))
	}
	if cfg.Namespace != "" {
		opts = append(opts, producer.WithNamespace(cfg.Namespace))
	}

	p, err := rocketmq.NewProducer(opts...)
	if err != nil {
		return nil, fmt.Errorf("mq: create rocketmq producer failed: %w", err)
	}

	if err = p.Start(); err != nil {
		return nil, fmt.Errorf("mq: start rocketmq producer failed: %w", err)
	}

	return &rocketMQProducer{producer: p}, nil
}

// SendMessage 普通消息
func (p *rocketMQProducer) SendMessage(ctx context.Context, msg *Message) (*SendResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrAlreadyClosed
	}
	p.mu.Unlock()

	rMsg := buildRocketMQMessage(msg)
	result, err := p.producer.SendSync(ctx, rMsg)
	if err != nil {
		return nil, fmt.Errorf("mq: rocketmq send failed: %w", err)
	}
	return &SendResult{MessageID: result.MsgID}, nil
}

// SendDelayMessage 延时消息 — 映射到最接近的延时级别
func (p *rocketMQProducer) SendDelayMessage(ctx context.Context, msg *Message, delay time.Duration) (*SendResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrAlreadyClosed
	}
	p.mu.Unlock()

	level := findClosestDelayLevel(delay)
	rMsg := buildRocketMQMessage(msg)
	rMsg.WithDelayTimeLevel(level)

	result, err := p.producer.SendSync(ctx, rMsg)
	if err != nil {
		return nil, fmt.Errorf("mq: rocketmq send delay message failed: %w", err)
	}
	return &SendResult{MessageID: result.MsgID}, nil
}

// SendOrderMessage 顺序消息 — 使用 shardingKey 保证相同 key 路由到同一队列
func (p *rocketMQProducer) SendOrderMessage(ctx context.Context, msg *Message, shardingKey string) (*SendResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrAlreadyClosed
	}
	p.mu.Unlock()

	rMsg := buildRocketMQMessage(msg)
	rMsg.WithShardingKey(shardingKey)

	result, err := p.producer.SendSync(ctx, rMsg)
	if err != nil {
		return nil, fmt.Errorf("mq: rocketmq send order message failed: %w", err)
	}
	return &SendResult{MessageID: result.MsgID}, nil
}

// Close 关闭 RocketMQ 生产者
func (p *rocketMQProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrAlreadyClosed
	}
	p.closed = true
	return p.producer.Shutdown()
}

// buildRocketMQMessage 构建 RocketMQ 原生消息
func buildRocketMQMessage(msg *Message) *primitive.Message {
	rMsg := primitive.NewMessage(msg.Topic, msg.Body)
	if msg.Key != "" {
		rMsg.WithKeys([]string{msg.Key})
	}
	if len(msg.Properties) > 0 {
		// 将自定义属性序列化后存入 RocketMQ 属性
		if data, err := json.Marshal(msg.Properties); err == nil {
			rMsg.WithProperty("mq_properties", string(data))
		}
	}
	return rMsg
}

// findClosestDelayLevel 找到最接近指定延迟时长的延时级别（1-based）
func findClosestDelayLevel(delay time.Duration) int {
	bestLevel := 1
	bestDiff := absDuration(delayLevelDurations[0] - delay)

	for i, d := range delayLevelDurations {
		diff := absDuration(d - delay)
		if diff < bestDiff {
			bestDiff = diff
			bestLevel = i + 1 // 延时级别从 1 开始
		}
	}
	return bestLevel
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
