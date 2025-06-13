package api

import (
	"fmt"
	"github.com/xsxdot/aio/internal/monitoring/models"
	"github.com/xsxdot/aio/pkg/utils"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// setupAPIMetricsRoutes 设置API指标查询和聚合相关的路由
func (api *API) setupAPIMetricsRoutes(monitoringGroup fiber.Router, handler func(*fiber.Ctx) error, roleHandler func(*fiber.Ctx) error) {
	// API指标聚合查询
	apiGroup := monitoringGroup.Group("/api-metrics", handler)

	// API概要统计
	apiGroup.Get("/summary", api.getAPISummary)

	// 响应时间相关
	apiGroup.Get("/response-time", api.getAPIResponseTime)

	// QPS相关
	apiGroup.Get("/qps", api.getAPIQPS)

	// 错误率相关
	apiGroup.Get("/error-rate", api.getAPIErrorRate)

	// 调用分布相关
	apiGroup.Get("/distribution", api.getAPIDistribution)

	// 聚合查询
	apiGroup.Get("/query", api.handleAPIMetricsQuery)

	// 获取API数据源
	apiGroup.Get("/sources", api.getAPISources)

	api.logger.Info("API指标查询路由已设置")
}

// getAPISummary 获取API调用概要统计
func (api *API) getAPISummary(c *fiber.Ctx) error {
	// 解析查询参数
	options := api.parseAPIMetricsOptions(c)

	// 查询API概要统计
	result, err := api.storage.QueryAPISummary(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API概要统计失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// getAPIResponseTime 获取API响应时间统计
func (api *API) getAPIResponseTime(c *fiber.Ctx) error {
	// 解析查询参数
	options := api.parseAPIMetricsOptions(c)

	// 查询响应时间统计
	result, err := api.storage.QueryAPIResponseTime(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API响应时间失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// getAPIQPS 获取API的QPS统计
func (api *API) getAPIQPS(c *fiber.Ctx) error {
	// 解析查询参数
	options := api.parseAPIMetricsOptions(c)

	// 查询QPS统计
	result, err := api.storage.QueryAPIQPS(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API QPS失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// getAPIErrorRate 获取API错误率统计
func (api *API) getAPIErrorRate(c *fiber.Ctx) error {
	// 解析查询参数
	options := api.parseAPIMetricsOptions(c)

	// 查询错误率统计
	result, err := api.storage.QueryAPIErrorRate(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API错误率失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// getAPIDistribution 获取API调用分布统计
func (api *API) getAPIDistribution(c *fiber.Ctx) error {
	// 解析查询参数
	options := api.parseAPIMetricsOptions(c)

	// 查询调用分布统计
	result, err := api.storage.QueryAPICallDistribution(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API调用分布失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// handleAPIMetricsQuery 处理API指标聚合查询
func (api *API) handleAPIMetricsQuery(c *fiber.Ctx) error {
	// 解析查询参数
	options := api.parseAPIMetricsOptions(c)

	// 检查聚合类型
	aggregationType := models.APIMetricsAggregationType(c.Query("aggregation", string(models.AggregationAvg)))
	options.Aggregation = aggregationType

	// 查询时间序列
	result, err := api.storage.QueryAPIMetricsTimeSeries(options)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API指标失败: %v", err))
	}

	return utils.SuccessResponse(c, result)
}

// parseAPIMetricsOptions 从请求中解析API指标查询选项
func (api *API) parseAPIMetricsOptions(c *fiber.Ctx) models.APIMetricsQueryOptions {
	// 解析基本查询参数
	startTime := api.parseTimeParam(c.Query("start"), time.Now().Add(-1*time.Hour))
	endTime := api.parseTimeParam(c.Query("end"), time.Now())
	limit := api.parseIntParam(c.Query("limit"), 100)

	// 解析过滤参数
	source := c.Query("source", "")
	instance := c.Query("instance", "")
	endpoint := c.Query("endpoint", "")
	method := c.Query("method", "")

	// 解析标签过滤
	tagFilter := make(map[string]string)
	tagFilterStr := c.Query("tags", "")
	if tagFilterStr != "" {
		tagFilter = api.parseLabels(tagFilterStr)
	}

	// 解析聚合参数
	intervalStr := c.Query("interval", "60s")
	var interval time.Duration
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		api.logger.Warn("解析interval参数失败，使用默认值60s", zap.Error(err))
		interval = 60 * time.Second
	}

	// 解析分组字段
	groupBy := make([]string, 0)
	groupByStr := c.Query("group_by", "")
	if groupByStr != "" {
		for _, field := range api.parseCommaSeparated(groupByStr) {
			groupBy = append(groupBy, field)
		}
	}

	return models.APIMetricsQueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		Limit:       limit,
		Source:      source,
		Instance:    instance,
		Endpoint:    endpoint,
		Method:      method,
		TagFilter:   tagFilter,
		Interval:    interval,
		GroupBy:     groupBy,
		Aggregation: models.AggregationAvg, // 默认值，会在各方法中根据需要修改
	}
}

// parseCommaSeparated 解析逗号分隔的字符串为字符串切片
func (api *API) parseCommaSeparated(input string) []string {
	if input == "" {
		return []string{}
	}

	result := make([]string, 0)
	for _, item := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// getAPISources 获取所有API数据源
func (api *API) getAPISources(c *fiber.Ctx) error {
	// 查询所有数据源
	sources, err := api.storage.QueryAPISources()
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询API数据源失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"sources": sources,
	})
}
