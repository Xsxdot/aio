package rocketmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	errorc "github.com/xsxdot/aio/pkg/core/err"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

var (
	e = errorc.NewErrorBuilder("Rocketmq")
)

const (
	productEntry = "RocketMQ"
	CreateTag    = "creation"
	UpdateTag    = "update"
	DeleteTag    = "delete"
)

type Entity interface {
	TableName() string
}

type MessageBuilder struct {
	manager  *RocketMQManager
	topic    string
	tag      string
	sharding string
	message  interface{}
}

type normalProducerBuilder struct {
	message *MessageBuilder
}

type delayProducer struct {
	message *MessageBuilder
	Level   int
}

type tagBuilder struct {
	messageBuilder *MessageBuilder
}

type messageBuilder struct {
	messageBuilder *MessageBuilder
}

type stepBuilder struct {
	messageBuilder *MessageBuilder
}

func (m *RocketMQManager) SendEntity(ctx context.Context, topic string, tag string, entity Entity) error {
	return m.MessageBuilder(topic).
		Tag(tag).
		Message(entity).
		Sharding(entity.TableName()).
		NewNormalProducer().
		SendShardingSync(ctx, entity.TableName())
}

func (m *RocketMQManager) MessageBuilder(topic string) *tagBuilder {
	return &tagBuilder{
		messageBuilder: &MessageBuilder{
			manager: m,
			topic:   topic,
		},
	}
}

func (t *tagBuilder) Tag(tag string) *messageBuilder {
	// 自动为 tag 添加环境前缀，与消费者保持一致
	fullTag := tag
	if t.messageBuilder.manager.env != "" {
		fullTag = t.messageBuilder.manager.env + "_" + tag
	}
	return &messageBuilder{
		messageBuilder: &MessageBuilder{
			manager: t.messageBuilder.manager,
			topic:   t.messageBuilder.topic,
			tag:     fullTag,
		},
	}
}

func (t *messageBuilder) Message(message interface{}) *stepBuilder {
	return &stepBuilder{
		messageBuilder: &MessageBuilder{
			manager: t.messageBuilder.manager,
			topic:   t.messageBuilder.topic,
			tag:     t.messageBuilder.tag,
			message: message,
		},
	}
}

func (t *stepBuilder) Sharding(sharding string) *stepBuilder {
	return &stepBuilder{
		messageBuilder: &MessageBuilder{
			manager:  t.messageBuilder.manager,
			topic:    t.messageBuilder.topic,
			tag:      t.messageBuilder.tag,
			message:  t.messageBuilder.message,
			sharding: sharding,
		},
	}
}

func (t *stepBuilder) NewNormalProducer() *normalProducerBuilder {
	return &normalProducerBuilder{
		message: t.messageBuilder,
	}
}

func (t *stepBuilder) Delay1s() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   1,
	}
}

func (t *stepBuilder) Delay5s() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   2,
	}
}

func (t *stepBuilder) Delay10s() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   3,
	}
}

func (t *stepBuilder) Delay30s() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   4,
	}
}

func (t *stepBuilder) Delay1Min() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   5,
	}
}

func (t *stepBuilder) Delay5Min() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   9,
	}
}

func (t *stepBuilder) Delay10Min() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   10,
	}
}

func (t *stepBuilder) Delay1Hour() *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   11,
	}
}

func (t *stepBuilder) Level(level int) *delayProducer {
	return &delayProducer{
		message: t.messageBuilder,
		Level:   level,
	}
}

func (p *normalProducerBuilder) SendShardingSync(ctx context.Context, keys ...string) error {
	return p.message.manager.sendShardingSync(ctx, p.message.topic, p.message.tag, p.message.sharding, p.message.message, keys)
}

func (p *delayProducer) SendDelaySyncWithLevel(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		keys = []string{p.message.sharding}
	}
	return p.message.manager.sendDelaySyncWithDelayTimeLevel(ctx, p.message.topic, p.message.tag, p.message.sharding, keys, p.message.message, p.Level)
}

func (m *RocketMQManager) StartProducer() error {
	var err error
	p, err := rocketmq.NewProducer(
		producer.WithNameServer(strings.Split(m.rocketConfig.NameServer, ",")),
		producer.WithGroupName("GID_DATA_CHANGE"),
		//producer.WithCredentials(primitive.Credentials{
		//	AccessKey: m.rocketConfig.AccessKey,
		//	SecretKey: m.rocketConfig.AccessSecret,
		//}),
		//producer.WithNamespace(m.rocketConfig.NameSpace),
		producer.WithRetry(5),
		producer.WithQueueSelector(producer.NewHashQueueSelector()))
	if err != nil {
		log.WithErr(err).Error("创建Producer失败")
		return fmt.Errorf("创建Producer失败: %w", err)
	}

	err = p.Start()
	if err != nil {
		log.WithErr(err).Error("启动Producer失败")
		return fmt.Errorf("启动Producer失败: %w", err)
	}

	m.producer = p
	log.Info("rocketmq生产者启动成功")
	return nil
}

func (m *RocketMQManager) sendShardingSync(ctx context.Context, topic, tag, shardingKey string, message interface{}, keys []string) error {
	body, err := m.coverMsg(ctx, message)
	if err != nil {
		return e.New("MQ将消息json化失败", err).WithTraceID(ctx)
	}
	newMessage := primitive.NewMessage(topic, body)
	newMessage.WithTag(tag)
	newMessage.WithShardingKey(shardingKey)
	newMessage.WithKeys(keys)

	result, err := m.producer.SendSync(ctx, newMessage)
	if err != nil {
		return e.New("发送分区顺序消息失败", err).WithTraceID(ctx)
	}
	log.WithField("topic", topic).
		WithField("tag", tag).
		WithField("key", keys).
		WithField("shardingKey", shardingKey).
		WithField("msgID", result.MsgID).
		WithField("status", result.Status).Debug("发送分区顺序消息")
	return nil
}

func (m *RocketMQManager) coverMsg(ctx context.Context, message interface{}) ([]byte, error) {
	if message == nil {
		return []byte(""), nil
	}
	body, ok := message.([]byte)
	if ok {
		return body, nil
	}
	str, ok := message.(string)
	if ok {
		return []byte(str), nil
	}
	jsonBody, err := json.Marshal(message)
	if err != nil {
		return nil, e.New("MQ将消息json化失败", err).WithTraceID(ctx)
	}
	return jsonBody, nil
}

// sendDelaySyncWithDelayTimeLevel WithDelayTimeLevel set message delay time to consume.
// reference delay level definition: 1s 5s 10s 30s 1m 2m 3m 4m 5m 6m 7m 8m 9m 10m 20m 30m 1h 2h
// delay level starts from 1. for example, if we set param level=1, then the delay time is 1s.
func (m *RocketMQManager) sendDelaySyncWithDelayTimeLevel(ctx context.Context, topic, tag, shardingKey string, keys []string, message interface{}, level int) error {
	msg, err := m.coverMsg(ctx, message)
	if err != nil {
		return err
	}
	newMessage := primitive.NewMessage(topic, msg)
	newMessage.WithTag(tag)
	newMessage.WithShardingKey(shardingKey)
	newMessage.WithKeys(keys)
	newMessage.WithDelayTimeLevel(level)

	result, err := m.producer.SendSync(ctx, newMessage)
	if err != nil {
		return e.New("发送延时消息失败", err).WithTraceID(ctx)
	}
	log.WithField("topic", topic).
		WithField("tag", tag).
		WithField("key", keys).
		WithField("msgID", result.MsgID).
		WithField("status", result.Status).Debug("发送延时消息")
	return nil
}
