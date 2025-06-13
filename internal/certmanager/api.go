package certmanager

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/utils"
	"go.uber.org/zap"
)

// API 证书管理API控制器
type API struct {
	cm     *CertManager
	logger *zap.Logger
}

// NewAPI 创建证书管理API控制器
func NewAPI(cm *CertManager, logger *zap.Logger) *API {
	return &API{
		cm:     cm,
		logger: logger,
	}
}

// RegisterRoutes 注册API路由
func (a *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	api := router.Group("/cert", authHandler, adminRoleHandler)

	// 域名证书管理
	api.Get("/domains", a.GetDomains)
	api.Post("/domains", a.AddDomain)
	api.Get("/domains/:domain", a.GetDomainCert)
	api.Delete("/domains/:domain", a.RemoveDomain)
	api.Get("/domains/:domain/content", a.GetCertificateContent)
	api.Get("/domains/:domain/convert/:format", a.ConvertCertificate)

	// DNS提供商管理
	api.Get("/dns-providers", a.ListDNSProviders)
	api.Post("/dns-providers", a.AddDNSProvider)
	api.Get("/dns-providers/:name", a.GetDNSProvider)
	api.Delete("/dns-providers/:name", a.DeleteDNSProvider)

	// DNS配置管理
	api.Get("/dns-config", a.GetDNSConfig)
	api.Post("/dns-config", a.SetDNSConfig)

	// 部署配置管理
	api.Get("/deploy-configs", a.ListDeployConfigs)
	api.Post("/deploy-configs", a.AddDeployConfig)
	api.Get("/deploy-configs/:id", a.GetDeployConfig)
	api.Put("/deploy-configs/:id", a.UpdateDeployConfig)
	api.Delete("/deploy-configs/:id", a.DeleteDeployConfig)
	api.Get("/deploy-configs/domain/:domain", a.ListDeployConfigsByDomain)

	// 部署操作
	api.Post("/deploy-configs/:id/deploy", a.DeployCertificate)
	api.Get("/deploy-configs/:id/status", a.GetDeployStatus)
	api.Post("/deploy-configs/:id/enable", a.EnableDeployConfig)
	api.Post("/deploy-configs/:id/disable", a.DisableDeployConfig)
	api.Post("/deploy-configs/:id/auto-deploy/enable", a.EnableAutoDeployConfig)
	api.Post("/deploy-configs/:id/auto-deploy/disable", a.DisableAutoDeployConfig)

	// 连接测试
	api.Post("/test/ssh", a.TestSSHConnection)
	api.Post("/test/tcp", a.TestTCPConnection)

	a.logger.Info("已注册证书管理API路由")
}

// GetDomains 获取所有域名
func (a *API) GetDomains(c *fiber.Ctx) error {
	// 返回所有域名的完整证书信息，而不仅仅是域名列表
	certs := a.cm.GetAllDomainCerts()
	return utils.SuccessResponse(c, certs)
}

// AddDomain 添加域名并申请证书
func (a *API) AddDomain(c *fiber.Ctx) error {
	type AddDomainRequest struct {
		Domain      string `json:"domain"`
		DNSProvider string `json:"dnsProvider"`
	}

	var req AddDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	if req.Domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}

	if err := a.cm.AddDomain(c.Context(), req.Domain, req.DNSProvider); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	cert, err := a.cm.GetDomainCert(req.Domain)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"domain":     cert.Domain,
		"isWildcard": cert.IsWildcard,
		"issuedAt":   cert.IssuedAt,
		"expiresAt":  cert.ExpiresAt,
	})
}

// GetDomainCert 获取域名证书详情
func (a *API) GetDomainCert(c *fiber.Ctx) error {
	domain := c.Params("domain")
	if domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}

	cert, err := a.cm.GetDomainCert(domain)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, err.Error())
	}

	return utils.SuccessResponse(c, cert)
}

// RemoveDomain 删除域名
func (a *API) RemoveDomain(c *fiber.Ctx) error {
	domain := c.Params("domain")
	if domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}

	if err := a.cm.RemoveDomain(c.Context(), domain); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, nil)
}

// ListDNSProviders 获取所有DNS提供商
func (a *API) ListDNSProviders(c *fiber.Ctx) error {
	providers, err := a.cm.ListDNSProviders(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	// 过滤敏感信息
	result := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		result = append(result, map[string]interface{}{
			"name":         p.Name,
			"providerType": p.ProviderType,
			"createdAt":    p.CreatedAt,
			"updatedAt":    p.UpdatedAt,
		})
	}

	return utils.SuccessResponse(c, result)
}

