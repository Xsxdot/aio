package http

import (
	"strconv"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/model/common"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/system/ssl/internal/app"
	"xiaozhizhang/system/ssl/internal/model"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// SslController SSL 证书后台管理控制器
type SslController struct {
	app *app.App
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewSslController 创建 SSL 证书控制器实例
func NewSslController(app *app.App) *SslController {
	return &SslController{
		app: app,
		log: logger.GetLogger().WithEntryName("SslController"),
		err: errorc.NewErrorBuilder("SslController"),
	}
}

// RegisterRoutes 注册 SSL 证书相关路由
func (c *SslController) RegisterRoutes(admin fiber.Router) {
	ssl := admin.Group("/ssl")

	// DNS 凭证管理
	dnsCredRouter := ssl.Group("/dns-credentials")
	dnsCredRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:ssl:dns:create"), c.CreateDnsCredential)
	dnsCredRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:ssl:dns:read"), c.ListDnsCredentials)
	dnsCredRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:dns:read"), c.GetDnsCredential)
	dnsCredRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:dns:update"), c.UpdateDnsCredential)
	dnsCredRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:dns:delete"), c.DeleteDnsCredential)

	// 证书管理
	certRouter := ssl.Group("/certificates")
	certRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:create"), c.IssueCertificate)
	certRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:read"), c.ListCertificates)
	certRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:read"), c.GetCertificate)
	certRouter.Post("/:id/renew", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:renew"), c.RenewCertificate)
	certRouter.Post("/:id/deploy", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:deploy"), c.DeployCertificate)
	certRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:delete"), c.DeleteCertificate)
	certRouter.Get("/:id/deploy-history", base.AdminAuth.RequireAdminAuth("admin:ssl:cert:read"), c.GetDeployHistory)

	// 部署目标管理
	deployTargetRouter := ssl.Group("/deploy-targets")
	deployTargetRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:ssl:deploy:create"), c.CreateDeployTarget)
	deployTargetRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:ssl:deploy:read"), c.ListDeployTargets)
	deployTargetRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:deploy:read"), c.GetDeployTarget)
	deployTargetRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:deploy:update"), c.UpdateDeployTarget)
	deployTargetRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:ssl:deploy:delete"), c.DeleteDeployTarget)
}

// ===== DNS 凭证管理 =====

type CreateDnsCredentialRequest struct {
	Name        string            `json:"name" validate:"required"`
	Provider    model.DnsProvider `json:"provider" validate:"required"`
	AccessKey   string            `json:"access_key" validate:"required"`
	SecretKey   string            `json:"secret_key" validate:"required"`
	ExtraConfig string            `json:"extra_config"`
	Description string            `json:"description"`
}

func (c *SslController) CreateDnsCredential(ctx *fiber.Ctx) error {
	var req CreateDnsCredentialRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	// 解析 ExtraConfig JSON 字符串
	var extraConfig *common.JSON
	if req.ExtraConfig != "" {
		var jsonData common.JSON
		if err := utils.ParseJSON(req.ExtraConfig, &jsonData); err != nil {
			return c.err.New("解析 extra_config 失败", err).ValidWithCtx()
		}
		extraConfig = &jsonData
	}

	credential := &model.DnsCredential{
		Name:        req.Name,
		Provider:    req.Provider,
		AccessKey:   req.AccessKey,
		SecretKey:   req.SecretKey,
		ExtraConfig: extraConfig,
		Status:      1,
		Description: req.Description,
	}

	err := c.app.DnsCredSvc.Create(ctx.UserContext(), credential)
	return result.Once(ctx, credential, err)
}

func (c *SslController) ListDnsCredentials(ctx *fiber.Ctx) error {
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("page_size", "20"))

	pageInfo := &mvc.Page{
		PageNum: page,
		Size:    pageSize,
	}
	credentials, total, err := c.app.DnsCredentialDao.FindPage(ctx.UserContext(), pageInfo, nil)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": credentials,
	})
}

func (c *SslController) GetDnsCredential(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	credential, err := c.app.DnsCredentialDao.FindById(ctx.UserContext(), uint(id))
	return result.Once(ctx, credential, err)
}

type UpdateDnsCredentialRequest struct {
	Name        string `json:"name"`
	AccessKey   string `json:"access_key"`
	SecretKey   string `json:"secret_key"`
	ExtraConfig string `json:"extra_config"`
	Status      *int   `json:"status"`
	Description string `json:"description"`
}

func (c *SslController) UpdateDnsCredential(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	var req UpdateDnsCredentialRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	credential, err := c.app.DnsCredentialDao.FindById(ctx.UserContext(), uint(id))
	if err != nil {
		return err
	}

	// 更新字段
	if req.Name != "" {
		credential.Name = req.Name
	}
	if req.AccessKey != "" {
		encrypted, err := c.app.CryptoService.Encrypt(req.AccessKey)
		if err != nil {
			return c.err.New("加密 AccessKey 失败", err)
		}
		credential.AccessKey = encrypted
	}
	if req.SecretKey != "" {
		encrypted, err := c.app.CryptoService.Encrypt(req.SecretKey)
		if err != nil {
			return c.err.New("加密 SecretKey 失败", err)
		}
		credential.SecretKey = encrypted
	}
	if req.ExtraConfig != "" {
		var jsonData common.JSON
		if err := utils.ParseJSON(req.ExtraConfig, &jsonData); err != nil {
			return c.err.New("解析 extra_config 失败", err).ValidWithCtx()
		}
		credential.ExtraConfig = &jsonData
	}
	if req.Status != nil {
		credential.Status = *req.Status
	}
	if req.Description != "" {
		credential.Description = req.Description
	}

	_, err = c.app.DnsCredentialDao.UpdateById(ctx.UserContext(), uint(id), credential)
	return result.Once(ctx, credential, err)
}

func (c *SslController) DeleteDnsCredential(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	err = c.app.DnsCredentialDao.DeleteById(ctx.UserContext(), uint(id))
	return result.Once(ctx, nil, err)
}

// ===== 证书管理 =====

func (c *SslController) IssueCertificate(ctx *fiber.Ctx) error {
	var req app.IssueCertificateRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	cert, err := c.app.IssueCertificate(ctx.UserContext(), &req)
	return result.Once(ctx, cert, err)
}

func (c *SslController) ListCertificates(ctx *fiber.Ctx) error {
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("page_size", "20"))

	certificates, total, err := c.app.ListCertificates(ctx.UserContext(), page, pageSize)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": certificates,
	})
}

