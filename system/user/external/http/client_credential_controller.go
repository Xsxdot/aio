package controller

import (
	"encoding/json"
	"strconv"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/user/api/dto"
	"xiaozhizhang/system/user/internal/app"
	internaldto "xiaozhizhang/system/user/internal/model/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ClientCredentialController 客户端凭证控制器
type ClientCredentialController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewClientCredentialController 创建客户端凭证控制器实例
func NewClientCredentialController(app *app.App) *ClientCredentialController {
	return &ClientCredentialController{
		app: app,
		err: errorc.NewErrorBuilder("ClientCredentialController"),
		log: logger.GetLogger().WithEntryName("ClientCredentialController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *ClientCredentialController) RegisterRoutes(admin fiber.Router) {
	clientRouter := admin.Group("/client-credentials")

	// 客户端凭证管理接口
	clientRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:client:create"), ctrl.Create)
	clientRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:client:read"), ctrl.List)
	clientRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:client:read"), ctrl.GetByID)
	clientRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:client:update"), ctrl.Update)
	clientRouter.Put("/:id/status", base.AdminAuth.RequireAdminAuth("admin:client:update"), ctrl.UpdateStatus)
	clientRouter.Post("/:id/rotate-secret", base.AdminAuth.RequireAdminAuth("admin:client:update"), ctrl.RotateSecret)
	clientRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:client:delete"), ctrl.Delete)
}

// Create 创建客户端凭证
func (ctrl *ClientCredentialController) Create(ctx *fiber.Ctx) error {
	var req internaldto.CreateClientCredentialReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	client, secret, err := ctrl.app.ClientCredentialService.CreateClient(
		util.Context(ctx),
		req.Name,
		req.Description,
		req.IPWhitelist,
		req.ExpiresAt,
	)
	if err != nil {
		return err
	}

	// 解析 IP 白名单
	var ipWhitelist []string
	if client.IPWhitelist != "" {
		_ = json.Unmarshal([]byte(client.IPWhitelist), &ipWhitelist)
	}

	// 返回包含 secret 的 DTO（仅创建时返回）
	response := &dto.ClientCredentialWithSecretDTO{
		ClientCredentialDTO: dto.ClientCredentialDTO{
			ID:          client.ID,
			Name:        client.Name,
			ClientKey:   client.ClientKey,
			Status:      client.Status,
			Description: client.Description,
			IPWhitelist: ipWhitelist,
			ExpiresAt:   client.ExpiresAt,
			CreatedAt:   client.CreatedAt,
			UpdatedAt:   client.UpdatedAt,
		},
		ClientSecret: secret,
	}

	return result.OK(ctx, response)
}

// List 查询客户端凭证列表
func (ctrl *ClientCredentialController) List(ctx *fiber.Ctx) error {
	clients, err := ctrl.app.ClientCredentialService.FindAllActive(util.Context(ctx))
	if err != nil {
		return err
	}

	// 转换为 DTO
	content := make([]*dto.ClientCredentialDTO, 0, len(clients))
	for _, client := range clients {
		var ipWhitelist []string
		if client.IPWhitelist != "" {
			_ = json.Unmarshal([]byte(client.IPWhitelist), &ipWhitelist)
		}

		content = append(content, &dto.ClientCredentialDTO{
			ID:          client.ID,
			Name:        client.Name,
			ClientKey:   client.ClientKey,
			Status:      client.Status,
			Description: client.Description,
			IPWhitelist: ipWhitelist,
			ExpiresAt:   client.ExpiresAt,
			CreatedAt:   client.CreatedAt,
			UpdatedAt:   client.UpdatedAt,
		})
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(content),
		"content": content,
	})
}

// GetByID 根据 ID 查询客户端凭证
func (ctrl *ClientCredentialController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	client, err := ctrl.app.ClientCredentialService.FindById(util.Context(ctx), id)
	if err != nil {
		return err
	}

	// 解析 IP 白名单
	var ipWhitelist []string
	if client.IPWhitelist != "" {
		_ = json.Unmarshal([]byte(client.IPWhitelist), &ipWhitelist)
	}

	response := &dto.ClientCredentialDTO{
		ID:          client.ID,
		Name:        client.Name,
		ClientKey:   client.ClientKey,
		Status:      client.Status,
		Description: client.Description,
		IPWhitelist: ipWhitelist,
		ExpiresAt:   client.ExpiresAt,
		CreatedAt:   client.CreatedAt,
		UpdatedAt:   client.UpdatedAt,
	}

	return result.OK(ctx, response)
}

// Update 更新客户端凭证
func (ctrl *ClientCredentialController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req internaldto.UpdateClientCredentialReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	req.ID = id
	err = ctrl.app.ClientCredentialService.UpdateClient(
		util.Context(ctx),
		req.ID,
		req.Name,
		req.Description,
		req.IPWhitelist,
		req.ExpiresAt,
	)
	return result.Once(ctx, "更新成功", err)
}

// UpdateStatus 更新客户端状态
func (ctrl *ClientCredentialController) UpdateStatus(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req internaldto.UpdateClientStatusReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	req.ID = id
	err = ctrl.app.ClientCredentialService.UpdateStatus(util.Context(ctx), req.ID, req.Status)
	return result.Once(ctx, "更新状态成功", err)
}

// RotateSecret 重新生成客户端 secret
func (ctrl *ClientCredentialController) RotateSecret(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	newSecret, err := ctrl.app.ClientCredentialService.RotateSecret(util.Context(ctx), id)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"clientSecret": newSecret,
		"message":      "secret 已重新生成，请妥善保管",
	})
}

// Delete 删除客户端凭证
func (ctrl *ClientCredentialController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	err = ctrl.app.ClientCredentialService.DeleteById(util.Context(ctx), id)
	return result.Once(ctx, "删除成功", err)
}



