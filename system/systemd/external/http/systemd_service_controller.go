package controller

import (
	"strconv"

	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/systemd/internal/app"
	"xiaozhizhang/system/systemd/internal/model/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// SystemdServiceController Systemd 服务管理控制器
type SystemdServiceController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewSystemdServiceController 创建 Systemd 服务管理控制器实例
func NewSystemdServiceController(app *app.App) *SystemdServiceController {
	return &SystemdServiceController{
		app: app,
		err: errorc.NewErrorBuilder("SystemdServiceController"),
		log: logger.GetLogger().WithEntryName("SystemdServiceController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *SystemdServiceController) RegisterRoutes(admin fiber.Router) {
	serviceRouter := admin.Group("/systemd/services")

	// CRUD 接口
	serviceRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:systemd:service:create"), ctrl.Create)
	serviceRouter.Put("/:name", base.AdminAuth.RequireAdminAuth("admin:systemd:service:update"), ctrl.Update)
	serviceRouter.Delete("/:name", base.AdminAuth.RequireAdminAuth("admin:systemd:service:delete"), ctrl.Delete)
	serviceRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:systemd:service:read"), ctrl.Query)
	serviceRouter.Get("/:name", base.AdminAuth.RequireAdminAuth("admin:systemd:service:read"), ctrl.GetByName)

	// 按参数生成接口
	serviceRouter.Post("/generate", base.AdminAuth.RequireAdminAuth("admin:systemd:service:read"), ctrl.Generate)
	serviceRouter.Post("/from-params", base.AdminAuth.RequireAdminAuth("admin:systemd:service:create"), ctrl.CreateFromParams)
	serviceRouter.Put("/:name/from-params", base.AdminAuth.RequireAdminAuth("admin:systemd:service:update"), ctrl.UpdateFromParams)

	// 生命周期控制接口
	serviceRouter.Post("/:name/start", base.AdminAuth.RequireAdminAuth("admin:systemd:service:control"), ctrl.Start)
	serviceRouter.Post("/:name/stop", base.AdminAuth.RequireAdminAuth("admin:systemd:service:control"), ctrl.Stop)
	serviceRouter.Post("/:name/restart", base.AdminAuth.RequireAdminAuth("admin:systemd:service:control"), ctrl.Restart)
	serviceRouter.Post("/:name/reload", base.AdminAuth.RequireAdminAuth("admin:systemd:service:control"), ctrl.Reload)

	// 启用/禁用接口
	serviceRouter.Post("/:name/enable", base.AdminAuth.RequireAdminAuth("admin:systemd:service:control"), ctrl.Enable)
	serviceRouter.Post("/:name/disable", base.AdminAuth.RequireAdminAuth("admin:systemd:service:control"), ctrl.Disable)

	// 状态与日志接口
	serviceRouter.Get("/:name/status", base.AdminAuth.RequireAdminAuth("admin:systemd:service:read"), ctrl.GetStatus)
	serviceRouter.Get("/:name/logs", base.AdminAuth.RequireAdminAuth("admin:systemd:service:read"), ctrl.GetLogs)
}

// Create 创建服务
func (ctrl *SystemdServiceController) Create(ctx *fiber.Ctx) error {
	var req dto.CreateServiceRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.CreateService(util.Context(ctx), &req)
	return result.Once(ctx, "创建成功", err)
}

// Update 更新服务
func (ctrl *SystemdServiceController) Update(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateServiceRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.UpdateService(util.Context(ctx), name, &req)
	return result.Once(ctx, "更新成功", err)
}

// Delete 删除服务
func (ctrl *SystemdServiceController) Delete(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	// 获取 force 参数
	forceStr := ctx.Query("force", "0")
	force := forceStr == "1" || forceStr == "true"

	err := ctrl.app.DeleteService(util.Context(ctx), name, force)
	return result.Once(ctx, "删除成功", err)
}

// Query 查询服务列表
func (ctrl *SystemdServiceController) Query(ctx *fiber.Ctx) error {
	var req dto.QueryServiceRequest
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

	// 处理 includeStatus 参数
	includeStatusStr := ctx.Query("includeStatus", "0")
	req.IncludeStatus = includeStatusStr == "1" || includeStatusStr == "true"

	services, total, err := ctrl.app.ListServices(util.Context(ctx), &req)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": services,
	})
}

// GetByName 根据名称查询服务
func (ctrl *SystemdServiceController) GetByName(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	service, err := ctrl.app.GetService(util.Context(ctx), name)
	return result.Once(ctx, service, err)
}

// Start 启动服务
func (ctrl *SystemdServiceController) Start(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.ControlService(util.Context(ctx), name, "start")
	return result.Once(ctx, "启动成功", err)
}

// Stop 停止服务
func (ctrl *SystemdServiceController) Stop(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.ControlService(util.Context(ctx), name, "stop")
	return result.Once(ctx, "停止成功", err)
}

// Restart 重启服务
func (ctrl *SystemdServiceController) Restart(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.ControlService(util.Context(ctx), name, "restart")
	return result.Once(ctx, "重启成功", err)
}

// Reload 重载服务
func (ctrl *SystemdServiceController) Reload(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.ControlService(util.Context(ctx), name, "reload")
	return result.Once(ctx, "重载成功", err)
}

// Enable 启用服务
func (ctrl *SystemdServiceController) Enable(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.SetServiceEnabled(util.Context(ctx), name, true)
	return result.Once(ctx, "启用成功", err)
}

// Disable 禁用服务
func (ctrl *SystemdServiceController) Disable(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err := ctrl.app.SetServiceEnabled(util.Context(ctx), name, false)
	return result.Once(ctx, "禁用成功", err)
}

// GetStatus 获取服务状态
func (ctrl *SystemdServiceController) GetStatus(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	status, err := ctrl.app.GetServiceStatus(util.Context(ctx), name)
	return result.Once(ctx, status, err)
}

// GetLogs 获取服务日志
func (ctrl *SystemdServiceController) GetLogs(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	var req dto.LogsRequest
	if err := ctx.QueryParser(&req); err != nil {
		return ctrl.err.New("解析查询参数失败", err).WithTraceID(util.Context(ctx))
	}

	// 处理 n 参数
	nStr := ctx.Query("n", "200")
	n, _ := strconv.Atoi(nStr)
	if n > 0 {
		req.Lines = n
	}

	logs, err := ctrl.app.GetServiceLogs(util.Context(ctx), name, &req)
	return result.Once(ctx, logs, err)
}

// ------------------- Unit 生成相关接口 -------------------

// Generate 生成服务 unit 内容（仅预览，不落盘）
func (ctrl *SystemdServiceController) Generate(ctx *fiber.Ctx) error {
	var req dto.GenerateServiceRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	resp, err := ctrl.app.GenerateService(util.Context(ctx), &req)
	return result.Once(ctx, resp, err)
}

// CreateFromParams 按参数创建服务
func (ctrl *SystemdServiceController) CreateFromParams(ctx *fiber.Ctx) error {
	var req dto.CreateServiceFromParamsRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.CreateServiceFromParams(util.Context(ctx), &req)
	return result.Once(ctx, "创建成功", err)
}

// UpdateFromParams 按参数更新服务
func (ctrl *SystemdServiceController) UpdateFromParams(ctx *fiber.Ctx) error {
	name := ctx.Params("name")
	if name == "" {
		return ctrl.err.New("服务名称不能为空", nil).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateServiceFromParamsRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	err := ctrl.app.UpdateServiceFromParams(util.Context(ctx), name, &req)
	return result.Once(ctx, "更新成功", err)
}

