package certmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/internal/etcd"

	"github.com/xsxdot/aio/pkg/common"

	"encoding/base64"

	"github.com/xsxdot/aio/app/config"

	"github.com/xsxdot/aio/pkg/scheduler"
	"go.uber.org/zap"
)

const (
	// etcd前缀，用于存储证书数据
	etcdCertPrefix = "/aio/certs/"
	// etcd前缀，用于存储DNS提供商配置
	etcdDNSConfigPrefix = "/aio/dns-providers/"
	// etcd前缀，用于存储部署配置
	etcdDeployConfigPrefix = "/aio/deploy-configs/"
	// 本地证书保存目录
	defaultCertDir = "ssl"
	// 证书检查任务名称
	certCheckTaskName = "cert_check_task"
	// 证书检查周期
	certCheckInterval = 24 * time.Hour
	// 证书续期阈值（天数）
	renewThresholdDays = 10
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
	config     *config.SSLConfig
	etcdClient *etcd.EtcdClient
	scheduler  *scheduler.Scheduler
	certDir    string // 证书存储目录
	mu         sync.RWMutex
	// 移除内存缓存字段
	// domains       map[string]*DomainCert // 管理的域名及其证书状态
	// dnsConfig     *DNSConfig             // dnsConfig存储当前使用的DNS配置
	// deployConfigs map[string]*DeployConfig // deployConfigs存储部署配置
	// certCheckTaskID 存储证书检查任务的ID
	certCheckTaskID string
}

// DomainCert 表示域名的证书信息
type DomainCert struct {
	Domain        string    `json:"domain"`          // 域名
	CertPath      string    `json:"cert_path"`       // 证书路径（保留用于兼容性）
	KeyPath       string    `json:"key_path"`        // 私钥路径（保留用于兼容性）
	CertContent   string    `json:"cert_content"`    // 证书内容（PEM格式）
	KeyContent    string    `json:"key_content"`     // 私钥内容（PEM格式）
	IssuedAt      time.Time `json:"issued_at"`       // 颁发时间
	ExpiresAt     time.Time `json:"expires_at"`      // 过期时间
	IsWildcard    bool      `json:"is_wildcard"`     // 是否是通配符证书
	LastRenewalAt time.Time `json:"last_renewal_at"` // 上次续期时间
	DNSProvider   string    `json:"dns_provider"`    // DNS提供商名称
}

// DNSProviderConfig DNS提供商配置
type DNSProviderConfig struct {
	Name         string            `json:"name"`         // 提供商名称
	ProviderType string            `json:"providerType"` // 提供商类型
	Credentials  map[string]string `json:"credentials"`  // 验证凭证
	CreatedAt    time.Time         `json:"created_at"`   // 创建时间
	UpdatedAt    time.Time         `json:"updated_at"`   // 更新时间
}

// DeployType 部署类型枚举
type DeployType string

const (
	DeployTypeLocal     DeployType = "local"  // 本地文件部署
	DeployTypeRemote    DeployType = "remote" // 远程服务器部署
	DeployTypeAliyunCDN DeployType = "aliyun" // 阿里云CDN部署
)

// DeployConfig 部署配置
type DeployConfig struct {
	ID              string        `json:"id"`                      // 配置ID
	Name            string        `json:"name"`                    // 配置名称
	Domain          string        `json:"domain"`                  // 关联的域名
	Type            DeployType    `json:"type"`                    // 部署类型
	Enabled         bool          `json:"enabled"`                 // 是否启用
	AutoDeploy      bool          `json:"auto_deploy"`             // 是否自动部署
	LocalConfig     *LocalConfig  `json:"local_config,omitempty"`  // 本地部署配置
	RemoteConfig    *RemoteConfig `json:"remote_config,omitempty"` // 远程服务器配置
	AliyunConfig    *AliyunConfig `json:"aliyun_config,omitempty"` // 阿里云配置
	CreatedAt       time.Time     `json:"created_at"`              // 创建时间
	UpdatedAt       time.Time     `json:"updated_at"`              // 更新时间
	LastDeployAt    time.Time     `json:"last_deploy_at"`          // 上次部署时间
	LastDeployError string        `json:"last_deploy_error"`       // 上次部署错误信息
}

// LocalConfig 本地文件部署配置
type LocalConfig struct {
	CertPath           string   `json:"cert_path"`            // 证书文件绝对路径
	KeyPath            string   `json:"key_path"`             // 私钥文件绝对路径
	PostDeployCommands []string `json:"post_deploy_commands"` // 部署后执行的命令数组
}

// RemoteConfig 远程服务器部署配置
type RemoteConfig struct {
	Host               string   `json:"host"`                 // 服务器地址
	Port               int      `json:"port"`                 // SSH端口
	Username           string   `json:"username"`             // 用户名
	Password           string   `json:"password"`             // 密码（可选）
	PrivateKey         string   `json:"private_key"`          // SSH私钥（可选）
	CertPath           string   `json:"cert_path"`            // 远程证书文件路径
	KeyPath            string   `json:"key_path"`             // 远程私钥文件路径
	PostDeployCommands []string `json:"post_deploy_commands"` // 部署后执行的命令数组
}

