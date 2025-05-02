// Package api 提供监控系统的HTTP API接口
package api

import (
	"crypto/rand"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xsxdot/aio/internal/monitoring/alerting"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"github.com/xsxdot/aio/internal/monitoring/notifier"
	"github.com/xsxdot/aio/internal/monitoring/storage"
	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// API 监控系统的HTTP API
type API struct {
	storage     *storage.Storage
	alertMgr    *alerting.Manager
	notifierMgr *notifier.Manager
	logger      *zap.Logger

	// 服务监控API
	serviceAPI *ServiceAPI
}

// NewAPI 创建新的监控系统API
func NewAPI(storage *storage.Storage, alertMgr *alerting.Manager, notifierMgr *notifier.Manager, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	api := &API{
		storage:     storage,
		alertMgr:    alertMgr,
		notifierMgr: notifierMgr,
		logger:      logger,
	}

	// 初始化服务监控API
	api.serviceAPI = NewServiceAPI(storage)

	return api
}

// RegisterRoutes 注册所有API路由
func (api *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建API组
	monitoringGroup := router.Group("/monitoring")

	// 健康检查
	monitoringGroup.Get("/health", api.handleHealthCheck)

	// 系统概览
	monitoringGroup.Get("/system/overview", authHandler, api.getSystemOverview)

	// 获取所有可用指标名称
	monitoringGroup.Get("/metrics/names", authHandler, api.getAllMetricNames)

	// 通用指标路径，支持直接查询指标
	monitoringGroup.Get("/metrics/:name", authHandler, api.getMetricsByName)

	// 服务器指标相关API
	monitoringGroup.Get("/server/metrics", authHandler, api.getServerMetrics)
	monitoringGroup.Get("/server/metrics/:name", authHandler, api.getServerMetricByName)

	// 应用指标相关API
	monitoringGroup.Get("/app/metrics", authHandler, api.getAppMetrics)
	monitoringGroup.Get("/app/metrics/:name", authHandler, api.getAppMetricByName)

	// API调用相关
	monitoringGroup.Get("/api/calls", authHandler, api.getAPICalls)
	monitoringGroup.Get("/api/calls/:endpoint", authHandler, api.getAPICallsByEndpoint)

	// 通用指标查询API
	monitoringGroup.Get("/metrics/query", authHandler, api.handleMetricsQuery)

	// API指标查询和聚合API
	api.setupAPIMetricsRoutes(monitoringGroup, authHandler, adminRoleHandler)

	// 告警规则CRUD
	alertGroup := monitoringGroup.Group("/alerts")
	alertGroup.Get("/rules", authHandler, api.getAlertRules)
	alertGroup.Get("/rules/:id", authHandler, api.getAlertRule)
	alertGroup.Post("/rules", authHandler, adminRoleHandler, api.createAlertRule)
	alertGroup.Put("/rules/:id", authHandler, adminRoleHandler, api.updateAlertRule)
	alertGroup.Delete("/rules/:id", authHandler, adminRoleHandler, api.deleteAlertRule)
	alertGroup.Get("/active", authHandler, api.getActiveAlerts)
	alertGroup.Patch("/rules/:id/toggle", authHandler, adminRoleHandler, api.toggleAlertRule)

	// 添加与前端兼容的告警规则路径
	monitoringGroup.Get("/rules", authHandler, api.getAlertRules)
	monitoringGroup.Post("/rules", authHandler, adminRoleHandler, api.createAlertRule)
	monitoringGroup.Put("/rules/:id", authHandler, adminRoleHandler, api.updateAlertRule)
	monitoringGroup.Delete("/rules/:id", authHandler, adminRoleHandler, api.deleteAlertRule)
	monitoringGroup.Put("/rules/:id/toggle", authHandler, adminRoleHandler, api.toggleAlertRule)

	// 通知器CRUD
	notifierGroup := monitoringGroup.Group("/notifiers")
	notifierGroup.Get("/", authHandler, api.getNotifiers)
	notifierGroup.Get("/:id", authHandler, api.getNotifier)
	notifierGroup.Post("/", authHandler, adminRoleHandler, api.createNotifier)
	notifierGroup.Put("/:id", authHandler, adminRoleHandler, api.updateNotifier)
	notifierGroup.Delete("/:id", authHandler, adminRoleHandler, api.deleteNotifier)
	notifierGroup.Post("/:id/test", authHandler, adminRoleHandler, api.testNotifier)

	// 注册服务监控相关路由
	if api.serviceAPI != nil {
		api.serviceAPI.RegisterRoutes(monitoringGroup, authHandler, adminRoleHandler)
	}

	// 应用服务相关路由
	monitoringGroup.Get("/apps", authHandler, api.handleGetAllServices)
	monitoringGroup.Get("/apps/:serviceName", authHandler, api.handleGetService)

	// 服务实例相关路由
	monitoringGroup.Get("/apps/:serviceName/instances", authHandler, api.handleGetServiceInstances)
	monitoringGroup.Get("/apps/:serviceName/instances/:instanceId", authHandler, api.handleGetServiceInstance)
	monitoringGroup.Get("/apps/:serviceName/instances/:instanceId/metrics", authHandler, api.handleGetServiceInstanceMetrics)

	// 服务接口相关路由
	monitoringGroup.Get("/apps/:serviceName/endpoints", authHandler, api.handleGetServiceEndpoints)
	monitoringGroup.Get("/apps/:serviceName/endpoints/:endpoint", authHandler, api.handleGetServiceEndpoint)
	monitoringGroup.Get("/apps/:serviceName/endpoints/:endpoint/metrics", authHandler, api.handleGetServiceEndpointMetrics)

	// 服务汇总指标相关路由
	monitoringGroup.Get("/apps/:serviceName/metrics/summary", authHandler, api.handleGetServiceMetricsSummary)
	monitoringGroup.Get("/apps/:serviceName/metrics/qps", authHandler, api.handleGetServiceQPSMetrics)
	monitoringGroup.Get("/apps/:serviceName/metrics/error-rate", authHandler, api.handleGetServiceErrorRateMetrics)
	monitoringGroup.Get("/apps/:serviceName/metrics/response-time", authHandler, api.handleGetServiceResponseTimeMetrics)

	api.logger.Info("监控系统API路由已注册")
}

// handleHealthCheck 处理健康检查请求
func (api *API) handleHealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"time":   time.Now(),
	})
}

