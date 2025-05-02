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
	config := a.cm.GetDNSConfig()
	if config == nil {
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

	config := a.cm.GetDNSConfig()
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
