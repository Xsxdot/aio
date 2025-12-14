# 精简宝塔（多机协同集群版）总体设计说明书

## 1. 背景与目标

本项目目标是做成 **精简版宝塔 + 配置中心 + 证书中心 + 服务注册中心** 的统一控制面，并支持 **多机协同管理**（集群视角）与 **单机精细化管理**（节点视角）。

你已确认的约束与优先级（本说明书以此为准）：

- **运行载体**：优先支持 **Systemd**（应用以进程/服务方式运行）。
- **节点连通**：优先走 **Agent**（而非 SSH 执行）。
- **服务发现**：Nginx 上游由 **控制面渲染静态 upstream 并下发**（非 Nginx 动态拉 registry）。
- **多租户**：先 **单租户**（但保留未来扩展位）。
- **证书**：主要走 **ACME（Let’s Encrypt）+ 部署**。
- **应用类型**：优先落地 **反代到后端**、**本机进程** 两类。

## 2. 形态与边界

### 2.1 控制面（Control Plane：本项目）

- **负责**：Application/Ingress/Service/Release 的声明式模型；配置/证书/注册中心的编排；版本化、审计、权限；调度下发任务；状态汇聚与展示。
- **不负责**：在目标机器上直接执行文件写入、reload、systemd 操作等重动作（统一由 Agent 执行）。

### 2.2 执行面（Data Plane：Node Agent）

每台受管机器运行一个 **Agent**（后续组件），职责：

- 拉取/接收控制面下发的 **期望状态（desired）** 或 **任务（task）**。
- 在节点上执行：写文件、校验、reload Nginx、systemd start/stop/restart、探活与上报。
- 回报节点能力（是否有 nginx/systemd 等）、心跳、任务执行结果、应用实际状态（actual）。

### 2.3 与宝塔的关键差异（设计原则）

- **集群优先**：一个应用可部署到多节点；证书/配置/nginx 入口可多机一致。
- **声明式 + 收敛**：控制面保存期望状态，Agent 负责收敛并回报实际状态。
- **可回滚**：每次变更具备版本、diff、发布记录、回滚点。
- **站点概念统一**：宝塔的站点/项目与注册中心的服务统一为 Application（应用）。

## 3. 核心抽象：Application（应用）

### 3.1 为什么需要 Application

你当前仓库已有组件：

- `system/nginx`：站点/反代/配置文件管理能力
- `system/ssl`：证书申请/部署能力
- `system/config`：配置中心能力（版本/历史）
- `system/registry`：服务注册中心能力

但缺少一个“上层统一编排对象”，导致：

- 站点、证书、配置、注册之间缺少强一致的生命周期与版本边界
- 无法自然支持“一个应用跨多机发布、灰度、回滚”

因此引入 **Application** 作为唯一入口，把上述能力都变成“应用的资源（Resource）”。

### 3.2 Application 领域对象（最小集合）

- **Application**
  - 标识：`name`（单租户阶段可唯一），`labels`
  - 类型：`ReverseProxy` / `SystemdService`
  - 部署范围：节点选择器（labels/显式列表）
  - 期望状态：期望版本、期望实例、期望运行状态（Running/Stopped）
- **Ingress（入口/站点）**
  - 域名、路径、HTTPS、重写/限流等（先做最小必需）
  - 上游指向：静态 `upstream`（控制面渲染）
- **Service（服务）**
  - `serviceName`、协议、端口、健康检查、元数据
  - 用于把应用实例“标准化地暴露出来”，并与注册中心联动
- **Release（发布单）**
  - 变更的版本边界：一次从 A -> B 的声明式变更
  - 包含：配置版本、证书版本、nginx 配置版本、systemd unit 版本、目标节点集合、分批策略
- **Endpoint（实例）**
  - `nodeId` + `ip:port` + `weight` + `health` + `version`
  - 由 Agent 上报，控制面用于渲染静态 upstream

## 4. 多机协同：声明式状态与任务模型

### 4.1 状态视图

- **desired（期望）**：用户在控制面配置的目标状态（应用应运行在哪些节点、对外如何暴露、证书/配置版本等）。
- **actual（实际）**：Agent 执行后上报的节点实际状态（进程是否 running、nginx 是否 reload 成功、证书是否已落盘等）。
- 控制面以 **收敛状态** 展示应用健康：`desired` 与 `actual` 的差异（drift）可视化。

### 4.2 任务（Task）建议规范（必须幂等）

