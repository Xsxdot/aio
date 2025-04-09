# AIO SDK 客户端

AIO SDK 提供了统一的客户端接口，用于与 AIO 分布式系统进行通信。客户端内部维护各节点的连接和主节点的信息，为服务发现等组件提供通信功能。

## 主要特性

- **统一客户端**：单一客户端实例处理所有通信需求
- **主节点感知**：自动检测和连接主节点
- **服务发现**：通过独立组件提供服务注册、注销、发现和监控功能
- **事件通知**：服务事件、主节点变更和连接状态的事件回调
- **高可用性**：支持多服务器连接和自动故障转移
- **缓存服务**：集成Redis客户端，支持主节点变更时的自动重连

## 架构

SDK 包含以下主要组件：

1. **Client**: 核心客户端，负责连接管理、主节点跟踪和消息路由
2. **DiscoveryService**: 服务发现组件，负责服务的注册、发现和监控
3. **EtcdService**: ETCD客户端组件，提供分布式存储能力
4. **ConfigService**: 配置中心组件，提供配置管理能力
5. **RedisService**: Redis客户端组件，提供缓存服务能力

这种设计将不同功能与基础通信功能分离，同时保持紧密集成。

## 使用方法

### 创建客户端

```go
// 创建服务器端点列表
servers := []sdk.ServerEndpoint{
    {Host: "localhost", Port: 8080},
    {Host: "localhost", Port: 8081},
}

// 配置客户端选项
options := &sdk.ClientOptions{
    // 认证信息（可选）
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    
    // 网络选项
    ConnectionTimeout:    5 * time.Second,
    RetryCount:           3,
    RetryInterval:        1 * time.Second,
    
    // 功能选项
    AutoConnectToLeader:  true,
    ServiceWatchInterval: 10 * time.Second,
}

// 创建客户端
client := sdk.NewClient(servers, options)

// 连接到服务器
err := client.Connect()
if err != nil {
    panic(err)
}
```

### 服务发现操作

有两种方式使用服务发现功能：

#### 1. 通过客户端代理方法（简洁但功能有限）

```go
// 注册服务
service := discovery.ServiceInfo{
    ID:           "my-service-1",
    Name:         "my-service",
    Address:      "localhost:9090",
}
ctx := context.Background()
err = client.RegisterService(ctx, service)

// 发现服务
services, err := client.DiscoverServices(ctx, "my-service")

// 监听服务变更
err = client.WatchServices(ctx, "my-service")

// 注销服务
err = client.DeregisterService(ctx, "my-service-1")

// 注册服务事件处理器
client.OnServiceEvent(func(event *distributed.AdaptedDiscoveryEvent) {
    fmt.Printf("服务事件: 类型=%s, 服务ID=%s\n", 
        event.Type, event.Service.ID)
})
```

#### 2. 直接使用服务发现组件（更灵活）

```go
// 获取服务发现组件引用
discovery := client.Discovery

// 注册服务
service := discovery.ServiceInfo{
    ID:           "my-service-2",
    Name:         "my-service",
    Address:      "localhost:9091",
}
err = discovery.RegisterService(ctx, service)

// 发现服务
services, err := discovery.DiscoverServices(ctx, "my-service")

// 监听特定服务
err = discovery.WatchService(ctx, "my-service")

// 停止监听特定服务
err = discovery.StopWatchService(ctx, "my-service")

// 停止所有监听
err = discovery.StopWatchAll(ctx)

// 注销服务
err = discovery.DeregisterService(ctx, "my-service-2")

// 获取所有服务
allServices := discovery.GetServices()

// 根据ID获取服务
service, found := discovery.GetServiceByID("my-service-1")

// 根据名称获取服务
services := discovery.GetServicesByName("my-service")

// 注册服务事件处理器
discovery.OnServiceEvent(func(event *sdk.DiscoveryServiceEvent) {
    fmt.Printf("服务事件: 类型=%s, 服务ID=%s\n", 
        event.Type, event.Service.ID)
})
```

