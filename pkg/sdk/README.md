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

### 任务执行器

```go
// 提交任务（DedupKey 为必填项，用于幂等去重）
ctx := context.Background()
jobID, err := client.Executor.SubmitJob(ctx, &sdk.SubmitJobRequest{
    TargetService: "my-service",
    Method:        "processData",
    ArgsJSON:      `{"input":"data"}`,
    MaxAttempts:   3,
    Priority:      5,
    DedupKey:      "my-service:processData:unique-id-123",
})
if err != nil {
    panic(err)
}
fmt.Println("Job submitted:", jobID)

// 或使用便捷方法（自动序列化参数，dedupKey 为必填第三个参数）
jobID, err = client.Executor.SubmitJobWithArgs(ctx, "my-service", "processData",
    "my-service:processData:unique-id-123",
    map[string]interface{}{"input": "data"},
    sdk.WithMaxAttempts(3),
    sdk.WithPriority(5),
)

// Worker 侧：领取任务（可选按 method 过滤）
job, err := client.Executor.AcquireJob(ctx, &sdk.AcquireJobRequest{
    TargetService: "my-service",
    Method:        "",        // 可选：指定方法名过滤，空表示领取所有方法的任务
    ConsumerID:    "worker-1",
    LeaseDuration: 30, // 30秒租约
})
if err != nil {
    panic(err)
}

if job == nil {
    fmt.Println("No job available")
} else {
    fmt.Printf("Acquired job %d: %s.%s\n", job.JobID, job.TargetService, job.Method)
    
    // 执行任务...
    // time.Sleep(10 * time.Second)
    
    // 如果任务执行时间较长，需要续租
    newLeaseUntil, err := client.Executor.RenewLease(ctx, job.JobID, job.AttemptNo, "worker-1", 30)
    if err != nil {
        fmt.Println("Renew lease failed:", err)
    } else {
        fmt.Println("Lease renewed until:", newLeaseUntil)
    }
    
    // 确认任务完成（成功）
    err = client.Executor.AckJob(ctx, &sdk.AckJobRequest{
        JobID:      job.JobID,
        AttemptNo:  job.AttemptNo,
        ConsumerID: "worker-1",
        Status:     sdk.AckStatusSucceeded,
        ResultJSON: `{"status":"done"}`,
    })
    if err != nil {
        panic(err)
    }
    
    // 或者报告失败（会自动重试）
    // err = client.Executor.AckJob(ctx, &sdk.AckJobRequest{
    //     JobID:      job.JobID,
    //     AttemptNo:  job.AttemptNo,
    //     ConsumerID: "worker-1",
    //     Status:     sdk.AckStatusFailed,
    //     Error:      "processing failed",
    //     RetryAfter: 60, // 60秒后重试
    // })
}

// 按方法过滤示例：创建只处理特定方法的 Worker
emailJob, err := client.Executor.AcquireJob(ctx, &sdk.AcquireJobRequest{
    TargetService: "my-service",
    Method:        "SendEmailNotification",  // 只领取邮件通知任务
    ConsumerID:    "email-worker-1",
    LeaseDuration: 30,
})
// 处理 emailJob...
```

### 开箱即用的 Worker（推荐）

SDK 提供了基于 `pkg/scheduler` 的开箱即用 Worker，自动处理任务拉取、执行、续租和 Ack。

