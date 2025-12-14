package dto

// CreateConfigRequest 创建配置文件请求
type CreateConfigRequest struct {
	Name    string `json:"name" validate:"required,max=200" comment:"配置文件名称（必须以 .conf 结尾）"`
	Content string `json:"content" validate:"required" comment:"配置文件内容"`
}

// CreateConfigByParamsRequest 按参数创建配置文件请求
type CreateConfigByParamsRequest struct {
	Name string     `json:"name" validate:"required,max=200" comment:"配置文件名称（必须以 .conf 结尾）"`
	Spec ConfigSpec `json:"spec" validate:"required" comment:"配置规格"`
}

// UpdateConfigByParamsRequest 按参数更新配置文件请求
type UpdateConfigByParamsRequest struct {
	Spec ConfigSpec `json:"spec" validate:"required" comment:"配置规格"`
}

// UpdateConfigRequest 更新配置文件请求
type UpdateConfigRequest struct {
	Content string `json:"content" validate:"required" comment:"配置文件内容"`
}

// QueryConfigRequest 查询配置文件请求
type QueryConfigRequest struct {
	Keyword string `query:"keyword" comment:"关键字（模糊查询文件名或描述）"`
	PageNum int    `query:"pageNum" validate:"min=1" comment:"页码"`
	Size    int    `query:"size" validate:"min=1,max=100" comment:"每页数量"`
}

// ConfigInfo 配置文件信息（响应）
type ConfigInfo struct {
	Name        string `json:"name" comment:"配置文件名称"`
	Content     string `json:"content,omitempty" comment:"配置文件内容"`
	Description string `json:"description,omitempty" comment:"描述"`
	ModTime     string `json:"modTime" comment:"修改时间"`
}

// ConfigListItem 配置文件列表项
type ConfigListItem struct {
	Name        string `json:"name" comment:"配置文件名称"`
	Description string `json:"description,omitempty" comment:"描述"`
	ModTime     string `json:"modTime" comment:"修改时间"`
}
