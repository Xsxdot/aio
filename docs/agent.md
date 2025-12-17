# Agent 模式文档

## 概述

Agent 是一个独立的守护进程，运行在每台被管理的服务器上，负责执行 nginx/systemd/ssl 的本机操作（文件落盘、命令执行、状态读取等）。

主服务通过 gRPC 调用 agent，实现对远程服务器的管理。

## 架构

```
┌─────────────────┐         gRPC (JWT Auth)        ┌──────────────────┐
│  Main Server    │ ─────────────────────────────> │  Agent Daemon    │
│  (本项目)        │  <───────────────────────────  │  (目标服务器)     │
└─────────────────┘                                 └──────────────────┘
       │                                                     │
       │                                                     │
       ├─ nginx 组件                                        ├─ nginx 文件操作
       ├─ systemd 组件                                     ├─ systemd 操作
       └─ ssl 组件                                         └─ ssl 证书部署
```

## 部署步骤

### 1. 编译 Agent

```bash
cd /path/to/xiaozhizhang
go build -o agent ./cmd/agent
```

### 2. 配置 Agent

复制配置示例并修改：

```bash
sudo mkdir -p /etc/xiaozhizhang
sudo cp resources/agent.yaml.example /etc/xiaozhizhang/agent.yaml
sudo vi /etc/xiaozhizhang/agent.yaml
```

关键配置项：

```yaml
# JWT 配置（必须与主服务器一致）
jwt:
  secret: "your-jwt-secret-must-match-main-server"
  expires: 86400

# Agent 配置
agent:
  address: ":50052"  # gRPC 监听地址
  timeout: 30s
  
  nginx:
    root_dir: "/etc/nginx/conf.d"
    file_mode: "0644"
    validate_command: "nginx -t"
    reload_command: "nginx -s reload"
  
  systemd:
    unit_dir: "/etc/systemd/system"
```

### 3. 启动 Agent

作为 systemd 服务运行：

```bash
sudo ./agent -config /etc/xiaozhizhang/agent.yaml
```

或创建 systemd service：

```ini
[Unit]
Description=Xiaozhizhang Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/agent -config /etc/xiaozhizhang/agent.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo cp agent /usr/local/bin/
sudo systemctl daemon-reload
sudo systemctl enable agent
sudo systemctl start agent
sudo systemctl status agent
```

### 4. 在主服务器中注册服务器

在主服务器的管理界面中：

1. 添加服务器记录
2. 填写 `AgentGrpcAddress`，例如：`192.168.1.100:50052`
3. 测试连接（可通过健康检查 API）

## API 变更

### Nginx 管理 API

**旧路由** (不再使用):
```
POST   /admin/nginx/configs
GET    /admin/nginx/configs
...
```

**新路由** (按 serverId 路由):
```
POST   /admin/servers/:serverId/nginx/configs
GET    /admin/servers/:serverId/nginx/configs
GET    /admin/servers/:serverId/nginx/configs/:name
PUT    /admin/servers/:serverId/nginx/configs/:name
DELETE /admin/servers/:serverId/nginx/configs/:name
POST   /admin/servers/:serverId/nginx/configs/generate
PUT    /admin/servers/:serverId/nginx/configs/:name/generate
```

### Systemd 管理 API

**新路由** (按 serverId 路由):
```
POST   /admin/servers/:serverId/systemd/services
GET    /admin/servers/:serverId/systemd/services
GET    /admin/servers/:serverId/systemd/services/:name
PUT    /admin/servers/:serverId/systemd/services/:name
DELETE /admin/servers/:serverId/systemd/services/:name
POST   /admin/servers/:serverId/systemd/services/:name/start
POST   /admin/servers/:serverId/systemd/services/:name/stop
POST   /admin/servers/:serverId/systemd/services/:name/restart
...
```

### SSL 本机部署

SSL 的本机部署现在支持通过 agent 进行：

- 在 DeployTarget 的 LocalDeployConfig 中添加 `agentAddress` 字段
- 如果配置了 `agentAddress`，证书将通过 agent 部署到目标服务器
- 如果未配置，保持向后兼容，直接本机部署

## 鉴权机制

Agent 使用与主服务器相同的 JWT 机制进行鉴权：

1. 客户端（主服务器）调用 agent 时，在 gRPC metadata 中携带 `authorization: Bearer <token>`
2. Agent 验证 token 的签名和过期时间
3. Token 使用主服务器的客户端凭证系统生成（`ClientCredential`）

## 安全建议

1. **使用 mTLS**：在生产环境中，建议配置 mTLS 双向认证
2. **限制网络访问**：使用防火墙限制只有主服务器可以访问 agent 的 gRPC 端口
3. **定期轮换 JWT Secret**：建议定期更新 JWT secret，并同步更新主服务器和所有 agent
4. **最小权限原则**：agent 以 root 运行（因为需要操作 nginx/systemd），确保只有授权的主服务器可以访问

## 故障排查

### Agent 无法启动

1. 检查配置文件是否正确
2. 检查端口是否被占用：`lsof -i :50052`
3. 查看日志输出

### 主服务器无法连接 agent

1. 检查网络连通性：`telnet agent-host 50052`
2. 检查防火墙规则
3. 检查 agent 是否正在运行：`systemctl status agent`
4. 检查主服务器中配置的 `AgentGrpcAddress` 是否正确

### JWT 认证失败

1. 确认主服务器和 agent 的 JWT secret 一致
2. 检查 token 是否过期
3. 检查客户端凭证是否有效

### Nginx/Systemd 操作失败

1. 检查 agent 运行用户权限（需要 root）
2. 检查 nginx/systemd 是否已安装
3. 检查配置路径是否正确
4. 查看 agent 日志中的详细错误信息

## 性能考虑

- Agent 为每个 gRPC 连接创建独立的 goroutine，可以并发处理多个请求
- 文件操作使用原子写入（临时文件 + rename），确保安全性
- 命令执行有超时保护（默认 30 秒），避免阻塞

## 后续增强

1. **健康检查**：实现定期健康检查机制，主动发现 agent 故障
2. **指标收集**：agent 暴露 Prometheus 指标端点
3. **审计日志**：详细记录所有操作历史
4. **批量操作**：支持一次调用批量操作多个配置
5. **版本管理**：配置文件版本控制和回滚