```go
import (
    "context"
    "encoding/json"
    "log"
    
    "xiaozhizhang/pkg/scheduler"
    "xiaozhizhang/pkg/sdk"
)

// 1. 创建 SDK 客户端
client, err := sdk.New(sdk.Config{
    RegistryAddr: "localhost:50051",
    ClientKey:    "your-client-key",
    ClientSecret: "your-client-secret",
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 2. 创建 Worker（方式 A：内部自建 scheduler）
config := sdk.DefaultWorkerConfig("my-service")
config.LeaseDuration = 30        // 租约 30 秒
config.EnableAutoRenew = true    // 启用自动续租
worker, err := client.Executor.NewWorker(config)
if err != nil {
    log.Fatal(err)
}

// 或者方式 B：使用外部 scheduler（推荐，便于统一管理）
schedulerConfig := scheduler.DefaultSchedulerConfig()
s := scheduler.NewScheduler(schedulerConfig)
if err := s.Start(); err != nil {
    log.Fatal(err)
}
defer s.Stop()

worker, err := client.Executor.NewWorkerWithScheduler(s, config, false)
if err != nil {
    log.Fatal(err)
}

// 3. 注册方法处理器
// 邮件通知处理器
err = worker.Register("SendEmailNotification", func(ctx context.Context, job *sdk.AcquiredJob) (interface{}, error) {
    // 解析参数
    var args struct {
        UserID int    `json:"user_id"`
        Email  string `json:"email"`
        Title  string `json:"title"`
        Body   string `json:"body"`
    }
    if err := json.Unmarshal([]byte(job.ArgsJSON), &args); err != nil {
        return nil, err
    }
    
    // 执行业务逻辑
    log.Printf("发送邮件: user_id=%d, email=%s, title=%s", args.UserID, args.Email, args.Title)
    // ... 实际发送邮件 ...
    
    // 返回结果（会自动序列化为 JSON）
    return map[string]interface{}{
        "sent_at": time.Now().Unix(),
        "status":  "success",
    }, nil
})
if err != nil {
    log.Fatal(err)
}

// 支付处理器（带重试延迟）
err = worker.Register("ProcessPayment", func(ctx context.Context, job *sdk.AcquiredJob) (interface{}, error) {
    var args struct {
        OrderID string  `json:"order_id"`
        Amount  float64 `json:"amount"`
    }
    if err := json.Unmarshal([]byte(job.ArgsJSON), &args); err != nil {
        return nil, err
    }
    
    // 执行支付
    log.Printf("处理支付: order_id=%s, amount=%.2f", args.OrderID, args.Amount)
    
    // 如果支付失败，可以指定重试延迟
    if args.Amount <= 0 {
        return nil, &sdk.JobFailedError{
            Message:    "invalid amount",
            RetryAfter: 60, // 60 秒后重试
        }
    }
    
    // ... 实际支付逻辑 ...
    
    return map[string]interface{}{"order_id": args.OrderID}, nil
})
if err != nil {
    log.Fatal(err)
}

// 4. 启动 Worker
if err := worker.Start(); err != nil {
    log.Fatal(err)
}
defer worker.Stop()

// Worker 会自动：
// - 每秒拉取一次任务（每个注册的方法一个独立的定时任务）
// - 执行处理器函数
// - 长任务自动续租（默认每 LeaseDuration/3 续租一次）
// - 自动 Ack 成功/失败
// - 捕获 panic 并 Ack 失败

log.Println("Worker 已启动，按 Ctrl+C 退出")
select {} // 保持运行
```

#### Worker 高级配置

```go
config := &sdk.WorkerConfig{
    TargetService:   "my-service",
    ConsumerID:      "worker-1",           // 消费者 ID
    LeaseDuration:   30,                    // 租约时长（秒）
    EnableAutoRenew: true,                  // 启用自动续租
    RenewInterval:   10 * time.Second,      // 续租间隔（默认 LeaseDuration/3）
    ExtendDuration:  30,                    // 续租延长时长（秒，默认使用 LeaseDuration）
    TaskTimeout:     25 * time.Second,      // 任务超时时间（应小于 LeaseDuration）
    PollInterval:    1 * time.Second,       // 轮询间隔
    
    // 结果序列化失败回调
    OnResultSerializeError: func(job *sdk.AcquiredJob, result interface{}, err error) {
        log.Printf("结果序列化失败: job_id=%d, error=%v", job.JobID, err)
        // 默认行为：仍然上报成功（避免任务重复执行）
    },
    
    // 续租失败回调
    OnRenewLeaseError: func(job *sdk.AcquiredJob, err error) {
        log.Printf("续租失败: job_id=%d, error=%v", job.JobID, err)
    },
}
```