控制面下发任务给 Agent，任务必须：

- **幂等**：同一 task 可重复执行，不产生副作用或可安全覆盖。
- **可重试**：网络波动、节点重启后可继续。
- **可审计**：记录输入参数、输出、耗时、traceId。

最小任务类型（MVP）：

- **RenderAndDeployNginx**：写入目标配置文件、`nginx -t` 校验、reload
- **DeployCertificate**：写证书/私钥到目标路径、权限校验、触发 reload（可与上一个合并）
- **ApplyConfigFiles**：把配置中心“渲染后的文件”落盘到指定路径
- **SystemdInstallUnit**：写 unit 文件、`daemon-reload`
- **SystemdControl**：start/stop/restart、并上报状态
- **HealthProbe**：执行探测（HTTP/TCP/进程）并上报

## 5. 组件划分与仓库落地建议（遵循你项目分层规范）

### 5.1 新增两个业务组件（建议）

- **`system/cluster`**：节点、Agent、任务、心跳与能力
- **`system/application`**：Application/Ingress/Release/编排逻辑（核心）

现有组件保持独立：

- `system/nginx`：只管 Nginx 域能力（模板/配置/下发目标等）
- `system/ssl`：只管证书域能力（ACME/导入/部署）
- `system/config`：只管配置域能力（版本/历史/发布）
- `system/registry`：只管注册域能力（Service/Endpoint）

### 5.2 跨组件调用方式（必须）

`system/application` 只通过各组件的 `api/client` 调用：

- 调 `ssl/api/client`：申请证书、获取证书内容、触发部署计划（由 application 统一编排）
- 调 `nginx/api/client`：生成/管理 nginx 配置对象（由 application 决定何时下发）
- 调 `config/api/client`：获取配置版本与渲染结果
- 调 `registry/api/client`：写入 service 定义、写入/读取 endpoints（用于 upstream 渲染）

> 严格遵守：**Controller -> App -> Service -> DAO**，跨组件只能走 client/facade。

## 6. 数据模型（表结构草案，MVP 优先）

> 先单租户：不引入 tenant_id，但所有表预留 `namespace`/`project` 字段以便未来扩展。

### 6.1 cluster 组件

- **cluster_node**
  - `id`
  - `name`
  - `ip`
  - `labels`（json）
  - `agent_status`（online/offline）
  - `capabilities`（json：nginx/systemd/…）
  - `last_heartbeat_at`

- **cluster_task**
  - `id`
  - `node_id`
  - `type`
  - `payload`（json）
  - `status`（pending/running/succeeded/failed/cancelled）
  - `retry_count`
  - `error_message`
  - `started_at/finished_at`
  - `trace_id`

### 6.2 application 组件

- **app_application**
  - `id`
  - `name`（唯一）
  - `type`（ReverseProxy/SystemdService）
  - `labels`（json）
  - `selector`（json：node labels selector/explicit node list）
  - `desired_state`（Running/Stopped）
  - `current_release_id`
  - `created_at/updated_at`

- **app_ingress**
  - `id`
  - `app_id`
  - `domains`（json array）
  - `path_rules`（json：path -> serviceName/port）
  - `tls_enabled`
  - `certificate_id`（来自 ssl 组件的证书对象引用）
  - `nginx_site_name`（与 nginx 组件对象关联）

- **app_service**
  - `id`
  - `app_id`
  - `service_name`（建议默认：`app.<appName>` 或 `<appName>`）
  - `protocol`（http/tcp）
  - `port`
  - `health_check`（json）

- **app_release**
  - `id`
  - `app_id`
  - `version`（自增/语义化均可）
  - `status`（created/approved/running/succeeded/failed/rolled_back）
  - `plan`（json：批次、并发度、节点集合、超时策略）
  - `artifacts`（json：configVersion/certVersion/nginxVersion/unitVersion…）
  - `diff`（json：变更摘要）
  - `created_by`
  - `created_at/updated_at`

### 6.3 registry 组件的联动（建议）

- `registry` 继续维护：
  - service 定义表
  - endpoints 实例表（含 nodeId、version、weight、health）

application 需要的只是“通过 client 读写 service/endpoints”，不直接依赖内部表结构。

## 7. 关键流程（MVP 可落地）

### 7.1 创建应用（ReverseProxy）

- 用户创建 Application（type=ReverseProxy）
- 创建/绑定 Service（serviceName + port）
- 创建 Ingress（domain + path -> serviceName）
- 可选：绑定证书（ACME/导入）

