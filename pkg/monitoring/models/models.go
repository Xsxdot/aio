// Package models 定义监控系统使用的数据模型
package models

import (
	"time"
)

// MetricPoint 表示一个统一的指标数据点
type MetricPoint struct {
	Timestamp   time.Time         `json:"timestamp"`             // 时间戳
	MetricName  string            `json:"metric_name"`           // 指标名称
	MetricType  MetricType        `json:"metric_type"`           // 指标类型
	Value       float64           `json:"value"`                 // 指标值
	Source      string            `json:"source"`                // 数据来源(hostname, service_name等)
	Instance    string            `json:"instance,omitempty"`    // 实例标识
	Category    MetricCategory    `json:"category"`              // 指标分类
	Labels      map[string]string `json:"labels,omitempty"`      // 额外标签
	Unit        string            `json:"unit,omitempty"`        // 单位
	Description string            `json:"description,omitempty"` // 描述
}

// MetricCategory 表示指标分类
type MetricCategory string

const (
	// CategoryServer 服务器指标
	CategoryServer MetricCategory = "server"
	// CategoryApp 应用指标
	CategoryApp MetricCategory = "app"
	// CategoryAPI API指标
	CategoryAPI MetricCategory = "api"
	// CategoryCustom 自定义指标
	CategoryCustom MetricCategory = "custom"
)

// MetricProvider 定义指标提供者接口，用于获取指标名称列表
type MetricProvider interface {
	// GetMetricNames 返回该类型支持的所有指标名称
	GetMetricNames() []string
	// GetCategory 返回指标分类
	GetCategory() MetricCategory
	// ToMetricPoints 将数据转换为统一的MetricPoint切片
	ToMetricPoints() []MetricPoint
}

// MetricType 表示指标的类型
type MetricType string

const (
	// MetricTypeGauge 表示仪表盘类型指标，值可上可下
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeCounter 表示计数器类型指标，值只增不减
	MetricTypeCounter MetricType = "counter"
)

// MetricDataPoint 表示一个指标数据点
type MetricDataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// QueryOptions 包含查询指标数据的选项
type QueryOptions struct {
	StartTime     time.Time
	EndTime       time.Time
	MetricNames   []string
	LabelMatchers map[string]string
	Aggregation   string
	Interval      string
	Limit         int
}

// TimeRange 表示时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TimeSeriesPoint 表示时间序列中的一个点
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// TimeSeries 表示一个指标的时间序列数据
type TimeSeries struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Points []TimeSeriesPoint `json:"points"`
}

// QueryResult 表示查询结果
type QueryResult struct {
	Series []TimeSeries `json:"series"`
}

// CPUInfo 表示CPU信息
type CPUInfo struct {
	Usage       float64 `json:"usage"`       // CPU使用率（百分比）
	Temperature float64 `json:"temperature"` // CPU温度（摄氏度）
	Cores       int     `json:"cores"`       // CPU核心数
}

// MemoryInfo 表示内存信息
type MemoryInfo struct {
	Total     int64   `json:"total"`      // 总内存（字节）
	Used      int64   `json:"used"`       // 已用内存（字节）
	Free      int64   `json:"free"`       // 空闲内存（字节）
	UsageRate float64 `json:"usage_rate"` // 内存使用率（百分比）
}

// DiskInfo 表示磁盘信息
type DiskInfo struct {
	Total     int64   `json:"total"`      // 总磁盘空间（字节）
	Used      int64   `json:"used"`       // 已用磁盘空间（字节）
	Free      int64   `json:"free"`       // 可用磁盘空间（字节）
	UsageRate float64 `json:"usage_rate"` // 磁盘使用率（百分比）
}

// NetworkInfo 表示网络信息
type NetworkInfo struct {
	UploadSpeed   float64 `json:"upload_speed"`   // 上传速度（字节/秒）
	DownloadSpeed float64 `json:"download_speed"` // 下载速度（字节/秒）
	TotalSent     int64   `json:"total_sent"`     // 总发送数据（字节）
	TotalReceived int64   `json:"total_received"` // 总接收数据（字节）
}

// ProcessInfo 表示进程信息
type ProcessInfo struct {
	Total  int `json:"total"`  // 总进程数
	Active int `json:"active"` // 活跃进程数
}

// SystemOverview 表示系统概览信息
type SystemOverview struct {
	Timestamp time.Time   `json:"timestamp"` // 时间戳
	CPU       CPUInfo     `json:"cpu"`       // CPU信息
	Memory    MemoryInfo  `json:"memory"`    // 内存信息
	Disk      DiskInfo    `json:"disk"`      // 磁盘信息
	Network   NetworkInfo `json:"network"`   // 网络信息
	Process   ProcessInfo `json:"process"`   // 进程信息
}
