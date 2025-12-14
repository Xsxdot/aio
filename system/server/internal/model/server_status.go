package model

import (
	"time"
	"xiaozhizhang/pkg/core/model/common"
)

// ServerStatusModel 服务器状态模型
type ServerStatusModel struct {
	common.Model
	ServerID     int64       `gorm:"type:bigint;not null;uniqueIndex;comment:服务器 ID" json:"serverId" comment:"服务器 ID"`
	CPUPercent   float64     `gorm:"type:decimal(5,2);comment:CPU 使用率（%）" json:"cpuPercent" comment:"CPU 使用率"`
	MemUsed      int64       `gorm:"type:bigint;comment:内存已使用（字节）" json:"memUsed" comment:"内存已使用"`
	MemTotal     int64       `gorm:"type:bigint;comment:内存总量（字节）" json:"memTotal" comment:"内存总量"`
	Load1        float64      `gorm:"type:decimal(10,2);comment:1 分钟负载" json:"load1" comment:"1 分钟负载"`
	Load5        float64      `gorm:"type:decimal(10,2);comment:5 分钟负载" json:"load5" comment:"5 分钟负载"`
	Load15       float64      `gorm:"type:decimal(10,2);comment:15 分钟负载" json:"load15" comment:"15 分钟负载"`
	DiskItems    []DiskItem   `gorm:"serializer:json;comment:磁盘信息（JSON）" json:"diskItems" comment:"磁盘信息"`
	CollectedAt  time.Time    `gorm:"type:datetime;comment:采集时间" json:"collectedAt" comment:"采集时间"`
	ReportedAt   time.Time   `gorm:"type:datetime;comment:上报时间" json:"reportedAt" comment:"上报时间"`
	ErrorMessage string      `gorm:"type:varchar(1000);comment:错误信息" json:"errorMessage" comment:"错误信息"`
}

// TableName 设置表名
func (ServerStatusModel) TableName() string {
	return "server_status"
}

// DiskItem 磁盘项
type DiskItem struct {
	MountPoint string  `json:"mountPoint"`
	Used       int64   `json:"used"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
}

