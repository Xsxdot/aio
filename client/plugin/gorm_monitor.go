package plugin

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/xsxdot/aio/client"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/monitoring/collector"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// GORMMonitorPlugin GORM 监控插件
type GORMMonitorPlugin struct {
	monitorClient *client.MonitorClient
	userPackage   string // 用户包名，用于过滤堆栈信息
	traceKey      string // context 中 TraceID 的键名
	logger        *zap.Logger
	name          string
	slowThreshold time.Duration // 慢查询阈值，默认 200ms
	debug         bool          // debug 模式，打印详细指标信息
}

// GORMMonitorConfig GORM 监控插件配置
type GORMMonitorConfig struct {
	MonitorClient *client.MonitorClient // 监控客户端
	UserPackage   string                // 用户包名前缀
	TraceKey      string                // context 中 TraceID 的键名
	Logger        *zap.Logger           // 日志器
	SlowThreshold time.Duration         // 慢查询阈值，默认 200ms
	Debug         bool                  // debug 模式，打印详细指标信息
}

// NewGORMMonitorPlugin 创建新的 GORM 监控插件
func NewGORMMonitorPlugin(config GORMMonitorConfig) *GORMMonitorPlugin {
	plugin := &GORMMonitorPlugin{
		monitorClient: config.MonitorClient,
		userPackage:   config.UserPackage,
		traceKey:      config.TraceKey,
		logger:        config.Logger,
		name:          "gorm:monitor",
		slowThreshold: config.SlowThreshold,
		debug:         config.Debug,
	}

	// 设置默认值
	if plugin.slowThreshold == 0 {
		plugin.slowThreshold = 200 * time.Millisecond
	}

	if plugin.logger == nil {
		plugin.logger = common.GetLogger().GetZapLogger("gorm_monitor")
	}

	return plugin
}

// Name 返回插件名称 - 实现 gorm.Plugin 接口
func (p *GORMMonitorPlugin) Name() string {
	return p.name
}

// Initialize 初始化插件 - 实现 gorm.Plugin 接口
func (p *GORMMonitorPlugin) Initialize(db *gorm.DB) error {
	p.logger.Info("开始初始化 GORM 监控插件",
		zap.String("plugin_name", p.name),
		zap.Duration("slow_threshold", p.slowThreshold),
		zap.Bool("debug", p.debug))

	// 注册回调函数
	if err := p.registerCallbacks(db); err != nil {
		p.logger.Error("GORM 监控插件初始化失败", zap.Error(err))
		return fmt.Errorf("数据库监控失败: %w", err)
	}

	p.logger.Info("GORM 监控插件初始化成功")
	return nil
}

// registerCallbacks 注册回调函数
func (p *GORMMonitorPlugin) registerCallbacks(db *gorm.DB) error {
	// 直接为不同的操作类型注册具体的回调，避免重复注册
	p.registerOperationCallbacks(db)

	p.logger.Info("GORM 监控插件回调注册完成")

	return nil
}

// registerOperationCallbacks 为具体的操作类型注册回调
func (p *GORMMonitorPlugin) registerOperationCallbacks(db *gorm.DB) {
	// Create 操作
	db.Callback().Create().Before("gorm:create").Register("gorm:monitor_create_before", p.beforeCallback)
	db.Callback().Create().After("gorm:create").Register("gorm:monitor_create_after", p.afterCallback)

	// Update 操作
	db.Callback().Update().Before("gorm:update").Register("gorm:monitor_update_before", p.beforeCallback)
	db.Callback().Update().After("gorm:update").Register("gorm:monitor_update_after", p.afterCallback)

	// Delete 操作
	db.Callback().Delete().Before("gorm:delete").Register("gorm:monitor_delete_before", p.beforeCallback)
	db.Callback().Delete().After("gorm:delete").Register("gorm:monitor_delete_after", p.afterCallback)

	// Query 操作
	db.Callback().Query().Before("gorm:query").Register("gorm:monitor_query_before", p.beforeCallback)
	db.Callback().Query().After("gorm:query").Register("gorm:monitor_query_after", p.afterCallback)

	// Raw 操作
	db.Callback().Raw().Before("gorm:raw").Register("gorm:monitor_raw_before", p.beforeCallback)
	db.Callback().Raw().After("gorm:raw").Register("gorm:monitor_raw_after", p.afterCallback)

	// Row 操作
	db.Callback().Row().Before("gorm:row").Register("gorm:monitor_row_before", p.beforeCallback)
	db.Callback().Row().After("gorm:row").Register("gorm:monitor_row_after", p.afterCallback)
}

// beforeCallback 执行前回调
func (p *GORMMonitorPlugin) beforeCallback(db *gorm.DB) {
	startTime := time.Now()

	// 保存开始时间
	db.Set("gorm:monitor_start_time", startTime)

	// 获取调用堆栈信息
	if callerInfo := p.getCallerInfo(); callerInfo != nil {
		db.Set("gorm:monitor_caller", callerInfo)
	}
}