// handleMetricsQuery 处理指标查询请求
func (api *API) handleMetricsQuery(c *fiber.Ctx) error {
	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)

	// 获取指标名称
	metricNames := api.parseMetricNames(c.Query("metrics", ""))
	if len(metricNames) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "必须指定至少一个指标名称",
		})
	}

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		MetricNames:   metricNames,
		Limit:         limit,
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
		Aggregation:   c.Query("agg", "avg"),
		Interval:      c.Query("interval", "1m"),
	}

	// 执行查询
	result, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询指标失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// getServerMetrics 获取服务器指标
func (api *API) getServerMetrics(c *fiber.Ctx) error {
	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-24*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		Limit:         limit,
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
	}

	// 获取指标名称列表
	metricNames := api.parseMetricNames(c.Query("metrics", ""))
	if len(metricNames) == 0 {
		// 如果未指定指标，获取所有服务器指标
		for _, name := range api.serverMetricNames() {
			metricNames = append(metricNames, string(name))
		}
	}
	options.MetricNames = metricNames

	// 查询时间序列数据
	result, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询服务器指标失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// getServerMetricByName 根据名称获取特定服务器指标
func (api *API) getServerMetricByName(c *fiber.Ctx) error {
	name := c.Params("name")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-24*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		MetricNames:   []string{name},
		Limit:         limit,
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
	}

	// 查询时间序列数据
	result, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询服务器指标失败: %v", err))
	}

	// 如果没有找到该指标
	if len(result.Series) == 0 {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("未找到指定的指标: %s", name))
	}

	return utils.SuccessResponse(c, result.Series[0])
}

// getAppMetrics 获取应用指标
func (api *API) getAppMetrics(c *fiber.Ctx) error {
	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-24*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)
	detailed := c.Query("detailed", "false") == "true"

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		Limit:         limit,
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
	}

	// 添加应用过滤
	if source := c.Query("source"); source != "" {
		options.LabelMatchers["source"] = source
	}
	if instance := c.Query("instance"); instance != "" {
		options.LabelMatchers["instance"] = instance
	}

	if detailed {
		// 详细模式返回原始应用指标
		appMetrics, err := api.storage.QueryAppMetricsDetails(options)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询应用指标失败: %v", err))
		}
		return utils.SuccessResponse(c, appMetrics)
	} else {
		// 获取指标名称列表
		metricNames := api.parseMetricNames(c.Query("metrics", ""))
		if len(metricNames) == 0 {
			// 如果未指定指标，获取所有应用指标
			for _, name := range api.appMetricNames() {
				metricNames = append(metricNames, string(name))
			}
		}
		options.MetricNames = metricNames

		// 查询时间序列数据
		result, err := api.storage.QueryTimeSeries(options)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询应用指标失败: %v", err))
		}
		return utils.SuccessResponse(c, result)
	}
}

