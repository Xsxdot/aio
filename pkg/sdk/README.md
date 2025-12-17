# AIO Go gRPC SDK

AIO 平台的 Go 语言 gRPC SDK，提供服务发现、鉴权、注册等功能。

## 特性

- **自动鉴权**：基于 ClientKey/ClientSecret 自动获取和续期 JWT token
- **服务发现**：从注册中心拉取服务实例列表，支持缓存和刷新
- **负载均衡**：内置 round-robin 负载均衡策略
- **故障转移**：自动检测失败实例并进行短期熔断
- **服务注册**：支持注册自身到注册中心并保持心跳
- **配置中心**：查询配置（GetConfig/BatchGetConfigs）
- **短网址服务**：创建、查询、解析短链接，自动处理 JSON 序列化
- **应用部署**：流式上传产物、触发部署、查询部署状态
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
    // 创建 SDK 客户端
    client, err := sdk.New(sdk.Config{
        RegistryAddr: "localhost:50051",
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

### 配置中心

```go
// 获取单个配置
ctx := context.Background()
jsonStr, err := client.ConfigClient.GetConfigJSON(ctx, "database.config", "dev")
if err != nil {
    panic(err)
}
fmt.Println("Config:", jsonStr)

// 批量获取配置
configs, err := client.ConfigClient.BatchGetConfigs(ctx, 
    []string{"database.config", "redis.config"}, 
    "dev",
)
if err != nil {
    panic(err)
}
for key, value := range configs {
    fmt.Printf("%s: %s\n", key, value)
}
```

### 短网址服务

```go
ctx := context.Background()

// 创建短链接
resp, err := client.ShortURL.CreateShortLink(ctx, &sdk.CreateShortLinkRequest{
    DomainID:   1,
    TargetType: "url",
    TargetConfig: map[string]interface{}{
        "url": "https://example.com/long-url",
    },
    Comment: "示例短链接",
})
if err != nil {
    panic(err)
}
fmt.Printf("短链接: %s\n", resp.ShortURL)

// 解析短链接
resolveResp, err := client.ShortURL.Resolve(ctx, "short.domain.com", resp.Code, "")
if err != nil {
    panic(err)
}
fmt.Printf("目标类型: %s\n", resolveResp.TargetType)
fmt.Printf("建议动作: %s\n", resolveResp.Action)

// 上报成功
err = client.ShortURL.ReportSuccess(ctx, resp.Code, "event-123", map[string]interface{}{
    "source": "sdk-example",
})
```

### 应用部署

```go
ctx := context.Background()

// 上传产物（流式）
file, _ := os.Open("artifact.tar.gz")
defer file.Close()

uploadResp, err := client.Application.UploadArtifactFromReader(ctx,
    &applicationpb.ArtifactMeta{
        ApplicationId: 1,
        FileName:     "app-v1.0.0.tar.gz",
        ArtifactType: "backend",
        Size:         1024000,
        Sha256:       "abc123...",
        ContentType:  "application/gzip",
    },
    file,
    512*1024, // 512KB 分块
)
if err != nil {
    panic(err)
}
fmt.Printf("产物 ID: %d\n", uploadResp.ArtifactID)

// 触发部署
deployResp, err := client.Application.Deploy(ctx, &sdk.DeployRequest{
    ApplicationID:     1,
    Version:          "v1.0.0",
    BackendArtifactID: uploadResp.ArtifactID,
    Operator:         "sdk-user",
})
if err != nil {
    panic(err)
}
fmt.Printf("部署 ID: %d, 状态: %s\n", deployResp.DeploymentID, deployResp.Status)

// 查询部署状态
deployInfo, err := client.Application.GetDeployment(ctx, deployResp.DeploymentID)
if err != nil {
    panic(err)
}
fmt.Printf("部署状态: %s\n", deployInfo.Status)
```

## API 文档

### Client

主客户端，包含所有子客户端。

```go
type Client struct {
    Auth         *AuthClient         // 鉴权客户端
    Registry     *RegistryClient     // 注册中心客户端
    Discovery    *DiscoveryClient    // 服务发现客户端
    ConfigClient *ConfigClient       // 配置中心客户端
    ShortURL     *ShortURLClient     // 短网址客户端
    Application  *ApplicationClient  // 应用部署客户端
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

### ConfigClient

配置中心查询客户端（只读接口）。

```go
type ConfigClient struct { ... }

// 获取单个配置（返回 JSON 字符串）
func (c *ConfigClient) GetConfigJSON(ctx context.Context, key, env string) (string, error)

// 批量获取配置
func (c *ConfigClient) BatchGetConfigs(ctx context.Context, keys []string, env string) (map[string]string, error)
```

**注意**：管理端接口（创建/更新/删除配置）需要 admin token，SDK 的 client-credentials 认证不支持，请使用 HTTP Admin API。

### ShortURLClient

短网址服务客户端。

```go
type ShortURLClient struct { ... }

// 创建短链接
func (c *ShortURLClient) CreateShortLink(ctx context.Context, req *CreateShortLinkRequest) (*CreateShortLinkResponse, error)

// 获取短链接详情
func (c *ShortURLClient) GetShortLink(ctx context.Context, id int64) (*ShortLinkInfo, error)

// 查询短链接列表
func (c *ShortURLClient) ListShortLinks(ctx context.Context, domainID int64, page, size int32) ([]*ShortLinkInfo, int64, error)

// 解析短链接（返回目标配置和建议动作）
func (c *ShortURLClient) Resolve(ctx context.Context, host, code, password string) (*ResolveResponse, error)

// 上报跳转成功（无鉴权）
func (c *ShortURLClient) ReportSuccess(ctx context.Context, code, eventID string, attrs map[string]interface{}) error
```

**特性**：自动处理 JSON 序列化/反序列化，`TargetConfig` 和 `Attrs` 直接使用 `map[string]interface{}`。

### ApplicationClient

应用部署客户端。

```go
type ApplicationClient struct { ... }

// 触发部署
func (c *ApplicationClient) Deploy(ctx context.Context, req *DeployRequest) (*DeployResponse, error)

// 获取部署状态
func (c *ApplicationClient) GetDeployment(ctx context.Context, deploymentID int64) (*DeploymentInfo, error)

// 流式上传产物（易用封装）
func (c *ApplicationClient) UploadArtifactFromReader(
    ctx context.Context,
    meta *applicationpb.ArtifactMeta,
    r io.Reader,
    chunkSize int,
) (*UploadArtifactResponse, error)
```

**特性**：
- `UploadArtifactFromReader` 自动处理流式上传，默认 512KB 分块
- 支持从任何 `io.Reader` 上传（文件、网络流、内存等）

### RegistrationHandle

服务注册句柄，用于管理心跳和注销。

```go
type RegistrationHandle struct { ... }

// 停止心跳并注销实例
func (h *RegistrationHandle) Stop() error
```

## 配置

**重要说明**：`RegistryAddr` 实际上是 **AIO 平台的 gRPC 服务地址**（统一对外服务地址），所有 gRPC 服务（Registry、Config、ShortURL、Application 等）都在这个地址上提供服务。

```go
type Config struct {
    // RegistryAddr AIO 平台 gRPC 服务地址（必填）
    // 例如：localhost:50051 或 aio.example.com:50051
    // 所有服务（registry/config/shorturl/application）都在此地址上
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

## 示例

完整的示例程序请参考 [`example/main.go`](./example/main.go)。

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
- 负载均衡策略仅支持 round-robin
- 服务缓存刷新为固定 30 秒

## License

与主项目相同
