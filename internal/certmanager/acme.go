package certmanager

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/registration"
	"github.com/sirupsen/logrus"
)

// LegoClient 是一个使用lego库实现的ACME客户端
type LegoClient struct {
	email        string
	privateKey   crypto.PrivateKey
	registration *registration.Resource
	config       *lego.Config
	client       *lego.Client
	logger       *logrus.Logger
	dnsConfig    *Config // 包含DNS配置的完整配置
}

// 用户需要实现的接口
type userImpl struct {
	email        string
	key          crypto.PrivateKey
	registration *registration.Resource
}

func (u *userImpl) GetEmail() string {
	return u.email
}

func (u *userImpl) GetRegistration() *registration.Resource {
	return u.registration
}

func (u *userImpl) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// NewLegoClient 创建一个新的ACME客户端
func NewLegoClient(logger *logrus.Logger, config *Config) *LegoClient {
	if logger == nil {
		logger = logrus.New()
	}

	return &LegoClient{
		logger:    logger,
		dnsConfig: config,
	}
}

// Init 初始化ACME客户端
func (c *LegoClient) Init(ctx context.Context, email string, staging bool) error {
	c.email = email

	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("无法生成私钥: %w", err)
	}
	c.privateKey = privateKey

	// 创建用户
	user := &userImpl{
		email: email,
		key:   privateKey,
	}

	// 创建配置
	config := lego.NewConfig(user)
	if staging {
		config.CADirURL = lego.LEDirectoryStaging
	} else {
		config.CADirURL = lego.LEDirectoryProduction
	}
	c.config = config

	// 创建客户端
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("无法创建ACME客户端: %w", err)
	}
	c.client = client

	// 设置验证方式
	if c.dnsConfig.VerifyMethod == VerifyDNS {
		// 使用DNS-01验证
		if err := c.setupDNSProvider(); err != nil {
			return fmt.Errorf("无法设置DNS验证提供程序: %w", err)
		}

		c.logger.Info("使用DNS-01验证方式")
	} else {
		// 默认使用HTTP-01验证
		err = client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))
		if err != nil {
			return fmt.Errorf("无法设置HTTP验证提供程序: %w", err)
		}

		c.logger.Info("使用HTTP-01验证方式")
	}

	// 注册账号
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return fmt.Errorf("无法注册Let's Encrypt账号: %w", err)
	}
	user.registration = reg
	c.registration = reg

	c.logger.Infof("成功初始化ACME客户端，账号: %s", email)
	return nil
}

// setupDNSProvider 设置DNS提供商
func (c *LegoClient) setupDNSProvider() error {
	var provider dns01.Provider
	var err error

	switch c.dnsConfig.DNSProvider {
	case DNSProviderAliyun:
		// 阿里云DNS配置
		config := alidns.Config{
			APIKey:             c.dnsConfig.DNSConfig.AliyunAccessKeyID,
			SecretKey:          c.dnsConfig.DNSConfig.AliyunAccessKeySecret,
			PropagationTimeout: c.dnsConfig.DNSPropagationTimeout,
			PollingInterval:    time.Second * 15, // 每15秒检查一次DNS传播
			TTL:                600,              // 默认TTL为600秒
			HTTPTimeout:        time.Second * 30, // HTTP请求超时
		}

		if c.dnsConfig.DNSConfig.AliyunRegionID != "" {
			config.RegionID = c.dnsConfig.DNSConfig.AliyunRegionID
		}

		provider, err = alidns.NewDNSProviderConfig(&config)
		if err != nil {
			return fmt.Errorf("配置阿里云DNS提供商失败: %w", err)
		}

		c.logger.Info("成功配置阿里云DNS提供商")
	default:
		return fmt.Errorf("不支持的DNS提供商: %s", c.dnsConfig.DNSProvider)
	}

	// 设置DNS验证提供商
	if err := c.client.Challenge.SetDNS01Provider(provider); err != nil {
		return err
	}

	return nil
}

// ObtainCertificate 获取证书
func (c *LegoClient) ObtainCertificate(domain string) (certPEM, keyPEM []byte, err error) {
	if c.client == nil {
		return nil, nil, fmt.Errorf("ACME客户端未初始化")
	}

	c.logger.Infof("正在为域名 %s 申请证书", domain)

	// 申请证书
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}
	certificates, err := c.client.Certificate.Obtain(request)
	if err != nil {
		return nil, nil, fmt.Errorf("无法获取证书: %w", err)
	}

	c.logger.Infof("成功获取域名 %s 的证书", domain)
	return certificates.Certificate, certificates.PrivateKey, nil
}
