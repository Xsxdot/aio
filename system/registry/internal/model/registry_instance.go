package model

import (
	"time"

	"github.com/xsxdot/aio/pkg/core/model/common"
)

// EndpointConfig 单个端点配置（固定结构）
type EndpointConfig struct {
	Host     string `json:"host"`     // 主机地址（不含协议前缀）
	Network  string `json:"network"`  // 网络类型标识：local/internal/external/tailscale
	Priority int    `json:"priority"` // 优先级（数值越小优先级越高）
}

// RegistryInstance 实例登记记录（持久化 + 在线判断兜底）
type RegistryInstance struct {
	common.Model
	ServiceID       int64            `gorm:"not null;index:idx_registry_instance_key,unique" json:"serviceId" comment:"服务ID"`
	InstanceKey     string           `gorm:"size:128;not null;index:idx_registry_instance_key,unique" json:"instanceKey" comment:"实例键"`
	Env             string           `gorm:"size:32;not null;index:idx_registry_instance_env" json:"env" comment:"环境"`
	Host            string           `gorm:"size:128;not null" json:"host" comment:"主机"`
	Endpoint        string           `gorm:"size:255" json:"endpoint" comment:"默认访问地址（向后兼容）"`
	Endpoints       []EndpointConfig `gorm:"type:json;serializer:json" json:"endpoints" comment:"多端点配置"` // 新增
	HTTPPort        int64            `gorm:"default:0" json:"httpPort" comment:"HTTP端口"`                 // 新增
	GRPCPort        int64            `gorm:"default:0" json:"grpcPort" comment:"gRPC端口"`                 // 新增
	Meta            common.JSON      `gorm:"type:json" json:"meta" comment:"元数据(JSON)"`
	TTLSeconds      int64            `gorm:"not null;default:60" json:"ttlSeconds" comment:"租约秒数"`
	LastHeartbeatAt time.Time        `gorm:"not null" json:"lastHeartbeatAt" comment:"最后心跳时间"`
}

func (RegistryInstance) TableName() string {
	return "registry_instance"
}
