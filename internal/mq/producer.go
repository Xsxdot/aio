package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Producer 是消息生产者接口
type Producer interface {
	// Publish 发布消息到指定主题
	Publish(subject string, data interface{}) error
	// PublishWithContext 带上下文的发布消息
	PublishWithContext(ctx context.Context, subject string, data interface{}) error
	// PublishAsync 异步发布消息（仅JetStream支持）
	PublishAsync(subject string, data interface{}) (nats.PubAckFuture, error)
	// Request 发送请求并等待响应
	Request(subject string, data interface{}, response interface{}, timeout time.Duration) error
	// Close 关闭生产者
	Close()
}

// DefaultProducer 默认的消息生产者实现
type DefaultProducer struct {
	client *NatsClient
	logger *zap.Logger
}

// NewProducer 创建一个新的消息生产者
func NewProducer(client *NatsClient, logger *zap.Logger) (Producer, error) {
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

	return &DefaultProducer{
		client: client,
		logger: logger,
	}, nil
}

// Publish 发布消息到指定主题
func (p *DefaultProducer) Publish(subject string, data interface{}) error {
	return p.PublishWithContext(context.Background(), subject, data)
}

// PublishWithContext 带上下文的发布消息
func (p *DefaultProducer) PublishWithContext(ctx context.Context, subject string, data interface{}) error {
	// 序列化消息
	payload, err := p.marshalData(data)
	if err != nil {
		return err
	}

	// 获取带有上下文信息的连接
	nc, err := p.getContextConn(ctx)
	if err != nil {
		return err
	}

	// 发布消息
	p.logger.Debug("发布消息",
		zap.String("subject", subject),
		zap.Int("size", len(payload)),
	)

	return nc.Publish(subject, payload)
}

// PublishAsync 异步发布消息（仅JetStream支持）
func (p *DefaultProducer) PublishAsync(subject string, data interface{}) (nats.PubAckFuture, error) {
	if p.client.js == nil {
		return nil, errors.New("JetStream未启用")
	}

	// 序列化消息
	payload, err := p.marshalData(data)
	if err != nil {
		return nil, err
	}

	// 发布消息
	p.logger.Debug("异步发布消息",
		zap.String("subject", subject),
		zap.Int("size", len(payload)),
	)

	return p.client.js.PublishAsync(subject, payload)
}

// Request 发送请求并等待响应
func (p *DefaultProducer) Request(subject string, data interface{}, response interface{}, timeout time.Duration) error {
	// 序列化消息
	payload, err := p.marshalData(data)
	if err != nil {
		return err
	}

	// 发送请求
	p.logger.Debug("发送请求",
		zap.String("subject", subject),
		zap.Int("size", len(payload)),
		zap.Duration("timeout", timeout),
	)

	msg, err := p.client.conn.Request(subject, payload, timeout)
	if err != nil {
		return err
	}

	// 解析响应
	if response != nil {
		if err := json.Unmarshal(msg.Data, response); err != nil {
			return fmt.Errorf("解析响应失败: %v", err)
		}
	}

	return nil
}

// Close 关闭生产者
func (p *DefaultProducer) Close() {
	p.logger.Info("关闭消息生产者")
}

// 序列化消息数据
func (p *DefaultProducer) marshalData(data interface{}) ([]byte, error) {
	// 如果数据已经是字节数组，则直接返回
	if payload, ok := data.([]byte); ok {
		return payload, nil
	}

	// 如果数据是字符串，则转换为字节数组
	if str, ok := data.(string); ok {
		return []byte(str), nil
	}

	// 否则使用JSON序列化
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %v", err)
	}

	return payload, nil
}

// 获取带有上下文信息的连接
func (p *DefaultProducer) getContextConn(ctx context.Context) (*nats.Conn, error) {
	if p.client.conn == nil {
		return nil, ErrNotConnected
	}

	// 检查上下文是否已取消
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return p.client.conn, nil
}
