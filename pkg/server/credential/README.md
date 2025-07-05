# 密钥管理组件

密钥管理组件提供了安全的密钥存储、管理和验证功能，支持多种类型的认证凭据管理。

## 功能特性

### 🔐 密钥类型支持
- **SSH私钥**: 支持RSA、Ed25519、ECDSA等多种SSH密钥格式
- **用户名密码**: 支持传统的用户名密码认证
- **Token**: 支持各种Token类型的认证凭据

### 🛡️ 安全特性
- **AES-256加密**: 使用AES-256-GCM算法加密存储敏感内容
- **安全传输**: 所有敏感数据传输均经过加密
- **访问控制**: 支持基于用户的密钥访问控制
- **审计日志**: 记录所有密钥操作的审计日志

### 🔍 密钥验证
- **连接测试**: 实时验证密钥的有效性
- **格式检查**: 自动验证密钥格式的正确性
- **SSH密钥分析**: 提供SSH密钥的详细信息分析

### 📊 管理功能
- **生命周期管理**: 支持密钥的创建、更新、删除
- **分类管理**: 按类型和用户分类管理密钥
- **批量操作**: 支持批量密钥操作
- **搜索过滤**: 强大的密钥搜索和过滤功能

## 架构设计

### 组件结构
```
pkg/credential/
├── types.go     # 类型定义和存储接口
├── service.go   # 服务接口和实现
├── storage.go   # ETCD存储实现
└── README.md    # 组件文档
```

### 接口设计

#### Service 接口
```go
type Service interface {
    // 密钥管理
    CreateCredential(ctx context.Context, req *CredentialCreateRequest) (*CredentialSafe, error)
    GetCredential(ctx context.Context, id string) (*CredentialSafe, error)
    UpdateCredential(ctx context.Context, id string, req *CredentialUpdateRequest) (*CredentialSafe, error)
    DeleteCredential(ctx context.Context, id string) error
    ListCredentials(ctx context.Context, req *CredentialListRequest) ([]*CredentialSafe, int, error)

    // 密钥测试
    TestCredential(ctx context.Context, id string, req *CredentialTestRequest) (*CredentialTestResult, error)

    // 获取密钥内容（供其他组件使用）
    GetCredentialContent(ctx context.Context, id string) (string, error)

    // 分析密钥信息
    AnalyzeSSHKey(ctx context.Context, id string) (*SSHKeyInfo, error)
}
```

#### Storage 接口
```go
type Storage interface {
    // 密钥管理
    CreateCredential(ctx context.Context, credential *Credential) error
    GetCredential(ctx context.Context, id string) (*Credential, error)
    GetCredentialSafe(ctx context.Context, id string) (*CredentialSafe, error)
    UpdateCredential(ctx context.Context, credential *Credential) error
    DeleteCredential(ctx context.Context, id string) error
    ListCredentials(ctx context.Context, req *CredentialListRequest) ([]*CredentialSafe, int, error)

    // 密钥查询
    GetCredentialsByType(ctx context.Context, credType CredentialType) ([]*CredentialSafe, error)
    GetCredentialsByUser(ctx context.Context, userID string) ([]*CredentialSafe, error)

    // 安全相关
    EncryptContent(content string) (string, error)
    DecryptContent(encryptedContent string) (string, error)
}
```

## 使用示例

### 创建密钥管理服务

```go
import (
    "github.com/xsxdot/aio/pkg/credential"
    "github.com/xsxdot/aio/internal/etcd"
)

// 创建ETCD客户端
etcdClient := etcd.NewEtcdClient(etcdConfig)

// 创建密钥存储
storage, err := credential.NewETCDStorage(credential.ETCDStorageConfig{
    Client:     etcdClient,
    Logger:     logger,
    EncryptKey: "your-32-character-encryption-key",
})
if err != nil {
    log.Fatal(err)
}

// 创建密钥管理服务
credentialService := credential.NewService(credential.Config{
    Storage: storage,
    Logger:  logger,
})
```

