package rocketmq

import (
	"context"
	"fmt"
	"strings"
	"github.com/xsxdot/aio/pkg/core/config"
	"github.com/xsxdot/aio/pkg/core/consts"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/tracer"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/rlog"
)

var (
	log = logger.GetLogger()
)

// RocketMQManager 管理 RocketMQ 生产者和消费者
type RocketMQManager struct {
	rocketConfig config.RocketMQ
	env          string
	rdb          *redis.Client
	producer     rocketmq.Producer
	tracer       tracer.Tracer
}

// NewRocketMQManager 创建一个新的 RocketMQManager
func NewRocketMQManager(e string, rocket config.RocketMQ, r *redis.Client) (*RocketMQManager, error) {
	r2 := &RocketMQManager{
		rocketConfig: rocket,
		env:          e,
		rdb:          r,
		tracer:       tracer.NewSimpleTracer(),
	}
	return r2, r2.StartProducer()
}

// StartPushConsumer 使用给定的 bean 配置启动一个推送消费者
func (m *RocketMQManager) StartPushConsumer(bean *ConsumerBean) error {
	rlog.SetLogLevel("error")
	if !m.rocketConfig.Enable {
		return nil
	}
	pushConsumer, err := rocketmq.NewPushConsumer(
		consumer.WithNameServer(strings.Split(m.rocketConfig.NameServer, ",")),
		consumer.WithGroupName(bean.GroupName+"_"+m.env),
		//consumer.WithCredentials(primitive.Credentials{
		//	AccessKey: m.rocketConfig.AccessKey,
		//	SecretKey: m.rocketConfig.AccessSecret,
		//}),
		//consumer.WithNamespace(m.rocketConfig.NameSpace),

		consumer.WithConsumerModel(bean.ConsumerModel),
		consumer.WithRetry(5))

	if err != nil {
		log.WithField("Consumer", bean.EntryName).WithErr(err).Error("创建消费者失败")
		return fmt.Errorf("创建消费者 %s 失败: %w", bean.EntryName, err)
	}

	var selector consumer.MessageSelector

	if len(bean.Tag) > 0 {
		tag := m.env + "_" + bean.Tag[0]
		for i := 1; i < len(bean.Tag); i++ {
			tag = tag + " || " + m.env + "_" + bean.Tag[i]
		}
		selector = consumer.MessageSelector{
			Type:       consumer.TAG,
			Expression: tag,
		}
	} else {
		selector = consumer.MessageSelector{
			Type:       consumer.TAG,
			Expression: "*",
		}
	}

	err = pushConsumer.Subscribe(bean.Topic, selector, func(ctx context.Context, ext ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		// 广播模式直接处理
		if bean.ConsumerModel == consumer.BroadCasting {
			trace, _, _ := m.tracer.StartTrace(context.Background(), bean.EntryName)
			return bean.ConsumerFunc(trace, ext...)
		}

		// 集群模式处理幂等性，并逐条处理消息
		overallResult := consumer.ConsumeSuccess // 默认整个批次是成功的

		for _, msg := range ext {
			lockKey := consts.MSG_KEY_CACHE + bean.GroupName + ":" + msg.MsgId
			locked, err := m.rdb.SetNX(ctx, lockKey, 1, 1*time.Hour).Result()
			if err != nil {
				log.WithField("err", err).WithField("msgID", msg.MsgId).Error("幂等性检查失败: Redis SETNX 错误")
				// Redis 异常，无法继续处理，要求重试整个批-次
				return consumer.ConsumeRetryLater, err
			}

			if !locked {
				log.WithField("msgID", msg.MsgId).Debug("消息已被其他消费者锁定或已处理，跳过")
				continue // 跳过此消息
			}

			// 成功获取锁，处理单个消息
			// 调用业务逻辑时，我们只传递当前这一条消息
			trace, _, _ := m.tracer.StartTrace(context.Background(), bean.EntryName)
			consumeResult, consumeErr := bean.ConsumerFunc(trace, msg)

			// 如果业务逻辑要求重试
			if consumeResult == consumer.ConsumeRetryLater {
				// 设置最终结果为重试，这将告诉RocketMQ重新投递整个批次
				overallResult = consumer.ConsumeRetryLater

				// 为了让此条消息在下次投递时能被处理，必须删除它的锁
				if err := m.rdb.Del(ctx, lockKey).Err(); err != nil {
					log.WithField("err", err).WithField("key", lockKey).Error("消费重试前，删除 Redis 锁失败")
					// 这是一个严重问题，锁删不掉，消息就无法重试。
					// 此时我们仍然返回重试，寄希望于下次Redis恢复正常，
					// 但更好的做法是增加监控告警。
				}
				if consumeErr != nil {
					log.WithField("err", consumeErr).WithField("msgID", msg.MsgId).Warn("业务逻辑处理失败，将进行重试")
				}
			} else if consumeErr != nil {
				// 业务逻辑返回了成功，但附带了一个错误。
				// 我们尊重业务的成功返回，但需要记录下这个异常情况。
				log.WithField("err", consumeErr).WithField("msgID", msg.MsgId).Error("业务逻辑返回成功但带有错误，消息将不会重试")
			}
			// 如果消费成功(consumeResult == consumer.ConsumeSuccess)，我们保留锁，
			// 以防止在批次重试时重复执行。
		}

		// 如果 overallResult 在循环中被设置成了 ConsumeRetryLater，
		// 即使有些消息成功了，我们还是需要告诉 RocketMQ 我们需要重试。
		// 成功的消息因为有锁的存在，在下次投递时会被跳过。
		return overallResult, nil
	})

	if err != nil {
		log.WithField("Consumer", bean.EntryName).WithErr(err).Error("订阅消息失败")
		return fmt.Errorf("订阅主题 %s 失败: %w", bean.Topic, err)
	}

	err = pushConsumer.Start()

	if err != nil {
		log.WithField("Consumer", bean.EntryName).WithErr(err).Error("启动消费者失败")
		return fmt.Errorf("启动消费者 %s 失败: %w", bean.EntryName, err)
	}

	log.WithField("Consumer", bean.EntryName).Debug("启动消费者成功")
	return nil
}

func (c *ConsumerBean) ErrParse(msgId string, err error) {
	log.WithEntryName(c.EntryName).
		WithErr(err).
		WithField("msgId", msgId).
		Error("消费者解析消息失败")
}

type ConsumerBean struct {
	EntryName     string
	Topic         string
	GroupName     string
	ConsumerModel consumer.MessageModel
	Tag           []string
	ConsumerFunc  func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)
}