// afterCallback 执行后回调
func (p *GORMMonitorPlugin) afterCallback(db *gorm.DB) {
	// 获取开始时间
	startTimeVal, exists := db.Get("gorm:monitor_start_time")
	if !exists {
		return
	}

	startTime, ok := startTimeVal.(time.Time)
	if !ok {
		return
	}

	// 计算执行时间
	duration := time.Since(startTime)

	// 构建数据库操作指标
	metrics := p.buildDatabaseMetrics(db, duration)
	if metrics == nil {
		return
	}

	// 发送指标
	if p.monitorClient != nil {
		p.monitorClient.RecordDatabaseOperation(metrics)
	}
}

// buildDatabaseMetrics 构建数据库操作指标
func (p *GORMMonitorPlugin) buildDatabaseMetrics(db *gorm.DB, duration time.Duration) *collector.DatabaseOperationMetrics {
	metrics := &collector.DatabaseOperationMetrics{
		Timestamp: time.Now(),
		Duration:  float64(duration.Nanoseconds()) / 1e6, // 转换为毫秒
	}

	// 获取 TraceID
	if p.traceKey != "" && db.Statement != nil && db.Statement.Context != nil {
		if traceID := p.getTraceIDFromContext(db.Statement.Context); traceID != "" {
			metrics.TraceID = traceID
		}
	}

	// 获取调用信息
	if callerVal, exists := db.Get("gorm:monitor_caller"); exists {
		if caller, ok := callerVal.(*CallerInfo); ok {
			metrics.Method = caller.Function
			metrics.FileName = caller.File
			metrics.Line = caller.Line
		}
	}

	// 获取数据库信息
	if db.Statement != nil {
		metrics.TableName = db.Statement.Table
		metrics.SQL = db.Statement.SQL.String()
		metrics.RowsAffected = db.RowsAffected
	}

	// 获取错误信息
	if db.Error != nil {
		metrics.ErrorMessage = db.Error.Error()
		metrics.ErrorCode = p.extractErrorCode(db.Error.Error())
	}

	// 获取操作类型
	metrics.Operation = p.getOperationType(metrics.SQL)

	// 获取驱动名称
	if db.Dialector != nil {
		metrics.Driver = db.Dialector.Name()
	}

	// 判断是否为慢查询
	metrics.IsSlowQuery = duration >= p.slowThreshold

	// Debug 模式下打印详细的指标信息
	if p.debug {
		p.printDebugMetrics(metrics, duration)
	}

	return metrics
}

// extractErrorCode 提取错误码
func (p *GORMMonitorPlugin) extractErrorCode(errorMessage string) string {
	errorMessage = strings.ToLower(errorMessage)
	switch {
	case strings.Contains(errorMessage, "duplicate"):
		return "DUPLICATE"
	case strings.Contains(errorMessage, "not found"):
		return "NOT_FOUND"
	case strings.Contains(errorMessage, "timeout"):
		return "TIMEOUT"
	case strings.Contains(errorMessage, "connection"):
		return "CONNECTION"
	default:
		return "UNKNOWN"
	}
}

// CallerInfo 调用者信息
type CallerInfo struct {
	Function string
	File     string
	Line     int
}

// getCallerInfo 获取调用者信息
func (p *GORMMonitorPlugin) getCallerInfo() *CallerInfo {
	const maxStackDepth = 20
	pcs := make([]uintptr, maxStackDepth)
	n := runtime.Callers(0, pcs)

	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()

		if p.isUserCode(frame.Function, frame.File) {
			return &CallerInfo{
				Function: p.extractFunctionName(frame.Function),
				File:     p.extractFileName(frame.File),
				Line:     frame.Line,
			}
		}

		if !more {
			break
		}
	}

	return nil
}

// isUserCode 判断是否为用户代码
func (p *GORMMonitorPlugin) isUserCode(function, file string) bool {
	// 跳过 GORM 相关代码
	if strings.Contains(function, "gorm.io/gorm") ||
		strings.Contains(function, "github.com/xsxdot/aio/client/plugin") ||
		strings.Contains(file, "gorm.io/gorm") ||
		strings.Contains(file, "/client/plugin/") {
		return false
	}

	// 如果指定了用户包名，只匹配用户包
	if p.userPackage != "" {
		return strings.Contains(function, p.userPackage) || strings.Contains(file, p.userPackage)
	}

	// 否则排除常见的非用户代码
	excludePatterns := []string{
		"runtime.",
		"reflect.",
		"net/http.",
		"github.com/gin-gonic/gin",
		"github.com/gofiber/fiber",
	}

	for _, pattern := range excludePatterns {
		if strings.Contains(function, pattern) {
			return false
		}
	}

	return true
}

// extractFunctionName 提取函数名
func (p *GORMMonitorPlugin) extractFunctionName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if dotIndex := strings.LastIndex(lastPart, "."); dotIndex != -1 {
			return lastPart[dotIndex+1:]
		}
		return lastPart
	}
	return fullName
}

// extractFileName 提取文件名
func (p *GORMMonitorPlugin) extractFileName(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullPath
}

