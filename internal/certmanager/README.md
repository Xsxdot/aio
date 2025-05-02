# 证书管理器

这个包提供了自动申请和管理Let's Encrypt证书的功能，支持使用DNS验证方式来申请证书。

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