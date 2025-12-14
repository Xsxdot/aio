package dto

import "time"

// ===== Service DTO =====

type ServiceDTO struct {
	ID          int64                  `json:"id"`
	Project     string                 `json:"project"`
	Name        string                 `json:"name"`
	Owner       string                 `json:"owner"`
	Description string                 `json:"description"`
	Spec        map[string]interface{} `json:"spec"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

type InstanceDTO struct {
	ID              int64                  `json:"id"`
	ServiceID       int64                  `json:"serviceId"`
	InstanceKey     string                 `json:"instanceKey"`
	Env             string                 `json:"env"`
	Host            string                 `json:"host"`
	Endpoint        string                 `json:"endpoint"`
	Meta            map[string]interface{} `json:"meta"`
	TTLSeconds      int64                  `json:"ttlSeconds"`
	LastHeartbeatAt time.Time              `json:"lastHeartbeatAt"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

type ServiceWithInstancesDTO struct {
	Service   *ServiceDTO    `json:"service"`
	Instances []*InstanceDTO `json:"instances"`
}

// ===== Service CRUD =====

// CreateServiceReq 创建服务定义请求
type CreateServiceReq struct {
	Project     string                 `json:"project" validate:"required"`
	Name        string                 `json:"name" validate:"required"`
	Owner       string                 `json:"owner"`
	Description string                 `json:"description"`
	Spec        map[string]interface{} `json:"spec"`
}

// UpdateServiceReq 更新服务定义请求
type UpdateServiceReq struct {
	Project     string                 `json:"project"`
	Name        string                 `json:"name"`
	Owner       string                 `json:"owner"`
	Description string                 `json:"description"`
	Spec        map[string]interface{} `json:"spec"`
}

// ===== Instance register/heartbeat =====

type RegisterInstanceReq struct {
	ServiceID   int64                  `json:"serviceId" validate:"required"`
	InstanceKey string                 `json:"instanceKey"` // 可为空：服务端生成
	Env         string                 `json:"env" validate:"required"`
	Host        string                 `json:"host" validate:"required"`
	Endpoint    string                 `json:"endpoint" validate:"required"`
	Meta        map[string]interface{} `json:"meta"`
	TTLSeconds  int64                  `json:"ttlSeconds"` // 默认 60
}

type RegisterInstanceResp struct {
	InstanceKey string    `json:"instanceKey"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type HeartbeatReq struct {
	ServiceID   int64  `json:"serviceId" validate:"required"`
	InstanceKey string `json:"instanceKey" validate:"required"`
}

type HeartbeatResp struct {
	ExpiresAt time.Time `json:"expiresAt"`
}

type DeregisterInstanceReq struct {
	ServiceID   int64  `json:"serviceId" validate:"required"`
	InstanceKey string `json:"instanceKey" validate:"required"`
}
