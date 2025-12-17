# 配置文件说明

## 配置文件结构

本项目使用 YAML 格式的配置文件，根据不同环境使用不同的配置文件：

- `dev.yaml` - 开发环境配置（本地文件，不提交到 Git）
- `test.yaml` - 测试环境配置（本地文件，不提交到 Git）
- `prod.yaml` - 生产环境配置（本地文件，不提交到 Git）
- `agent.yaml` - Agent 配置（本地文件，不提交到 Git）

## 快速开始

### 1. 创建配置文件

复制示例配置文件并根据你的环境修改：

```bash
# 开发环境
cp config.yaml.example dev.yaml

# 测试环境
cp config.yaml.example test.yaml

# 生产环境
cp config.yaml.example prod.yaml

# Agent 配置
cp agent.yaml.example agent.yaml
```

### 2. 修改配置

编辑相应的配置文件，填入真实的配置信息：

- 数据库连接信息（`db` 部分）
- Redis 连接信息（`redis` 部分）
- JWT 密钥（`jwt` 部分）
- OSS 凭证（`oss` 部分）
- 服务器信息（`server` 部分）

### 3. 安全注意事项

⚠️ **重要**：以下信息属于敏感信息，请勿提交到 Git 仓库：

- 数据库密码
- Redis 密码
- JWT 密钥
- OSS AccessKey 和 AccessSecret
- SSH 密钥和密码
- 任何生产环境的凭证

这些文件已经在 `.gitignore` 中配置忽略，但请确保不要使用 `git add -f` 强制添加。

## 配置项说明

### 数据库配置 (db)

```yaml
db:
  host: localhost      # 数据库地址
  port: 3306          # 数据库端口
  user: username      # 数据库用户名
  password: password  # 数据库密码
  db-name: database   # 数据库名称
```

### Redis 配置 (redis)

```yaml
redis:
  mode: single              # 模式：single/cluster/sentinel
  host: 'localhost:6379'    # Redis 地址
  password: your_password   # Redis 密码
  db: 0                     # 数据库编号
```

### JWT 配置 (jwt)

```yaml
jwt:
  secret: 'base64_encoded_secret'         # 用户 JWT 密钥
  admin-secret: 'base64_encoded_secret'   # 管理员 JWT 密钥
  expire-time: 24                         # Token 过期时间（小时）
```

### OSS 配置 (oss)

```yaml
oss:
  access-key: 'your_access_key'          # 阿里云 AccessKey ID
  access-secret: 'your_access_secret'    # 阿里云 AccessKey Secret
  bucket-name: your-bucket               # OSS Bucket 名称
  region: cn-hangzhou                    # OSS 区域
```

### gRPC 配置 (grpc)

```yaml
grpc:
  address: ':50051'                # 监听地址
  enable_reflection: true          # 启用反射（开发环境推荐）
  enable_recovery: true            # 启用恢复中间件
  enable_validation: true          # 启用参数验证
  enable_auth: true                # 启用鉴权
  enable_permission: true          # 启用权限验证
  log_level: 'info'                # 日志级别
  max_recv_msg_size: 4194304       # 最大接收消息大小（字节）
  max_send_msg_size: 4194304       # 最大发送消息大小（字节）
  connection_timeout: 30s          # 连接超时
```

### 服务器配置 (server)

```yaml
server:
  bootstrap:
    - name: 'server-1'                    # 服务器名称
      host: '192.168.1.100'               # 服务器地址
      agent_grpc_address: '192.168.1.100:50052'  # Agent gRPC 地址
      enabled: true                       # 是否启用
      tags:                               # 标签
        env: 'dev'
        region: 'cn-north'
      ssh:                                # SSH 配置
        port: 22
        username: 'root'
        auth_method: 'privatekey'         # 认证方式：password/privatekey
        private_key_file: '/path/to/key'  # 私钥文件路径（推荐）
```

## 环境变量

配置文件可以通过环境变量 `ENV` 来指定：

```bash
ENV=dev ./main      # 使用 dev.yaml
ENV=test ./main     # 使用 test.yaml
ENV=prod ./main     # 使用 prod.yaml
```

## 故障排查

### 配置文件找不到

确保配置文件在 `resources/` 目录下，文件名格式为 `{env}.yaml`。

### 配置项解析失败

检查 YAML 语法是否正确，特别注意：
- 缩进使用空格，不要使用 Tab
- 字符串中有特殊字符时使用引号包裹
- 布尔值使用 `true`/`false`

### 数据库连接失败

检查：
- 数据库地址和端口是否正确
- 用户名和密码是否正确
- 数据库是否已创建
- 网络是否可达

## 更多信息

如有问题，请查看项目文档或联系开发团队。

