package dto

import "time"

// ApplicationDTO 应用对外 DTO
type ApplicationDTO struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Project          string    `json:"project"`
	Env              string    `json:"env"`
	Type             string    `json:"type"`
	Domain           string    `json:"domain"`
	Port             int       `json:"port"`
	SSL              bool      `json:"ssl"`
	InstallPath      string    `json:"installPath"`
	Owner            string    `json:"owner"`
	Description      string    `json:"description"`
	Status           int       `json:"status"`
	CurrentReleaseID int64     `json:"currentReleaseId"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// CreateApplicationReq 创建应用请求
type CreateApplicationReq struct {
	Name        string `json:"name" validate:"required"`
	Project     string `json:"project" validate:"required"`
	Env         string `json:"env" validate:"required"`
	Type        string `json:"type" validate:"required"`
	Domain      string `json:"domain"`
	Port        int    `json:"port"`
	SSL         bool   `json:"ssl"`
	InstallPath string `json:"installPath"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
}

// UpdateApplicationReq 更新应用请求
type UpdateApplicationReq struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Port        int    `json:"port"`
	SSL         *bool  `json:"ssl"`
	InstallPath string `json:"installPath"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	Status      *int   `json:"status"`
}

