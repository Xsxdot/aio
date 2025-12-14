# Server 组件 API 文档

## 概述

Server 组件提供服务器清单管理、状态上报与聚合查询能力。

**一期功能**：
- 服务器资产管理（增删改查、标签）
- 状态上报接口（供 agent/脚本调用）
- 主页聚合查询接口（返回所有服务器 + 实时状态）

**二期规划**：
- Agent 模式：自动采集服务器指标并上报
- 部署执行：通过 agent 执行部署、配置变更等操作

---

## HTTP API

### 1. 后台管理接口

**鉴权要求**：需要管理员登录（`AdminAuth`）并具有对应权限。

#### 1.1 获取服务器列表（分页）

```http
GET /admin/servers
```

**权限**：`admin:server:read`

**查询参数**：
- `name` (string, optional)：服务器名称（模糊搜索）
- `tag` (string, optional)：标签过滤
- `enabled` (boolean, optional)：启用状态过滤
- `pageNum` (int, optional)：页码，默认 1
- `size` (int, optional)：每页大小，默认 10

**响应示例**：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 10,
    "content": [
      {
        "id": 1,
        "name": "dev-server-1",
        "host": "10.0.12.13",
        "agentGrpcAddress": "10.0.12.13:50052",
        "enabled": true,
        "tags": {
          "env": "dev",
          "region": "cn-north"
        },
        "comment": "开发环境服务器1",
        "createdAt": "2025-01-01T00:00:00Z",
        "updatedAt": "2025-01-01T00:00:00Z"
      }
    ]
  }
}
```

#### 1.2 获取服务器详情

```http
GET /admin/servers/:id
```

**权限**：`admin:server:read`

**路径参数**：
- `id` (int64)：服务器 ID

**响应**：返回单个服务器详情（格式同上）。

#### 1.3 创建服务器

```http
POST /admin/servers
```

**权限**：`admin:server:write`

**请求体**：

```json
{
  "name": "prod-server-1",
  "host": "192.168.1.10",
  "agentGrpcAddress": "192.168.1.10:50052",
  "enabled": true,
  "tags": {
    "env": "prod",
    "region": "cn-east"
  },
  "comment": "生产环境服务器1"
}
```

**字段说明**：
- `name` (string, required, max=100)：服务器名称（唯一）
- `host` (string, required, max=255)：服务器地址（IP/域名）
- `agentGrpcAddress` (string, optional, max=255)：Agent gRPC 地址（预留）
- `enabled` (boolean, optional)：是否启用，默认 false
- `tags` (map, optional)：标签（key-value 形式）
- `comment` (string, optional, max=500)：备注

#### 1.4 更新服务器

```http
PUT /admin/servers/:id
```

**权限**：`admin:server:write`

**路径参数**：
- `id` (int64)：服务器 ID

**请求体**（所有字段可选）：

```json
{
  "name": "prod-server-1-updated",
  "host": "192.168.1.10",
  "enabled": false,
  "tags": {
    "env": "prod",
    "region": "cn-east",
    "version": "v2"
  },
  "comment": "已更新"
}
```

#### 1.5 删除服务器

```http
DELETE /admin/servers/:id
```

**权限**：`admin:server:write`

**路径参数**：
- `id` (int64)：服务器 ID

**说明**：删除服务器时会同时删除对应的状态记录。

#### 1.6 获取所有服务器状态（主页用）

```http
GET /admin/servers/status
```

**权限**：`admin:server:read`

**响应示例**：

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "id": 1,
      "name": "dev-server-1",
      "host": "10.0.12.13",
      "agentGrpcAddress": "10.0.12.13:50052",
      "enabled": true,
      "tags": {
        "env": "dev",
        "region": "cn-north"
      },
      "comment": "开发环境服务器1",
      "cpuPercent": 45.5,
      "memUsed": 8589934592,
      "memTotal": 17179869184,
      "load1": 2.5,
      "load5": 2.1,
      "load15": 1.8,
      "diskItems": [
        {
          "mountPoint": "/",
          "used": 107374182400,
          "total": 214748364800,
          "percent": 50.0
        }
      ],
      "collectedAt": "2025-01-10T10:00:00Z",
      "reportedAt": "2025-01-10T10:00:05Z",
      "errorMessage": "",
      "statusSummary": "online"
    },
    {
      "id": 2,
      "name": "dev-server-2",
      "host": "10.0.12.14",
      "agentGrpcAddress": "10.0.12.14:50052",
      "enabled": true,
      "tags": {
        "env": "dev"
      },
      "comment": "开发环境服务器2",
      "statusSummary": "unknown"
    }
  ]
}
```

**字段说明**：
- `statusSummary`：服务器状态摘要
  - `online`：在线（最近 5 分钟内有上报）
  - `offline`：离线（超过 5 分钟未上报）
  - `error`：错误（上报时带有错误信息）
  - `unknown`：未知（从未上报过）

