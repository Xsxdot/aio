package service

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/ssl/internal/model"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
	"github.com/go-acme/lego/v4/registration"
)

const (
	// DefaultACMEServer Let's Encrypt 生产环境
	DefaultACMEServer = lego.LEDirectoryProduction
	// StagingACMEServer Let's Encrypt 测试环境（用于开发测试）
	StagingACMEServer = lego.LEDirectoryStaging
)

// AcmeService ACME 服务
// 封装 lego 调用，实现 Let's Encrypt 证书申请与续期
type AcmeService struct {
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewAcmeService 创建 ACME 服务实例
func NewAcmeService(log *logger.Log) *AcmeService {
	return &AcmeService{
		log: log.WithEntryName("AcmeService"),
		err: errorc.NewErrorBuilder("AcmeService"),
	}
}

// AcmeUser 实现 lego.User 接口
type AcmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *AcmeUser) GetEmail() string {
	return u.Email
}

func (u *AcmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *AcmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// IssueCertificateRequest 证书签发请求
type IssueCertificateRequest struct {
	Domains    []string          // 域名列表
	Email      string            // 注册邮箱
	Provider   model.DnsProvider // DNS 服务商
	AccessKey  string            // 访问密钥（已解密）
	SecretKey  string            // 访问密钥（已解密）
	AccountKey string            // ACME 账户私钥（可选，为空则创建新账户）
	UseStaging bool              // 是否使用测试环境
}

// IssueCertificateResponse 证书签发响应
type IssueCertificateResponse struct {
	FullchainPem  string    // 完整证书链 PEM
	PrivkeyPem    string    // 私钥 PEM
	CertURL       string    // 证书资源 URL
	ExpiresAt     time.Time // 过期时间
	IssuedAt      time.Time // 签发时间
	AccountURL    string    // ACME 账户 URL
	AccountKeyPem string    // ACME 账户私钥 PEM
}

// IssueCertificate 签发证书
func (s *AcmeService) IssueCertificate(req *IssueCertificateRequest) (*IssueCertificateResponse, error) {
	s.log.WithFields(map[string]interface{}{
		"domains":  req.Domains,
		"email":    req.Email,
		"provider": req.Provider,
	}).Info("开始签发证书")

	// 1. 创建或加载 ACME 账户
	user, err := s.createOrLoadUser(req.Email, req.AccountKey)
	if err != nil {
		return nil, s.err.New("创建或加载 ACME 账户失败", err)
	}

	// 2. 创建 lego 配置
	config := lego.NewConfig(user)
	if req.UseStaging {
		config.CADirURL = StagingACMEServer
	} else {
		config.CADirURL = DefaultACMEServer
	}

	// 3. 创建 lego 客户端
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, s.err.New("创建 ACME 客户端失败", err)
	}

	// 4. 设置 DNS-01 Challenge Provider
	dnsProvider, err := s.createDNSProvider(req.Provider, req.AccessKey, req.SecretKey)
	if err != nil {
		return nil, s.err.New("创建 DNS Provider 失败", err)
	}

	err = client.Challenge.SetDNS01Provider(dnsProvider,
		dns01.AddDNSTimeout(120*time.Second), // DNS 传播超时时间
		dns01.AddRecursiveNameservers([]string{ // 使用公共 DNS 服务器检查
			"8.8.8.8:53",
			"1.1.1.1:53",
		}),
	)
	if err != nil {
		return nil, s.err.New("设置 DNS-01 Provider 失败", err)
	}

	// 5. 注册 ACME 账户（如果是新账户）
	if user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, s.err.New("注册 ACME 账户失败", err)
		}
		user.Registration = reg
		s.log.WithField("account_url", reg.URI).Info("ACME 账户注册成功")
	}

	// 6. 申请证书
	request := certificate.ObtainRequest{
		Domains: req.Domains,
		Bundle:  true, // 获取完整证书链
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return nil, s.err.New("申请证书失败", err)
	}

	s.log.WithField("cert_url", certificates.CertURL).Info("证书申请成功")

	// 7. 解析证书获取过期时间
	expiresAt, issuedAt, err := s.parseCertificate(certificates.Certificate)
	if err != nil {
		return nil, s.err.New("解析证书失败", err)
	}

	// 8. 序列化账户私钥
	accountKeyPem, err := s.encodePrivateKey(user.key)
	if err != nil {
		return nil, s.err.New("序列化账户私钥失败", err)
	}

	return &IssueCertificateResponse{
		FullchainPem:  string(certificates.Certificate),
		PrivkeyPem:    string(certificates.PrivateKey),
		CertURL:       certificates.CertURL,
		ExpiresAt:     expiresAt,
		IssuedAt:      issuedAt,
		AccountURL:    user.Registration.URI,
		AccountKeyPem: accountKeyPem,
	}, nil
}

