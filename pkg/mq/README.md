# 消息队列统一组件 (pkg/mq)

该组件提供了一套统一的消息队列抽象接口，支持多种后端驱动（目前支持 Redis Stream 和 RocketMQ）。通过该组件，开发者可以无感地在不同的消息中间件之间切换，同时支持 **普通消息**、**延时消息** 和 **顺序消息**。

## 特性

- **统一接口**: 屏蔽底层实现差异，提供统一的 `Producer` 和 `Consumer` 接口。
- **多驱动支持**:
    - **Redis Stream**: 适合轻量级、低延迟场景。
    - **RocketMQ**: 适合高吞吐、分布式事务和严格的消息可靠性场景。
- **消息模式**:
    - **普通消息**: 即时发送与消费。
    - **延时消息**: 
        - RocketMQ: 使用原生延时级别。
        - Redis: 通过 Sorted Set + 定时轮询模拟实现。
    - **顺序消息**:
        - RocketMQ: 通过 ShardingKey 路由。
        - Redis: 通过扩展 Topic 名称实现分区保序。

## 快速开始

### 1. 引入包

```go
import "github.com/xsxdot/aio/pkg/mq"
```

### 2. 初始化生产者

```go
cfg := &mq.Config{
    Driver: mq.DriverRedis, // 或 mq.DriverRocketMQ
    Redis: mq.RedisConfig{
        Addr: "127.0.0.1:6379",
        DB:   0,
    },
}

// 也可以直接传入已有的 redis.Client (见下文 "高级用法")
producer, err := mq.NewProducer(cfg)
```

### 高级用法：复用现有 Redis 客户端

如果你已经在项目中初始化了 `redis.Client`，可以直接将其传入以复用连接：

```go
existingClient := redis.NewClient(...)

cfg := &mq.Config{
    Driver: mq.DriverRedis,
    Redis: mq.RedisConfig{
        Client: existingClient, // 直接传入客户端
    },
}

producer, _ := mq.NewProducer(cfg)
// 注意：当传入外部 Client 时，调用 producer.Close() 不会关闭该 Redis 连接。
```

### 3. 初始化消费者

```go
consumer, err := mq.NewConsumer(cfg)
if err != nil {
    panic(err)
}

// 订阅消息
consumer.Subscribe("user-register", "auth-service-group", func(ctx context.Context, msg *mq.Message) mq.ConsumeResult {
    fmt.Printf("收到消息: %s\n", string(msg.Body))
    return mq.ConsumeSuccess
})

// 启动
if err := consumer.Start(); err != nil {
    panic(err)
}
defer consumer.Close()
```

## 配置项详解

### 核心配置 (`mq.Config`)
- `Driver`: 驱动类型，可选 `redis` 或 `rocketmq`。

### Redis 配置 (`mq.RedisConfig`)
- `Addr`: Redis 地址。
- `DelayPollIntervalMs`: 延时消息检查间隔（毫秒），默认 500。
- `BlockTimeoutMs`: 消费者拉取阻塞时长（毫秒），默认 5000。
- `PendingRetryIntervalSec`: Pending 消息（未 ACK）重试间隔（秒），默认 30。

### RocketMQ 配置 (`mq.RocketConfig`)
- `NameServer`: 地址，多个地址用逗号分割。
- `GroupName`: 默认生产/消费组名。
- `RetryTimes`: 发送重试次数。

## 注意事项

1. **Redis 延时消息**: 底层使用 Sorted Set 实现，建议在消息量不是极大的场景下使用。
2. **RocketMQ 延时级别**: RocketMQ 仅支持固定级别（1s 到 2h）。如果传入的 `delay` 不在级别中，组件会自动匹配最接近的一个级别。
3. **顺序消息**: 顺序消息必须在发送时指定 `shardingKey`。
