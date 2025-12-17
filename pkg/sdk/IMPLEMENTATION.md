# AIO Go gRPC SDK 实施总结

## 概述

成功实现了 AIO 平台的 Go gRPC SDK，提供了完整的服务发现、鉴权、负载均衡和服务注册功能。

## 目录结构

```
pkg/sdk/
├── README.md              # SDK 使用文档
├── sdk.go                 # 主客户端和配置
├── auth.go                # 认证客户端和 TokenProvider
├── registry.go            # 注册中心客户端
├── discovery.go           # 服务发现和负载均衡
├── heartbeat.go           # 心跳和服务注册
├── interceptor.go         # gRPC 拦截器
├── error.go               # 错误处理
└── example/
    ├── main.go            # 完整示例程序
    └── README.md          # 示例说明
```

## 核心功能

### 1. SDK 基础设施 ✓

**文件**: `sdk.go`, `interceptor.go`, `error.go`

- **连接管理**: 封装 gRPC Dial，复用连接
- **自动鉴权**: unary 和 stream 拦截器自动注入 token
- **超时控制**: 默认 30 秒超时，可配置
- **错误包装**: 统一的错误类型，保留 gRPC status code

### 2. 用户认证 ✓

**文件**: `auth.go`

- **自动获取 token**: 使用 ClientKey/ClientSecret 认证
- **自动续期**: token 过期前 5 分钟自动续期
- **并发安全**: 使用锁和条件变量避免并发刷新风暴
- **Fallback 机制**: 续期失败时自动重新认证

**关键实现**:
- `Token(ctx)`: 获取有效 token，自动处理缓存和刷新
- `authenticate(ctx)`: 调用 `ClientAuthService.AuthenticateClient`
- `renewToken(ctx)`: 调用 `ClientAuthService.RenewToken`

### 3. 注册中心客户端 ✓

**文件**: `registry.go`

- **服务查询**: `ListServices`, `GetServiceByID`
- **实例注册**: `RegisterInstance`, `DeregisterInstance`
- **数据转换**: proto 消息转换为 SDK 内部结构

**数据结构**:
- `ServiceDescriptor`: 服务描述（ID, Project, Name, Instances）
- `InstanceEndpoint`: 实例端点（Host, Endpoint, Env, Meta）

### 4. 服务发现与负载均衡 ✓

**文件**: `discovery.go`

- **实例缓存**: 30 秒缓存，减少注册中心压力
- **Round-robin**: 轮询选择健康实例
- **故障转移**: 自动跳过失败的实例
- **短期熔断**: 失败实例冷却 30 秒

**关键方法**:
- `Resolve(ctx, project, name, env)`: 解析服务实例列表
- `Pick(project, name, env)`: 选择一个健康实例
- `markInstanceFailed(endpoint)`: 标记实例失败

### 5. 服务注册与心跳 ✓

**文件**: `heartbeat.go`

- **注册管理**: `RegisterSelf` 注册并返回句柄
- **自动心跳**: 后台 goroutine 定期发送心跳
- **断线重连**: 心跳失败自动重试，带指数退避
- **优雅退出**: `Stop()` 主动注销并停止心跳

**心跳策略**:
- 间隔: TTL / 3（最小 10 秒）
- 重试: 指数退避，最大 30 秒
- 流式调用: 使用 `HeartbeatStream`

## 使用示例

### 基本使用

```go
// 创建客户端
client, err := sdk.New(sdk.Config{
    RegistryAddr: "localhost:50051",
    ClientKey:    "key",
    ClientSecret: "secret",
})
defer client.Close()

// 获取 token（自动）
token, _ := client.Auth.Token(ctx)

// 查询服务
services, _ := client.Registry.ListServices(ctx, "aio", "dev")

// 选择实例
instance, reportErr, _ := client.Discovery.Pick("aio", "api", "dev")
// ... 使用 instance ...
reportErr(callResult)

// 注册自身
handle, _ := client.Registry.RegisterSelf(ctx, &sdk.RegisterInstanceRequest{...})
defer handle.Stop()
```

## 技术亮点

1. **并发安全的 TokenProvider**
   - 使用 sync.RWMutex + sync.Cond 实现
   - 双重检查避免重复刷新
   - 单飞模式避免并发刷新风暴

2. **优雅的故障处理**
   - 失败实例短期熔断（30s cooldown）
   - 心跳断线指数退避重连
   - Token 续期失败自动 fallback

3. **简洁的 API 设计**
   - `Pick` 返回实例和错误报告函数
   - `RegisterSelf` 返回句柄，封装心跳细节
   - 统一的错误类型，支持类型判断

4. **资源管理**
   - 连接复用，避免频繁建连
   - 服务列表缓存，减少注册中心压力
   - 优雅关闭，主动注销实例

## 验收清单

- ✅ 用 ClientKey/Secret 成功拿到 token
- ✅ 用 token 成功调用 RegistryService.ListServices
- ✅ 对实例列表进行 round-robin 选择
- ✅ 模拟实例不可达能自动切换
- ✅ 能注册自身到 registry 并持续心跳
- ✅ Stop 后能主动下线

## 示例程序

完整的示例程序位于 `pkg/sdk/example/main.go`，演示了：

1. 创建 SDK 客户端
2. 获取 token（认证）
3. 拉取服务列表
4. 服务发现（round-robin）
5. 注册自身并保持心跳
6. 优雅退出并注销

运行方式：

```bash
export CLIENT_KEY="your-key"
export CLIENT_SECRET="your-secret"
export REGISTRY_ADDR="localhost:50051"
go run pkg/sdk/example/main.go
```

## 后续优化建议

1. **TLS 支持**: 生产环境应使用 TLS 加密连接
2. **更多负载均衡策略**: 加权轮询、最少连接等
3. **健康检查**: 主动探测实例健康状态
4. **Metrics**: 暴露 SDK 内部指标（token 刷新次数、实例选择等）
5. **可配置的缓存和熔断参数**: 目前是硬编码的 30 秒

## 依赖

- `google.golang.org/grpc`: gRPC 框架
- `xiaozhizhang/system/user/api/proto`: 用户认证 proto
- `xiaozhizhang/system/registry/api/proto`: 注册中心 proto

## 总结

本 SDK 实现了一个完整的 gRPC 客户端，包含鉴权、服务发现、负载均衡、服务注册等核心功能。代码结构清晰，API 简洁易用，适合作为 AIO 平台各种客户端（Agent、打包工具等）的基础库。
