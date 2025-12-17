# SDK 项目结构

```
pkg/sdk/
├── README.md                    # API 文档和使用指南
├── IMPLEMENTATION.md            # 实施总结和技术细节
├── DELIVERY.md                  # 交付报告和验收结果
│
├── sdk.go                       # 主客户端和配置 (105 行)
│   ├── type Config
│   ├── type Client
│   ├── func New(config) (*Client, error)
│   ├── func (*Client) Close() error
│   └── func (*Client) DefaultContext()
│
├── auth.go                      # 认证客户端 (152 行)
│   ├── type AuthClient
│   ├── func newAuthClient(client) (*AuthClient, error)
│   ├── func (*AuthClient) Token(ctx) (string, error)
│   ├── func (*AuthClient) authenticate(ctx)
│   ├── func (*AuthClient) renewToken(ctx)
│   └── func (*AuthClient) refreshToken(ctx)
│
├── interceptor.go               # gRPC 拦截器 (52 行)
│   ├── func (*Client) unaryAuthInterceptor()
│   ├── func (*Client) streamAuthInterceptor()
│   └── func (*Client) injectToken(ctx, token)
│
├── error.go                     # 错误处理 (65 行)
│   ├── type Error
│   ├── func WrapError(err, message) error
│   ├── func IsNotFound(err) bool
│   ├── func IsUnauthenticated(err) bool
│   └── func IsUnavailable(err) bool
│
├── registry.go                  # 注册中心客户端 (145 行)
│   ├── type RegistryClient
│   ├── type ServiceDescriptor
│   ├── type InstanceEndpoint
│   ├── type RegisterInstanceRequest
│   ├── type RegisterInstanceResponse
│   ├── func newRegistryClient(client, conn)
│   ├── func (*RegistryClient) ListServices(ctx, project, env)
│   ├── func (*RegistryClient) GetServiceByID(ctx, serviceID)
│   ├── func (*RegistryClient) RegisterInstance(ctx, req)
│   ├── func (*RegistryClient) DeregisterInstance(ctx, serviceID, key)
│   └── func (*RegistryClient) RegisterSelf(ctx, req)
│
├── discovery.go                 # 服务发现和负载均衡 (174 行)
│   ├── type DiscoveryClient
│   ├── type serviceCache
│   ├── type instanceFailure
│   ├── func newDiscoveryClient(client)
│   ├── func (*DiscoveryClient) Resolve(ctx, project, name, env)
│   ├── func (*DiscoveryClient) Pick(project, name, env)
│   ├── func (*DiscoveryClient) markInstanceFailed(endpoint)
│   └── func (*DiscoveryClient) RefreshService(ctx, project, name, env)
│
├── heartbeat.go                 # 服务注册和心跳 (128 行)
│   ├── type RegistrationHandle
│   ├── func (*RegistrationHandle) Stop() error
│   ├── func (*RegistrationHandle) heartbeatLoop()
│   └── func (*RegistrationHandle) sendHeartbeatStream()
│
├── config_client.go             # 配置中心客户端 (~60 行)
│   ├── type ConfigClient
│   ├── func newConfigClient(conn)
│   ├── func (*ConfigClient) GetConfigJSON(ctx, key, env)
│   └── func (*ConfigClient) BatchGetConfigs(ctx, keys, env)
│
├── shorturl_client.go           # 短网址客户端 (~230 行)
│   ├── type ShortURLClient
│   ├── type CreateShortLinkRequest
│   ├── type ShortLinkInfo
│   ├── type ResolveResponse
│   ├── func newShortURLClient(conn)
│   ├── func (*ShortURLClient) CreateShortLink(ctx, req)
│   ├── func (*ShortURLClient) GetShortLink(ctx, id)
│   ├── func (*ShortURLClient) ListShortLinks(ctx, domainID, page, size)
│   ├── func (*ShortURLClient) Resolve(ctx, host, code, password)
│   └── func (*ShortURLClient) ReportSuccess(ctx, code, eventID, attrs)
│
├── application_client.go        # 应用部署客户端 (~100 行)
│   ├── type ApplicationClient
│   ├── type DeployRequest
│   ├── type DeployResponse
│   ├── type DeploymentInfo
│   ├── func newApplicationClient(conn)
│   ├── func (*ApplicationClient) Deploy(ctx, req)
│   └── func (*ApplicationClient) GetDeployment(ctx, deploymentID)
│
├── application_upload.go        # 流式上传封装 (~70 行)
│   ├── type UploadArtifactResponse
│   └── func (*ApplicationClient) UploadArtifactFromReader(ctx, meta, r, chunkSize)
│
├── sdk_test.go                  # 单元测试 (~180 行)
│   ├── func TestConfig(t)
│   ├── func TestConfigDefaults(t)
│   ├── func TestErrorWrapping(t)
│   ├── func TestServiceDescriptor(t)
│   ├── func TestClientStructure(t)
│   ├── func TestShortURLRequestStructure(t)
│   └── func TestDeployRequestStructure(t)
│
└── example/                     # 示例程序
    ├── README.md                # 示例说明
    └── main.go                  # 完整示例 (~200 行)
        ├── Step 1: 认证
        ├── Step 2: 拉取服务列表
        ├── Step 3: 服务发现
        ├── Step 4: 配置中心（可选）
        ├── Step 5: 短网址（可选）
        ├── Step 6: 注册自身
        └── Step 7: 运行和优雅退出
```

