package model

import (
	"time"
	"github.com/xsxdot/aio/pkg/core/model/common"
)

// ClientCredential 客户端凭证数据库模型（独立 key/secret，用于 API 调用鉴权）
type ClientCredential struct {
	common.Model
	Name         string     `gorm:"size:200;not null" json:"name" comment:"客户端名称"`
	ClientKey    string     `gorm:"uniqueIndex;size:100;not null" json:"clientKey" comment:"客户端 key（唯一）"`
	ClientSecret string     `gorm:"size:255;not null" json:"clientSecret" comment:"客户端 secret（加密/散列存储）"`
	Status       int8       `gorm:"default:1;not null" json:"status" comment:"状态：1=启用，0=禁用"`
	Description  string     `gorm:"size:500" json:"description" comment:"客户端描述"`
	IPWhitelist  string     `gorm:"type:json" json:"ipWhitelist" comment:"IP白名单，JSON数组格式"`
	ExpiresAt    *time.Time `gorm:"index" json:"expiresAt" comment:"过期时间，null表示永不过期"`
}

// TableName 指定表名
func (ClientCredential) TableName() string {
	return "user_client_credential"
}

// ClientCredentialStatus 客户端凭证状态枚举
const (
	ClientCredentialStatusDisabled = 0 // 禁用
	ClientCredentialStatusEnabled  = 1 // 启用
)

// IsExpired 检查客户端凭证是否已过期
func (c *ClientCredential) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// IsActive 检查客户端凭证是否处于活跃状态（启用且未过期）
func (c *ClientCredential) IsActive() bool {
	return c.Status == ClientCredentialStatusEnabled && !c.IsExpired()
}