// AddDNSProvider 添加DNS提供商
func (a *API) AddDNSProvider(c *fiber.Ctx) error {
	type AddDNSProviderRequest struct {
		Name         string            `json:"name"`
		ProviderType string            `json:"providerType"`
		Credentials  map[string]string `json:"credentials"`
	}

	var req AddDNSProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	if req.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS提供商名称不能为空")
	}

	if req.ProviderType == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS提供商类型不能为空")
	}

	if len(req.Credentials) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS凭证不能为空")
	}

	if err := a.cm.AddDNSProvider(c.Context(), req.Name, req.ProviderType, req.Credentials); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	provider, err := a.cm.GetDNSProvider(c.Context(), req.Name)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"name":         provider.Name,
		"providerType": provider.ProviderType,
		"createdAt":    provider.CreatedAt,
		"updatedAt":    provider.UpdatedAt,
	})
}

// GetDNSProvider 获取DNS提供商详情
func (a *API) GetDNSProvider(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS提供商名称不能为空")
	}

	provider, err := a.cm.GetDNSProvider(c.Context(), name)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, err.Error())
	}

	return utils.SuccessResponse(c, provider)
}

// DeleteDNSProvider 删除DNS提供商
func (a *API) DeleteDNSProvider(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS提供商名称不能为空")
	}

	if err := a.cm.DeleteDNSProvider(c.Context(), name); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, nil)
}

// GetDNSConfig 获取当前DNS配置
func (a *API) GetDNSConfig(c *fiber.Ctx) error {
	config, err := a.cm.GetDNSConfig(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "未找到DNS配置")
	}

	// 过滤敏感信息
	return utils.SuccessResponse(c, map[string]interface{}{
		"provider":  config.Provider,
		"updatedAt": config.UpdatedAt,
	})
}

// SetDNSConfig 设置DNS配置
func (a *API) SetDNSConfig(c *fiber.Ctx) error {
	type SetDNSConfigRequest struct {
		Provider    string            `json:"provider"`
		Credentials map[string]string `json:"credentials"`
	}

	var req SetDNSConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	if req.Provider == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS提供商不能为空")
	}

	if len(req.Credentials) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "DNS凭证不能为空")
	}

	if err := a.cm.SetDNSConfig(c.Context(), req.Provider, req.Credentials); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	config, err := a.cm.GetDNSConfig(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取更新后的DNS配置失败")
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"provider":  config.Provider,
		"updatedAt": config.UpdatedAt,
	})
}

// GetCertificateContent 获取证书文件内容
func (a *API) GetCertificateContent(c *fiber.Ctx) error {
	domain := c.Params("domain")
	if domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}

	content, err := a.cm.GetCertificateContent(domain)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, content)
}

// ConvertCertificate 转换证书为不同格式
func (a *API) ConvertCertificate(c *fiber.Ctx) error {
	domain := c.Params("domain")
	if domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}

	format := c.Params("format")
	if format == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "转换格式不能为空")
	}

	// 支持的格式检查
	supportedFormats := map[string]bool{
		"nginx":  true,
		"apache": true,
		"pkcs12": true, // 用于Tomcat
		"jks":    true,
		"iis":    true,
	}

	if !supportedFormats[format] {
		return utils.FailResponse(c, utils.StatusBadRequest, "不支持的转换格式，支持的格式有: nginx, apache, pkcs12, jks, iis")
	}

	result, err := a.cm.ConvertCertificate(c.Context(), domain, format)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("转换证书失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// ================ 部署配置管理API ================

// ListDeployConfigs 获取所有部署配置
func (a *API) ListDeployConfigs(c *fiber.Ctx) error {
	configs := a.cm.ListDeployConfigs()

	// 过滤敏感信息
	result := make([]map[string]interface{}, 0, len(configs))
	for _, config := range configs {
		result = append(result, a.sanitizeDeployConfig(config))
	}

	return utils.SuccessResponse(c, result)
}

// AddDeployConfig 添加部署配置
func (a *API) AddDeployConfig(c *fiber.Ctx) error {
	type AddDeployConfigRequest struct {
		Name         string        `json:"name"`
		Domain       string        `json:"domain"`
		Type         string        `json:"type"`
		Enabled      bool          `json:"enabled"`
		AutoDeploy   bool          `json:"auto_deploy"`
		LocalConfig  *LocalConfig  `json:"local_config,omitempty"`  // 本地部署配置
		RemoteConfig *RemoteConfig `json:"remote_config,omitempty"` // 远程服务器配置
		AliyunConfig *AliyunConfig `json:"aliyun_config,omitempty"` // 阿里云配置
	}

	var req AddDeployConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	// 验证必填字段
	if req.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置名称不能为空")
	}
	if req.Domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}
	if req.Type == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "部署类型不能为空")
	}

	// 验证部署类型
	deployType := DeployType(req.Type)
	if deployType != DeployTypeLocal && deployType != DeployTypeRemote && deployType != DeployTypeAliyunCDN {
		return utils.FailResponse(c, utils.StatusBadRequest, "不支持的部署类型，支持的类型有: local, remote, aliyun_cdn")
	}

	// 创建部署配置
	var deployConfig *DeployConfig
	switch deployType {
	case DeployTypeLocal:
		if req.LocalConfig == nil {
			return utils.FailResponse(c, utils.StatusBadRequest, "本地部署配置不能为空")
		}
		deployConfig = a.cm.CreateLocalDeployConfig(req.Domain, req.Name, req.LocalConfig.CertPath, req.LocalConfig.KeyPath, req.Enabled, req.AutoDeploy)
		// 设置部署后命令
		deployConfig.LocalConfig.PostDeployCommands = req.LocalConfig.PostDeployCommands
	case DeployTypeRemote:
		if req.RemoteConfig == nil {
			return utils.FailResponse(c, utils.StatusBadRequest, "远程部署配置不能为空")
		}
		deployConfig = a.cm.CreateRemoteDeployConfig(req.Domain, req.Name, req.RemoteConfig, req.Enabled, req.AutoDeploy)
	case DeployTypeAliyunCDN:
		if req.AliyunConfig == nil {
			return utils.FailResponse(c, utils.StatusBadRequest, "阿里云CDN部署配置不能为空")
		}
		deployConfig = a.cm.CreateAliyunCDNDeployConfig(req.Domain, req.Name, req.AliyunConfig, req.Enabled, req.AutoDeploy)
	}

	// 添加配置
	if err := a.cm.AddDeployConfig(c.Context(), deployConfig); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, a.sanitizeDeployConfig(deployConfig))
}

