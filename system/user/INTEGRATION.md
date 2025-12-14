# User 组件集成指南

本文档说明如何将 user 组件集成到主应用中。

## 前置要求

1. Go 1.21+
2. 已安装 protoc 和相关插件
3. 项目依赖已安装（`go mod tidy`）

## 集成步骤

### 1. 更新数据库迁移

在 `pkg/db/migrate.go` 中添加用户组件的迁移：

```go
package db

import (
	"xiaozhizhang/base"
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/user"  // 新增导入
)

func AutoMigrate() {
	log := base.Logger
	log.Info("开始执行数据库迁移...")

	// 配置中心迁移
	if err := config.AutoMigrate(base.DB, log); err != nil {
		log.WithErr(err).Error("配置中心迁移失败")
	}

	// 用户组件迁移（新增）
	if err := user.AutoMigrate(base.DB, log); err != nil {
		log.WithErr(err).Error("用户组件迁移失败")
	}

	log.Info("数据库迁移完成")
}
```

### 2. 更新应用 App

在 `app/app.go` 中添加用户模块：

```go
package app

import (
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/user"  // 新增导入
)

type App struct {
	ConfigModule *config.Module
	UserModule   *user.Module    // 新增字段
}

func NewApp() *App {
	return &App{
		ConfigModule: config.NewModule(),
		UserModule:   user.NewModule(),  // 新增初始化
	}
}
```

### 3. 更新 HTTP 路由

在 `router/router.go` 中添加用户路由：

```go
package router

import (
	"xiaozhizhang/app"
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/user"  // 新增导入

	"github.com/gofiber/fiber/v2"
)

func Register(a *app.App, f *fiber.App) {
	// API 路由组
	api := f.Group("/api")
	
	// 管理后台路由组
	admin := f.Group("/admin")

	// 配置中心路由
	config.RegisterRoutes(a.ConfigModule, api, admin)

	// 用户组件路由（新增）
	user.RegisterRoutes(a.UserModule, api, admin)
}
```

### 4. 注册 gRPC 服务（如果使用 gRPC）

在你的 gRPC 服务器启动代码中添加：

```go
package main

import (
	"xiaozhizhang/app"
	"xiaozhizhang/base"
	"xiaozhizhang/pkg/grpc"
)

func startGrpcServer(a *app.App) {
	// 创建 gRPC 服务器配置
	grpcConfig := grpc.DefaultConfig()
	grpcConfig.Address = base.Configures.Config.Grpc.Address

	// 创建 gRPC 服务器
	grpcServer := grpc.NewServer(grpcConfig, base.Logger)

	// 注册配置中心服务
	if err := a.ConfigModule.GRPCService.RegisterService(grpcServer.Server); err != nil {
		base.Logger.WithErr(err).Fatal("注册配置中心 gRPC 服务失败")
	}

	// 注册客户端认证服务（新增）
	if err := a.UserModule.GRPCService.RegisterService(grpcServer.Server); err != nil {
		base.Logger.WithErr(err).Fatal("注册客户端认证 gRPC 服务失败")
	}

	// 启动服务器
	if err := grpcServer.Start(); err != nil {
		base.Logger.WithErr(err).Fatal("gRPC 服务器启动失败")
	}
}
```

### 5. 更新配置文件（如需要）

在 `resources/dev.yaml`、`resources/test.yaml`、`resources/prod.yaml` 中确保 JWT 配置存在：

```yaml
jwt:
  secret: "your-jwt-secret-key-here"  # 生产环境使用强密钥
  admin-secret: "your-admin-jwt-secret"
  expire-time: 24  # token 过期时间（小时）
```

### 6. 运行迁移

在启动应用前，确保运行了数据库迁移：

```bash
# 如果你的 main.go 中有迁移逻辑
go run main.go

# 或者单独运行迁移
go run pkg/db/migrate.go
```

这将创建以下数据库表：
- `user_admin` - 管理员表
- `user_client_credential` - 客户端凭证表

### 7. 验证集成

#### 7.1 检查 HTTP 路由

启动应用后，以下路由应该可用：

