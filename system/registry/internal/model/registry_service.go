package model

import "github.com/xsxdot/aio/pkg/core/model/common"

// RegistryService 服务定义（跨环境唯一，持久化真相源）
type RegistryService struct {
	common.Model
	Project     string      `gorm:"size:64;not null;uniqueIndex:idx_registry_service_key" json:"project" comment:"项目"`
	Name        string      `gorm:"size:64;not null;uniqueIndex:idx_registry_service_key" json:"name" comment:"服务名"`
	Owner       string      `gorm:"size:128" json:"owner" comment:"负责人"`
	Description string      `gorm:"size:500" json:"description" comment:"描述"`
	Spec        common.JSON `gorm:"type:json" json:"spec" comment:"期望配置(JSON)"`
}

func (RegistryService) TableName() string {
	return "registry_service"
}
