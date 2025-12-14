package dto

import "time"

// CreateClientCredentialReq 创建客户端凭证请求
type CreateClientCredentialReq struct {
	Name        string     `json:"name" validate:"required,min=3,max=200" comment:"客户端名称"`
	Description string     `json:"description" validate:"max=500" comment:"客户端描述"`
	IPWhitelist []string   `json:"ipWhitelist" comment:"IP白名单"`
	ExpiresAt   *time.Time `json:"expiresAt" comment:"过期时间，null表示永不过期"`
}

// UpdateClientCredentialReq 更新客户端凭证请求
type UpdateClientCredentialReq struct {
	ID          int64      `json:"id" validate:"required" comment:"客户端ID"`
	Name        string     `json:"name" validate:"required,min=3,max=200" comment:"客户端名称"`
	Description string     `json:"description" validate:"max=500" comment:"客户端描述"`
	IPWhitelist []string   `json:"ipWhitelist" comment:"IP白名单"`
	ExpiresAt   *time.Time `json:"expiresAt" comment:"过期时间"`
}

// UpdateClientStatusReq 更新客户端状态请求
type UpdateClientStatusReq struct {
	ID     int64 `json:"id" validate:"required" comment:"客户端ID"`
	Status int8  `json:"status" validate:"required,oneof=0 1" comment:"状态：1=启用，0=禁用"`
}

// RotateClientSecretReq 重新生成客户端 secret 请求
type RotateClientSecretReq struct {
	ID int64 `json:"id" validate:"required" comment:"客户端ID"`
}

// ClientAuthReq 客户端认证请求
type ClientAuthReq struct {
	ClientKey    string `json:"clientKey" validate:"required" comment:"客户端 key"`
	ClientSecret string `json:"clientSecret" validate:"required" comment:"客户端 secret"`
}