// getTraceIDFromContext 从 context 中获取 TraceID
func (p *GORMMonitorPlugin) getTraceIDFromContext(ctx context.Context) string {
	if ctx == nil || p.traceKey == "" {
		return ""
	}

	value := ctx.Value(p.traceKey)
	if value == nil {
		return ""
	}

	return fmt.Sprintf("%v", value)
}

// getOperationType 获取操作类型
func (p *GORMMonitorPlugin) getOperationType(sql string) collector.DatabaseOperation {
	if sql == "" {
		return collector.DatabaseOperationOTHER
	}

	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))

	switch {
	case strings.HasPrefix(sqlUpper, "SELECT"):
		return collector.DatabaseOperationSELECT
	case strings.HasPrefix(sqlUpper, "INSERT"):
		return collector.DatabaseOperationINSERT
	case strings.HasPrefix(sqlUpper, "UPDATE"):
		return collector.DatabaseOperationUPDATE
	case strings.HasPrefix(sqlUpper, "DELETE"):
		return collector.DatabaseOperationDELETE
	case strings.HasPrefix(sqlUpper, "CREATE"):
		return collector.DatabaseOperationCREATE
	case strings.HasPrefix(sqlUpper, "DROP"):
		return collector.DatabaseOperationDROP
	case strings.HasPrefix(sqlUpper, "ALTER"):
		return collector.DatabaseOperationALTER
	default:
		return collector.DatabaseOperationOTHER
	}
}

// printDebugMetrics 在 debug 模式下打印详细的指标信息
func (p *GORMMonitorPlugin) printDebugMetrics(metrics *collector.DatabaseOperationMetrics, duration time.Duration) {
	// 打印基本执行信息
	p.logger.Info("🐛 [GORM DEBUG] 数据库操作详情",
		zap.String("service_name", metrics.ServiceName),
		zap.String("instance_id", metrics.InstanceID),
		zap.String("env", metrics.Env),
		zap.String("trace_id", metrics.TraceID),
		zap.String("database_name", metrics.DatabaseName),
		zap.String("table_name", metrics.TableName),
		zap.String("operation", string(metrics.Operation)),
		zap.String("driver", metrics.Driver),
		zap.Float64("duration_ms", metrics.Duration),
		zap.Int64("rows_affected", metrics.RowsAffected),
		zap.Int64("rows_returned", metrics.RowsReturned),
		zap.Bool("is_slow_query", metrics.IsSlowQuery),
		zap.String("method", metrics.Method),
		zap.String("file_name", metrics.FileName),
		zap.Int("line", metrics.Line),
		zap.String("error_code", metrics.ErrorCode),
		zap.String("error_message", metrics.ErrorMessage),
	)

	// 打印 SQL 语句
	if metrics.SQL != "" {
		sqlLog := metrics.SQL
		if len(sqlLog) > 500 {
			sqlLog = sqlLog[:500] + "... (truncated)"
		}
		p.logger.Info("🐛 [GORM DEBUG] SQL 语句", zap.String("sql", sqlLog))
	}

	// 性能提示
	if metrics.IsSlowQuery {
		p.logger.Warn("🐛 [GORM DEBUG] 🐌 检测到慢查询",
			zap.Float64("duration_ms", metrics.Duration),
			zap.Duration("threshold", p.slowThreshold),
			zap.String("suggestion", "考虑优化SQL语句或添加索引"))
	}

	// 错误提示
	if metrics.ErrorMessage != "" {
		p.logger.Error("🐛 [GORM DEBUG] ❌ 数据库操作失败",
			zap.String("error_code", metrics.ErrorCode),
			zap.String("error_message", metrics.ErrorMessage),
			zap.String("table", metrics.TableName),
			zap.String("operation", string(metrics.Operation)))
	}
}

// ValidateGORMDB 验证 GORM DB 对象
func ValidateGORMDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("数据库对象不能为 nil")
	}
	return nil
}

// DebugGORMDB 打印 GORM DB 对象的详细信息，用于调试
func DebugGORMDB(db *gorm.DB) {
	fmt.Printf("=== GORM DB 对象调试信息 ===\n")

	if db == nil {
		fmt.Printf("❌ 数据库对象为 nil\n")
		return
	}

	fmt.Printf("✅ 数据库对象类型: %T\n", db)

	if db.Config != nil {
		fmt.Printf("✅ 配置信息存在\n")
	} else {
		fmt.Printf("❌ 配置信息缺失\n")
	}

	if db.Statement != nil {
		fmt.Printf("✅ Statement 存在\n")
	} else {
		fmt.Printf("❌ Statement 缺失\n")
	}

	if db.Dialector != nil {
		fmt.Printf("✅ 驱动类型: %s\n", db.Dialector.Name())
	} else {
		fmt.Printf("❌ 驱动信息缺失\n")
	}

	fmt.Printf("✅ 验证通过，这是一个有效的 GORM DB 对象\n")
	fmt.Printf("========================\n")
}
