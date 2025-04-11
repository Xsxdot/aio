package certmanager

import (
	"time"
)

// 验证方式
type VerifyMethod string

const (
	// HTTP-01验证
	VerifyHTTP VerifyMethod = "http-01"
	// DNS-01验证
	VerifyDNS VerifyMethod = "dns-01"
)

// DNS提供商类型
type DNSProviderType string

const (
	// 阿里云DNS
	DNSProviderAliyun DNSProviderType = "aliyun"
	// 其他DNS提供商可以在此添加
)

// Config 证书管理器配置
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

	// 验证方式：http-01 或 dns-01
	VerifyMethod VerifyMethod `json:"verify_method"`

	// DNS提供商类型
	DNSProvider DNSProviderType `json:"dns_provider"`

	// DNS提供商配置
	DNSConfig DNSConfig `json:"dns_config"`

	// DNS记录传播等待时间
	DNSPropagationTimeout time.Duration `json:"dns_propagation_timeout"`
}

// DNSConfig DNS提供商配置
type DNSConfig struct {
	// 阿里云AccessKey ID
	AliyunAccessKeyID string `json:"aliyun_access_key_id"`

	// 阿里云AccessKey Secret
	AliyunAccessKeySecret string `json:"aliyun_access_key_secret"`

	// 阿里云区域
	AliyunRegionID string `json:"aliyun_region_id"`

	// 其他DNS提供商的配置可以在此添加
}

// Certificate 表示一个证书
type Certificate struct {
	// 域名
	Domain string `json:"domain"`

	// 证书文件路径
	CertFile string `json:"cert_file"`

	// 私钥文件路径
	KeyFile string `json:"key_file"`

	// 证书过期时间
	ExpiryDate time.Time `json:"expiry_date"`

	// 上次续期时间
	LastRenewalDate time.Time `json:"last_renewal_date"`

	// 是否是通配符证书
	IsWildcard bool `json:"is_wildcard"`
}

// 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:               false,
		CertDir:               "./certs",
		RenewBefore:           30, // 证书有效期小于30天时自动续期
		Staging:               false,
		CheckInterval:         24 * time.Hour, // 每天检查一次
		VerifyMethod:          VerifyHTTP,
		DNSPropagationTimeout: 120 * time.Second, // DNS记录传播等待2分钟
	}
}
