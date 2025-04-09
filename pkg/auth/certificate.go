package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/xsxdot/aio/pkg/common"
)

// CertificateManager 证书管理器
type CertificateManager struct {
	ca           *x509.Certificate
	caPrivKey    *rsa.PrivateKey
	caCertPEM    []byte
	caPrivKeyPEM []byte
	nodeCerts    map[string]NodeCertificate
	logger       *common.Logger
	certDir      string // 证书存储目录
}

// CertConfig 证书配置项
type CertConfig struct {
	CommonName   string   // 通用名称
	Organization []string // 组织名称
	DNSNames     []string // DNS名称
	IPAddresses  []string // IP地址
	ValidDays    int      // 有效天数，默认365天
	IsServer     bool     // 是否为服务器证书
	IsClient     bool     // 是否为客户端证书
}

// NewCertificateManager 创建证书管理器
func NewCertificateManager(caCertPath, caKeyPath string, certDir string) (*CertificateManager, error) {
	manager := &CertificateManager{
		nodeCerts: make(map[string]NodeCertificate),
		logger:    common.GetLogger().WithField("component", "certificate_manager"),
		certDir:   certDir,
	}

	// 确保证书目录存在
	if certDir != "" {
		if err := os.MkdirAll(certDir, 0755); err != nil {
			return nil, fmt.Errorf("创建证书目录失败: %w", err)
		}
	}

	// 如果提供了CA证书和私钥路径，则加载
	if caCertPath != "" && caKeyPath != "" {
		// 加载CA证书
		caCertPEM, err := LoadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("加载CA证书失败: %w", err)
		}

		// 解码CA证书
		caCert, err := DecodeCertificatePEM(caCertPEM)
		if err != nil {
			return nil, fmt.Errorf("解码CA证书失败: %w", err)
		}

		// 加载CA私钥
		caKeyPEM, err := LoadFile(caKeyPath)
		if err != nil {
			return nil, fmt.Errorf("加载CA私钥失败: %w", err)
		}

		// 解码CA私钥
		caKey, err := DecodeRSAPrivateKeyFromPEM(caKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("解码CA私钥失败: %w", err)
		}

		manager.ca = caCert
		manager.caPrivKey = caKey
		manager.caCertPEM = caCertPEM
		manager.caPrivKeyPEM = caKeyPEM
	} else {
		// 创建新的CA证书和私钥
		ca, caKey, caCertPEM, caKeyPEM, err := CreateCACertificate()
		if err != nil {
			return nil, fmt.Errorf("创建CA证书失败: %w", err)
		}

		manager.ca = ca
		manager.caPrivKey = caKey
		manager.caCertPEM = caCertPEM
		manager.caPrivKeyPEM = caKeyPEM

		// 如果提供了保存目录，则保存CA证书和私钥
		if certDir != "" {
			caCertPath = filepath.Join(certDir, "ca.crt")
			caKeyPath = filepath.Join(certDir, "ca.key")

			if err := os.WriteFile(caCertPath, caCertPEM, 0644); err != nil {
				return nil, fmt.Errorf("保存CA证书失败: %w", err)
			}

			if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
				return nil, fmt.Errorf("保存CA私钥失败: %w", err)
			}

			manager.logger.Infof("CA证书已保存至: %s", caCertPath)
			manager.logger.Infof("CA私钥已保存至: %s", caKeyPath)
		}
	}

	return manager, nil
}

// CreateCACertificate 创建CA证书
func CreateCACertificate() (*x509.Certificate, *rsa.PrivateKey, []byte, []byte, error) {
	// 生成私钥
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 生成序列号
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("生成序列号失败: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"AIO CA"},
			CommonName:   "AIO Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10年有效期
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// 生成证书
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("生成证书失败: %w", err)
	}

	// 解析证书
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("解析证书失败: %w", err)
	}

	// 编码证书
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// 编码私钥
	privKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	return cert, privKey, certPEM, privKeyPEM, nil
}

// GenerateNodeCertificate 生成节点证书 - 适用于etcd和NATS的通用证书
func (m *CertificateManager) GenerateNodeCertificate(nodeID string, ips []string, validityDays int) (*NodeCertificate, error) {
	return m.GenerateCustomCertificate(CertConfig{
		CommonName:   nodeID,
		Organization: []string{"AIO Node"},
		DNSNames:     []string{nodeID},
		IPAddresses:  ips,
		ValidDays:    validityDays,
		IsServer:     true,
		IsClient:     true,
	}, nodeID)
}

