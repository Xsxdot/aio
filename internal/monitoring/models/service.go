// Package models 定义监控系统使用的数据模型
package models

import "time"

// ServiceStatus 表示服务实例的健康状态
type ServiceStatus string

const (
	// StatusUp 表示服务实例正常运行
	StatusUp ServiceStatus = "up"
	// StatusDown 表示服务实例不可用
	StatusDown ServiceStatus = "down"
	// StatusUnknown 表示服务实例状态未知
	StatusUnknown ServiceStatus = "unknown"
	// StatusWarning 表示服务实例有警告
	StatusWarning ServiceStatus = "warning"
)

// ServiceInstance 表示服务的一个实例
type ServiceInstance struct {
	InstanceID     string                 `json:"instance_id"`     // 实例ID
	Hostname       string                 `json:"hostname"`        // 主机名
	IP             string                 `json:"ip"`              // IP地址
	Port           int                    `json:"port"`            // 端口
	Status         ServiceStatus          `json:"status"`          // 健康状态
	LastActive     time.Time              `json:"last_active"`     // 最后活跃时间
	Version        string                 `json:"version"`         // 应用版本
	StartTime      time.Time              `json:"start_time"`      // 启动时间
	Tags           map[string]string      `json:"tags,omitempty"`  // 标签
	SystemMetrics  map[string]interface{} `json:"system_metrics"`  // 系统资源使用情况
	RuntimeMetrics map[string]interface{} `json:"runtime_metrics"` // 运行时指标
}

// ServiceEndpoint 表示服务提供的一个接口
type ServiceEndpoint struct {
	Path        string                 `json:"path"`           // 接口路径
	Method      string                 `json:"method"`         // 请求方法
	Description string                 `json:"description"`    // 接口描述
	Tags        map[string]string      `json:"tags,omitempty"` // 标签
	Metrics     map[string]interface{} `json:"metrics"`        // 性能指标
}

// ServiceMetricsSummary 表示服务的汇总指标
type ServiceMetricsSummary struct {
	TotalRequests int64   `json:"total_requests"`  // 总请求数
	ErrorCount    int64   `json:"error_count"`     // 错误数
	ErrorRate     float64 `json:"error_rate"`      // 错误率
	AvgResponseMs float64 `json:"avg_response_ms"` // 平均响应时间
	MaxResponseMs float64 `json:"max_response_ms"` // 最大响应时间
	MinResponseMs float64 `json:"min_response_ms"` // 最小响应时间
	P95ResponseMs float64 `json:"p95_response_ms"` // 95%响应时间
	P99ResponseMs float64 `json:"p99_response_ms"` // 99%响应时间
	QPS           float64 `json:"qps"`             // 每秒查询数
}

// Service 表示一个应用服务
type Service struct {
	ServiceName    string                `json:"service_name"`    // 服务名称
	Description    string                `json:"description"`     // 服务描述
	Version        string                `json:"version"`         // 服务版本
	Tags           map[string]string     `json:"tags,omitempty"`  // 标签
	Instances      []ServiceInstance     `json:"instances"`       // 实例列表
	Endpoints      []ServiceEndpoint     `json:"endpoints"`       // 接口列表
	MetricsSummary ServiceMetricsSummary `json:"metrics_summary"` // 汇总指标
	UpdateTime     time.Time             `json:"update_time"`     // 更新时间
}

// ServiceData 表示从应用发送来的自包含监控数据
type ServiceData struct {
	Source     string            `json:"source"`      // 服务名称（必填）
	Instance   string            `json:"instance"`    // 实例ID（必填）
	IP         string            `json:"ip"`          // 实例IP地址（必填）
	Port       int               `json:"port"`        // 实例端口（必填）
	Version    string            `json:"version"`     // 应用版本（可选）
	Tags       map[string]string `json:"tags"`        // 标签（可选）
	Timestamp  time.Time         `json:"timestamp"`   // 时间戳
	AppMetrics interface{}       `json:"app_metrics"` // 应用指标数据
}

// ServiceAPIData 表示从应用发送来的API调用信息
type ServiceAPIData struct {
	Source    string    `json:"source"`    // 服务名称（必填）
	Instance  string    `json:"instance"`  // 实例ID（必填）
	IP        string    `json:"ip"`        // 实例IP地址（可选）
	Port      int       `json:"port"`      // 实例端口（可选）
	Timestamp time.Time `json:"timestamp"` // 时间戳
	APICalls  []APICall `json:"api_calls"` // API调用数据
}

// ServiceListOptions 包含查询服务列表的选项
type ServiceListOptions struct {
	Tag        string `json:"tag"`         // 按标签筛选
	Status     string `json:"status"`      // 按状态筛选
	SearchTerm string `json:"search_term"` // 搜索关键词
	Limit      int    `json:"limit"`       // 限制返回数量
	Offset     int    `json:"offset"`      // 分页偏移量
}

// ServiceQueryOptions 包含查询服务指标的选项
type ServiceQueryOptions struct {
	ServiceName string    `json:"service_name"` // 服务名称
	InstanceID  string    `json:"instance_id"`  // 实例ID（可选）
	Endpoint    string    `json:"endpoint"`     // 接口路径（可选）
	Method      string    `json:"method"`       // 请求方法（可选）
	StartTime   time.Time `json:"start_time"`   // 开始时间
	EndTime     time.Time `json:"end_time"`     // 结束时间
	Aggregation string    `json:"aggregation"`  // 聚合方式
	Interval    string    `json:"interval"`     // 时间间隔
}

// ServiceEndpointMetric 表示服务接口的指标数据
type ServiceEndpointMetric struct {
	Path            string    `json:"path"`
	Method          string    `json:"method"`
	Timestamp       time.Time `json:"timestamp"`
	RequestCount    int64     `json:"request_count"`
	ErrorCount      int64     `json:"error_count"`
	AvgResponseTime float64   `json:"avg_response_time"`
	MaxResponseTime float64   `json:"max_response_time"`
	MinResponseTime float64   `json:"min_response_time"`
	P95ResponseTime float64   `json:"p95_response_time"`
	P99ResponseTime float64   `json:"p99_response_time"`
}
