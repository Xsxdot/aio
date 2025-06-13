package models

import "time"

// APIMetricsAggregationType 表示API指标聚合类型
type APIMetricsAggregationType string

const (
	// AggregationAvg 计算平均值
	AggregationAvg APIMetricsAggregationType = "avg"
	// AggregationMax 计算最大值
	AggregationMax APIMetricsAggregationType = "max"
	// AggregationMin 计算最小值
	AggregationMin APIMetricsAggregationType = "min"
	// AggregationP95 计算95%分位数
	AggregationP95 APIMetricsAggregationType = "p95"
	// AggregationP99 计算99%分位数
	AggregationP99 APIMetricsAggregationType = "p99"
	// AggregationSum 计算总和
	AggregationSum APIMetricsAggregationType = "sum"
	// AggregationCount 计算次数
	AggregationCount APIMetricsAggregationType = "count"
)

// APIMetricsQueryOptions 定义API指标查询选项
type APIMetricsQueryOptions struct {
	// 基本查询选项
	StartTime time.Time
	EndTime   time.Time
	Limit     int

	// 过滤选项
	Source    string            // 应用来源
	Instance  string            // 实例标识
	Endpoint  string            // API端点
	Method    string            // HTTP方法
	TagFilter map[string]string // 自定义标签过滤

	// 聚合选项
	Aggregation APIMetricsAggregationType // 聚合类型
	Interval    time.Duration             // 时间间隔分组
	GroupBy     []string                  // 分组字段
}

// APIResponseTimeResult 表示接口响应时间结果
type APIResponseTimeResult struct {
	Period     TimeRange         `json:"period"`      // 时间范围
	GroupBy    map[string]string `json:"group_by"`    // 分组信息
	Avg        float64           `json:"avg"`         // 平均响应时间(毫秒)
	Max        float64           `json:"max"`         // 最大响应时间(毫秒)
	Min        float64           `json:"min"`         // 最小响应时间(毫秒)
	P95        float64           `json:"p95"`         // 95%分位数响应时间(毫秒)
	P99        float64           `json:"p99"`         // 99%分位数响应时间(毫秒)
	SampleSize int               `json:"sample_size"` // 样本数量
}

// APIQPSResult 表示QPS结果
type APIQPSResult struct {
	Period     TimeRange         `json:"period"`      // 时间范围
	GroupBy    map[string]string `json:"group_by"`    // 分组信息
	QPS        float64           `json:"qps"`         // 每秒查询数
	SampleSize int               `json:"sample_size"` // 样本数量
}

// APIErrorRateResult 表示错误率结果
type APIErrorRateResult struct {
	Period     TimeRange         `json:"period"`      // 时间范围
	GroupBy    map[string]string `json:"group_by"`    // 分组信息
	ErrorRate  float64           `json:"error_rate"`  // 错误率(0-1)
	ErrorCount int               `json:"error_count"` // 错误次数
	TotalCount int               `json:"total_count"` // 总请求次数
}

// APICallDistributionItem 表示API调用分布的单个项
type APICallDistributionItem struct {
	Endpoint string            `json:"endpoint"` // API端点
	Method   string            `json:"method"`   // HTTP方法
	Tags     map[string]string `json:"tags"`     // 标签
	Count    int               `json:"count"`    // 调用次数
	Percent  float64           `json:"percent"`  // 占比百分比(0-100)
}

// APICallDistributionResult 表示API调用分布结果
type APICallDistributionResult struct {
	Period  TimeRange                 `json:"period"`   // 时间范围
	GroupBy map[string]string         `json:"group_by"` // 分组信息
	Total   int                       `json:"total"`    // 总调用次数
	Items   []APICallDistributionItem `json:"items"`    // 各API调用项
}

// APIMetricsResult 表示API指标查询结果
type APIMetricsResult struct {
	// 时间序列数据
	TimeSeries []struct {
		Timestamp time.Time `json:"timestamp"`
		Value     float64   `json:"value"`
	} `json:"time_series,omitempty"`

	// 聚合结果类型
	ResponseTime     *APIResponseTimeResult     `json:"response_time,omitempty"`
	QPS              *APIQPSResult              `json:"qps,omitempty"`
	ErrorRate        *APIErrorRateResult        `json:"error_rate,omitempty"`
	CallDistribution *APICallDistributionResult `json:"call_distribution,omitempty"`
}

// APISummaryResult API概要统计结果
type APISummaryResult struct {
	Period          TimeRange                 `json:"period"`            // 时间范围
	GroupBy         map[string]string         `json:"group_by"`          // 分组信息
	TotalCalls      int                       `json:"total_calls"`       // API调用总数
	UniqueEndpoints int                       `json:"unique_endpoints"`  // 不同API端点数量
	ErrorCount      int                       `json:"error_count"`       // 错误次数
	ErrorRate       float64                   `json:"error_rate"`        // 错误率(0-1)
	AvgResponseTime float64                   `json:"avg_response_time"` // 平均响应时间(毫秒)
	P95ResponseTime float64                   `json:"p95_response_time"` // 95%分位数响应时间(毫秒)
	TopEndpoints    []APICallDistributionItem `json:"top_endpoints"`     // 调用次数最多的端点
}
