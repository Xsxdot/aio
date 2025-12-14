package dto

import "time"

// CreateServerRequest 创建服务器请求
type CreateServerRequest struct {
	Name             string            `json:"name" validate:"required,max=100"`
	Host             string            `json:"host" validate:"required,max=255"`
	AgentGrpcAddress string            `json:"agentGrpcAddress" validate:"max=255"`
	Enabled          bool              `json:"enabled"`
	Tags             map[string]string `json:"tags"`
	Comment          string            `json:"comment" validate:"max=500"`
}

// UpdateServerRequest 更新服务器请求
type UpdateServerRequest struct {
	Name             *string            `json:"name" validate:"omitempty,max=100"`
	Host             *string            `json:"host" validate:"omitempty,max=255"`
	AgentGrpcAddress *string            `json:"agentGrpcAddress" validate:"omitempty,max=255"`
	Enabled          *bool              `json:"enabled"`
	Tags             map[string]string  `json:"tags"`
	Comment          *string            `json:"comment" validate:"omitempty,max=500"`
}

// QueryServerRequest 查询服务器请求
type QueryServerRequest struct {
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	Enabled *bool  `json:"enabled"`
	PageNum int    `json:"pageNum"`
	Size    int    `json:"size"`
}

// ReportServerStatusRequest 上报服务器状态请求
type ReportServerStatusRequest struct {
	ServerID     int64                `json:"serverId" validate:"required"`
	CPUPercent   float64              `json:"cpuPercent"`
	MemUsed      int64                `json:"memUsed"`
	MemTotal     int64                `json:"memTotal"`
	Load1        float64              `json:"load1"`
	Load5        float64              `json:"load5"`
	Load15       float64              `json:"load15"`
	DiskItems    []DiskItemDTO        `json:"diskItems"`
	CollectedAt  time.Time            `json:"collectedAt"`
	ErrorMessage string               `json:"errorMessage"`
}

// DiskItemDTO 磁盘项 DTO
type DiskItemDTO struct {
	MountPoint string  `json:"mountPoint"`
	Used       int64   `json:"used"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
}

// ServerStatusInfo 服务器状态信息
type ServerStatusInfo struct {
	// 服务器基本信息
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	Host             string            `json:"host"`
	AgentGrpcAddress string            `json:"agentGrpcAddress"`
	Enabled          bool              `json:"enabled"`
	Tags             map[string]string `json:"tags"`
	Comment          string            `json:"comment"`
	
	// 状态信息
	CPUPercent    *float64       `json:"cpuPercent,omitempty"`
	MemUsed       *int64         `json:"memUsed,omitempty"`
	MemTotal      *int64         `json:"memTotal,omitempty"`
	Load1         *float64       `json:"load1,omitempty"`
	Load5         *float64       `json:"load5,omitempty"`
	Load15        *float64       `json:"load15,omitempty"`
	DiskItems     []DiskItemDTO  `json:"diskItems,omitempty"`
	CollectedAt   *time.Time     `json:"collectedAt,omitempty"`
	ReportedAt    *time.Time     `json:"reportedAt,omitempty"`
	ErrorMessage  string         `json:"errorMessage,omitempty"`
	StatusSummary string         `json:"statusSummary"` // online/offline/unknown
}


