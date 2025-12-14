package model

import (
	"time"

	"xiaozhizhang/pkg/core/model/common"
)

// DeploymentAction 部署动作
type DeploymentAction string

const (
	// DeploymentActionDeploy 新部署
	DeploymentActionDeploy DeploymentAction = "deploy"
	// DeploymentActionUpdate 更新
	DeploymentActionUpdate DeploymentAction = "update"
	// DeploymentActionRollback 回滚
	DeploymentActionRollback DeploymentAction = "rollback"
)

// DeploymentStatus 部署状态
type DeploymentStatus string

const (
	// DeploymentStatusPending 等待执行
	DeploymentStatusPending DeploymentStatus = "pending"
	// DeploymentStatusRunning 执行中
	DeploymentStatusRunning DeploymentStatus = "running"
	// DeploymentStatusSuccess 成功
	DeploymentStatusSuccess DeploymentStatus = "success"
	// DeploymentStatusFailed 失败
	DeploymentStatusFailed DeploymentStatus = "failed"
	// DeploymentStatusCancelled 已取消
	DeploymentStatusCancelled DeploymentStatus = "cancelled"
)

// Deployment 部署记录
type Deployment struct {
	common.Model
	ApplicationID int64            `gorm:"not null;index" json:"applicationId" comment:"应用 ID"`
	ReleaseID     int64            `gorm:"not null;index" json:"releaseId" comment:"版本 ID"`
	Action        DeploymentAction `gorm:"size:32;not null" json:"action" comment:"动作：deploy/update/rollback"`
	Status        DeploymentStatus `gorm:"size:32;not null;default:'pending'" json:"status" comment:"状态"`
	StartedAt     *time.Time       `json:"startedAt" comment:"开始时间"`
	FinishedAt    *time.Time       `json:"finishedAt" comment:"完成时间"`
	Logs          common.JSON      `gorm:"type:json" json:"logs" comment:"执行日志（JSON 数组）"`
	ErrorMessage  string           `gorm:"type:text" json:"errorMessage" comment:"错误信息"`
	Operator      string           `gorm:"size:128" json:"operator" comment:"操作人"`
}

func (Deployment) TableName() string {
	return "app_deployment"
}

