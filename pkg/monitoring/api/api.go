// Package api 提供监控系统的HTTP API接口
package api

import (
	"crypto/rand"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	monitoringv1 "github.com/xsxdot/aio/api/proto/monitoring/v1"
	"github.com/xsxdot/aio/pkg/monitoring/alerting"
	"github.com/xsxdot/aio/pkg/monitoring/collector"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"github.com/xsxdot/aio/pkg/notifier"

	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"go.uber.org/zap"
)

// API 监控系统的HTTP API
type API struct {
	storage     *storage.Storage
	grpcStorage *storage.GrpcStorage
	alertMgr    *alerting.Manager
	notifierMgr *notifier.Manager
	port        int
	logger      *zap.Logger
}

// NewAPI 创建新的监控系统API
func NewAPI(port int, storage *storage.Storage, grpcStorage *storage.GrpcStorage, alertMgr *alerting.Manager, notifierMgr *notifier.Manager, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	api := &API{
		port:        port,
		storage:     storage,
		grpcStorage: grpcStorage,
		alertMgr:    alertMgr,
		notifierMgr: notifierMgr,
		logger:      logger,
	}

	return api
}

// RegisterRoutes 注册所有API路由
func (api *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建API组
	monitoringGroup := router.Group("/monitoring")

	// 为整个监控组添加转发中间件
	monitoringGroup.Use(api.nodeForwardMiddleware())

	// 系统概览
	monitoringGroup.Get("/system/overview", authHandler, api.getSystemOverview)

	// 通用指标路径，支持直接查询指标
	monitoringGroup.Get("/metrics/:name", authHandler, api.getMetricsByName)

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

	// API监控相关路由
	apiMonitoringGroup := monitoringGroup.Group("/service")
	apiMonitoringGroup.Use(api.serviceForwardMiddleware())
	apiMonitoringGroup.Get("/:serviceName/api_endpoints", authHandler, api.getServiceAPIEndpoints)
	apiMonitoringGroup.Post("/:serviceName/api_summary", authHandler, api.getServiceAPISummary)
	apiMonitoringGroup.Get("/metrics/:name", authHandler, api.getMetricsByName)

	api.logger.Info("监控系统API路由已注册")
}

// nodeForwardMiddleware 节点转发中间件
func (api *API) nodeForwardMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 检查是否需要转发到其他节点
		targetIP := c.Query("target_ip")
		targetPortStr := c.Query("target_port")

		if targetIP != "" {
			var targetURL string

			// 检查 targetIP 是否已经包含端口号
			if strings.Contains(targetIP, ":") {
				// 如果已经包含端口号，直接使用
				targetURL = fmt.Sprintf("http://%s", targetIP)
			} else if targetPortStr != "" {
				// 如果没有端口号但提供了端口参数，进行拼接
				targetPort, err := strconv.Atoi(targetPortStr)
				if err != nil {
					return utils.FailResponse(c, utils.StatusBadRequest, "无效的目标端口")
				}
				targetURL = fmt.Sprintf("http://%s:%d", targetIP, targetPort)
			} else {
				// 既没有端口号也没有端口参数，使用默认端口
				targetURL = fmt.Sprintf("http://%s", targetIP)
			}

			// 构建目标URL，需要移除转发参数以避免死循环

			// 构建新的查询参数，排除转发相关参数
			newQuery := make(map[string]string)
			c.Context().QueryArgs().VisitAll(func(key, value []byte) {
				keyStr := string(key)
				if keyStr != "target_ip" && keyStr != "target_port" {
					newQuery[keyStr] = string(value)
				}
			})

			// 构建新的URL路径
			newPath := c.Path()
			if len(newQuery) > 0 {
				queryParams := make([]string, 0, len(newQuery))
				for k, v := range newQuery {
					queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
				}
				newPath += "?" + strings.Join(queryParams, "&")
			}

			// 解析端口号用于日志记录
			targetPort := 0
			if targetPortStr != "" {
				if port, err := strconv.Atoi(targetPortStr); err == nil {
					targetPort = port
				}
			}

			api.logger.Info("转发监控请求",
				zap.String("target_ip", targetIP),
				zap.Int("target_port", targetPort),
				zap.String("path", c.Path()),
				zap.String("new_path", newPath))

			// 使用fiber的proxy中间件进行转发
			return proxy.Do(c, targetURL+newPath)
		}

		// 如果不需要转发，继续到下一个中间件
		return c.Next()
	}
}

