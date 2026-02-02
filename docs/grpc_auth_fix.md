# gRPC 认证中间件初始化顺序问题修复

## 问题描述

在 gRPC 服务中，调用需要认证的接口时，一直报错"未找到认证信息"（在 `config_service.go:323-328`）：

```go
authInfo, ok := ctx.Value("authInfo").(*pkggrpc.AuthInfo)
if !ok || authInfo == nil {
    s.log.Warn("未找到认证信息")
    return "", 0, status.Error(codes.Unauthenticated, "未授权的请求")
}
```

## 根本原因

**中间件初始化顺序错误**：

1. 在 `main.go` 原代码中（第 161-166 行），gRPC Server 的创建顺序为：
   ```go
   // 创建 gRPC Server
   grpcServer := grpc.NewServer(grpcServerConfig, zapLogger)  // ❌ 此时构建中间件链
   
   // 设置客户端凭证鉴权提供者
   tokenParser := appRoot.UserModule.NewTokenParser()
   authProvider := grpc.NewClientAuthProvider(tokenParser, zapLogger)
   grpcServer.SetAuthProvider(authProvider)  // ⚠️ 为时已晚！
   ```

2. 在 `grpc.NewServer()` 被调用时，会立即调用 `config.BuildServerOptions()` 构建中间件链

3. 在 `config.go:119-121` 中，只有当 `c.EnableAuth && c.Auth != nil` 时才会添加鉴权中间件：
   ```go
   // 鉴权中间件
   if c.EnableAuth && c.Auth != nil {  // ❌ 此时 c.Auth 为 nil
       interceptors = append(interceptors, AuthInterceptor(c.Auth, logger))
   }
   ```

4. 虽然配置中 `EnableAuth: true`，但此时 `grpcServerConfig.Auth` 为 `nil`，所以鉴权中间件**没有被添加到中间件链**

5. 后续调用 `SetAuthProvider()` 虽然设置了 `AuthProvider`，但中间件链已经构建完成，无法再修改

## 解决方案

**在创建 gRPC Server 之前就初始化好 Auth 配置**：

```go
// 创建 zap logger 用于 gRPC
zapLogger, err := createZapLogger(env)
if err != nil {
    configures.Logger.Panic(fmt.Sprintf("创建 zap logger 失败: %v", err))
}

// ✅ 先创建客户端凭证鉴权提供者
tokenParser := appRoot.UserModule.NewTokenParser()
authProvider := grpc.NewClientAuthProvider(tokenParser, zapLogger)

// ✅ 创建 gRPC Server 配置（必须在创建 Server 之前设置好 Auth）
authConfig := grpc.DefaultAuthConfig()
authConfig.AuthProvider = authProvider
// 添加需要跳过鉴权的方法（短网址的 ReportSuccess）
authConfig.SkipMethods = append(authConfig.SkipMethods, "/shorturl.v1.ShortURLService/ReportSuccess")

grpcServerConfig := &grpc.Config{
    Address:           grpcConfig.Address,
    EnableReflection:  grpcConfig.EnableReflection,
    EnableRecovery:    grpcConfig.EnableRecovery,
    EnableValidation:  grpcConfig.EnableValidation,
    EnableAuth:        grpcConfig.EnableAuth,
    EnablePermission:  grpcConfig.EnablePermission,
    LogLevel:          grpcConfig.LogLevel,
    MaxRecvMsgSize:    grpcConfig.MaxRecvMsgSize,
    MaxSendMsgSize:    grpcConfig.MaxSendMsgSize,
    ConnectionTimeout: grpcConfig.ConnectionTimeout,
    Auth:              authConfig,  // ✅ Auth 配置已完整设置
}

// ✅ 创建 gRPC Server（此时中间件链会正确包含鉴权中间件）
grpcServer := grpc.NewServer(grpcServerConfig, zapLogger)
```

## 修改清单

1. **main.go**：
   - 将 `TokenParser` 和 `AuthProvider` 的创建移到 gRPC Server 创建**之前**
   - 在创建 `grpcServerConfig` 之前就初始化好 `authConfig`
   - 将所有需要跳过鉴权的方法在创建 Server 之前就添加到 `SkipMethods` 列表
   - 移除了 `grpcServer.SetAuthProvider()` 调用（不再需要）
   - 移除了 `grpcServer.EnableAuth()` 调用（不再需要）

## 关键要点