### 主节点操作

```go
// 获取主节点信息
leader, err := client.GetLeaderInfo(ctx)
if err == nil {
    fmt.Printf("主节点: ID=%s, 地址=%s:%d\n", 
        leader.ID, leader.Endpoint.Host, leader.Endpoint.Port)
}

// 连接到主节点
err = client.ConnectToLeader(ctx)

// 监听主节点变更
client.OnLeaderChange(func(oldLeader, newLeader *sdk.NodeInfo) {
    fmt.Printf("主节点变更: 从 %s 到 %s\n", oldLeader.ID, newLeader.ID)
})
```

### 节点连接状态监听

```go
client.OnConnectionStatusChange(func(nodeID, connID string, connected bool) {
    status := "连接"
    if !connected {
        status = "断开"
    }
    fmt.Printf("节点 %s 连接状态变更: %s\n", nodeID, status)
})
```

### 关闭客户端

```go
// 优雅关闭
client.Close()
```

## Redis缓存组件使用示例

SDK提供了Redis缓存组件，可以自动连接到主节点的Redis服务。当主节点变更时，Redis客户端会自动重新连接到新的主节点。

### 基本使用

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yourdomain/aio/pkg/sdk"
)

func main() {
	// 创建SDK客户端
	servers := []sdk.ServerEndpoint{
		{Host: "localhost", Port: 8000},
	}
	client := sdk.NewClient(servers, nil)

	// 连接到服务器
	err := client.Connect()
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer client.Close()

	// 获取Redis客户端
	redisClient, err := client.Redis.Get()
	if err != nil {
		fmt.Printf("获取Redis客户端失败: %v\n", err)
		return
	}

	// 使用Redis客户端
	ctx := context.Background()
	
	// 设置键值
	err = redisClient.Set(ctx, "test_key", "test_value", 10*time.Minute)
	if err != nil {
		fmt.Printf("设置键值失败: %v\n", err)
		return
	}
	
	// 获取键值
	value, err := redisClient.Get(ctx, "test_key")
	if err != nil {
		fmt.Printf("获取键值失败: %v\n", err)
		return
	}
	
	fmt.Printf("键值: %s\n", value)
}
```

### 自定义Redis选项

可以在创建SDK客户端时指定Redis选项：

```go
// 自定义Redis选项
redisOptions := &sdk.RedisClientOptions{
	ConnTimeout:   5 * time.Second,
	ReadTimeout:   3 * time.Second,
	WriteTimeout:  3 * time.Second,
	Password:      "your_redis_password",
	DB:            0,
	MaxRetries:    3,
	MinIdleConns:  10,
	PoolSize:      20,
	AutoReconnect: true,
}

// 创建Redis服务组件
client.Redis = sdk.NewRedisService(client, redisOptions)
```

### 监听主节点变更

Redis服务组件已经自动注册了主节点变更事件处理器，如果需要自定义处理逻辑，可以通过SDK客户端的`OnLeaderChange`方法注册自己的处理函数：

```go
client.OnLeaderChange(func(oldLeader, newLeader *sdk.NodeInfo) {
	if newLeader != nil {
		fmt.Printf("主节点变更为 %s (IP: %s, CachePort: %d)\n", 
			newLeader.NodeID, newLeader.IP, newLeader.CachePort)
	} else {
		fmt.Println("主节点已下线")
	}
})
```

### Redis操作

Redis客户端封装了常用的Redis操作，包括：

- `Set(ctx, key, value, expiration)`: 设置键值对
- `Get(ctx, key)`: 获取键值
- `Del(ctx, keys...)`: 删除键
- `Exists(ctx, keys...)`: 检查键是否存在
- `Expire(ctx, key, expiration)`: 设置键过期时间
- `TTL(ctx, key)`: 获取键剩余时间
- `HSet(ctx, key, values...)`: 设置哈希表字段值
- `HGet(ctx, key, field)`: 获取哈希表字段值
- `HGetAll(ctx, key)`: 获取哈希表所有字段和值
- `HDel(ctx, key, fields...)`: 删除哈希表字段
- `HExists(ctx, key, field)`: 检查哈希表字段是否存在

如果需要使用更高级的Redis功能，可以通过`redisClient.GetClient()`获取原始的go-redis客户端：

```go
// 获取原始的go-redis客户端
rawRedisClient := redisClient.GetClient()