// serviceForwardMiddleware 服务转发中间件，根据serviceName转发到正确的存储节点
func (api *API) serviceForwardMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 获取服务名称
		serviceName := c.Params("serviceName")
		if serviceName == "" {
			// 如果没有服务名称，继续到下一个中间件
			return c.Next()
		}

		// 检查是否有显式的目标节点参数（用于调试或手动指定）
		targetIP := c.Query("target_ip")
		targetPortStr := c.Query("target_port")

		// 如果没有显式指定目标节点，通过grpcStorage获取服务的存储节点
		if targetIP == "" && api.grpcStorage != nil {
			ctx := c.Context()

			// 获取服务的存储节点分配
			allocation, err := api.grpcStorage.GetStorageNode(ctx, &monitoringv1.GetStorageNodeRequest{
				ServiceName:   serviceName,
				ForceReassign: false, // 不强制重新分配
			})

			if err != nil {
				api.logger.Warn("获取服务存储节点失败，使用本地节点",
					zap.String("service_name", serviceName),
					zap.Error(err))
				// 如果获取失败，继续使用本地节点处理
				return c.Next()
			}

			if allocation != nil && allocation.Node != nil && allocation.Success {
				// 从gRPC地址中提取IP地址，去掉gRPC端口
				nodeAddress := allocation.Node.Address
				var nodeIP string

				// 检查地址是否包含端口号
				if strings.Contains(nodeAddress, ":") {
					// 分离IP和端口
					parts := strings.Split(nodeAddress, ":")
					nodeIP = parts[0]
				} else {
					// 没有端口号，直接使用地址
					nodeIP = nodeAddress
				}

				// 使用API端口构建目标地址
				targetIP = fmt.Sprintf("%s:%d", nodeIP, api.port)

				api.logger.Debug("获取到服务存储节点",
					zap.String("service_name", serviceName),
					zap.String("grpc_address", nodeAddress),
					zap.String("api_address", targetIP))
			}
		}

		// 如果有目标IP，进行转发
		if targetIP != "" {
			var targetURL string

			// 检查 targetIP 是否已经包含端口号
			if strings.Contains(targetIP, ":") {
				// 如果已经包含端口号，直接使用
				targetURL = fmt.Sprintf("http://%s", targetIP)
			} else if targetPortStr != "" {
				// 如果没有端口号但提供了端口参数，进行拼接
				targetPort, err := strconv.Atoi(targetPortStr)
				if err != nil {
					return utils.FailResponse(c, utils.StatusBadRequest, "无效的目标端口")
				}
				targetURL = fmt.Sprintf("http://%s:%d", targetIP, targetPort)
			} else {
				// 既没有端口号也没有端口参数，使用默认端口
				targetURL = fmt.Sprintf("http://%s", targetIP)
			}

			// 构建新的查询参数，排除转发相关参数
			newQuery := make(map[string]string)
			c.Context().QueryArgs().VisitAll(func(key, value []byte) {
				keyStr := string(key)
				if keyStr != "target_ip" && keyStr != "target_port" {
					newQuery[keyStr] = string(value)
				}
			})

			// 构建新的URL路径
			newPath := c.Path()
			if len(newQuery) > 0 {
				queryParams := make([]string, 0, len(newQuery))
				for k, v := range newQuery {
					queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
				}
				newPath += "?" + strings.Join(queryParams, "&")
			}

			api.logger.Info("转发服务监控请求",
				zap.String("service_name", serviceName),
				zap.String("target_url", targetURL),
				zap.String("path", c.Path()),
				zap.String("new_path", newPath))

			// 使用fiber的proxy中间件进行转发
			return proxy.Do(c, targetURL+newPath)
		}

		// 如果不需要转发，继续到下一个中间件
		return c.Next()
	}
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
	rule := new(models.AlertRule)
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
	rule := new(models.AlertRule)
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
	query := storage.MetricQuery{
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
		query.MetricNames = []string{metricName}
		result, err := api.storage.QueryTimeSeries(query)
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
	if cpuUsage, err := safeQuery(string(collector.MetricCPUUsage)); err == nil {
		response["cpu_usage"] = cpuUsage
	} else {
		api.logger.Warn("获取CPU使用率失败", zap.Error(err))
	}

	// 查询内存使用率
	if memUsage, err := safeQuery(string(collector.MetricMemoryUsedPercent)); err == nil {
		response["memory_usage"] = memUsage
	} else {
		api.logger.Warn("获取内存使用率失败", zap.Error(err))
	}

	// 查询磁盘使用率
	if diskUsage, err := safeQuery(string(collector.MetricDiskUsedPercent)); err == nil {
		response["disk_usage"] = diskUsage
	} else {
		api.logger.Warn("获取磁盘使用率失败", zap.Error(err))
	}

	// 安全封装负载查询逻辑
	loadAvg := []float64{0.0, 0.0, 0.0}

	// 查询负载平均值 - 分开查询以提高可靠性
	if load1, err := safeQuery(string(collector.MetricCPULoad1)); err == nil {
		loadAvg[0] = load1
	} else {
		api.logger.Warn("获取Load1失败", zap.Error(err))
	}

	if load5, err := safeQuery(string(collector.MetricCPULoad5)); err == nil {
		loadAvg[1] = load5
	} else {
		api.logger.Warn("获取Load5失败", zap.Error(err))
	}

	if load15, err := safeQuery(string(collector.MetricCPULoad15)); err == nil {
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
	query := storage.MetricQuery{
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
		result := &models.QueryResult{
			Series: []models.TimeSeries{},
		}

		// 逐个处理指标查询
		for _, metricName := range metricNames {
			singleQuery := query
			singleQuery.MetricNames = []string{metricName}

			singleResult, err := api.storage.QueryTimeSeries(singleQuery)
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
	result, err := api.storage.QueryTimeSeries(query)
	if err != nil {
		api.logger.Error("查询指标数据失败",
			zap.Strings("metrics", metricNames),
			zap.Error(err))

		// 返回空结果集而非错误，以便前端仍能正常显示
		result = &models.QueryResult{
			Series: []models.TimeSeries{},
		}
	}

	return utils.SuccessResponse(c, result)
}

// 通知器相关处理函数

// getNotifiers 获取所有通知器
func (api *API) getNotifiers(c *fiber.Ctx) error {
	configs := api.notifierMgr.GetNotifiers()

	// 转换为前端期望的格式
	result := make([]*models.Notifier, 0, len(configs))
	for _, config := range configs {
		result = append(result, convertNotifierConfigToModel(config))
	}

	return utils.SuccessResponse(c, result)
}

// getNotifier 获取特定通知器
func (api *API) getNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	config, err := api.notifierMgr.GetNotifier(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取通知器失败: %v", err))
	}

	// 转换为前端期望的格式
	result := convertNotifierConfigToModel(config)
	return utils.SuccessResponse(c, result)
}

// createNotifier 创建通知器
func (api *API) createNotifier(c *fiber.Ctx) error {
	notifier := new(models.Notifier)
	if err := c.BodyParser(notifier); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 检查ID是否为空，如果为空则生成UUID
	if notifier.ID == "" {
		// 导入内部UUID生成方法
		notifier.ID = api.generateNotifierID(notifier.Name, string(notifier.Type))
	}

	// 转换为新的配置格式
	config := convertModelToNotifierConfig(notifier)
	if err := api.notifierMgr.CreateNotifier(config); err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("创建通知器失败: %v", err))
	}

	return utils.SuccessResponse(c, notifier)
}

