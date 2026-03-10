package controller

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/result"
	"github.com/xsxdot/aio/pkg/core/util"
	"github.com/xsxdot/aio/system/executor/api/dto"
	"github.com/xsxdot/aio/system/executor/internal/app"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/aio/utils"

	"github.com/gofiber/fiber/v2"
)

// ExecutorAdminController 任务管理控制器
type ExecutorAdminController struct {
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewExecutorAdminController 创建任务管理控制器实例
func NewExecutorAdminController(app *app.App) *ExecutorAdminController {
	return &ExecutorAdminController{
		app: app,
		err: errorc.NewErrorBuilder("ExecutorAdminController"),
		log: logger.GetLogger().WithEntryName("ExecutorAdminController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *ExecutorAdminController) RegisterRoutes(admin fiber.Router) {
	executorRouter := admin.Group("/executor")

	// 任务管理接口
	executorRouter.Post("/jobs", base.AdminAuth.RequireAdminAuth("admin:executor:submit"), ctrl.SubmitJob)
	executorRouter.Get("/jobs", base.AdminAuth.RequireAdminAuth("admin:executor:read"), ctrl.ListJobs)
	executorRouter.Get("/jobs/:id", base.AdminAuth.RequireAdminAuth("admin:executor:read"), ctrl.GetJob)
	executorRouter.Post("/jobs/:id/cancel", base.AdminAuth.RequireAdminAuth("admin:executor:cancel"), ctrl.CancelJob)
	executorRouter.Post("/jobs/:id/requeue", base.AdminAuth.RequireAdminAuth("admin:executor:requeue"), ctrl.RequeueJob)
	executorRouter.Put("/jobs/:id/args", base.AdminAuth.RequireAdminAuth("admin:executor:update"), ctrl.UpdateJobArgs)
	executorRouter.Get("/jobs/:id/attempts", base.AdminAuth.RequireAdminAuth("admin:executor:read"), ctrl.GetJobAttempts)
	
	// 统计信息接口
	executorRouter.Get("/stats", base.AdminAuth.RequireAdminAuth("admin:executor:read"), ctrl.GetStats)
	
	// 清理任务接口
	executorRouter.Post("/cleanup", base.AdminAuth.RequireAdminAuth("admin:executor:cleanup"), ctrl.CleanupJobs)
}

// SubmitJob 提交任务
func (ctrl *ExecutorAdminController) SubmitJob(ctx *fiber.Ctx) error {
	var req dto.SubmitJobRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	jobID, err := ctrl.app.JobService.SubmitJob(
		util.Context(ctx),
		req.Env,
		req.TargetService,
		req.Method,
		req.ArgsJSON,
		req.RunAt,
		req.MaxAttempts,
		req.Priority,
		req.DedupKey,
	)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"job_id": jobID,
	})
}

// ListJobs 列出任务
func (ctrl *ExecutorAdminController) ListJobs(ctx *fiber.Ctx) error {
	var req dto.ListJobsRequest
	if err := ctx.QueryParser(&req); err != nil {
		return ctrl.err.New("解析查询参数失败", err).WithTraceID(util.Context(ctx))
	}

	if strings.TrimSpace(req.Env) == "" {
		return ctrl.err.New("env 不能为空", nil).WithTraceID(util.Context(ctx))
	}

	// 转换状态
	var status model.JobStatus
	if req.Status != "" {
		status = model.JobStatus(req.Status)
	}

	jobs, total, err := ctrl.app.JobService.ListJobs(
		util.Context(ctx),
		req.Env,
		req.TargetService,
		status,
		req.PageNum,
		req.PageSize,
	)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": jobs,
	})
}

// GetJob 获取任务详情
func (ctrl *ExecutorAdminController) GetJob(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	job, err := ctrl.app.JobService.GetJob(util.Context(ctx), id)
	return result.Once(ctx, job, err)
}

// CancelJob 取消任务
func (ctrl *ExecutorAdminController) CancelJob(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	err = ctrl.app.JobService.CancelJob(util.Context(ctx), id)
	return result.Once(ctx, "任务取消成功", err)
}

// RequeueJob 重新入队任务
func (ctrl *ExecutorAdminController) RequeueJob(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req dto.RequeueJobRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	err = ctrl.app.JobService.RequeueJob(util.Context(ctx), id, req.RunAt)
	return result.Once(ctx, "任务重新入队成功", err)
}

// UpdateJobArgs 更新任务参数
func (ctrl *ExecutorAdminController) UpdateJobArgs(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req dto.UpdateJobArgsRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	// 校验 JSON 合法性（非空时）
	if req.ArgsJSON != "" {
		if !json.Valid([]byte(req.ArgsJSON)) {
			return ctrl.err.New("参数 JSON 格式不合法", nil).WithTraceID(util.Context(ctx))
		}
	}

	err = ctrl.app.JobService.UpdateJobArgsJSON(util.Context(ctx), id, req.ArgsJSON)
	return result.Once(ctx, "任务参数更新成功", err)
}

// GetJobAttempts 获取任务的所有尝试记录
func (ctrl *ExecutorAdminController) GetJobAttempts(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctrl.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	attempts, err := ctrl.app.JobAttemptService.ListByJobID(util.Context(ctx), id)
	return result.Once(ctx, attempts, err)
}

// GetStats 获取统计信息
func (ctrl *ExecutorAdminController) GetStats(ctx *fiber.Ctx) error {
	var req dto.GetStatsRequest
	if err := ctx.QueryParser(&req); err != nil {
		return ctrl.err.New("解析查询参数失败", err).WithTraceID(util.Context(ctx))
	}

	if strings.TrimSpace(req.Env) == "" {
		return ctrl.err.New("env 不能为空", nil).WithTraceID(util.Context(ctx))
	}

	stats, err := ctrl.app.JobService.GetStats(util.Context(ctx), req.Env)
	return result.Once(ctx, stats, err)
}

// CleanupJobs 清理旧任务
func (ctrl *ExecutorAdminController) CleanupJobs(ctx *fiber.Ctx) error {
	var req dto.CleanupJobsRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(ctrl.log.GetLogger())
	}

	deleted, err := ctrl.app.JobService.CleanupOldJobs(util.Context(ctx), req.Env, req.SucceededDays, req.CanceledDays, req.DeadDays)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"deleted": deleted,
		"message": "清理完成",
	})
}
