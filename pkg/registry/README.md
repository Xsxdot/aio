# 服务注册与发现中心

基于ETCD的分布式服务注册与发现中心，提供高可用的服务管理能力。

## 功能特性

- 🔍 **服务注册与发现**: 支持服务实例的注册、注销和发现
- ⏱️ **服务运行时间跟踪**: 自动记录服务启动时间和注册时间，提供运行时长查询
- 🔄 **状态管理**: 服务状态管理机制，支持在线/离线状态切换
- 👁️ **实时监听**: 支持服务变更的实时监听和通知
- ⚖️ **负载均衡**: 支持服务权重配置，便于负载均衡实现
- 📊 **健康检查**: 基于心跳续约的服务健康状态管理
- 🕐 **自动离线**: 通过调度器定时检查，将长时间未续约的服务自动标记为离线状态
- 🏷️ **元数据支持**: 支持自定义服务元数据
- 🔒 **并发安全**: 线程安全的操作，支持并发访问
- 🔄 **故障恢复**: 支持服务异常时的自动重新注册

## 核心概念

### 状态管理机制

本组件使用状态管理机制来管理服务的生命周期：

- **状态跟踪**: 服务注册时直接保存在ETCD中，不使用租约
- **心跳续约**: 客户端需要定期调用`Renew`方法更新服务状态和续约时间
- **调度监控**: 通过调度器创建定时任务，定期检查服务续约状态
- **自动离线**: 长时间未续约的服务会被调度器自动标记为离线状态，但不会删除
- **物理删除**: 只有主动调用`Unregister`方法才会物理删除服务记录

### 服务实例

每个服务实例包含以下信息：

- **基本信息**: ID、名称、地址、协议
- **时间信息**: 注册时间、启动时间、最后续约时间、下线时间
- **状态信息**: 服务状态、负载权重
- **元数据**: 自定义的键值对信息

## 快速开始

### 1. 创建注册中心

```go
package main

import (
    "context"
    "time"
    
    "github.com/xsxdot/aio/app/config"
    "github.com/xsxdot/aio/internal/etcd"
    "github.com/xsxdot/aio/pkg/registry"
    "github.com/xsxdot/aio/pkg/scheduler"
    "github.com/xsxdot/aio/pkg/lock"
)

func main() {
    // 创建ETCD客户端
    etcdClient, err := etcd.NewClient(&config.EtcdConfig{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer etcdClient.Close()

    // 创建分布式锁管理器
    lockManager := lock.NewEtcdLockManager(etcdClient)

    // 创建调度器（如果需要监控过期服务）
    schedulerInstance := scheduler.NewScheduler(lockManager, scheduler.DefaultSchedulerConfig())
    if err := schedulerInstance.Start(); err != nil {
        panic(err)
    }
    defer schedulerInstance.Stop()

    // 创建注册中心
    reg, err := registry.NewRegistry(etcdClient, schedulerInstance,
        registry.WithLeaseTTL(30),                    // 30秒超时阈值
        registry.WithRenewInterval(10*time.Second),   // 每10秒续约
        registry.WithPrefix("/my-app/services"),      // 自定义前缀
    )
    if err != nil {
        panic(err)
    }
    defer reg.Close()
}
```

### 2. 注册服务

```go
// 创建服务实例
instance := &registry.ServiceInstance{
    ID:       "user-service-001",
    Name:     "user-service",
    Address:  "127.0.0.1:8080",
    Protocol: "http",
    Status:   "active",
    Weight:   100,
    Metadata: map[string]string{
        "version": "1.0.0",
        "env":     "production",
    },
}

// 注册服务
ctx := context.Background()
err := reg.Register(ctx, instance)
if err != nil {
    panic(err)
}
```

### 3. 客户端主动续约

```go
// 方式1: 手动续约
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := reg.Renew(ctx, instance.ID); err != nil {
                log.Printf("续约失败: %v", err)
                // 如果服务不存在，需要重新注册
                if err.Error() == "service not found" {
                    if err := reg.Register(ctx, instance); err != nil {
                        log.Printf("重新注册失败: %v", err)
                    }
                }
            }
        }
    }
}()

// 方式2: 使用便捷函数
stopRenew := registry.RenewService(reg, instance.ID, 10*time.Second)
defer stopRenew() // 停止续约
```

