package controller

import (
	"strconv"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/security"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/config/internal/app"
	"xiaozhizhang/system/config/internal/model/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ConfigController 配置管理控制器
type ConfigController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewConfigController 创建配置管理控制器实例
func NewConfigController(app *app.App) *ConfigController {
	return &ConfigController{
		app: app,
		err: errorc.NewErrorBuilder("ConfigController"),
		log: logger.GetLogger().WithEntryName("ConfigController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *ConfigController) RegisterRoutes(admin fiber.Router) {
	configRouter := admin.Group("/configs")

	// 配置管理接口
	configRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:config:create"), ctrl.Create)
	configRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:config:update"), ctrl.Update)
	configRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:config:delete"), ctrl.Delete)
	configRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:config:read"), ctrl.Query)
	configRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:config:read"), ctrl.GetByID)

	// 历史版本接口
	configRouter.Get("/:id/history", base.AdminAuth.RequireAdminAuth("admin:config:read"), ctrl.GetHistory)
	configRouter.Post("/:id/rollback/:version", base.AdminAuth.RequireAdminAuth("admin:config:update"), ctrl.Rollback)

	// 导入导出接口
	configRouter.Post("/export", base.AdminAuth.RequireAdminAuth("admin:config:export"), ctrl.Export)
	configRouter.Post("/import", base.AdminAuth.RequireAdminAuth("admin:config:import"), ctrl.Import)
}

// Create 创建配置
func (ctrl *ConfigController) Create(ctx *fiber.Ctx) error {
	var req dto.CreateConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	// 获取操作人信息
	adminID, _ := security.GetAdminId(ctx)
	adminAccount, _ := security.GetAdminAccount(ctx)

	err := ctrl.app.CreateConfig(util.Context(ctx), &req, adminAccount, adminID)
	return result.Once(ctx, "创建成功", err)
}

// Update 更新配置
func (ctrl *ConfigController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	// 获取操作人信息
	adminID, _ := security.GetAdminId(ctx)
	adminAccount, _ := security.GetAdminAccount(ctx)

	err = ctrl.app.UpdateConfig(util.Context(ctx), id, &req, adminAccount, adminID)
	return result.Once(ctx, "更新成功", err)
}

// Delete 删除配置
func (ctrl *ConfigController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	err = ctrl.app.DeleteConfig(util.Context(ctx), id)
	return result.Once(ctx, "删除成功", err)
}

// Query 查询配置列表
func (ctrl *ConfigController) Query(ctx *fiber.Ctx) error {
	var req dto.QueryConfigRequest
	if err := ctx.QueryParser(&req); err != nil {
		return ctrl.err.New("解析查询参数失败", err).WithTraceID(util.Context(ctx))
	}

	configs, total, err := ctrl.app.QueryConfigs(util.Context(ctx), &req)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": configs,
	})
}

// GetByID 根据ID查询配置
func (ctrl *ConfigController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	config, err := ctrl.app.ConfigItemService.FindById(util.Context(ctx), id)
	return result.Once(ctx, config, err)
}

// GetHistory 查询配置历史版本
func (ctrl *ConfigController) GetHistory(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	// 查询配置
	config, err := ctrl.app.ConfigItemService.FindById(util.Context(ctx), id)
	if err != nil {
		return err
	}

	// 查询历史记录
	histories, err := ctrl.app.ConfigHistoryService.FindByConfigKey(util.Context(ctx), config.Key)
	return result.Once(ctx, histories, err)
}

// Rollback 回滚配置到指定版本
func (ctrl *ConfigController) Rollback(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	version, err := strconv.ParseInt(ctx.Params("version"), 10, 64)
	if err != nil {
		return ctrl.err.New("版本号参数错误", err).WithTraceID(util.Context(ctx))
	}

	// 获取操作人信息
	adminID, _ := security.GetAdminId(ctx)
	adminAccount, _ := security.GetAdminAccount(ctx)

	err = ctrl.app.RollbackConfig(util.Context(ctx), id, version, adminAccount, adminID)
	return result.Once(ctx, "回滚成功", err)
}

// Export 导出配置
func (ctrl *ConfigController) Export(ctx *fiber.Ctx) error {
	var req dto.ExportConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	result, err := ctrl.app.ExportConfigs(util.Context(ctx), &req)
	if err != nil {
		return err
	}

	// 设置响应头，提示下载
	ctx.Set("Content-Type", "application/json")
	ctx.Set("Content-Disposition", "attachment; filename=configs_export.json")

	return ctx.JSON(result)
}

// Import 导入配置
func (ctrl *ConfigController) Import(ctx *fiber.Ctx) error {
	var req dto.ImportConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	// 获取操作人信息
	adminID, _ := security.GetAdminId(ctx)
	adminAccount, _ := security.GetAdminAccount(ctx)

	err := ctrl.app.ImportConfigs(util.Context(ctx), &req, adminAccount, adminID)
	return result.Once(ctx, "导入成功", err)
}
