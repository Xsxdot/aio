package dto

import "xiaozhizhang/system/config/internal/model"

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	Key         string                         `json:"key" validate:"required" comment:"配置键"`
	Value       map[string]*model.ConfigValue  `json:"value" validate:"required" comment:"配置值"`
	Metadata    map[string]string              `json:"metadata" comment:"元数据"`
	Description string                         `json:"description" comment:"配置描述"`
	ChangeNote  string                         `json:"changeNote" comment:"变更说明"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Value       map[string]*model.ConfigValue  `json:"value" comment:"配置值"`
	Metadata    map[string]string              `json:"metadata" comment:"元数据"`
	Description string                         `json:"description" comment:"配置描述"`
	ChangeNote  string                         `json:"changeNote" validate:"required" comment:"变更说明"`
}

// QueryConfigRequest 查询配置请求
type QueryConfigRequest struct {
	Key         string `json:"key" comment:"配置键（支持模糊查询）"`
	Environment string `json:"environment" comment:"环境"`
	PageNum     int    `json:"pageNum" comment:"页码"`
	Size        int    `json:"size" comment:"每页数量"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	ID          int64                         `json:"id"`
	Key         string                        `json:"key"`
	Value       map[string]*model.ConfigValue `json:"value"`
	Version     int64                         `json:"version"`
	Metadata    map[string]string             `json:"metadata"`
	Description string                        `json:"description"`
	CreatedAt   string                        `json:"createdAt"`
	UpdatedAt   string                        `json:"updatedAt"`
}

// ConfigHistoryResponse 配置历史响应
type ConfigHistoryResponse struct {
	ID         int64                         `json:"id"`
	ConfigKey  string                        `json:"configKey"`
	Version    int64                         `json:"version"`
	Value      map[string]*model.ConfigValue `json:"value"`
	Metadata   map[string]string             `json:"metadata"`
	Operator   string                        `json:"operator"`
	OperatorID int64                         `json:"operatorId"`
	ChangeNote string                        `json:"changeNote"`
	CreatedAt  string                        `json:"createdAt"`
}