### 4. 发现服务

```go
// 发现服务实例
instances, err := reg.Discover(ctx, "user-service")
if err != nil {
    panic(err)
}

for _, inst := range instances {
    fmt.Printf("服务: %s, 地址: %s, 运行时长: %v\n", 
        inst.ID, inst.Address, inst.GetUptime())
}
```

### 5. 监听服务变更

```go
// 创建监听器
watcher, err := reg.Watch(ctx, "user-service")
if err != nil {
    panic(err)
}
defer watcher.Stop()

// 监听变更
go func() {
    for {
        instances, err := watcher.Next()
        if err != nil {
            log.Printf("监听错误: %v", err)
            break
        }
        
        fmt.Printf("服务变更，当前实例数: %d\n", len(instances))
    }
}()
```

## API 参考

### Registry 接口

```go
type Registry interface {
    // 注册服务实例
    Register(ctx context.Context, instance *ServiceInstance) error
    
    // 注销服务实例（物理删除）
    Unregister(ctx context.Context, serviceID string) error
    
    // 下线服务实例（逻辑删除，保留记录）
    Offline(ctx context.Context, serviceID string) (*ServiceInstance, error)
    
    // 续约服务实例
    Renew(ctx context.Context, serviceID string) error
    
    // 发现服务实例列表
    Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
    
    // 监听服务变更
    Watch(ctx context.Context, serviceName string) (Watcher, error)
    
    // 获取单个服务实例
    GetService(ctx context.Context, serviceID string) (*ServiceInstance, error)
    
    // 列出所有服务名称
    ListServices(ctx context.Context) ([]string, error)
    
    // 关闭注册中心
    Close() error
}
```

### ServiceInstance 结构

```go
type ServiceInstance struct {
    ID           string            `json:"id"`            // 服务实例ID
    Name         string            `json:"name"`          // 服务名称
    Address      string            `json:"address"`       // 服务地址
    Protocol     string            `json:"protocol"`      // 协议类型
    RegisterTime time.Time         `json:"register_time"` // 注册时间
    StartTime    time.Time         `json:"start_time"`    // 启动时间
    Metadata     map[string]string `json:"metadata"`      // 元数据
    Weight       int               `json:"weight"`        // 权重
    Status       string            `json:"status"`        // 状态
}
```

## 配置选项

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| Prefix | string | "/aio/registry" | ETCD键前缀 |
| LeaseTTL | int64 | 30 | 租约生存时间(秒) |
| KeepAlive | bool | true | 是否启用续约 |
| RenewInterval | time.Duration | 10s | 续约间隔时间 |
| RetryTimes | int | 3 | 重试次数 |
| RetryDelay | time.Duration | 1s | 重试延迟 |

## 最佳实践

### 1. 续约策略

- **续约间隔**: 建议设置为租约TTL的1/3，确保有足够的重试机会
- **错误处理**: 续约失败时可以选择重新注册服务
- **资源清理**: 服务停止时记得调用停止续约函数

```go
// 推荐的续约配置
registry.WithLeaseTTL(30),                    // 30秒租约
registry.WithRenewInterval(10*time.Second),   // 10秒续约间隔
```

### 2. 服务注册

- **唯一ID**: 确保服务实例ID的唯一性
- **详细元数据**: 提供足够的元数据信息便于服务发现
- **合理权重**: 根据服务能力设置权重值

### 3. 错误处理

- **网络异常**: 实现重试机制处理临时网络问题
- **ETCD故障**: 考虑实现熔断机制
- **租约过期**: 监控续约状态，及时处理异常

### 4. 性能优化

- **批量操作**: 对于大量服务发现，考虑缓存机制
- **监听优化**: 合理设置监听范围，避免不必要的事件
- **连接复用**: 共享ETCD客户端连接

## 故障排查

### 常见问题

1. **服务注册失败**
   - 检查ETCD连接状态
   - 验证服务实例数据格式
   - 确认权限配置

2. **续约失败**
   - 检查网络连接
   - 确认租约是否已过期
   - 验证服务ID是否正确

