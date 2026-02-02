package model

import "github.com/xsxdot/aio/pkg/core/model/common"

// ConfigHistoryModel 配置历史版本数据库模型
type ConfigHistoryModel struct {
	common.Model
	ConfigKey  string `gorm:"index;size:255;not null" json:"configKey" comment:"关联的配置键"`
	Version    int64  `gorm:"not null" json:"version" comment:"版本号"`
	Value      string `gorm:"not null" json:"value" comment:"该版本的配置值"`
	Metadata   string `json:"metadata" comment:"该版本的元数据"`
	Operator   string `gorm:"size:100" json:"operator" comment:"操作人账号"`
	OperatorID int64  `json:"operatorId" comment:"操作人ID"`
	ChangeNote string `gorm:"size:500" json:"changeNote" comment:"变更说明"`
}

// TableName 指定表名
func (ConfigHistoryModel) TableName() string {
	return "config_history"
}
