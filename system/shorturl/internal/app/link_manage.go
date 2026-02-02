package app

import (
	"context"
	"fmt"
	"time"
	"xiaozhizhang/base"
	"xiaozhizhang/system/shorturl/internal/model"
	"xiaozhizhang/system/shorturl/internal/service"

	"github.com/go-redis/cache/v9"
)

// CreateShortLinkRequest 创建短链接请求
type CreateShortLinkRequest struct {
	DomainID     int64                  `json:"domainId" validate:"required"`
	TargetType   model.TargetType       `json:"targetType" validate:"required"`
	TargetConfig map[string]interface{} `json:"targetConfig" validate:"required"`
	ExpiresAt    *time.Time             `json:"expiresAt"`
	Password     string                 `json:"password"`
	MaxVisits    *int64                 `json:"maxVisits"`
	CodeLength   int                    `json:"codeLength"`
	CustomCode   string                 `json:"customCode"`
	Comment      string                 `json:"comment"`
}

// UpdateShortLinkRequest 更新短链接请求
type UpdateShortLinkRequest struct {
	TargetConfig map[string]interface{} `json:"targetConfig"`
	ExpiresAt    *time.Time             `json:"expiresAt"`
	MaxVisits    *int64                 `json:"maxVisits"`
	Comment      string                 `json:"comment"`
}

// CreateShortLink 创建短链接
func (a *App) CreateShortLink(ctx context.Context, req *CreateShortLinkRequest) (*model.ShortLink, error) {
	// 校验目标类型
	if !req.TargetType.IsValid() {
		return nil, a.err.New("无效的目标类型", nil)
	}

	// 校验域名
	domain, err := a.DomainService.FindById(ctx, req.DomainID)
	if err != nil {
		return nil, err
	}
	if !domain.Enabled {
		return nil, a.err.New("域名未启用", nil)
	}

	// 生成或使用自定义短码
	var code string
	if req.CustomCode != "" {
		// 使用自定义短码，需检查是否已存在
		exists, err := a.LinkService.Dao.ExistsByDomainAndCode(ctx, req.DomainID, req.CustomCode)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, a.err.New("短码已存在", nil)
		}
		code = req.CustomCode
	} else {
		// 生成随机短码
		codeLength := req.CodeLength
		if codeLength <= 0 {
			codeLength = 6
		}
		code, err = a.LinkService.GenerateUniqueCode(ctx, req.DomainID, codeLength, 10)
		if err != nil {
			return nil, err
		}
	}

	// 处理密码
	passwordHash := a.LinkService.HashPassword(req.Password)

	// 创建短链接
	link := &model.ShortLink{
		DomainID:     req.DomainID,
		Code:         code,
		TargetType:   req.TargetType,
		TargetConfig: req.TargetConfig,
		ExpiresAt:    req.ExpiresAt,
		PasswordHash: passwordHash,
		MaxVisits:    req.MaxVisits,
		VisitCount:   0,
		SuccessCount: 0,
		Enabled:      true,
		Comment:      req.Comment,
	}

	// 从 TargetConfig 同步 URL/BackupURL 字段
	link.SyncURLsFromTargetConfig()

	if err := a.LinkService.Create(ctx, link); err != nil {
		return nil, err
	}

	// 创建成功后，预热缓存（可选）
	a.invalidateLinkCache(ctx, req.DomainID, code)

	return link, nil
}

// UpdateShortLink 更新短链接
func (a *App) UpdateShortLink(ctx context.Context, id int64, req *UpdateShortLinkRequest) error {
	// 查询短链接
	link, err := a.LinkService.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 更新字段
	if req.TargetConfig != nil {
		link.TargetConfig = req.TargetConfig
		// 同步 URL/BackupURL 字段
		link.SyncURLsFromTargetConfig()
	}
	if req.ExpiresAt != nil {
		link.ExpiresAt = req.ExpiresAt
	}
	if req.MaxVisits != nil {
		link.MaxVisits = req.MaxVisits
	}
	if req.Comment != "" {
		link.Comment = req.Comment
	}

	// 保存更新
	if err := a.LinkService.Dao.DB.WithContext(ctx).Save(link).Error; err != nil {
		return a.err.New("更新短链接失败", err).DB()
	}

	// 清除缓存
	a.invalidateLinkCache(ctx, link.DomainID, link.Code)

	return nil
}

