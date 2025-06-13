# 证书管理器部署功能

这个证书管理器实现了完整的SSL证书申请、续期和部署功能。

## 功能特性

### 证书管理
- 自动申请SSL证书（支持Let's Encrypt）
- 支持通配符证书（*.example.com）
- 自动证书续期（30天前开始续期）
- 多DNS提供商支持
- 证书格式转换（Nginx、Apache、IIS、JKS等）

### 部署功能（新增）
支持四种部署类型：

1. **本地文件部署**
   - 将证书复制到本地指定路径
   - 支持绝对路径配置
   - 自动创建目录结构

2. **远程服务器部署**
   - 通过SSH部署到远程服务器
   - 支持密码和私钥认证
   - 自动创建远程目录
   - 使用SCP协议传输文件

3. **阿里云CDN部署**
   - 部署证书到阿里云CDN服务
   - 支持通配符证书匹配
   - 指定目标域名配置

4. **阿里云OSS部署**
   - 部署证书到阿里云OSS存储
   - 支持多区域配置
   - 指定存储桶名称

### 自动化特性
- **自动部署触发**：证书申请或续期成功后自动触发部署
- **异步执行**：部署过程不阻塞证书申请流程
- **错误隔离**：单个部署失败不影响其他部署和证书操作
- **状态跟踪**：记录每次部署的状态和错误信息

## 文件结构

```
internal/certmanager/
├── certmanager.go      # 主要证书管理逻辑和部署配置管理
├── deploy.go          # 具体的部署实现（本地、远程、阿里云）
├── deploy_example.md  # 部署功能使用指南和示例
├── README.md          # 本文档
├── acme.go           # ACME客户端实现
├── utils.go          # 工具函数
└── ...               # 其他相关文件
```

## 主要API方法

### 部署配置管理
- `AddDeployConfig(ctx, config)` - 添加部署配置
- `UpdateDeployConfig(ctx, config)` - 更新部署配置
- `DeleteDeployConfig(ctx, configID)` - 删除部署配置
- `GetDeployConfig(configID)` - 获取部署配置
- `ListDeployConfigs()` - 列出所有部署配置
- `ListDeployConfigsByDomain(domain)` - 根据域名列出部署配置

### 部署执行
- `DeployCertificate(ctx, configID)` - 手动执行部署
- `autoDeployAfterRenewal(ctx, domain)` - 自动部署（内部调用）

### 便利方法
- `CreateLocalDeployConfig(...)` - 创建本地部署配置
- `CreateRemoteDeployConfig(...)` - 创建远程部署配置
- `CreateAliyunCDNDeployConfig(...)` - 创建阿里云CDN部署配置
- `CreateAliyunOSSDeployConfig(...)` - 创建阿里云OSS部署配置

详细使用示例请参考 `deploy_example.md` 文件。

## 支持的DNS提供商

目前支持以下DNS提供商：

- 阿里云DNS (alidns/aliyun)
- Cloudflare (cloudflare)
- 腾讯云DNS/DNSPod (dnspod/tencentcloud)
- GoDaddy (godaddy)
- AWS Route53 (route53/aws)
- DigitalOcean (digitalocean/do)
- Namesilo (namesilo)
- 测试用模拟提供商 (mock)

## DNS提供商配置参数

每个DNS提供商都配置了以下默认参数：

| DNS提供商 | TTL (秒) | 传播等待时间 | 轮询间隔 | 备注 |
|----------|---------|------------|---------|------|
| 阿里云DNS | 600 | 15分钟 | 30秒 | - |
| Cloudflare | 120 | 10分钟 | 10秒 | 传播速度较快 |
| 腾讯云DNS/DNSPod | 600 | 15分钟 | 20秒 | - |
| GoDaddy | 600 | 30分钟 | 30秒 | 传播较慢 |
| AWS Route53 | 300 | 15分钟 | 20秒 | 配置了5次API重试 |
| DigitalOcean | 30 | 10分钟 | 15秒 | TTL默认值较低 |
| Namesilo | 7200 | 40分钟 | 60秒 | 传播非常慢，TTL较高 |

这些参数代表：

- **TTL**: DNS记录的生存时间，即DNS缓存的有效期
- **传播等待时间**: 等待DNS记录在全球范围内传播的最长时间
- **轮询间隔**: 检查DNS记录是否已成功传播的间隔时间

## 使用方法