**管理员接口**：
- `POST /admin/admins` - 创建管理员
- `GET /admin/admins` - 查询管理员列表
- `POST /admin/login` - 管理员登录

**客户端凭证接口**：
- `POST /admin/client-credentials` - 创建客户端凭证
- `GET /admin/client-credentials` - 查询客户端列表

#### 7.2 检查 gRPC 服务（如已启用）

使用 grpcurl 测试：

```bash
# 列出服务
grpcurl -plaintext localhost:9090 list

# 应该看到：
# user.v1.ClientAuthService

# 测试客户端认证
grpcurl -plaintext -d '{"client_key":"your-key","client_secret":"your-secret"}' \
  localhost:9090 user.v1.ClientAuthService/AuthenticateClient
```

## 使用示例

### 创建第一个管理员

```bash
curl -X POST http://localhost:8080/admin/admins \
  -H "Content-Type: application/json" \
  -d '{
    "account": "admin",
    "password": "admin123",
    "remark": "系统管理员"
  }'
```

### 创建第一个客户端凭证

```bash
curl -X POST http://localhost:8080/admin/client-credentials \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "name": "测试客户端",
    "description": "用于测试的客户端",
    "ipWhitelist": ["127.0.0.1", "192.168.1.0/24"]
  }'
```

响应会包含 `clientKey` 和 `clientSecret`，**请妥善保管 secret，它只会返回这一次**。

### 使用客户端凭证获取 Token（gRPC）

```bash
grpcurl -plaintext -d '{
  "client_key":"<your-client-key>",
  "client_secret":"<your-client-secret>"
}' localhost:9090 user.v1.ClientAuthService/AuthenticateClient
```

响应示例：

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresAt": 1703088000,
  "tokenType": "Bearer"
}
```

### 续期 Token（gRPC）

```bash
grpcurl -plaintext \
  -H "authorization: Bearer <your-access-token>" \
  localhost:9090 user.v1.ClientAuthService/RenewToken
```

## 其他组件调用示例

如果其他组件需要使用用户能力，可以通过 `UserClient` 调用：

```go
package mycomponent

import (
	"context"
	"xiaozhizhang/app"
)

func MyBusinessLogic(app *app.App, ctx context.Context) error {
	// 验证管理员登录
	admin, err := app.UserModule.Client.ValidateAdminLogin(ctx, "admin", "password")
	if err != nil {
		return err
	}
	
	// 验证客户端凭证
	client, err := app.UserModule.Client.ValidateClientCredential(ctx, clientKey, clientSecret)
	if err != nil {
		return err
	}
	
	// 查询管理员信息
	admin, err = app.UserModule.Client.GetAdminByID(ctx, adminID)
	if err != nil {
		return err
	}
	
	return nil
}
```

## 故障排查

### 问题 1：编译错误 "cannot find package"

**解决方案**：
```bash
go mod tidy
go clean -cache
go build ./...
```

### 问题 2：数据库表未创建

**解决方案**：
- 确保在 `pkg/db/migrate.go` 中调用了 `user.AutoMigrate`
- 检查数据库连接配置
- 查看日志中是否有迁移错误

### 问题 3：gRPC 服务无法启动

**解决方案**：
- 确保 proto 文件已生成 Go 代码
- 检查 gRPC 端口是否被占用
- 查看日志中的详细错误信息

### 问题 4：JWT token 无效

**解决方案**：
- 检查 `resources/*.yaml` 中的 JWT secret 配置
- 确保所有服务使用相同的 JWT secret
- 检查 token 是否已过期

## 注意事项

1. **生产环境**务必使用强 JWT secret，不要使用默认值
2. 客户端 secret 只在创建和 rotate 时返回一次，请妥善保管
3. 建议为不同环境（dev/test/prod）使用不同的 JWT secret
4. IP 白名单功能已预留，需要在中间件中实现实际的 IP 校验逻辑
5. gRPC 服务和 HTTP 服务可以独立部署，也可以在同一进程中运行

## 下一步

- 集成到权限系统（如 RBAC）
- 实现 IP 白名单中间件
- 添加审计日志
- 实现 Token 黑名单（用于主动撤销）
- 添加 Refresh Token 机制（如需要）