### 创建SSH密钥

```go
// SSH私钥内容
sshPrivateKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAACmFlczI1Ni1jdHIAAAAGYmNyeXB0AAAAGAAAABDQd+XnqNhWW
...
-----END OPENSSH PRIVATE KEY-----`

// 创建SSH密钥
req := &credential.CredentialCreateRequest{
    Name:        "生产环境SSH密钥",
    Description: "用于生产环境服务器访问的SSH密钥",
    Type:        credential.CredentialTypeSSHKey,
    Content:     sshPrivateKey,
}

cred, err := credentialService.CreateCredential(ctx, req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("SSH密钥创建成功: %s\n", cred.ID)
```

### 创建密码凭据

```go
// 创建密码凭据
req := &credential.CredentialCreateRequest{
    Name:        "数据库管理员密码",
    Description: "MySQL数据库管理员账户密码",
    Type:        credential.CredentialTypePassword,
    Content:     "SecurePassword123!",
}

cred, err := credentialService.CreateCredential(ctx, req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("密码凭据创建成功: %s\n", cred.ID)
```

### 测试密钥连接

```go
// 测试SSH密钥连接
testReq := &credential.CredentialTestRequest{
    Host:     "192.168.1.100",
    Port:     22,
    Username: "root",
}

result, err := credentialService.TestCredential(ctx, cred.ID, testReq)
if err != nil {
    log.Fatal(err)
}

if result.Success {
    fmt.Printf("密钥测试成功，连接延迟: %dms\n", result.Latency)
} else {
    fmt.Printf("密钥测试失败: %s\n", result.Message)
}
```

### 分析SSH密钥信息

```go
// 分析SSH密钥
keyInfo, err := credentialService.AnalyzeSSHKey(ctx, cred.ID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("SSH密钥信息:\n")
fmt.Printf("- 类型: %s\n", keyInfo.Type)
fmt.Printf("- 指纹: %s\n", keyInfo.Fingerprint)
fmt.Printf("- 密钥长度: %d位\n", keyInfo.KeySize)
```

### 查询密钥列表

```go
// 查询所有SSH密钥
listReq := &credential.CredentialListRequest{
    Type:   credential.CredentialTypeSSHKey,
    Limit:  20,
    Offset: 0,
}

credentials, total, err := credentialService.ListCredentials(ctx, listReq)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("找到 %d 个SSH密钥，总共 %d 个\n", len(credentials), total)
for _, cred := range credentials {
    fmt.Printf("- %s (%s) - 创建时间: %s\n", 
        cred.Name, cred.ID, cred.CreatedAt.Format("2006-01-02 15:04:05"))
}
```

### 更新密钥

```go
// 更新密钥描述
updateReq := &credential.CredentialUpdateRequest{
    Name:        "更新后的SSH密钥名称",
    Description: "更新后的描述信息",
    // Content 为空时不更新密钥内容
}

updatedCred, err := credentialService.UpdateCredential(ctx, cred.ID, updateReq)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("密钥更新成功: %s\n", updatedCred.Name)
```

## 安全模型

### 加密存储
- **算法**: AES-256-GCM
- **密钥管理**: 支持自定义加密密钥
- **前缀标识**: 使用 `ENC_AES:` 前缀标识加密内容
- **向后兼容**: 支持明文存储的历史数据

### 访问控制
- **用户隔离**: 支持按用户隔离密钥访问
- **最小权限**: 只返回用户有权限访问的密钥
- **安全传输**: 敏感内容不会在日志中输出

### 审计功能
- **操作日志**: 记录所有密钥操作
- **访问追踪**: 跟踪密钥的访问历史
- **异常监控**: 检测异常的密钥访问模式

## 支持的密钥类型

### SSH密钥 (CredentialTypeSSHKey)
- **RSA密钥**: 支持2048位及以上RSA密钥
- **Ed25519密钥**: 支持Ed25519椭圆曲线密钥
- **ECDSA密钥**: 支持P-256、P-384、P-521曲线
- **格式支持**: OpenSSH私钥格式

### 密码凭据 (CredentialTypePassword)
- **用户名密码**: 传统的用户名密码组合
- **数据库密码**: 数据库连接密码
- **API密码**: API访问密码

### Token凭据 (CredentialTypeToken)
- **API Token**: 各种API访问令牌
- **JWT Token**: JSON Web Token
- **OAuth Token**: OAuth访问令牌

## 配置项

### 存储配置
```go
type ETCDStorageConfig struct {
    Client     *etcd.EtcdClient  // ETCD客户端（必填）
    Logger     *zap.Logger       // 日志记录器（可选）
    EncryptKey string            // 加密密钥（建议32字符）
}
```

### 服务配置
```go
type Config struct {
    Storage Storage      // 存储实现（必填）
    Logger  *zap.Logger  // 日志记录器（可选）
}
```

## 最佳实践

### 1. 密钥命名
- 使用描述性名称，包含用途和环境信息
- 避免在名称中包含敏感信息
- 建议格式：`{环境}-{用途}-{类型}`

### 2. 加密密钥管理
- 使用强随机密钥，长度至少32字符
- 定期轮换加密密钥
- 妥善保管加密密钥，避免硬编码

### 3. 密钥轮换
- 定期更新SSH密钥和密码
- 建立密钥轮换流程和计划
- 及时删除不再使用的密钥

### 4. 访问控制
- 实施最小权限原则
- 定期审查密钥访问权限
- 监控异常的密钥访问

## 错误处理

组件提供了完善的错误处理机制：

- **格式验证错误**: 密钥格式不正确时返回详细错误信息
- **加密解密错误**: 加密或解密失败时的错误处理
- **存储错误**: ETCD操作失败时的重试和降级处理
- **网络错误**: SSH连接测试失败时的错误分类

## 监控和告警

### 关键指标
- **密钥总数**: 系统中管理的密钥总数
- **创建频率**: 密钥创建的频率统计
- **测试成功率**: 密钥连接测试的成功率
- **访问频率**: 密钥被访问的频率

### 告警规则
- **密钥测试失败**: 连续多次测试失败时告警
- **异常访问**: 异常频繁的密钥访问时告警
- **存储故障**: ETCD存储操作失败时告警

## 扩展性

### 存储扩展
实现 `Storage` 接口可以支持其他存储后端：
- **数据库**: MySQL、PostgreSQL等关系型数据库
- **NoSQL**: MongoDB、Redis等NoSQL数据库
- **云存储**: AWS Secrets Manager、Azure Key Vault等
- **专业工具**: HashiCorp Vault、CyberArk等

### 加密扩展
可以扩展支持其他加密算法：
- **对称加密**: AES-128、AES-256、ChaCha20等
- **非对称加密**: RSA、ECC等
- **密钥派生**: PBKDF2、Argon2等

### 认证扩展
可以集成其他认证方式：
- **多因素认证**: TOTP、HOTP等
- **生物识别**: 指纹、面部识别等
- **硬件令牌**: YubiKey、智能卡等

## 性能优化

### 缓存策略
- **内存缓存**: 缓存频繁访问的密钥信息
- **连接池**: 复用ETCD连接，减少连接开销
- **批量操作**: 支持批量密钥操作，提高效率

### 并发控制
- **读写分离**: 分离读写操作，提高并发性能
- **锁机制**: 合理使用锁，避免死锁
- **异步处理**: 异步处理非关键操作

## 故障恢复

### 数据备份
- **定期备份**: 定期备份密钥数据
- **增量备份**: 支持增量备份，减少备份时间
- **异地备份**: 将备份存储在不同地理位置

### 灾难恢复
- **快速恢复**: 支持快速恢复密钥服务
- **数据一致性**: 保证恢复后的数据一致性
- **服务降级**: 在故障时提供基本的密钥服务 