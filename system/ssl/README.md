# SSL 证书组件

基于 Let's Encrypt (ACME) 的 SSL 证书自动申请、续期与部署组件。

## 功能特性

### 核心功能
- **证书签发**：通过 Let's Encrypt 使用 DNS-01 验证方式自动申请证书
- **自动续期**：定时扫描即将过期的证书并自动续期（默认提前 30 天）
- **自动部署**：证书签发/续期后自动部署到目标环境
- **多种部署方式**：支持本机文件、SSH 远端、阿里云 CAS

### DNS Provider 支持
- **阿里云 DNS** (`alidns`)
- **腾讯云 DNS** (`tencentcloud`)
- **DNSPod** (`dnspod`，自动映射到 `tencentcloud`)

### 证书类型支持
- 单域名证书（example.com）
- 泛域名证书（*.example.com）

## 组件结构

```
system/ssl/
├── module.go                      # 组件对外门面
├── router.go                      # 路由注册
├── migrate.go                     # 数据库迁移
├── api/
│   ├── client/                    # 对外客户端（供其他组件调用）
│   └── dto/                       # 对外数据传输对象
├── internal/
│   ├── model/                     # 数据模型
│   │   ├── types.go              # 枚举类型定义
│   │   ├── dns_credential.go     # DNS 凭证模型
│   │   ├── certificate.go        # 证书模型
│   │   ├── deploy_target.go      # 部署目标模型
│   │   └── deploy_history.go     # 部署历史模型
│   ├── dao/                       # 数据访问层
│   ├── service/                   # 业务逻辑层
│   │   ├── acme_service.go       # ACME 证书申请服务
│   │   ├── crypto_service.go     # 加密服务
│   │   └── deploy_service.go     # 部署服务
│   └── app/                       # 应用编排层
│       ├── certificate_manage.go # 证书管理
│       └── deploy_manage.go      # 部署管理
└── external/
    └── http/                      # HTTP 控制器
        └── ssl_controller.go      # 后台管理接口
```

## 使用指南

### 1. DNS 凭证管理

#### 创建 DNS 凭证
```http
POST /admin/ssl/dns-credentials
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "阿里云 DNS 凭证",
  "provider": "alidns",
  "access_key": "your_access_key",
  "secret_key": "your_secret_key",
  "description": "用于 example.com 域名验证"
}
```

#### 查询 DNS 凭证列表
```http
GET /admin/ssl/dns-credentials?page=1&page_size=20
Authorization: Bearer {admin_token}
```

### 2. 部署目标管理

#### 创建本机文件部署目标
```http
POST /admin/ssl/deploy-targets
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "本机 Nginx",
  "type": "local",
  "config": {
    "base_path": "/etc/nginx/certs",
    "create_subdir": true,
    "fullchain_name": "fullchain.pem",
    "privkey_name": "privkey.pem",
    "file_mode": "0600",
    "reload_command": "nginx -s reload"
  },
  "description": "本机 Nginx 证书部署"
}
```

#### 创建 SSH 远端部署目标
```http
POST /admin/ssl/deploy-targets
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "远程服务器 Nginx",
  "type": "ssh",
  "config": {
    "host": "192.168.1.100",
    "port": 22,
    "username": "root",
    "auth_method": "privatekey",
    "private_key": "-----BEGIN RSA PRIVATE KEY-----\n...",
    "remote_path": "/etc/nginx/certs",
    "create_subdir": true,
    "fullchain_name": "fullchain.pem",
    "privkey_name": "privkey.pem",
    "file_mode": "0600",
    "reload_command": "systemctl reload nginx"
  },
  "description": "远程服务器证书部署"
}
```

#### 创建阿里云 CAS 部署目标
```http
POST /admin/ssl/deploy-targets
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "阿里云证书服务",
  "type": "aliyun_cas",
  "config": {
    "access_key_id": "your_access_key_id",
    "access_key_secret": "your_access_key_secret",
    "region": "cn-hangzhou",
    "cert_name": "example.com"
  },
  "description": "上传到阿里云 CAS"
}
```

### 3. 证书管理

#### 申请证书
```http
POST /admin/ssl/certificates
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "example.com 证书",
  "domain": "example.com",
  "email": "admin@example.com",
  "dns_credential_id": 1,
  "renew_before_days": 30,
  "auto_renew": true,
  "auto_deploy": true,
  "description": "示例域名证书",
  "use_staging": false
}
```

#### 查询证书列表
```http
GET /admin/ssl/certificates?page=1&page_size=20
Authorization: Bearer {admin_token}
```

#### 手动续期证书
```http
POST /admin/ssl/certificates/{id}/renew
Authorization: Bearer {admin_token}
```

#### 手动部署证书
```http
POST /admin/ssl/certificates/{id}/deploy
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "target_ids": [1, 2]
}
```

#### 查看部署历史
```http
GET /admin/ssl/certificates/{id}/deploy-history?limit=20
Authorization: Bearer {admin_token}
```

## 自动续期任务

组件会在应用启动时自动注册定时任务：
- **执行时间**：每天凌晨 2:30
- **执行模式**：分布式（需要获取锁）
- **续期策略**：证书过期前 N 天自动续期（N 为证书配置的 `renew_before_days`，默认 30 天）

## 数据模型

### 证书状态 (CertificateStatus)
- `pending`：待签发
- `issuing`：签发中
- `active`：已签发有效
- `renewing`：续期中
- `expired`：已过期
- `failed`：签发失败

### 部署状态 (DeployStatus)
- `pending`：待部署
- `deploying`：部署中
- `success`：部署成功
- `failed`：部署失败
- `partial`：部分成功

## 安全性

- **敏感字段加密**：DNS AK/SK、SSH 私钥、阿里云凭证等敏感信息使用 AES-GCM 加密存储
- **HTTPS Only**：所有 API 需要通过 HTTPS 访问
- **权限控制**：所有接口需要管理员权限认证

## 依赖

- `github.com/go-acme/lego/v4`：ACME 协议实现
- `github.com/pkg/sftp`：SFTP 客户端
- `github.com/alibabacloud-go/cas-20200407/v2`：阿里云证书服务 SDK
- `golang.org/x/crypto/ssh`：SSH 客户端

## 注意事项

1. **DNS 传播时间**：DNS-01 验证需要等待 DNS 记录传播，通常需要几分钟
2. **Let's Encrypt 限制**：
   - 每个域名每周最多签发 50 个证书
   - 建议使用 `use_staging=true` 进行测试
3. **证书续期**：建议至少提前 30 天续期，避免证书过期
4. **SSH 部署**：确保 SSH 用户有目标目录的写权限
5. **Nginx 重载**：部署后自动重载 Nginx 需要相应的执行权限

## 故障排查

### 证书签发失败
1. 检查 DNS 凭证是否正确
2. 检查 DNS Provider 的 API 访问权限
3. 查看 `last_error` 字段的错误信息
4. 使用测试环境 (`use_staging=true`) 调试

### 部署失败
1. 检查部署目标配置是否正确
2. 检查目标路径的写权限
3. 检查 SSH 连接是否正常
4. 查看部署历史记录的错误信息

### 自动续期不工作
1. 检查调度器是否正常运行
2. 检查证书的 `auto_renew` 字段是否为 1
3. 检查证书状态是否为 `active`
4. 检查证书过期时间距离当前时间是否小于 `renew_before_days`
