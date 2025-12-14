# Application 组件

## 概述

Application 组件是应用部署编排的核心，负责调配 `ssl`、`nginx`、`systemd`、`registry` 组件，实现应用的自动化部署、更新与回滚。

## 功能

- **应用管理**：创建、查询、更新、删除应用定义
- **产物存储**：支持本地文件系统或 OSS 存储构建产物
- **部署编排**：自动完成证书申请/复用、Nginx 配置、Systemd 服务、服务注册
- **版本管理**：保留 N 个历史版本，支持快速回滚
- **多应用类型**：支持 backend（后端）、frontend（前端）、fullstack（前后端）

## 应用类型

| 类型 | 说明 | 部署动作 |
|------|------|----------|
| `backend` | 后端服务 | systemd unit + nginx 反代 + registry 注册 |
| `frontend` | 纯前端静态站点 | nginx 静态站点 |
| `fullstack` | 前后端一体 | backend + frontend 组合 |

## 证书策略

部署时自动处理 TLS 证书：
1. 优先按域名匹配已有证书（支持通配符，如 `*.example.com`）
2. 命中未过期且可用的证书则直接复用
3. 未命中时触发 ACME 申请新证书

## 目录结构

```
system/application/
├── module.go           # 组件门面
├── router.go           # 路由注册入口
├── migrate.go          # 数据库迁移入口
├── README.md           # 本文档
├── api/
│   ├── client/         # 对外客户端（供其他组件调用）
│   ├── dto/            # 对外 DTO
│   └── proto/          # gRPC 协议定义
├── external/
│   ├── http/           # HTTP Controller
│   └── grpc/           # gRPC Service
└── internal/
    ├── app/            # 应用层编排
    ├── dao/            # 数据访问层
    ├── model/          # 领域模型
    │   └── dto/        # 内部 DTO
    └── service/        # 业务逻辑层
        └── storage/    # 存储抽象与实现
```

## 配置

在 `resources/*.yaml` 中添加：

```yaml
application:
  storageMode: local    # local 或 oss
  localArtifactDir: /opt/apps/artifacts
  releaseDir: /opt/apps/releases
  keepReleases: 5       # 保留版本数
  uploadMaxBytes: 536870912  # 512MB
```

## HTTP API

### 管理端（需要 admin 权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/admin/applications` | 创建应用 |
| GET | `/admin/applications` | 应用列表 |
| GET | `/admin/applications/:id` | 应用详情 |
| PUT | `/admin/applications/:id` | 更新应用 |
| DELETE | `/admin/applications/:id` | 删除应用 |
| POST | `/admin/applications/:id/deploy` | 触发部署 |
| POST | `/admin/applications/:id/rollback` | 回滚到指定版本 |
| GET | `/admin/applications/:id/releases` | 版本列表 |
| GET | `/admin/deployments/:id` | 部署详情 |

## gRPC API

供本地打包程序调用：

- `UploadArtifact`：流式上传构建产物
- `Deploy`：触发部署
- `GetDeployment`：查询部署状态

## 使用示例

```go
// 在 app/app.go 中
ApplicationModule: application.NewModule(
    sslModule,
    nginxModule,
    systemdModule,
    registryModule,
),

// 在 router/router.go 中
application.RegisterRoutes(a.ApplicationModule, api, admin)

// 在 main.go 中注册 gRPC
grpcServer.RegisterService(appRoot.ApplicationModule.GRPCService)
```