// getAppMetricByName 根据名称获取特定应用指标
func (api *API) getAppMetricByName(c *fiber.Ctx) error {
	name := c.Params("name")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-24*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		MetricNames:   []string{name},
		Limit:         limit,
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
	}

	// 查询时间序列数据
	result, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询应用指标失败: %v", err))
	}

	// 如果没有找到该指标
	if len(result.Series) == 0 {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("未找到指定的指标: %s", name))
	}

	return utils.SuccessResponse(c, result.Series[0])
}

// getAPICalls 获取API调用信息
func (api *API) getAPICalls(c *fiber.Ctx) error {
	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-24*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)
	detailed := c.Query("detailed", "false") == "true"

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		Limit:         limit,
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
	}

	// 添加应用过滤
	if source := c.Query("source"); source != "" {
		options.LabelMatchers["source"] = source
	}
	if instance := c.Query("instance"); instance != "" {
		options.LabelMatchers["instance"] = instance
	}

	if detailed {
		// 详细模式返回原始API调用信息
		calls, err := api.storage.QueryAPICallsDetails(options)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API调用失败: %v", err))
		}
		return utils.SuccessResponse(c, calls)
	} else {
		// 获取API指标
		metricNames := []string{
			string(models2.MetricAPIRequestCount),
			string(models2.MetricAPIRequestDuration),
			string(models2.MetricAPIRequestError),
			string(models2.MetricAPIRequestSize),
			string(models2.MetricAPIResponseSize),
		}
		options.MetricNames = metricNames

		// 查询时间序列数据
		result, err := api.storage.QueryTimeSeries(options)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API指标失败: %v", err))
		}
		return utils.SuccessResponse(c, result)
	}
}

// getAPICallsByEndpoint 获取特定端点的API调用信息
func (api *API) getAPICallsByEndpoint(c *fiber.Ctx) error {
	endpoint := c.Params("endpoint")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-24*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)

	// 构建查询选项，使用endpoint作为标签匹配条件
	options := models2.QueryOptions{
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
		LabelMatchers: map[string]string{
			"endpoint": endpoint,
		},
	}

	// 解析其他标签
	for k, v := range api.parseLabels(c.Query("labels", "")) {
		options.LabelMatchers[k] = v
	}

	// 查询API调用详情
	calls, err := api.storage.QueryAPICallsDetails(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API调用失败: %v", err))
	}

	return utils.SuccessResponse(c, calls)
}

// 告警相关处理函数

// getAlertRules 获取所有告警规则
func (api *API) getAlertRules(c *fiber.Ctx) error {
	rules := api.alertMgr.GetRules()
	return utils.SuccessResponse(c, rules)
}

// getAlertRule 获取特定告警规则
func (api *API) getAlertRule(c *fiber.Ctx) error {
	id := c.Params("id")
	rule, err := api.alertMgr.GetRule(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取告警规则失败: %v", err))
	}
	return utils.SuccessResponse(c, rule)
}

// createAlertRule 创建告警规则
func (api *API) createAlertRule(c *fiber.Ctx) error {
	rule := new(models2.AlertRule)
	if err := c.BodyParser(rule); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	if err := api.alertMgr.CreateRule(rule); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("创建告警规则失败: %v", err))
	}

	return utils.SuccessResponse(c, rule)
}

// updateAlertRule 更新告警规则
func (api *API) updateAlertRule(c *fiber.Ctx) error {
	id := c.Params("id")
	rule := new(models2.AlertRule)
	if err := c.BodyParser(rule); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 确保ID匹配
	rule.ID = id

	if err := api.alertMgr.UpdateRule(rule); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("更新告警规则失败: %v", err))
	}

	return utils.SuccessResponse(c, rule)
}

