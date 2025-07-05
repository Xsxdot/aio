# 服务器管理组件

服务器管理组件提供了完整的服务器生命周期管理功能，包括服务器的注册、配置、连接测试和健康检查等。

## 功能特性

### 🏗️ 服务器管理
- **服务器注册**: 支持添加新的服务器到管理系统
- **服务器配置**: 灵活的服务器配置管理，支持多种认证方式
- **服务器更新**: 动态更新服务器配置信息
- **服务器删除**: 安全删除服务器及相关数据

### 🔐 认证支持
- **SSH密钥认证**: 支持RSA、Ed25519等多种SSH密钥类型
- **密码认证**: 支持用户名密码认证方式
- **密钥文件认证**: 支持本地密钥文件认证

### 🌐 连接管理
- **连接测试**: 实时测试服务器连接状态
- **连接池**: 复用SSH连接，提高性能
- **超时控制**: 配置连接超时和重试策略

### 📊 健康检查
- **定时检查**: 定期检查服务器健康状态
- **批量检查**: 支持批量服务器健康检查
- **历史记录**: 保存健康检查历史数据
- **状态监控**: 实时监控服务器在线状态

### 🏷️ 标签管理
- **服务器标签**: 支持给服务器添加自定义标签
- **标签过滤**: 基于标签进行服务器筛选和管理
- **批量操作**: 支持基于标签的批量操作

### 监控管理

#### 获取监控节点IP
```http
GET /api/servers/{serverId}/monitor-node
```

返回指定服务器的监控节点IP和端口信息。

**响应示例：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "ip": "192.168.1.100",
    "port": "9999"
  }
}
```

#### 获取监控分配信息
```http
GET /api/servers/{serverId}/monitor-assignment
```

获取服务器的详细监控分配信息，包括分配的节点、分配时间等。

**响应示例：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "server_id": "server-001",
    "server_name": "Web服务器1",
    "assigned_node": "node-001",
    "assign_time": "2024-01-01T12:00:00Z"
  }
}
```

#### 重新分配监控节点
```http
POST /api/servers/{serverId}/monitor-reassign
```

将服务器的监控任务重新分配给指定的节点。

**请求参数：**
```json
{
  "nodeId": "node-002"
}
```

**响应示例：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "监控节点重新分配成功",
    "serverId": "server-001",
    "nodeId": "node-002"
  }
}
```

## 架构设计

### 组件结构
```
pkg/server/
├── types.go              # 类型定义
├── service.go             # 服务接口和实现
├── storage.go             # 存储接口和ETCD实现
├── credential_adapter.go  # 密钥服务适配器
└── README.md             # 组件文档
```

### 接口设计

#### Service 接口
```go
type Service interface {
    // 服务器管理
    CreateServer(ctx context.Context, req *ServerCreateRequest) (*Server, error)
    GetServer(ctx context.Context, id string) (*Server, error)
    UpdateServer(ctx context.Context, id string, req *ServerUpdateRequest) (*Server, error)
    DeleteServer(ctx context.Context, id string) error
    ListServers(ctx context.Context, req *ServerListRequest) ([]*Server, int, error)

    // 连接测试
    TestConnection(ctx context.Context, req *ServerTestConnectionRequest) (*ServerTestConnectionResult, error)

    // 健康检查
    PerformHealthCheck(ctx context.Context, serverID string) (*ServerHealthCheck, error)
    BatchHealthCheck(ctx context.Context, serverIDs []string) ([]*ServerHealthCheck, error)
    GetServerHealth(ctx context.Context, serverID string) (*ServerHealthCheck, error)
    GetServerHealthHistory(ctx context.Context, serverID string, limit int) ([]*ServerHealthCheck, error)
}
```

#### Storage 接口
```go
type Storage interface {
    // 服务器管理
    CreateServer(ctx context.Context, server *Server) error
    GetServer(ctx context.Context, id string) (*Server, error)
    UpdateServer(ctx context.Context, server *Server) error
    DeleteServer(ctx context.Context, id string) error
    ListServers(ctx context.Context, req *ServerListRequest) ([]*Server, int, error)

    // 服务器查询
    GetServersByIDs(ctx context.Context, ids []string) ([]*Server, error)
    GetServersByTags(ctx context.Context, tags []string) ([]*Server, error)

    // 健康检查
    UpdateServerStatus(ctx context.Context, id string, status ServerStatus) error
    SaveHealthCheck(ctx context.Context, check *ServerHealthCheck) error
    GetLatestHealthCheck(ctx context.Context, serverID string) (*ServerHealthCheck, error)
    GetHealthCheckHistory(ctx context.Context, serverID string, limit int) ([]*ServerHealthCheck, error)
}
```

#### CredentialProvider 接口
```go
type CredentialProvider interface {
    // GetCredentialContent 获取密钥内容
    GetCredentialContent(ctx context.Context, id string) (string, error)
    // TestCredential 测试密钥连接
    TestCredential(ctx context.Context, id string, host string, port int, username string) error
}
```

## 使用示例

### 创建服务器管理服务

```go
import (
    "github.com/xsxdot/aio/pkg/server"
    "github.com/xsxdot/aio/pkg/credential"
    "github.com/xsxdot/aio/internal/etcd"
)

// 创建ETCD客户端
etcdClient := etcd.NewEtcdClient(etcdConfig)

// 创建服务器存储
serverStorage := server.NewETCDStorage(server.ETCDStorageConfig{
    Client: etcdClient,
    Logger: logger,
})

