package http

import (
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/shorturl/api/dto"
	internalapp "xiaozhizhang/system/shorturl/internal/app"
	"xiaozhizhang/system/shorturl/internal/model"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ShortURLAPIController 短网址API控制器（跳转与上报）
type ShortURLAPIController struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewShortURLAPIController 创建短网址API控制器
func NewShortURLAPIController(app *internalapp.App) *ShortURLAPIController {
	return &ShortURLAPIController{
		app: app,
		err: errorc.NewErrorBuilder("ShortURLAPIController"),
		log: logger.GetLogger().WithEntryName("ShortURLAPIController"),
	}
}

// RegisterRoutes 注册路由
func (c *ShortURLAPIController) RegisterRoutes(api fiber.Router) {
	// 短链接访问与跳转（无鉴权）
	api.Get("/s/:code", c.Visit)

	// 短链接解析（返回JSON，供第三方页面使用，无鉴权）
	api.Get("/s/:code/resolve", c.ResolveJSON)

	// 跳转成功上报（无鉴权）
	api.Post("/s/:code/success", c.ReportSuccess)
}

// Visit 访问短链接并跳转
func (c *ShortURLAPIController) Visit(ctx *fiber.Ctx) error {
	code := ctx.Params("code")
	password := ctx.Query("password", "")
	host := ctx.Hostname()

	// 解析短链接
	link, domain, err := c.app.ResolveShortLink(util.Context(ctx), host, code, password)
	if err != nil {
		return err
	}

	// 记录访问
	ip := ctx.IP()
	ua := ctx.Get("User-Agent")
	referer := ctx.Get("Referer")

	if err := c.app.VisitShortLink(util.Context(ctx), link, ip, ua, referer); err != nil {
		c.log.WithErr(err).Error("记录访问失败")
		// 不阻断跳转流程
	}

	// 根据目标类型决定跳转方式
	if link.TargetType == model.TargetTypeURL {
		// 普通 URL 直接 302 跳转
		// 优先使用 URL 字段，为空则回退到 TargetConfig["url"]
		targetURL := link.URL
		if targetURL == "" && link.TargetConfig != nil {
			if v, ok := link.TargetConfig["url"].(string); ok {
				targetURL = v
			}
		}
		if targetURL == "" {
			return c.err.New("目标URL配置错误", nil).ValidWithCtx()
		}
		return ctx.Redirect(targetURL, fiber.StatusFound)
	}

	// 其他类型返回落地页 HTML
	html := c.app.GenerateLandingPageHTML(link, domain)
	ctx.Set("Content-Type", "text/html; charset=utf-8")
	return ctx.SendString(html)
}

// ResolveJSON 解析短链接并返回JSON（供第三方页面渲染使用）
func (c *ShortURLAPIController) ResolveJSON(ctx *fiber.Ctx) error {
	code := ctx.Params("code")
	password := ctx.Query("password", "")
	host := ctx.Hostname()

	// 解析短链接
	link, domain, err := c.app.ResolveShortLink(util.Context(ctx), host, code, password)
	if err != nil {
		return err
	}

	// 记录访问
	ip := ctx.IP()
	ua := ctx.Get("User-Agent")
	referer := ctx.Get("Referer")

	if err := c.app.VisitShortLink(util.Context(ctx), link, ip, ua, referer); err != nil {
		c.log.WithErr(err).Error("记录访问失败")
		// 不阻断返回流程
	}

	// 构建返回数据
	targetConfig := make(map[string]interface{})
	if link.TargetConfig != nil {
		targetConfig = link.TargetConfig
	}

	response := &dto.ShortLinkResolveDTO{
		Code:         link.Code,
		Domain:       domain.Domain,
		TargetType:   string(link.TargetType),
		Url:          link.URL,
		BackupUrl:    link.BackupURL,
		TargetConfig: targetConfig,
		HasPassword:  link.HasPassword(),
		Comment:      link.Comment,
	}

	return result.OK(ctx, response)
}

// ReportSuccess 上报跳转成功（无鉴权）
func (c *ShortURLAPIController) ReportSuccess(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var req struct {
		EventID string                 `json:"eventId"`
		Attrs   map[string]interface{} `json:"attrs"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	if err := c.app.ReportShortLinkSuccess(util.Context(ctx), code, req.EventID, req.Attrs); err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{"message": "上报成功"})
}
