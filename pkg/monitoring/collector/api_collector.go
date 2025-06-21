// Package collector 实现API指标采集功能
package collector

import (
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"sync"
	"time"

	"go.uber.org/zap"
)

// APICollectorConfig API收集器配置
type APICollectorConfig struct {
	ServiceName string                       // 服务名称
	InstanceID  string                       // 实例ID
	Logger      *zap.Logger                  // 日志记录器
	Storage     storage.UnifiedMetricStorage // 存储层
}

// APICollector API指标收集器
type APICollector struct {
	config  APICollectorConfig
	logger  *zap.Logger
	storage storage.UnifiedMetricStorage
	mu      sync.RWMutex
}

// NewAPICollector 创建新的API收集器
func NewAPICollector(config APICollectorConfig) *APICollector {
	// 设置默认logger，如果没有提供
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &APICollector{
		config:  config,
		logger:  logger,
		storage: config.Storage,
	}
}

// RecordAPICall 记录API调用指标
func (c *APICollector) RecordAPICall(callMetrics *APICallMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 补充服务信息
	if callMetrics.ServiceName == "" {
		callMetrics.ServiceName = c.config.ServiceName
	}
	if callMetrics.InstanceID == "" {
		callMetrics.InstanceID = c.config.InstanceID
	}
	if callMetrics.Timestamp.IsZero() {
		callMetrics.Timestamp = time.Now()
	}

	// 使用统一存储方法
	if err := c.storage.StoreMetricProvider(callMetrics); err != nil {
		c.logger.Error("存储API指标失败",
			zap.String("path", callMetrics.Path),
			zap.String("method", string(callMetrics.Method)),
			zap.Error(err))
		return err
	}

	c.logger.Debug("API指标记录成功",
		zap.String("service_name", callMetrics.ServiceName),
		zap.String("instance_id", callMetrics.InstanceID),
		zap.String("path", callMetrics.Path),
		zap.String("method", string(callMetrics.Method)),
		zap.Int("status_code", callMetrics.StatusCode),
		zap.Float64("duration_ms", callMetrics.Duration))

	return nil
}

// RecordBatch 批量记录API调用指标
func (c *APICollector) RecordBatch(callMetrics []*APICallMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(callMetrics) == 0 {
		return nil
	}

	// 转换为models.MetricPoint切片
	allPoints := make([]models.MetricPoint, 0, len(callMetrics)*5) // 估算每个API调用产生5个指标点

	for _, apiCall := range callMetrics {
		// 补充服务信息
		if apiCall.ServiceName == "" {
			apiCall.ServiceName = c.config.ServiceName
		}
		if apiCall.InstanceID == "" {
			apiCall.InstanceID = c.config.InstanceID
		}
		if apiCall.Timestamp.IsZero() {
			apiCall.Timestamp = time.Now()
		}

		points := apiCall.ToMetricPoints()
		allPoints = append(allPoints, points...)
	}

	// 批量存储
	if err := c.storage.StoreMetricPoints(allPoints); err != nil {
		c.logger.Error("批量存储API指标失败", zap.Error(err))
		return err
	}

	c.logger.Debug("批量API指标记录成功", zap.Int("count", len(callMetrics)))
	return nil
}

// GetSupportedMetrics 获取支持的指标名称
func (c *APICollector) GetSupportedMetrics() []string {
	// 创建一个空的APICallMetrics实例来获取支持的指标
	dummyMetrics := &APICallMetrics{}
	return dummyMetrics.GetMetricNames()
}

// Start 启动API收集器（实际上API收集器是被动的，不需要主动启动）
func (c *APICollector) Start() error {
	c.logger.Info("API指标收集器已准备就绪",
		zap.String("service_name", c.config.ServiceName),
		zap.String("instance_id", c.config.InstanceID))
	return nil
}

// Stop 停止API收集器
func (c *APICollector) Stop() error {
	c.logger.Info("API指标收集器已停止")
	return nil
}

// ========================= API 监控相关定义 =========================

// APIMetricName 定义API指标的名称常量
type APIMetricName string

const (
	// API调用相关指标
	MetricAPIRequestCount       APIMetricName = "api.request.count"       // API请求总数
	MetricAPIRequestDuration    APIMetricName = "api.request.duration"    // API请求耗时
	MetricAPIRequestSize        APIMetricName = "api.request.size"        // API请求大小
	MetricAPIResponseSize       APIMetricName = "api.response.size"       // API响应大小
	MetricAPIErrorCount         APIMetricName = "api.error.count"         // API错误数
	MetricAPIErrorRate          APIMetricName = "api.error.rate"          // API错误率
	MetricAPISuccessRate        APIMetricName = "api.success.rate"        // API成功率
	MetricAPIThroughput         APIMetricName = "api.throughput"          // API吞吐量(请求/秒)
	MetricAPILatencyP50         APIMetricName = "api.latency.p50"         // API延迟P50
	MetricAPILatencyP90         APIMetricName = "api.latency.p90"         // API延迟P90
	MetricAPILatencyP95         APIMetricName = "api.latency.p95"         // API延迟P95
	MetricAPILatencyP99         APIMetricName = "api.latency.p99"         // API延迟P99
	MetricAPIConcurrentRequests APIMetricName = "api.concurrent.requests" // API并发请求数
	MetricAPIActiveConnections  APIMetricName = "api.active.connections"  // API活跃连接数
	MetricAPITimeoutCount       APIMetricName = "api.timeout.count"       // API超时次数
	MetricAPIRetryCount         APIMetricName = "api.retry.count"         // API重试次数
)