// AliyunConfig 阿里云CDN部署配置
type AliyunConfig struct {
	AccessKeyID     string `json:"access_key_id"`     // 阿里云AccessKey ID
	AccessKeySecret string `json:"access_key_secret"` // 阿里云AccessKey Secret
	TargetDomain    string `json:"target_domain"`     // 目标域名（针对CDN）
}

// NewCertManager 创建新的证书管理器实例
func NewCertManager(cfg *config.BaseConfig, etcdClient *etcd.EtcdClient, sched *scheduler.Scheduler) (*CertManager, error) {
	logger = common.GetLogger().GetZapLogger("aio-ssl")
	if etcdClient == nil {
		return nil, fmt.Errorf("etcdClient不能为空")
	}

	if sched == nil {
		return nil, fmt.Errorf("scheduler不能为空")
	}

	c := &CertManager{
		logger:          logger,
		config:          cfg.SSL,
		etcdClient:      etcdClient,
		scheduler:       sched,
		certDir:         filepath.Join(cfg.System.DataDir, cfg.SSL.CertDir),
		certCheckTaskID: "",
	}
	return c, c.init()
}

// Init 初始化组件
func (cm *CertManager) init() error {
	cm.logger.Info("正在初始化证书管理器...")

	// 确保证书目录存在
	if err := createDirIfNotExist(cm.certDir); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}

	// 移除从etcd加载数据到内存缓存的逻辑，改为直接从etcd读取
	// 原本的加载逻辑已移除，数据将在需要时直接从etcd获取

	// 初始化ACME客户端
	if err := cm.initACMEClient(); err != nil {
		return fmt.Errorf("初始化ACME客户端失败: %v", err)
	}

	cm.logger.Info("证书管理器初始化完成")
	return nil
}

// Start 启动组件
func (cm *CertManager) Start(ctx context.Context) error {
	cm.logger.Info("正在启动证书管理器...")

	// 移除DNS配置加载逻辑，改为在需要时直接从etcd获取

	// 创建证书检查定时任务（每天检查一次）
	certCheckTask := scheduler.NewIntervalTask(
		certCheckTaskName,
		time.Now().Add(time.Minute),          // 1分钟后开始执行
		certCheckInterval,                    // 每24小时执行一次
		scheduler.TaskExecuteModeDistributed, // 分布式任务，只有领导者执行
		10*time.Minute,                       // 10分钟超时
		func(ctx context.Context) error {
			return cm.checkAndRenewCertificates(ctx)
		},
	)

	// 添加任务到调度器
	if err := cm.scheduler.AddTask(certCheckTask); err != nil {
		return fmt.Errorf("设置证书检查任务失败: %v", err)
	}

	// 保存任务ID以便后续取消
	cm.certCheckTaskID = certCheckTask.GetID()

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
	if cm.certCheckTaskID != "" {
		if removed := cm.scheduler.RemoveTask(cm.certCheckTaskID); !removed {
			cm.logger.Warn("取消证书检查任务失败", zap.String("task_id", cm.certCheckTaskID))
		} else {
			cm.logger.Info("证书检查任务已取消", zap.String("task_id", cm.certCheckTaskID))
		}
		cm.certCheckTaskID = ""
	}

	cm.logger.Info("证书管理器已停止")
	return nil
}

func (cm *CertManager) AddDomain(ctx context.Context, domain string, dnsProviderName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查域名是否已存在 - 直接从etcd查询
	etcdKey := etcdCertPrefix + domain
	existingValue, err := cm.etcdClient.Get(ctx, etcdKey)
	if err != nil {
		// 检查是否是键不存在的错误，这是正常的（我们要添加新域名）
		if !strings.Contains(err.Error(), "键不存在") {
			return fmt.Errorf("检查域名是否存在失败: %v", err)
		}
		// 键不存在是正常的，继续执行
	} else if existingValue != "" {
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

	// 申请证书并获取内容
	certPath, keyPath, certContent, keyContent, issuedAt, expiresAt, err := cm.obtainCertificateWithContent(ctx, domain, dnsProvider.ProviderType, dnsProvider.Credentials)
	if err != nil {
		return fmt.Errorf("申请证书失败: %v", err)
	}

	// 创建域名证书信息
	domainCert := &DomainCert{
		Domain:        domain,
		CertPath:      certPath,
		KeyPath:       keyPath,
		CertContent:   certContent,
		KeyContent:    keyContent,
		IssuedAt:      issuedAt,
		ExpiresAt:     expiresAt,
		IsWildcard:    isWildcard,
		LastRenewalAt: time.Now(),
		DNSProvider:   dnsProviderName,
	}

	// 直接保存到etcd，不再维护内存缓存
	if err := cm.saveDomainToEtcd(ctx, domainCert); err != nil {
		return fmt.Errorf("保存域名信息到etcd失败: %v", err)
	}

	cm.logger.Info("域名证书已申请成功",
		zap.String("domain", domain),
		zap.Time("expires_at", expiresAt))

	// 执行自动部署
	go cm.autoDeployAfterRenewal(ctx, domain)

	return nil
}

// RemoveDomain 删除域名
func (cm *CertManager) RemoveDomain(ctx context.Context, domain string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查域名是否存在 - 直接从etcd查询
	etcdKey := etcdCertPrefix + domain
	existingValue, err := cm.etcdClient.Get(ctx, etcdKey)
	if err != nil {
		return fmt.Errorf("检查域名是否存在失败: %v", err)
	}
	if existingValue == "" {
		return fmt.Errorf("域名 %s 不存在", domain)
	}

	// 从etcd中删除
	if err := cm.etcdClient.Delete(ctx, etcdKey); err != nil {
		return fmt.Errorf("从etcd删除域名信息失败: %v", err)
	}

	cm.logger.Info("域名已删除", zap.String("domain", domain))
	return nil
}

// GetDomains 获取所有管理的域名
func (cm *CertManager) GetDomains() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 直接从etcd获取所有域名
	certs, err := cm.etcdClient.GetWithPrefix(context.Background(), etcdCertPrefix)
	if err != nil {
		cm.logger.Error("从etcd获取域名列表失败", zap.Error(err))
		return []string{}
	}

	domains := make([]string, 0, len(certs))
	for key := range certs {
		domain := key[len(etcdCertPrefix):]
		domains = append(domains, domain)
	}

	return domains
}

