package certmanager

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
)

// Service 提供证书管理功能
type Service struct {
	manager CertificateManager
	config  *Config
	logger  *logrus.Logger
}

// NewService 创建证书管理服务
func NewService(config *Config, logger *logrus.Logger) *Service {
	if logger == nil {
		logger = logrus.New()
	}

	return &Service{
		config: config,
		logger: logger,
	}
}

// Start 启动证书管理服务
func (s *Service) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.logger.Info("证书管理服务已禁用")
		return nil
	}

	s.logger.Info("启动证书管理服务")

	// 验证DNS配置
	if s.config.VerifyMethod == VerifyDNS {
		if err := s.validateDNSConfig(); err != nil {
			return err
		}
	}

	// 根据配置决定是否使用真实的ACME客户端
	var client acmeClient
	if s.config.Enabled {
		// 在实际环境中使用我们的Lego客户端实现
		// 注意：目前需要在go.mod中添加go-acme/lego/v4依赖
		client = NewLegoClient(s.logger, s.config)

		// 临时注释：当lego库导入问题解决后，取消注释上面的代码
		// client = &defaultAcmeClient{}
	} else {
		client = &defaultAcmeClient{}
	}

	// 创建证书管理器
	s.manager = NewManager(s.config, s.logger, client)

	// 初始化管理器
	if err := s.manager.Init(ctx); err != nil {
		return err
	}

	// 启动自动续期任务
	if err := s.manager.StartRenewalTask(ctx); err != nil {
		return err
	}

	s.logger.Info("证书管理服务已启动")
	return nil
}

// validateDNSConfig 验证DNS配置是否有效
func (s *Service) validateDNSConfig() error {
	if s.config.DNSProvider == "" {
		return errors.New("未指定DNS提供商")
	}

	// 验证阿里云DNS配置
	if s.config.DNSProvider == DNSProviderAliyun {
		if s.config.DNSConfig.AliyunAccessKeyID == "" || s.config.DNSConfig.AliyunAccessKeySecret == "" {
			return errors.New("阿里云DNS配置不完整，需要AccessKey ID和Secret")
		}
	}

	// 通配符证书检查
	for _, domain := range s.config.Domains {
		if len(domain) > 1 && domain[0] == '*' {
			s.logger.Infof("检测到通配符域名 %s，将使用DNS验证", domain)
		}
	}

	return nil
}

// Stop 停止证书管理服务
func (s *Service) Stop() error {
	if s.manager != nil {
		return s.manager.StopRenewalTask()
	}
	return nil
}

// GetCertificate 获取某个域名的证书
func (s *Service) GetCertificate(domain string) (*Certificate, error) {
	if s.manager == nil {
		return nil, ErrServiceNotStarted
	}
	return s.manager.GetCertificate(domain)
}

// RenewCertificate 手动续期某个域名的证书
func (s *Service) RenewCertificate(domain string) error {
	if s.manager == nil {
		return ErrServiceNotStarted
	}
	return s.manager.RenewCertificate(domain)
}

// ErrServiceNotStarted 服务未启动错误
var ErrServiceNotStarted = serviceError("证书管理服务未启动")

// 定义错误类型
type serviceError string

func (e serviceError) Error() string {
	return string(e)
}