// GetDeployConfig 获取部署配置详情
func (a *API) GetDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	config, err := a.cm.GetDeployConfig(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, err.Error())
	}

	return utils.SuccessResponse(c, a.sanitizeDeployConfig(config))
}

// UpdateDeployConfig 更新部署配置
func (a *API) UpdateDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	// 先获取现有配置
	existingConfig, err := a.cm.GetDeployConfig(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, err.Error())
	}

	type UpdateDeployConfigRequest struct {
		Name         string        `json:"name"`
		Enabled      *bool         `json:"enabled,omitempty"`
		AutoDeploy   *bool         `json:"auto_deploy,omitempty"`
		LocalConfig  *LocalConfig  `json:"local_config,omitempty"`
		RemoteConfig *RemoteConfig `json:"remote_config,omitempty"`
		AliyunConfig *AliyunConfig `json:"aliyun_config,omitempty"`
	}

	var req UpdateDeployConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	// 更新字段
	if req.Name != "" {
		existingConfig.Name = req.Name
	}
	if req.Enabled != nil {
		existingConfig.Enabled = *req.Enabled
	}
	if req.AutoDeploy != nil {
		existingConfig.AutoDeploy = *req.AutoDeploy
	}

	// 根据部署类型更新相应配置
	switch existingConfig.Type {
	case DeployTypeLocal:
		if req.LocalConfig != nil {
			existingConfig.LocalConfig = req.LocalConfig
		}
	case DeployTypeRemote:
		if req.RemoteConfig != nil {
			existingConfig.RemoteConfig = req.RemoteConfig
		}
	case DeployTypeAliyunCDN:
		if req.AliyunConfig != nil {
			existingConfig.AliyunConfig = req.AliyunConfig
		}
	}

	// 保存更新
	if err := a.cm.UpdateDeployConfig(c.Context(), existingConfig); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, a.sanitizeDeployConfig(existingConfig))
}

// DeleteDeployConfig 删除部署配置
func (a *API) DeleteDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	if err := a.cm.DeleteDeployConfig(c.Context(), id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, nil)
}

// ListDeployConfigsByDomain 根据域名获取部署配置
func (a *API) ListDeployConfigsByDomain(c *fiber.Ctx) error {
	domain := c.Params("domain")
	if domain == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "域名不能为空")
	}

	configs := a.cm.ListDeployConfigsByDomain(domain)

	// 过滤敏感信息
	result := make([]map[string]interface{}, 0, len(configs))
	for _, config := range configs {
		result = append(result, a.sanitizeDeployConfig(config))
	}

	return utils.SuccessResponse(c, result)
}

// ================ 部署操作API ================

// DeployCertificate 手动执行部署
func (a *API) DeployCertificate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	if err := a.cm.DeployCertificate(c.Context(), id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	// 获取最新的配置状态
	config, err := a.cm.GetDeployConfig(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message":         "部署成功",
		"lastDeployAt":    config.LastDeployAt,
		"lastDeployError": config.LastDeployError,
	})
}

