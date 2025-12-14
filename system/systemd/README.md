# Systemd 服务管理组件

本组件用于管理本机 Linux 系统的 systemd service unit。

## 功能概述

- **CRUD 操作**：对 `/etc/systemd/system` 下的 `*.service` unit 文件进行创建、读取、更新、删除
- **按参数生成**：通过结构化参数自动生成 unit 文件内容（支持预览、创建、更新）
- **生命周期控制**：启动（start）、停止（stop）、重启（restart）、重载（reload）
- **自启管理**：启用（enable）、禁用（disable）开机自启
- **状态查询**：获取服务运行状态（ActiveState、SubState 等）
- **日志查看**：获取服务最近日志（journalctl）

## 安全约束

1. **仅管理 `/etc/systemd/system` 目录**：不允许操作 `/usr/lib/systemd/system`、`/lib/systemd/system` 等系统目录
2. **文件名校验**：unit 名称必须以 `.service` 结尾，不能包含路径分隔符或 `..`
3. **原子写入**：使用"写临时文件 -> rename"的方式，避免半写入导致 unit 损坏

## HTTP API

所有接口挂载在 `/admin/systemd/services`，需要相应权限。

### 权限点

| 权限点 | 说明 |
|--------|------|
| `admin:systemd:service:read` | 查询服务列表、详情、状态、日志 |
| `admin:systemd:service:create` | 创建服务 |
| `admin:systemd:service:update` | 更新服务 |
| `admin:systemd:service:delete` | 删除服务 |
| `admin:systemd:service:control` | 启动/停止/重启/重载/启用/禁用 |

### 接口列表

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/` | create | 创建服务（name + content） |
| PUT | `/:name` | update | 更新服务内容 |
| DELETE | `/:name?force=1` | delete | 删除服务（force 可选） |
| GET | `/` | read | 列表/分页/搜索 |
| GET | `/:name` | read | 获取服务详情 |
| POST | `/generate` | read | 按参数生成 unit 内容（仅预览，不落盘） |
| POST | `/from-params` | create | 按参数创建服务（生成并落盘） |
| PUT | `/:name/from-params` | update | 按参数更新服务（生成并落盘） |
| POST | `/:name/start` | control | 启动服务 |
| POST | `/:name/stop` | control | 停止服务 |
| POST | `/:name/restart` | control | 重启服务 |
| POST | `/:name/reload` | control | 重载服务 |
| POST | `/:name/enable` | control | 启用自启 |
| POST | `/:name/disable` | control | 禁用自启 |
| GET | `/:name/status` | read | 获取运行状态 |
| GET | `/:name/logs?n=200&since=...` | read | 获取日志 |

## 请求/响应示例

### 创建服务

```http
POST /admin/systemd/services
Content-Type: application/json

{
  "name": "my-app.service",
  "content": "[Unit]\nDescription=My Application\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=/usr/local/bin/my-app\nRestart=always\n\n[Install]\nWantedBy=multi-user.target\n"
}
```

### 查询列表

```http
GET /admin/systemd/services?keyword=app&includeStatus=1&pageNum=1&size=20
```

响应：
```json
{
  "code": 200,
  "data": {
    "total": 5,
    "content": [
      {
        "name": "my-app.service",
        "description": "My Application",
        "modTime": "2024-01-15 10:30:00",
        "activeState": "active",
        "subState": "running",
        "unitFileState": "enabled"
      }
    ]
  }
}
```

### 获取状态

```http
GET /admin/systemd/services/my-app.service/status
```

响应：
```json
{
  "code": 200,
  "data": {
    "name": "my-app.service",
    "description": "My Application",
    "loadState": "loaded",
    "activeState": "active",
    "subState": "running",
    "unitFileState": "enabled",
    "mainPID": 12345,
    "execMainStartAt": "Mon 2024-01-15 10:00:00 CST",
    "memoryCurrent": 52428800,
    "result": "success"
  }
}
```

### 按参数生成预览

```http
POST /admin/systemd/services/generate
Content-Type: application/json

{
  "params": {
    "description": "My Application",
    "after": ["network.target"],
    "execStart": "/usr/local/bin/my-app --config /etc/my-app/config.yaml",
    "workingDirectory": "/var/lib/my-app",
    "user": "myapp",
    "group": "myapp",
    "environment": ["ENV=production", "PORT=8080"],
    "restart": "always",
    "restartSec": 5
  }
}
```

响应：
```json
{
  "code": 200,
  "data": {
    "content": "[Unit]\nDescription=My Application\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=/usr/local/bin/my-app --config /etc/my-app/config.yaml\nWorkingDirectory=/var/lib/my-app\nUser=myapp\nGroup=myapp\nEnvironment=ENV=production\nEnvironment=PORT=8080\nRestart=always\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n"
  }
}
```

### 按参数创建服务

```http
POST /admin/systemd/services/from-params
Content-Type: application/json

