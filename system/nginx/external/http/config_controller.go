package controller

import (
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/nginx/internal/app"
	"xiaozhizhang/system/nginx/internal/model/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ConfigController Nginx 配置文件管理控制器
type ConfigController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewConfigController 创建配置文件管理控制器实例
func NewConfigController(app *app.App) *ConfigController {
	return &ConfigController{
		app: app,
		err: errorc.NewErrorBuilder("NginxConfigController"),
		log: logger.GetLogger().WithEntryName("NginxConfigController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *ConfigController) RegisterRoutes(admin fiber.Router) {
	configRouter := admin.Group("/nginx/configs")

	// CRUD 接口（直接传 content）
	configRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:nginx:config:create"), ctrl.Create)
	configRouter.Put("/:name", base.AdminAuth.RequireAdminAuth("admin:nginx:config:update"), ctrl.Update)
	configRouter.Delete("/:name", base.AdminAuth.RequireAdminAuth("admin:nginx:config:delete"), ctrl.Delete)
	configRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:nginx:config:read"), ctrl.Query)
	configRouter.Get("/:name", base.AdminAuth.RequireAdminAuth("admin:nginx:config:read"), ctrl.GetByName)

	// 按参数生成并保存接口
	configRouter.Post("/generate", base.AdminAuth.RequireAdminAuth("admin:nginx:config:create"), ctrl.CreateByParams)
	configRouter.Put("/:name/generate", base.AdminAuth.RequireAdminAuth("admin:nginx:config:update"), ctrl.UpdateByParams)
}

// Create 创建配置文件
func (ctrl *ConfigController) Create(ctx *fiber.Ctx) error {
	var req dto.CreateConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.CreateConfig(util.Context(ctx), &req)
	return result.Once(ctx, "创建成功", err)
}

// Update 更新配置文件
func (ctrl *ConfigController) Update(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("配置文件名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.UpdateConfig(util.Context(ctx), name, &req)
	return result.Once(ctx, "更新成功", err)
}

// Delete 删除配置文件
func (ctrl *ConfigController) Delete(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("配置文件名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.DeleteConfig(util.Context(ctx), name)
	return result.Once(ctx, "删除成功", err)
}

// Query 查询配置文件列表
func (ctrl *ConfigController) Query(ctx *fiber.Ctx) error {
	var req dto.QueryConfigRequest
	if err := ctx.QueryParser(&req); err != nil {
		return ctrl.err.New("解析查询参数失败", err).WithTraceID(util.Context(ctx))
	}

	// 设置默认分页参数
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	configs, total, err := ctrl.app.ListConfigs(util.Context(ctx), &req)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": configs,
	})
}

// GetByName 根据名称查询配置文件
func (ctrl *ConfigController) GetByName(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("配置文件名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	config, err := ctrl.app.GetConfig(util.Context(ctx), name)
	return result.Once(ctx, config, err)
}

// CreateByParams 按参数生成并创建配置文件
func (ctrl *ConfigController) CreateByParams(ctx *fiber.Ctx) error {
	var req dto.CreateConfigByParamsRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.CreateConfigByParams(util.Context(ctx), &req)
	return result.Once(ctx, "创建成功", err)
}

// UpdateByParams 按参数生成并更新配置文件
func (ctrl *ConfigController) UpdateByParams(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("配置文件名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateConfigByParamsRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.UpdateConfigByParams(util.Context(ctx), name, &req)
	return result.Once(ctx, "更新成功", err)
}
