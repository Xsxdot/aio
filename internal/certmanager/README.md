# 证书管理器 (CertManager)

该模块提供从Let's Encrypt自动申请SSL证书和自动续期的功能。

## 功能特性

- 自动从Let's Encrypt申请免费SSL证书
- 证书到期前自动续期
- 支持多域名配置
- 支持HTTP-01和DNS-01验证方式
- 支持阿里云DNS验证（可用于申请通配符证书）
- 支持测试环境(staging)和生产环境
- 本地文件存储证书和私钥
- 简单易用的API接口

## 配置项

```go
type Config struct {
    // 是否启用自动证书管理
    Enabled bool `json:"enabled"`
    
    // 证书存储路径
    CertDir string `json:"cert_dir"`
    
    // Let's Encrypt 账户邮箱
    Email string `json:"email"`
    
    // 要申请证书的域名列表
    Domains []string `json:"domains"`
    
    // 证书有效期小于此天数时自动续期
    RenewBefore int `json:"renew_before"`
    
    // 使用测试环境（staging）
    Staging bool `json:"staging"`
    
    // 自动检查证书并续期的间隔
    CheckInterval time.Duration `json:"check_interval"`
    
    // 验证方式：VerifyHTTP 或 VerifyDNS
    VerifyMethod VerifyMethod `json:"verify_method"`
    
    // DNS提供商类型（如：DNSProviderAliyun）
    DNSProvider DNSProviderType `json:"dns_provider"`
    
    // DNS提供商配置
    DNSConfig DNSConfig `json:"dns_config"`
    
    // DNS记录传播等待时间
    DNSPropagationTimeout time.Duration `json:"dns_propagation_timeout"`
}

// DNS配置
type DNSConfig struct {
    // 阿里云AccessKey ID
    AliyunAccessKeyID string `json:"aliyun_access_key_id"`
    
    // 阿里云AccessKey Secret
    AliyunAccessKeySecret string `json:"aliyun_access_key_secret"`
    
    // 阿里云区域
    AliyunRegionID string `json:"aliyun_region_id"`
}
```

## 使用方法

### 使用HTTP-01验证方式（默认）

```go
import (
    "context"
    "github.com/sirupsen/logrus"
    "aio/internal/certmanager"
)

// 创建配置
config := &certmanager.Config{
    Enabled:       true,
    CertDir:       "./certs",
    Email:         "your@email.com",
    Domains:       []string{"example.com", "www.example.com"},
    RenewBefore:   30, // 有效期小于30天时自动续期
    Staging:       false, // 生产环境
    CheckInterval: 24 * time.Hour, // 每天检查一次
    VerifyMethod:  certmanager.VerifyHTTP, // 使用HTTP-01验证
}

// 创建日志器
logger := logrus.New()

// 创建服务
service := certmanager.NewService(config, logger)

// 启动服务
ctx := context.Background()
if err := service.Start(ctx); err != nil {
    logger.Fatalf("启动证书管理服务失败: %v", err)
}

// 获取证书
cert, err := service.GetCertificate("example.com")
if err != nil {
    logger.Errorf("获取证书失败: %v", err)
} else {
    logger.Infof("证书文件: %s", cert.CertFile)
    logger.Infof("私钥文件: %s", cert.KeyFile)
}

// 停止服务
if err := service.Stop(); err != nil {
    logger.Errorf("停止服务失败: %v", err)
}
```

### 使用阿里云DNS验证方式（支持通配符证书）

```go
// 创建配置
config := &certmanager.Config{
    Enabled:       true,
    CertDir:       "./certs",
    Email:         "your@email.com",
    Domains:       []string{"*.example.com", "example.com"}, // 通配符证书
    RenewBefore:   30,
    Staging:       false,
    CheckInterval: 24 * time.Hour,
    // 使用DNS验证
    VerifyMethod:  certmanager.VerifyDNS,
    DNSProvider:   certmanager.DNSProviderAliyun,
    DNSConfig: certmanager.DNSConfig{
        AliyunAccessKeyID:     "你的阿里云AccessKey ID",
        AliyunAccessKeySecret: "你的阿里云AccessKey Secret",
        AliyunRegionID:        "cn-hangzhou", // 可选
    },
    DNSPropagationTimeout: 120 * time.Second, // DNS记录传播等待时间
}

// 其他代码同上...
```

## 依赖说明

该模块依赖以下外部库：

- github.com/go-acme/lego/v4 - 与Let's Encrypt通信的ACME客户端
- github.com/sirupsen/logrus - 日志库

请确保在项目的go.mod中添加这些依赖：

```
go get github.com/go-acme/lego/v4
go get github.com/sirupsen/logrus
```

## 验证方式说明

### HTTP-01 验证

HTTP-01验证需要服务器的80端口可访问，Let's Encrypt会请求你域名的`/.well-known/acme-challenge/`路径下的特定文件。因此，你需要确保：

1. 域名已正确解析到你的服务器
2. 服务器的80端口可访问（用于验证）
3. 没有防火墙或代理阻止这些请求

**注意**: HTTP-01验证**不支持**通配符证书申请。

### DNS-01 验证

DNS-01验证通过在域名的DNS记录中添加特定的TXT记录来验证域名所有权。使用此方式的优势：

1. 服务器80端口不需要对外开放
2. 可以申请通配符证书（如 *.example.com）
3. 适用于内网服务器或无公网IP的场景

使用阿里云DNS验证时，需要提供有权限管理域名DNS记录的AccessKey。请确保：

1. 域名已添加到阿里云DNS
2. 提供的AccessKey有管理DNS记录的权限
3. 考虑DNS记录的传播时间（默认等待2分钟）

## 通配符证书申请

要申请通配符证书（如 *.example.com），你**必须**使用DNS-01验证方式：

```go
config := &certmanager.Config{
    // ... 其他配置 ...
    Domains:      []string{"*.example.com"},
    VerifyMethod: certmanager.VerifyDNS,
    DNSProvider:  certmanager.DNSProviderAliyun,
    // ... DNS配置 ...
}
```

## 测试环境

在正式使用前，建议先启用`Staging`模式进行测试，以避免触发Let's Encrypt的速率限制：

```go
config.Staging = true
```

测试通过后再切换到生产环境：

```go
config.Staging = false
```

## 命令行工具使用示例

```bash
# HTTP验证方式申请证书
./certmanager -email your@email.com -domain example.com -verify http-01

# 阿里云DNS验证方式申请通配符证书
./certmanager -email your@email.com -domain "*.example.com" \
  -verify dns-01 -dns-provider aliyun \
  -aliyun-key-id YOUR_KEY_ID -aliyun-key-secret YOUR_KEY_SECRET
```

## 注意事项

- 证书有效期为90天，模块会在到期前自动续期
- Let's Encrypt有速率限制，请不要频繁请求生产环境证书
- 通配符证书只能使用DNS验证方式申请
- 请妥善保管证书私钥和阿里云AccessKey
- 请确保证书存储目录有适当的权限

## 示例程序

在`example`目录下有一个完整的示例程序，展示了如何使用此模块。 