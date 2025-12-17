# Facade 层架构说明

## 核心设计：单接口隔离

### 问题
- SSL 组件需要调用 server 组件获取 SSH 配置
- 如果在 `ssl/internal/app` 直接依赖 `server/api/client.ServerClient`，会产生耦合

### 解决方案
使用**单接口隔离**实现依赖倒置：

```
ssl/internal/facade/
└── server_facade.go           # 定义 IServerFacade 接口
    └── IServerFacade          # ssl 需要的 server 能力（抽象）
```

### 关键点

1. **ssl/internal/app 只依赖抽象接口**
   - 持有 `facade.IServerFacade` 字段
   - 不知道具体实现是什么

2. **server/api/client.ServerClient 隐式实现接口**
   - 无需修改 server 组件代码
   - 符合 Go 的隐式接口实现

3. **只有组装根（module.go）知道具体类型**
   - 接收 `*serverclient.ServerClient`
   - 传给 app 时自动转为 `IServerFacade` 接口

## 依赖关系图

```
┌─────────────────────────────────────────────────────┐
│              ssl/module.go                          │
│  (唯一依赖 server/api/client 的地方)                │
│                                                     │
│  internalApp := app.NewApp(                         │
│      serverClient  // *serverclient.ServerClient    │
│  )                 // 自动转为 facade.IServerFacade │
└─────────────────────────────────────────────────────┘
                        │
                        │ 注入接口
                        ▼
┌─────────────────────────────────────────────────────┐
│          ssl/internal/app/                          │
│                                                     │
│  type App struct {                                  │
│      serverFacade facade.IServerFacade              │
│  }                                                  │
│                                                     │
│  // 只调用接口方法                                   │
│  sshConfig, err := a.serverFacade.                  │
│      GetServerSSHConfigByID(ctx, serverID)          │
└─────────────────────────────────────────────────────┘
                        ▲
                        │ 隐式实现
                        │
┌─────────────────────────────────────────────────────┐
│       server/api/client.ServerClient                │
│       (隐式实现 IServerFacade 接口)                  │
│                                                     │
│  func (c *ServerClient) GetServerSSHConfigByID(     │
│      ctx, serverID) (*dto.ServerSSHConfig, error)   │
└─────────────────────────────────────────────────────┘
```

## 优势

✅ **简洁明了**：只有一个接口，无冗余抽象  
✅ **依赖隔离**：ssl/internal/app 不直接依赖 server 包  
✅ **易测试**：可以轻松 mock `IServerFacade` 接口  
✅ **灵活替换**：未来可以换成任何实现（gRPC、HTTP 客户端等）  
✅ **符合 DIP**：依赖抽象（接口），不依赖具体实现  

## 为什么不需要 Adapter？

当前场景下，`server.ServerClient` 的方法签名已经和 `IServerFacade` 完全一致，可以**直接隐式实现接口**，无需额外的适配器层。

### 什么时候需要 Adapter？

只有以下情况才需要引入适配器：

- **需要转换**：DTO 结构不一致，需要字段映射
- **需要组合**：一次 facade 调用内部要调用多个 server 方法并聚合
- **需要治理**：缓存、重试、熔断、限流、降级
- **需要兼容**：同时支持多种数据源（DB、RPC、HTTP）

如果只是"原样转发"，adapter 就是噪音。

## 使用示例

### 在 App 层使用 Facade

```go
// system/ssl/internal/app/deploy_manage.go

func (a *App) resolveSSHConfig(ctx context.Context, target *model.DeployTarget) (*model.DeployTarget, error) {
    // 通过 facade 接口调用 server 组件
    sshConfig, err := a.serverFacade.GetServerSSHConfigByID(ctx, refConfig.ServerID)
    if err != nil {
        return nil, a.err.New("获取服务器SSH配置失败", err)
    }
    
    // 使用返回的配置
    runtimeConfig := model.SSHDeployRuntimeConfig{
        Host:       sshConfig.Host,
        Port:       sshConfig.Port,
        Username:   sshConfig.Username,
        // ...
    }
    
    return runtimeTarget, nil
}
```

### 在单元测试中 Mock Facade

```go
// system/ssl/internal/app/deploy_manage_test.go

type mockServerFacade struct{}

func (m *mockServerFacade) GetServerSSHConfigByID(ctx context.Context, serverID int64) (*serverdto.ServerSSHConfig, error) {
    return &serverdto.ServerSSHConfig{
        Host:       "192.168.1.100",
        Port:       22,
        Username:   "root",
        AuthMethod: "password",
        Password:   "test123",
    }, nil
}

func TestDeploySSH(t *testing.T) {
    mockFacade := &mockServerFacade{}
    app := app.NewApp(mockFacade)
    
    // 测试部署逻辑，无需依赖真实的 server 组件
    err := app.deployCertificateToTarget(ctx, cert, targetID, "manual")
    assert.NoError(t, err)
}
```

## 扩展指南

当需要添加新的跨组件依赖时：

1. **在 `facade/` 定义新接口**：如 `IApplicationFacade`
2. **在 `app.go` 持有接口字段**：如 `applicationFacade IApplicationFacade`
3. **在 `module.go` 注入具体实现**：如 `app.NewApp(serverClient, applicationClient)`
4. **确保对方 Client 实现了接口**：方法签名一致即可（Go 隐式实现）

## 核心原则

- **ssl/internal/app** 只依赖 `facade.IServerFacade` 接口
- **ssl/module.go** 负责将具体类型注入到 app
- **facade 包** 只定义接口，不包含任何实现
- **保持简洁**：不需要的适配器就不要加
