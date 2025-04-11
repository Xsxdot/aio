package certmanager

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// 定义接口方便进行单元测试
type CertificateManager interface {
	// 初始化证书管理器
	Init(ctx context.Context) error

	// 获取某个域名的证书文件路径，如果不存在或过期则申请
	GetCertificate(domain string) (*Certificate, error)

	// 手动触发证书续期
	RenewCertificate(domain string) error

	// 启动自动续期任务
	StartRenewalTask(ctx context.Context) error

	// 停止自动续期任务
	StopRenewalTask() error
}

// 创建了一个简化版的ACME客户端接口，以便后续实现和测试
type acmeClient interface {
	Init(ctx context.Context, email string, staging bool) error
	ObtainCertificate(domain string) (certPEM, keyPEM []byte, err error)
}

// 实现 ACME 用户接口
type acmeUser struct {
	email        string
	registration interface{}
	key          *rsa.PrivateKey
}

func (u *acmeUser) GetEmail() string {
	return u.email
}

func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// Manager 实现CertificateManager接口
type Manager struct {
	config *Config
	logger *logrus.Logger
	client acmeClient
	user   *acmeUser
	certs  map[string]*Certificate
	mutex  sync.RWMutex
	cancel context.CancelFunc
}

// NewManager 创建一个新的证书管理器
func NewManager(config *Config, logger *logrus.Logger, client acmeClient) *Manager {
	if logger == nil {
		logger = logrus.New()
	}

	if client == nil {
		client = &defaultAcmeClient{}
	}

	return &Manager{
		config: config,
		logger: logger,
		client: client,
		certs:  make(map[string]*Certificate),
	}
}

// 默认的ACME客户端实现
type defaultAcmeClient struct {
	// 这里将在实际实现中使用go-acme/lego库
}

func (c *defaultAcmeClient) Init(ctx context.Context, email string, staging bool) error {
	// 这里将在实际实现中使用go-acme/lego库
	return nil
}

func (c *defaultAcmeClient) ObtainCertificate(domain string) (certPEM, keyPEM []byte, err error) {
	// 这里将在实际实现中使用go-acme/lego库
	// 现在返回一个简单的自签名证书用于测试

	// 使用示例数据创建证书和私钥，避免实际生成以防止错误
	// 在实际实现中，这将是从Let's Encrypt获取的证书
	certPEM = []byte("--- PLACEHOLDER CERTIFICATE ---")
	keyPEM = []byte("--- PLACEHOLDER PRIVATE KEY ---")

	return certPEM, keyPEM, nil
}

// Init 初始化证书管理器
func (m *Manager) Init(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("Certificate manager is disabled")
		return nil
	}

	// 创建证书目录
	if err := os.MkdirAll(m.config.CertDir, 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	// 创建私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// 创建用户
	m.user = &acmeUser{
		email: m.config.Email,
		key:   privateKey,
	}

	// 初始化ACME客户端
	if err := m.client.Init(ctx, m.config.Email, m.config.Staging); err != nil {
		return fmt.Errorf("failed to initialize ACME client: %w", err)
	}

	// 加载已有的证书信息
	if err := m.loadExistingCertificates(); err != nil {
		m.logger.Warnf("Failed to load existing certificates: %v", err)
	}

	return nil
}

// loadExistingCertificates 加载已存在的证书
func (m *Manager) loadExistingCertificates() error {
	// 确保目录存在
	if _, err := os.Stat(m.config.CertDir); os.IsNotExist(err) {
		return nil
	}

	for _, domain := range m.config.Domains {
		certFile := filepath.Join(m.config.CertDir, domain+".crt")
		keyFile := filepath.Join(m.config.CertDir, domain+".key")

		// 检查文件是否存在
		if !fileExists(certFile) || !fileExists(keyFile) {
			continue
		}

		// 解析证书以获取过期时间
		certData, err := os.ReadFile(certFile)
		if err != nil {
			m.logger.Warnf("Failed to read certificate file for %s: %v", domain, err)
			continue
		}

		block, _ := pem.Decode(certData)
		if block == nil {
			m.logger.Warnf("Failed to decode PEM for %s", domain)
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			m.logger.Warnf("Failed to parse certificate for %s: %v", domain, err)
			continue
		}

		// 保存证书信息
		m.mutex.Lock()
		m.certs[domain] = &Certificate{
			Domain:     domain,
			CertFile:   certFile,
			KeyFile:    keyFile,
			ExpiryDate: cert.NotAfter,
		}
		m.mutex.Unlock()

		m.logger.Infof("Loaded existing certificate for %s, expires on %s", domain, cert.NotAfter)
	}

	return nil
}