// updateNotifier 更新通知器
func (api *API) updateNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	notifier := new(models.Notifier)
	if err := c.BodyParser(notifier); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 确保ID匹配
	notifier.ID = id

	// 转换为新的配置格式
	config := convertModelToNotifierConfig(notifier)
	if err := api.notifierMgr.UpdateNotifier(config); err != nil {
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
	testAlert := &models.Alert{
		ID:          "test-alert",
		RuleID:      "test-rule",
		RuleName:    "测试告警规则",
		TargetType:  models.AlertTargetServer,
		Metric:      "cpu.usage",
		Labels:      map[string]string{"host": "test-server"},
		Value:       95.5,
		Threshold:   90.0,
		Condition:   models.ConditionGreaterThan,
		Severity:    models.AlertSeverityWarning,
		State:       models.AlertStateFiring,
		StartsAt:    now,
		Description: "这是一个测试告警，用于验证通知器配置是否正确",
		UpdatedAt:   now,
	}

	// 创建通知消息
	notification := &notifier.Notification{
		ID:        "test-notification",
		Title:     "测试通知",
		Content:   fmt.Sprintf("这是一个测试通知，告警规则：%s，指标值：%.2f，阈值：%.2f", testAlert.RuleName, testAlert.Value, testAlert.Threshold),
		Level:     notifier.NotificationLevelWarning,
		CreatedAt: time.Now(),
		Labels:    testAlert.Labels,
		Data: map[string]interface{}{
			"alert_id":    testAlert.ID,
			"rule_name":   testAlert.RuleName,
			"metric":      testAlert.Metric,
			"description": testAlert.Description,
		},
	}

	// 发送测试通知
	results := api.notifierMgr.SendNotification(notification)

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

// ========================= API 监控相关结构体定义 =========================

// APIEndpoint 表示API端点信息
type APIEndpoint struct {
	Method string `json:"method"` // HTTP方法
	Path   string `json:"path"`   // API路径
}

// APIEndpointStats 表示API端点的统计信息
type APIEndpointStats struct {
	Method string   `json:"method"` // HTTP方法
	Path   string   `json:"path"`   // API路径
	Stats  APIStats `json:"stats"`  // 统计数据
}

// APIStats 表示API统计数据
type APIStats struct {
	RequestCount int64   `json:"request_count"`  // 请求总数
	ErrorCount   int64   `json:"error_count"`    // 错误数量
	ErrorRate    float64 `json:"error_rate"`     // 错误率
	AvgLatencyMs float64 `json:"avg_latency_ms"` // 平均延迟(毫秒)
	P95LatencyMs float64 `json:"p95_latency_ms"` // P95延迟(毫秒)
	P99LatencyMs float64 `json:"p99_latency_ms"` // P99延迟(毫秒)
	QPS          float64 `json:"qps"`            // 每秒请求数
}

// APIEndpointsRequest 表示获取API端点聚合统计的请求体
type APIEndpointsRequest struct {
	Endpoints []APIEndpoint `json:"endpoints"` // 端点列表
}

// ========================= API 监控处理函数 =========================

// getServiceAPIEndpoints 获取服务下的API端点列表
func (api *API) getServiceAPIEndpoints(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	if serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须指定服务名称")
	}

	// 解析查询参数
	instanceID := c.Query("instanceId", "")
	timeRange := c.Query("timeRange", "1h")

	// 解析时间范围
	duration, err := time.ParseDuration(timeRange)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的时间范围格式: %v", err))
	}

	// 设置查询时间
	now := time.Now()
	startTime := now.Add(-duration)

	// 构建查询条件
	query := storage.MetricQuery{
		StartTime:   startTime,
		EndTime:     now,
		MetricNames: []string{string(collector.MetricAPIRequestCount)},
		Limit:       10000, // 设置较大的限制以获取所有端点
		LabelMatchers: map[string]string{
			"service_name": serviceName,
		},
	}

	// 如果指定了实例ID，添加到查询条件
	if instanceID != "" {
		query.LabelMatchers["instance_id"] = instanceID
	}

	api.logger.Info("查询服务API端点",
		zap.String("service_name", serviceName),
		zap.String("instance_id", instanceID),
		zap.String("time_range", timeRange))

	// 查询指标数据
	result, err := api.storage.QueryTimeSeries(query)
	if err != nil {
		api.logger.Error("查询API端点数据失败",
			zap.String("service_name", serviceName),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API端点数据失败: %v", err))
	}

	// 提取唯一的端点组合
	endpointsMap := make(map[string]APIEndpoint)
	for _, series := range result.Series {
		if method, exists := series.Labels["method"]; exists {
			if path, exists := series.Labels["path"]; exists {
				key := fmt.Sprintf("%s:%s", method, path)
				endpointsMap[key] = APIEndpoint{
					Method: method,
					Path:   path,
				}
			}
		}
	}

	// 转换为数组
	endpoints := make([]APIEndpoint, 0, len(endpointsMap))
	for _, endpoint := range endpointsMap {
		endpoints = append(endpoints, endpoint)
	}

	api.logger.Info("获取API端点成功",
		zap.String("service_name", serviceName),
		zap.Int("endpoint_count", len(endpoints)))

	return utils.SuccessResponse(c, endpoints)
}

