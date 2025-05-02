package certmanager

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/digitalocean"
	"github.com/go-acme/lego/v4/providers/dns/dnspod"
	"github.com/go-acme/lego/v4/providers/dns/godaddy"
	"github.com/go-acme/lego/v4/providers/dns/namesilo"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"go.uber.org/zap"
)

// 注册DNS提供商工厂函数
func init() {
	// 设置DNS提供商工厂函数
	CustomDNSProviderFactory = createDNSProvider
}

// createDNSProvider 创建DNS提供商实例
func createDNSProvider(providerName string, credentials map[string]string) (SimpleDNSProvider, error) {
	switch strings.ToLower(providerName) {
	case "mock":
		// 测试用的模拟DNS提供商
		return NewMockDNSProvider(), nil
	case "aliyun", "alidns":
		// 阿里云DNS提供商
		apiKey, ok1 := credentials["ALICLOUD_ACCESS_KEY"]
		secretKey, ok2 := credentials["ALICLOUD_SECRET_KEY"]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("阿里云DNS需要ALICLOUD_ACCESS_KEY和ALICLOUD_SECRET_KEY凭证")
		}
		return NewAliDNSProvider(apiKey, secretKey)
	case "cloudflare":
		// Cloudflare DNS提供商
		apiToken, ok := credentials["CF_API_TOKEN"]
		if !ok {
			apiKey, ok1 := credentials["CF_API_KEY"]
			email, ok2 := credentials["CF_API_EMAIL"]
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("Cloudflare DNS需要CF_API_TOKEN或(CF_API_KEY和CF_API_EMAIL)凭证")
			}
			return NewCloudflareProvider(email, apiKey)
		}
		return NewCloudflareTokenProvider(apiToken)
	case "dnspod", "tencentcloud":
		// DNSPod/腾讯云DNS提供商
		secretId, ok1 := credentials["TENCENTCLOUD_SECRET_ID"]
		secretKey, ok2 := credentials["TENCENTCLOUD_SECRET_KEY"]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("腾讯云DNS需要TENCENTCLOUD_SECRET_ID和TENCENTCLOUD_SECRET_KEY凭证")
		}
		return NewDNSPodProvider(secretId, secretKey)
	case "godaddy":
		// GoDaddy DNS提供商
		apiKey, ok1 := credentials["GODADDY_API_KEY"]
		apiSecret, ok2 := credentials["GODADDY_API_SECRET"]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("GoDaddy DNS需要GODADDY_API_KEY和GODADDY_API_SECRET凭证")
		}
		return NewGoDaddyProvider(apiKey, apiSecret)
	case "aws", "route53":
		// AWS Route53 DNS提供商
		accessKey, ok1 := credentials["AWS_ACCESS_KEY_ID"]
		secretKey, ok2 := credentials["AWS_SECRET_ACCESS_KEY"]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("AWS Route53需要AWS_ACCESS_KEY_ID和AWS_SECRET_ACCESS_KEY凭证")
		}
		region := credentials["AWS_REGION"]
		if region == "" {
			region = "us-east-1" // 默认区域
		}
		return NewRoute53Provider(accessKey, secretKey, region)
	case "digitalocean", "do":
		// DigitalOcean DNS提供商
		token, ok := credentials["DO_AUTH_TOKEN"]
		if !ok {
			return nil, fmt.Errorf("DigitalOcean DNS需要DO_AUTH_TOKEN凭证")
		}
		return NewDigitalOceanProvider(token)
	case "namesilo":
		// Namesilo DNS提供商
		apiKey, ok := credentials["NAMESILO_API_KEY"]
		if !ok {
			return nil, fmt.Errorf("Namesilo DNS需要NAMESILO_API_KEY凭证")
		}
		return NewNamesiloProvider(apiKey)
	default:
		return nil, fmt.Errorf("不支持的DNS提供商: %s", providerName)
	}
}

// ====================================================================================
// DNS提供商的通用包装器，用于将lego的DNS提供商转换为我们的SimpleDNSProvider接口
// ====================================================================================

// LegoDNSProviderWrapper 用于包装lego库的DNS提供商
type LegoDNSProviderWrapper struct {
	provider challenge.Provider
	logger   *zap.Logger
}

// Present 添加DNS TXT记录
func (w *LegoDNSProviderWrapper) Present(domain, token, keyAuth string) error {
	if w.logger != nil {
		w.logger.Info("添加DNS记录",
			zap.String("domain", domain),
			zap.String("token", token))
	}
	return w.provider.Present(domain, token, keyAuth)
}

// CleanUp 清理DNS TXT记录
func (w *LegoDNSProviderWrapper) CleanUp(domain, token, keyAuth string) error {
	if w.logger != nil {
		w.logger.Info("清理DNS记录",
			zap.String("domain", domain),
			zap.String("token", token))
	}
	return w.provider.CleanUp(domain, token, keyAuth)
}

// ====================================================================================
// 以下是不同DNS提供商的实现
// ====================================================================================

// MockDNSProvider 测试用的模拟DNS提供商
type MockDNSProvider struct {
	logger *zap.Logger
}

// NewMockDNSProvider 创建模拟DNS提供商
func NewMockDNSProvider() *MockDNSProvider {
	return &MockDNSProvider{
		logger: logger,
	}
}

// Present 添加DNS TXT记录
func (p *MockDNSProvider) Present(domain, token, keyAuth string) error {
	// 模拟添加DNS记录
	if p.logger != nil {
		p.logger.Info("MockDNSProvider: 模拟添加DNS记录",
			zap.String("domain", domain),
			zap.String("token", token))
	}

	// 模拟DNS传播延迟
	time.Sleep(100 * time.Millisecond)
	return nil
}

