// Package models 定义监控系统使用的数据模型
package models

import "time"

// MetricType 表示指标的类型
type MetricType string

const (
	// MetricTypeGauge 表示仪表盘类型指标，值可上可下
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeCounter 表示计数器类型指标，值只增不减
	MetricTypeCounter MetricType = "counter"
)

// ServerMetricName 定义服务器指标的名称常量
type ServerMetricName string

const (
	// CPU相关指标
	MetricCPUUsage       ServerMetricName = "cpu.usage"
	MetricCPUUsageUser   ServerMetricName = "cpu.usage.user"
	MetricCPUUsageSystem ServerMetricName = "cpu.usage.system"
	MetricCPUUsageIdle   ServerMetricName = "cpu.usage.idle"
	MetricCPUUsageIOWait ServerMetricName = "cpu.usage.iowait"
	MetricCPULoad1       ServerMetricName = "cpu.load1"
	MetricCPULoad5       ServerMetricName = "cpu.load5"
	MetricCPULoad15      ServerMetricName = "cpu.load15"

	// 内存相关指标
	MetricMemoryTotal       ServerMetricName = "memory.total"
	MetricMemoryUsed        ServerMetricName = "memory.used"
	MetricMemoryFree        ServerMetricName = "memory.free"
	MetricMemoryBuffers     ServerMetricName = "memory.buffers"
	MetricMemoryCache       ServerMetricName = "memory.cache"
	MetricMemoryUsedPercent ServerMetricName = "memory.used_percent"

	// 磁盘相关指标
	MetricDiskTotal        ServerMetricName = "disk.total"
	MetricDiskUsed         ServerMetricName = "disk.used"
	MetricDiskFree         ServerMetricName = "disk.free"
	MetricDiskUsedPercent  ServerMetricName = "disk.used_percent"
	MetricDiskIORead       ServerMetricName = "disk.io.read"
	MetricDiskIOWrite      ServerMetricName = "disk.io.write"
	MetricDiskIOReadBytes  ServerMetricName = "disk.io.read_bytes"
	MetricDiskIOWriteBytes ServerMetricName = "disk.io.write_bytes"

	// 网络相关指标
	MetricNetworkIn         ServerMetricName = "network.in"
	MetricNetworkOut        ServerMetricName = "network.out"
	MetricNetworkInPackets  ServerMetricName = "network.in_packets"
	MetricNetworkOutPackets ServerMetricName = "network.out_packets"
)

// ApplicationMetricName 定义应用指标的名称常量
type ApplicationMetricName string

const (
	// API相关指标
	MetricAPIRequestCount    ApplicationMetricName = "api.request.count"
	MetricAPIRequestDuration ApplicationMetricName = "api.request.duration"
	MetricAPIRequestError    ApplicationMetricName = "api.request.error"
	MetricAPIRequestSize     ApplicationMetricName = "api.request.size"
	MetricAPIResponseSize    ApplicationMetricName = "api.response.size"

	// 应用状态相关指标
	MetricAppMemoryUsed    ApplicationMetricName = "app.memory.used"
	MetricAppMemoryTotal   ApplicationMetricName = "app.memory.total"
	MetricAppMemoryHeap    ApplicationMetricName = "app.memory.heap"
	MetricAppMemoryNonHeap ApplicationMetricName = "app.memory.non_heap"
	MetricAppGCCount       ApplicationMetricName = "app.gc.count"
	MetricAppGCTime        ApplicationMetricName = "app.gc.time"
	MetricAppThreadTotal   ApplicationMetricName = "app.thread.total"
	MetricAppThreadActive  ApplicationMetricName = "app.thread.active"
	MetricAppThreadBlocked ApplicationMetricName = "app.thread.blocked"
	MetricAppThreadWaiting ApplicationMetricName = "app.thread.waiting"
	MetricAppCPUUsage      ApplicationMetricName = "app.cpu.usage"
	MetricAppClassLoaded   ApplicationMetricName = "app.class.loaded"
)

// MetricDataPoint 表示一个指标数据点
type MetricDataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ServerMetrics 表示服务器指标集合
type ServerMetrics struct {
	Timestamp time.Time                    `json:"timestamp"`
	Hostname  string                       `json:"hostname"`
	Metrics   map[ServerMetricName]float64 `json:"metrics"`
}

// APICall 表示单个API调用信息
type APICall struct {
	Endpoint     string            `json:"endpoint"`
	Method       string            `json:"method"`
	Timestamp    time.Time         `json:"timestamp"` // 调用时间戳
	DurationMs   float64           `json:"duration_ms"`
	StatusCode   int               `json:"status_code"`
	HasError     bool              `json:"has_error"`
	ErrorMessage string            `json:"error_message,omitempty"`
	RequestSize  int64             `json:"request_size_bytes"`
	ResponseSize int64             `json:"response_size_bytes"`
	ClientIP     string            `json:"client_ip"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// APICalls 表示一组API调用信息
type APICalls struct {
	Source    string    `json:"source"`
	Instance  string    `json:"instance"`
	Timestamp time.Time `json:"timestamp"`
	Calls     []APICall `json:"api_calls"`
}

// AppMetrics 表示应用状态指标
type AppMetrics struct {
	Source    string    `json:"source"`
	Instance  string    `json:"instance"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   struct {
		Memory struct {
			TotalMB   float64 `json:"total_mb"`
			UsedMB    float64 `json:"used_mb"`
			HeapMB    float64 `json:"heap_mb"`
			NonHeapMB float64 `json:"non_heap_mb"`
			GCCount   int     `json:"gc_count"`
			GCTimeMs  int     `json:"gc_time_ms"`
		} `json:"memory"`
		Threads struct {
			Total   int `json:"total"`
			Active  int `json:"active"`
			Blocked int `json:"blocked"`
			Waiting int `json:"waiting"`
		} `json:"threads"`
		CPUUsagePercent float64 `json:"cpu_usage_percent"`
		ClassLoaded     int     `json:"class_loaded"`
		ConnectionPools map[string]struct {
			Active int `json:"active"`
			Idle   int `json:"idle"`
			Max    int `json:"max"`
		} `json:"connection_pools,omitempty"`
	} `json:"app_metrics"`
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
