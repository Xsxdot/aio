package model

import (
	"time"
	"xiaozhizhang/pkg/core/model/common"
)

// DeployHistory 部署历史记录模型
type DeployHistory struct {
	common.Model
	CertificateID  uint         `gorm:"not null;index" json:"certificate_id" comment:"证书 ID"`
	DeployTargetID uint         `gorm:"not null;index" json:"deploy_target_id" comment:"部署目标 ID"`
	Status         DeployStatus `gorm:"size:50;not null;index" json:"status" comment:"部署状态"`
	StartTime      time.Time    `gorm:"not null" json:"start_time" comment:"部署开始时间"`
	EndTime        *time.Time   `json:"end_time" comment:"部署结束时间"`
	ErrorMessage   string       `gorm:"type:text" json:"error_message" comment:"错误信息"`
	ResultData     *string      `gorm:"type:json" json:"result_data" comment:"部署结果数据（JSON，如 CAS CertId）"`
	TriggerType    string       `gorm:"size:50;not null" json:"trigger_type" comment:"触发方式：manual/auto_renew/auto_issue"`
}

// TableName 指定表名
func (DeployHistory) TableName() string {
	return "ssl_deploy_histories"
}
