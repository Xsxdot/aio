// Package fiber_handle 提供 Fiber 框架的中间件处理器
//
// API 监控中间件使用示例：
//
// 1. 基本使用：
//
//	monitorClient := NewMonitorClient(...)
//	monitorConfig := fiber_handle.MonitorConfig{
//		Client:      monitorClient,
//		ServiceName: "xiaozhizhang",
//		InstanceID:  "instance-001",
//	}
//	app.Use(fiber_handle.NewAPIMonitor(monitorConfig))
//
// 2. 带过滤器使用（跳过健康检查和静态文件）：
//
//	filters := []fiber_handle.FilterFunc{
//		fiber_handle.SkipHealthCheck,
//		fiber_handle.SkipStaticFiles,
//	}
//	app.Use(fiber_handle.NewAPIMonitorWithFilters(monitorConfig, filters))
//
// 3. 自定义过滤器：
//
//	customFilter := func(c *fiber.Ctx) bool {
//		return !strings.HasPrefix(c.Path(), "/internal")
//	}
//	filters := []fiber_handle.FilterFunc{customFilter}
//	app.Use(fiber_handle.NewAPIMonitorWithFilters(monitorConfig, filters))
package fiber_handle

import (
	"fmt"
	"strconv"
	"strings"
	"xiaozhizhang/pkg/core/consts"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/monitoring/collector"
)

// MonitorClient 接口定义，直接使用 MonitorClient
type MonitorClient interface {
	RecordAPICall(apiMetrics *collector.APICallMetrics)
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Client MonitorClient
}

// FilterFunc 过滤器函数类型
type FilterFunc func(c *fiber.Ctx) bool

// NewAPIMonitor 创建 API 监控中间件
func NewAPIMonitor(config MonitorConfig) fiber.Handler {
	return NewAPIMonitorWithFilters(config, nil)
}

// NewAPIMonitorWithFilters 创建带过滤器列表的 API 监控中间件
func NewAPIMonitorWithFilters(config MonitorConfig, filters ...FilterFunc) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if config.Client == nil {
			// 如果没有提供监控客户端，则跳过监控
			return c.Next()
		}

		// 应用过滤器列表，如果任何一个过滤器返回 false，则跳过监控
		if filters != nil {
			for _, filter := range filters {
				if !filter(c) {
					return c.Next()
				}
			}
		}

		// 记录开始时间
		startTime := time.Now()

		// 执行下一个中间件/处理器
		err := c.Next()

		// 记录 API 调用指标
		recordAPIMetrics(config, c, startTime, err)

		return err
	}
}

// recordAPIMetrics 记录 API 调用指标
func recordAPIMetrics(config MonitorConfig, c *fiber.Ctx, startTime time.Time, handlerErr error) {
	// 计算执行时间
	duration := time.Since(startTime)

	// 获取路径，优先使用路由路径，如果没有则使用实际路径
	path := c.Route().Path
	if path == "" {
		path = c.Path()
	}

	// 构建 API 调用指标
	apiMetrics := &collector.APICallMetrics{
		Timestamp:  startTime,
		Method:     collector.HTTPMethod(c.Method()),
		Path:       path,
		StatusCode: c.Response().StatusCode(),
		Duration:   float64(duration.Milliseconds()),
	}

	// 添加请求大小（如果有 Content-Length）
	if contentLength := c.Get("Content-Length"); contentLength != "" {
		if size, parseErr := strconv.ParseInt(contentLength, 10, 64); parseErr == nil {
			apiMetrics.RequestSize = size
		}
	}

	// 添加响应大小
	apiMetrics.ResponseSize = int64(len(c.Response().Body()))

	// 安全地获取用户 ID
	if userID := c.Locals("user_id"); userID != nil {
		apiMetrics.UserID = safeStringConvert(userID)
	} else if userID := c.Locals("userID"); userID != nil {
		// 兼容不同的命名方式
		apiMetrics.UserID = safeStringConvert(userID)
	}

	if traceID := c.Locals(consts.TraceKey); traceID != nil {
		traceKey := safeStringConvert(traceID)
		apiMetrics.TraceID = traceKey
		apiMetrics.SpanID = traceKey
		apiMetrics.RequestID = traceKey
	}

	// 添加客户端 IP
	apiMetrics.ClientIP = c.IP()

	// 添加用户代理
	apiMetrics.UserAgent = c.Get("User-Agent")

	// 添加错误信息（如果有）
	if handlerErr != nil {
		apiMetrics.ErrorMessage = handlerErr.Error()
	}

	// 记录 API 调用
	config.Client.RecordAPICall(apiMetrics)
}

// safeStringConvert 安全地将任意类型转换为字符串
func safeStringConvert(value interface{}) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", value)
}

// SkipHealthCheck 健康检查过滤器，跳过健康检查端点的监控
func SkipHealthCheck(c *fiber.Ctx) bool {
	path := strings.ToLower(c.Path())
	return !strings.Contains(path, "/health")
}

// SkipStaticFiles 静态文件过滤器，跳过静态资源的监控
func SkipStaticFiles(c *fiber.Ctx) bool {
	path := strings.ToLower(c.Path())
	staticExtensions := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".woff", ".woff2", ".ttf", ".eot"}

	for _, ext := range staticExtensions {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}
	return true
}

// SkipAdminPaths 跳过管理员路径的监控
func SkipAdminPaths(c *fiber.Ctx) bool {
	path := strings.ToLower(c.Path())
	return !strings.HasPrefix(path, "/admin") &&
		!strings.HasPrefix(path, "/internal")
}

// OnlyMonitorAPI 仅监控API路径
func OnlyPathStartWith(paths ...string) FilterFunc {
	return func(c *fiber.Ctx) bool {
		for _, path := range paths {
			s := c.Path()
			if strings.HasPrefix(s, path) {
				return true
			}
		}
		return false
	}
}

// SkipMethods 跳过指定HTTP方法的监控
func SkipMethods(methods ...string) FilterFunc {
	skipMap := make(map[string]bool)
	for _, method := range methods {
		skipMap[strings.ToUpper(method)] = true
	}

	return func(c *fiber.Ctx) bool {
		return !skipMap[c.Method()]
	}
}
