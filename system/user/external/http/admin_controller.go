package controller

import (
	"strconv"
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/result"
	"github.com/xsxdot/aio/pkg/core/security"
	"github.com/xsxdot/aio/pkg/core/util"
	"github.com/xsxdot/aio/system/user/internal/app"
	"github.com/xsxdot/aio/system/user/internal/model/dto"
	"github.com/xsxdot/aio/utils"

	"github.com/gofiber/fiber/v2"
)

// AdminController 管理员控制器
type AdminController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewAdminController 创建管理员控制器实例
func NewAdminController(app *app.App) *AdminController {
	return &AdminController{
		app: app,
		err: errorc.NewErrorBuilder("AdminController"),
		log: logger.GetLogger().WithEntryName("AdminController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *AdminController) RegisterRoutes(admin fiber.Router) {
	adminRouter := admin.Group("/admins")

	// 管理员管理接口
	adminRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:user:create"), ctrl.Create)
	adminRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:user:read"), ctrl.List)
	adminRouter.Get("/info", base.AdminAuth.RequireAdminAuth(), ctrl.Info)
	adminRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:user:read"), ctrl.GetByID)
	adminRouter.Put("/:id/password", base.AdminAuth.RequireAdminAuth("admin:user:update"), ctrl.ResetPassword)
	adminRouter.Put("/:id/status", base.AdminAuth.RequireAdminAuth("admin:user:update"), ctrl.UpdateStatus)
	adminRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:user:delete"), ctrl.Delete)

	// 登录接口（不需要鉴权）
	admin.Post("/login", ctrl.Login)
}

// Create 创建管理员
func (ctrl *AdminController) Create(ctx *fiber.Ctx) error {
	var req dto.CreateAdminReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	admin, err := ctrl.app.AdminService.CreateAdmin(util.Context(ctx), req.Account, req.Password, req.Remark)
	return result.Once(ctx, admin, err)
}

// List 查询管理员列表
func (ctrl *AdminController) List(ctx *fiber.Ctx) error {
	admins, err := ctrl.app.AdminService.FindAllActive(util.Context(ctx))
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(admins),
		"content": admins,
	})
}

// GetByID 根据 ID 查询管理员
func (ctrl *AdminController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	admin, err := ctrl.app.AdminService.FindById(util.Context(ctx), id)
	return result.Once(ctx, admin, err)
}

// Info 获取当前登录管理员信息
func (ctrl *AdminController) Info(ctx *fiber.Ctx) error {
	adminID, err := security.GetAdminId(ctx)
	if err != nil {
		return ctrl.err.New("获取管理员ID失败", err).WithTraceID(util.Context(ctx))
	}

	admin, err := ctrl.app.AdminService.FindById(util.Context(ctx), adminID)
	return result.Once(ctx, admin, err)
}

// ResetPassword 重置管理员密码
func (ctrl *AdminController) ResetPassword(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req dto.ResetAdminPasswordReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	req.AdminID = id
	err = ctrl.app.AdminService.ResetPassword(util.Context(ctx), req.AdminID, req.NewPassword)
	return result.Once(ctx, "重置密码成功", err)
}

// UpdateStatus 更新管理员状态
func (ctrl *AdminController) UpdateStatus(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateAdminStatusReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	req.AdminID = id
	err = ctrl.app.AdminService.UpdateStatus(util.Context(ctx), req.AdminID, req.Status)
	return result.Once(ctx, "更新状态成功", err)
}

// Delete 删除管理员
func (ctrl *AdminController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	err = ctrl.app.AdminService.DeleteById(util.Context(ctx), id)
	return result.Once(ctx, "删除成功", err)
}

// Login 管理员登录
func (ctrl *AdminController) Login(ctx *fiber.Ctx) error {
	var req dto.AdminLoginReq
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	admin, err := ctrl.app.AdminService.ValidateLogin(util.Context(ctx), req.Account, req.Password)
	if err != nil {
		return err
	}

	// 构建 AdminClaims，根据 DB 中的 IsSuper 和 Roles 设置 AdminType
	adminType := make([]string, 0, len(admin.Roles)+1)
	if admin.IsSuper {
		adminType = append(adminType, "SuperAdmin")
	}
	adminType = append(adminType, admin.Roles...)

	claims := &security.AdminClaims{
		ID:        admin.ID,
		Account:   admin.Account,
		AdminType: adminType,
	}

	// 创建 JWT token
	token, expiresAt, err := base.AdminAuth.CreateAdminToken(claims)
	if err != nil {
		return ctrl.err.New("创建登录令牌失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	return result.OK(ctx, fiber.Map{
		"accessToken": token,
		"expiresAt":   expiresAt,
		"tokenType":   "Bearer",
		"admin": fiber.Map{
			"id":      admin.ID,
			"account": admin.Account,
			"isSuper": admin.IsSuper,
			"roles":   admin.Roles,
			"status":  admin.Status,
			"remark":  admin.Remark,
		},
	})
}
