// Package collector 实现数据库指标采集功能
package collector

import (
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"

	"go.uber.org/zap"
)

// DatabaseCollectorConfig 数据库收集器配置
type DatabaseCollectorConfig struct {
	ServiceName string                       // 服务名称
	InstanceID  string                       // 实例ID
	Env         string                       // 环境标识（如：dev, test, prod）
	Logger      *zap.Logger                  // 日志记录器
	Storage     storage.UnifiedMetricStorage // 存储层
}

// DatabaseCollector 数据库指标收集器
type DatabaseCollector struct {
	config  DatabaseCollectorConfig
	logger  *zap.Logger
	storage storage.UnifiedMetricStorage
	mu      sync.RWMutex
}

// NewDatabaseCollector 创建新的数据库收集器
func NewDatabaseCollector(config DatabaseCollectorConfig) *DatabaseCollector {
	// 设置默认logger，如果没有提供
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &DatabaseCollector{
		config:  config,
		logger:  logger,
		storage: config.Storage,
	}
}

// RecordDatabaseOperation 记录数据库操作指标
func (c *DatabaseCollector) RecordDatabaseOperation(operationMetrics *DatabaseOperationMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 补充服务信息
	if operationMetrics.ServiceName == "" {
		operationMetrics.ServiceName = c.config.ServiceName
	}
	if operationMetrics.InstanceID == "" {
		operationMetrics.InstanceID = c.config.InstanceID
	}
	if operationMetrics.Env == "" {
		operationMetrics.Env = c.config.Env
	}
	if operationMetrics.Timestamp.IsZero() {
		operationMetrics.Timestamp = time.Now()
	}

	// 使用统一存储方法
	if err := c.storage.StoreMetricProvider(operationMetrics); err != nil {
		c.logger.Error("存储数据库指标失败",
			zap.String("table", operationMetrics.TableName),
			zap.String("operation", string(operationMetrics.Operation)),
			zap.String("env", operationMetrics.Env),
			zap.Error(err))
		return err
	}

	c.logger.Debug("数据库指标记录成功",
		zap.String("service_name", operationMetrics.ServiceName),
		zap.String("instance_id", operationMetrics.InstanceID),
		zap.String("env", operationMetrics.Env),
		zap.String("table", operationMetrics.TableName),
		zap.String("operation", string(operationMetrics.Operation)),
		zap.Float64("duration_ms", operationMetrics.Duration),
		zap.Bool("has_error", operationMetrics.ErrorMessage != ""))

	return nil
}

// RecordBatch 批量记录数据库操作指标
func (c *DatabaseCollector) RecordBatch(operationMetrics []*DatabaseOperationMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(operationMetrics) == 0 {
		return nil
	}

	// 转换为models.MetricPoint切片
	allPoints := make([]models.MetricPoint, 0, len(operationMetrics)*5) // 估算每个数据库操作产生5个指标点

	for _, dbOperation := range operationMetrics {
		// 补充服务信息
		if dbOperation.ServiceName == "" {
			dbOperation.ServiceName = c.config.ServiceName
		}
		if dbOperation.InstanceID == "" {
			dbOperation.InstanceID = c.config.InstanceID
		}
		if dbOperation.Env == "" {
			dbOperation.Env = c.config.Env
		}
		if dbOperation.Timestamp.IsZero() {
			dbOperation.Timestamp = time.Now()
		}

		points := dbOperation.ToMetricPoints()
		allPoints = append(allPoints, points...)
	}

	// 批量存储
	if err := c.storage.StoreMetricPoints(allPoints); err != nil {
		c.logger.Error("批量存储数据库指标失败", zap.Error(err))
		return err
	}

	c.logger.Debug("批量数据库指标记录成功", zap.Int("count", len(operationMetrics)))
	return nil
}

// GetSupportedMetrics 获取支持的指标名称
func (c *DatabaseCollector) GetSupportedMetrics() []string {
	// 创建一个空的DatabaseOperationMetrics实例来获取支持的指标
	dummyMetrics := &DatabaseOperationMetrics{}
	return dummyMetrics.GetMetricNames()
}

// Start 启动数据库收集器（实际上数据库收集器是被动的，不需要主动启动）
func (c *DatabaseCollector) Start() error {
	c.logger.Info("数据库指标收集器已准备就绪",
		zap.String("service_name", c.config.ServiceName),
		zap.String("instance_id", c.config.InstanceID),
		zap.String("env", c.config.Env))
	return nil
}

// Stop 停止数据库收集器
func (c *DatabaseCollector) Stop() error {
	c.logger.Info("数据库指标收集器已停止")
	return nil
}

// ========================= 数据库监控相关定义 =========================

// DatabaseMetricName 定义数据库指标的名称常量
type DatabaseMetricName string

const (
	// 数据库操作相关指标
	MetricDBOperationCount    DatabaseMetricName = "db.operation.count"    // 数据库操作总数
	MetricDBOperationDuration DatabaseMetricName = "db.operation.duration" // 数据库操作耗时
	MetricDBErrorCount        DatabaseMetricName = "db.error.count"        // 数据库错误数
	MetricDBSlowQueryCount    DatabaseMetricName = "db.slow.query.count"   // 慢查询数
	MetricDBConnectionCount   DatabaseMetricName = "db.connection.count"   // 数据库连接数
	MetricDBRowsAffected      DatabaseMetricName = "db.rows.affected"      // 影响行数
)

// DatabaseOperation 定义数据库操作类型
type DatabaseOperation string