// deleteAlertRule 删除告警规则
func (api *API) deleteAlertRule(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := api.alertMgr.DeleteRule(id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("删除告警规则失败: %v", err))
	}

	return utils.SuccessResponse(c, true)
}

// toggleAlertRule 切换告警规则的启用状态
func (api *API) toggleAlertRule(c *fiber.Ctx) error {
	id := c.Params("id")

	// 获取规则
	rule, err := api.alertMgr.GetRule(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取告警规则失败: %v", err))
	}

	// 解析请求体
	request := struct {
		Enabled bool `json:"enabled"`
	}{}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 更新规则状态
	rule.Enabled = request.Enabled

	// 保存规则
	if err := api.alertMgr.UpdateRule(rule); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("更新告警规则状态失败: %v", err))
	}

	return utils.SuccessResponse(c, rule)
}

// getActiveAlerts 获取活动告警
func (api *API) getActiveAlerts(c *fiber.Ctx) error {
	alerts := api.alertMgr.GetActiveAlerts()
	return utils.SuccessResponse(c, alerts)
}

// getSystemOverview 获取系统概览信息
func (api *API) getSystemOverview(c *fiber.Ctx) error {
	// 查询所需的指标
	now := time.Now()
	startTime := now.Add(-1 * time.Minute)

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime: startTime,
		EndTime:   now,
		Limit:     1,
	}

	// 创建前端期望的扁平响应结构
	response := fiber.Map{
		"cpu_usage":    0.0,                      // CPU使用率（百分比）
		"memory_usage": 0.0,                      // 内存使用率（百分比）
		"disk_usage":   0.0,                      // 磁盘使用率（百分比）
		"load_average": []float64{0.0, 0.0, 0.0}, // 1分钟、5分钟、15分钟负载
		"uptime":       0,                        // 系统运行时间（秒）
		"boot_time":    now.Unix() - 3600,        // 系统启动时间（Unix时间戳）
	}

	// 安全封装查询逻辑，避免任何可能的错误
	safeQuery := func(metricName string) (float64, error) {
		options.MetricNames = []string{metricName}
		result, err := api.storage.QueryTimeSeries(options)
		if err != nil {
			return 0.0, err
		}

		if len(result.Series) == 0 {
			return 0.0, fmt.Errorf("没有找到指标：%s", metricName)
		}

		if len(result.Series[0].Points) == 0 {
			return 0.0, fmt.Errorf("指标没有数据点：%s", metricName)
		}

		return result.Series[0].Points[0].Value, nil
	}

	// 查询CPU使用率
	if cpuUsage, err := safeQuery(string(models2.MetricCPUUsage)); err == nil {
		response["cpu_usage"] = cpuUsage
	} else {
		api.logger.Warn("获取CPU使用率失败", zap.Error(err))
	}

	// 查询内存使用率
	if memUsage, err := safeQuery(string(models2.MetricMemoryUsedPercent)); err == nil {
		response["memory_usage"] = memUsage
	} else {
		api.logger.Warn("获取内存使用率失败", zap.Error(err))
	}

	// 查询磁盘使用率
	if diskUsage, err := safeQuery(string(models2.MetricDiskUsedPercent)); err == nil {
		response["disk_usage"] = diskUsage
	} else {
		api.logger.Warn("获取磁盘使用率失败", zap.Error(err))
	}

	// 安全封装负载查询逻辑
	loadAvg := []float64{0.0, 0.0, 0.0}

	// 查询负载平均值 - 分开查询以提高可靠性
	if load1, err := safeQuery(string(models2.MetricCPULoad1)); err == nil {
		loadAvg[0] = load1
	} else {
		api.logger.Warn("获取Load1失败", zap.Error(err))
	}

	if load5, err := safeQuery(string(models2.MetricCPULoad5)); err == nil {
		loadAvg[1] = load5
	} else {
		api.logger.Warn("获取Load5失败", zap.Error(err))
	}

	if load15, err := safeQuery(string(models2.MetricCPULoad15)); err == nil {
		loadAvg[2] = load15
	} else {
		api.logger.Warn("获取Load15失败", zap.Error(err))
	}

	response["load_average"] = loadAvg

	// 返回前端期望格式的响应
	return utils.SuccessResponse(c, response)
}