// getServiceAPISummary 获取API端点聚合统计数据
func (api *API) getServiceAPISummary(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	if serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须指定服务名称")
	}

	// 解析查询参数
	instanceID := c.Query("instanceId", "")
	startTimeParam := c.Query("startTime", "")
	endTimeParam := c.Query("endTime", "")

	// 解析请求体
	var request APIEndpointsRequest
	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	if len(request.Endpoints) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须提供至少一个API端点")
	}

	// 解析时间参数
	var startTime, endTime time.Time
	now := time.Now()

	if startTimeParam != "" {
		if startSec, err := strconv.ParseInt(startTimeParam, 10, 64); err == nil {
			startTime = time.Unix(startSec, 0)
		} else {
			startTime = api.parseTimeParam(startTimeParam, now.Add(-1*time.Hour))
		}
	} else {
		startTime = now.Add(-1 * time.Hour) // 默认最近1小时
	}

	if endTimeParam != "" {
		if endSec, err := strconv.ParseInt(endTimeParam, 10, 64); err == nil {
			endTime = time.Unix(endSec, 0)
		} else {
			endTime = api.parseTimeParam(endTimeParam, now)
		}
	} else {
		endTime = now
	}

	api.logger.Info("查询API端点聚合统计",
		zap.String("service_name", serviceName),
		zap.String("instance_id", instanceID),
		zap.Time("start_time", startTime),
		zap.Time("end_time", endTime),
		zap.Int("endpoint_count", len(request.Endpoints)))

	// 为每个端点计算统计数据
	results := make([]APIEndpointStats, 0, len(request.Endpoints))

	for _, endpoint := range request.Endpoints {
		stats, err := api.calculateAPIStats(serviceName, instanceID, endpoint, startTime, endTime)
		if err != nil {
			api.logger.Warn("计算API端点统计失败",
				zap.String("service_name", serviceName),
				zap.String("method", endpoint.Method),
				zap.String("path", endpoint.Path),
				zap.Error(err))
			// 使用空统计数据而非返回错误
			stats = APIStats{}
		}

		results = append(results, APIEndpointStats{
			Method: endpoint.Method,
			Path:   endpoint.Path,
			Stats:  stats,
		})
	}

	api.logger.Info("API端点聚合统计计算完成",
		zap.String("service_name", serviceName),
		zap.Int("result_count", len(results)))

	return utils.SuccessResponse(c, results)
}