// GenerateCustomCertificate 生成自定义证书
func (m *CertificateManager) GenerateCustomCertificate(config CertConfig, certID string) (*NodeCertificate, error) {
	if m.ca == nil || m.caPrivKey == nil {
		return nil, fmt.Errorf("CA证书或私钥未初始化")
	}

	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 生成序列号
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("生成序列号失败: %w", err)
	}

	// 设置有效期
	notBefore := time.Now()
	var notAfter time.Time
	if config.ValidDays <= 0 {
		notAfter = notBefore.AddDate(1, 0, 0) // 默认1年
	} else {
		notAfter = notBefore.AddDate(0, 0, config.ValidDays)
	}

	// 设置密钥用途
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	var extKeyUsage []x509.ExtKeyUsage

	if config.IsServer {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageServerAuth)
	}

	if config.IsClient {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageClientAuth)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: config.Organization,
			CommonName:   config.CommonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
	}

	// 添加IP地址
	for _, ip := range config.IPAddresses {
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil {
			template.IPAddresses = append(template.IPAddresses, parsedIP)
		}
	}

	// 添加DNS名称
	template.DNSNames = append(template.DNSNames, config.DNSNames...)

	// 生成证书
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, m.ca, &privateKey.PublicKey, m.caPrivKey)
	if err != nil {
		return nil, fmt.Errorf("生成证书失败: %w", err)
	}

	// 解析证书
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("解析证书失败: %w", err)
	}

	// 编码证书
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// 编码私钥
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// 文件路径
	var certPath, keyPath string
	if m.certDir != "" {
		certPath = filepath.Join(m.certDir, fmt.Sprintf("%s.crt", certID))
		keyPath = filepath.Join(m.certDir, fmt.Sprintf("%s.key", certID))

		// 保存证书和私钥到文件
		if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
			return nil, fmt.Errorf("保存证书失败: %w", err)
		}

		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			return nil, fmt.Errorf("保存私钥失败: %w", err)
		}

		m.logger.Infof("证书已保存至: %s", certPath)
		m.logger.Infof("私钥已保存至: %s", keyPath)
	}

	// 创建节点证书
	nodeCert := &NodeCertificate{
		NodeID:         certID,
		Certificate:    cert,
		CertificatePEM: string(certPEM),
		PrivateKey:     privateKey,
		PrivateKeyPEM:  string(keyPEM),
		ExpiresAt:      notAfter,
		CreatedAt:      time.Now(),
		CertPath:       certPath,
		KeyPath:        keyPath,
	}

	// 存储节点证书
	m.nodeCerts[certID] = *nodeCert

	return nodeCert, nil
}

// VerifyNodeCertificate 验证节点证书
func (m *CertificateManager) VerifyNodeCertificate(cert *x509.Certificate) (bool, string, error) {
	if m.ca == nil {
		return false, "", fmt.Errorf("CA证书未初始化")
	}

	// 创建证书池
	roots := x509.NewCertPool()
	roots.AddCert(m.ca)

	// 验证证书
	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	if _, err := cert.Verify(opts); err != nil {
		return false, "", fmt.Errorf("证书验证失败: %w", err)
	}

	// 提取节点ID
	nodeID := ""
	if len(cert.Subject.CommonName) > 5 && cert.Subject.CommonName[:5] == "node-" {
		nodeID = cert.Subject.CommonName[5:]
	}

	return true, nodeID, nil
}

// GetNodeCertificate 获取节点证书
func (m *CertificateManager) GetNodeCertificate(nodeID string) (*NodeCertificate, error) {
	cert, ok := m.nodeCerts[nodeID]
	if !ok {
		return nil, fmt.Errorf("节点证书不存在")
	}
	return &cert, nil
}

// GetCACertificate 获取CA证书
func (m *CertificateManager) GetCACertificate() ([]byte, error) {
	if m.caCertPEM == nil {
		return nil, fmt.Errorf("CA证书未初始化")
	}
	return m.caCertPEM, nil
}

// GetCAPrivateKey 获取CA私钥
func (m *CertificateManager) GetCAPrivateKey() ([]byte, error) {
	if m.caPrivKeyPEM == nil {
		return nil, fmt.Errorf("CA私钥未初始化")
	}
	return m.caPrivKeyPEM, nil
}

// GetCAFilePath 获取CA证书文件路径
func (m *CertificateManager) GetCAFilePath() string {
	if m.certDir == "" {
		return ""
	}
	return filepath.Join(m.certDir, "ca.crt")
}