func (c *SslController) GetCertificate(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	cert, err := c.app.GetCertificate(ctx.UserContext(), uint(id))
	return result.Once(ctx, cert, err)
}

func (c *SslController) RenewCertificate(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	err = c.app.RenewCertificate(ctx.UserContext(), uint(id))
	return result.Once(ctx, fiber.Map{"message": "证书续期已触发"}, err)
}

type DeployCertificateRequest struct {
	TargetIDs []uint `json:"target_ids" validate:"required"`
}

func (c *SslController) DeployCertificate(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	var req DeployCertificateRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	// 异步部署
	go func() {
		deployCtx := ctx.UserContext()
		if err := c.app.DeployCertificateToTargets(deployCtx, uint(id), req.TargetIDs, "manual"); err != nil {
			c.log.WithErr(err).Error("手动部署证书失败")
		}
	}()

	return result.OK(ctx, fiber.Map{"message": "证书部署已触发"})
}

func (c *SslController) DeleteCertificate(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	err = c.app.DeleteCertificate(ctx.UserContext(), uint(id))
	return result.Once(ctx, nil, err)
}

func (c *SslController) GetDeployHistory(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))

	histories, err := c.app.GetDeployHistory(ctx.UserContext(), uint(id), limit)
	return result.Once(ctx, histories, err)
}

// ===== 部署目标管理 =====

type CreateDeployTargetRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Domain      string                 `json:"domain" validate:"required"`
	Type        model.DeployTargetType `json:"type" validate:"required"`
	Config      string                 `json:"config" validate:"required"`
	Description string                 `json:"description"`
}

