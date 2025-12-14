## 本地打包程序对接 `application` 部署组件指南

本文面向「本地打包程序（Packager）」的开发者，描述如何对接本项目的 `system/application` 组件，实现：**上传产物 → 触发部署/更新 → 轮询部署状态**。

> 部署拓扑：**per-host agent**（每台服务器跑一套本项目实例）。因此“对某台机器部署”，就是把 gRPC 请求打到那台机器上的 agent。

---

## 前置条件

- **目标机器上已启动 agent**：即本项目服务进程，且 `resources/<env>.yaml` 配置了 `grpc.address` 并成功监听。
- **已创建客户端凭证**（`clientKey/clientSecret`）：用于 gRPC 鉴权（JWT）。
- **部署平台运行在 Linux**：当前 systemd 管理与实际部署依赖 Linux（`systemd` 组件会校验平台）。

---

## 认证（获取 gRPC Token）

本项目已提供 `user.v1.ClientAuthService`：
- `AuthenticateClient(client_key, client_secret) -> access_token`
- `RenewToken()`：通过 metadata 的 `authorization` 续期

更详细的 user 组件对接可参考：`system/user/INTEGRATION.md`。

### 1) 创建客户端凭证（管理员 HTTP）

管理员先登录并创建客户端凭证（只会在创建时返回一次 secret）：

```bash
curl -X POST http://<agent-host>:<http-port>/admin/client-credentials \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "name": "packager",
    "description": "本地打包程序",
    "ipWhitelist": ["127.0.0.1", "192.168.1.0/24"]
  }'
```

响应包含：
- `clientKey`
- `clientSecret`（只返回一次）

### 2) 使用客户端凭证获取 token（gRPC）

使用 `AuthenticateClient` 获取 JWT：

```bash
grpcurl -plaintext -d '{
  "client_key":"<clientKey>",
  "client_secret":"<clientSecret>"
}' <agent-grpc-host:port> user.v1.ClientAuthService/AuthenticateClient
```

拿到：
- `accessToken`
- `tokenType`（通常是 `Bearer`）

后续调用 `application.v1.ApplicationService/*` 时，把 token 放进 gRPC metadata：
- `authorization: Bearer <accessToken>`  
或  
- `token: <accessToken>`

（服务端支持两种 key，见 `pkg/grpc/middleware.go` 的 `extractToken`）

---

## 对接流程总览（Packager 需要做什么）

对某台机器部署的完整步骤：

- **Step A：上传产物**（`UploadArtifact` 流式）
- **Step B：触发部署/更新**（`Deploy`）
- **Step C：轮询部署状态**（`GetDeployment`）

> 注意：因为是 per-host agent，你要对 4 台机器部署，就需要分别对 4 台 agent 执行 A/B/C。

---

## A. 上传产物（gRPC 流式 UploadArtifact）

### 接口

- **Service**：`application.v1.ApplicationService`
- **Method**：`UploadArtifact(stream UploadArtifactChunk) returns (UploadArtifactResponse)`
- **Proto**：`system/application/api/proto/application.proto`

### 传输约定

- 第一个 chunk **必须**携带 `meta`（产物元信息）
- 后续 chunk 只需要 `data`
- 推荐把产物打包成 `tar.gz`（或 `tgz`），便于服务端解压；也支持单文件上传（例如后端二进制）。

`ArtifactMeta` 关键字段：
- `application_id`：要部署的应用 ID（来自管理员创建的 Application）
- `file_name`：例如 `backend.tgz`、`dist.tar.gz`、`myapp`（单文件）
- `artifact_type`：`backend` 或 `frontend`
- `size`：文件大小（byte）
- `sha256`：可选（服务端会计算并保存）
- `content_type`：可选

上传成功后，返回：
- `artifact_id`：后续 Deploy 里使用
- `storage_mode`：`local` 或 `oss`

---

## B. 触发部署/更新（gRPC Deploy）

### 接口

- **Method**：`Deploy(DeployRequest) returns (DeployResponse)`

`DeployRequest` 字段：
- `application_id`：应用 ID
- `version`：版本号（建议用构建号/commit hash/时间戳）
- `backend_artifact_id`：后端产物 ID（可为 0）
- `frontend_artifact_id`：前端产物 ID（可为 0）
- `spec_json`：部署 JSON（见下文 spec 说明）
- `operator`：操作人标识（可填 packager 名称/机器名）

返回：
- `deployment_id`：用于轮询
- `release_id`：版本记录 ID
- `status`：初始状态

> 当前实现会在后台异步执行部署，Deploy 返回后请用 `GetDeployment` 轮询结果。

---

## C. 轮询部署状态（gRPC GetDeployment）

### 接口

- **Method**：`GetDeployment(GetDeploymentRequest) returns (DeploymentInfo)`

