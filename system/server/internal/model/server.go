package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// ServerModel 服务器模型
type ServerModel struct {
	common.Model
	Name             string      `gorm:"type:varchar(100);not null;uniqueIndex;comment:服务器名称" json:"name" comment:"服务器名称"`
	Host             string      `gorm:"type:varchar(255);not null;comment:服务器地址" json:"host" comment:"服务器地址"`
	AgentGrpcAddress string      `gorm:"type:varchar(255);comment:Agent gRPC 地址（预留）" json:"agentGrpcAddress" comment:"Agent gRPC 地址"`
	Enabled          bool        `gorm:"type:tinyint(1);not null;default:1;comment:是否启用" json:"enabled" comment:"是否启用"`
	Tags             common.JSON `gorm:"serializer:json;comment:标签（JSON）" json:"tags" comment:"标签"`
	Comment          string      `gorm:"type:varchar(500);comment:备注" json:"comment" comment:"备注"`
}

// TableName 设置表名
func (ServerModel) TableName() string {
	return "server_servers"
}