// 使用原始客户端执行更复杂的操作
pipeline := rawRedisClient.Pipeline()
pipeline.Set(ctx, "key1", "value1", 0)
pipeline.Set(ctx, "key2", "value2", 0)
_, err = pipeline.Exec(ctx)
if err != nil {
	fmt.Printf("执行管道操作失败: %v\n", err)
}
```

## 示例

完整的示例代码请查看 [example/unified_client](example/unified_client) 目录。

## 配置中心 SDK 使用示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/xsxdot/aio/pkg/config"
    "github.com/xsxdot/aio/pkg/sdk"
)

func main() {
    // 创建客户端
    servers := []sdk.ServerEndpoint{
        {Host: "localhost", Port: 8080},
    }
    
    client := sdk.NewClient(servers, nil)
    defer client.Close()
    
    // 连接服务器
    if err := client.Connect(); err != nil {
        log.Fatalf("连接服务器失败: %v", err)
    }
    
    // 创建一个超时上下文
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // 获取一个配置项
    configKey := "app/settings"
    configData, err := client.Config.GetConfig(ctx, configKey)
    if err != nil {
        log.Printf("获取配置失败: %v", err)
    } else {
        fmt.Printf("配置数据: %+v\n", configData)
    }
    
    // 设置一个配置项
    configValue := map[string]config.ConfigValue{
        "timeout": {Value: "30s", Type: config.ValueTypeString},
        "maxRetry": {Value: "3", Type: config.ValueTypeInt},
        "debug": {Value: "true", Type: config.ValueTypeBool},
    }
    
    metadata := map[string]string{
        "version": "1.0.0",
        "author": "system",
    }
    
    err = client.Config.SetConfig(ctx, configKey, configValue, metadata)
    if err != nil {
        log.Printf("设置配置失败: %v", err)
    } else {
        log.Printf("配置已成功设置")
    }
    
    // 监听配置更新
    client.Config.OnConfigUpdate(func(event *sdk.ConfigUpdateEvent) {
        log.Printf("配置已更新: 键=%s, 环境=%s, 时间=%s", event.Key, event.Env, event.Timestamp)
        
        // 重新获取更新后的配置
        if event.Key == configKey {
            updatedConfig, err := client.Config.GetConfig(context.Background(), configKey)
            if err == nil {
                log.Printf("更新后的配置: %+v", updatedConfig)
            }
        }
    })
    
    // 获取环境特定的配置
    envConfigKey := "app/database"
    env := "production"
    fallbacks := []string{"staging", "development", "default"}
    
    envConfig, err := client.Config.GetEnvConfig(ctx, envConfigKey, env, fallbacks)
    if err != nil {
        log.Printf("获取环境配置失败: %v", err)
    } else {
        fmt.Printf("环境配置数据: %+v\n", envConfig)
    }
    
    // 获取配置历史
    history, err := client.Config.GetHistory(ctx, configKey, 10)
    if err != nil {
        log.Printf("获取配置历史失败: %v", err)
    } else {
        fmt.Printf("配置历史: %+v\n", history)
    }
    
    // 使用组合配置
    compositeKey := "app/composite/settings"
    composite, err := client.Config.GetCompositeConfig(ctx, compositeKey)
    if err != nil {
        log.Printf("获取组合配置失败: %v", err)
    } else {
        fmt.Printf("组合配置数据: %+v\n", composite)
    }
    
    // 合并多个配置
    keys := []string{"app/settings", "app/database", "app/network"}
    mergedConfig, err := client.Config.MergeCompositeConfigs(ctx, keys)
    if err != nil {
        log.Printf("合并配置失败: %v", err)
    } else {
        fmt.Printf("合并配置数据: %+v\n", mergedConfig)
    }
} 

## 指标收集功能

SDK 集成了应用指标收集功能，可以自动收集和发送应用状态指标和 API 调用信息到监控系统。该功能通过 NATS 消息队列发送指标数据，与监控系统保持松耦合。

### 功能特点

- 自动收集应用运行状态指标（CPU、内存、线程等）
- 支持记录和批量发送 API 调用信息
- 支持连接池指标监控
- 可配置的发送间隔和缓冲区大小
- 使用固定主题发送指标数据："metrics.api"和"metrics.app"

### 使用示例

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xsxdot/aio/pkg/sdk"
)

func main() {
	// 创建 SDK 客户端
	client := sdk.NewClient([]sdk.ServerEndpoint{
		{Host: "localhost", Port: 2379},
	}, nil)

	// 连接服务
	if err := client.Connect(); err != nil {
		panic(fmt.Sprintf("连接失败: %v", err))
	}
	defer client.Close()

	// 启动指标收集，使用自定义配置
	err := client.StartMetrics(&sdk.MetricsCollectorOptions{
		ServiceName:           "example-service",
		StatusCollectInterval: 15 * time.Second,  // 每15秒收集一次状态指标
		APIBufferSize:         50,                // 缓存50个API调用后批量发送
		APIFlushInterval:      5 * time.Second,   // 或每5秒发送一次
		AutoCollectStatus:     true,              // 自动收集应用状态
	})
	if err != nil {
		fmt.Printf("启动指标收集失败: %v\n", err)
	}

	// 注册连接池（如数据库连接池）
	client.Metrics.RegisterConnectionPool("db_pool", 20)

	// 更新连接池状态
	client.Metrics.UpdateConnectionPoolStats("db_pool", 5, 15)

	// 模拟 HTTP 服务，记录 API 调用
	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		// 记录请求开始时间
		startTime := time.Now()
		
		// 处理请求...
		time.Sleep(100 * time.Millisecond) // 模拟处理时间
		
		// 计算请求处理时间
		duration := time.Since(startTime)
		
		// 记录API调用
		client.Metrics.RecordAPICallSimple(
			"/api/users",        // 端点
			r.Method,            // 方法
			duration,            // 持续时间
			http.StatusOK,       // 状态码
			false,               // 是否有错误
			"",                  // 错误消息
		)
		
		// 返回响应
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello, World!")
	})

	// 启动 HTTP 服务
	fmt.Println("Starting HTTP server on :8080")
	http.ListenAndServe(":8080", nil)
}

### 高级用法

#### 手动收集和发送状态指标

如果需要控制指标收集的时机，可以关闭自动收集并手动触发：

```go
// 创建不自动收集指标的收集器
err := client.StartMetrics(&sdk.MetricsCollectorOptions{
    ServiceName:       "example-service",
    AutoCollectStatus: false,  // 关闭自动收集
})

// 在需要的时候手动收集
if err := client.Metrics.CollectStatusMetrics(); err != nil {
    fmt.Printf("收集指标失败: %v\n", err)
}
```

#### 详细记录 API 调用

如果需要记录更多API调用的详细信息：

```go
// 记录详细的API调用信息
client.Metrics.RecordAPICall(sdk.APICall{
    Endpoint:     "/api/users/123",
    Method:       "GET",
    Timestamp:    time.Now(),
    DurationMs:   45.5,
    StatusCode:   200,
    HasError:     false,
    ErrorMessage: "",
    RequestSize:  256,
    ResponseSize: 1024,
    ClientIP:     "192.168.1.10",
    Tags: map[string]string{
        "user_id": "123",
        "region":  "cn-north",
    },
})
```