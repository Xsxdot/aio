package http

import (
	"strconv"

	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/system/application/internal/app"
	"xiaozhizhang/system/application/internal/model"
	"xiaozhizhang/system/application/internal/model/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ApplicationController Application 后台管理控制器
type ApplicationController struct {
	app *app.App
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewApplicationController 创建 Application 控制器实例
func NewApplicationController(app *app.App) *ApplicationController {
	return &ApplicationController{
		app: app,
		log: logger.GetLogger().WithEntryName("ApplicationController"),
		err: errorc.NewErrorBuilder("ApplicationController"),
	}
}

// RegisterRoutes 注册 Application 相关路由
func (c *ApplicationController) RegisterRoutes(admin fiber.Router) {
	apps := admin.Group("/applications")

	// 应用 CRUD
	apps.Post("/", base.AdminAuth.RequireAdminAuth("admin:application:create"), c.CreateApplication)
	apps.Get("/", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.ListApplications)
	apps.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.GetApplication)
	apps.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:application:update"), c.UpdateApplication)
	apps.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:application:delete"), c.DeleteApplication)

	// 产物管理
	apps.Get("/:id/artifacts", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.ListArtifacts)
	apps.Post("/:id/artifacts", base.AdminAuth.RequireAdminAuth("admin:application:deploy"), c.UploadArtifact)

	// 版本管理
	apps.Get("/:id/releases", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.ListReleases)

	// 部署操作
	apps.Post("/:id/deploy", base.AdminAuth.RequireAdminAuth("admin:application:deploy"), c.Deploy)
	apps.Post("/:id/rollback", base.AdminAuth.RequireAdminAuth("admin:application:deploy"), c.Rollback)

	// 部署记录
	apps.Get("/:id/deployments", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.ListDeployments)

	// 独立的部署详情接口
	admin.Get("/deployments/:id", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.GetDeployment)

	// 独立的产物详情和删除接口
	admin.Get("/artifacts/:id", base.AdminAuth.RequireAdminAuth("admin:application:read"), c.GetArtifact)
	admin.Delete("/artifacts/:id", base.AdminAuth.RequireAdminAuth("admin:application:delete"), c.DeleteArtifact)
}

// CreateApplication 创建应用
func (c *ApplicationController) CreateApplication(ctx *fiber.Ctx) error {
	var req dto.CreateApplicationRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	application, err := c.app.CreateApplication(ctx.UserContext(), &req)
	return result.Once(ctx, application, err)
}

// ListApplications 列出应用
func (c *ApplicationController) ListApplications(ctx *fiber.Ctx) error {
	req := &dto.QueryApplicationRequest{
		Project: ctx.Query("project"),
		Env:     ctx.Query("env"),
		Type:    ctx.Query("type"),
		Keyword: ctx.Query("keyword"),
	}
	req.PageNum, _ = strconv.Atoi(ctx.Query("page", "1"))
	req.Size, _ = strconv.Atoi(ctx.Query("page_size", "20"))

	apps, err := c.app.ListApplications(ctx.UserContext(), req)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(apps),
		"content": apps,
	})
}

// GetApplication 获取应用详情
func (c *ApplicationController) GetApplication(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	application, err := c.app.GetApplication(ctx.UserContext(), id)
	return result.Once(ctx, application, err)
}

// UpdateApplication 更新应用
func (c *ApplicationController) UpdateApplication(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	var req dto.UpdateApplicationRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	application, err := c.app.UpdateApplication(ctx.UserContext(), id, &req)
	return result.Once(ctx, application, err)
}

// DeleteApplication 删除应用
func (c *ApplicationController) DeleteApplication(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	err = c.app.DeleteApplication(ctx.UserContext(), id)
	return result.Once(ctx, nil, err)
}

// ListReleases 列出版本
func (c *ApplicationController) ListReleases(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))

	releases, err := c.app.ListReleases(ctx.UserContext(), id, limit)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(releases),
		"content": releases,
	})
}

// Deploy 触发部署
func (c *ApplicationController) Deploy(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	var req dto.DeployRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	req.ApplicationID = id

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	deployment, err := c.app.Deploy(ctx.UserContext(), &req)
	return result.Once(ctx, deployment, err)
}

// Rollback 回滚
func (c *ApplicationController) Rollback(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	var req dto.RollbackRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).ValidWithCtx()
	}

	req.ApplicationID = id

	if _, err := utils.Validate(req); err != nil {
		return c.err.New("参数验证失败", err).ValidWithCtx()
	}

	deployment, err := c.app.Rollback(ctx.UserContext(), &req)
	return result.Once(ctx, deployment, err)
}

// ListDeployments 列出部署记录
func (c *ApplicationController) ListDeployments(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))

	deployments, err := c.app.ListDeployments(ctx.UserContext(), id, limit)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(deployments),
		"content": deployments,
	})
}

// GetDeployment 获取部署详情
func (c *ApplicationController) GetDeployment(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	deployment, err := c.app.GetDeployment(ctx.UserContext(), id)
	return result.Once(ctx, deployment, err)
}

// ListArtifacts 列出应用的产物
func (c *ApplicationController) ListArtifacts(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	artifacts, err := c.app.ListArtifacts(ctx.UserContext(), id)
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(artifacts),
		"content": artifacts,
	})
}

// UploadArtifact 上传产物
func (c *ApplicationController) UploadArtifact(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	// 获取上传的文件
	file, err := ctx.FormFile("file")
	if err != nil {
		return c.err.New("获取上传文件失败", err).ValidWithCtx()
	}

	// 获取产物类型
	artifactType := ctx.FormValue("type", "backend")

	// 打开文件
	src, err := file.Open()
	if err != nil {
		return c.err.New("打开上传文件失败", err).ValidWithCtx()
	}
	defer src.Close()

	// 调用 app 上传
	req := &app.UploadArtifactRequest{
		ApplicationID: id,
		Type:          model.ArtifactType(artifactType),
		FileName:      file.Filename,
		Size:          file.Size,
		ContentType:   file.Header.Get("Content-Type"),
		Reader:        src,
	}

	artifact, err := c.app.UploadArtifact(ctx.UserContext(), req)
	return result.Once(ctx, artifact, err)
}

// GetArtifact 获取产物详情
func (c *ApplicationController) GetArtifact(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	artifact, err := c.app.GetArtifact(ctx.UserContext(), id)
	return result.Once(ctx, artifact, err)
}

// DeleteArtifact 删除产物
func (c *ApplicationController) DeleteArtifact(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的 ID", err).ValidWithCtx()
	}

	err = c.app.DeleteArtifact(ctx.UserContext(), id)
	return result.Once(ctx, nil, err)
}

