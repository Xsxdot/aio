# gRPC 服务测试指南

## 概述

本文档介绍如何测试已实现的 config 组件 gRPC 服务。

## 前提条件

1. 安装 grpcurl 工具（用于测试 gRPC 接口）

```bash
# macOS
brew install grpcurl

# Linux
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

## 启动服务

1. 启动应用：

```bash
cd /Users/xushixin/workspace/go/xiaozhizhang
go run main.go -env dev
```

2. 验证服务启动：

- HTTP 服务应该在 `:9000` 端口启动
- gRPC 服务应该在 `:50051` 端口启动

在日志中查找类似信息：
```
gRPC 服务器已启动，监听地址: :50051
```

## 测试 gRPC 服务

### 1. 列出所有可用的服务

```bash
grpcurl -plaintext localhost:50051 list
```

预期输出：
```
config.v1.ConfigService
grpc.health.v1.Health
grpc.reflection.v1alpha.ServerReflection
```

### 2. 列出 ConfigService 的所有方法

```bash
grpcurl -plaintext localhost:50051 list config.v1.ConfigService
```

预期输出：
```
config.v1.ConfigService.BatchGetConfigs
config.v1.ConfigService.CreateConfig
config.v1.ConfigService.DeleteConfig
config.v1.ConfigService.GetConfig
config.v1.ConfigService.GetConfigForAdmin
config.v1.ConfigService.ListConfigsForAdmin
config.v1.ConfigService.UpdateConfig
config.v1.ConfigService.UpdateConfigStatus
```

### 3. 查看服务描述

```bash
grpcurl -plaintext localhost:50051 describe config.v1.ConfigService
```

### 4. 获取管理员 Token

首先需要通过 HTTP 接口登录获取管理员 token。假设你已经有了 token，后续测试会用到。

### 5. 测试查询接口（不需要鉴权或需要用户权限）

#### GetConfig - 获取单个配置

```bash
grpcurl -plaintext \
  -d '{
    "key": "test.config",
    "env": "dev"
  }' \
  localhost:50051 config.v1.ConfigService/GetConfig
```

#### BatchGetConfigs - 批量获取配置

```bash
grpcurl -plaintext \
  -d '{
    "keys": ["test.config1", "test.config2"],
    "env": "dev"
  }' \
  localhost:50051 config.v1.ConfigService/BatchGetConfigs
```

### 6. 测试管理接口（需要管理员权限）

**注意**：以下请求需要在 header 中携带管理员 token。

#### CreateConfig - 创建配置

```bash
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "key": "grpc.test.config",
    "value": {
      "dev": {
        "value": "{\"host\":\"localhost\",\"port\":8080}",
        "type": "VALUE_TYPE_OBJECT"
      },
      "prod": {
        "value": "{\"host\":\"production.com\",\"port\":80}",
        "type": "VALUE_TYPE_OBJECT"
      }
    },
    "metadata": {
      "owner": "test-team",
      "env": "all"
    },
    "description": "gRPC测试配置",
    "change_note": "通过gRPC创建"
  }' \
  localhost:50051 config.v1.ConfigService/CreateConfig
```

#### GetConfigForAdmin - 获取配置详情（管理端）

```bash
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "key": "grpc.test.config"
  }' \
  localhost:50051 config.v1.ConfigService/GetConfigForAdmin
```

#### ListConfigsForAdmin - 列表查询（管理端）

```bash
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "key": "grpc",
    "page_num": 1,
    "size": 10
  }' \
  localhost:50051 config.v1.ConfigService/ListConfigsForAdmin
```

#### UpdateConfig - 更新配置

```bash
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "key": "grpc.test.config",
    "value": {
      "dev": {
        "value": "{\"host\":\"localhost\",\"port\":9090}",
        "type": "VALUE_TYPE_OBJECT"
      }
    },
    "description": "更新后的描述",
    "change_note": "通过gRPC更新端口"
  }' \
  localhost:50051 config.v1.ConfigService/UpdateConfig
```

#### DeleteConfig - 删除配置

```bash
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{
    "key": "grpc.test.config"
  }' \
  localhost:50051 config.v1.ConfigService/DeleteConfig
```

## 错误处理测试

### 1. 测试鉴权失败

不带 token 调用管理接口：

```bash
grpcurl -plaintext \
  -d '{
    "key": "test.config"
  }' \
  localhost:50051 config.v1.ConfigService/GetConfigForAdmin
```

预期返回：
```
ERROR:
  Code: Unauthenticated
  Message: 未授权的请求
```

### 2. 测试无效的 token

使用无效的 token：

```bash
grpcurl -plaintext \
  -H "authorization: Bearer INVALID_TOKEN" \
  -d '{
    "key": "test.config"
  }' \
  localhost:50051 config.v1.ConfigService/GetConfigForAdmin
```

预期返回：
```
ERROR:
  Code: Unauthenticated
  Message: 无效的认证令牌
```

### 3. 测试配置不存在

查询不存在的配置：

```bash
grpcurl -plaintext \
  -d '{
    "key": "non.existent.config",
    "env": "dev"
  }' \
  localhost:50051 config.v1.ConfigService/GetConfig
```

预期返回：
```
ERROR:
  Code: NotFound
  Message: 配置不存在
```

## 性能测试

可以使用 ghz 工具进行压力测试：

```bash
# 安装 ghz
go install github.com/bojand/ghz/cmd/ghz@latest

# 运行压力测试
ghz --insecure \
  --proto system/config/api/proto/config.proto \
  --call config.v1.ConfigService/GetConfig \
  -d '{
    "key": "test.config",
    "env": "dev"
  }' \
  -c 50 \
  -n 10000 \
  localhost:50051
```

## 监控和调试

### 查看服务健康状态

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

### 启用详细日志

在开发环境中，gRPC 服务器会记录详细的请求日志，包括：
- 请求方法
- 请求耗时
- 错误信息
- 鉴权信息

查看日志以了解请求处理的详细信息。

## 注意事项

1. **开发环境 vs 生产环境**
   - 开发环境（dev/test）启用了反射服务，可以使用 grpcurl
   - 生产环境（prod）默认关闭反射服务，需要使用 proto 文件调用

2. **Token 格式**
   - 支持 `Bearer TOKEN` 格式
   - 也支持直接传递 token（会自动识别）

3. **权限控制**
   - 管理接口需要管理员权限
   - 查询接口可以使用用户权限或无需鉴权（根据配置）
   - SuperAdmin 拥有所有权限

4. **配置缓存**
   - GetConfig 接口使用了缓存（5分钟TTL）
   - 更新/删除配置后会自动清除相关缓存

## 故障排查

### gRPC 服务无法启动

1. 检查端口是否被占用：
   ```bash
   lsof -i :50051
   ```

2. 检查配置文件中的 gRPC 配置是否正确

3. 查看启动日志中的错误信息

### 无法连接到 gRPC 服务

1. 确认服务已启动
2. 确认端口配置正确
3. 检查防火墙设置

### 鉴权失败

1. 确认 token 格式正确
2. 确认 token 未过期
3. 确认使用了正确的密钥（admin-secret）

## 相关文档

- [gRPC 官方文档](https://grpc.io/docs/)
- [grpcurl 文档](https://github.com/fullstorydev/grpcurl)
- [Protocol Buffers 文档](https://protobuf.dev/)