// GetDeployStatus 获取部署状态
func (a *API) GetDeployStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	config, err := a.cm.GetDeployConfig(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, err.Error())
	}

	// 构造部署状态响应
	status := map[string]interface{}{
		"id":              config.ID,
		"name":            config.Name,
		"domain":          config.Domain,
		"type":            config.Type,
		"enabled":         config.Enabled,
		"autoDeploy":      config.AutoDeploy,
		"lastDeployAt":    config.LastDeployAt,
		"lastDeployError": config.LastDeployError,
		"deployStatus":    "unknown",
	}

	// 根据最后部署时间和错误状态判断部署状态
	if !config.LastDeployAt.IsZero() {
		if config.LastDeployError == "" {
			status["deployStatus"] = "success"
		} else {
			status["deployStatus"] = "failed"
		}
	} else {
		status["deployStatus"] = "never_deployed"
	}

	return utils.SuccessResponse(c, status)
}

// EnableDeployConfig 启用部署配置
func (a *API) EnableDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	if err := a.cm.EnableDeployConfig(c.Context(), id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "部署配置已启用",
	})
}

// DisableDeployConfig 禁用部署配置
func (a *API) DisableDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	if err := a.cm.DisableDeployConfig(c.Context(), id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "部署配置已禁用",
	})
}

// EnableAutoDeployConfig 启用自动部署
func (a *API) EnableAutoDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	if err := a.cm.EnableAutoDeployConfig(c.Context(), id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "自动部署已启用",
	})
}

// DisableAutoDeployConfig 禁用自动部署
func (a *API) DisableAutoDeployConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置ID不能为空")
	}

	if err := a.cm.DisableAutoDeployConfig(c.Context(), id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "自动部署已禁用",
	})
}

// ================ 连接测试API ================

// TestSSHConnection 测试SSH连接
func (a *API) TestSSHConnection(c *fiber.Ctx) error {
	var config RemoteConfig
	if err := c.BodyParser(&config); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	if config.Host == "" || config.Username == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "主机地址和用户名不能为空")
	}

	if config.Port <= 0 {
		config.Port = 22 // 默认SSH端口
	}

	if err := a.cm.TestSSHConnection(&config); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("SSH连接测试失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "SSH连接测试成功",
		"host":    config.Host,
		"port":    config.Port,
	})
}

// TestTCPConnection 测试TCP连接
func (a *API) TestTCPConnection(c *fiber.Ctx) error {
	type TestTCPRequest struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	var req TestTCPRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	if req.Host == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "主机地址不能为空")
	}

	if req.Port <= 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "端口号必须大于0")
	}

	if err := a.cm.TestTCPConnection(req.Host, req.Port); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("TCP连接测试失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "TCP连接测试成功",
		"host":    req.Host,
		"port":    req.Port,
	})
}

// ================ 辅助方法 ================

// sanitizeDeployConfig 过滤部署配置中的敏感信息
func (a *API) sanitizeDeployConfig(config *DeployConfig) map[string]interface{} {
	result := map[string]interface{}{
		"id":                config.ID,
		"name":              config.Name,
		"domain":            config.Domain,
		"type":              config.Type,
		"enabled":           config.Enabled,
		"auto_deploy":       config.AutoDeploy,
		"created_at":        config.CreatedAt,
		"updated_at":        config.UpdatedAt,
		"last_deploy_at":    config.LastDeployAt,
		"last_deploy_error": config.LastDeployError,
	}

	switch config.Type {
	case DeployTypeLocal:
		if config.LocalConfig != nil {
			result["local_config"] = map[string]interface{}{
				"cert_path":            config.LocalConfig.CertPath,
				"key_path":             config.LocalConfig.KeyPath,
				"post_deploy_commands": config.LocalConfig.PostDeployCommands,
			}
		}
	case DeployTypeRemote:
		if config.RemoteConfig != nil {
			result["remote_config"] = map[string]interface{}{
				"host":                 config.RemoteConfig.Host,
				"port":                 config.RemoteConfig.Port,
				"username":             config.RemoteConfig.Username,
				"cert_path":            config.RemoteConfig.CertPath,
				"key_path":             config.RemoteConfig.KeyPath,
				"post_deploy_commands": config.RemoteConfig.PostDeployCommands,
				// 不返回密码和私钥
			}
		}
	case DeployTypeAliyunCDN:
		if config.AliyunConfig != nil {
			result["aliyun_config"] = map[string]interface{}{
				"target_domain": config.AliyunConfig.TargetDomain,
				// 不返回AccessKey信息
			}
		}
	}

	return result
}
