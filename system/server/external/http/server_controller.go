package http

import (
	"strconv"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/result"
	internalapp "xiaozhizhang/system/server/internal/app"
	"xiaozhizhang/system/server/internal/model"
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

	// 状态查询接口（主页用）
	serverRouter.Get("/status", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetAllStatus)

	// 后台管理路由
	serverRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetAll)
	serverRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetByID)
	serverRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.Create)
	serverRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.Update)
	serverRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.Delete)

	// SSH 凭证管理
	serverRouter.Get("/:id/ssh", base.AdminAuth.RequireAdminAuth("admin:server:read"), c.GetSSHCredential)
	serverRouter.Put("/:id/ssh", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.UpsertSSHCredential)
	serverRouter.Delete("/:id/ssh", base.AdminAuth.RequireAdminAuth("admin:server:write"), c.DeleteSSHCredential)
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

// GetSSHCredential 获取 SSH 凭证（脱敏）
func (c *ServerController) GetSSHCredential(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	// 先检查服务器是否存在
	_, err = c.app.GetServer(ctx.UserContext(), id)
	if err != nil {
		return err
	}

	// 获取 SSH 凭证（不解密，只返回元信息）
	credential, err := c.app.GetDecryptedServerSSHCredential(ctx.UserContext(), id)
	if err != nil {
		// 如果不存在，返回空响应
		if errorc.IsNotFound(err) {
			return result.OK(ctx, nil)
		}
		return err
	}

	// 脱敏响应
	resp := &dto.ServerSSHCredentialResponse{
		ServerID:      credential.ServerID,
		Port:          credential.Port,
		Username:      credential.Username,
		AuthMethod:    credential.AuthMethod,
		HasPassword:   credential.Password != "",
		HasPrivateKey: credential.PrivateKey != "",
		Comment:       credential.Comment,
	}

	return result.OK(ctx, resp)
}

// UpsertSSHCredential 更新或插入 SSH 凭证
func (c *ServerController) UpsertSSHCredential(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	// 先检查服务器是否存在
	_, err = c.app.GetServer(ctx.UserContext(), id)
	if err != nil {
		return err
	}

	var req dto.UpsertServerSSHCredentialRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求体失败", err).ValidWithCtx()
	}

	// 参数校验
	if _, err := utils.Validate(&req); err != nil {
		return c.err.New("参数校验失败", err).ValidWithCtx()
	}

	// 校验认证方式与字段一致性
	if req.AuthMethod == "password" && req.Password == "" {
		return c.err.New("使用密码认证时必须提供密码", nil).ValidWithCtx()
	}
	if req.AuthMethod == "privatekey" && req.PrivateKey == "" {
		return c.err.New("使用私钥认证时必须提供私钥", nil).ValidWithCtx()
	}

	// 构建模型
	credential := &model.ServerSSHCredential{
		ServerID:   id,
		Port:       req.Port,
		Username:   req.Username,
		AuthMethod: req.AuthMethod,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
		Comment:    req.Comment,
	}

	if err := c.app.UpsertServerSSHCredential(ctx.UserContext(), credential); err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{"message": "保存成功"})
}

// DeleteSSHCredential 删除 SSH 凭证
func (c *ServerController) DeleteSSHCredential(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	if err := c.app.DeleteServerSSHCredential(ctx.UserContext(), id); err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{"message": "删除成功"})
}
