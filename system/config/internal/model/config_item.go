package model

import "github.com/xsxdot/aio/pkg/core/model/common"

// ConfigItemModel 配置项数据库模型
type ConfigItemModel struct {
	common.Model
	Key         string `gorm:"column:key;uniqueIndex;size:255;not null" json:"key" comment:"配置键（带环境后缀），格式如 app.cert.dev"`
	Value       string `gorm:"not null" json:"value" comment:"配置值，map[属性名]ConfigValue 结构"`
	Version     int64  `gorm:"default:1;not null" json:"version" comment:"当前版本号"`
	Metadata    string `json:"metadata" comment:"元数据"`
	Description string `gorm:"size:500" json:"description" comment:"配置描述"`
}

// TableName 指定表名
func (ConfigItemModel) TableName() string {
	return "config_items"
}
