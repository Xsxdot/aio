package http

import (
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/registry/api/dto"
	internalapp "xiaozhizhang/system/registry/internal/app"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// RegistryAdminController 注册中心后台管理控制器
// 负责维护服务定义（spec 等）。
type RegistryAdminController struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

func NewRegistryAdminController(app *internalapp.App) *RegistryAdminController {
	return &RegistryAdminController{
		app: app,
		err: errorc.NewErrorBuilder("RegistryAdminController"),
		log: logger.GetLogger().WithEntryName("RegistryAdminController"),
	}
}

func (c *RegistryAdminController) RegisterRoutes(admin fiber.Router) {
	r := admin.Group("/registry")
	svc := r.Group("/services")

	svc.Post("/", base.AdminAuth.RequireAdminAuth("admin:registry:service:create"), c.CreateService)
	svc.Get("/", base.AdminAuth.RequireAdminAuth("admin:registry:service:read"), c.ListServices)
	svc.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:registry:service:read"), c.GetService)
	svc.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:registry:service:update"), c.UpdateService)
	svc.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:registry:service:delete"), c.DeleteService)

	// 实例管理接口
	svc.Get("/:id/instances", base.AdminAuth.RequireAdminAuth("admin:registry:instance:read"), c.ListInstances)
	svc.Post("/:id/instances/:instanceKey/offline", base.AdminAuth.RequireAdminAuth("admin:registry:instance:offline"), c.OfflineInstance)
	svc.Delete("/:id/instances/:instanceKey", base.AdminAuth.RequireAdminAuth("admin:registry:instance:delete"), c.DeleteInstance)
}

type CreateServiceRequest struct {
	Project     string                 `json:"project" validate:"required"`
	Name        string                 `json:"name" validate:"required"`
	Owner       string                 `json:"owner"`
	Description string                 `json:"description"`
	Spec        map[string]interface{} `json:"spec"`
}

func (c *RegistryAdminController) CreateService(ctx *fiber.Ctx) error {
	var req CreateServiceRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}
	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	service, err := c.app.CreateService(ctx.UserContext(), req.Project, req.Name, req.Owner, req.Description, req.Spec)
	if err != nil {
		return err
	}
	return result.OK(ctx, service)
}

func (c *RegistryAdminController) ListServices(ctx *fiber.Ctx) error {
	project := ctx.Query("project")

	services, err := c.app.ListServiceDefs(ctx.UserContext(), project)
	return result.Once(ctx, fiber.Map{"total": len(services), "content": services}, err)
}

func (c *RegistryAdminController) GetService(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	svc, err := c.app.GetServiceDefByID(ctx.UserContext(), int64(id))
	return result.Once(ctx, svc, err)
}

type UpdateServiceRequest struct {
	Project     string                 `json:"project"`
	Name        string                 `json:"name"`
	Owner       string                 `json:"owner"`
	Description string                 `json:"description"`
	Spec        map[string]interface{} `json:"spec"`
}

func (c *RegistryAdminController) UpdateService(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req UpdateServiceRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	updated, err := c.app.UpdateService(ctx.UserContext(), int64(id), req.Project, req.Name, req.Owner, req.Description, req.Spec)
	if err != nil {
		return err
	}
	return result.OK(ctx, updated)
}

func (c *RegistryAdminController) DeleteService(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	err = c.app.DeleteService(ctx.UserContext(), int64(id))
	return result.Once(ctx, fiber.Map{"message": "删除成功"}, err)
}

// ===== Instance Management =====

func (c *RegistryAdminController) ListInstances(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("服务ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	env := ctx.Query("env")
	aliveOnly := ctx.QueryBool("aliveOnly", true) // 默认只查看在线实例

	instances, err := c.app.ListInstancesForAdmin(ctx.UserContext(), int64(id), env, aliveOnly)
	return result.Once(ctx, fiber.Map{"total": len(instances), "content": instances}, err)
}

func (c *RegistryAdminController) OfflineInstance(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("服务ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	instanceKey := ctx.Params("instanceKey")
	if instanceKey == "" {
		return c.err.New("instanceKey参数不能为空", nil).WithTraceID(util.Context(ctx))
	}

	err = c.app.ForceOfflineInstance(ctx.UserContext(), int64(id), instanceKey)
	return result.Once(ctx, fiber.Map{"message": "实例已强制下线"}, err)
}

func (c *RegistryAdminController) DeleteInstance(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return c.err.New("服务ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	instanceKey := ctx.Params("instanceKey")
	if instanceKey == "" {
		return c.err.New("instanceKey参数不能为空", nil).WithTraceID(util.Context(ctx))
	}

	req := &dto.DeregisterInstanceReq{
		ServiceID:   int64(id),
		InstanceKey: instanceKey,
	}

	err = c.app.DeregisterInstance(ctx.UserContext(), req)
	return result.Once(ctx, fiber.Map{"message": "实例已删除"}, err)
}