#### Worker 特性

- **开箱即用**：注册方法后自动拉取、执行、Ack，无需手写循环
- **自动续租**：长任务执行期间自动续租，避免租约过期
- **故障容错**：
  - 自动捕获 panic 并 Ack 失败
  - Ack 使用独立短超时 context，避免被任务 context 取消
  - 续租失败不会导致 panic，会继续尝试 Ack
- **灵活重试**：通过 `JobFailedError{RetryAfter}` 控制重试延迟
- **结果序列化**：handler 返回结果自动序列化为 JSON
- **并发控制**：通过 scheduler 的 `MaxWorkers` 控制并发数
- **优雅退出**：`Stop()` 会等待所有正在执行的任务完成

## API 文档

### Client

主客户端，包含所有子客户端。

```go
type Client struct {
    Auth      *AuthClient      // 鉴权客户端
    Registry  *RegistryClient  // 注册中心客户端
    Discovery *DiscoveryClient // 服务发现客户端
    Executor  *ExecutorClient  // 任务执行器客户端
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

### ExecutorClient

任务执行器客户端。

```go
type ExecutorClient struct { ... }

// 提交任务（req.DedupKey 为必填项，空值会在客户端直接返回 InvalidArgument 错误）
func (c *ExecutorClient) SubmitJob(ctx context.Context, req *SubmitJobRequest) (int64, error)

// 提交任务（自动序列化参数，dedupKey 为必填参数）
func (c *ExecutorClient) SubmitJobWithArgs(ctx context.Context, targetService, method, dedupKey string, args interface{}, opts ...SubmitJobOption) (int64, error)

// 领取任务（Worker 侧，低级 API）
func (c *ExecutorClient) AcquireJob(ctx context.Context, req *AcquireJobRequest) (*AcquiredJob, error)

// 续租（长任务周期性调用，低级 API）
func (c *ExecutorClient) RenewLease(ctx context.Context, jobID int64, attemptNo int32, consumerID string, extendDuration int32) (int64, error)

// 确认任务执行结果（低级 API）
func (c *ExecutorClient) AckJob(ctx context.Context, req *AckJobRequest) error

// 创建开箱即用的 Worker（内部自建 scheduler）
func (c *ExecutorClient) NewWorker(config *WorkerConfig) (*ExecutorWorker, error)

// 创建开箱即用的 Worker（使用外部 scheduler）
func (c *ExecutorClient) NewWorkerWithScheduler(s *scheduler.Scheduler, config *WorkerConfig, ownScheduler bool) (*ExecutorWorker, error)
```

### ExecutorWorker

开箱即用的任务消费者（基于 `pkg/scheduler`）。

```go
type ExecutorWorker struct { ... }

// 注册方法处理器
func (w *ExecutorWorker) Register(method string, handler JobHandler) error

// 注销方法处理器
func (w *ExecutorWorker) Unregister(method string) error

// 启动 Worker
func (w *ExecutorWorker) Start() error

// 停止 Worker
func (w *ExecutorWorker) Stop() error

// 检查 Worker 是否正在运行
func (w *ExecutorWorker) IsRunning() bool

// 获取 Scheduler（用于观测）
func (w *ExecutorWorker) GetScheduler() *scheduler.Scheduler
```

#### JobHandler

任务处理函数类型。

```go
type JobHandler func(ctx context.Context, job *AcquiredJob) (result interface{}, err error)
```

#### JobFailedError

任务失败错误（带重试延迟）。

```go
type JobFailedError struct {
    Message    string // 错误信息
    RetryAfter int32  // 重试延迟（秒），0 表示使用默认退避策略
}
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
