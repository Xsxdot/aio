package http

import (
	"strconv"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/result"
	internalapp "xiaozhizhang/system/server/internal/app"
	"xiaozhizhang/system/server/internal/model/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ServerController 服务器控制器
type ServerController struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
}

// NewServerController 创建服务器控制器
func NewServerController(app *internalapp.App) *ServerController {
	return &ServerController{
		app: app,
		err: errorc.NewErrorBuilder("ServerController"),
	}
}

// RegisterRoutes 注册路由
func (c *ServerController) RegisterRoutes(admin fiber.Router) {
	serverRouter := admin.Group("/servers")
	
	// 后台管理路由
	serverRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetAll)
	serverRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetByID)
	serverRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.Create)
	serverRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.Update)
	serverRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.Delete)
	
	// 状态查询接口（主页用）
	serverRouter.Get("/status", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetAllStatus)
}

// GetAll 获取所有服务器（分页）
func (c *ServerController) GetAll(ctx *fiber.Ctx) error {
	// 解析查询参数
	var req dto.QueryServerRequest
	if err := ctx.QueryParser(&req); err != nil {
		return c.err.New("解析查询参数失败", err).ValidWithCtx()
	}

	// 默认分页参数
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.Size <= 0 {
		req.Size = 10
	}

	// 查询
	servers, total, err := c.app.QueryServers(ctx.UserContext(), &req)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": servers,
	})
}

// GetByID 根据 ID 获取服务器
func (c *ServerController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	server, err := c.app.GetServer(ctx.UserContext(), id)
	if err != nil {
		return err
	}

	return result.OK(ctx, server)
}

// Create 创建服务器
func (c *ServerController) Create(ctx *fiber.Ctx) error {
	var req dto.CreateServerRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求体失败", err).ValidWithCtx()
	}

	// 参数校验
	if _, err := utils.Validate(&req); err != nil {
		return c.err.New("参数校验失败", err).ValidWithCtx()
	}

	server, err := c.app.CreateServer(ctx.UserContext(), &req)
	if err != nil {
		return err
	}

	return result.OK(ctx, server)
}

// Update 更新服务器
func (c *ServerController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	var req dto.UpdateServerRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求体失败", err).ValidWithCtx()
	}

	// 参数校验
	if _, err := utils.Validate(&req); err != nil {
		return c.err.New("参数校验失败", err).ValidWithCtx()
	}

	if err := c.app.UpdateServer(ctx.UserContext(), id, &req); err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{"message": "更新成功"})
}

// Delete 删除服务器
func (c *ServerController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	if err := c.app.DeleteServer(ctx.UserContext(), id); err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{"message": "删除成功"})
}

// GetAllStatus 获取所有服务器状态（主页用）
func (c *ServerController) GetAllStatus(ctx *fiber.Ctx) error {
	servers, err := c.app.GetAllServerStatus(ctx.UserContext())
	if err != nil {
		return err
	}

	return result.OK(ctx, servers)
}

