# Server 组件

## 概述

Server 组件负责服务器清单管理、状态上报与聚合查询。

**一期功能**（已实现）：
- 服务器资产管理：增删改查、标签、启停用
- Bootstrap 配置：从 YAML 自动 upsert 服务器到数据库
- 状态上报接口：供 agent/脚本调用，写入服务器最新状态
- 主页聚合查询：返回所有服务器 + 最新状态（online/offline/unknown）

**二期规划**：
- Agent 模式：自动采集服务器指标（CPU/内存/磁盘/负载）并上报
- 部署执行：通过 agent 执行应用部署、配置变更等操作

---

## 目录结构

```
system/server/
├── api/                        # 对外接口定义
│   ├── client/                 # 对外 Client
│   │   └── server_client.go
│   └── proto/                  # gRPC proto 定义
│       ├── server.proto
│       ├── server.pb.go
│       └── server_grpc.pb.go
├── external/                   # 外部适配层（HTTP/gRPC）
│   ├── grpc/                   # gRPC 服务实现
│   │   └── server_service.go
│   └── http/                   # HTTP Controller
│       ├── server_controller.go
│       └── report_controller.go
├── internal/                   # 组件内部实现
│   ├── app/                    # 应用层编排
│   │   ├── app.go
│   │   ├── bootstrap.go        # Bootstrap 服务器初始化
│   │   ├── server_manage.go    # 服务器 CRUD
│   │   └── server_status.go    # 状态上报与聚合查询
│   ├── dao/                    # 数据访问层
│   │   ├── server_dao.go
│   │   └── server_status_dao.go
│   ├── model/                  # 数据模型
│   │   ├── server.go
│   │   ├── server_status.go
│   │   └── dto/                # 内部 DTO
│   │       └── server_dto.go
│   └── service/                # 业务逻辑层
│       ├── server_service.go
│       └── server_status_service.go
├── migrate.go                  # 数据库迁移入口
├── module.go                   # 组件门面 Module
├── router.go                   # 路由注册入口
└── README.md                   # 本文档
```

---

## 数据模型

### ServerModel（服务器表）

- 表名：`server_servers`
- 字段：
  - `id`：主键
  - `name`：服务器名称（唯一）
  - `host`：服务器地址
  - `agent_grpc_address`：Agent gRPC 地址（预留）
  - `enabled`：是否启用
  - `tags`：标签（JSON）
  - `comment`：备注
  - `created_at`、`updated_at`

### ServerStatusModel（服务器状态表）

- 表名：`server_status`
- 字段：
  - `id`：主键
  - `server_id`：服务器 ID（唯一，外键）
  - `cpu_percent`：CPU 使用率
  - `mem_used`、`mem_total`：内存使用/总量
  - `load1`、`load5`、`load15`：负载
  - `disk_items`：磁盘信息（JSON）
  - `collected_at`：采集时间
  - `reported_at`：上报时间
  - `error_message`：错误信息
  - `created_at`、`updated_at`

**说明**：每个服务器最多一条状态记录，采用 upsert 方式更新。

---

## 接口

### HTTP（后台管理 + 上报）

- **后台管理**（`/admin/servers`）：
  - `GET /`：分页查询服务器
  - `GET /:id`：获取详情
  - `POST /`：创建
  - `PUT /:id`：更新
  - `DELETE /:id`：删除
  - `GET /status`：获取所有服务器状态（主页用）

- **状态上报**（`/api/servers/:id/status/report`）：
  - `POST`：上报状态（需要 ClientAuth）

### gRPC

- `ReportServerStatus`：上报服务器状态
- `GetAllServerStatus`：获取所有服务器状态
- `GetServerStatus`：获取单个服务器状态

**鉴权**：所有接口需要 ClientAuth/JWT Token。

详细文档：[docs/server_component_api.md](/Users/xushixin/workspace/go/xiaozhizhang/docs/server_component_api.md)

---

## Bootstrap 配置

在 `resources/*.yaml` 中配置 bootstrap 服务器列表：

```yaml
server:
  bootstrap:
    - name: 'dev-server-1'
      host: '10.0.12.13'
      agent_grpc_address: '10.0.12.13:50052'
      enabled: true
      tags:
        env: 'dev'
        region: 'cn-north'
      comment: '开发环境服务器1'
```

应用启动时会自动将 bootstrap 列表 upsert 到数据库（按 `name` 判重）。

---

## 使用示例

### 1. 创建服务器

```bash
curl -X POST http://localhost:9000/admin/servers \
  -H "Authorization: Bearer {admin_token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-server-1",
    "host": "192.168.1.10",
    "agentGrpcAddress": "192.168.1.10:50052",
    "enabled": true,
    "tags": {"env": "prod"},
    "comment": "生产服务器"
  }'
```

### 2. 上报状态

```bash
curl -X POST http://localhost:9000/api/servers/1/status/report \
  -H "Authorization: Bearer {client_token}" \
  -H "Content-Type: application/json" \
  -d '{
    "cpuPercent": 45.5,
    "memUsed": 8589934592,
    "memTotal": 17179869184,
    "load1": 2.5,
    "diskItems": [
      {
        "mountPoint": "/",
        "used": 107374182400,
        "total": 214748364800,
        "percent": 50.0
      }
    ],
    "collectedAt": "2025-01-10T10:00:00Z"
  }'
```

### 3. 查询所有服务器状态

```bash
curl -X GET http://localhost:9000/admin/servers/status \
  -H "Authorization: Bearer {admin_token}"
```

---

## 二期：Agent 模式（待实现）

### 功能

- 自动采集本机系统指标（CPU/内存/磁盘/负载）
- 定时调用平台上报接口
- 接收平台指令并执行部署/配置变更等操作

### 启动方式

```bash
./xiaozhizhang -mode agent -config /path/to/agent.yaml
```

### Agent 配置

```yaml
agent:
  server_id: 1                        # 对应平台的服务器 ID
  platform_url: 'http://platform:9000'  # 平台地址
  client_key: 'agent-client-key'      # 客户端凭证
  client_secret: 'agent-client-secret'
  report_interval: 30s                # 上报间隔
```

---

## 技术细节

### 状态判断逻辑

- **online**：最后上报时间距今 ≤ 5 分钟
- **offline**：最后上报时间距今 > 5 分钟
- **error**：上报时带有 `errorMessage`
- **unknown**：从未上报过状态

### 鉴权

- 后台管理接口：`AdminAuth`（管理员登录 + 权限码）
- 上报接口：`ClientAuth`（客户端凭证 JWT）
- gRPC：复用 ClientAuth

---

## 依赖

- `base.DB`：数据库连接
- `base.AdminAuth`：管理员鉴权
- `base.ClientAuth`：客户端鉴权
- `pkg/core/logger`：日志
- `pkg/core/err`：错误处理
- `pkg/core/mvc`：基础 DAO/Service 封装

---

## 参考

- API 文档：[docs/server_component_api.md](/Users/xushixin/workspace/go/xiaozhizhang/docs/server_component_api.md)
- Proto 定义：[api/proto/server.proto](/Users/xushixin/workspace/go/xiaozhizhang/system/server/api/proto/server.proto)
- 配置示例：[resources/dev.yaml](/Users/xushixin/workspace/go/xiaozhizhang/resources/dev.yaml)


