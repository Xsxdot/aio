# AIO Go gRPC SDK

AIO 平台的 Go 语言 gRPC SDK，提供服务发现、鉴权、注册等功能。

## 特性

- **自动鉴权**：基于 ClientKey/ClientSecret 自动获取和续期 JWT token
- **服务发现**：从注册中心拉取服务实例列表，支持缓存和刷新
- **负载均衡**：内置 round-robin 负载均衡策略
- **故障转移**：自动检测失败实例并进行短期熔断
- **服务注册**：支持注册自身到注册中心并保持心跳
- **并发安全**：所有客户端操作都是并发安全的

## 安装

```bash
go get xiaozhizhang/pkg/sdk
```

## 快速开始

### 基本用法

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "xiaozhizhang/pkg/sdk"
)

func main() {
    // 创建 SDK 客户端（支持单地址或集群地址）
    client, err := sdk.New(sdk.Config{
        // 单地址模式
        RegistryAddr: "localhost:50051",
        // 或集群模式（逗号分隔，自动 round-robin 负载均衡）
        // RegistryAddr: "registry1:50051,registry2:50051,registry3:50051",
        ClientKey:    "your-client-key",
        ClientSecret: "your-client-secret",
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()
    
    // 使用客户端...
}
```

### 认证

SDK 会自动处理认证，你不需要手动管理 token：

```go
// 获取 token（自动获取/续期）
ctx := context.Background()
token, err := client.Auth.Token(ctx)
if err != nil {
    panic(err)
}
fmt.Println("Token:", token)
```

### 服务发现

```go
// 列出所有服务
ctx := context.Background()
services, err := client.Registry.ListServices(ctx, "aio", "dev")
if err != nil {
    panic(err)
}

for _, svc := range services {
    fmt.Printf("Service: %s/%s\n", svc.Project, svc.Name)
    for _, inst := range svc.Instances {
        fmt.Printf("  Instance: %s\n", inst.Endpoint)
    }
}
```

### 负载均衡与故障转移

```go
// 选择一个健康的实例
instance, reportErr, err := client.Discovery.Pick("aio", "api-service", "dev")
if err != nil {
    panic(err)
}

fmt.Println("Selected:", instance.Endpoint)

// 使用实例进行调用...
// err := callService(instance.Endpoint)

// 报告调用结果（失败会触发熔断）
reportErr(err)
```

### 注册自身

```go
// 注册到注册中心
ctx := context.Background()
handle, err := client.Registry.RegisterSelf(ctx, &sdk.RegisterInstanceRequest{
    ServiceID:   1,
    InstanceKey: "my-app-1",
    Env:         "dev",
    Host:        "localhost",
    Endpoint:    "http://localhost:8080",
    MetaJSON:    `{"version":"1.0.0"}`,
    TTLSeconds:  60,
})
if err != nil {
    panic(err)
}

// 心跳会自动运行...

// 程序退出时注销
defer handle.Stop()
```

## API 文档

### Client

主客户端，包含所有子客户端。

```go
type Client struct {
    Auth      *AuthClient      // 鉴权客户端
    Registry  *RegistryClient  // 注册中心客户端
    Discovery *DiscoveryClient // 服务发现客户端
}

func New(config Config) (*Client, error)
func (c *Client) Close() error
func (c *Client) DefaultContext() (context.Context, context.CancelFunc)
```

### AuthClient

处理客户端认证和 token 管理。

```go
type AuthClient struct { ... }

// 获取有效的 token（自动获取/续期）
func (ac *AuthClient) Token(ctx context.Context) (string, error)
```

### RegistryClient

与注册中心交互。

```go
type RegistryClient struct { ... }

// 列出服务
func (rc *RegistryClient) ListServices(ctx context.Context, project, env string) ([]ServiceDescriptor, error)

// 获取服务详情
func (rc *RegistryClient) GetServiceByID(ctx context.Context, serviceID int64) (*ServiceDescriptor, error)

// 注册实例
func (rc *RegistryClient) RegisterInstance(ctx context.Context, req *RegisterInstanceRequest) (*RegisterInstanceResponse, error)

// 注销实例
func (rc *RegistryClient) DeregisterInstance(ctx context.Context, serviceID int64, instanceKey string) error

// 注册自身（包含心跳）
func (rc *RegistryClient) RegisterSelf(ctx context.Context, req *RegisterInstanceRequest) (*RegistrationHandle, error)
```

### DiscoveryClient

服务发现和负载均衡。

```go
type DiscoveryClient struct { ... }

// 解析服务实例列表
func (dc *DiscoveryClient) Resolve(ctx context.Context, project, serviceName, env string) ([]InstanceEndpoint, error)

// 选择一个健康的实例（round-robin + 故障转移）
func (dc *DiscoveryClient) Pick(project, serviceName, env string) (*InstanceEndpoint, func(error), error)

// 刷新服务缓存
func (dc *DiscoveryClient) RefreshService(ctx context.Context, project, serviceName, env string) error
```

### RegistrationHandle

服务注册句柄，用于管理心跳和注销。

```go
type RegistrationHandle struct { ... }

// 停止心跳并注销实例
func (h *RegistrationHandle) Stop() error
```

## 配置

```go
type Config struct {
    // 注册中心地址（必填）
    // 支持单地址：  "host:port"
    // 或集群地址：  "host1:port1,host2:port2,host3:port3"
    // 集群模式自动启用 round-robin 负载均衡和故障转移
    RegistryAddr string
    
    // 客户端认证密钥（必填）
    ClientKey string
    
    // 客户端认证密文（必填）
    ClientSecret string
    
    // 默认超时时间（可选，默认 30s）
    DefaultTimeout time.Duration
    
    // 禁用自动鉴权（可选，默认 false）
    DisableAuth bool
}
```

### 集群模式说明

当 `RegistryAddr` 包含多个地址（逗号分隔）时，SDK 会自动：

1. **启用 round-robin 负载均衡**：在多个注册中心节点间均匀分配请求
2. **自动故障转移**：当某个节点不可用时，自动切换到其他健康节点
3. **连接复用**：底层 gRPC 连接会自动管理和复用

示例：

```go
// 单节点（向后兼容）
RegistryAddr: "registry.example.com:50051"

// 高可用集群（推荐生产环境）
RegistryAddr: "registry1:50051,registry2:50051,registry3:50051"

// 支持空格
RegistryAddr: "registry1:50051, registry2:50051, registry3:50051"
```

## 错误处理

SDK 提供了统一的错误包装：

```go
// 检查是否为特定类型的错误
if sdk.IsNotFound(err) {
    // 处理 NotFound 错误
}

if sdk.IsUnauthenticated(err) {
    // 处理认证失败
}

if sdk.IsUnavailable(err) {
    // 处理服务不可用
}
```

## 集成测试

SDK 提供了完整的集成测试，覆盖所有功能（Auth、Registry、Discovery、Config CRUD、ShortURL、RegisterSelf with Heartbeat）。

### 运行集成测试

集成测试需要连接真实的服务环境，因此需要通过环境变量进行配置：

**必需的环境变量：**

```bash
SDK_INTEGRATION=1                      # 显式启用集成测试
REGISTRY_ADDR=localhost:50051          # 注册中心地址（支持集群：host1:50051,host2:50051）
CLIENT_KEY=your-client-key             # 客户端密钥
CLIENT_SECRET=your-client-secret       # 客户端密文
SDK_TEST_SERVICE_ID=1                  # 用于 RegisterSelf 测试的 serviceId
SDK_TEST_SHORTURL_DOMAIN_ID=1          # 用于短链接测试的 domainId
```

**可选的环境变量：**

```bash
SDK_TEST_PROJECT=aio                   # 默认 "aio"
SDK_TEST_ENV=dev                       # 默认 "dev"
SDK_TEST_SERVICE_NAME=your-service     # 指定用于 Pick 测试的服务名
SDK_TEST_SHORTURL_HOST=short.example.com  # 指定用于 Resolve 测试的 host
```

**运行测试：**

```bash
# 设置环境变量并运行
SDK_INTEGRATION=1 \
  REGISTRY_ADDR=localhost:50051 \
  CLIENT_KEY=your-key \
  CLIENT_SECRET=your-secret \
  SDK_TEST_SERVICE_ID=1 \
  SDK_TEST_SHORTURL_DOMAIN_ID=1 \
  go test ./pkg/sdk/example -run TestSDK_FullIntegration -v
```

**测试覆盖：**

1. **Auth**：获取并验证 token
2. **Registry.ListServices**：拉取服务列表
3. **Discovery.Pick**：负载均衡选择实例（3 轮 round-robin）
4. **RegisterSelf + Heartbeat + Stop**：注册实例、等待心跳、优雅注销
5. **Config CRUD**：创建 → 读取 → 更新 → 再次读取 → 删除
6. **ShortURL**：创建短链 → 解析 → 上报成功

**注意事项：**

- 集成测试会真实写入数据（配置、短链、注册中心实例），但会自动清理
- 需要测试环境的相应权限（Config 管理端可能需要 admin 权限）
- 如果未设置 `SDK_INTEGRATION=1` 或缺少必需环境变量，测试将自动跳过（`t.Skip`）

## 最佳实践

1. **复用 Client**：Client 是并发安全的，应该在整个应用生命周期中复用
2. **设置合理的超时**：根据实际情况调整 `DefaultTimeout`
3. **报告调用结果**：使用 `Pick` 时务必通过 `reportErr` 报告调用结果
4. **优雅退出**：程序退出时调用 `RegistrationHandle.Stop()` 主动注销

## 性能考虑

- **Token 缓存**：token 会被缓存并自动续期，避免频繁认证
- **服务列表缓存**：服务实例列表有 30 秒缓存，减少注册中心压力
- **连接复用**：gRPC 连接会被复用
- **并发控制**：使用单飞模式避免并发刷新 token

## 故障处理

- **Token 续期失败**：自动 fallback 到重新认证
- **实例调用失败**：自动切换到其他健康实例
- **心跳断线**：自动重连，带指数退避
- **短期熔断**：失败的实例会被冷却 30 秒

## 限制

- 目前只支持不安全的 gRPC 连接（生产环境需要添加 TLS 支持）
- 注册中心集群负载均衡策略为 round-robin（下游服务发现也是 round-robin）
- 服务缓存刷新为固定 30 秒
- 集群地址为静态配置（不支持动态服务发现，如需动态发现请使用 DNS）

## License

与主项目相同