// RevokeCertificate 撤销证书
func (m *CertificateManager) RevokeCertificate(certID string) error {
	cert, ok := m.nodeCerts[certID]
	if !ok {
		return fmt.Errorf("证书不存在: %s", certID)
	}

	// 从内存中删除证书
	delete(m.nodeCerts, certID)

	// 如果有文件，则删除证书文件
	if cert.CertPath != "" && cert.KeyPath != "" {
		if err := os.Remove(cert.CertPath); err != nil && !os.IsNotExist(err) {
			m.logger.Warnf("删除证书文件失败: %s, %v", cert.CertPath, err)
		}

		if err := os.Remove(cert.KeyPath); err != nil && !os.IsNotExist(err) {
			m.logger.Warnf("删除私钥文件失败: %s, %v", cert.KeyPath, err)
		}

		m.logger.Infof("已撤销证书: %s", certID)
	}

	return nil
}

// LoadAllCertificates 从目录加载所有证书
func (m *CertificateManager) LoadAllCertificates() error {
	if m.certDir == "" {
		return fmt.Errorf("证书目录未设置")
	}

	files, err := os.ReadDir(m.certDir)
	if err != nil {
		return fmt.Errorf("读取证书目录失败: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".crt" || file.Name() == "ca.crt" {
			continue
		}

		certID := file.Name()[:len(file.Name())-4] // 移除.crt后缀

		certPath := filepath.Join(m.certDir, file.Name())
		keyPath := filepath.Join(m.certDir, certID+".key")

		// 检查私钥是否存在
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			m.logger.Warnf("找到证书但私钥不存在: %s", certPath)
			continue
		}

		// 加载证书
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			m.logger.Warnf("读取证书失败: %s, %v", certPath, err)
			continue
		}

		cert, err := DecodeCertificatePEM(certPEM)
		if err != nil {
			m.logger.Warnf("解码证书失败: %s, %v", certPath, err)
			continue
		}

		// 加载私钥
		keyPEM, err := os.ReadFile(keyPath)
		if err != nil {
			m.logger.Warnf("读取私钥失败: %s, %v", keyPath, err)
			continue
		}

		key, err := DecodeRSAPrivateKeyFromPEM(keyPEM)
		if err != nil {
			m.logger.Warnf("解码私钥失败: %s, %v", keyPath, err)
			continue
		}

		// 创建节点证书
		nodeCert := NodeCertificate{
			NodeID:         certID,
			Certificate:    cert,
			CertificatePEM: string(certPEM),
			PrivateKey:     key,
			PrivateKeyPEM:  string(keyPEM),
			ExpiresAt:      cert.NotAfter,
			CreatedAt:      cert.NotBefore,
			CertPath:       certPath,
			KeyPath:        keyPath,
		}

		// 存储节点证书
		m.nodeCerts[certID] = nodeCert
		m.logger.Infof("已加载证书: %s", certID)
	}

	return nil
}

// ListCertificates 列出所有证书
func (m *CertificateManager) ListCertificates() []NodeCertificate {
	certs := make([]NodeCertificate, 0, len(m.nodeCerts))
	for _, cert := range m.nodeCerts {
		certs = append(certs, cert)
	}
	return certs
}

// SaveCACertificate 保存CA证书到指定路径
func (m *CertificateManager) SaveCACertificate(certPath, keyPath string) error {
	if m.caCertPEM == nil || m.caPrivKeyPEM == nil {
		return fmt.Errorf("CA证书或私钥未初始化")
	}

	// 保存CA证书
	if err := os.WriteFile(certPath, m.caCertPEM, 0644); err != nil {
		return fmt.Errorf("保存CA证书失败: %w", err)
	}

	// 保存CA私钥
	if err := os.WriteFile(keyPath, m.caPrivKeyPEM, 0600); err != nil {
		return fmt.Errorf("保存CA私钥失败: %w", err)
	}

	m.logger.Infof("CA证书已保存至: %s", certPath)
	m.logger.Infof("CA私钥已保存至: %s", keyPath)

	return nil
}

// LoadFile 加载文件内容
func LoadFile(path string) ([]byte, error) {
	// 使用标准库读取文件
	return os.ReadFile(path)
}

// DecodeCertificatePEM 解码PEM格式证书
func DecodeCertificatePEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("无效的证书PEM格式")
	}

	return x509.ParseCertificate(block.Bytes)
}
