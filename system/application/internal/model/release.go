package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// ReleaseStatus 版本状态
type ReleaseStatus string

const (
	// ReleaseStatusPending 待部署
	ReleaseStatusPending ReleaseStatus = "pending"
	// ReleaseStatusDeploying 部署中
	ReleaseStatusDeploying ReleaseStatus = "deploying"
	// ReleaseStatusActive 当前运行版本
	ReleaseStatusActive ReleaseStatus = "active"
	// ReleaseStatusSuperseded 已被替换（历史版本）
	ReleaseStatusSuperseded ReleaseStatus = "superseded"
	// ReleaseStatusFailed 部署失败
	ReleaseStatusFailed ReleaseStatus = "failed"
)

// Release 可部署版本
type Release struct {
	common.Model
	ApplicationID     int64         `gorm:"not null;index" json:"applicationId" comment:"应用 ID"`
	Version           string        `gorm:"size:64;not null" json:"version" comment:"版本号"`
	BackendArtifactID int64         `gorm:"default:0" json:"backendArtifactId" comment:"后端产物 ID"`
	FrontendArtifactID int64        `gorm:"default:0" json:"frontendArtifactId" comment:"前端产物 ID"`
	Spec              common.JSON   `gorm:"type:json" json:"spec" comment:"部署规格（JSON）"`
	Status            ReleaseStatus `gorm:"size:32;not null;default:'pending'" json:"status" comment:"状态"`
	ReleasePath       string        `gorm:"size:500" json:"releasePath" comment:"解压后的 Release 目录路径"`
	Operator          string        `gorm:"size:128" json:"operator" comment:"操作人"`
}

func (Release) TableName() string {
	return "app_release"
}

