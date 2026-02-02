package model

import (
	"github.com/xsxdot/aio/pkg/core/model/common"
)

// ServerSSHCredential SSH 连接凭证模型
type ServerSSHCredential struct {
	common.Model
	ServerID   int64  `gorm:"type:bigint;not null;uniqueIndex;comment:服务器ID" json:"serverId" comment:"服务器ID"`
	Port       int    `gorm:"type:int;not null;default:22;comment:SSH端口" json:"port" comment:"SSH端口"`
	Username   string `gorm:"type:varchar(100);not null;comment:SSH用户名" json:"username" comment:"SSH用户名"`
	AuthMethod string `gorm:"type:varchar(20);not null;comment:认证方式(password/privatekey)" json:"authMethod" comment:"认证方式"`
	Password   string `gorm:"type:text;comment:密码(加密存储)" json:"password" comment:"密码"`
	PrivateKey string `gorm:"type:text;comment:SSH私钥(加密存储)" json:"privateKey" comment:"SSH私钥"`
	Comment    string `gorm:"type:varchar(500);comment:备注" json:"comment" comment:"备注"`
}

// TableName 设置表名
func (ServerSSHCredential) TableName() string {
	return "server_ssh_credentials"
}