当 `DeploymentInfo.status` 为：
- `success`：部署成功
- `failed`：部署失败（看 `error_message` 与 `logs`）
- `running/pending`：继续轮询

建议轮询策略：
- 间隔 1–2 秒
- 超时 10–20 分钟（按你们包大小与机器性能调整）

---

## `spec_json`（平台 JSON spec）说明与 factory.yaml 映射

`spec_json` 用于描述如何把产物落到机器上以及如何启动/对外暴露。

当前 `application` 组件内部使用的结构（关键字段）：

```json
{
  "domain": "backend-test.yourdomain.com",
  "ssl": true,
  "sslKeyPath": "/opt/cert/your-domain.key",
  "sslCertPath": "/opt/cert/your-domain.crt",
  "backend": {
    "port": 8080,
    "startCommand": "${installPath}/backend-app -env=${env} -config=${installPath}/config/app.yaml",
    "healthUrl": "http://${host}:8080/health",
    "workingDir": ""
  },
  "frontend": {
    "rootPath": "",
    "indexFile": "index.html"
  }
}
```

与 `docs/factory.yaml.example` 的映射建议：

- `domain` ← `testEnv.domain / prodEnv.domain`
- `backend.port` ← `testEnv.port / prodEnv.port`
- `backend.startCommand` ← `startCommand`
- `ssl / sslKeyPath / sslCertPath` ← `ssl / sslKeyPath / sslCertPath`
- `frontend.rootPath`：
  - 如果你把前端产物解压到 release 下的 `web/`，则可以不传（服务端默认用 `<releaseDir>/web`）

变量替换（服务端会做）：
- `${installPath}` → 本次 release 解压目录（例如 `/opt/apps/releases/<project>/<name>/<env>/<version>`）
- `${env}` → 应用 env
- `${version}` → version

---

## Packager 侧 Go 伪代码示例（高层）

下面示例展示核心调用顺序（省略错误处理与重试）：

```go
// 1) Dial gRPC
conn, _ := grpc.Dial(agentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
defer conn.Close()

// 2) AuthenticateClient -> token
authCli := userproto.NewClientAuthServiceClient(conn)
authResp, _ := authCli.AuthenticateClient(ctx, &userproto.AuthenticateClientRequest{
  ClientKey: clientKey,
  ClientSecret: clientSecret,
})
token := authResp.AccessToken

// 3) attach metadata
md := metadata.Pairs("authorization", "Bearer "+token)
ctx = metadata.NewOutgoingContext(ctx, md)

// 4) UploadArtifact(stream)
appCli := appproto.NewApplicationServiceClient(conn)
stream, _ := appCli.UploadArtifact(ctx)
stream.Send(&appproto.UploadArtifactChunk{Meta: &appproto.ArtifactMeta{
  ApplicationId: appID,
  FileName: fileName,
  ArtifactType: "backend",
  Size: fileSize,
}})
// 循环 send chunk.Data ...
uploadResp, _ := stream.CloseAndRecv()
artifactID := uploadResp.ArtifactId

// 5) Deploy
depResp, _ := appCli.Deploy(ctx, &appproto.DeployRequest{
  ApplicationId: appID,
  Version: version,
  BackendArtifactId: artifactID,
  SpecJson: specJSON,
})

// 6) Poll GetDeployment
for {
  info, _ := appCli.GetDeployment(ctx, &appproto.GetDeploymentRequest{DeploymentId: depResp.DeploymentId})
  if info.Status == "success" || info.Status == "failed" { break }
  time.Sleep(time.Second)
}
```

---

## 常见问题 / 注意事项

- **一次部署多台机器**：对每台机器的 agent 重复执行“上传 + Deploy + 轮询”。（因为 agent 与部署执行在同机）
- **产物建议统一为 tar.gz**：服务端会自动解压；单文件也支持，但不会解压。
- **systemd service name**：服务端会创建 `<project>-<name>-<env>.service`。
- **gRPC token 传递**：metadata 支持 `authorization: Bearer xxx` 或 `token: xxx`。
- **存储模式**：
  - `local`：产物落到 agent 本机目录（见 `application.localArtifactDir`）
  - `oss`：产物上传 OSS，但 **仍需要对目标 agent 调用 UploadArtifact** 以在该 agent 的 DB 中生成 artifact 记录（当前接口以 `artifact_id` 作为 Deploy 引用）。

---

## 你需要改哪些地方（Packager 改造清单）

- **新增 gRPC 客户端**：引入 `user.proto` + `application.proto` 的客户端代码
- **增加 token 管理**：AuthenticateClient 获取 token、缓存、过期前 RenewToken
- **增加流式上传**：把构建产物（推荐 tar.gz）按 chunk 推送到 UploadArtifact
- **增加部署触发与轮询**：Deploy + GetDeployment
- **（可选）并行发布**：对多台 agent 并发执行上面的流程（注意限流与超时）