3. **服务发现异常**
   - 检查服务名称是否正确
   - 确认服务是否已注册
   - 验证键前缀配置

### 日志配置

```go
// 设置日志器获取详细日志
registry.SetLogger(logger)
```

## 许可证

本项目采用 MIT 许可证。

# 服务注册中心 API 文档

本文档详细描述了服务注册中心 (Registry) 的 HTTP API。所有 API 都需要经过身份验证，部分管理员接口需要额外的管理员角色权限。

## 统一响应格式

### 成功响应

```json
{
  "code": 20000,
  "msg": "success",
  "data": { ... }
}
```

### 失败响应

```json
{
  "code": "<error_code>",
  "msg": "<error_message>",
  "data": null
}
```

- `code`: 业务状态码，`20000` 表示成功，其他值表示失败。
- `msg`: 提示信息。
- `data`: 成功时返回的数据。

---

## 1. 服务实例管理

### 1.1 注册服务实例

- **POST** `/registry/services`
- **描述**: 注册一个新的服务实例。服务实例ID将由系统自动生成。
- **认证**: 需要

#### 请求体

```json
{
  "name": "user-service",
  "address": "127.0.0.1:8080",
  "protocol": "http",
  "metadata": {
    "version": "1.0.2",
    "region": "us-east-1"
  },
  "weight": 100,
  "status": "active"
}
```

| 字段 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `name` | `string` | 是 | 服务名称，例如 `user-service`。 |
| `address` | `string` | 是 | 服务地址，格式为 `host:port`。 |
| `protocol` | `string` | 否 | 协议类型，例如 `http`, `grpc`。默认为 `http`。 |
| `metadata` | `object` | 否 | 服务元数据，键值对形式。 |
| `weight` | `integer`| 否 | 负载均衡权重，默认为 `100`。 |
| `status` | `string` | 否 | 服务状态，`active`, `inactive`, `maintenance`。默认为 `active`。 |

#### 成功响应 (200 OK)

返回创建的服务实例对象。

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "id": "user-service-b2f7a9a8-c2b1-4a8a-9f0a-4f1e5e3b2e1a",
    "name": "user-service",
    "address": "127.0.0.1:8080",
    "protocol": "http",
    "register_time": "2023-10-27T10:00:00Z",
    "start_time": "2023-10-27T10:00:00Z",
    "metadata": {
      "version": "1.0.2",
      "region": "us-east-1"
    },
    "weight": 100,
    "status": "active"
  }
}
```

---

### 1.2 注销服务实例

- **DELETE** `/registry/services/:serviceID`
- **描述**: 根据服务实例ID注销一个服务实例。
- **认证**: 需要

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceID` | `string` | 服务实例的唯一ID。 |

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": "服务注销成功"
}
```

---

### 1.3 续约服务实例

- **PUT** `/registry/services/:serviceID/renew`
- **描述**: 为一个服务实例续约租期，防止其被注销。
- **认证**: 需要

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceID` | `string` | 服务实例的唯一ID。 |

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": "服务续约成功"
}
```

---

### 1.4 获取单个服务实例

- **GET** `/registry/services/:serviceID`
- **描述**: 根据服务实例ID获取其详细信息。
- **认证**: 需要

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceID` | `string` | 服务实例的唯一ID。 |

#### 成功响应 (200 OK)

返回服务实例对象。

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "id": "user-service-b2f7a9a8-c2b1-4a8a-9f0a-4f1e5e3b2e1a",
    "name": "user-service",
    "address": "127.0.0.1:8080",
    "protocol": "http",
    "register_time": "2023-10-27T10:00:00Z",
    "start_time": "2023-10-27T10:00:00Z",
    "metadata": {
      "version": "1.0.2",
      "region": "us-east-1"
    },
    "weight": 100,
    "status": "active"
  }
}
```

---

## 2. 服务发现

### 2.1 列出所有服务名称

- **GET** `/registry/services`
- **描述**: 获取注册中心所有不重复的服务名称列表。
- **认证**: 需要

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": [
    "user-service",
    "order-service",
    "product-service"
  ]
}
```

---

### 2.2 发现服务实例列表

