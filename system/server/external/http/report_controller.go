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

// ReportController 状态上报控制器
type ReportController struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
}

// NewReportController 创建状态上报控制器
func NewReportController(app *internalapp.App) *ReportController {
	return &ReportController{
		app: app,
		err: errorc.NewErrorBuilder("ReportController"),
	}
}

// RegisterRoutes 注册路由
func (c *ReportController) RegisterRoutes(api fiber.Router) {
	// 上报接口（需要 Client 鉴权）
	reportRouter := api.Group("/servers")
	reportRouter.Post("/:id/status/report", base.ClientAuth.RequireClientAuth(), c.ReportStatus)
}

// ReportStatus 上报服务器状态
func (c *ReportController) ReportStatus(ctx *fiber.Ctx) error {
	// 解析服务器 ID
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("无效的服务器 ID", err).ValidWithCtx()
	}

	// 解析请求体
	var req dto.ReportServerStatusRequest
	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求体失败", err).ValidWithCtx()
	}

	// 设置服务器 ID
	req.ServerID = id

	// 参数校验
	if _, err := utils.Validate(&req); err != nil {
		return c.err.New("参数校验失败", err).ValidWithCtx()
	}

	// 上报状态
	if err := c.app.ReportServerStatus(ctx.UserContext(), &req); err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{"message": "上报成功"})
}

