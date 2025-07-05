// Package credential 凭证管理 HTTP API 实现
package credential

import (
	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/utils"
	"go.uber.org/zap"
)

// API 凭证管理 HTTP API
type API struct {
	service Service
	logger  *zap.Logger
}

// NewAPI 创建凭证管理 API
func NewAPI(service Service, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &API{
		service: service,
		logger:  logger.With(zap.String("component", "credential_api")),
	}
}

// RegisterRoutes 注册凭证管理相关路由
func (a *API) RegisterRoutes(app fiber.Router, baseAuthHandler func(c *fiber.Ctx) error, adminRoleHandler func(c *fiber.Ctx) error) {
	// 凭证管理路由
	credentialGroup := app.Group("/credentials")
	credentialGroup.Post("/", baseAuthHandler, a.CreateCredential)
	credentialGroup.Get("/", baseAuthHandler, a.ListCredentials)
	credentialGroup.Get("/:id", baseAuthHandler, a.GetCredential)
	credentialGroup.Put("/:id", baseAuthHandler, a.UpdateCredential)
	credentialGroup.Delete("/:id", baseAuthHandler, a.DeleteCredential)
	credentialGroup.Post("/:id/test", baseAuthHandler, a.TestCredential)
}

// CreateCredential 创建凭证
func (a *API) CreateCredential(c *fiber.Ctx) error {
	var req CredentialCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	result, err := a.service.CreateCredential(c.Context(), &req)
	if err != nil {
		a.logger.Error("创建凭证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ListCredentials 获取凭证列表
func (a *API) ListCredentials(c *fiber.Ctx) error {
	req := &CredentialListRequest{
		Limit:  20,
		Offset: 0,
	}

	// 解析查询参数
	if limit := c.QueryInt("limit", 20); limit > 0 && limit <= 100 {
		req.Limit = limit
	}
	if offset := c.QueryInt("offset", 0); offset >= 0 {
		req.Offset = offset
	}

	credentials, total, err := a.service.ListCredentials(c.Context(), req)
	if err != nil {
		a.logger.Error("获取凭证列表失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"credentials": credentials,
		"total":       total,
	})
}

// GetCredential 获取凭证详情
func (a *API) GetCredential(c *fiber.Ctx) error {
	credentialID := c.Params("id")
	if credentialID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "凭证ID不能为空")
	}

	credential, err := a.service.GetCredential(c.Context(), credentialID)
	if err != nil {
		a.logger.Error("获取凭证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, credential)
}

// UpdateCredential 更新凭证
func (a *API) UpdateCredential(c *fiber.Ctx) error {
	credentialID := c.Params("id")
	if credentialID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "凭证ID不能为空")
	}

	var req CredentialUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	result, err := a.service.UpdateCredential(c.Context(), credentialID, &req)
	if err != nil {
		a.logger.Error("更新凭证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// DeleteCredential 删除凭证
func (a *API) DeleteCredential(c *fiber.Ctx) error {
	credentialID := c.Params("id")
	if credentialID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "凭证ID不能为空")
	}

	err := a.service.DeleteCredential(c.Context(), credentialID)
	if err != nil {
		a.logger.Error("删除凭证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]string{"message": "凭证删除成功"})
}

// TestCredential 测试凭证
func (a *API) TestCredential(c *fiber.Ctx) error {
	credentialID := c.Params("id")
	if credentialID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "凭证ID不能为空")
	}

	var req CredentialTestRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	result, err := a.service.TestCredential(c.Context(), credentialID, &req)
	if err != nil {
		a.logger.Error("测试凭证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}
