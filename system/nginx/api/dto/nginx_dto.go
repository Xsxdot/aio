package dto

import internaldto "xiaozhizhang/system/nginx/internal/model/dto"

// CreateConfigReq 创建配置文件请求
type CreateConfigReq struct {
	Name    string `json:"name" validate:"required,max=200" comment:"配置文件名称（必须以 .conf 结尾）"`
	Content string `json:"content" validate:"required" comment:"配置文件内容"`
}

// CreateConfigByParamsReq 按参数创建配置文件请求
type CreateConfigByParamsReq struct {
	Name string                 `json:"name" validate:"required,max=200" comment:"配置文件名称（必须以 .conf 结尾）"`
	Spec internaldto.ConfigSpec `json:"spec" validate:"required" comment:"配置规格"`
}

// UpdateConfigByParamsReq 按参数更新配置文件请求
type UpdateConfigByParamsReq struct {
	Spec internaldto.ConfigSpec `json:"spec" validate:"required" comment:"配置规格"`
}

// UpdateConfigReq 更新配置文件请求
type UpdateConfigReq struct {
	Content string `json:"content" validate:"required" comment:"配置文件内容"`
}

// QueryConfigReq 查询配置文件请求
type QueryConfigReq struct {
	Keyword string `query:"keyword" comment:"关键字（模糊查询文件名或描述）"`
	PageNum int    `query:"pageNum" validate:"min=1" comment:"页码"`
	Size    int    `query:"size" validate:"min=1,max=100" comment:"每页数量"`
}

// ConfigDTO 配置文件信息
type ConfigDTO struct {
	Name        string `json:"name" comment:"配置文件名称"`
	Content     string `json:"content,omitempty" comment:"配置文件内容"`
	Description string `json:"description,omitempty" comment:"描述"`
	ModTime     string `json:"modTime" comment:"修改时间"`
}

// ConfigListItemDTO 配置文件列表项
type ConfigListItemDTO struct {
	Name        string `json:"name" comment:"配置文件名称"`
	Description string `json:"description,omitempty" comment:"描述"`
	ModTime     string `json:"modTime" comment:"修改时间"`
}