// getMetricsByName 直接通过指标名称获取数据
func (api *API) getMetricsByName(c *fiber.Ctx) error {
	// 获取指标名称
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须指定指标名称")
	}

	// 记录详细的请求日志，帮助调试
	api.logger.Info("接收到指标查询请求",
		zap.String("metrics", name),
		zap.String("start", c.Query("start", "")),
		zap.String("step", c.Query("step", "")),
		zap.String("labels", c.Query("labels", "")))

	// 支持多个指标（以逗号分隔）
	metricNames := strings.Split(name, ",")

	// 添加安全检查: 确保所有指标名称有效
	validatedNames := make([]string, 0, len(metricNames))
	for _, metricName := range metricNames {
		if metricName = strings.TrimSpace(metricName); metricName != "" {
			validatedNames = append(validatedNames, metricName)
		}
	}

	if len(validatedNames) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须提供至少一个有效的指标名称")
	}

	// 使用验证后的指标名称
	metricNames = validatedNames

	// 解析查询参数，添加错误处理以防止NaN值
	startParam := c.Query("start", "")
	endParam := c.Query("end", "")
	stepParam := c.Query("step", "60s") // 默认步长60秒

	// 设置默认值
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	// 尝试解析开始时间
	if startParam != "" && startParam != "NaN" {
		// 尝试解析数字时间戳（秒）
		if startSec, err := strconv.ParseInt(startParam, 10, 64); err == nil {
			startTime = time.Unix(startSec, 0)
		} else {
			// 尝试使用parseTimeParam解析
			startTime = api.parseTimeParam(startParam, startTime)
		}
	}

	// 尝试解析结束时间
	if endParam != "" && endParam != "NaN" {
		// 尝试解析数字时间戳（秒）
		if endSec, err := strconv.ParseInt(endParam, 10, 64); err == nil {
			endTime = time.Unix(endSec, 0)
		} else {
			// 尝试使用parseTimeParam解析
			endTime = api.parseTimeParam(endParam, endTime)
		}
	}

	// 处理步长参数
	if stepParam == "NaN" {
		stepParam = "60s" // 使用默认值替代NaN
	}

	// 解析可选的聚合方法
	aggregation := c.Query("agg", "avg")

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:     startTime,
		EndTime:       endTime,
		MetricNames:   metricNames,
		Limit:         1000, // 设置较大的限制
		LabelMatchers: api.parseLabels(c.Query("labels", "")),
		Aggregation:   aggregation,
		Interval:      stepParam,
	}

	// 添加特殊处理网络指标的逻辑
	containsNetworkMetrics := false
	for _, metricName := range metricNames {
		if strings.HasPrefix(metricName, "network.") {
			containsNetworkMetrics = true
			break
		}
	}

	// 如果包含网络指标，单独处理每个指标以避免批量请求可能导致的索引越界
	if containsNetworkMetrics {
		api.logger.Info("检测到网络指标查询，使用安全处理模式")
		result := &models2.QueryResult{
			Series: []models2.TimeSeries{},
		}

		// 逐个处理指标查询
		for _, metricName := range metricNames {
			singleOptions := options
			singleOptions.MetricNames = []string{metricName}

			singleResult, err := api.storage.QueryTimeSeries(singleOptions)
			if err != nil {
				api.logger.Warn("查询单个指标失败",
					zap.String("metric", metricName),
					zap.Error(err))
				continue
			}

			// 如果有数据，添加到结果集
			if len(singleResult.Series) > 0 {
				result.Series = append(result.Series, singleResult.Series...)
			}
		}

		return utils.SuccessResponse(c, result)
	}

	// 添加错误处理并尝试使用默认响应
	result, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		api.logger.Error("查询指标数据失败",
			zap.Strings("metrics", metricNames),
			zap.Error(err))

		// 返回空结果集而非错误，以便前端仍能正常显示
		result = &models2.QueryResult{
			Series: []models2.TimeSeries{},
		}
	}

	return utils.SuccessResponse(c, result)
}

// 通知器相关处理函数

// getNotifiers 获取所有通知器
func (api *API) getNotifiers(c *fiber.Ctx) error {
	notifiers := api.notifierMgr.GetNotifiers()
	return utils.SuccessResponse(c, notifiers)
}

// getNotifier 获取特定通知器
func (api *API) getNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	notifier, err := api.notifierMgr.GetNotifier(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取通知器失败: %v", err))
	}
	return utils.SuccessResponse(c, notifier)
}