// UpdateShortLinkStatus 更新短链接状态
func (a *App) UpdateShortLinkStatus(ctx context.Context, id int64, enabled bool) error {
	// 查询短链接
	link, err := a.LinkService.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 更新状态
	link.Enabled = enabled
	if err := a.LinkService.Dao.DB.WithContext(ctx).Save(link).Error; err != nil {
		return a.err.New("更新短链接状态失败", err).DB()
	}

	// 清除缓存
	a.invalidateLinkCache(ctx, link.DomainID, link.Code)

	return nil
}

// DeleteShortLink 删除短链接
func (a *App) DeleteShortLink(ctx context.Context, id int64) error {
	// 先查询，以便清除缓存
	link, err := a.LinkService.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 删除短链接
	if err := a.LinkService.DeleteById(ctx, id); err != nil {
		return err
	}

	// 清除缓存
	a.invalidateLinkCache(ctx, link.DomainID, link.Code)

	return nil
}

// ResolveLinkCacheData 缓存数据结构
type ResolveLinkCacheData struct {
	Link   *model.ShortLink
	Domain *model.ShortDomain
}

// ResolveShortLink 解析短链接（通过 Host + Code，带缓存）
func (a *App) ResolveShortLink(ctx context.Context, host, code, password string) (*model.ShortLink, *model.ShortDomain, error) {
	// 1. 根据 host 查找域名（域名查询也使用缓存）
	domain, err := a.resolveDomainWithCache(ctx, host)
	if err != nil {
		return nil, nil, err
	}

	// 2. 查找短链接（使用缓存）
	cacheKey := fmt.Sprintf("shorturl:domain:%d:code:%s", domain.ID, code)
	var link *model.ShortLink

	err = base.Cache.Once(&cache.Item{
		Key:   cacheKey,
		Value: &link,
		TTL:   5 * time.Minute, // 缓存5分钟
		Do: func(*cache.Item) (interface{}, error) {
			return a.LinkService.Dao.FindByDomainAndCode(ctx, domain.ID, code)
		},
	})

	if err != nil {
		return nil, nil, err
	}

	// 3. 验证短链接（不缓存验证结果，因为涉及密码等动态验证）
	if err := a.LinkService.ValidateLink(link, password); err != nil {
		return nil, nil, err
	}

	return link, domain, nil
}

// resolveDomainWithCache 根据 host 解析域名（带缓存）
func (a *App) resolveDomainWithCache(ctx context.Context, host string) (*model.ShortDomain, error) {
	cacheKey := fmt.Sprintf("shorturl:domain:host:%s", host)
	var domain *model.ShortDomain

	err := base.Cache.Once(&cache.Item{
		Key:   cacheKey,
		Value: &domain,
		TTL:   10 * time.Minute, // 域名缓存10分钟
		Do: func(*cache.Item) (interface{}, error) {
			// 先尝试根据 host 查找域名
			d, err := a.DomainService.Dao.FindByDomain(ctx, host)
			if err != nil {
				// 找不到则使用默认域名
				d, err = a.DomainService.Dao.FindDefault(ctx)
				if err != nil {
					return nil, a.err.New("未找到匹配的域名且无默认域名", nil)
				}
			}
			return d, nil
		},
	})

	if err != nil {
		return nil, err
	}

	return domain, nil
}

// VisitShortLink 访问短链接（记录访问并返回跳转信息）
func (a *App) VisitShortLink(ctx context.Context, link *model.ShortLink, ip, ua, referer string) error {
	visitDao := a.StatsService.VisitDao
	return a.LinkService.RecordVisit(ctx, link.ID, visitDao, ip, ua, referer)
}

// ReportShortLinkSuccess 上报短链接跳转成功
func (a *App) ReportShortLinkSuccess(ctx context.Context, code, eventID string, attrs map[string]interface{}) error {
	// 根据 code 查找短链接（任意域名）
	link, err := a.LinkService.Dao.FindByCode(ctx, code)
	if err != nil {
		return err
	}

	successDao := a.StatsService.SuccessEventDao
	return a.LinkService.RecordSuccess(ctx, link.ID, successDao, eventID, attrs)
}