## 模块关系

```
                    +----------------+
                    |   Client       |
                    |  (主客户端)     |
                    +-------+--------+
                            |
           +----------------+----------------+
           |                |                |
     +-----v------+   +-----v------+   +-----v--------+
     | AuthClient |   | Registry   |   | Discovery    |
     | (认证)      |   | Client     |   | Client       |
     +------------+   +-----+------+   +-----+--------+
                            |                |
                            |                |
                      +-----v------+   +-----v--------+
                      | Heartbeat  |   | Load Balance |
                      | (心跳)      |   | (负载均衡)    |
                      +------------+   +--------------+
```

## 数据流

### 认证流程

```
Client.Auth.Token(ctx)
  |
  +-> 检查缓存 (token + expiresAt)
  |     |
  |     +-> 有效: 返回 token
  |     |
  |     +-> 过期/无效: 刷新 token
  |           |
  |           +-> 尝试续期 (RenewToken)
  |           |     |
  |           |     +-> 成功: 更新缓存
  |           |     |
  |           |     +-> 失败: Fallback
  |           |
  |           +-> 重新认证 (AuthenticateClient)
  |                 |
  |                 +-> 更新缓存
  |
  +-> 返回 token
```

### 服务发现流程

```
Client.Discovery.Pick(project, name, env)
  |
  +-> 检查服务缓存
  |     |
  |     +-> 缓存有效 (< 30s): 使用缓存
  |     |
  |     +-> 缓存过期/不存在: 刷新
  |           |
  |           +-> ListServices(project, env)
  |           |
  |           +-> 更新缓存
  |
  +-> 过滤健康实例 (排除熔断的)
  |
  +-> Round-robin 选择
  |
  +-> 返回 (instance, reportErr, nil)
```

### 心跳流程

```
Client.Registry.RegisterSelf(ctx, req)
  |
  +-> RegisterInstance
  |     |
  |     +-> 获取 instanceKey + expiresAt
  |
  +-> 创建 RegistrationHandle
  |
  +-> 启动心跳 goroutine
        |
        +-> 循环:
              |
              +-> 等待 (TTL / 3)
              |
              +-> HeartbeatStream.Send(req)
              |
              +-> HeartbeatStream.Recv()
              |     |
              |     +-> 成功: 继续
              |     |
              |     +-> 失败: 重连 (指数退避)
              |
              +-> 检查 ctx.Done()
                    |
                    +-> 退出
```

## 并发安全

### AuthClient

- **RWMutex**: 保护 token 和 expiresAt
- **Cond**: 协调并发刷新，避免重复刷新
- **双重检查**: 减少锁竞争

### DiscoveryClient

- **RWMutex**: 保护服务缓存和故障记录
- **原子操作**: Round-robin 索引递增

### RegistrationHandle

- **Mutex**: 保护 stopped 标志
- **WaitGroup**: 等待 goroutine 退出
- **Context**: 取消心跳循环

## 配置项

```go
Config {
    RegistryAddr   string        // 注册中心地址 (必填)
    ClientKey      string        // 客户端 Key (必填)
    ClientSecret   string        // 客户端 Secret (必填)
    DefaultTimeout time.Duration // 默认超时 (可选, 默认 30s)
    DisableAuth    bool          // 禁用鉴权 (可选, 默认 false)
}
```

## 常量和默认值

| 项目 | 值 | 说明 |
|------|-----|------|
| DefaultTimeout | 30s | 默认超时时间 |
| Token 刷新提前时间 | 5min | 过期前 5 分钟刷新 |
| 服务列表缓存 TTL | 30s | 服务实例列表缓存时间 |
| 实例熔断时间 | 30s | 失败实例冷却期 |
| 心跳间隔 | TTL/3 | 最小 10s |
| 心跳最大重试延迟 | 30s | 指数退避上限 |
