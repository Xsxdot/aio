package model

import "xiaozhizhang/pkg/core/model/common"

// ConfigItemModel 配置项数据库模型
type ConfigItemModel struct {
	common.Model
	Key         string `gorm:"uniqueIndex;size:255;not null" json:"key" comment:"配置键，格式如 app.cert.dev"`
	Value       string `gorm:"type:json;not null" json:"value" comment:"配置值，map[env]ConfigValue 结构"`
	Version     int64  `gorm:"default:1;not null" json:"version" comment:"当前版本号"`
	Metadata    string `gorm:"type:json" json:"metadata" comment:"元数据"`
	Description string `gorm:"size:500" json:"description" comment:"配置描述"`
}

// TableName 指定表名
func (ConfigItemModel) TableName() string {
	return "config_items"
}