// RenewCertificate 续期证书
func (s *AcmeService) RenewCertificate(req *IssueCertificateRequest, existingCertPem, existingKeyPem string) (*IssueCertificateResponse, error) {
	s.log.WithFields(map[string]interface{}{
		"domains":  req.Domains,
		"email":    req.Email,
		"provider": req.Provider,
	}).Info("开始续期证书")

	// 1. 创建或加载 ACME 账户
	user, err := s.createOrLoadUser(req.Email, req.AccountKey)
	if err != nil {
		return nil, s.err.New("创建或加载 ACME 账户失败", err)
	}

	// 2. 创建 lego 配置
	config := lego.NewConfig(user)
	if req.UseStaging {
		config.CADirURL = StagingACMEServer
	} else {
		config.CADirURL = DefaultACMEServer
	}

	// 3. 创建 lego 客户端
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, s.err.New("创建 ACME 客户端失败", err)
	}

	// 4. 设置 DNS-01 Challenge Provider
	dnsProvider, err := s.createDNSProvider(req.Provider, req.AccessKey, req.SecretKey)
	if err != nil {
		return nil, s.err.New("创建 DNS Provider 失败", err)
	}

	err = client.Challenge.SetDNS01Provider(dnsProvider,
		dns01.AddDNSTimeout(120*time.Second),
		dns01.AddRecursiveNameservers([]string{
			"8.8.8.8:53",
			"1.1.1.1:53",
		}),
	)
	if err != nil {
		return nil, s.err.New("设置 DNS-01 Provider 失败", err)
	}

	// 5. 注册 ACME 账户（如果是新账户）
	if user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, s.err.New("注册 ACME 账户失败", err)
		}
		user.Registration = reg
	}

	// 6. 续期证书
	certResource := certificate.Resource{
		Certificate: []byte(existingCertPem),
		PrivateKey:  []byte(existingKeyPem),
	}

	certificates, err := client.Certificate.Renew(certResource, true, false, "")
	if err != nil {
		return nil, s.err.New("续期证书失败", err)
	}

	s.log.WithField("cert_url", certificates.CertURL).Info("证书续期成功")

	// 7. 解析证书获取过期时间
	expiresAt, issuedAt, err := s.parseCertificate(certificates.Certificate)
	if err != nil {
		return nil, s.err.New("解析证书失败", err)
	}

	// 8. 序列化账户私钥
	accountKeyPem, err := s.encodePrivateKey(user.key)
	if err != nil {
		return nil, s.err.New("序列化账户私钥失败", err)
	}

	return &IssueCertificateResponse{
		FullchainPem:  string(certificates.Certificate),
		PrivkeyPem:    string(certificates.PrivateKey),
		CertURL:       certificates.CertURL,
		ExpiresAt:     expiresAt,
		IssuedAt:      issuedAt,
		AccountURL:    user.Registration.URI,
		AccountKeyPem: accountKeyPem,
	}, nil
}

// createOrLoadUser 创建或加载 ACME 用户
func (s *AcmeService) createOrLoadUser(email, accountKeyPem string) (*AcmeUser, error) {
	var privateKey crypto.PrivateKey
	var err error

	if accountKeyPem != "" {
		// 加载现有账户私钥
		privateKey, err = s.decodePrivateKey(accountKeyPem)
		if err != nil {
			return nil, fmt.Errorf("解码账户私钥失败: %w", err)
		}
	} else {
		// 创建新的账户私钥
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("生成账户私钥失败: %w", err)
		}
	}

	return &AcmeUser{
		Email: email,
		key:   privateKey,
	}, nil
}

// createDNSProvider 创建 DNS Provider
func (s *AcmeService) createDNSProvider(provider model.DnsProvider, accessKey, secretKey string) (challenge.Provider, error) {
	// DNSPod 映射到 TencentCloud
	if provider == model.DnsProviderDNSPod {
		provider = model.DnsProviderTencentCloud
		s.log.Info("DNSPod 映射到 TencentCloud DNS Provider")
	}

	switch provider {
	case model.DnsProviderAliDNS:
		config := alidns.NewDefaultConfig()
		config.APIKey = accessKey
		config.SecretKey = secretKey
		return alidns.NewDNSProviderConfig(config)

	case model.DnsProviderTencentCloud:
		config := tencentcloud.NewDefaultConfig()
		config.SecretID = accessKey
		config.SecretKey = secretKey
		return tencentcloud.NewDNSProviderConfig(config)

	default:
		return nil, fmt.Errorf("不支持的 DNS Provider: %s", provider)
	}
}

// parseCertificate 解析证书获取过期时间和签发时间
func (s *AcmeService) parseCertificate(certPem []byte) (expiresAt, issuedAt time.Time, err error) {
	block, _ := pem.Decode(certPem)
	if block == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("无法解析 PEM 格式证书")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("解析 X509 证书失败: %w", err)
	}

	return cert.NotAfter, cert.NotBefore, nil
}

// encodePrivateKey 编码私钥为 PEM 格式
func (s *AcmeService) encodePrivateKey(key crypto.PrivateKey) (string, error) {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return "", err
		}
		pemBlock := &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: keyBytes,
		}
		return string(pem.EncodeToMemory(pemBlock)), nil
	default:
		return "", fmt.Errorf("不支持的私钥类型")
	}
}

// decodePrivateKey 解码 PEM 格式私钥
func (s *AcmeService) decodePrivateKey(pemStr string) (crypto.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("无法解析 PEM 格式私钥")
	}

	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("不支持的私钥类型: %s", block.Type)
	}
}

// ExtraConfig DNS Provider 额外配置
type ExtraConfig struct {
	Region   string `json:"region"`
	Endpoint string `json:"endpoint"`
}

// ParseExtraConfig 解析额外配置
func ParseExtraConfig(configJSON string) (*ExtraConfig, error) {
	if configJSON == "" {
		return &ExtraConfig{}, nil
	}

	var config ExtraConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("解析额外配置失败: %w", err)
	}

	return &config, nil
}
