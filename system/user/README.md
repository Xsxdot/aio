# User 组件

用户组件提供管理员账号管理和客户端 API 凭证管理功能。

## 功能概述

### 1. 管理员管理
- 管理员账号的创建、查询、更新、删除
- 密码管理（重置密码、修改密码）
- 状态管理（启用/禁用）
- 管理员登录验证

### 2. 客户端凭证管理
- 客户端 key/secret 的生成和管理
- 客户端状态管理（启用/禁用）
- 客户端过期时间管理
- IP 白名单配置
- Secret 重新生成（rotate）

### 3. 客户端认证服务（gRPC）
- 基于 key/secret 的客户端认证，返回 JWT token
- Token 续期功能（在未过期时延长有效期）

## 目录结构

```
system/user/
├── internal/                    # 内部实现（不对外暴露）
│   ├── model/                   # 领域模型
│   │   ├── admin.go             # 管理员模型
│   │   ├── client_credential.go # 客户端凭证模型
│   │   └── dto/                 # 内部 DTO
│   ├── dao/                     # 数据访问层
│   │   ├── admin_dao.go
│   │   └── client_credential_dao.go
│   ├── service/                 # 业务逻辑层
│   │   ├── admin_service.go
│   │   ├── client_credential_service.go
│   │   └── jwt_service.go       # JWT 服务
│   └── app/                     # 应用组合层
│       └── app.go
├── api/                         # 对外 API（供其他组件调用）
│   ├── dto/                     # 对外 DTO
│   │   ├── admin_dto.go
│   │   └── client_dto.go
│   ├── client/                  # 对外客户端
│   │   └── user_client.go
│   └── proto/                   # gRPC proto 定义
│       ├── user.proto
│       ├── user.pb.go
│       └── user_grpc.pb.go
├── external/                    # 对外协议适配层
│   ├── http/                    # HTTP 控制器
│   │   ├── admin_controller.go
│   │   └── client_credential_controller.go
│   └── grpc/                    # gRPC 服务实现
│       └── client_auth_service.go
├── module.go                    # 模块门面
├── migrate.go                   # 数据库迁移
├── router.go                    # 路由注册
└── README.md                    # 本文档
```

## 使用方式

### 1. 集成到应用

在 `app/app.go` 中添加用户模块：

```go
import "xiaozhizhang/system/user"

type App struct {
    // ... 其他模块
    UserModule *user.Module
}

func NewApp() *App {
    return &App{
        // ... 其他模块初始化
        UserModule: user.NewModule(),
    }
}
```

### 2. 注册数据库迁移

在 `pkg/db/migrate.go` 中添加：

```go
import "xiaozhizhang/system/user"

func AutoMigrate() {
    // ... 其他组件迁移
    user.AutoMigrate(base.DB, base.Logger)
}
```

### 3. 注册 HTTP 路由

在 `router/router.go` 中添加：

```go
import "xiaozhizhang/system/user"

func Register(app *app.App, f *fiber.App) {
    // ... 其他路由注册
    user.RegisterRoutes(app.UserModule, api, admin)
}
```

### 4. 注册 gRPC 服务

在 gRPC 服务器启动代码中：

```go
// 注册客户端认证服务
app.UserModule.GRPCService.RegisterService(grpcServer)
```

## HTTP API 接口

### 管理员管理

- `POST /admin/admins` - 创建管理员
- `GET /admin/admins` - 查询管理员列表
- `GET /admin/admins/:id` - 查询管理员详情
- `PUT /admin/admins/:id/password` - 重置管理员密码
- `PUT /admin/admins/:id/status` - 更新管理员状态
- `DELETE /admin/admins/:id` - 删除管理员
- `POST /admin/login` - 管理员登录

### 客户端凭证管理

- `POST /admin/client-credentials` - 创建客户端凭证
- `GET /admin/client-credentials` - 查询客户端列表
- `GET /admin/client-credentials/:id` - 查询客户端详情
- `PUT /admin/client-credentials/:id` - 更新客户端信息
- `PUT /admin/client-credentials/:id/status` - 更新客户端状态
- `POST /admin/client-credentials/:id/rotate-secret` - 重新生成 secret
- `DELETE /admin/client-credentials/:id` - 删除客户端

## gRPC API 接口

### ClientAuthService

#### AuthenticateClient
客户端认证接口，返回 JWT token。

**请求**：
```protobuf
message AuthenticateClientRequest {
  string client_key = 1;
  string client_secret = 2;
}
```

**响应**：
```protobuf
message AuthenticateClientResponse {
  string access_token = 1;  // JWT token
  int64 expires_at = 2;     // 过期时间戳（秒）
  string token_type = 3;    // "Bearer"
}
```

#### RenewToken
续期 token 接口（在未过期时延长有效期）。

**请求**：
- Token 通过 gRPC metadata 的 `authorization` 字段传递
- 格式：`Bearer <token>`

**响应**：
```protobuf
message RenewTokenResponse {
  string access_token = 1;  // 新的 JWT token
  int64 expires_at = 2;     // 新的过期时间戳（秒）
  string token_type = 3;    // "Bearer"
}
```

## 对外 Client 使用示例

其他组件可以通过 `UserClient` 调用用户能力：

```go
// 验证管理员登录
admin, err := app.UserModule.Client.ValidateAdminLogin(ctx, "admin", "password")

// 验证客户端凭证
client, err := app.UserModule.Client.ValidateClientCredential(ctx, clientKey, clientSecret)

// 查询管理员
admin, err := app.UserModule.Client.GetAdminByID(ctx, adminID)
```

## 数据库表

### user_admin（管理员表）
- `id` - 主键
- `account` - 管理员账号（唯一）
- `password_hash` - 密码散列
- `status` - 状态（1=启用，0=禁用）
- `remark` - 备注
- `created_at` - 创建时间
- `updated_at` - 更新时间
- `deleted_at` - 软删除时间

### user_client_credential（客户端凭证表）
- `id` - 主键
- `name` - 客户端名称
- `client_key` - 客户端 key（唯一）
- `client_secret` - 客户端 secret 散列
- `status` - 状态（1=启用，0=禁用）
- `description` - 描述
- `ip_whitelist` - IP 白名单（JSON）
- `expires_at` - 过期时间
- `created_at` - 创建时间
- `updated_at` - 更新时间
- `deleted_at` - 软删除时间

## 安全说明

1. **密码存储**：使用 bcrypt 进行密码散列，不存储明文密码
2. **Secret 存储**：客户端 secret 同样使用 bcrypt 散列存储
3. **Secret 返回**：客户端 secret 仅在创建和 rotate 时返回一次，请妥善保管
4. **JWT 签名**：使用 HS256 算法签名，secret 从配置中读取
5. **Token 续期**：只有在 token 未过期时才能续期，过期后需重新认证

## 配置说明

在 `resources/*.yaml` 中配置 JWT：

```yaml
jwt:
  secret: "your-jwt-secret"      # JWT 签名密钥
  expire-time: 24                 # token 过期时间（小时）
```

## 注意事项

1. 客户端 secret 只在创建和重新生成时返回一次，请妥善保管
2. 管理员密码重置操作需要管理员权限
3. 客户端凭证可以设置过期时间，过期后将无法认证
4. IP 白名单功能已预留，需要在中间件中实现 IP 校验逻辑
5. gRPC 服务默认使用与 HTTP 相同的 JWT secret，可根据需要分开配置



