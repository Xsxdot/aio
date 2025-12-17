package model

import (
	"time"
	"xiaozhizhang/pkg/core/model/common"
)

// ShortLink 短链接模型
type ShortLink struct {
	common.Model
	DomainID     int64       `gorm:"type:bigint;not null;index:idx_domain_code;comment:短域名ID" json:"domainId" comment:"短域名ID"`
	Code         string      `gorm:"type:varchar(100);not null;index:idx_domain_code;comment:短码" json:"code" comment:"短码"`
	TargetType   TargetType  `gorm:"type:varchar(50);not null;comment:跳转类型" json:"targetType" comment:"跳转类型"`
	URL          string      `gorm:"type:varchar(2048);comment:主URL" json:"url" comment:"主URL"`
	BackupURL    string      `gorm:"type:varchar(2048);comment:备用URL" json:"backupUrl" comment:"备用URL"`
	TargetConfig common.JSON `gorm:"serializer:json;comment:目标配置JSON" json:"targetConfig" comment:"目标配置JSON"`
	ExpiresAt    *time.Time  `gorm:"comment:过期时间" json:"expiresAt" comment:"过期时间"`
	PasswordHash string      `gorm:"type:varchar(255);comment:访问密码哈希" json:"-" comment:"访问密码哈希"`
	MaxVisits    *int64      `gorm:"comment:最大访问次数（NULL表示无限制）" json:"maxVisits" comment:"最大访问次数"`
	VisitCount   int64       `gorm:"type:bigint;not null;default:0;comment:访问次数" json:"visitCount" comment:"访问次数"`
	SuccessCount int64       `gorm:"type:bigint;not null;default:0;comment:成功上报次数" json:"successCount" comment:"成功上报次数"`
	Enabled      bool        `gorm:"type:tinyint(1);not null;default:1;comment:是否启用" json:"enabled" comment:"是否启用"`
	Comment      string      `gorm:"type:varchar(500);comment:备注" json:"comment" comment:"备注"`
}

// TableName 设置表名
func (ShortLink) TableName() string {
	return "shorturl_links"
}

// IsExpired 是否已过期
func (s *ShortLink) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// IsVisitLimitReached 是否达到访问次数上限
func (s *ShortLink) IsVisitLimitReached() bool {
	if s.MaxVisits == nil {
		return false
	}
	return s.VisitCount >= *s.MaxVisits
}

// HasPassword 是否设置了访问密码
func (s *ShortLink) HasPassword() bool {
	return s.PasswordHash != ""
}

// SyncURLsFromTargetConfig 从 TargetConfig 同步 URL/BackupURL 字段
// 用于创建/更新时自动填充 URL/BackupURL，兼容现有以 targetConfig 传参的请求
func (s *ShortLink) SyncURLsFromTargetConfig() {
	if s.TargetConfig == nil {
		return
	}

	switch s.TargetType {
	case TargetTypeURL:
		// URL 类型：读取 targetConfig.url / targetConfig.backupUrl
		if url, ok := s.TargetConfig["url"].(string); ok {
			s.URL = url
		}
		if backupURL, ok := s.TargetConfig["backupUrl"].(string); ok {
			s.BackupURL = backupURL
		}

	case TargetTypeURLScheme:
		// URL_SCHEME 类型：优先读取 targetConfig.url / targetConfig.backupUrl
		// 如果不存在则读取 targetConfig.schemeUrl / targetConfig.fallbackUrl
		if url, ok := s.TargetConfig["url"].(string); ok {
			s.URL = url
		} else if schemeURL, ok := s.TargetConfig["schemeUrl"].(string); ok {
			s.URL = schemeURL
		}

		if backupURL, ok := s.TargetConfig["backupUrl"].(string); ok {
			s.BackupURL = backupURL
		} else if fallbackURL, ok := s.TargetConfig["fallbackUrl"].(string); ok {
			s.BackupURL = fallbackURL
		}
	}
}