---

### 2. 状态上报接口

**鉴权要求**：需要客户端凭证（`ClientAuth`），使用 JWT Token。

#### 2.1 上报服务器状态

```http
POST /api/servers/:id/status/report
```

**鉴权**：`Bearer {client_token}`（在 `Authorization` header 中）

**路径参数**：
- `id` (int64)：服务器 ID

**请求体**：

```json
{
  "cpuPercent": 45.5,
  "memUsed": 8589934592,
  "memTotal": 17179869184,
  "load1": 2.5,
  "load5": 2.1,
  "load15": 1.8,
  "diskItems": [
    {
      "mountPoint": "/",
      "used": 107374182400,
      "total": 214748364800,
      "percent": 50.0
    },
    {
      "mountPoint": "/data",
      "used": 536870912000,
      "total": 1073741824000,
      "percent": 50.0
    }
  ],
  "collectedAt": "2025-01-10T10:00:00Z",
  "errorMessage": ""
}
```

**字段说明**：
- `cpuPercent` (float64)：CPU 使用率（%）
- `memUsed` (int64)：内存已使用（字节）
- `memTotal` (int64)：内存总量（字节）
- `load1`、`load5`、`load15` (float64)：1/5/15 分钟负载
- `diskItems` (array)：磁盘信息列表
  - `mountPoint` (string)：挂载点
  - `used` (int64)：已使用（字节）
  - `total` (int64)：总量（字节）
  - `percent` (float64)：使用率（%）
- `collectedAt` (timestamp)：采集时间（ISO 8601 格式）
- `errorMessage` (string)：错误信息（可选，agent 自报错误）

**响应**：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "上报成功"
  }
}
```

---

## gRPC API

### 服务定义

```protobuf
service ServerService {
  // 上报服务器状态（供 agent/脚本调用）
  rpc ReportServerStatus(ReportServerStatusRequest) returns (ReportServerStatusResponse);
  
  // 获取所有服务器状态（主页聚合查询）
  rpc GetAllServerStatus(GetAllServerStatusRequest) returns (GetAllServerStatusResponse);
  
  // 获取单个服务器状态
  rpc GetServerStatus(GetServerStatusRequest) returns (ServerStatusInfo);
}
```

**鉴权**：所有 gRPC 接口需要客户端 JWT Token（在 metadata 的 `authorization` 或 `token` 字段中）。

详细定义参见：`system/server/api/proto/server.proto`

---

## 配置示例

### resources/dev.yaml

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
    - name: 'dev-server-2'
      host: '10.0.12.14'
      agent_grpc_address: '10.0.12.14:50052'
      enabled: true
      tags:
        env: 'dev'
        region: 'cn-north'
      comment: '开发环境服务器2'
```

**说明**：
- `bootstrap` 数组中的服务器会在应用启动时自动 upsert 到数据库（按 `name` 判重）。
- 如果数据库中已存在同名服务器，会更新其配置；否则创建新记录。

---

## 二期规划：Agent 模式

### Agent 启动方式

```bash
# 使用 -mode agent 参数启动 agent 模式
./xiaozhizhang -mode agent -config /path/to/agent.yaml
```

### Agent 功能（待实现）

- **指标采集**：自动采集本机 CPU/内存/磁盘/负载等指标。
- **定时上报**：定期调用平台的上报接口（HTTP 或 gRPC）。
- **执行能力**：接收平台下发的部署/配置变更等指令并执行（预留）。

### Agent 鉴权

- 复用平台现有的 ClientAuth/JWT 体系。
- Agent 启动时配置 `client_key` 和 `client_secret`，向平台申请 token。
- 上报状态时在请求头中携带 token。

---

## 常见问题

### 1. 如何获取客户端 Token？

- 在平台的「客户端管理」中创建一个客户端凭证（Client Key + Secret）。
- 调用 `/api/auth/client` 接口（参见 user 组件文档）获取 token。
- 使用 token 调用上报接口。

### 2. 服务器状态多久更新一次？

- 由 agent/脚本自行决定上报频率（建议 30-60 秒）。
- 平台不主动拉取，采用被动接收上报的方式。

### 3. 如何判断服务器在线/离线？

- 平台会根据 `reportedAt`（最后上报时间）判断：
  - 最近 5 分钟内有上报：`online`
  - 超过 5 分钟未上报：`offline`
  - 从未上报：`unknown`

### 4. 一期为什么不实现 Agent？

- 先打通「服务器清单 + 状态存储 + 查询接口」的基础能力。
- Agent 后续还需要扩展部署/配置变更等执行类能力，二期统一规划实现。

---

## 权限码说明

| 权限码                 | 说明               |
|------------------------|------------------|
| `admin:server:read`    | 查询服务器及状态   |
| `admin:server:write`   | 管理服务器（增删改）|

**说明**：权限码需要在管理员角色中配置，详见用户组件文档。