// fileExists 检查文件是否存在
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// GetCertificate 获取证书，如果不存在或已过期则自动申请
func (m *Manager) GetCertificate(domain string) (*Certificate, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("certificate manager is disabled")
	}

	m.mutex.RLock()
	cert, exists := m.certs[domain]
	m.mutex.RUnlock()

	// 如果证书不存在或已过期，则申请新证书
	if !exists || time.Now().Add(time.Duration(m.config.RenewBefore)*24*time.Hour).After(cert.ExpiryDate) {
		if err := m.RenewCertificate(domain); err != nil {
			return nil, err
		}

		m.mutex.RLock()
		cert = m.certs[domain]
		m.mutex.RUnlock()
	}

	return cert, nil
}

// RenewCertificate 申请或续期证书
func (m *Manager) RenewCertificate(domain string) error {
	if !m.config.Enabled {
		return fmt.Errorf("certificate manager is disabled")
	}

	m.logger.Infof("Requesting certificate for domain: %s", domain)

	// 检查域名是否在配置的域名列表中
	domainAllowed := false
	for _, d := range m.config.Domains {
		if d == domain {
			domainAllowed = true
			break
		}
	}

	if !domainAllowed {
		return fmt.Errorf("domain %s is not in the allowed domains list", domain)
	}

	// 检查是否为通配符证书
	isWildcard := len(domain) > 1 && domain[0] == '*'
	if isWildcard && m.config.VerifyMethod != VerifyDNS {
		return fmt.Errorf("通配符证书 %s 只能使用DNS验证方式", domain)
	}

	// 申请证书
	certPEM, keyPEM, err := m.client.ObtainCertificate(domain)
	if err != nil {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// 保存证书和私钥
	certFile := filepath.Join(m.config.CertDir, domain+".crt")
	keyFile := filepath.Join(m.config.CertDir, domain+".key")

	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// 解析证书以获取过期时间
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// 更新证书信息
	m.mutex.Lock()
	m.certs[domain] = &Certificate{
		Domain:          domain,
		CertFile:        certFile,
		KeyFile:         keyFile,
		ExpiryDate:      cert.NotAfter,
		LastRenewalDate: time.Now(),
		IsWildcard:      isWildcard,
	}
	m.mutex.Unlock()

	m.logger.Infof("Certificate for %s has been issued and saved, expires on %s", domain, cert.NotAfter)
	return nil
}

// StartRenewalTask 启动自动续期任务
func (m *Manager) StartRenewalTask(ctx context.Context) error {
	if !m.config.Enabled {
		return nil
	}

	m.logger.Info("Starting certificate renewal task")

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	go func() {
		ticker := time.NewTicker(m.config.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				m.logger.Info("Certificate renewal task stopped")
				return
			case <-ticker.C:
				m.checkAndRenewCertificates()
			}
		}
	}()

	return nil
}

// StopRenewalTask 停止自动续期任务
func (m *Manager) StopRenewalTask() error {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	return nil
}

// checkAndRenewCertificates 检查所有证书并在需要时续期
func (m *Manager) checkAndRenewCertificates() {
	m.mutex.RLock()
	domains := make([]string, 0, len(m.config.Domains))
	for _, domain := range m.config.Domains {
		domains = append(domains, domain)
	}
	m.mutex.RUnlock()

	for _, domain := range domains {
		m.mutex.RLock()
		cert, exists := m.certs[domain]
		m.mutex.RUnlock()

		// 证书不存在或即将过期
		needsRenewal := !exists
		if exists {
			needsRenewal = time.Now().Add(time.Duration(m.config.RenewBefore) * 24 * time.Hour).After(cert.ExpiryDate)
		}

		if needsRenewal {
			m.logger.Infof("Certificate for %s needs renewal, requesting new certificate", domain)
			if err := m.RenewCertificate(domain); err != nil {
				m.logger.Errorf("Failed to renew certificate for %s: %v", domain, err)
			} else {
				m.logger.Infof("Successfully renewed certificate for %s", domain)
			}
		}
	}
}