// GetAllDomainCerts 获取所有域名的证书详情
func (cm *CertManager) GetAllDomainCerts() []*DomainCert {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 直接从etcd获取所有证书信息
	certs, err := cm.etcdClient.GetWithPrefix(context.Background(), etcdCertPrefix)
	if err != nil {
		cm.logger.Error("从etcd获取证书列表失败", zap.Error(err))
		return []*DomainCert{}
	}

	domainCerts := make([]*DomainCert, 0, len(certs))
	for key, value := range certs {
		domain := key[len(etcdCertPrefix):]
		var cert DomainCert
		if err := json.Unmarshal([]byte(value), &cert); err != nil {
			cm.logger.Error("解析域名证书信息失败", zap.String("domain", domain), zap.Error(err))
			continue
		}
		domainCerts = append(domainCerts, &cert)
	}

	return domainCerts
}

// GetDomainCert 获取域名证书信息
func (cm *CertManager) GetDomainCert(domain string) (*DomainCert, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 直接从etcd获取域名证书信息
	etcdKey := etcdCertPrefix + domain
	value, err := cm.etcdClient.Get(context.Background(), etcdKey)
	if err != nil {
		// 检查是否是键不存在的错误
		if strings.Contains(err.Error(), "键不存在") {
			return nil, fmt.Errorf("域名 %s 不存在", domain)
		}
		return nil, fmt.Errorf("从etcd获取域名证书信息失败: %v", err)
	}

	if value == "" {
		return nil, fmt.Errorf("域名 %s 不存在", domain)
	}

	var cert DomainCert
	if err := json.Unmarshal([]byte(value), &cert); err != nil {
		return nil, fmt.Errorf("解析域名证书信息失败: %v", err)
	}

	return &cert, nil
}

// 检查并续期证书
func (cm *CertManager) checkAndRenewCertificates(ctx context.Context) error {
	// 直接从etcd获取所有域名
	domains := cm.GetDomains()

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
		// 检查是否是键不存在的错误
		if strings.Contains(err.Error(), "键不存在") {
			return nil, fmt.Errorf("DNS提供商 %s 不存在", name)
		}
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

// obtainCertificateWithContent 申请证书并返回内容
func (cm *CertManager) obtainCertificateWithContent(ctx context.Context, domain string, dnsProvider string, dnsCredentials map[string]string) (certPath, keyPath, certContent, keyContent string, issuedAt, expiresAt time.Time, err error) {
	// 调用acme.go中的实现获取证书文件路径
	certPath, keyPath, issuedAt, expiresAt, err = obtainCertificate(ctx, domain, cm.certDir, dnsProvider, dnsCredentials)
	if err != nil {
		return "", "", "", "", time.Time{}, time.Time{}, err
	}

	// 读取证书内容
	certContentBytes, err := os.ReadFile(certPath)
	if err != nil {
		return "", "", "", "", time.Time{}, time.Time{}, fmt.Errorf("读取证书文件失败: %v", err)
	}

	// 读取私钥内容
	keyContentBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return "", "", "", "", time.Time{}, time.Time{}, fmt.Errorf("读取私钥文件失败: %v", err)
	}

	return certPath, keyPath, string(certContentBytes), string(keyContentBytes), issuedAt, expiresAt, nil
}

// 申请证书（保持向后兼容）
func (cm *CertManager) obtainCertificate(ctx context.Context, domain string, dnsProvider string, dnsCredentials map[string]string) (certPath, keyPath string, issuedAt, expiresAt time.Time, err error) {
	// 这里将在acme.go中实现
	return obtainCertificate(ctx, domain, cm.certDir, dnsProvider, dnsCredentials)
}

