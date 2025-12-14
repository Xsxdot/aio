package dto

// GetConfigRequest 获取配置请求
type GetConfigRequest struct {
	Key string `json:"key" validate:"required" comment:"配置键"`
	Env string `json:"env" validate:"required" comment:"环境"`
}

// BatchGetConfigRequest 批量获取配置请求
type BatchGetConfigRequest struct {
	Keys []string `json:"keys" validate:"required" comment:"配置键列表"`
	Env  string   `json:"env" validate:"required" comment:"环境"`
}