// createNotifier 创建通知器
func (api *API) createNotifier(c *fiber.Ctx) error {
	notifier := new(models2.Notifier)
	if err := c.BodyParser(notifier); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 检查ID是否为空，如果为空则生成UUID
	if notifier.ID == "" {
		// 导入内部UUID生成方法
		notifier.ID = api.generateNotifierID(notifier.Name, string(notifier.Type))
	}

	if err := api.notifierMgr.CreateNotifier(notifier); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("创建通知器失败: %v", err))
	}

	return utils.SuccessResponse(c, notifier)
}

// updateNotifier 更新通知器
func (api *API) updateNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	notifier := new(models2.Notifier)
	if err := c.BodyParser(notifier); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 确保ID匹配
	notifier.ID = id

	if err := api.notifierMgr.UpdateNotifier(notifier); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("更新通知器失败: %v", err))
	}

	return utils.SuccessResponse(c, notifier)
}

// deleteNotifier 删除通知器
func (api *API) deleteNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := api.notifierMgr.DeleteNotifier(id); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("删除通知器失败: %v", err))
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// testNotifier 测试通知器
func (api *API) testNotifier(c *fiber.Ctx) error {
	id := c.Params("id")

	// 获取通知器，仅检查存在性
	_, err := api.notifierMgr.GetNotifier(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("通知器不存在: %v", err))
	}

	// 创建测试告警
	now := time.Now()
	testAlert := &models2.Alert{
		ID:          "test-alert",
		RuleID:      "test-rule",
		RuleName:    "测试告警规则",
		TargetType:  models2.AlertTargetServer,
		Metric:      "cpu.usage",
		Labels:      map[string]string{"host": "test-server"},
		Value:       95.5,
		Threshold:   90.0,
		Condition:   models2.ConditionGreaterThan,
		Severity:    models2.AlertSeverityWarning,
		State:       models2.AlertStateFiring,
		StartsAt:    now,
		Description: "这是一个测试告警，用于验证通知器配置是否正确",
		UpdatedAt:   now,
	}

	// 发送测试告警
	results := api.notifierMgr.SendAlert(testAlert, "test")

	return utils.SuccessResponse(c, fiber.Map{
		"message": "测试通知已发送",
		"results": results,
	})
}

// 辅助方法

// parseTimeParam 解析时间参数
func (api *API) parseTimeParam(value string, defaultValue time.Time) time.Time {
	if value == "" {
		return defaultValue
	}

	// 尝试解析标准时间格式
	t, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return t
	}

	// 尝试解析相对时间（如 -1h, -30m）
	if strings.HasPrefix(value, "-") {
		duration, err := time.ParseDuration(value)
		if err == nil {
			return time.Now().Add(duration)
		}
	}

	// 无法解析，返回默认值
	return defaultValue
}

// parseIntParam 解析整数参数
func (api *API) parseIntParam(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

// parseLabels 解析标签参数
func (api *API) parseLabels(labelsString string) map[string]string {
	result := make(map[string]string)
	if labelsString == "" {
		return result
	}

	pairs := strings.Split(labelsString, ",")
	for _, pair := range pairs {
		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) == 2 {
			result[keyValue[0]] = keyValue[1]
		}
	}

	return result
}

// parseMetricNames 解析指标名称列表
func (api *API) parseMetricNames(metricsString string) []string {
	if metricsString == "" {
		return []string{}
	}

	return strings.Split(metricsString, ",")
}

// serverMetricNames 获取服务器指标名称列表
func (api *API) serverMetricNames() []models2.ServerMetricName {
	return []models2.ServerMetricName{
		models2.MetricCPUUsage,
		models2.MetricCPUUsageUser,
		models2.MetricCPUUsageSystem,
		models2.MetricCPUUsageIdle,
		models2.MetricCPUUsageIOWait,
		models2.MetricCPULoad1,
		models2.MetricCPULoad5,
		models2.MetricCPULoad15,
		models2.MetricMemoryTotal,
		models2.MetricMemoryUsed,
		models2.MetricMemoryFree,
		models2.MetricMemoryBuffers,
		models2.MetricMemoryCache,
		models2.MetricMemoryUsedPercent,
		models2.MetricDiskTotal,
		models2.MetricDiskUsed,
		models2.MetricDiskFree,
		models2.MetricDiskUsedPercent,
		models2.MetricDiskIORead,
		models2.MetricDiskIOWrite,
		models2.MetricDiskIOReadBytes,
		models2.MetricDiskIOWriteBytes,
		models2.MetricNetworkIn,
		models2.MetricNetworkOut,
		models2.MetricNetworkInPackets,
		models2.MetricNetworkOutPackets,
	}
}

