# 前端修改指南：SSL 证书单域名 + 部署目标按域名绑定

本文档用于指导前端适配后端 SSL 组件的接口与交互变更（无旧数据、无需兼容迁移）。

---

## 1. 背景与目标

### 1.1 证书模型变更

- 证书从「多域名」改为「单域名」：每个证书对象只包含一个 `domain`（字符串），支持通配符如 `*.a.com`。

### 1.2 部署目标变更

- 部署目标 `DeployTarget` 的配置与域名强绑定：创建部署目标时必须指定 `domain`。
- 当证书 `domain="*.a.com"` 续期/签发完成后，需要对多个部署目标生效（`*.a.com` 和 `b.a.com` 等）。

### 1.3 部署目录变更

- 不再支持「按域名创建子目录」：部署配置不再使用 `create_subdir`。

---

## 2. Breaking Changes（必须改）

### 2.1 证书：`domains` -> `domain`

- **请求字段**（签发证书）：
  - 旧：`domains: string[]`
  - 新：`domain: string`（必填）
- **响应字段**（证书列表/详情）：
  - 旧：`domains: string[]`
  - 新：`domain: string`

### 2.2 部署目标：新增 `domain` 必填

- **创建部署目标**：新增 `domain: string`（必填，支持 `b.a.com` 或 `*.a.com`）
- **更新部署目标**：新增 `domain: string`（可选更新）

### 2.3 移除 `create_subdir`

- 本地/SSH 部署配置里 **不要再传** `create_subdir`。
- 后端部署将直接使用配置中的路径，不再按域名拼接子目录。

---

## 3. 接口字段对照表

### 3.1 证书 DTO（前端 TypeScript 建议）

建议将前端 DTO 从：

```ts
type CertificateDTO_Old = {
  id: number
  name: string
  domains: string[]
  email: string
  status: string
  expiresAt: string
  issuedAt: string
  autoRenew: boolean
  autoDeploy: boolean
  renewBeforeDays: number
  description: string
}
```

改为：

```ts
type CertificateDTO = {
  id: number
  name: string
  domain: string
  email: string
  status: string
  expiresAt: string
  issuedAt: string
  autoRenew: boolean
  autoDeploy: boolean
  renewBeforeDays: number
  description: string
}
```

### 3.2 签发证书请求

旧：

```ts
type IssueCertificateReq_Old = {
  name?: string
  domains: string[]
  email: string
  dnsCredentialId: number
  renewBeforeDays?: number
  autoRenew?: boolean
  autoDeploy?: boolean
  deployTargetIds?: number[]
  description?: string
  useStaging?: boolean
}
```

新：

```ts
type IssueCertificateReq = {
  name?: string
  domain: string
  email: string
  dnsCredentialId: number
  renewBeforeDays?: number
  autoRenew?: boolean
  autoDeploy?: boolean
  deployTargetIds?: number[]
  description?: string
  useStaging?: boolean
}
```

> 注意：后端 HTTP 接口字段多为 `snake_case`（如 `dns_credential_id`），若你们前端使用 `camelCase`，请继续沿用你们的序列化规则/拦截器进行转换。

---

## 4. HTTP 接口示例（snake_case）

以下路径以后台管理路由为例（`/ssl` 组）。

### 4.1 签发证书：`POST /ssl/certificates`

请求：

```json
{
  "name": "my-cert",
  "domain": "*.a.com",
  "email": "admin@a.com",
  "dns_credential_id": 1,
  "renew_before_days": 30,
  "auto_renew": true,
  "auto_deploy": true,
  "deploy_target_ids": [],
  "description": "",
  "use_staging": false
}
```

说明：

- `deploy_target_ids`：
  - **非空**：按你指定的目标列表部署
  - **为空**：后端会按 `certificate.domain` 自动匹配部署目标（见「第 6 节」）

### 4.2 续期证书：`POST /ssl/certificates/:id/renew`

请求体：无（按当前证书记录续期）

### 4.3 创建部署目标：`POST /ssl/deploy-targets`

请求：

```json
{
  "name": "nginx-ssh-b.a.com",
  "domain": "b.a.com",
  "type": "ssh",
  "config": "{...}",
  "description": ""
}
```

### 4.4 更新部署目标：`PUT /ssl/deploy-targets/:id`

请求（示例：更新绑定域名）：

```json
{
  "domain": "*.a.com"
}
```

---

## 5. UI/交互改造建议（前端）

### 5.1 证书管理页面

- 列表/详情：
  - 展示 `domain`（单域名字符串）
  - 移除/隐藏域名数组展示逻辑
- 签发/编辑表单：
  - 将“域名列表”改为“单域名输入框”
  - 文案提示：支持 `*.a.com`

### 5.2 部署目标管理页面

- 创建/编辑表单：
  - 新增必填输入：**绑定域名/通配符**（`domain`）
  - 配置项仍按 `type` 展示对应字段（local/ssh/aliyun_cas）

---

## 6. 自动部署命中规则（重要）

当证书续期/签发成功且 `auto_deploy=true` 时，后端会根据 `certificate.domain` 自动匹配部署目标：

### 6.1 证书为精确域名（如 `b.a.com`）

- 仅命中：`DeployTarget.domain == "b.a.com"`

### 6.2 证书为通配符域名（如 `*.a.com`）

- 命中：
  - `DeployTarget.domain == "*.a.com"`
  - `DeployTarget.domain == "b.a.com"`（单层子域）
- 不命中：
  - `DeployTarget.domain == "a.com"`（根域）
  - `DeployTarget.domain == "x.b.a.com"`（多层子域）

---

## 7. 验收 Checklist

- [ ] 证书签发请求已改为 `domain`（不再发送 `domains`）
- [ ] 证书列表/详情已读取并展示 `domain`
- [ ] 创建部署目标时必须填写 `domain`
- [ ] 部署配置不再包含 `create_subdir`
- [ ] 用 `domain="*.a.com"` 的证书触发续期后，能看到 `*.a.com` 与 `b.a.com` 的部署目标都产生部署记录