// 创建密钥服务
credentialStorage, _ := credential.NewETCDStorage(credential.ETCDStorageConfig{
    Client:     etcdClient,
    Logger:     logger,
    EncryptKey: "your-encryption-key",
})

credentialService := credential.NewService(credential.Config{
    Storage: credentialStorage,
    Logger:  logger,
})

// 创建密钥服务适配器
credentialProvider := server.NewCredentialServiceAdapter(credentialService)

// 创建服务器管理服务
serverService := server.NewService(server.Config{
    Storage:            serverStorage,
    CredentialProvider: credentialProvider,
    Logger:             logger,
})
```

### 注册服务器

```go
// 创建服务器
req := &server.ServerCreateRequest{
    Name:         "生产服务器-1",
    Host:         "192.168.1.100",
    Port:         22,
    Username:     "root",
    AuthType:     server.AuthTypeSSHKey,
    CredentialID: "cred-ssh-key-123",
    Description:  "生产环境Web服务器",
    Tags: map[string]string{
        "env":  "production",
        "role": "web",
    },
}

srv, err := serverService.CreateServer(ctx, req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("服务器创建成功: %s\n", srv.ID)
```

### 测试服务器连接

```go
testReq := &server.ServerTestConnectionRequest{
    Host:         "192.168.1.100",
    Port:         22,
    Username:     "root",
    AuthType:     server.AuthTypeSSHKey,
    CredentialID: "cred-ssh-key-123",
}

result, err := serverService.TestConnection(ctx, testReq)
if err != nil {
    log.Fatal(err)
}

if result.Success {
    fmt.Printf("连接测试成功，延迟: %dms\n", result.Latency)
} else {
    fmt.Printf("连接测试失败: %s\n", result.Message)
}
```

### 健康检查

```go
// 单个服务器健康检查
healthCheck, err := serverService.PerformHealthCheck(ctx, srv.ID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("服务器状态: %s, 延迟: %dms\n", healthCheck.Status, healthCheck.Latency)

// 批量健康检查
serverIDs := []string{srv.ID, "other-server-id"}
healthChecks, err := serverService.BatchHealthCheck(ctx, serverIDs)
if err != nil {
    log.Fatal(err)
}

for _, check := range healthChecks {
    fmt.Printf("服务器 %s 状态: %s\n", check.ServerID, check.Status)
}
```

### 服务器列表查询

```go
// 查询所有在线服务器
listReq := &server.ServerListRequest{
    Limit:  20,
    Offset: 0,
    Status: "online",
    Tags: map[string]string{
        "env": "production",
    },
}

servers, total, err := serverService.ListServers(ctx, listReq)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("找到 %d 台在线生产服务器，总共 %d 台\n", len(servers), total)
for _, srv := range servers {
    fmt.Printf("- %s (%s:%d) - %s\n", srv.Name, srv.Host, srv.Port, srv.Status)
}
```

## 依赖关系

### 外部依赖
- `github.com/xsxdot/aio/internal/etcd`: ETCD客户端
- `github.com/xsxdot/aio/pkg/credential`: 密钥管理组件
- `golang.org/x/crypto/ssh`: SSH连接支持
- `go.uber.org/zap`: 日志记录

### 组件特性
- **无状态设计**: 服务本身无状态，所有数据存储在ETCD中
- **接口驱动**: 通过接口实现依赖注入，便于测试和扩展
- **并发安全**: 支持多实例并发访问
- **可观测性**: 集成日志记录和错误处理

## 配置项

### 服务器配置
- `Name`: 服务器名称（必填）
- `Host`: 服务器地址（必填）
- `Port`: SSH端口，默认22
- `Username`: 登录用户名（必填）
- `AuthType`: 认证类型（必填）
- `CredentialID`: 密钥ID（必填）
- `Description`: 服务器描述
- `Tags`: 服务器标签

### 存储配置
- `Client`: ETCD客户端实例
- `Logger`: 日志记录器

## 最佳实践

### 1. 服务器命名
- 使用有意义的名称，包含环境和用途信息
- 避免使用特殊字符和空格
- 建议格式：`{环境}-{用途}-{序号}`

### 2. 标签使用
- 使用标准的标签键名，如 `env`、`role`、`zone` 等
- 标签值保持简洁明确
- 利用标签进行服务器分组和批量操作

### 3. 健康检查
- 定期执行健康检查，建议间隔1-5分钟
- 监控健康检查失败率，及时处理异常
- 保留适当的历史记录用于故障分析

### 4. 安全考虑
- 定期轮换SSH密钥
- 使用最小权限原则配置用户权限
- 监控异常连接和操作日志

## 错误处理

组件提供了完整的错误处理机制：

- **参数验证错误**: 输入参数不合法时返回明确的错误信息
- **网络连接错误**: SSH连接失败时提供详细的故障原因
- **存储错误**: ETCD操作失败时进行适当的重试和降级
- **权限错误**: 密钥认证失败时返回安全的错误信息

## 扩展性

### 存储扩展
实现 `Storage` 接口可以支持其他存储后端：
- MySQL/PostgreSQL
- MongoDB
- Redis
- 文件系统

### 认证扩展
通过 `CredentialProvider` 接口可以集成其他认证服务：
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Kubernetes Secrets

### 监控扩展
可以集成各种监控系统：
- Prometheus
- Grafana
- DataDog
- 自定义监控系统 