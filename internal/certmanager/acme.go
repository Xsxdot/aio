package certmanager

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"go.uber.org/zap"
)

// 全局ACME客户端
var (
	acmeClient *lego.Client
	acmeUser   *ACMEUser
	logger     *zap.Logger
)

// ACMEUser 实现acme.User接口
type ACMEUser struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey
}

// GetEmail 实现acme.User接口
func (u *ACMEUser) GetEmail() string {
	return u.Email
}

// GetRegistration 实现acme.User接口
func (u *ACMEUser) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey 实现acme.User接口
func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

// SetLogger 设置日志记录器
func SetLogger(l *zap.Logger) {
	logger = l
}

// initACMEClient 初始化ACME客户端
func initACMEClient(email string, useStaging bool) error {
	// 生成用户私钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("生成私钥失败: %v", err)
	}

	// 创建用户
	acmeUser = &ACMEUser{
		Email: email,
		Key:   privateKey,
	}

	// 创建客户端配置
	config := lego.NewConfig(acmeUser)

	// 设置使用的ACME目录URL（正式或测试环境）
	if useStaging {
		config.CADirURL = lego.LEDirectoryStaging
	} else {
		config.CADirURL = lego.LEDirectoryProduction
	}

	// 创建客户端
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("创建ACME客户端失败: %v", err)
	}

	// 注册
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return fmt.Errorf("注册ACME账户失败: %v", err)
	}

	acmeUser.Registration = reg
	acmeClient = client

	return nil
}

// SimpleDNSProvider 简单DNS提供商接口
type SimpleDNSProvider interface {
	// Present 添加DNS TXT记录
	Present(domain, token, keyAuth string) error
	// CleanUp 清理DNS TXT记录
	CleanUp(domain, token, keyAuth string) error
}

// CustomDNSProviderFactory 自定义DNS提供商工厂
// 由外部实现不同DNS提供商的工厂函数
var CustomDNSProviderFactory func(providerName string, credentials map[string]string) (SimpleDNSProvider, error)

// InitWithEmail 使用指定电子邮件初始化ACME客户端
func InitWithEmail(email string, useStaging bool) error {
	return initACMEClient(email, useStaging)
}

// GetCertificate 申请或续期证书
func GetCertificate(ctx context.Context, domain string, certDir string, dnsProvider string, dnsCredentials map[string]string) (certPath, keyPath string, issuedAt, expiresAt time.Time, err error) {
	return obtainCertificate(ctx, domain, certDir, dnsProvider, dnsCredentials)
}

// obtainCertificate 申请证书（仅使用DNS验证）
func obtainCertificate(ctx context.Context, domain string, certDir string, dnsProvider string, dnsCredentials map[string]string) (certPath, keyPath string, issuedAt, expiresAt time.Time, err error) {
	if acmeClient == nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("ACME客户端未初始化")
	}

	// 验证参数
	if dnsProvider == "" {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("DNS提供商不能为空")
	}

	if len(dnsCredentials) == 0 {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("DNS提供商的凭证不能为空")
	}

	if logger != nil {
		logger.Info("开始申请证书（使用DNS验证）",
			zap.String("domain", domain),
			zap.String("dns_provider", dnsProvider))
	}

	// 设置环境变量
	for key, value := range dnsCredentials {
		os.Setenv(key, value)
	}
	defer func() {
		// 清理环境变量
		for key := range dnsCredentials {
			os.Unsetenv(key)
		}
	}()

	// 设置DNS提供商
	var provider challenge.Provider

	// 使用自定义DNS提供商工厂
	if CustomDNSProviderFactory != nil {
		simpleProvider, err := CustomDNSProviderFactory(dnsProvider, dnsCredentials)
		if err != nil {
			return "", "", time.Time{}, time.Time{}, fmt.Errorf("创建DNS提供商失败: %v", err)
		}

		provider = simpleProvider
		if logger != nil {
			logger.Info("使用自定义DNS提供商", zap.String("provider", dnsProvider))
		}
	} else {
		// 无法使用外部DNS提供商，报错
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("未设置DNS提供商工厂函数，无法创建DNS提供商")
	}

	// 设置DNS提供商到ACME客户端
	// 添加公共DNS解析器，确保DNS记录传播验证正常工作
	dnsOptions := []dns01.ChallengeOption{
		// 添加全球常用的公共DNS解析器
		dns01.AddRecursiveNameservers([]string{
			"8.8.8.8:53",         // Google DNS
			"8.8.4.4:53",         // Google DNS 备用
			"1.1.1.1:53",         // Cloudflare DNS
			"1.0.0.1:53",         // Cloudflare DNS 备用
			"223.5.5.5:53",       // 阿里DNS
			"223.6.6.6:53",       // 阿里DNS 备用
			"114.114.114.114:53", // 114DNS
			"114.114.115.115:53", // 114DNS 备用
			"9.9.9.9:53",         // Quad9
			"149.112.112.112:53", // Quad9 备用
		}),
		// 设置DNS验证超时时间较长，以适应慢速DNS提供商
		dns01.AddDNSTimeout(30 * time.Second),
	}

	err = acmeClient.Challenge.SetDNS01Provider(provider, dnsOptions...)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("设置DNS验证提供商失败: %v", err)
	}

	// 申请证书
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	certificates, err := acmeClient.Certificate.Obtain(request)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("申请证书失败: %v", err)
	}

	// 解析证书获取有效期
	certPEM, _ := pem.Decode(certificates.Certificate)
	if certPEM == nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("解析证书PEM失败")
	}

	cert, err := x509.ParseCertificate(certPEM.Bytes)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("解析证书失败: %v", err)
	}

	issuedAt = cert.NotBefore
	expiresAt = cert.NotAfter

	// 确保域名目录存在
	domainDir := filepath.Join(certDir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("创建域名目录失败: %v", err)
	}

	// 保存证书和私钥
	timestamp := time.Now().Format("20060102150405")
	certFilename := fmt.Sprintf("%s-%s.crt", domain, timestamp)
	keyFilename := fmt.Sprintf("%s-%s.key", domain, timestamp)

	certPath = filepath.Join(domainDir, certFilename)
	keyPath = filepath.Join(domainDir, keyFilename)

	// 写入证书文件
	if err := os.WriteFile(certPath, certificates.Certificate, 0644); err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("保存证书文件失败: %v", err)
	}

	// 写入密钥文件
	if err := os.WriteFile(keyPath, certificates.PrivateKey, 0600); err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("保存私钥文件失败: %v", err)
	}

	if logger != nil {
		logger.Info("证书申请成功",
			zap.String("domain", domain),
			zap.Time("issued_at", issuedAt),
			zap.Time("expires_at", expiresAt),
			zap.String("cert_path", certPath),
			zap.String("key_path", keyPath),
			zap.Int("valid_days", int(expiresAt.Sub(time.Now()).Hours()/24)))
	}

	return certPath, keyPath, issuedAt, expiresAt, nil
}