// appMetricNames 获取应用指标名称列表
func (api *API) appMetricNames() []models2.ApplicationMetricName {
	return []models2.ApplicationMetricName{
		models2.MetricAPIRequestCount,
		models2.MetricAPIRequestDuration,
		models2.MetricAPIRequestError,
		models2.MetricAPIRequestSize,
		models2.MetricAPIResponseSize,
		models2.MetricAppMemoryUsed,
		models2.MetricAppMemoryTotal,
		models2.MetricAppMemoryHeap,
		models2.MetricAppMemoryNonHeap,
		models2.MetricAppGCCount,
		models2.MetricAppGCTime,
		models2.MetricAppThreadTotal,
		models2.MetricAppThreadActive,
		models2.MetricAppThreadBlocked,
		models2.MetricAppThreadWaiting,
		models2.MetricAppCPUUsage,
		models2.MetricAppClassLoaded,
	}
}

// getAllMetricNames 获取所有可用的指标名称
func (api *API) getAllMetricNames(c *fiber.Ctx) error {
	result := fiber.Map{
		"server_metrics": api.serverMetricNames(),
		"app_metrics":    api.appMetricNames(),
	}
	return utils.SuccessResponse(c, result)
}

// generateNotifierID 生成通知器ID
func (api *API) generateNotifierID(name string, typeName string) string {
	// 生成一个简短的UUID（前8位），然后添加类型前缀和名称的前5个字符（如果有）
	uuid := make([]byte, 16)
	io.ReadFull(rand.Reader, uuid)
	uuid[6] = (uuid[6] & 0x0F) | 0x40 // 版本 4
	uuid[8] = (uuid[8] & 0x3F) | 0x80 // 变体 RFC4122

	shortUUID := fmt.Sprintf("%x", uuid[:4])

	// 使用类型的第一个字母作为前缀
	typePrefix := ""
	if len(typeName) > 0 {
		typePrefix = string(typeName[0])
	}

	// 名称的前5个字符（如果有）
	namePrefix := ""
	if len(name) > 5 {
		namePrefix = name[:5]
	} else {
		namePrefix = name
	}

	// 转为小写，移除空格
	namePrefix = strings.ToLower(strings.ReplaceAll(namePrefix, " ", ""))

	// 生成ID，格式：类型前缀-名称前缀-短UUID
	return fmt.Sprintf("%s-%s-%s", typePrefix, namePrefix, shortUUID)
}

// 服务监控API处理函数，将请求委托给ServiceAPI

// handleGetAllServices 处理获取所有服务列表的请求
func (api *API) handleGetAllServices(c *fiber.Ctx) error {
	// 解析查询参数
	options := models2.ServiceListOptions{
		Tag:        c.Query("tag"),
		Status:     c.Query("status"),
		SearchTerm: c.Query("search"),
		Limit:      api.parseIntParam(c.Query("limit"), 50),
		Offset:     api.parseIntParam(c.Query("offset"), 0),
	}

	// 从存储中获取服务列表
	services, err := api.storage.GetAllServices(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取服务列表失败: %v", err))
	}

	return utils.SuccessResponse(c, services)
}

// handleGetService 处理获取特定服务详情的请求
func (api *API) handleGetService(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务详情
	service, err := api.storage.GetService(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务不存在: %v", err))
	}

	return utils.SuccessResponse(c, service)
}

// handleGetServiceInstances 处理获取服务所有实例的请求
func (api *API) handleGetServiceInstances(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务实例列表
	instances, err := api.storage.GetServiceInstances(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务不存在: %v", err))
	}

	return utils.SuccessResponse(c, instances)
}

// handleGetServiceInstance 处理获取服务特定实例的请求
func (api *API) handleGetServiceInstance(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	instanceID := c.Params("instanceId")

	// 从存储中获取服务实例
	instance, err := api.storage.GetServiceInstance(serviceName, instanceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务实例不存在: %v", err))
	}

	return utils.SuccessResponse(c, instance)
}