// calculateAPIStats 计算单个API端点的统计数据
func (api *API) calculateAPIStats(serviceName, instanceID string, endpoint APIEndpoint, startTime, endTime time.Time) (APIStats, error) {
	// 构建基础标签匹配器
	labelMatchers := map[string]string{
		"service_name": serviceName,
		"method":       endpoint.Method,
		"path":         endpoint.Path,
	}

	if instanceID != "" {
		labelMatchers["instance_id"] = instanceID
	}

	// 基础查询配置
	baseQuery := storage.MetricQuery{
		StartTime:     startTime,
		EndTime:       endTime,
		LabelMatchers: labelMatchers,
		Limit:         10000,
	}

	stats := APIStats{}

	// 计算时间间隔（秒）
	duration := endTime.Sub(startTime).Seconds()

	// 1. 查询请求总数
	requestCountQuery := baseQuery
	requestCountQuery.MetricNames = []string{string(collector.MetricAPIRequestCount)}
	requestCountQuery.Aggregation = "sum"

	if requestResult, err := api.storage.QueryTimeSeries(requestCountQuery); err == nil {
		totalRequests := float64(0)
		for _, series := range requestResult.Series {
			for _, point := range series.Points {
				totalRequests += point.Value
			}
		}
		stats.RequestCount = int64(totalRequests)

		// 计算QPS
		if duration > 0 {
			stats.QPS = totalRequests / duration
		}
	}

	// 2. 查询错误数量
	errorCountQuery := baseQuery
	errorCountQuery.MetricNames = []string{string(collector.MetricAPIErrorCount)}
	errorCountQuery.Aggregation = "sum"

	if errorResult, err := api.storage.QueryTimeSeries(errorCountQuery); err == nil {
		totalErrors := float64(0)
		for _, series := range errorResult.Series {
			for _, point := range series.Points {
				totalErrors += point.Value
			}
		}
		stats.ErrorCount = int64(totalErrors)

		// 计算错误率
		if stats.RequestCount > 0 {
			stats.ErrorRate = float64(stats.ErrorCount) / float64(stats.RequestCount)
		}
	}

	// 3. 查询延迟数据
	latencyQuery := baseQuery
	latencyQuery.MetricNames = []string{string(collector.MetricAPIRequestDuration)}

	if latencyResult, err := api.storage.QueryTimeSeries(latencyQuery); err == nil {
		latencies := make([]float64, 0)
		totalLatency := float64(0)
		count := 0

		for _, series := range latencyResult.Series {
			for _, point := range series.Points {
				latencies = append(latencies, point.Value)
				totalLatency += point.Value
				count++
			}
		}

		if count > 0 {
			// 计算平均延迟
			stats.AvgLatencyMs = totalLatency / float64(count)

			// 计算百分位数
			if len(latencies) > 0 {
				sort.Float64s(latencies)

				// P95
				p95Index := int(float64(len(latencies)) * 0.95)
				if p95Index >= len(latencies) {
					p95Index = len(latencies) - 1
				}
				stats.P95LatencyMs = latencies[p95Index]

				// P99
				p99Index := int(float64(len(latencies)) * 0.99)
				if p99Index >= len(latencies) {
					p99Index = len(latencies) - 1
				}
				stats.P99LatencyMs = latencies[p99Index]
			}
		}
	}

	return stats, nil
}

// 转换函数：将新的NotifierConfig转换为旧的models.Notifier
func convertNotifierConfigToModel(config *notifier.NotifierConfig) *models.Notifier {
	return &models.Notifier{
		ID:      config.ID,
		Name:    config.Name,
		Type:    models.NotifierType(config.Type),
		Enabled: config.Enabled,
		Config:  config.Config,
	}
}

// 转换函数：将旧的models.Notifier转换为新的NotifierConfig
func convertModelToNotifierConfig(model *models.Notifier) *notifier.NotifierConfig {
	return &notifier.NotifierConfig{
		ID:      model.ID,
		Name:    model.Name,
		Type:    notifier.NotifierType(model.Type),
		Enabled: model.Enabled,
		Config:  model.Config,
	}
}