1. **中间件链构建时机**：gRPC 的中间件链在 `grpc.NewServer()` 时就构建完成，后续无法修改
2. **配置完整性**：在创建 Server 之前，必须确保所有配置（包括 Auth）都已完整设置
3. **初始化顺序**：依赖项（如 AuthProvider）必须在被使用之前就创建好

## 测试验证

可以通过以下方式验证修复：

1. 启动服务器后，查看日志是否包含：
   ```
   gRPC 服务器已启动，监听地址: :50051
   ```

2. 使用 gRPC 客户端调用需要认证的接口（如 `CreateConfig`），应该能正常获取到认证信息

3. 检查日志中的鉴权相关信息：
   ```
   鉴权成功 method=/config.v1.ConfigService/CreateConfig subject_id=xxx subject_type=client
   ```

## 问题 2：跳过鉴权列表中的方法名错误（2026-02-01 发现）

### 问题描述

SDK 客户端在调用 `AuthenticateClient` 获取 token 时，服务端报错：
```
WARN grpc/middleware.go:117 提取令牌失败 {"method": "/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient", "error": "rpc error: code = Unauthenticated desc = 缺少认证令牌"}
```

### 根本原因

在 `pkg/grpc/config.go:89` 的 `DefaultAuthConfig()` 中，跳过鉴权的方法名写错了：

```go
"/auth.v1.AuthService/ClientAuth",  // ❌ 错误的方法名
```

实际的方法名（从 proto 生成的代码）应该是：

```go
"/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient"  // ✅ 正确的方法名
```

这导致**认证接口本身也需要鉴权**，形成死锁：
- SDK 需要先调用 `AuthenticateClient` 获取 token
- 但服务端要求这个接口也必须带 token
- 结果就是无法获取 token

### 解决方案

修正 `pkg/grpc/config.go` 中的跳过鉴权列表：

```go
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		SkipMethods: []string{
			"/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient",     // ✅ 客户端认证不需要鉴权
			"/grpc.health.v1.Health/Check",                                   // 健康检查不需要鉴权
			"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo", // 反射服务不需要鉴权
		},
		RequireAuth:  true,
		AuthProvider: nil,
	}
}
```

### SDK 的认证流程

SDK 中有两种连接：

1. **认证连接**（`auth.go:46`）：用于获取 token，**不带任何拦截器**
   ```go
   // 建立不带鉴权的连接（用于获取 token）
   conn, err := grpc.Dial(target, opts...)
   ```

2. **业务连接**（`sdk.go:107-124`）：用于其他业务调用，**带认证拦截器**
   ```go
   if !c.config.DisableAuth {
       opts = append(opts,
           grpc.WithUnaryInterceptor(c.unaryAuthInterceptor()),
           grpc.WithStreamInterceptor(c.streamAuthInterceptor()),
       )
   }
   ```

### 关键要点

- `AuthenticateClient`（初次认证）不需要 token，必须在跳过鉴权列表中
- `RenewToken`（续期）需要有效的 token，不应该跳过鉴权
- 方法名必须与 proto 生成的完整方法名一致（包括包名前缀）

---

## 相关文件

- `main.go`：gRPC 服务器初始化入口
- `pkg/grpc/server.go`：gRPC 服务器管理
- `pkg/grpc/config.go`：配置和中间件链构建 ⚠️ 两次修复
- `pkg/grpc/middleware.go`：认证中间件实现
- `pkg/grpc/client_auth_provider.go`：客户端认证提供者
- `system/user/token_parser.go`：Token 解析器适配器
- `system/config/external/grpc/config_service.go`：配置服务实现
- `pkg/sdk/auth.go`：SDK 认证客户端
- `pkg/sdk/interceptor.go`：SDK 客户端拦截器
- `pkg/sdk/sdk.go`：SDK 主入口

## 经验教训

1. **理解框架的生命周期**：需要清楚了解中间件、拦截器等在什么时机被构建和注册
2. **配置的完整性**：在构建组件之前，确保所有依赖的配置都已就绪
3. **延迟初始化的风险**：避免在组件创建后再修改其核心配置（如中间件链）
4. **日志的重要性**：通过日志可以快速定位初始化顺序问题
5. **方法名的精确性**：gRPC 方法名必须与 proto 生成的完整路径一致，包括所有命名空间
6. **避免死锁设计**：认证接口本身不应该要求认证，否则会形成死锁
7. **proto 生成代码是权威**：跳过鉴权列表中的方法名应该从 proto 生成的 `*_FullMethodName` 常量中获取

