package certmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/common"

	"encoding/base64"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/scheduler"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

const (
	// etcd前缀，用于存储证书数据
	etcdCertPrefix = "/aio/certs/"
	// etcd前缀，用于存储DNS提供商配置
	etcdDNSConfigPrefix = "/aio/dns-providers/"
	// 本地证书保存目录
	defaultCertDir = "ssl"
	// 证书检查任务名称
	certCheckTaskName = "cert_check_task"
	// 证书检查周期
	certCheckInterval = 24 * time.Hour
	// 证书续期阈值（天数）
	renewThresholdDays = 30
	// 新增DNS配置的etcd前缀
	dnsConfigPrefix = "/cert/dns_config/"
)

// DNSConfig 存储DNS验证提供商的配置
type DNSConfig struct {
	Provider    string            `json:"provider"`
	Credentials map[string]string `json:"credentials"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CertManager 证书管理器组件
type CertManager struct {
	logger     *zap.Logger
	config     *CertManagerConfig
	etcdClient *etcd.EtcdClient
	scheduler  *scheduler.Scheduler
	domains    map[string]*DomainCert // 管理的域名及其证书状态
	certDir    string                 // 证书存储目录
	status     consts.ComponentStatus
	mu         sync.RWMutex
	// dnsConfig存储当前使用的DNS配置
	dnsConfig *DNSConfig
}

// DomainCert 表示域名的证书信息
type DomainCert struct {
	Domain        string    `json:"domain"`          // 域名
	CertPath      string    `json:"cert_path"`       // 证书路径
	KeyPath       string    `json:"key_path"`        // 私钥路径
	IssuedAt      time.Time `json:"issued_at"`       // 颁发时间
	ExpiresAt     time.Time `json:"expires_at"`      // 过期时间
	IsWildcard    bool      `json:"is_wildcard"`     // 是否是通配符证书
	LastRenewalAt time.Time `json:"last_renewal_at"` // 上次续期时间
	DNSProvider   string    `json:"dns_provider"`    // DNS提供商名称
}

// CertManagerConfig 证书管理器配置
type CertManagerConfig struct {
	CertDir string `json:"cert_dir"` // 证书存储目录
	config.SSLConfig
}

// DNSProviderConfig DNS提供商配置
type DNSProviderConfig struct {
	Name         string            `json:"name"`         // 提供商名称
	ProviderType string            `json:"providerType"` // 提供商类型
	Credentials  map[string]string `json:"credentials"`  // 验证凭证
	CreatedAt    time.Time         `json:"created_at"`   // 创建时间
	UpdatedAt    time.Time         `json:"updated_at"`   // 更新时间
}

// NewCertManager 创建新的证书管理器实例
func NewCertManager(etcdClient *etcd.EtcdClient, sched *scheduler.Scheduler) (*CertManager, error) {
	logger = common.GetLogger().GetZapLogger("aio-ssl")
	if etcdClient == nil {
		return nil, fmt.Errorf("etcdClient不能为空")
	}

	if sched == nil {
		return nil, fmt.Errorf("scheduler不能为空")
	}

	return &CertManager{
		logger:     logger,
		etcdClient: etcdClient,
		scheduler:  sched,
		domains:    make(map[string]*DomainCert),
		status:     consts.StatusNotInitialized,
		dnsConfig:  nil,
	}, nil
}

// Name 返回组件名称
func (cm *CertManager) Name() string {
	return consts.ComponentSSLManager
}

// Status 返回组件状态
func (cm *CertManager) Status() consts.ComponentStatus {
	return cm.status
}

// Init 初始化组件
func (cm *CertManager) Init(baseConfig *config.BaseConfig, cfgBody []byte) error {
	cm.logger.Info("正在初始化证书管理器...")

	cm.config = &CertManagerConfig{
		CertDir: filepath.Join(baseConfig.System.DataDir, defaultCertDir),
	}

	c := new(config.SSLConfig)
	err := json.Unmarshal(cfgBody, c)
	if err != nil {
		return err
	}
	cm.config.Email = c.Email
	cm.certDir = cm.config.CertDir

	// 确保证书目录存在
	if err := createDirIfNotExist(cm.certDir); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}

	// 从etcd加载现有证书信息
	if err := cm.loadCertsFromEtcd(context.Background()); err != nil {
		cm.logger.Warn("从etcd加载证书信息失败", zap.Error(err))
		// 继续执行，不阻止初始化
	}

	// 从etcd加载DNS提供商配置
	if err := cm.loadDNSProvidersFromEtcd(context.Background()); err != nil {
		cm.logger.Warn("从etcd加载DNS提供商配置失败", zap.Error(err))
		// 继续执行，不阻止初始化
	}

	// 初始化ACME客户端
	if err := cm.initACMEClient(); err != nil {
		return fmt.Errorf("初始化ACME客户端失败: %v", err)
	}

	cm.status = consts.StatusInitialized
	cm.logger.Info("证书管理器初始化完成")
	return nil
}

// Start 启动组件
func (cm *CertManager) Start(ctx context.Context) error {
	cm.logger.Info("正在启动证书管理器...")

	// 加载DNS配置
	if err := cm.loadDNSConfig(ctx); err != nil {
		cm.logger.Error("加载DNS配置失败", zap.Error(err))
		return err
	}

	// 设置定时任务，每天检查证书
	_, err := cm.scheduler.AddIntervalTask(
		certCheckTaskName,
		certCheckInterval,
		true, // 立即执行一次
		func(ctx context.Context) error {
			return cm.checkAndRenewCertificates(ctx)
		},
		true, // 需要分布式锁
	)

	if err != nil {
		return fmt.Errorf("设置证书检查任务失败: %v", err)
	}

	cm.status = consts.StatusRunning
	cm.logger.Info("证书管理器已启动")
	return nil
}

// Restart 重启组件
func (cm *CertManager) Restart(ctx context.Context) error {
	if err := cm.Stop(ctx); err != nil {
		return err
	}
	return cm.Start(ctx)
}

// Stop 停止组件
func (cm *CertManager) Stop(ctx context.Context) error {
	cm.logger.Info("正在停止证书管理器...")

	// 取消定时任务
	if err := cm.scheduler.CancelTask(certCheckTaskName); err != nil {
		cm.logger.Warn("取消证书检查任务失败", zap.Error(err))
		// 继续执行，不阻止停止
	}

	cm.status = consts.StatusStopped
	cm.logger.Info("证书管理器已停止")
	return nil
}

// RegisterMetadata 注册组件元数据
func (cm *CertManager) RegisterMetadata() (bool, int, map[string]string) {
	return false, 0, nil
}

// DefaultConfig 返回默认配置
func (cm *CertManager) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	return &config.SSLConfig{Email: ""}
}

// GetClientConfig 返回客户端配置
func (cm *CertManager) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}

// AddDomain 添加域名并申请证书
func (cm *CertManager) AddDomain(ctx context.Context, domain string, dnsProviderName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查域名是否已存在
	if _, exists := cm.domains[domain]; exists {
		return fmt.Errorf("域名 %s 已存在", domain)
	}

	cm.logger.Info("添加新域名", zap.String("domain", domain))

	// 检查是否为通配符域名
	isWildcard := isWildcardDomain(domain)

	// 如果还是空，则报错
	if dnsProviderName == "" {
		return fmt.Errorf("未指定DNS提供商，无法申请证书")
	}

	// 获取DNS提供商配置
	dnsProvider, err := cm.GetDNSProvider(ctx, dnsProviderName)
	if err != nil {
		return fmt.Errorf("获取DNS提供商配置失败: %v", err)
	}

	// 申请证书
	certPath, keyPath, issuedAt, expiresAt, err := cm.obtainCertificate(ctx, domain, dnsProviderName, dnsProvider.Credentials)
	if err != nil {
		return fmt.Errorf("申请证书失败: %v", err)
	}

	// 更新域名证书信息
	domainCert := &DomainCert{
		Domain:        domain,
		CertPath:      certPath,
		KeyPath:       keyPath,
		IssuedAt:      issuedAt,
		ExpiresAt:     expiresAt,
		IsWildcard:    isWildcard,
		LastRenewalAt: time.Now(),
		DNSProvider:   dnsProviderName,
	}

	cm.domains[domain] = domainCert

	// 保存到etcd
	if err := cm.saveDomainToEtcd(ctx, domainCert); err != nil {
		cm.logger.Error("保存域名信息到etcd失败", zap.String("domain", domain), zap.Error(err))
		// 不返回错误，证书已经申请成功
	}

	cm.logger.Info("域名证书已申请成功",
		zap.String("domain", domain),
		zap.Time("expires_at", expiresAt))

	return nil
}

// RemoveDomain 删除域名
func (cm *CertManager) RemoveDomain(ctx context.Context, domain string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查域名是否存在
	_, exists := cm.domains[domain]
	if !exists {
		return fmt.Errorf("域名 %s 不存在", domain)
	}

	// 从etcd中删除
	etcdKey := etcdCertPrefix + domain
	if err := cm.etcdClient.Delete(ctx, etcdKey); err != nil {
		cm.logger.Error("从etcd删除域名信息失败", zap.String("domain", domain), zap.Error(err))
		// 继续执行，不阻止删除
	}

	// 从内存中移除
	delete(cm.domains, domain)

	cm.logger.Info("域名已删除", zap.String("domain", domain))
	return nil
}

// GetDomains 获取所有管理的域名
func (cm *CertManager) GetDomains() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	domains := make([]string, 0, len(cm.domains))
	for domain := range cm.domains {
		domains = append(domains, domain)
	}

	return domains
}

// GetAllDomainCerts 获取所有域名的证书详情
func (cm *CertManager) GetAllDomainCerts() []*DomainCert {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	certs := make([]*DomainCert, 0, len(cm.domains))
	for _, cert := range cm.domains {
		certs = append(certs, cert)
	}

	return certs
}

// GetDomainCert 获取域名证书信息
func (cm *CertManager) GetDomainCert(domain string) (*DomainCert, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cert, exists := cm.domains[domain]
	if !exists {
		return nil, fmt.Errorf("域名 %s 不存在", domain)
	}

	return cert, nil
}

// 检查并续期证书
func (cm *CertManager) checkAndRenewCertificates(ctx context.Context) error {
	cm.mu.Lock()
	domains := make([]string, 0, len(cm.domains))
	for domain := range cm.domains {
		domains = append(domains, domain)
	}
	cm.mu.Unlock()

	cm.logger.Info("开始检查证书", zap.Int("domain_count", len(domains)))

	for _, domain := range domains {
		cert, err := cm.GetDomainCert(domain)
		if err != nil {
			cm.logger.Error("获取域名证书信息失败", zap.String("domain", domain), zap.Error(err))
			continue
		}

		// 检查证书是否需要续期
		if needsRenewal(cert.ExpiresAt) {
			cm.logger.Info("证书需要续期",
				zap.String("domain", domain),
				zap.Time("expires_at", cert.ExpiresAt))

			// 执行续期
			if err := cm.renewCertificate(ctx, domain); err != nil {
				cm.logger.Error("证书续期失败", zap.String("domain", domain), zap.Error(err))
				// 继续处理其他域名
			}
		}
	}

	cm.logger.Info("证书检查完成")
	return nil
}

// 从etcd加载证书信息
func (cm *CertManager) loadCertsFromEtcd(ctx context.Context) error {
	certs, err := cm.etcdClient.GetWithPrefix(ctx, etcdCertPrefix)
	if err != nil {
		return err
	}

	for key, value := range certs {
		domain := key[len(etcdCertPrefix):]
		var cert DomainCert
		if err := json.Unmarshal([]byte(value), &cert); err != nil {
			cm.logger.Error("解析域名证书信息失败", zap.String("domain", domain), zap.Error(err))
			continue
		}

		cm.domains[domain] = &cert
		cm.logger.Info("从etcd加载域名证书信息", zap.String("domain", domain))
	}

	return nil
}

// 保存域名证书信息到etcd
func (cm *CertManager) saveDomainToEtcd(ctx context.Context, cert *DomainCert) error {
	data, err := json.Marshal(cert)
	if err != nil {
		return err
	}

	key := etcdCertPrefix + cert.Domain
	return cm.etcdClient.Put(ctx, key, string(data))
}

// 初始化ACME客户端
func (cm *CertManager) initACMEClient() error {
	// 这里将在acme.go中实现
	return initACMEClient(cm.config.Email, false)
}

// 从etcd加载DNS提供商配置
func (cm *CertManager) loadDNSProvidersFromEtcd(ctx context.Context) error {
	providers, err := cm.etcdClient.GetWithPrefix(ctx, etcdDNSConfigPrefix)
	if err != nil {
		return err
	}

	cm.logger.Info("从etcd加载DNS提供商配置", zap.Int("count", len(providers)))

	return nil
}

// 添加DNS提供商配置
func (cm *CertManager) AddDNSProvider(ctx context.Context, name string, providerType string, credentials map[string]string) error {
	if name == "" {
		return fmt.Errorf("DNS提供商名称不能为空")
	}

	if providerType == "" {
		providerType = "unknown" // 设置默认值
	}

	provider := &DNSProviderConfig{
		Name:         name,
		ProviderType: providerType,
		Credentials:  credentials,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 将配置保存到etcd
	data, err := json.Marshal(provider)
	if err != nil {
		return fmt.Errorf("序列化DNS提供商配置失败: %v", err)
	}

	key := etcdDNSConfigPrefix + name
	if err := cm.etcdClient.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("保存DNS提供商配置到etcd失败: %v", err)
	}

	cm.logger.Info("DNS提供商配置已添加", zap.String("name", name), zap.String("type", providerType))
	return nil
}

// 获取DNS提供商配置
func (cm *CertManager) GetDNSProvider(ctx context.Context, name string) (*DNSProviderConfig, error) {
	if name == "" {
		return nil, fmt.Errorf("DNS提供商名称不能为空")
	}

	key := etcdDNSConfigPrefix + name
	value, err := cm.etcdClient.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("从etcd获取DNS提供商配置失败: %v", err)
	}

	if value == "" {
		return nil, fmt.Errorf("DNS提供商 %s 不存在", name)
	}

	var provider DNSProviderConfig
	if err := json.Unmarshal([]byte(value), &provider); err != nil {
		return nil, fmt.Errorf("解析DNS提供商配置失败: %v", err)
	}

	return &provider, nil
}

// 删除DNS提供商配置
func (cm *CertManager) DeleteDNSProvider(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("DNS提供商名称不能为空")
	}

	key := etcdDNSConfigPrefix + name
	if err := cm.etcdClient.Delete(ctx, key); err != nil {
		return fmt.Errorf("从etcd删除DNS提供商配置失败: %v", err)
	}

	cm.logger.Info("DNS提供商配置已删除", zap.String("name", name))
	return nil
}

// 列出所有DNS提供商
func (cm *CertManager) ListDNSProviders(ctx context.Context) ([]*DNSProviderConfig, error) {
	configs, err := cm.etcdClient.GetWithPrefix(ctx, etcdDNSConfigPrefix)
	if err != nil {
		return nil, fmt.Errorf("从etcd获取DNS提供商配置列表失败: %v", err)
	}

	providers := make([]*DNSProviderConfig, 0, len(configs))
	for _, value := range configs {
		var provider DNSProviderConfig
		if err := json.Unmarshal([]byte(value), &provider); err != nil {
			cm.logger.Error("解析DNS提供商配置失败", zap.Error(err))
			continue
		}
		providers = append(providers, &provider)
	}

	return providers, nil
}

// 申请证书
func (cm *CertManager) obtainCertificate(ctx context.Context, domain string, dnsProvider string, dnsCredentials map[string]string) (certPath, keyPath string, issuedAt, expiresAt time.Time, err error) {
	// 这里将在acme.go中实现
	return obtainCertificate(ctx, domain, cm.certDir, dnsProvider, dnsCredentials)
}

// 续期证书
func (cm *CertManager) renewCertificate(ctx context.Context, domain string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cert, exists := cm.domains[domain]
	if !exists {
		return fmt.Errorf("域名 %s 不存在", domain)
	}

	cm.logger.Info("开始续期证书", zap.String("domain", domain))

	// 获取DNS提供商配置
	dnsProvider, err := cm.GetDNSProvider(ctx, cert.DNSProvider)
	if err != nil {
		return fmt.Errorf("获取DNS提供商配置失败: %v", err)
	}

	// 申请新证书
	certPath, keyPath, issuedAt, expiresAt, err := cm.obtainCertificate(ctx, domain, cert.DNSProvider, dnsProvider.Credentials)
	if err != nil {
		return fmt.Errorf("续期证书失败: %v", err)
	}

	// 更新证书信息
	cert.CertPath = certPath
	cert.KeyPath = keyPath
	cert.IssuedAt = issuedAt
	cert.ExpiresAt = expiresAt
	cert.LastRenewalAt = time.Now()

	// 保存到etcd
	if err := cm.saveDomainToEtcd(ctx, cert); err != nil {
		cm.logger.Error("保存续期证书信息到etcd失败", zap.String("domain", domain), zap.Error(err))
		// 不返回错误，证书已经续期成功
	}

	cm.logger.Info("证书续期成功",
		zap.String("domain", domain),
		zap.Time("expires_at", expiresAt))

	return nil
}

// 检查证书是否需要续期
func needsRenewal(expiresAt time.Time) bool {
	return time.Until(expiresAt) < time.Duration(renewThresholdDays*24)*time.Hour
}

// 检查是否为通配符域名
func isWildcardDomain(domain string) bool {
	return len(domain) >= 2 && domain[0:2] == "*."
}

// 确保目录存在

// loadDNSConfig 从etcd加载DNS配置
func (cm *CertManager) loadDNSConfig(ctx context.Context) error {
	resp, err := cm.etcdClient.Client.Get(ctx, dnsConfigPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("从etcd获取DNS配置失败: %v", err)
	}

	if len(resp.Kvs) == 0 {
		cm.logger.Warn("未找到DNS配置，请先设置DNS配置")
		return nil
	}

	var config DNSConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &config); err != nil {
		return fmt.Errorf("解析DNS配置失败: %v", err)
	}

	cm.dnsConfig = &config
	cm.logger.Info("已加载DNS配置",
		zap.String("provider", config.Provider),
		zap.Time("updated_at", config.UpdatedAt))

	return nil
}

// SetDNSConfig 设置DNS配置并保存到etcd
func (cm *CertManager) SetDNSConfig(ctx context.Context, provider string, credentials map[string]string) error {
	if provider == "" {
		return fmt.Errorf("DNS提供商不能为空")
	}

	if len(credentials) == 0 {
		return fmt.Errorf("DNS凭证不能为空")
	}

	config := &DNSConfig{
		Provider:    provider,
		Credentials: credentials,
		UpdatedAt:   time.Now(),
	}

	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化DNS配置失败: %v", err)
	}

	err = cm.etcdClient.Put(ctx, dnsConfigPrefix+"config", string(data))
	if err != nil {
		return fmt.Errorf("保存DNS配置到etcd失败: %v", err)
	}

	cm.dnsConfig = config
	cm.logger.Info("已更新DNS配置", zap.String("provider", provider))

	return nil
}

// GetDNSConfig 获取当前DNS配置
func (cm *CertManager) GetDNSConfig() *DNSConfig {
	return cm.dnsConfig
}

// checkAndRenewCertificate 检查并更新证书
func (cm *CertManager) checkAndRenewCertificate(ctx context.Context, domain string, dc *DomainCert) error {
	// 检查证书是否需要续期
	if !needsRenewal(dc.ExpiresAt) {
		cm.logger.Info("证书暂不需要续期",
			zap.String("domain", domain),
			zap.Time("expires_at", dc.ExpiresAt),
			zap.Duration("remaining_time", time.Until(dc.ExpiresAt)))
		return nil
	}

	// 记录续期操作开始
	cm.logger.Info("开始续期证书",
		zap.String("domain", domain),
		zap.Time("current_expires_at", dc.ExpiresAt))

	// 确保DNS配置已设置
	if cm.dnsConfig == nil {
		cm.logger.Error("无法更新证书，DNS配置未设置", zap.String("domain", domain))
		return fmt.Errorf("DNS配置未设置，无法申请证书")
	}

	// 使用DNS验证获取证书
	certPath, keyPath, issuedAt, expiresAt, err := obtainCertificate(ctx, domain, cm.certDir, cm.dnsConfig.Provider, cm.dnsConfig.Credentials)
	if err != nil {
		cm.logger.Error("获取证书失败", zap.String("domain", domain), zap.Error(err))
		return err
	}

	// 更新证书信息
	dc.CertPath = certPath
	dc.KeyPath = keyPath
	dc.IssuedAt = issuedAt
	dc.ExpiresAt = expiresAt
	dc.LastRenewalAt = time.Now()

	// 保存到etcd
	if err := cm.saveDomainToEtcd(ctx, dc); err != nil {
		cm.logger.Error("保存续期证书信息到etcd失败", zap.String("domain", domain), zap.Error(err))
		// 不返回错误，证书已经续期成功
	}

	cm.logger.Info("证书续期成功",
		zap.String("domain", domain),
		zap.Time("new_expires_at", expiresAt),
		zap.Time("issued_at", issuedAt))

	return nil
}

// GetCertificateContent 获取证书和私钥文件内容
func (cm *CertManager) GetCertificateContent(domain string) (map[string]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cert, exists := cm.domains[domain]
	if !exists {
		return nil, fmt.Errorf("域名 %s 不存在", domain)
	}

	// 读取证书文件内容
	certContent, err := os.ReadFile(cert.CertPath)
	if err != nil {
		return nil, fmt.Errorf("读取证书文件失败: %v", err)
	}

	// 读取私钥文件内容
	keyContent, err := os.ReadFile(cert.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %v", err)
	}

	return map[string]string{
		"cert": string(certContent),
		"key":  string(keyContent),
	}, nil
}

// ConvertCertificate 将证书转换为各种服务器所需的格式
func (cm *CertManager) ConvertCertificate(ctx context.Context, domain string, format string) (map[string]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cert, exists := cm.domains[domain]
	if !exists {
		return nil, fmt.Errorf("域名 %s 不存在", domain)
	}

	// 读取证书文件内容
	certContent, err := os.ReadFile(cert.CertPath)
	if err != nil {
		return nil, fmt.Errorf("读取证书文件失败: %v", err)
	}

	// 读取私钥文件内容
	keyContent, err := os.ReadFile(cert.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %v", err)
	}

	// 准备临时文件路径基础名
	tempFileBase := filepath.Join(os.TempDir(), domain)

	// 将证书和私钥写入临时文件，用于转换
	tempCertPath := tempFileBase + ".pem"
	tempKeyPath := tempFileBase + ".key"

	if err := os.WriteFile(tempCertPath, certContent, 0600); err != nil {
		return nil, fmt.Errorf("写入临时证书文件失败: %v", err)
	}
	if err := os.WriteFile(tempKeyPath, keyContent, 0600); err != nil {
		return nil, fmt.Errorf("写入临时私钥文件失败: %v", err)
	}

	// 根据请求的格式进行相应的转换
	result := map[string]string{
		"cert": string(certContent),
		"key":  string(keyContent),
	}

	switch format {
	case "nginx":
		// Nginx 直接使用 PEM 格式
		result["config"] = getNginxConfig(domain, cert.CertPath, cert.KeyPath)
	case "apache":
		// Apache 直接使用 PEM 格式
		result["config"] = getApacheConfig(domain, cert.CertPath, cert.KeyPath)
	case "pkcs12":
		// 转换为 PKCS12 格式
		pkcs12Path := tempFileBase + ".p12"
		pkcs12Password := "changeit" // 使用默认密码，实际应用中可让用户设置

		if err := convertToPKCS12(tempCertPath, tempKeyPath, pkcs12Path, pkcs12Password); err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath)
			return nil, fmt.Errorf("转换为PKCS12格式失败: %v", err)
		}

		// 读取生成的 PKCS12 文件
		pkcs12Content, err := os.ReadFile(pkcs12Path)
		if err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath, pkcs12Path)
			return nil, fmt.Errorf("读取PKCS12文件失败: %v", err)
		}

		// Base64 编码 PKCS12 内容
		result["pkcs12"] = base64.StdEncoding.EncodeToString(pkcs12Content)
		result["password"] = pkcs12Password
		result["config"] = getTomcatConfig(domain)

		cleanupTempFiles(tempCertPath, tempKeyPath, pkcs12Path)
	case "jks":
		// 先转换为 PKCS12，再转为 JKS
		pkcs12Path := tempFileBase + ".p12"
		jksPath := tempFileBase + ".jks"
		password := "changeit" // 使用默认密码

		if err := convertToPKCS12(tempCertPath, tempKeyPath, pkcs12Path, password); err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath)
			return nil, fmt.Errorf("转换为PKCS12格式失败: %v", err)
		}

		if err := convertPKCS12ToJKS(pkcs12Path, jksPath, password, domain); err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath, pkcs12Path)
			return nil, fmt.Errorf("转换为JKS格式失败: %v", err)
		}

		// 读取生成的 JKS 文件
		jksContent, err := os.ReadFile(jksPath)
		if err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath, pkcs12Path, jksPath)
			return nil, fmt.Errorf("读取JKS文件失败: %v", err)
		}

		// Base64 编码 JKS 内容
		result["jks"] = base64.StdEncoding.EncodeToString(jksContent)
		result["password"] = password
		result["config"] = getJKSConfig(domain)

		cleanupTempFiles(tempCertPath, tempKeyPath, pkcs12Path, jksPath)
	case "iis":
		// 转换为 PFX 格式 (与 PKCS12 相同，但命名为 .pfx)
		pfxPath := tempFileBase + ".pfx"
		pfxPassword := "changeit" // 使用默认密码

		if err := convertToPKCS12(tempCertPath, tempKeyPath, pfxPath, pfxPassword); err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath)
			return nil, fmt.Errorf("转换为PFX格式失败: %v", err)
		}

		// 读取生成的 PFX 文件
		pfxContent, err := os.ReadFile(pfxPath)
		if err != nil {
			cleanupTempFiles(tempCertPath, tempKeyPath, pfxPath)
			return nil, fmt.Errorf("读取PFX文件失败: %v", err)
		}

		// Base64 编码 PFX 内容
		result["pfx"] = base64.StdEncoding.EncodeToString(pfxContent)
		result["password"] = pfxPassword
		result["config"] = getIISConfig(domain)

		cleanupTempFiles(tempCertPath, tempKeyPath, pfxPath)
	default:
		cleanupTempFiles(tempCertPath, tempKeyPath)
		return nil, fmt.Errorf("不支持的证书格式: %s", format)
	}

	// 清理临时文件
	cleanupTempFiles(tempCertPath, tempKeyPath)
	return result, nil
}

// 辅助函数：转换证书为PKCS12格式
func convertToPKCS12(certPath, keyPath, outputPath, password string) error {
	cmd := exec.Command("openssl", "pkcs12", "-export", "-in", certPath, "-inkey", keyPath,
		"-out", outputPath, "-passout", "pass:"+password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行openssl命令失败: %v, 输出: %s", err, output)
	}
	return nil
}

// 辅助函数：转换PKCS12为JKS格式
func convertPKCS12ToJKS(pkcs12Path, jksPath, password, alias string) error {
	cmd := exec.Command("keytool", "-importkeystore", "-srckeystore", pkcs12Path,
		"-srcstoretype", "PKCS12", "-srcstorepass", password,
		"-destkeystore", jksPath, "-deststoretype", "JKS",
		"-deststorepass", password, "-destkeypass", password,
		"-srcalias", "1", "-destalias", alias)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行keytool命令失败: %v, 输出: %s", err, output)
	}
	return nil
}

// 辅助函数：清理临时文件
func cleanupTempFiles(filePaths ...string) {
	for _, path := range filePaths {
		if path != "" {
			os.Remove(path)
		}
	}
}

// 生成Nginx配置
func getNginxConfig(domain, certPath, keyPath string) string {
	return fmt.Sprintf(`# Nginx SSL配置示例
server {
    listen 443 ssl;
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_session_timeout 5m;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:HIGH:!aNULL:!MD5:!RC4:!DHE;
    ssl_prefer_server_ciphers on;
    
    # 其他配置...
    
    location / {
        root /usr/share/nginx/html;
        index index.html index.htm;
    }
}`, domain, certPath, keyPath)
}

// 生成Apache配置
func getApacheConfig(domain, certPath, keyPath string) string {
	return fmt.Sprintf(`# Apache SSL配置示例
<VirtualHost *:443>
    ServerName %s
    
    SSLEngine on
    SSLCertificateFile %s
    SSLCertificateKeyFile %s
    
    # 设置强加密套件
    SSLProtocol all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1
    SSLHonorCipherOrder on
    SSLCipherSuite EECDH+AESGCM:EDH+AESGCM
    
    # 其他配置...
    
    DocumentRoot /var/www/html
    <Directory /var/www/html>
        Options Indexes FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>
</VirtualHost>`, domain, certPath, keyPath)
}

// 生成Tomcat配置
func getTomcatConfig(domain string) string {
	return fmt.Sprintf(`# Tomcat SSL配置示例
# 将下载的 .p12 文件放置在安全的位置

# 在Tomcat的server.xml中配置
<Connector
    port="8443" protocol="org.apache.coyote.http11.Http11NioProtocol"
    maxThreads="150" SSLEnabled="true">
    <SSLHostConfig>
        <Certificate
            certificateKeystoreFile="/path/to/%s.p12"
            certificateKeystorePassword="changeit"
            certificateKeystoreType="PKCS12"
            type="RSA" />
    </SSLHostConfig>
</Connector>`, domain)
}

// 生成IIS配置
func getIISConfig(domain string) string {
	return fmt.Sprintf(`# IIS SSL配置说明
# 1. 下载 .pfx 文件到您的IIS服务器上
# 2. 在IIS管理器中导入PFX证书:
#    - 打开IIS管理器
#    - 选择服务器 > "服务器证书"
#    - 点击"导入"操作，选择下载的 .pfx 文件
#    - 输入密码: changeit
#    - 在网站绑定中，添加类型为https的绑定，并选择导入的证书

# PowerShell导入证书示例:
$securePassword = ConvertTo-SecureString -String "changeit" -Force -AsPlainText
Import-PfxCertificate -FilePath C:\path\to\%s.pfx -CertStoreLocation Cert:\LocalMachine\My -Password $securePassword

# 完成上述步骤后，您的IIS服务器即可使用该证书提供HTTPS服务。`, domain)
}

// 生成JKS配置
func getJKSConfig(domain string) string {
	return fmt.Sprintf(`# Java应用程序SSL配置说明
# 1. 下载 .jks 文件到您的Java应用服务器上

# 2. Java应用程序示例配置:
System.setProperty("javax.net.ssl.keyStore", "/path/to/%s.jks");
System.setProperty("javax.net.ssl.keyStorePassword", "changeit");
System.setProperty("javax.net.ssl.trustStore", "/path/to/truststore.jks");
System.setProperty("javax.net.ssl.trustStorePassword", "changeit");

# 3. Spring Boot配置示例:
# server.ssl.key-store=/path/to/%s.jks
# server.ssl.key-store-password=changeit
# server.ssl.key-store-type=JKS
# server.ssl.key-alias=%s`, domain, domain, domain)
}
