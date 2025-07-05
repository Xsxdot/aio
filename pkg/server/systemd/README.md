# Systemd 服务管理模块

## 概述

Systemd模块提供了通过服务器ID远程管理systemd服务的功能。支持对远程服务器上的systemd服务进行完整的生命周期管理，包括创建、更新、删除、启动、停止、重启等操作。

## 核心功能

### 🎯 服务生命周期管理
- **启动服务** - 启动指定的systemd服务
- **停止服务** - 停止指定的systemd服务  
- **重启服务** - 重启指定的systemd服务
- **重载服务** - 重载服务配置而不重启服务

### 📋 服务配置管理
- **创建服务** - 在远程服务器上创建新的systemd服务文件
- **更新服务** - 修改现有服务的配置
- **删除服务** - 完全删除服务文件和配置
- **查询服务** - 获取单个服务的详细信息

### 🔍 服务监控与查询
- **服务列表** - 获取服务器上所有systemd服务列表
- **服务状态** - 查询服务的运行状态
- **服务日志** - 获取服务的journald日志
- **分页查询** - 支持大量服务的分页显示

### ⚙️ 服务启用管理
- **启用服务** - 设置服务开机自启动
- **禁用服务** - 取消服务开机自启动
- **守护进程重载** - 重载systemd守护进程配置

## 技术特性

### 🔒 安全性
- 通过服务器ID进行身份验证
- 所有systemd操作都需要sudo权限
- 支持SSH密钥和密码认证

### 📊 过滤与搜索
- **状态过滤** - 按服务状态筛选（active/inactive/failed等）
- **启用状态过滤** - 按开机启动状态筛选
- **名称模式匹配** - 支持服务名称模式搜索
- **分页支持** - 支持大量数据的分页查询

### 🎨 服务配置
- **服务类型** - 支持simple、forking、oneshot、notify、dbus等类型
- **执行命令** - 支持ExecStart、ExecReload、ExecStop配置
- **运行环境** - 支持工作目录、用户、组、环境变量配置
- **重启策略** - 支持多种重启策略配置

## API 接口

### 服务列表
```http
GET /api/systemd/servers/{serverId}/services
```
查询参数：
- `status` - 服务状态过滤
- `enabled` - 启用状态过滤
- `pattern` - 名称模式匹配
- `userOnly` - 仅显示用户创建的服务（默认false）
- `limit` - 分页大小（默认20）
- `offset` - 分页偏移（默认0）

### 服务管理
```http
GET    /api/systemd/servers/{serverId}/services/{serviceName}     # 获取服务信息
POST   /api/systemd/servers/{serverId}/services/{serviceName}     # 创建服务
PUT    /api/systemd/servers/{serverId}/services/{serviceName}     # 更新服务
DELETE /api/systemd/servers/{serverId}/services/{serviceName}     # 删除服务
```

### 服务操作
```http
POST /api/systemd/servers/{serverId}/services/{serviceName}/start     # 启动服务
POST /api/systemd/servers/{serverId}/services/{serviceName}/stop      # 停止服务
POST /api/systemd/servers/{serverId}/services/{serviceName}/restart   # 重启服务
POST /api/systemd/servers/{serverId}/services/{serviceName}/reload    # 重载服务
POST /api/systemd/servers/{serverId}/services/{serviceName}/enable    # 启用服务
POST /api/systemd/servers/{serverId}/services/{serviceName}/disable   # 禁用服务
GET  /api/systemd/servers/{serverId}/services/{serviceName}/status    # 获取状态
GET  /api/systemd/servers/{serverId}/services/{serviceName}/logs      # 获取日志
```

### 系统操作
```http
POST /api/systemd/servers/{serverId}/daemon-reload    # 重载守护进程
POST /api/systemd/servers/{serverId}/reload          # 重载systemd
```

## 使用示例

### 创建服务
```json
POST /api/systemd/servers/server-001/services/myapp
{
  "description": "My Application Service",
  "type": "simple",
  "execStart": "/usr/local/bin/myapp",
  "workingDir": "/opt/myapp",
  "user": "myapp",
  "group": "myapp",
  "environment": {
    "ENV": "production",
    "PORT": "8080"
  },
  "restart": "on-failure",
  "enabled": true
}
```

### 获取服务列表
```http
GET /api/systemd/servers/server-001/services?status=active&limit=50&offset=0
```

### 获取用户创建的服务列表
```http
GET /api/systemd/servers/server-001/services?userOnly=true&limit=50
```

### 获取服务日志
```http
GET /api/systemd/servers/server-001/services/myapp/logs?lines=200&follow=false
```

## 数据结构

### SystemdService
```go
type SystemdService struct {
    Name        string            `json:"name"`        // 服务名称
    Status      ServiceState      `json:"status"`      // 服务状态
    Enabled     bool             `json:"enabled"`     // 是否开机启动
    Description string           `json:"description"` // 服务描述
    Type        ServiceType      `json:"type"`        // 服务类型
    ExecStart   string           `json:"execStart"`   // 启动命令
    ExecReload  string           `json:"execReload"`  // 重载命令
    ExecStop    string           `json:"execStop"`    // 停止命令
    WorkingDir  string           `json:"workingDir"`  // 工作目录
    User        string           `json:"user"`        // 运行用户
    Group       string           `json:"group"`       // 运行组
    Environment map[string]string `json:"environment"` // 环境变量
    PIDFile     string           `json:"pidFile"`     // PID文件路径
    Restart     string           `json:"restart"`     // 重启策略
    CreatedAt   time.Time        `json:"createdAt"`   // 创建时间
    UpdatedAt   time.Time        `json:"updatedAt"`   // 更新时间
}
```

### 服务状态
- `active` - 活动状态
- `inactive` - 非活动状态
- `failed` - 失败状态
- `activating` - 启动中
- `deactivating` - 停止中

### 服务类型
- `simple` - 简单服务
- `forking` - 分叉服务
- `oneshot` - 一次性服务
- `notify` - 通知服务
- `dbus` - D-Bus服务

## 集成说明

### 依赖注入
```go
// 创建服务管理器
manager := systemd.NewManager(serverService, executor)

// 创建API处理器
apiHandler := systemd.NewAPIHandler(manager)

// 注册路由
apiHandler.RegisterRoutes(router)
```

### 接口实现
模块需要实现以下接口：
- `ServerProvider` - 提供服务器信息
- `CommandExecutor` - 执行远程命令

## 注意事项

1. **权限要求** - 所有systemd操作都需要sudo权限
2. **服务器连接** - 确保服务器SSH连接正常
3. **服务文件路径** - 服务文件创建在`/etc/systemd/system/`目录
4. **守护进程重载** - 创建或修改服务后需要重载守护进程
5. **日志大小** - 获取日志时注意设置合理的行数限制

## 错误处理

模块提供详细的错误信息，包括：
- 连接错误
- 权限错误  
- 命令执行错误
- 服务不存在错误
- 配置文件错误

所有错误都会包含具体的错误描述，便于问题排查和处理。 