输出：形成 `desired`，但不立即对节点产生动作，直到触发 Release（或自动发布）。

### 7.2 创建应用（SystemdService）

- 用户创建 Application（type=SystemdService）
- 上传/选择 systemd unit 模板 + 环境变量/配置文件映射
- 配置健康检查（HTTP/TCP/进程）
- 选择节点范围（selector 或显式节点）

输出：形成 `desired`，等待 Release 执行下发与启动。

### 7.3 发布（Release 执行：核心链路）

发布是声明式变更的执行边界，建议流程：

1. 生成 Release（写入变更摘要 diff 与 artifacts 引用）
2. 计算目标节点集合（selector -> nodes）
3. 生成节点批次（batch plan）
4. 对每个节点下发任务（按依赖顺序）：
   - ApplyConfigFiles（如需要）
   - SystemdInstallUnit（SystemdService）
   - SystemdControl（start/restart）
   - DeployCertificate（如 Ingress TLS）
   - RenderAndDeployNginx（如 Ingress）
5. 等待每批完成，失败则：
   - 停止后续批次
   - 触发回滚策略（可选：自动/手动）
6. 成功则：
   - 标记 Release succeeded
   - 更新 Application.current_release_id

### 7.4 Nginx upstream 静态渲染策略（你已确认）

控制面渲染 upstream 的输入：

- registry endpoints（来自 Agent 上报/注册中心）
- Ingress path_rules（domain/path -> serviceName）
- 权重/健康：只选取健康实例（或降级策略）

控制面渲染的输出：

- per-app 或 per-site 的 nginx conf（模板化 + 可 diff）
- 下发给目标节点：写文件 -> `nginx -t` -> reload

### 7.5 证书（ACME + 部署）

证书中心建议具备：

- ACME 申请/续期（含 DNS-01/HTTP-01 的策略配置）
- 证书对象版本化（每次续期产生新版本）
- 证书部署目标（哪些节点、哪些路径、关联哪些 ingress）

续期触发动作（自动任务）：

- 续期成功 -> 生成 DeployCertificate 任务（多节点）
- 证书落盘后 -> 触发 RenderAndDeployNginx（或仅 reload）
- 全链路记录审计与告警（失败告警）

## 8. 状态机（建议）

### 8.1 Node 状态

- **online**：心跳正常
- **offline**：超时无心跳
- **cordoned**：隔离（不参与发布/调度）

### 8.2 Release 状态

- **created** ->（可选审批）-> **running** -> **succeeded**
- **running** -> **failed**
- **failed/succeeded** -> **rolled_back**（回滚产生新 release 或标记回滚关系）

### 8.3 任务状态

- **pending** -> **running** -> **succeeded**
- **running** -> **failed**（可重试）
- **pending/running** -> **cancelled**

## 9. 核心 API（建议清单，按组件划分）

> 这里是“对外 HTTP API”视角；内部实现按你仓库规范分层落地（controller/app/service/dao）。

### 9.1 cluster

- **Node**
  - `POST /admin/nodes`：新增节点（生成 agent token）
  - `GET /admin/nodes`：列表/筛选
  - `PUT /admin/nodes/:id`：更新 labels/启停管理
  - `POST /admin/nodes/:id/cordon`、`/uncordon`
- **Agent**
  - `POST /agent/heartbeat`：心跳 + capabilities 上报
  - `POST /agent/tasks/poll`：拉取任务（长轮询/短轮询）
  - `POST /agent/tasks/report`：回报执行结果

### 9.2 application

- **Application**
  - `POST /admin/apps`
  - `GET /admin/apps`
  - `GET /admin/apps/:id`
  - `PUT /admin/apps/:id`
  - `DELETE /admin/apps/:id`
- **Ingress**
  - `POST /admin/apps/:id/ingresses`
  - `PUT /admin/ingresses/:ingressId`
- **Release**
  - `POST /admin/apps/:id/releases`：创建发布单（含 diff）
  - `POST /admin/releases/:releaseId/execute`
  - `POST /admin/releases/:releaseId/rollback`

### 9.3 registry（已有能力补齐用法）

- `GET /api/registry/services/:name/endpoints`：控制面渲染 upstream 时读取
- Agent/控制面写 endpoints：建议通过内部 client 完成（避免对外暴露过多）

### 9.4 ssl（已有能力补齐用法）

- `POST /admin/certificates/acme`：发起签发
- `POST /admin/certificates/:id/deploy`：部署到指定节点/目标（或由 application 编排调用）