{
  "name": "my-app.service",
  "params": {
    "description": "My Application",
    "after": ["network.target"],
    "execStart": "/usr/local/bin/my-app --config /etc/my-app/config.yaml",
    "workingDirectory": "/var/lib/my-app",
    "user": "myapp",
    "group": "myapp",
    "environment": ["ENV=production", "PORT=8080"],
    "restart": "always",
    "restartSec": 5,
    "limitNOFILE": 65535
  }
}
```

响应：
```json
{
  "code": 200,
  "data": "创建成功"
}
```

### 按参数更新服务

```http
PUT /admin/systemd/services/my-app.service/from-params
Content-Type: application/json

{
  "params": {
    "description": "My Application v2",
    "after": ["network.target", "redis.service"],
    "execStart": "/usr/local/bin/my-app-v2 --config /etc/my-app/config.yaml",
    "workingDirectory": "/var/lib/my-app",
    "user": "myapp",
    "group": "myapp",
    "environment": ["ENV=production", "PORT=8080", "LOG_LEVEL=info"],
    "restart": "always",
    "restartSec": 10,
    "limitNOFILE": 65535,
    "extraServiceLines": ["Nice=-5", "CPUWeight=200"]
  }
}
```

响应：
```json
{
  "code": 200,
  "data": "更新成功"
}
```

### ServiceUnitParams 参数说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| description | string | 否 | 服务描述 |
| documentation | string | 否 | 文档链接 |
| after | []string | 否 | 在哪些 unit 之后启动 |
| wants | []string | 否 | 弱依赖的 unit |
| requires | []string | 否 | 强依赖的 unit |
| type | string | 否 | 服务类型（默认 simple） |
| execStart | string | **是** | 启动命令 |
| execStartPre | []string | 否 | 启动前命令 |
| execStartPost | []string | 否 | 启动后命令 |
| execStop | string | 否 | 停止命令 |
| execReload | string | 否 | 重载命令 |
| workingDirectory | string | 否 | 工作目录 |
| user | string | 否 | 运行用户 |
| group | string | 否 | 运行用户组 |
| environment | []string | 否 | 环境变量（KEY=VALUE 格式） |
| environmentFile | string | 否 | 环境变量文件路径 |
| restart | string | 否 | 重启策略（默认 always） |
| restartSec | int | 否 | 重启间隔秒数 |
| timeoutStartSec | int | 否 | 启动超时秒数 |
| timeoutStopSec | int | 否 | 停止超时秒数 |
| limitNOFILE | int | 否 | 最大文件描述符数 |
| limitNPROC | int | 否 | 最大进程数 |
| wantedBy | []string | 否 | 被哪些 target 依赖（默认 multi-user.target） |
| requiredBy | []string | 否 | 被哪些 target 强依赖 |
| alias | []string | 否 | 别名 |
| extraUnitLines | []string | 否 | [Unit] 段额外行 |
| extraServiceLines | []string | 否 | [Service] 段额外行 |
| extraInstallLines | []string | 否 | [Install] 段额外行 |

## 运维要求

该服务进程需要具备以下权限：

1. **写入权限**：`/etc/systemd/system` 目录
2. **执行权限**：`systemctl` 命令
3. **读取权限**：`journalctl` 命令

建议配置方式：
- 以 root 用户运行
- 或通过 sudoers 精确授权（仅 systemctl/journalctl + 目标目录写权限）

## 目录结构

```
system/systemd/
├── module.go                           # 模块门面
├── router.go                           # 路由注册入口
├── README.md                           # 本文档
├── api/
│   ├── client/
│   │   └── systemd_client.go           # 对外客户端（进程内调用）
│   └── dto/
│       └── systemd_dto.go              # 对外 DTO 定义
├── internal/
│   ├── app/
│   │   └── app.go                      # 应用层编排
│   ├── model/
│   │   └── dto/
│   │       └── systemd_dto.go          # 内部 DTO 定义
│   └── service/
│       ├── unit_file_service.go        # unit 文件 CRUD
│       ├── unit_generator_service.go   # unit 内容生成（按参数）
│       ├── systemctl_service.go        # systemctl 命令封装
│       └── journal_service.go          # journalctl 日志封装
└── external/
    └── http/
        └── systemd_service_controller.go  # HTTP 控制器
```

## 平台兼容性

- **Linux**：完全支持
- **macOS/Windows**：编译通过，但运行时会返回"仅支持 Linux 平台"错误

