# 单机优先（Local-First）MVP 方案：先做“精简宝塔”，后续平滑升级多机

## 0. 这份文档解决什么问题

你觉得之前“多机/Agent/任务”太复杂——完全正常。**第一阶段我们先只做本机**：把它当成“精简宝塔”，能在一台机器上完成：

- 管理 **Systemd 应用**（启动/停止/重启/状态）
- 管理 **Nginx 反代站点**（生成配置、校验、reload）
- 管理 **ACME 证书**（签发/续期、绑定域名到站点）
- 管理 **配置版本**（一键发布到本机路径）

等单机 MVP 验证跑通后，再把“执行能力”从本机抽象成 Agent，即可升级多机。

---

## 1. 单机版的目标范围（只保留最小闭环）

### 1.1 我们先实现的两个“应用类型”

1) **SystemdService（本机进程）**

- 你提供 unit 模板（或填写关键字段），平台负责落盘、daemon-reload、start/restart、探活

2) **ReverseProxy（反代到后端）**

- 你提供 upstream（本机端口/外部地址），平台负责生成站点配置并 reload

### 1.2 单机版暂时不做（全部延后）

- 多机节点管理、Agent 心跳、任务队列
- 注册中心 endpoints 自动发现（可以先手动填 upstream）
- 灰度/分批发布（单机只要“可回滚”即可）

---

## 2. 单机版的核心对象（你只需要记住 4 个）

### 2.1 Application（应用）

- 代表一个“网站/项目/服务”
- 两种类型：`SystemdService` / `ReverseProxy`

### 2.2 ConfigVersion（配置版本）

- 来自 `system/config` 的版本能力
- 本机发布时把渲染后的文件写到目标路径（如 `/etc/myapp/app.yaml`）

### 2.3 Site（站点 / Ingress）

- 来自 `system/nginx` 的站点概念
- 绑定域名、路径规则、upstream、TLS

### 2.4 Certificate（证书）

- 来自 `system/ssl` 的证书概念
- 通过 ACME 签发，写到固定路径，并与站点绑定后触发 reload

---

## 3. 单机版的“最小用户流程”（按宝塔使用习惯）

### 3.1 创建并发布一个 Systemd 应用（本机进程）

1. 创建应用：填写 `appName`、选择类型 `SystemdService`
2. 配置：选择配置版本（或创建一版配置）
3. Unit：填写/上传 unit 内容（或选择模板 + 变量）
4. 发布（本机执行，顺序固定）：
   - 写配置文件（来自 config 渲染结果）
   - 写 unit 到 `/etc/systemd/system/<appName>.service`
   - `systemctl daemon-reload`
   - `systemctl enable <unit>`（可选）
   - `systemctl restart <unit>`
   - 记录状态与日志摘要

### 3.2 创建一个反代站点（域名 -> upstream）

1. 创建应用：填写 `appName`、选择类型 `ReverseProxy`
2. 创建站点：绑定域名（如 `api.example.com`）
3. 设置 upstream：
   - MVP 先支持两种：`127.0.0.1:<port>` 或 `ip:port`
4. 生成并下发 nginx 配置：
   - 写 `/etc/nginx/conf.d/<appName>.conf`
   - `nginx -t`
   - `systemctl reload nginx`（或 `nginx -s reload`）

### 3.3 申请证书并启用 HTTPS（ACME）

1. 为域名申请证书（ACME，建议 DNS-01 优先；HTTP-01 也可）
2. 证书签发成功后：
   - 写证书到 `/etc/nginx/certs/<domain>.crt`、`/etc/nginx/certs/<domain>.key`
   - 更新站点 TLS 配置引用该路径
   - `nginx -t` + reload

---

## 4. 单机版的实现方式（最简单、最贴近你现有代码）

### 4.1 不引入 Agent/任务：直接在控制面执行本机命令

单机 MVP 允许“控制面就是本机”，因此：

- controller/app/service 内部可以调用一个 `pkg/localexec`（建议新增）来执行命令：
  - `systemctl ...`
  - `nginx -t` / reload
  - 写文件（通过 Go 写入即可，不必 shell）

> 注意：这是单机阶段的“特权捷径”。后续升级多机时，把 `localexec` 替换成 `agentclient` 即可。

### 4.2 目录结构建议（最小新增）

新增一个组件（建议）：

- `system/application`：只做“把 nginx/ssl/config/systemd 串起来”的编排层（单机版不需要 cluster）

它通过各组件 `api/client` 调用：

- `config/api/client`：取配置版本、渲染结果
- `ssl/api/client`：申请证书、取证书内容
- `nginx/api/client`：生成站点配置内容（或直接由 application 渲染也行）
- `registry`：单机阶段可不接入

---

## 5. 单机版的数据与版本（MVP 建议）

### 5.1 最小表（建议放在 `system/application`）

- `app_application`
  - `id`, `name`, `type`, `created_at`, `updated_at`
- `app_release`
  - `id`, `app_id`, `version`, `status`, `diff`, `created_at`

> nginx/config/ssl 各自的版本/历史继续用现有表，application 只做“引用与编排”。

### 5.2 回滚怎么做（单机也必须有）

- 所有发布都生成 `Release`，保存：
  - unit 内容版本
  - nginx conf 内容版本
  - 配置文件内容版本（或 config version id）
  - 证书版本 id（可选）
- 回滚就是“把上一版内容重新写回 + reload/restart”

---

## 6. 先做什么（按 1 周内能见到效果的顺序）

### 6.1 第 1 步：Systemd 管理（最直观）

- 应用 CRUD（SystemdService）
- 发布：写 unit + restart + 状态查询

### 6.2 第 2 步：Nginx 站点管理

- 反代站点 CRUD
- 生成配置、`nginx -t`、reload

### 6.3 第 3 步：ACME 证书 + 绑定站点

- ACME 签发/续期（复用 `system/ssl`）
- 部署到固定路径 + 更新站点配置 + reload

### 6.4 第 4 步：配置版本发布到本机

- 从 `system/config` 选择版本
- 发布时写入目标路径（可多文件）

---

## 7. 未来如何从单机升级到多机（不推翻重来）

关键点：把“执行方式”抽象成接口。

### 7.1 抽象一个执行器接口（建议）

- 单机：`LocalExecutor`（本机执行命令/写文件）
- 多机：`AgentExecutor`（把同样的 payload 发送给 Agent）

只要 application 的编排逻辑不直接写 `systemctl/nginx`，而是调用 executor：

- 单机阶段：executor 直接执行
- 多机阶段：executor 变成下发任务 + 等待回报

### 7.2 升级时新增什么

- 新增 `system/cluster`（node/agent/task）
- 把 `LocalExecutor` 替换为 `AgentExecutor`
- Nginx upstream 从“手动填”升级为“从 registry endpoints 渲染静态 upstream”

---

## 8. 你现在最需要做的选择（单机 MVP 的 3 个默认值）

如果你不想纠结，我建议直接默认：

- **nginx 配置路径**：`/etc/nginx/conf.d/<appName>.conf`
- **证书路径**：`/etc/nginx/certs/<domain>.crt`、`/etc/nginx/certs/<domain>.key`
- **unit 路径**：`/etc/systemd/system/<appName>.service`