### 9.5 nginx（已有能力补齐用法）

- `POST /admin/nginx/sites/render`：渲染预览（diff）
- `POST /admin/nginx/sites/deploy`：下发（由 application 发起）

## 10. 安全、权限、审计（MVP 最小实现）

- **权限码**（示例）：
  - `admin:app:read`、`admin:app:write`
  - `admin:release:execute`、`admin:release:rollback`
  - `admin:node:manage`
  - `admin:ssl:issue`、`admin:ssl:deploy`
  - `admin:nginx:deploy`
- **审计事件**（必须记录）：
  - 应用创建/更新/删除
  - 发布执行/失败/回滚
  - 证书签发/续期/部署
  - nginx 部署与 reload
  - 节点加入/离线/隔离

## 11. MVP 交付清单（按优先级）

### 11.1 第一阶段（能跑起来）

- **cluster**
  - Node 管理（录入 + token）
  - Agent 心跳 + 任务拉取/回报
  - 任务表 + 幂等执行框架
- **application**
  - Application（ReverseProxy/SystemdService）
  - Release：生成、执行（分批）
  - 与 registry 联动：读取 endpoints 渲染静态 upstream
- **nginx**
  - 站点配置渲染（模板）
  - 下发与 reload（通过 Agent 任务）
- **ssl**
  - ACME 签发 + 证书落库
  - 部署（通过 Agent 任务）+ reload

### 11.2 第二阶段（可用性与稳定性）

- 灰度/分批策略增强（并发、失败阈值、超时）
- drift 检测：desired vs actual
- 告警：证书到期、节点离线、发布失败
- 回滚策略：回到上一 release artifacts

## 12. 与现有模块的对接点（最关键的“怎么用”）

- **registry**
  - Agent 上报实例 -> registry 存 endpoints
  - application 执行发布时 -> 从 registry 取 endpoints -> 渲染 nginx upstream -> 下发
- **nginx**
  - nginx 不负责服务发现策略，仅负责“配置对象 + 下发执行”
  - upstream 内容由 application 控制面提供（静态）
- **ssl**
  - ssl 负责 ACME 获取证书与版本管理
  - 部署与 reload 由 application 编排触发（通过 cluster task）
- **config**
  - application 选择 config 版本作为 artifacts
  - 发布执行时把“渲染结果”下发到节点（通过 cluster task）

## 13. 控制面 ↔ Agent 接口契约（MVP 必须明确）

### 13.1 总体原则

- **通信模式**：Agent 主动 `poll`（短轮询/长轮询均可），控制面不要求反向连通。
- **身份认证**：每个 Node 一枚 Agent Token（可滚动），请求必须携带；后续可升级 mTLS。
- **幂等键**：所有任务必须包含 `taskId`，Agent 以 `taskId` 作为幂等键（重复拉取/重复执行可安全处理）。
- **可观测**：所有请求/回报携带 `traceId`（或由控制面生成回传），统一写审计与任务日志。

### 13.2 Agent 心跳（capabilities + runtime 事实）

- **POST `/agent/heartbeat`**
  - **request（示例字段）**
    - `nodeId`
    - `agentVersion`
    - `timestamp`
    - `capabilities`：`{ "systemd": true, "nginx": true, "acme": false, "os": "linux", ... }`
    - `facts`：可选，如 nginx 版本、systemd 版本、机器资源等
  - **response**
    - `serverTime`
    - `nextPollIntervalMs`

### 13.3 任务拉取与回报

- **POST `/agent/tasks/poll`**
  - **request**
    - `nodeId`
    - `maxTasks`（建议 1~5）
    - `runningTaskIds`（用于断点续报/避免重复分配）
  - **response**
    - `tasks[]`：任务列表（见 13.4）

- **POST `/agent/tasks/report`**
  - **request**
    - `nodeId`
    - `taskId`
    - `status`：`running/succeeded/failed`
    - `startedAt/finishedAt`
    - `stdout/stderr`（可裁剪）
    - `artifacts`（可选：生成的文件 hash、版本信息等）
    - `errorMessage`（失败时）

### 13.4 任务 payload 规范（建议 JSON schema 约定）

统一字段：

- `taskId`、`type`、`traceId`、`timeoutSec`
- `idempotencyKey`（可选，默认等于 taskId）
- `payload`（按类型）

任务类型与最小 payload（MVP）：

