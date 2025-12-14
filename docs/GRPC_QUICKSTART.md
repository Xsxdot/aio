# gRPC 服务快速开始

## 5 分钟快速上手

### 1. 启动服务

```bash
cd /Users/xushixin/workspace/go/xiaozhizhang
go run main.go -env dev
```

看到以下日志表示启动成功：
```
gRPC 服务器已启动，监听地址: :50051
```

### 2. 安装测试工具

```bash
# macOS
brew install grpcurl

# 或使用 go install
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

### 3. 测试连接

```bash
# 列出所有服务
grpcurl -plaintext localhost:50051 list
```

预期输出：
```
config.v1.ConfigService
grpc.health.v1.Health
```

### 4. 调用查询接口（无需认证）

```bash
# 获取配置
grpcurl -plaintext \
  -d '{"key": "test.config", "env": "dev"}' \
  localhost:50051 config.v1.ConfigService/GetConfig
```

### 5. 调用管理接口（需要认证）

#### 步骤 1：获取管理员 Token

通过 HTTP 接口登录获取 token（假设你已经有管理员账号）：

```bash
curl -X POST http://localhost:9000/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "account": "admin",
    "password": "your_password"
  }'
```

从响应中获取 `token` 字段的值。

#### 步骤 2：使用 Token 调用 gRPC 接口

```bash
# 创建配置
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{
    "key": "demo.config",
    "value": {
      "dev": {
        "value": "test_value",
        "type": "VALUE_TYPE_STRING"
      }
    },
    "description": "演示配置",
    "change_note": "初始创建"
  }' \
  localhost:50051 config.v1.ConfigService/CreateConfig
```

```bash
# 查询配置列表
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{
    "page_num": 1,
    "size": 10
  }' \
  localhost:50051 config.v1.ConfigService/ListConfigsForAdmin
```

## 常见问题

### Q: 如何查看所有可用的方法？

```bash
grpcurl -plaintext localhost:50051 list config.v1.ConfigService
```

### Q: 如何查看方法的参数定义？

```bash
grpcurl -plaintext localhost:50051 describe config.v1.ConfigService.GetConfig
```

### Q: 鉴权失败怎么办？

1. 确认 token 是否正确
2. 确认 token 是否过期
3. 确认使用了正确的 header 格式：`authorization: Bearer TOKEN`

### Q: 生产环境如何测试？

生产环境关闭了反射服务，需要使用 proto 文件：

```bash
grpcurl -proto system/config/api/proto/config.proto \
  -d '{"key": "test.config", "env": "prod"}' \
  production-server:50051 config.v1.ConfigService/GetConfig
```

## 下一步

- 📖 查看完整的 [测试指南](grpc_testing_guide.md)
- 📋 查看 [实施总结](grpc_implementation_summary.md)
- 🔧 为其他组件添加 gRPC 服务（参考实施总结中的扩展指南）

## 配置调整

如需修改 gRPC 配置，编辑对应环境的配置文件：

```yaml
# resources/dev.yaml
grpc:
  address: ':50051'              # 修改监听端口
  enable_reflection: true        # 开发环境建议 true
  enable_auth: true              # 是否启用鉴权
  max_recv_msg_size: 4194304     # 最大消息大小（字节）
```

修改后重启服务即可生效。



