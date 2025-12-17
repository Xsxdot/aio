package dto

import (
	"time"
)

// ShortDomainDTO 短域名DTO
type ShortDomainDTO struct {
	ID        int64     `json:"id" comment:"ID"`
	Domain    string    `json:"domain" comment:"短域名"`
	Enabled   bool      `json:"enabled" comment:"是否启用"`
	IsDefault bool      `json:"isDefault" comment:"是否默认域名"`
	Comment   string    `json:"comment" comment:"备注"`
	CreatedAt time.Time `json:"createdAt" comment:"创建时间"`
	UpdatedAt time.Time `json:"updatedAt" comment:"更新时间"`
}

// ShortLinkDTO 短链接DTO
type ShortLinkDTO struct {
	ID           int64                  `json:"id" comment:"ID"`
	DomainID     int64                  `json:"domainId" comment:"短域名ID"`
	Domain       string                 `json:"domain" comment:"短域名"`
	Code         string                 `json:"code" comment:"短码"`
	ShortURL     string                 `json:"shortUrl" comment:"完整短链接"`
	TargetType   string                 `json:"targetType" comment:"跳转类型"`
	Url          string                 `json:"url" comment:"主URL"`
	BackupUrl    string                 `json:"backupUrl" comment:"备用URL"`
	TargetConfig map[string]interface{} `json:"targetConfig" comment:"目标配置"`
	ExpiresAt    *time.Time             `json:"expiresAt" comment:"过期时间"`
	MaxVisits    *int64                 `json:"maxVisits" comment:"最大访问次数"`
	VisitCount   int64                  `json:"visitCount" comment:"访问次数"`
	SuccessCount int64                  `json:"successCount" comment:"成功上报次数"`
	HasPassword  bool                   `json:"hasPassword" comment:"是否设置密码"`
	Enabled      bool                   `json:"enabled" comment:"是否启用"`
	Comment      string                 `json:"comment" comment:"备注"`
	CreatedAt    time.Time              `json:"createdAt" comment:"创建时间"`
	UpdatedAt    time.Time              `json:"updatedAt" comment:"更新时间"`
}

// ShortLinkStatsDTO 短链接统计DTO
type ShortLinkStatsDTO struct {
	TotalVisits   int64                   `json:"totalVisits" comment:"总访问次数"`
	TotalSuccess  int64                   `json:"totalSuccess" comment:"总成功次数"`
	DailyStats    []DailyStatDTO          `json:"dailyStats" comment:"每日统计"`
	RecentVisits  []VisitRecordDTO        `json:"recentVisits" comment:"最近访问记录"`
	RecentSuccess []SuccessEventRecordDTO `json:"recentSuccess" comment:"最近成功记录"`
}

// DailyStatDTO 每日统计DTO
type DailyStatDTO struct {
	Date         string `json:"date" comment:"日期"`
	VisitCount   int64  `json:"visitCount" comment:"访问次数"`
	SuccessCount int64  `json:"successCount" comment:"成功次数"`
}

// VisitRecordDTO 访问记录DTO
type VisitRecordDTO struct {
	ID        int64     `json:"id" comment:"ID"`
	IP        string    `json:"ip" comment:"IP地址"`
	UserAgent string    `json:"userAgent" comment:"User-Agent"`
	Referer   string    `json:"referer" comment:"Referer"`
	VisitedAt time.Time `json:"visitedAt" comment:"访问时间"`
}

// SuccessEventRecordDTO 成功事件记录DTO
type SuccessEventRecordDTO struct {
	ID        int64                  `json:"id" comment:"ID"`
	EventID   string                 `json:"eventId" comment:"事件ID"`
	Attrs     map[string]interface{} `json:"attrs" comment:"自定义参数"`
	CreatedAt time.Time              `json:"createdAt" comment:"创建时间"`
}

// ShortLinkResolveDTO 短链接解析结果DTO（供前端页面渲染使用）
type ShortLinkResolveDTO struct {
	Code         string                 `json:"code" comment:"短码"`
	Domain       string                 `json:"domain" comment:"短域名"`
	TargetType   string                 `json:"targetType" comment:"跳转类型"`
	Url          string                 `json:"url" comment:"主URL"`
	BackupUrl    string                 `json:"backupUrl" comment:"备用URL"`
	TargetConfig map[string]interface{} `json:"targetConfig" comment:"目标配置"`
	HasPassword  bool                   `json:"hasPassword" comment:"是否需要密码"`
	Comment      string                 `json:"comment" comment:"备注说明"`
}