// handleGetServiceInstanceMetrics 处理获取服务实例指标的请求
func (api *API) handleGetServiceInstanceMetrics(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	instanceID := c.Params("instanceId")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	interval := c.Query("interval", "1m")
	aggregation := c.Query("aggregation", "avg")

	// 构建查询选项
	options := models2.ServiceQueryOptions{
		ServiceName: serviceName,
		InstanceID:  instanceID,
		StartTime:   startTime,
		EndTime:     endTime,
		Interval:    interval,
		Aggregation: aggregation,
	}

	// 从存储中获取指标数据
	metrics, err := api.storage.QueryServiceMetrics(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询指标数据失败: %v", err))
	}

	return utils.SuccessResponse(c, metrics)
}

// handleGetServiceEndpoints 处理获取服务所有接口的请求
func (api *API) handleGetServiceEndpoints(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务接口列表
	endpoints, err := api.storage.GetServiceEndpoints(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务不存在: %v", err))
	}

	return utils.SuccessResponse(c, endpoints)
}

// handleGetServiceEndpoint 处理获取服务特定接口的请求
func (api *API) handleGetServiceEndpoint(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	endpoint := c.Params("endpoint")
	method := c.Query("method", "GET")

	// 从存储中获取服务接口
	endpointObj, err := api.storage.GetServiceEndpoint(serviceName, endpoint, method)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务接口不存在: %v", err))
	}

	return utils.SuccessResponse(c, endpointObj)
}

// handleGetServiceEndpointMetrics 处理获取服务接口指标的请求
func (api *API) handleGetServiceEndpointMetrics(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	endpoint := c.Params("endpoint")
	method := c.Query("method", "GET")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	interval := c.Query("interval", "1m")
	aggregation := c.Query("aggregation", "avg")

	// 构建查询选项
	options := models2.ServiceQueryOptions{
		ServiceName: serviceName,
		Endpoint:    endpoint,
		Method:      method,
		StartTime:   startTime,
		EndTime:     endTime,
		Interval:    interval,
		Aggregation: aggregation,
	}

	// 从存储中获取指标数据
	metrics, err := api.storage.QueryServiceMetrics(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询指标数据失败: %v", err))
	}

	return utils.SuccessResponse(c, metrics)
}

// handleGetServiceMetricsSummary 处理获取服务汇总指标的请求
func (api *API) handleGetServiceMetricsSummary(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务汇总指标
	summary, err := api.storage.QueryServiceSummary(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询服务汇总指标失败: %v", err))
	}

	return utils.SuccessResponse(c, summary)
}

// handleGetServiceQPSMetrics 处理获取服务QPS指标的请求
func (api *API) handleGetServiceQPSMetrics(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	interval := c.Query("interval", "1m")

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		MetricNames: []string{string(models2.MetricAPIRequestCount)},
		LabelMatchers: map[string]string{
			"source": serviceName,
		},
		Aggregation: "rate",
		Interval:    interval,
	}

	// 从存储中获取指标数据
	metrics, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询QPS指标失败: %v", err))
	}

	return utils.SuccessResponse(c, metrics)
}

// handleGetServiceErrorRateMetrics 处理获取服务错误率指标的请求
func (api *API) handleGetServiceErrorRateMetrics(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	interval := c.Query("interval", "1m")

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		MetricNames: []string{string(models2.MetricAPIRequestError)},
		LabelMatchers: map[string]string{
			"source": serviceName,
		},
		Aggregation: "rate",
		Interval:    interval,
	}

	// 从存储中获取指标数据
	metrics, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询错误率指标失败: %v", err))
	}

	return utils.SuccessResponse(c, metrics)
}

// handleGetServiceResponseTimeMetrics 处理获取服务响应时间指标的请求
func (api *API) handleGetServiceResponseTimeMetrics(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 解析查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	interval := c.Query("interval", "1m")

	// 构建查询选项
	options := models2.QueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		MetricNames: []string{string(models2.MetricAPIRequestDuration)},
		LabelMatchers: map[string]string{
			"source": serviceName,
		},
		Aggregation: "avg",
		Interval:    interval,
	}

	// 从存储中获取指标数据
	metrics, err := api.storage.QueryTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询响应时间指标失败: %v", err))
	}

	return utils.SuccessResponse(c, metrics)
}
