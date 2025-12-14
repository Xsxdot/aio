package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// ApplicationType 应用类型
type ApplicationType string

const (
	// ApplicationTypeBackend 后端服务
	ApplicationTypeBackend ApplicationType = "backend"
	// ApplicationTypeFrontend 前端静态站点
	ApplicationTypeFrontend ApplicationType = "frontend"
	// ApplicationTypeFullstack 前后端一体
	ApplicationTypeFullstack ApplicationType = "fullstack"
)

// Application 应用定义
type Application struct {
	common.Model
	Name        string          `gorm:"size:64;not null;uniqueIndex:idx_app_key" json:"name" comment:"应用名称"`
	Project     string          `gorm:"size:64;not null;uniqueIndex:idx_app_key" json:"project" comment:"项目"`
	Env         string          `gorm:"size:32;not null;uniqueIndex:idx_app_key" json:"env" comment:"环境"`
	Type        ApplicationType `gorm:"size:32;not null;default:'backend'" json:"type" comment:"应用类型：backend/frontend/fullstack"`
	Domain      string          `gorm:"size:255" json:"domain" comment:"域名"`
	Port        int             `gorm:"default:0" json:"port" comment:"服务端口（backend/fullstack）"`
	SSL         bool            `gorm:"default:false" json:"ssl" comment:"是否启用 HTTPS"`
	InstallPath string          `gorm:"size:500" json:"installPath" comment:"安装路径"`
	Owner       string          `gorm:"size:128" json:"owner" comment:"负责人"`
	Description string          `gorm:"size:500" json:"description" comment:"描述"`
	Status      int             `gorm:"default:1" json:"status" comment:"状态：1-启用 0-禁用"`
	// 关联 ID（可选，用于追溯）
	RegistryServiceID int64 `gorm:"default:0" json:"registryServiceId" comment:"Registry 服务 ID"`
	NginxTargetID     int64 `gorm:"default:0" json:"nginxTargetId" comment:"Nginx Target ID"`
	NginxConfigFileID int64 `gorm:"default:0" json:"nginxConfigFileId" comment:"Nginx ConfigFile ID"`
	CurrentReleaseID  int64 `gorm:"default:0" json:"currentReleaseId" comment:"当前运行的 Release ID"`
}

func (Application) TableName() string {
	return "app_application"
}