// CleanUp 清理DNS TXT记录
func (p *MockDNSProvider) CleanUp(domain, token, keyAuth string) error {
	// 模拟清理DNS记录
	if p.logger != nil {
		p.logger.Info("MockDNSProvider: 模拟清理DNS记录",
			zap.String("domain", domain),
			zap.String("token", token))
	}
	return nil
}

// ====================================================================================

// AliDNSProvider 阿里云DNS提供商
type AliDNSProvider struct {
	*LegoDNSProviderWrapper
}

// NewAliDNSProvider 创建阿里云DNS提供商
func NewAliDNSProvider(accessKey, secretKey string) (*AliDNSProvider, error) {
	config := alidns.Config{
		APIKey:    accessKey,
		SecretKey: secretKey,
		TTL:       600, // 默认TTL为600秒
		// 注意：部分提供商不支持设置HTTPClient
	}

	provider, err := alidns.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云DNS提供商失败: %w", err)
	}

	return &AliDNSProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// ====================================================================================

// CloudflareProvider Cloudflare DNS提供商
type CloudflareProvider struct {
	*LegoDNSProviderWrapper
}

// NewCloudflareProvider 创建Cloudflare DNS提供商（使用API密钥）
func NewCloudflareProvider(email, apiKey string) (*CloudflareProvider, error) {
	config := cloudflare.Config{
		AuthEmail: email,
		AuthKey:   apiKey,
		TTL:       120, // 默认TTL为120秒
		// HTTP客户端超时设置，部分提供商支持
	}

	provider, err := cloudflare.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建Cloudflare DNS提供商失败: %w", err)
	}

	return &CloudflareProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// NewCloudflareTokenProvider 创建Cloudflare DNS提供商（使用API令牌）
func NewCloudflareTokenProvider(token string) (*CloudflareProvider, error) {
	config := cloudflare.Config{
		AuthToken:          token,
		TTL:                120,              // 默认TTL为120秒
		PropagationTimeout: time.Minute * 10, // 等待DNS记录传播的超时时间
		PollingInterval:    time.Second * 10, // 检查DNS记录传播的轮询间隔
		// HTTP客户端超时设置，部分提供商支持
	}

	provider, err := cloudflare.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建Cloudflare DNS提供商失败: %w", err)
	}

	return &CloudflareProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// ====================================================================================

// DNSPodProvider DNSPod/腾讯云DNS提供商
type DNSPodProvider struct {
	*LegoDNSProviderWrapper
}

// NewDNSPodProvider 创建DNSPod/腾讯云DNS提供商
func NewDNSPodProvider(secretId, secretKey string) (*DNSPodProvider, error) {
	config := dnspod.Config{
		LoginToken: fmt.Sprintf("%s,%s", secretId, secretKey),
		TTL:        600, // 默认TTL为600秒
		// HTTP客户端超时设置，部分提供商支持
	}

	provider, err := dnspod.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建DNSPod DNS提供商失败: %w", err)
	}

	return &DNSPodProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// ====================================================================================

// GoDaddyProvider GoDaddy DNS提供商
type GoDaddyProvider struct {
	*LegoDNSProviderWrapper
}

// NewGoDaddyProvider 创建GoDaddy DNS提供商
func NewGoDaddyProvider(apiKey, apiSecret string) (*GoDaddyProvider, error) {
	config := godaddy.Config{
		APIKey:    apiKey,
		APISecret: apiSecret,
		TTL:       600, // 默认TTL为600秒
		// HTTP客户端超时设置，部分提供商支持
	}

	provider, err := godaddy.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建GoDaddy DNS提供商失败: %w", err)
	}

	return &GoDaddyProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// ====================================================================================

// Route53Provider AWS Route53 DNS提供商
type Route53Provider struct {
	*LegoDNSProviderWrapper
}

// NewRoute53Provider 创建AWS Route53 DNS提供商
func NewRoute53Provider(accessKey, secretKey, region string) (*Route53Provider, error) {
	config := route53.Config{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Region:          region,
		TTL:             300, // 默认TTL为300秒
		MaxRetries:      5,   // AWS API调用的最大重试次数
		// 注意: Route53使用AWS SDK自己管理HTTP连接
	}

	provider, err := route53.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建Route53 DNS提供商失败: %w", err)
	}

	return &Route53Provider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// ====================================================================================

// DigitalOceanProvider DigitalOcean DNS提供商
type DigitalOceanProvider struct {
	*LegoDNSProviderWrapper
}

// NewDigitalOceanProvider 创建DigitalOcean DNS提供商
func NewDigitalOceanProvider(token string) (*DigitalOceanProvider, error) {
	config := digitalocean.Config{
		AuthToken: token,
		TTL:       30, // 默认TTL为30秒（DigitalOcean默认值较低）
		// HTTP客户端超时设置，部分提供商支持
	}

	provider, err := digitalocean.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建DigitalOcean DNS提供商失败: %w", err)
	}

	return &DigitalOceanProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}

// ====================================================================================

// NamesiloProvider Namesilo DNS提供商
type NamesiloProvider struct {
	*LegoDNSProviderWrapper
}

// NewNamesiloProvider 创建Namesilo DNS提供商
func NewNamesiloProvider(apiKey string) (*NamesiloProvider, error) {
	config := namesilo.Config{
		APIKey: apiKey,
		TTL:    7200, // 默认TTL为7200秒（Namesilo默认值较高）
		// 注意：部分提供商不支持设置HTTPClient
	}

	provider, err := namesilo.NewDNSProviderConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("创建Namesilo DNS提供商失败: %w", err)
	}

	return &NamesiloProvider{
		LegoDNSProviderWrapper: &LegoDNSProviderWrapper{
			provider: provider,
			logger:   logger,
		},
	}, nil
}
