package dto

import "xiaozhizhang/system/application/internal/model"

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Name        string                `json:"name" validate:"required"`
	Project     string                `json:"project" validate:"required"`
	Env         string                `json:"env" validate:"required"`
	Type        model.ApplicationType `json:"type" validate:"required"`
	Domain      string                `json:"domain"`
	Port        int                   `json:"port"`
	SSL         bool                  `json:"ssl"`
	InstallPath string                `json:"installPath"`
	Owner       string                `json:"owner"`
	Description string                `json:"description"`
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Port        int    `json:"port"`
	SSL         *bool  `json:"ssl"`
	InstallPath string `json:"installPath"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	Status      *int   `json:"status"`
}

// QueryApplicationRequest 查询应用请求
type QueryApplicationRequest struct {
	Project string `json:"project"`
	Env     string `json:"env"`
	Type    string `json:"type"`
	Keyword string `json:"keyword"`
	PageNum int    `json:"pageNum"`
	Size    int    `json:"size"`
}