const (
	DatabaseOperationSELECT DatabaseOperation = "SELECT"
	DatabaseOperationINSERT DatabaseOperation = "INSERT"
	DatabaseOperationUPDATE DatabaseOperation = "UPDATE"
	DatabaseOperationDELETE DatabaseOperation = "DELETE"
	DatabaseOperationCREATE DatabaseOperation = "CREATE"
	DatabaseOperationDROP   DatabaseOperation = "DROP"
	DatabaseOperationALTER  DatabaseOperation = "ALTER"
	DatabaseOperationOTHER  DatabaseOperation = "OTHER"
)

// DatabaseOperationMetrics 表示单次数据库操作的指标数据
type DatabaseOperationMetrics struct {
	// 基础信息
	Timestamp   time.Time `json:"timestamp"`            // 操作时间戳
	ServiceName string    `json:"service_name"`         // 服务名称
	InstanceID  string    `json:"instance_id"`          // 服务实例ID
	Env         string    `json:"env"`                  // 环境标识（如：dev, test, prod）
	RequestID   string    `json:"request_id,omitempty"` // 请求ID（用于链路追踪）
	TraceID     string    `json:"trace_id,omitempty"`   // 链路追踪ID
	SpanID      string    `json:"span_id,omitempty"`    // Span ID

	// 数据库信息
	DatabaseName string            `json:"database_name"`    // 数据库名称
	TableName    string            `json:"table_name"`       // 数据表名称
	Operation    DatabaseOperation `json:"operation"`        // 操作类型
	Method       string            `json:"method,omitempty"` // 执行方法（如函数名、方法名）
	SQL          string            `json:"sql,omitempty"`    // SQL语句

	// 执行信息
	Duration     float64 `json:"duration_ms"`   // 执行耗时(毫秒)
	RowsAffected int64   `json:"rows_affected"` // 影响行数
	RowsReturned int64   `json:"rows_returned"` // 返回行数

	Driver string `json:"driver,omitempty"` // 数据库驱动

	// 错误信息
	ErrorCode    string `json:"error_code,omitempty"`    // 错误码
	ErrorMessage string `json:"error_message,omitempty"` // 错误信息

	// 性能分析
	IsSlowQuery bool `json:"is_slow_query"` // 是否为慢查询

	// 额外标签
	Labels map[string]string `json:"labels,omitempty"` // 自定义标签
}

// GetMetricNames 实现 MetricProvider 接口
func (d *DatabaseOperationMetrics) GetMetricNames() []string {
	return []string{
		string(MetricDBOperationCount),
		string(MetricDBOperationDuration),
		string(MetricDBErrorCount),
		string(MetricDBSlowQueryCount),
		string(MetricDBRowsAffected),
	}
}

// GetCategory 实现 MetricProvider 接口
func (d *DatabaseOperationMetrics) GetCategory() models.MetricCategory {
	return models.CategoryCustom // 数据库属于自定义类别
}

// ToMetricPoints 实现 MetricProvider 接口
func (d *DatabaseOperationMetrics) ToMetricPoints() []models.MetricPoint {
	baseLabels := map[string]string{
		"service_name":  d.ServiceName,
		"instance_id":   d.InstanceID,
		"env":           d.Env,
		"database_name": d.DatabaseName,
		"table_name":    d.TableName,
		"operation":     string(d.Operation),
	}

	// 添加可选标签
	if d.Method != "" {
		baseLabels["method"] = d.Method
	}
	if d.Driver != "" {
		baseLabels["driver"] = d.Driver
	}

	// 合并自定义标签
	for k, v := range d.Labels {
		baseLabels[k] = v
	}

	points := []models.MetricPoint{
		{
			Timestamp:  d.Timestamp,
			MetricName: string(MetricDBOperationCount),
			MetricType: models.MetricTypeCounter,
			Value:      1,
			Source:     d.ServiceName,
			Instance:   d.InstanceID,
			Category:   models.CategoryCustom,
			Labels:     baseLabels,
		},
		{
			Timestamp:  d.Timestamp,
			MetricName: string(MetricDBOperationDuration),
			MetricType: models.MetricTypeGauge,
			Value:      d.Duration,
			Source:     d.ServiceName,
			Instance:   d.InstanceID,
			Category:   models.CategoryCustom,
			Labels:     baseLabels,
			Unit:       "ms",
		},
		{
			Timestamp:  d.Timestamp,
			MetricName: string(MetricDBRowsAffected),
			MetricType: models.MetricTypeGauge,
			Value:      float64(d.RowsAffected),
			Source:     d.ServiceName,
			Instance:   d.InstanceID,
			Category:   models.CategoryCustom,
			Labels:     baseLabels,
		},
	}

	// 如果有错误，添加错误计数指标
	if d.ErrorMessage != "" {
		errorLabels := make(map[string]string)
		for k, v := range baseLabels {
			errorLabels[k] = v
		}
		if d.ErrorCode != "" {
			errorLabels["error_code"] = d.ErrorCode
		}

		points = append(points, models.MetricPoint{
			Timestamp:  d.Timestamp,
			MetricName: string(MetricDBErrorCount),
			MetricType: models.MetricTypeCounter,
			Value:      1,
			Source:     d.ServiceName,
			Instance:   d.InstanceID,
			Category:   models.CategoryCustom,
			Labels:     errorLabels,
		})
	}

	// 如果是慢查询，添加慢查询计数指标
	if d.IsSlowQuery {
		points = append(points, models.MetricPoint{
			Timestamp:  d.Timestamp,
			MetricName: string(MetricDBSlowQueryCount),
			MetricType: models.MetricTypeCounter,
			Value:      1,
			Source:     d.ServiceName,
			Instance:   d.InstanceID,
			Category:   models.CategoryCustom,
			Labels:     baseLabels,
		})
	}

	return points
}
