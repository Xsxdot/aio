package controller

import (
	"strconv"

	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/result"
	"github.com/xsxdot/aio/pkg/core/util"
	"github.com/xsxdot/aio/system/workflow/api/dto"
	"github.com/xsxdot/aio/system/workflow/internal/app"
	"github.com/xsxdot/aio/utils"

	"github.com/gofiber/fiber/v2"
)

type WorkflowAdminController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

func NewWorkflowAdminController(a *app.App) *WorkflowAdminController {
	return &WorkflowAdminController{
		app: a,
		err: errorc.NewErrorBuilder("WorkflowAdminController"),
		log: logger.GetLogger().WithEntryName("WorkflowAdminController"),
	}
}

func (ctrl *WorkflowAdminController) RegisterRoutes(admin fiber.Router) {
	router := admin.Group("/workflow")

	router.Post("/defs", base.AdminAuth.RequireAdminAuth("admin:workflow:create"), ctrl.CreateDef)
	router.Post("/instances/:id/rollback", base.AdminAuth.RequireAdminAuth("admin:workflow:update"), ctrl.Rollback)
	router.Get("/instances/:id", base.AdminAuth.RequireAdminAuth("admin:workflow:read"), ctrl.GetInstance)
	router.Get("/instances/:id/trail", base.AdminAuth.RequireAdminAuth("admin:workflow:read"), ctrl.GetExecutionTrail)
}

func (ctrl *WorkflowAdminController) CreateDef(c *fiber.Ctx) error {
	var req dto.CreateDefRequest
	if err := c.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(c)).ToLog(ctrl.log.GetLogger())
	}
	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(c)).ToLog(ctrl.log.GetLogger())
	}
	if req.Version <= 0 {
		req.Version = 1
	}
	id, err := ctrl.app.CreateDef(util.Context(c), req.Code, req.Name, req.DAGJSON, req.Version)
	if err != nil {
		return err
	}
	return result.OK(c, fiber.Map{"id": id})
}

func (ctrl *WorkflowAdminController) Rollback(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("实例ID参数错误", err).WithTraceID(util.Context(c))
	}
	var req dto.RollbackRequest
	if err := c.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(c)).ToLog(ctrl.log.GetLogger())
	}
	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(c)).ToLog(ctrl.log.GetLogger())
	}
	env := req.Env
	if env == "" {
		env = base.ENV
	}
	if err := ctrl.app.RollbackToNode(util.Context(c), id, req.TargetNodeID, env); err != nil {
		return err
	}
	return result.OK(c, fiber.Map{"msg": "回滚成功"})
}

func (ctrl *WorkflowAdminController) GetInstance(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("实例ID参数错误", err).WithTraceID(util.Context(c))
	}
	inst, err := ctrl.app.GetInstance(util.Context(c), id)
	if err != nil {
		return err
	}
	return result.OK(c, inst)
}

func (ctrl *WorkflowAdminController) GetExecutionTrail(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("实例ID参数错误", err).WithTraceID(util.Context(c))
	}
	trail, err := ctrl.app.GetExecutionTrail(util.Context(c), id)
	if err != nil {
		return err
	}
	return result.OK(c, trail)
}