func (c *SslController) CreateDeployTarget(ctx *fiber.Ctx) error {
	var req CreateDeployTargetRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	// 加密配置中的敏感字段
	encryptedConfig, err := c.encryptDeployConfig(req.Type, req.Config)
	if err != nil {
		return c.err.New("加密配置失败", err)
	}

	target := &model.DeployTarget{
		Name:        req.Name,
		Domain:      req.Domain,
		Type:        req.Type,
		Config:      encryptedConfig,
		Status:      1,
		Description: req.Description,
	}

	err = c.app.DeployTargetDao.Create(ctx.UserContext(), target)
	return result.Once(ctx, target, err)
}

func (c *SslController) ListDeployTargets(ctx *fiber.Ctx) error {
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("page_size", "20"))

	pageInfo := &mvc.Page{
		PageNum: page,
		Size:    pageSize,
	}
	targets, total, err := c.app.DeployTargetDao.FindPage(ctx.UserContext(), pageInfo, nil)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": targets,
	})
}

func (c *SslController) GetDeployTarget(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	target, err := c.app.DeployTargetDao.FindById(ctx.UserContext(), uint(id))
	return result.Once(ctx, target, err)
}

type UpdateDeployTargetRequest struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Config      string `json:"config"`
	Status      *int   `json:"status"`
	Description string `json:"description"`
}

func (c *SslController) UpdateDeployTarget(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	var req UpdateDeployTargetRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	target, err := c.app.DeployTargetDao.FindById(ctx.UserContext(), uint(id))
	if err != nil {
		return err
	}

	// 更新字段
	if req.Name != "" {
		target.Name = req.Name
	}
	if req.Domain != "" {
		target.Domain = req.Domain
	}
	if req.Config != "" {
		encryptedConfig, err := c.encryptDeployConfig(target.Type, req.Config)
		if err != nil {
			return c.err.New("加密配置失败", err)
		}
		target.Config = encryptedConfig
	}
	if req.Status != nil {
		target.Status = *req.Status
	}
	if req.Description != "" {
		target.Description = req.Description
	}

	_, err = c.app.DeployTargetDao.UpdateById(ctx.UserContext(), uint(id), target)
	return result.Once(ctx, target, err)
}

func (c *SslController) DeleteDeployTarget(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 32)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	err = c.app.DeployTargetDao.DeleteById(ctx.UserContext(), uint(id))
	return result.Once(ctx, nil, err)
}

// encryptDeployConfig 加密部署配置中的敏感字段
func (c *SslController) encryptDeployConfig(targetType model.DeployTargetType, configJSON string) (string, error) {
	switch targetType {
	case model.DeployTargetTypeSSH:
		var config model.SSHDeployConfig
		if err := utils.ParseJSON(configJSON, &config); err != nil {
			return "", err
		}
		if config.Password != "" && !c.app.CryptoService.IsEncrypted(config.Password) {
			encrypted, err := c.app.CryptoService.Encrypt(config.Password)
			if err != nil {
				return "", err
			}
			config.Password = encrypted
		}
		if config.PrivateKey != "" && !c.app.CryptoService.IsEncrypted(config.PrivateKey) {
			encrypted, err := c.app.CryptoService.Encrypt(config.PrivateKey)
			if err != nil {
				return "", err
			}
			config.PrivateKey = encrypted
		}
		return utils.ToJSON(config)

	case model.DeployTargetTypeAliyunCAS:
		var config model.AliyunCASDeployConfig
		if err := utils.ParseJSON(configJSON, &config); err != nil {
			return "", err
		}
		if config.AccessKeySecret != "" && !c.app.CryptoService.IsEncrypted(config.AccessKeySecret) {
			encrypted, err := c.app.CryptoService.Encrypt(config.AccessKeySecret)
			if err != nil {
				return "", err
			}
			config.AccessKeySecret = encrypted
		}
		return utils.ToJSON(config)

	default:
		return configJSON, nil
	}
}
