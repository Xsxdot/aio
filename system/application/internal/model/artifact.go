package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// StorageMode 存储模式
type StorageMode string

const (
	// StorageModeLocal 本地存储
	StorageModeLocal StorageMode = "local"
	// StorageModeOSS OSS 存储
	StorageModeOSS StorageMode = "oss"
)

// ArtifactType 产物类型
type ArtifactType string

const (
	// ArtifactTypeBackend 后端产物（如二进制、jar）
	ArtifactTypeBackend ArtifactType = "backend"
	// ArtifactTypeFrontend 前端产物（如 dist 目录打包）
	ArtifactTypeFrontend ArtifactType = "frontend"
)

// Artifact 构建产物元信息
type Artifact struct {
	common.Model
	ApplicationID int64        `gorm:"not null;index" json:"applicationId" comment:"应用 ID"`
	Type          ArtifactType `gorm:"size:32;not null" json:"type" comment:"产物类型：backend/frontend"`
	StorageMode   StorageMode  `gorm:"size:32;not null" json:"storageMode" comment:"存储模式：local/oss"`
	ObjectKey     string       `gorm:"size:500;not null" json:"objectKey" comment:"存储对象 Key 或本地路径"`
	FileName      string       `gorm:"size:255" json:"fileName" comment:"原始文件名"`
	Size          int64        `gorm:"default:0" json:"size" comment:"文件大小（字节）"`
	SHA256        string       `gorm:"size:64" json:"sha256" comment:"SHA256 校验和"`
	ContentType   string       `gorm:"size:128" json:"contentType" comment:"MIME 类型"`
}

func (Artifact) TableName() string {
	return "app_artifact"
}