### 环境变量设置

根据你使用的DNS提供商，需要设置相应的环境变量：

- 阿里云DNS
  ```
  ALICLOUD_ACCESS_KEY=your_access_key
  ALICLOUD_SECRET_KEY=your_secret_key
  ```

- Cloudflare
  ```
  CF_API_TOKEN=your_api_token
  # 或者使用API密钥和邮箱
  CF_API_KEY=your_api_key
  CF_API_EMAIL=your_email
  ```

- 腾讯云DNS/DNSPod
  ```
  TENCENTCLOUD_SECRET_ID=your_secret_id
  TENCENTCLOUD_SECRET_KEY=your_secret_key
  ```

- GoDaddy
  ```
  GODADDY_API_KEY=your_api_key
  GODADDY_API_SECRET=your_api_secret
  ```

- AWS Route53
  ```
  AWS_ACCESS_KEY_ID=your_access_key_id
  AWS_SECRET_ACCESS_KEY=your_secret_access_key
  AWS_REGION=us-east-1  # 可选，默认为us-east-1
  ```

- DigitalOcean
  ```
  DO_AUTH_TOKEN=your_auth_token
  ```

- Namesilo
  ```
  NAMESILO_API_KEY=your_api_key
  ```

### 示例程序

使用提供的示例程序申请证书：

```bash
# 设置环境变量
export ALICLOUD_ACCESS_KEY=your_access_key
export ALICLOUD_SECRET_KEY=your_secret_key

# 运行证书申请程序
go run cmd/certdemo/main.go \
  --email your_email@example.com \
  --domain example.com \
  --dns-provider alidns \
  --cert-dir ./certs \
  --staging  # 在测试环境申请，不会消耗真实配额
```

### 在代码中使用

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/xsxdot/aio/internal/certmanager"
    "go.uber.org/zap"
)

func main() {
    // 初始化日志
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    
    certmanager.SetLogger(logger)
    
    // 初始化ACME客户端
    email := "your_email@example.com"
    useStaging := true // 测试环境
    err := certmanager.InitWithEmail(email, useStaging)
    if err != nil {
        log.Fatalf("初始化ACME客户端失败: %v", err)
    }
    
    // DNS提供商配置
    domain := "example.com"
    certDir := "./certs"
    dnsProvider := "alidns"
    credentials := map[string]string{
        "ALICLOUD_ACCESS_KEY": "your_access_key",
        "ALICLOUD_SECRET_KEY": "your_secret_key",
    }
    
    // 设置超时
    ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute) // 注意:应比DNS传播时间更长
    defer cancel()
    
    // 申请证书
    certPath, keyPath, issuedAt, expiresAt, err := certmanager.GetCertificate(
        ctx, domain, certDir, dnsProvider, credentials,
    )
    if err != nil {
        log.Fatalf("申请证书失败: %v", err)
    }
    
    log.Printf("证书申请成功! 路径: %s, %s", certPath, keyPath)
    log.Printf("有效期: %s 至 %s", issuedAt, expiresAt)
}
```

## DNS记录传播和验证过程

Let's Encrypt在验证您的域名所有权时，会查询您添加的DNS TXT记录。DNS记录传播需要时间，这个时间因DNS提供商而异:

1. 系统会在您的DNS服务商添加一条特定的TXT记录
2. 系统会等待这个记录在全球DNS系统中传播（等待时间基于上述配置）
3. Let's Encrypt会验证这条记录是否存在
4. 验证成功后，会颁发证书

## 故障排除

如果证书申请失败，可能有以下原因：

1. **DNS记录传播时间不足**: 某些DNS提供商需要更长的传播时间。如果遇到这种情况，可以考虑增加`PropagationTimeout`参数。

2. **API凭证不正确**: 确认您的API密钥和令牌有足够的权限来创建和删除DNS记录。

3. **网络问题**: 如果遇到网络连接超时，可能需要配置HTTP代理或修改超时设置。

4. **域名配置问题**: 确保该域名确实由您配置的DNS提供商管理。

## 注意事项

1. 请确保你有权限管理域名的DNS记录
2. 在生产环境使用时，请关闭staging模式
3. Let's Encrypt有速率限制，请合理使用
4. 证书有效期通常为90天，建议在到期前至少30天续期
5. 不同DNS提供商的传播时间差异很大，配置的等待时间可能需要根据实际情况调整 