// GetShortLinkStats 获取短链接统计
func (a *App) GetShortLinkStats(ctx context.Context, linkID int64, days int) (*ShortLinkStats, error) {
	link, err := a.LinkService.FindById(ctx, linkID)
	if err != nil {
		return nil, err
	}

	dailyStats, err := a.StatsService.GetDailyStats(ctx, linkID, days)
	if err != nil {
		return nil, err
	}

	visitDao := a.StatsService.VisitDao
	recentVisits, err := visitDao.ListByLinkID(ctx, linkID, 20)
	if err != nil {
		return nil, err
	}

	successEventDao := a.StatsService.SuccessEventDao
	recentSuccess, err := successEventDao.ListByLinkID(ctx, linkID, 20)
	if err != nil {
		return nil, err
	}

	return &ShortLinkStats{
		TotalVisits:   link.VisitCount,
		TotalSuccess:  link.SuccessCount,
		DailyStats:    dailyStats,
		RecentVisits:  recentVisits,
		RecentSuccess: recentSuccess,
	}, nil
}

// ShortLinkStats 短链接统计结果
type ShortLinkStats struct {
	TotalVisits   int64
	TotalSuccess  int64
	DailyStats    []service.DailyStat
	RecentVisits  []*model.ShortVisit
	RecentSuccess []*model.ShortSuccessEvent
}

// GenerateLandingPageHTML 生成落地页HTML
func (a *App) GenerateLandingPageHTML(link *model.ShortLink, domain *model.ShortDomain) string {
	// 根据不同的 TargetType 生成不同的落地页
	switch link.TargetType {
	case model.TargetTypeURLScheme:
		return a.generateURLSchemeLandingPage(link, domain)
	default:
		return a.generateDefaultLandingPage(link, domain)
	}
}

func (a *App) generateURLSchemeLandingPage(link *model.ShortLink, domain *model.ShortDomain) string {
	schemeURL := link.URL
	fallbackURL := link.BackupURL

	// 如果新字段为空，回退到 TargetConfig
	if schemeURL == "" && link.TargetConfig != nil {
		if v, ok := link.TargetConfig["schemeUrl"].(string); ok {
			schemeURL = v
		}
	}
	if fallbackURL == "" && link.TargetConfig != nil {
		if v, ok := link.TargetConfig["fallbackUrl"].(string); ok {
			fallbackURL = v
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>正在跳转...</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .loading { font-size: 18px; color: #666; }
    </style>
</head>
<body>
    <div class="loading">正在打开应用...</div>
    <script>
        var schemeUrl = %q;
        var fallbackUrl = %q;
        
        window.location.href = schemeUrl;
        
        setTimeout(function() {
            if (fallbackUrl) {
                window.location.href = fallbackUrl;
            } else {
                document.querySelector('.loading').innerText = '如果应用未自动打开，请手动打开应用';
            }
        }, 2000);
    </script>
</body>
</html>`, schemeURL, fallbackURL)
}

func (a *App) generateDefaultLandingPage(link *model.ShortLink, domain *model.ShortDomain) string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>跳转中</title>
</head>
<body>
    <p>跳转中...</p>
</body>
</html>`
}

// invalidateLinkCache 清除短链接缓存
func (a *App) invalidateLinkCache(ctx context.Context, domainID int64, code string) {
	cacheKey := fmt.Sprintf("shorturl:domain:%d:code:%s", domainID, code)
	if err := base.Cache.Delete(ctx, cacheKey); err != nil {
		a.log.WithErr(err).WithField("cache_key", cacheKey).Warn("清除短链接缓存失败")
	}
}

// invalidateDomainCache 清除域名缓存
func (a *App) invalidateDomainCache(ctx context.Context, host string) {
	cacheKey := fmt.Sprintf("shorturl:domain:host:%s", host)
	if err := base.Cache.Delete(ctx, cacheKey); err != nil {
		a.log.WithErr(err).WithField("cache_key", cacheKey).Warn("清除域名缓存失败")
	}
}