// HTTPMethod 定义HTTP方法类型
type HTTPMethod string

const (
	HTTPMethodGET     HTTPMethod = "GET"
	HTTPMethodPOST    HTTPMethod = "POST"
	HTTPMethodPUT     HTTPMethod = "PUT"
	HTTPMethodDELETE  HTTPMethod = "DELETE"
	HTTPMethodPATCH   HTTPMethod = "PATCH"
	HTTPMethodHEAD    HTTPMethod = "HEAD"
	HTTPMethodOPTIONS HTTPMethod = "OPTIONS"
)

// APICallMetrics 表示单次API调用的指标数据
type APICallMetrics struct {
	// 基础信息
	Timestamp   time.Time `json:"timestamp"`            // 请求时间戳
	ServiceName string    `json:"service_name"`         // 服务名称
	InstanceID  string    `json:"instance_id"`          // 服务实例ID
	RequestID   string    `json:"request_id,omitempty"` // 请求ID（用于链路追踪）
	TraceID     string    `json:"trace_id,omitempty"`   // 链路追踪ID
	SpanID      string    `json:"span_id,omitempty"`    // Span ID

	// API信息
	Method  HTTPMethod `json:"method"`            // HTTP方法
	Path    string     `json:"path"`              // API路径
	Handler string     `json:"handler,omitempty"` // 处理器名称
	Version string     `json:"version,omitempty"` // API版本

	// 请求响应信息
	StatusCode   int     `json:"status_code"`         // HTTP状态码
	Duration     float64 `json:"duration_ms"`         // 请求耗时(毫秒)
	RequestSize  int64   `json:"request_size_bytes"`  // 请求大小(字节)
	ResponseSize int64   `json:"response_size_bytes"` // 响应大小(字节)

	// 客户端信息
	ClientIP  string `json:"client_ip,omitempty"`  // 客户端IP
	UserAgent string `json:"user_agent,omitempty"` // 用户代理
	UserID    string `json:"user_id,omitempty"`    // 用户ID

	// 错误信息
	ErrorCode    string `json:"error_code,omitempty"`    // 错误码
	ErrorMessage string `json:"error_message,omitempty"` // 错误信息

	// 额外标签
	Labels map[string]string `json:"labels,omitempty"` // 自定义标签
}

// GetMetricNames 实现 MetricProvider 接口
func (a *APICallMetrics) GetMetricNames() []string {
	return []string{
		string(MetricAPIRequestCount),
		string(MetricAPIRequestDuration),
		string(MetricAPIRequestSize),
		string(MetricAPIResponseSize),
		string(MetricAPIErrorCount),
	}
}

// GetCategory 实现 MetricProvider 接口
func (a *APICallMetrics) GetCategory() models.MetricCategory {
	return models.CategoryAPI
}

// Tomodels.MetricPoints 实现 MetricProvider 接口
func (a *APICallMetrics) ToMetricPoints() []models.MetricPoint {
	baseLabels := map[string]string{
		"service_name": a.ServiceName,
		"instance_id":  a.InstanceID,
		"method":       string(a.Method),
		"path":         a.Path,
		"status_code":  fmt.Sprintf("%d", a.StatusCode),
	}

	// 合并自定义标签
	for k, v := range a.Labels {
		baseLabels[k] = v
	}

	points := []models.MetricPoint{
		{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAPIRequestCount),
			MetricType: models.MetricTypeCounter,
			Value:      1,
			Source:     a.ServiceName,
			Instance:   a.InstanceID,
			Category:   models.CategoryAPI,
			Labels:     baseLabels,
		},
		{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAPIRequestDuration),
			MetricType: models.MetricTypeGauge,
			Value:      a.Duration,
			Source:     a.ServiceName,
			Instance:   a.InstanceID,
			Category:   models.CategoryAPI,
			Labels:     baseLabels,
			Unit:       "ms",
		},
		{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAPIRequestSize),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.RequestSize),
			Source:     a.ServiceName,
			Instance:   a.InstanceID,
			Category:   models.CategoryAPI,
			Labels:     baseLabels,
			Unit:       "bytes",
		},
		{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAPIResponseSize),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.ResponseSize),
			Source:     a.ServiceName,
			Instance:   a.InstanceID,
			Category:   models.CategoryAPI,
			Labels:     baseLabels,
			Unit:       "bytes",
		},
	}

	// 如果是错误请求，添加错误计数指标
	if a.StatusCode >= 400 {
		points = append(points, models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAPIErrorCount),
			MetricType: models.MetricTypeCounter,
			Value:      1,
			Source:     a.ServiceName,
			Instance:   a.InstanceID,
			Category:   models.CategoryAPI,
			Labels:     baseLabels,
		})
	}

	return points
}
