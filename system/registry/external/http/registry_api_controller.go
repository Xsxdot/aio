package http

import (
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/registry/api/client"
	"xiaozhizhang/system/registry/api/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// RegistryAPIController 注册中心对外接口控制器
// 提供实例注册/心跳/下线，以及 agent 拉取服务信息接口。
type RegistryAPIController struct {
	client *client.RegistryClient
	err    *errorc.ErrorBuilder
	log    *logger.Log
}

func NewRegistryAPIController(client *client.RegistryClient) *RegistryAPIController {
	return &RegistryAPIController{
		client: client,
		err:    errorc.NewErrorBuilder("RegistryAPIController"),
		log:    logger.GetLogger().WithEntryName("RegistryAPIController"),
	}
}

func (c *RegistryAPIController) RegisterRoutes(api fiber.Router) {
	r := api.Group("/registry/v1")

	// agent 拉取
	r.Get("/services", c.ListServices)
	r.Get("/services/:id", c.GetServiceByID)

	// 实例注册/心跳/下线：需要客户端鉴权
	inst := r.Group("/instances")
	inst.Post("/register", base.ClientAuth.RequireClientAuth(), c.RegisterInstance)
	inst.Post("/heartbeat", base.ClientAuth.RequireClientAuth(), c.Heartbeat)
	inst.Post("/deregister", base.ClientAuth.RequireClientAuth(), c.Deregister)
}

func (c *RegistryAPIController) ListServices(ctx *fiber.Ctx) error {
	project := ctx.Query("project")
	env := ctx.Query("env")

	list, err := c.client.ListServices(ctx.UserContext(), project, env)
	if err != nil {
		return err
	}
	return result.OK(ctx, fiber.Map{"total": len(list), "content": list})
}

func (c *RegistryAPIController) GetServiceByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	item, err := c.client.GetServiceByID(ctx.UserContext(), int64(id))
	return result.Once(ctx, item, err)
}

func (c *RegistryAPIController) RegisterInstance(ctx *fiber.Ctx) error {
	var req dto.RegisterInstanceReq
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}
	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	resp, err := c.client.RegisterInstance(ctx.UserContext(), &req)
	if err != nil {
		return err
	}
	return result.OK(ctx, resp)
}

func (c *RegistryAPIController) Heartbeat(ctx *fiber.Ctx) error {
	var req dto.HeartbeatReq
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}
	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	resp, err := c.client.HeartbeatInstance(ctx.UserContext(), &req)
	if err != nil {
		return err
	}
	return result.OK(ctx, resp)
}

func (c *RegistryAPIController) Deregister(ctx *fiber.Ctx) error {
	var req dto.DeregisterInstanceReq
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}
	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	err := c.client.DeregisterInstance(ctx.UserContext(), &req)
	return result.Once(ctx, fiber.Map{"message": "ok"}, err)
}
