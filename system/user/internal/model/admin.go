package model

import "github.com/xsxdot/aio/pkg/core/model/common"

// Admin 管理员数据库模型
type Admin struct {
	common.Model
	Account      string   `gorm:"uniqueIndex;size:100;not null" json:"account" comment:"管理员登录账号"`
	PasswordHash string   `gorm:"size:255;not null" json:"-" comment:"密码散列"`
	Status       int8     `gorm:"default:1;not null" json:"status" comment:"状态：1=启用，0=禁用"`
	IsSuper      bool     `gorm:"default:false;not null" json:"isSuper" comment:"是否为超级管理员"`
	Roles        []string `gorm:"type:json;serializer:json" json:"roles" comment:"权限码列表"`
	Remark       string   `gorm:"size:500" json:"remark" comment:"备注信息"`
}

// TableName 指定表名
func (Admin) TableName() string {
	return "user_admin"
}

// AdminStatus 管理员状态枚举
const (
	AdminStatusDisabled = 0 // 禁用
	AdminStatusEnabled  = 1 // 启用
)

