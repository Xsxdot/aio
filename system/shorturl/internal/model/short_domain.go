package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// ShortDomain 短域名模型
type ShortDomain struct {
	common.Model
	Domain    string `gorm:"type:varchar(255);not null;uniqueIndex;comment:短域名" json:"domain" comment:"短域名"`
	Enabled   bool   `gorm:"type:tinyint(1);not null;default:1;comment:是否启用" json:"enabled" comment:"是否启用"`
	IsDefault bool   `gorm:"type:tinyint(1);not null;default:0;comment:是否默认域名" json:"isDefault" comment:"是否默认域名"`
	Comment   string `gorm:"type:varchar(500);comment:备注" json:"comment" comment:"备注"`
}

// TableName 设置表名
func (ShortDomain) TableName() string {
	return "shorturl_domains"
}
