package model

import (
	"time"

	"github.com/xsxdot/aio/pkg/core/model/common"
)

// RegistryInstance 实例登记记录（持久化 + 在线判断兜底）
type RegistryInstance struct {
	common.Model
	ServiceID       int64       `gorm:"not null;index:idx_registry_instance_key,unique" json:"serviceId" comment:"服务ID"`
	InstanceKey     string      `gorm:"size:128;not null;index:idx_registry_instance_key,unique" json:"instanceKey" comment:"实例键"`
	Env             string      `gorm:"size:32;not null;index:idx_registry_instance_env" json:"env" comment:"环境"`
	Host            string      `gorm:"size:128;not null" json:"host" comment:"主机"`
	Endpoint        string      `gorm:"size:255;not null" json:"endpoint" comment:"访问地址"`
	Meta            common.JSON `gorm:"type:json" json:"meta" comment:"元数据(JSON)"`
	TTLSeconds      int64       `gorm:"not null;default:60" json:"ttlSeconds" comment:"租约秒数"`
	LastHeartbeatAt time.Time   `gorm:"not null" json:"lastHeartbeatAt" comment:"最后心跳时间"`
}

func (RegistryInstance) TableName() string {
	return "registry_instance"
}
