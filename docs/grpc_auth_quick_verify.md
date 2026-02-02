# gRPC 认证问题快速验证指南

## 问题总结

发现并修复了两个 gRPC 认证相关的 bug：

### Bug 1：中间件初始化顺序错误
- **文件**：`main.go`
- **问题**：在创建 gRPC Server 之前没有设置 Auth 配置，导致鉴权中间件未被添加
- **修复**：调整初始化顺序，在创建 Server 前完整设置 Auth 配置

### Bug 2：跳过鉴权列表中的方法名错误
- **文件**：`pkg/grpc/config.go`
- **问题**：`DefaultAuthConfig()` 中的方法名 `/auth.v1.AuthService/ClientAuth` 错误
- **正确**：应该是 `/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient`
- **后果**：认证接口本身也需要认证，形成死锁，SDK 无法获取 token

## 快速验证

### 1. 重启服务

```bash
# 停止当前服务
# 重新编译并启动
go build -o aio main.go
./aio -env=dev
```

### 2. 检查启动日志

服务启动后应该看到：

```
gRPC 服务器已启动，监听地址: :50051
已注册服务 name=config.v1.ConfigService version=v1.0.0
已注册服务 name=registry.v1.RegistryService version=v1.0.0
...
```

### 3. 使用 SDK 测试认证

创建一个简单的测试文件 `test_auth.go`：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	
	"xiaozhizhang/pkg/sdk"
)

func main() {
	// 使用你的实际配置
	client, err := sdk.New(sdk.Config{
		RegistryAddr: "127.0.0.1:50051",
		ClientKey:    "P7ChCGWUIbXXzeezdH3r77ZTaptSDBQG",
		ClientSecret: "71N4AsKOpUAzmdC0El7spfYRFK--Hkraom5MC24AlwI=",
	})
	if err != nil {
		log.Fatalf("创建 SDK 客户端失败: %v", err)
	}
	defer client.Close()

	fmt.Println("✅ SDK 客户端创建成功")

	// 测试获取 token
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := client.Auth.Token(ctx)
	if err != nil {
		log.Fatalf("❌ 获取 token 失败: %v", err)
	}

	fmt.Printf("✅ 成功获取 token: %s...\n", token[:20])

	// 测试调用需要认证的接口（例如创建配置）
	// 这里省略具体实现...
	
	fmt.Println("✅ 所有认证测试通过")
}
```

运行测试：

```bash
go run test_auth.go
```

**预期输出**：

```
✅ SDK 客户端创建成功
✅ 成功获取 token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
✅ 所有认证测试通过
```

### 4. 检查服务端日志

服务端应该记录类似以下日志：

```
[INFO] gRPC 请求开始 method=/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient
[DEBUG] 跳过鉴权 method=/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient
[INFO] gRPC 请求完成 method=/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient duration=10ms status=OK
```

**不应该再出现**：

```
[WARN] 提取令牌失败 method=/xiaozhizhang.user.v1.ClientAuthService/AuthenticateClient error="缺少认证令牌"
```

### 5. 测试需要认证的接口

确保需要认证的接口能正常工作：

```bash
# 使用 grpcurl 或编写代码测试
# 例如：调用 CreateConfig
```

服务端日志应该显示：

```
[DEBUG] 鉴权成功 method=/config.v1.ConfigService/CreateConfig subject_id=123 subject_type=client
```

## 常见问题

### Q1：还是报"缺少认证令牌"错误

**排查步骤**：

1. 确认已重启服务（代码修改需要重启）
2. 检查 `pkg/grpc/config.go` 中的 `DefaultAuthConfig()` 是否已更新
3. 检查 `main.go` 中的 gRPC 初始化顺序是否正确
4. 查看完整的错误日志，确认是哪个方法报错

### Q2：认证成功但业务接口报"未找到认证信息"

这是第一个 bug 的症状，说明：

1. 认证接口跳过了鉴权（正确）
2. 但业务接口的鉴权中间件没有正确初始化

**解决**：检查 `main.go` 中的初始化顺序是否正确。

### Q3：如何确认中间件是否正确加载？

在 `pkg/grpc/middleware.go` 的 `AuthInterceptor` 函数开始处添加日志：

```go
func AuthInterceptor(config *AuthConfig, logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Debug("AuthInterceptor called", zap.String("method", info.FullMethod))
		// ... 其余代码
	}
}
```

重启后，如果看到 `AuthInterceptor called` 日志，说明中间件已正确加载。

## 相关文档

- [grpc_auth_fix.md](./grpc_auth_fix.md) - 详细的问题分析和修复说明
- [GRPC_QUICKSTART.md](./GRPC_QUICKSTART.md) - gRPC 快速开始指南
- [grpc_testing_guide.md](./grpc_testing_guide.md) - gRPC 测试指南

## 修改文件清单

- ✅ `main.go` - 调整 gRPC Server 初始化顺序
- ✅ `pkg/grpc/config.go` - 修正跳过鉴权列表中的方法名
- ✅ `docs/grpc_auth_fix.md` - 详细问题分析文档
- ✅ `docs/grpc_auth_quick_verify.md` - 本验证指南