- **ApplyConfigFiles**
  - `payload.files[]`：`{ "path": "/etc/myapp/app.yaml", "content": "<rendered>", "mode": "0640", "owner": "root", "group": "root" }`
- **SystemdInstallUnit**
  - `payload.unitName`：如 `myapp.service`
  - `payload.unitPath`：建议固定 `/etc/systemd/system/<unitName>`
  - `payload.content`
  - `payload.enable`：bool
- **SystemdControl**
  - `payload.unitName`
  - `payload.action`：`start/stop/restart/status`
- **RenderAndDeployNginx**
  - `payload.siteName`：如 `<appName>`
  - `payload.targetPath`：建议 `/etc/nginx/conf.d/<siteName>.conf`
  - `payload.content`
  - `payload.testCommand`：默认 `nginx -t`
  - `payload.reloadCommand`：默认 `nginx -s reload` 或 `systemctl reload nginx`
- **DeployCertificate**
  - `payload.certPath`：如 `/etc/nginx/certs/<domain>.crt`
  - `payload.keyPath`：如 `/etc/nginx/certs/<domain>.key`
  - `payload.certPem` / `payload.keyPem`
  - `payload.modeOwnerGroup`：建议强制权限

> 注：ACME 签发建议在控制面完成（`system/ssl`），Agent 只负责“部署落盘与 reload”，减少节点环境差异与安全风险。

## 14. Systemd 应用的落地约定（MVP）

### 14.1 应用类型：SystemdService

- **unit 管理策略**
  - 控制面保存 unit 的版本（artifact），发布时下发 unit 文件到目标节点
  - unit 统一放 `/etc/systemd/system/`，并执行 `systemctl daemon-reload`
  - 变更 unit 或配置文件后，默认执行 `systemctl restart <unit>`
- **健康检查**
  - 最小支持：`process`（systemd active）、`tcp`（端口连通）、`http`（URL 200）
  - Agent 上报健康到 registry（或控制面汇总后写 registry）

### 14.2 Endpoint 生成（用于静态 upstream）

两种来源（MVP 先实现 1）：

1. **Agent 上报端口**：应用配置里声明 `listenPort`，Agent 上报 `ip:listenPort` 作为 endpoint（推荐，简单稳定）。
2. **控制面配置静态 endpoints**：用于“反代到外部后端/已有集群”，不依赖 Agent 探测。

## 15. 静态 upstream 渲染规则（MVP）

### 15.1 输入与过滤

- 输入：`registry` 中某个 `serviceName` 的 endpoints
- 过滤：默认只取 `health=healthy` 的实例；当全部不健康时可选：
  - **策略 A**：保留全部实例但降低权重（便于快速恢复）
  - **策略 B**：返回 503（更安全）

### 15.2 渲染稳定性（避免频繁 reload）

- endpoints 排序稳定：按 `nodeId` 或 `ip:port` 排序
- 内容 hash 不变则不下发、不 reload（控制面可做 diff）

## 16. ACME（Let’s Encrypt）在本系统的推荐做法（MVP）

### 16.1 挑战方式选择

- **优先 DNS-01**：更适合集群与多域名，不依赖某台机器暴露 80
- 次选 HTTP-01：要求某个入口节点可对外提供 `/.well-known/acme-challenge/`

### 16.2 续期触发

- `system/ssl` 定时扫描到期窗口（如 30 天内）
- 续期成功生成新版本证书
- 通过 `cluster_task` 下发 DeployCertificate + RenderAndDeployNginx 到所有绑定该证书的节点（分批）

## 17. 里程碑拆解（建议 2~4 周一个里程碑）

### 17.1 M1：Agent 基础连通 + 任务框架（1~2 周）

- Node 注册/发 token
- Agent heartbeat + poll/report
- 任务表与幂等执行框架（至少支持“写文件 + 执行命令 + 回报”）

### 17.2 M2：SystemdService 应用（1~2 周）

- Application CRUD（SystemdService）
- Release 执行：下发 unit + 下发 config + restart
- Agent 上报 endpoint 到 registry（静态 upstream 依赖它）

### 17.3 M3：ReverseProxy + 证书（1~2 周）

- Ingress/Service 建模
- upstream 静态渲染 + nginx 下发 reload
- ACME 证书签发 + 多节点部署 + reload

### 17.4 M4：回滚/灰度与可观测（1~2 周）

- Release 回滚（回到上一 artifacts）
- 分批/失败阈值/超时
- drift 展示与告警（证书到期、节点离线、发布失败）