- **GET** `/registry/discovery/:serviceName`
- **描述**: 根据服务名称发现所有健康的服务实例列表。
- **认证**: 需要

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceName`| `string` | 服务名称。 |

#### 查询参数

| 参数 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `status` | `string` | 否 | 按服务状态过滤，例如 `active`。 |
| `protocol` | `string` | 否 | 按协议类型过滤，例如 `grpc`。 |

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": [
    {
      "id": "user-service-b2f7a9a8-c2b1-4a8a-9f0a-4f1e5e3b2e1a",
      "name": "user-service",
      "address": "127.0.0.1:8080",
      "protocol": "http",
      ...
    },
    {
      "id": "user-service-a1b2c3d4-e5f6-7890-1234-567890abcdef",
      "name": "user-service",
      "address": "127.0.0.1:8081",
      "protocol": "http",
      ...
    }
  ]
}
```

---

## 3. 健康与统计

### 3.1 检查服务健康状态

- **GET** `/registry/services/:serviceID/health`
- **描述**: 检查单个服务实例的健康状态和运行时长。
- **认证**: 需要

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceID` | `string` | 服务实例的唯一ID。 |

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "service_id": "user-service-b2f7a9a8-c2b1-4a8a-9f0a-4f1e5e3b2e1a",
    "service_name": "user-service",
    "status": "active",
    "healthy": true,
    "uptime": "3h25m10s",
    "register_duration": "3h25m10s",
    "last_check": "2023-10-27T13:25:10Z"
  }
}
```

---

### 3.2 获取注册中心统计信息

- **GET** `/registry/stats`
- **描述**: 获取整个注册中心的统计信息，包括服务总数、实例总数等。
- **认证**: 需要

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "total_services": 3,
    "total_instances": 15,
    "healthy_instances": 14,
    "unhealthy_instances": 1,
    "service_stats": {
      "user-service": 5,
      "order-service": 7,
      "product-service": 3
    },
    "timestamp": "2023-10-27T13:30:00Z"
  }
}
```

---

### 3.3 获取指定服务的统计信息

- **GET** `/registry/services/:serviceName/stats`
- **描述**: 获取指定服务的详细统计信息，包括实例列表、协议分布等。
- **认证**: 需要

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceName`| `string` | 服务名称。 |

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "service_name": "user-service",
    "total_instances": 5,
    "healthy_instances": 5,
    "unhealthy_instances": 0,
    "protocols": {
      "http": 3,
      "grpc": 2
    },
    "statuses": {
      "active": 5
    },
    "instances": [ ... ], // 详细实例列表
    "timestamp": "2023-10-27T13:35:00Z"
  }
}
```

---

## 4. 管理员功能

### 4.1 获取所有服务详细信息

- **GET** `/registry/admin/all`
- **描述**: 获取所有服务的完整实例列表，以服务名称分组。
- **认证**: 需要 **管理员角色**

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "user-service": [
      {
        "id": "user-service-b2f7a9a8-c2b1-4a8a-9f0a-4f1e5e3b2e1a",
        ...
      }
    ],
    "order-service": [
      {
        "id": "order-service-c3d4e5f6-a1b2-c3d4-e5f6-789012abcdef",
        ...
      }
    ]
  }
}
```

---

### 4.2 删除指定服务的所有实例

- **DELETE** `/registry/admin/services/:serviceName`
- **描述**: 强制注销指定服务名称下的所有实例。这是一个危险操作。
- **认证**: 需要 **管理员角色**

#### 路径参数

| 参数 | 类型 | 描述 |
| :--- | :--- | :--- |
| `serviceName`| `string` | 服务名称。 |

#### 成功响应 (200 OK)

```json
{
  "code": 20000,
  "msg": "success",
  "data": {
    "service_name": "user-service",
    "total_instances": 5,
    "removed_count": 5,
    "errors": []
  }
}
```

#### 部分失败响应

如果部分实例删除失败，HTTP 状态码仍可能是成功的，但响应体会包含错误信息。

```json
{
  "code": 50000,
  "msg": "部分删除失败，详情请查看响应数据",
  "data": {
    "service_name": "user-service",
    "total_instances": 5,
    "removed_count": 4,
    "errors": [
      "删除实例 user-service-... 失败: <原因>"
    ]
  }
}
``` 