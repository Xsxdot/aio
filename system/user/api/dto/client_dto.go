package dto

import "time"

// ClientCredentialDTO 客户端凭证对外 DTO（不包含 secret）
type ClientCredentialDTO struct {
	ID          int64      `json:"id" comment:"客户端ID"`
	Name        string     `json:"name" comment:"客户端名称"`
	ClientKey   string     `json:"clientKey" comment:"客户端 key"`
	Status      int8       `json:"status" comment:"状态：1=启用，0=禁用"`
	Description string     `json:"description" comment:"客户端描述"`
	IPWhitelist []string   `json:"ipWhitelist" comment:"IP白名单"`
	ExpiresAt   *time.Time `json:"expiresAt" comment:"过期时间"`
	CreatedAt   time.Time  `json:"createdAt" comment:"创建时间"`
	UpdatedAt   time.Time  `json:"updatedAt" comment:"更新时间"`
}

// ClientCredentialWithSecretDTO 客户端凭证 DTO（包含 secret，仅创建时返回）
type ClientCredentialWithSecretDTO struct {
	ClientCredentialDTO
	ClientSecret string `json:"clientSecret" comment:"客户端 secret（仅创建时返回，请妥善保管）"`
}

// ClientCredentialListDTO 客户端凭证列表 DTO
type ClientCredentialListDTO struct {
	Total   int64                  `json:"total" comment:"总数"`
	Content []*ClientCredentialDTO `json:"content" comment:"客户端列表"`
}

// ClientAuthResponseDTO 客户端认证响应 DTO
type ClientAuthResponseDTO struct {
	AccessToken string `json:"accessToken" comment:"访问令牌"`
	ExpiresAt   int64  `json:"expiresAt" comment:"过期时间戳（秒）"`
	TokenType   string `json:"tokenType" comment:"令牌类型"`
}

// ClientRenewTokenResponseDTO 客户端续期响应 DTO
type ClientRenewTokenResponseDTO struct {
	AccessToken string `json:"accessToken" comment:"新的访问令牌"`
	ExpiresAt   int64  `json:"expiresAt" comment:"过期时间戳（秒）"`
	TokenType   string `json:"tokenType" comment:"令牌类型"`
}