// 续期证书
func (cm *CertManager) renewCertificate(ctx context.Context, domain string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 直接从etcd获取域名证书信息
	cert, err := cm.GetDomainCert(domain)
	if err != nil {
		return fmt.Errorf("获取域名 %s 证书信息失败: %v", domain, err)
	}

	cm.logger.Info("开始续期证书", zap.String("domain", domain))

	// 获取DNS提供商配置
	dnsProvider, err := cm.GetDNSProvider(ctx, cert.DNSProvider)
	if err != nil {
		return fmt.Errorf("获取DNS提供商配置失败: %v", err)
	}

	// 申请新证书并获取内容
	certPath, keyPath, certContent, keyContent, issuedAt, expiresAt, err := cm.obtainCertificateWithContent(ctx, domain, dnsProvider.ProviderType, dnsProvider.Credentials)
	if err != nil {
		return fmt.Errorf("续期证书失败: %v", err)
	}

	// 更新证书信息
	cert.CertPath = certPath
	cert.KeyPath = keyPath
	cert.CertContent = certContent
	cert.KeyContent = keyContent
	cert.IssuedAt = issuedAt
	cert.ExpiresAt = expiresAt
	cert.LastRenewalAt = time.Now()

	// 保存到etcd
	if err := cm.saveDomainToEtcd(ctx, cert); err != nil {
		return fmt.Errorf("保存续期证书信息到etcd失败: %v", err)
	}

	cm.logger.Info("证书续期成功",
		zap.String("domain", domain),
		zap.Time("new_expires_at", expiresAt),
		zap.Time("issued_at", issuedAt))

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

// 检查并更新证书
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

	// 获取DNS配置
	dnsConfig, err := cm.GetDNSConfig(ctx)
	if err != nil {
		cm.logger.Error("无法更新证书，DNS配置获取失败", zap.String("domain", domain), zap.Error(err))
		return fmt.Errorf("DNS配置获取失败，无法申请证书: %v", err)
	}

	// 使用DNS验证获取证书
	certPath, keyPath, issuedAt, expiresAt, err := obtainCertificate(ctx, domain, cm.certDir, dnsConfig.Provider, dnsConfig.Credentials)
	if err != nil {
		cm.logger.Error("获取证书失败", zap.String("domain", domain), zap.Error(err))
		return err
	}

	// 读取证书内容
	certContent, err := os.ReadFile(certPath)
	if err != nil {
		cm.logger.Error("读取证书文件失败", zap.String("domain", domain), zap.Error(err))
		return fmt.Errorf("读取证书文件失败: %v", err)
	}

	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		cm.logger.Error("读取私钥文件失败", zap.String("domain", domain), zap.Error(err))
		return fmt.Errorf("读取私钥文件失败: %v", err)
	}

	// 更新证书信息
	dc.CertPath = certPath
	dc.KeyPath = keyPath
	dc.CertContent = string(certContent)
	dc.KeyContent = string(keyContent)
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

	// 执行自动部署
	go cm.autoDeployAfterRenewal(ctx, domain)

	return nil
}

