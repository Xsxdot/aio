package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// DnsCredential DNS 服务商凭证模型
type DnsCredential struct {
	common.Model
	Name        string        `gorm:"size:100;not null" json:"name" comment:"凭证名称"`
	Provider    DnsProvider   `gorm:"size:50;not null;index" json:"provider" comment:"DNS 服务商类型(alidns/tencentcloud/dnspod)"`
	AccessKey   string        `gorm:"type:text;not null" json:"access_key" comment:"访问密钥 AK（加密存储）"`
	SecretKey   string        `gorm:"type:text;not null" json:"secret_key" comment:"访问密钥 SK（加密存储）"`
	ExtraConfig *common.JSON  `gorm:"type:json" json:"extra_config,omitempty" comment:"额外配置（JSON，如 region、endpoint 等）"`
	Status      int           `gorm:"default:1;not null;index" json:"status" comment:"状态：1=启用，0=禁用"`
	Description string        `gorm:"size:500" json:"description" comment:"凭证描述"`
}

// TableName 指定表名
func (DnsCredential) TableName() string {
	return "ssl_dns_credentials"
}
