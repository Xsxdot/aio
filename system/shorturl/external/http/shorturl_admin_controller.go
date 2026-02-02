package http

import (
	"fmt"
	"strconv"
	"time"
	"xiaozhizhang/base"
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

// ShortURLAdminController 短网址后台管理控制器
type ShortURLAdminController struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewShortURLAdminController 创建短网址后台管理控制器
func NewShortURLAdminController(app *internalapp.App) *ShortURLAdminController {
	return &ShortURLAdminController{
		app: app,
		err: errorc.NewErrorBuilder("ShortURLAdminController"),
		log: logger.GetLogger().WithEntryName("ShortURLAdminController"),
	}
}

// RegisterRoutes 注册路由
func (c *ShortURLAdminController) RegisterRoutes(admin fiber.Router) {
	// 短域名管理
	domainRouter := admin.Group("/short-domains")
	domainRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:shorturl:domain:create"), c.CreateDomain)
	domainRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:shorturl:domain:read"), c.ListDomains)
	domainRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:shorturl:domain:read"), c.GetDomain)
	domainRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:shorturl:domain:update"), c.UpdateDomain)
	domainRouter.Put("/:id/status", base.AdminAuth.RequireAdminAuth("admin:shorturl:domain:update"), c.UpdateDomainStatus)
	domainRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:shorturl:domain:delete"), c.DeleteDomain)

	// 短链接管理
	linkRouter := admin.Group("/short-links")
	linkRouter.Post("/", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:create"), c.CreateLink)
	linkRouter.Get("/", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:read"), c.ListLinks)
	linkRouter.Get("/:id", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:read"), c.GetLink)
	linkRouter.Put("/:id", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:update"), c.UpdateLink)
	linkRouter.Put("/:id/status", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:update"), c.UpdateLinkStatus)
	linkRouter.Delete("/:id", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:delete"), c.DeleteLink)
	linkRouter.Get("/:id/stats", base.AdminAuth.RequireAdminAuth("admin:shorturl:link:read"), c.GetLinkStats)
}

// CreateDomain 创建短域名
func (c *ShortURLAdminController) CreateDomain(ctx *fiber.Ctx) error {
	var req struct {
		Domain    string `json:"domain" validate:"required"`
		IsDefault bool   `json:"isDefault"`
		Comment   string `json:"comment"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	domain := &model.ShortDomain{
		Domain:    req.Domain,
		Enabled:   true,
		IsDefault: req.IsDefault,
		Comment:   req.Comment,
	}

	if err := c.app.DomainService.Create(util.Context(ctx), domain); err != nil {
		return err
	}

	return result.OK(ctx, domain)
}

// ListDomains 查询域名列表
func (c *ShortURLAdminController) ListDomains(ctx *fiber.Ctx) error {
	var domains []*model.ShortDomain
	err := c.app.DomainService.Dao.DB.WithContext(util.Context(ctx)).Find(&domains).Error
	if err != nil {
		return err
	}

	return result.OK(ctx, fiber.Map{
		"total":   len(domains),
		"content": domains,
	})
}

// GetDomain 获取域名详情
func (c *ShortURLAdminController) GetDomain(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	domain, err := c.app.DomainService.FindById(util.Context(ctx), id)
	if err != nil {
		return err
	}

	return result.OK(ctx, domain)
}

// UpdateDomain 更新域名
func (c *ShortURLAdminController) UpdateDomain(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req struct {
		Domain    string `json:"domain"`
		IsDefault *bool  `json:"isDefault"`
		Comment   string `json:"comment"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	domain, err := c.app.DomainService.FindById(util.Context(ctx), id)
	if err != nil {
		return err
	}

	if req.Domain != "" {
		domain.Domain = req.Domain
	}
	if req.IsDefault != nil {
		domain.IsDefault = *req.IsDefault
	}
	if req.Comment != "" {
		domain.Comment = req.Comment
	}

	err = c.app.DomainService.Dao.DB.WithContext(util.Context(ctx)).Save(domain).Error
	return result.Once(ctx, "更新成功", err)
}

// UpdateDomainStatus 更新域名状态
func (c *ShortURLAdminController) UpdateDomainStatus(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	domain, err := c.app.DomainService.FindById(util.Context(ctx), id)
	if err != nil {
		return err
	}

	domain.Enabled = req.Enabled
	err = c.app.DomainService.Dao.DB.WithContext(util.Context(ctx)).Save(domain).Error
	return result.Once(ctx, "更新状态成功", err)
}

// DeleteDomain 删除域名
func (c *ShortURLAdminController) DeleteDomain(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	err = c.app.DomainService.DeleteById(util.Context(ctx), id)
	return result.Once(ctx, "删除成功", err)
}

// CreateLink 创建短链接
func (c *ShortURLAdminController) CreateLink(ctx *fiber.Ctx) error {
	var req internalapp.CreateShortLinkRequest

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return c.err.New(errMsg, err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	link, err := c.app.CreateShortLink(util.Context(ctx), &req)
	if err != nil {
		return err
	}

	// 查询域名信息以构建完整URL
	domain, err := c.app.DomainService.FindById(util.Context(ctx), link.DomainID)
	if err != nil {
		return err
	}

	response := c.convertToDTO(link, domain)
	return result.OK(ctx, response)
}

// ListLinks 查询短链接列表
func (c *ShortURLAdminController) ListLinks(ctx *fiber.Ctx) error {
	domainID, _ := strconv.ParseInt(ctx.Query("domainId"), 10, 64)
	pageNum, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("size", "20"))

	if domainID <= 0 {
		return c.err.New("domainId参数必填", nil).ValidWithCtx()
	}

	links, total, err := c.app.LinkService.Dao.ListByDomainWithPage(util.Context(ctx), domainID, pageNum, pageSize)
	if err != nil {
		return err
	}

	// 查询域名信息
	domain, err := c.app.DomainService.FindById(util.Context(ctx), domainID)
	if err != nil {
		return err
	}

	content := make([]*dto.ShortLinkDTO, 0, len(links))
	for _, link := range links {
		content = append(content, c.convertToDTO(link, domain))
	}

	return result.OK(ctx, fiber.Map{
		"total":   total,
		"content": content,
	})
}

// GetLink 获取短链接详情
func (c *ShortURLAdminController) GetLink(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	link, err := c.app.LinkService.FindById(util.Context(ctx), id)
	if err != nil {
		return err
	}

	domain, err := c.app.DomainService.FindById(util.Context(ctx), link.DomainID)
	if err != nil {
		return err
	}

	return result.OK(ctx, c.convertToDTO(link, domain))
}

// UpdateLink 更新短链接
func (c *ShortURLAdminController) UpdateLink(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req struct {
		TargetConfig map[string]interface{} `json:"targetConfig"`
		ExpiresAt    *int64                 `json:"expiresAt"`
		MaxVisits    *int64                 `json:"maxVisits"`
		Comment      string                 `json:"comment"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	// 转换为 App 层请求结构
	appReq := &internalapp.UpdateShortLinkRequest{
		TargetConfig: req.TargetConfig,
		MaxVisits:    req.MaxVisits,
		Comment:      req.Comment,
	}
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0)
		appReq.ExpiresAt = &t
	}

	// 调用 App 层更新方法（会自动清除缓存）
	err = c.app.UpdateShortLink(util.Context(ctx), id, appReq)
	return result.Once(ctx, "更新成功", err)
}

// UpdateLinkStatus 更新短链接状态
func (c *ShortURLAdminController) UpdateLinkStatus(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return c.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx)).ToLog(c.log.GetLogger())
	}

	// 调用 App 层更新状态方法（会自动清除缓存）
	err = c.app.UpdateShortLinkStatus(util.Context(ctx), id, req.Enabled)
	return result.Once(ctx, "更新状态成功", err)
}

// DeleteLink 删除短链接
func (c *ShortURLAdminController) DeleteLink(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	// 调用 App 层删除方法（会自动清除缓存）
	err = c.app.DeleteShortLink(util.Context(ctx), id)
	return result.Once(ctx, "删除成功", err)
}

// GetLinkStats 获取短链接统计
func (c *ShortURLAdminController) GetLinkStats(ctx *fiber.Ctx) error {
	id, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		return c.err.New("ID参数错误", err).WithTraceID(util.Context(ctx))
	}

	days, _ := strconv.Atoi(ctx.Query("days", "30"))

	stats, err := c.app.GetShortLinkStats(util.Context(ctx), id, days)
	if err != nil {
		return err
	}

	// 转换为DTO
	response := &dto.ShortLinkStatsDTO{
		TotalVisits:   stats.TotalVisits,
		TotalSuccess:  stats.TotalSuccess,
		DailyStats:    make([]dto.DailyStatDTO, 0, len(stats.DailyStats)),
		RecentVisits:  make([]dto.VisitRecordDTO, 0, len(stats.RecentVisits)),
		RecentSuccess: make([]dto.SuccessEventRecordDTO, 0, len(stats.RecentSuccess)),
	}

	for _, ds := range stats.DailyStats {
		response.DailyStats = append(response.DailyStats, dto.DailyStatDTO{
			Date:         ds.Date,
			VisitCount:   ds.VisitCount,
			SuccessCount: ds.SuccessCount,
		})
	}

	for _, v := range stats.RecentVisits {
		response.RecentVisits = append(response.RecentVisits, dto.VisitRecordDTO{
			ID:        v.ID,
			IP:        v.IP,
			UserAgent: v.UserAgent,
			Referer:   v.Referer,
			VisitedAt: v.VisitedAt,
		})
	}

	for _, e := range stats.RecentSuccess {
		attrs := e.Attrs
		if attrs == nil {
			attrs = make(map[string]interface{})
		}
		response.RecentSuccess = append(response.RecentSuccess, dto.SuccessEventRecordDTO{
			ID:        e.ID,
			EventID:   e.EventID,
			Attrs:     attrs,
			CreatedAt: e.CreatedAt,
		})
	}

	return result.OK(ctx, response)
}

// convertToDTO 将模型转换为DTO
func (c *ShortURLAdminController) convertToDTO(link *model.ShortLink, domain *model.ShortDomain) *dto.ShortLinkDTO {
	targetConfig := make(map[string]interface{})
	if link.TargetConfig != nil {
		targetConfig = link.TargetConfig
	}

	return &dto.ShortLinkDTO{
		ID:           link.ID,
		DomainID:     link.DomainID,
		Domain:       domain.Domain,
		Code:         link.Code,
		ShortURL:     fmt.Sprintf("https://%s/%s", domain.Domain, link.Code),
		TargetType:   string(link.TargetType),
		Url:          link.URL,
		BackupUrl:    link.BackupURL,
		TargetConfig: targetConfig,
		ExpiresAt:    link.ExpiresAt,
		MaxVisits:    link.MaxVisits,
		VisitCount:   link.VisitCount,
		SuccessCount: link.SuccessCount,
		HasPassword:  link.HasPassword(),
		Enabled:      link.Enabled,
		Comment:      link.Comment,
		CreatedAt:    link.CreatedAt,
		UpdatedAt:    link.UpdatedAt,
	}
}