// GetCertificateContent 获取证书和私钥文件内容
func (cm *CertManager) GetCertificateContent(domain string) (map[string]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 直接从etcd获取域名证书信息
	cert, err := cm.GetDomainCert(domain)
	if err != nil {
		return nil, fmt.Errorf("获取域名 %s 证书信息失败: %v", domain, err)
	}

	// 优先使用存储在etcd中的证书内容
	if cert.CertContent != "" && cert.KeyContent != "" {
		return map[string]string{
			"cert": cert.CertContent,
			"key":  cert.KeyContent,
		}, nil
	}

	// 兼容性处理：如果etcd中没有证书内容，尝试从文件读取
	if cert.CertPath != "" && cert.KeyPath != "" {
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

	return nil, fmt.Errorf("域名 %s 没有可用的证书内容", domain)
}

// ConvertCertificate 将证书转换为各种服务器所需的格式
func (cm *CertManager) ConvertCertificate(ctx context.Context, domain string, format string) (map[string]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 直接从etcd获取域名证书信息
	cert, err := cm.GetDomainCert(domain)
	if err != nil {
		return nil, fmt.Errorf("获取域名 %s 证书信息失败: %v", domain, err)
	}

	// 获取证书内容
	var certContent, keyContent []byte

	// 优先使用存储在etcd中的证书内容
	if cert.CertContent != "" && cert.KeyContent != "" {
		certContent = []byte(cert.CertContent)
		keyContent = []byte(cert.KeyContent)
	} else if cert.CertPath != "" && cert.KeyPath != "" {
		// 兼容性处理：从文件读取
		var err error
		certContent, err = os.ReadFile(cert.CertPath)
		if err != nil {
			return nil, fmt.Errorf("读取证书文件失败: %v", err)
		}

		keyContent, err = os.ReadFile(cert.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥文件失败: %v", err)
		}
	} else {
		return nil, fmt.Errorf("域名 %s 没有可用的证书内容", domain)
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
    SSLCipherSuite EECDH+AESGCM
    
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

// ================ 部署配置管理 ================

// AddDeployConfig 添加部署配置
func (cm *CertManager) AddDeployConfig(ctx context.Context, config *DeployConfig) error {
	if config.ID == "" {
		return fmt.Errorf("部署配置ID不能为空")
	}

	if config.Name == "" {
		return fmt.Errorf("部署配置名称不能为空")
	}

	if config.Domain == "" {
		return fmt.Errorf("关联域名不能为空")
	}

	// 验证部署类型配置
	if err := cm.validateDeployConfig(config); err != nil {
		return fmt.Errorf("部署配置验证失败: %v", err)
	}

	// 检查配置是否已存在 - 直接从etcd查询
	etcdKey := etcdDeployConfigPrefix + config.ID
	existingValue, err := cm.etcdClient.Get(ctx, etcdKey)
	if err != nil {
		// 检查是否是键不存在的错误，这是正常的（我们要添加新配置）
		if !strings.Contains(err.Error(), "键不存在") {
			return fmt.Errorf("检查配置是否存在失败: %v", err)
		}
		// 键不存在是正常的，继续执行
	} else if existingValue != "" {
		return fmt.Errorf("部署配置 %s 已存在", config.ID)
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 直接保存到etcd，不再维护内存缓存
	if err := cm.saveDeployConfigToEtcd(ctx, config); err != nil {
		return fmt.Errorf("保存部署配置到etcd失败: %v", err)
	}

	cm.logger.Info("部署配置已添加",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("domain", config.Domain),
		zap.String("type", string(config.Type)))

	return nil
}

// UpdateDeployConfig 更新部署配置
func (cm *CertManager) UpdateDeployConfig(ctx context.Context, config *DeployConfig) error {
	if config.ID == "" {
		return fmt.Errorf("部署配置ID不能为空")
	}

	// 验证部署类型配置
	if err := cm.validateDeployConfig(config); err != nil {
		return fmt.Errorf("部署配置验证失败: %v", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查配置是否存在 - 直接从etcd查询
	existingConfig, err := cm.GetDeployConfig(config.ID)
	if err != nil {
		return fmt.Errorf("部署配置 %s 不存在: %v", config.ID, err)
	}

	// 保留创建时间，更新修改时间
	config.CreatedAt = existingConfig.CreatedAt
	config.UpdatedAt = time.Now()

	// 直接保存到etcd，不再维护内存缓存
	if err := cm.saveDeployConfigToEtcd(ctx, config); err != nil {
		return fmt.Errorf("更新部署配置到etcd失败: %v", err)
	}

	cm.logger.Info("部署配置已更新",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("domain", config.Domain))

	return nil
}

// DeleteDeployConfig 删除部署配置
func (cm *CertManager) DeleteDeployConfig(ctx context.Context, configID string) error {
	if configID == "" {
		return fmt.Errorf("部署配置ID不能为空")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查配置是否存在 - 直接从etcd查询
	config, err := cm.GetDeployConfig(configID)
	if err != nil {
		return fmt.Errorf("部署配置 %s 不存在: %v", configID, err)
	}

	// 从etcd删除
	etcdKey := etcdDeployConfigPrefix + configID
	if err := cm.etcdClient.Delete(ctx, etcdKey); err != nil {
		return fmt.Errorf("从etcd删除部署配置失败: %v", err)
	}

	cm.logger.Info("部署配置已删除",
		zap.String("id", configID),
		zap.String("name", config.Name))

	return nil
}

// GetDeployConfig 获取部署配置
func (cm *CertManager) GetDeployConfig(configID string) (*DeployConfig, error) {
	if configID == "" {
		return nil, fmt.Errorf("部署配置ID不能为空")
	}

	// 直接从etcd获取部署配置
	etcdKey := etcdDeployConfigPrefix + configID
	value, err := cm.etcdClient.Get(context.Background(), etcdKey)
	if err != nil {
		// 检查是否是键不存在的错误
		if strings.Contains(err.Error(), "键不存在") {
			return nil, fmt.Errorf("部署配置 %s 不存在", configID)
		}
		return nil, fmt.Errorf("从etcd获取部署配置失败: %v", err)
	}

	if value == "" {
		return nil, fmt.Errorf("部署配置 %s 不存在", configID)
	}

	var config DeployConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, fmt.Errorf("解析部署配置失败: %v", err)
	}

	return &config, nil
}

// ListDeployConfigs 列出所有部署配置
func (cm *CertManager) ListDeployConfigs() []*DeployConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 直接从etcd获取所有部署配置
	configs, err := cm.etcdClient.GetWithPrefix(context.Background(), etcdDeployConfigPrefix)
	if err != nil {
		cm.logger.Error("从etcd获取部署配置列表失败", zap.Error(err))
		return []*DeployConfig{}
	}

	deployConfigs := make([]*DeployConfig, 0, len(configs))
	for key, value := range configs {
		configID := key[len(etcdDeployConfigPrefix):]
		var config DeployConfig
		if err := json.Unmarshal([]byte(value), &config); err != nil {
			cm.logger.Error("解析部署配置失败", zap.String("config_id", configID), zap.Error(err))
			continue
		}
		deployConfigs = append(deployConfigs, &config)
	}

	return deployConfigs
}

// ListDeployConfigsByDomain 根据域名列出部署配置
func (cm *CertManager) ListDeployConfigsByDomain(domain string) []*DeployConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 获取所有部署配置
	allConfigs := cm.ListDeployConfigs()

	configs := make([]*DeployConfig, 0)
	for _, config := range allConfigs {
		if config.Domain == domain || (isWildcardDomain(config.Domain) && matchWildcardDomain(config.Domain, domain)) {
			configs = append(configs, config)
		}
	}

	return configs
}

// DeployCertificate 手动部署证书
func (cm *CertManager) DeployCertificate(ctx context.Context, configID string) error {
	config, err := cm.GetDeployConfig(configID)
	if err != nil {
		return err
	}

	if !config.Enabled {
		return fmt.Errorf("部署配置 %s 已禁用", configID)
	}

	return cm.deployCertificateWithConfig(ctx, config)
}

// deployCertificateWithConfig 使用指定配置部署证书
func (cm *CertManager) deployCertificateWithConfig(ctx context.Context, config *DeployConfig) error {
	// 获取域名证书
	cert, err := cm.GetDomainCert(config.Domain)
	if err != nil {
		return fmt.Errorf("获取域名证书失败: %v", err)
	}

	cm.logger.Info("开始部署证书",
		zap.String("config_id", config.ID),
		zap.String("config_name", config.Name),
		zap.String("domain", config.Domain),
		zap.String("type", string(config.Type)))

	var deployErr error

	// 根据部署类型执行部署
	switch config.Type {
	case DeployTypeLocal:
		deployErr = cm.deployToLocal(ctx, cert, config.LocalConfig)
	case DeployTypeRemote:
		deployErr = cm.deployToRemote(ctx, cert, config.RemoteConfig)
	case DeployTypeAliyunCDN:
		deployErr = cm.deployToAliyunCDN(ctx, cert, config.AliyunConfig)

	default:
		deployErr = fmt.Errorf("不支持的部署类型: %s", config.Type)
	}

	// 更新部署状态
	cm.mu.Lock()
	config.LastDeployAt = time.Now()
	if deployErr != nil {
		config.LastDeployError = deployErr.Error()
		cm.logger.Error("证书部署失败",
			zap.String("config_id", config.ID),
			zap.String("domain", config.Domain),
			zap.Error(deployErr))
	} else {
		config.LastDeployError = ""
		cm.logger.Info("证书部署成功",
			zap.String("config_id", config.ID),
			zap.String("domain", config.Domain))
	}
	cm.mu.Unlock()

	// 保存更新后的配置到etcd
	if err := cm.saveDeployConfigToEtcd(ctx, config); err != nil {
		cm.logger.Error("保存部署状态到etcd失败", zap.Error(err))
	}

	return deployErr
}

// validateDeployConfig 验证部署配置
func (cm *CertManager) validateDeployConfig(config *DeployConfig) error {
	switch config.Type {
	case DeployTypeLocal:
		if config.LocalConfig == nil {
			return fmt.Errorf("本地部署配置不能为空")
		}
		if config.LocalConfig.CertPath == "" || config.LocalConfig.KeyPath == "" {
			return fmt.Errorf("本地部署的证书和私钥路径不能为空")
		}
		if !filepath.IsAbs(config.LocalConfig.CertPath) || !filepath.IsAbs(config.LocalConfig.KeyPath) {
			return fmt.Errorf("本地部署的路径必须是绝对路径")
		}
	case DeployTypeRemote:
		if config.RemoteConfig == nil {
			return fmt.Errorf("远程部署配置不能为空")
		}
		if config.RemoteConfig.Host == "" || config.RemoteConfig.Username == "" {
			return fmt.Errorf("远程部署的主机和用户名不能为空")
		}
		if config.RemoteConfig.Password == "" && config.RemoteConfig.PrivateKey == "" {
			return fmt.Errorf("远程部署必须提供密码或私钥")
		}
		if config.RemoteConfig.CertPath == "" || config.RemoteConfig.KeyPath == "" {
			return fmt.Errorf("远程部署的证书和私钥路径不能为空")
		}
		if config.RemoteConfig.Port <= 0 {
			config.RemoteConfig.Port = 22 // 设置默认SSH端口
		}
	case DeployTypeAliyunCDN:
		if config.AliyunConfig == nil {
			return fmt.Errorf("阿里云部署配置不能为空")
		}
		if config.AliyunConfig.AccessKeyID == "" || config.AliyunConfig.AccessKeySecret == "" {
			return fmt.Errorf("阿里云部署的AccessKey不能为空")
		}
		if config.AliyunConfig.TargetDomain == "" {
			return fmt.Errorf("阿里云CDN部署的目标域名不能为空")
		}
	default:
		return fmt.Errorf("不支持的部署类型: %s", config.Type)
	}

	return nil
}

// saveDeployConfigToEtcd 保存部署配置到etcd
func (cm *CertManager) saveDeployConfigToEtcd(ctx context.Context, config *DeployConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	key := etcdDeployConfigPrefix + config.ID
	return cm.etcdClient.Put(ctx, key, string(data))
}

// matchWildcardDomain 检查域名是否匹配通配符域名
func matchWildcardDomain(wildcardDomain, domain string) bool {
	if !isWildcardDomain(wildcardDomain) {
		return wildcardDomain == domain
	}

	// 移除通配符前缀 "*."
	suffix := wildcardDomain[2:]

	// 检查域名是否以相同的后缀结尾
	if len(domain) <= len(suffix) {
		return false
	}

	// 确保匹配的是完整的域名层级
	return domain[len(domain)-len(suffix):] == suffix && domain[len(domain)-len(suffix)-1] == '.'
}

// autoDeployAfterRenewal 在证书续期后执行自动部署
func (cm *CertManager) autoDeployAfterRenewal(ctx context.Context, domain string) error {
	cm.logger.Info("开始执行自动部署", zap.String("domain", domain))

	// 获取与该域名相关的所有部署配置
	deployConfigs := cm.ListDeployConfigsByDomain(domain)
	if len(deployConfigs) == 0 {
		cm.logger.Info("未找到需要自动部署的配置", zap.String("domain", domain))
		return nil
	}

	var successCount, failCount int
	for _, config := range deployConfigs {
		// 检查是否启用自动部署
		if !config.Enabled || !config.AutoDeploy {
			cm.logger.Debug("跳过部署配置（未启用或未启用自动部署）",
				zap.String("domain", domain),
				zap.String("config_id", config.ID),
				zap.String("config_name", config.Name),
				zap.Bool("enabled", config.Enabled),
				zap.Bool("auto_deploy", config.AutoDeploy))
			continue
		}

		cm.logger.Info("执行自动部署",
			zap.String("domain", domain),
			zap.String("config_id", config.ID),
			zap.String("config_name", config.Name),
			zap.String("type", string(config.Type)))

		// 执行部署
		if err := cm.deployCertificateWithConfig(ctx, config); err != nil {
			cm.logger.Error("自动部署失败",
				zap.String("domain", domain),
				zap.String("config_id", config.ID),
				zap.String("config_name", config.Name),
				zap.Error(err))
			failCount++
		} else {
			cm.logger.Info("自动部署成功",
				zap.String("domain", domain),
				zap.String("config_id", config.ID),
				zap.String("config_name", config.Name))
			successCount++
		}
	}

	cm.logger.Info("自动部署完成",
		zap.String("domain", domain),
		zap.Int("success_count", successCount),
		zap.Int("fail_count", failCount),
		zap.Int("total_count", len(deployConfigs)))

	return nil
}

// ================ 便利方法 ================

// GenerateDeployConfigID 生成部署配置ID
func (cm *CertManager) GenerateDeployConfigID(domain string, deployType DeployType) string {
	return fmt.Sprintf("%s_%s_%d", domain, deployType, time.Now().Unix())
}

// CreateLocalDeployConfig 创建本地文件部署配置
func (cm *CertManager) CreateLocalDeployConfig(domain, name, certPath, keyPath string, enabled, autoDeploy bool) *DeployConfig {
	id := cm.GenerateDeployConfigID(domain, DeployTypeLocal)
	return &DeployConfig{
		ID:         id,
		Name:       name,
		Domain:     domain,
		Type:       DeployTypeLocal,
		Enabled:    enabled,
		AutoDeploy: autoDeploy,
		LocalConfig: &LocalConfig{
			CertPath:           certPath,
			KeyPath:            keyPath,
			PostDeployCommands: []string{}, // 初始化为空数组
		},
	}
}

// CreateLocalDeployConfigWithCommands 创建包含命令的本地文件部署配置
func (cm *CertManager) CreateLocalDeployConfigWithCommands(domain, name, certPath, keyPath string, enabled, autoDeploy bool, commands []string) *DeployConfig {
	id := cm.GenerateDeployConfigID(domain, DeployTypeLocal)
	return &DeployConfig{
		ID:         id,
		Name:       name,
		Domain:     domain,
		Type:       DeployTypeLocal,
		Enabled:    enabled,
		AutoDeploy: autoDeploy,
		LocalConfig: &LocalConfig{
			CertPath:           certPath,
			KeyPath:            keyPath,
			PostDeployCommands: commands,
		},
	}
}

// CreateRemoteDeployConfig 创建远程服务器部署配置
func (cm *CertManager) CreateRemoteDeployConfig(domain, name string, config *RemoteConfig, enabled, autoDeploy bool) *DeployConfig {
	id := cm.GenerateDeployConfigID(domain, DeployTypeRemote)
	// 确保PostDeployCommands字段已初始化
	if config.PostDeployCommands == nil {
		config.PostDeployCommands = []string{}
	}
	return &DeployConfig{
		ID:           id,
		Name:         name,
		Domain:       domain,
		Type:         DeployTypeRemote,
		Enabled:      enabled,
		AutoDeploy:   autoDeploy,
		RemoteConfig: config,
	}
}

// CreateAliyunCDNDeployConfig 创建阿里云CDN部署配置
func (cm *CertManager) CreateAliyunCDNDeployConfig(domain, name string, config *AliyunConfig, enabled, autoDeploy bool) *DeployConfig {
	id := cm.GenerateDeployConfigID(domain, DeployTypeAliyunCDN)
	return &DeployConfig{
		ID:           id,
		Name:         name,
		Domain:       domain,
		Type:         DeployTypeAliyunCDN,
		Enabled:      enabled,
		AutoDeploy:   autoDeploy,
		AliyunConfig: config,
	}
}

// EnableDeployConfig 启用部署配置
func (cm *CertManager) EnableDeployConfig(ctx context.Context, configID string) error {
	config, err := cm.GetDeployConfig(configID)
	if err != nil {
		return err
	}

	config.Enabled = true
	return cm.UpdateDeployConfig(ctx, config)
}

// DisableDeployConfig 禁用部署配置
func (cm *CertManager) DisableDeployConfig(ctx context.Context, configID string) error {
	config, err := cm.GetDeployConfig(configID)
	if err != nil {
		return err
	}

	config.Enabled = false
	return cm.UpdateDeployConfig(ctx, config)
}

// EnableAutoDeployConfig 启用自动部署
func (cm *CertManager) EnableAutoDeployConfig(ctx context.Context, configID string) error {
	config, err := cm.GetDeployConfig(configID)
	if err != nil {
		return err
	}

	config.AutoDeploy = true
	return cm.UpdateDeployConfig(ctx, config)
}

// DisableAutoDeployConfig 禁用自动部署
func (cm *CertManager) DisableAutoDeployConfig(ctx context.Context, configID string) error {
	config, err := cm.GetDeployConfig(configID)
	if err != nil {
		return err
	}

	config.AutoDeploy = false
	return cm.UpdateDeployConfig(ctx, config)
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

	cm.logger.Info("已更新DNS配置", zap.String("provider", provider))

	return nil
}

// GetDNSConfig 获取当前DNS配置
func (cm *CertManager) GetDNSConfig(ctx context.Context) (*DNSConfig, error) {
	value, err := cm.etcdClient.Get(ctx, dnsConfigPrefix+"config")
	if err != nil {
		// 检查是否是键不存在的错误
		if strings.Contains(err.Error(), "键不存在") {
			return nil, fmt.Errorf("DNS配置不存在")
		}
		return nil, fmt.Errorf("从etcd获取DNS配置失败: %v", err)
	}

	if value == "" {
		return nil, fmt.Errorf("DNS配置不存在")
	}

	var config DNSConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, fmt.Errorf("解析DNS配置失败: %v", err)
	}

	return &config, nil
}

// MigrateCertificateContent 迁移现有证书的内容到etcd
func (cm *CertManager) MigrateCertificateContent(ctx context.Context) error {
	cm.logger.Info("开始迁移证书内容到etcd")

	// 获取所有域名
	domains := cm.GetDomains()
	migrationCount := 0
	failureCount := 0

	for _, domain := range domains {
		cert, err := cm.GetDomainCert(domain)
		if err != nil {
			cm.logger.Error("获取域名证书信息失败", zap.String("domain", domain), zap.Error(err))
			failureCount++
			continue
		}

		// 检查是否已经有证书内容
		if cert.CertContent != "" && cert.KeyContent != "" {
			cm.logger.Debug("域名证书内容已存在，跳过迁移", zap.String("domain", domain))
			continue
		}

		// 检查是否有证书文件路径
		if cert.CertPath == "" || cert.KeyPath == "" {
			cm.logger.Warn("域名没有证书文件路径，跳过迁移", zap.String("domain", domain))
			continue
		}

		// 读取证书文件内容
		certContent, err := os.ReadFile(cert.CertPath)
		if err != nil {
			cm.logger.Error("读取证书文件失败", zap.String("domain", domain), zap.String("cert_path", cert.CertPath), zap.Error(err))
			failureCount++
			continue
		}

		keyContent, err := os.ReadFile(cert.KeyPath)
		if err != nil {
			cm.logger.Error("读取私钥文件失败", zap.String("domain", domain), zap.String("key_path", cert.KeyPath), zap.Error(err))
			failureCount++
			continue
		}

		// 更新证书信息，添加内容
		cert.CertContent = string(certContent)
		cert.KeyContent = string(keyContent)

		// 保存到etcd
		if err := cm.saveDomainToEtcd(ctx, cert); err != nil {
			cm.logger.Error("保存迁移的证书信息到etcd失败", zap.String("domain", domain), zap.Error(err))
			failureCount++
			continue
		}

		cm.logger.Info("成功迁移域名证书内容", zap.String("domain", domain))
		migrationCount++
	}

	cm.logger.Info("证书内容迁移完成",
		zap.Int("total_domains", len(domains)),
		zap.Int("migrated_count", migrationCount),
		zap.Int("failure_count", failureCount))

	return nil
}
