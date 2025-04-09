// Package api 提供监控系统的API接口
package api

import (
	"fmt"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"github.com/xsxdot/aio/internal/monitoring/storage"
	"github.com/xsxdot/aio/pkg/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

// APIError 表示API错误
type APIError struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
	Details string `json:"details"` // 错误详情
}

// ServiceAPI 是服务监控相关的API处理器
type ServiceAPI struct {
	storage *storage.Storage
}

// NewServiceAPI 创建一个新的服务监控API处理器
func NewServiceAPI(storage *storage.Storage) *ServiceAPI {
	return &ServiceAPI{
		storage: storage,
	}
}

// RegisterRoutes 注册服务监控API路由
func (api *ServiceAPI) RegisterRoutes(router fiber.Router) {
	// 应用服务相关路由
	router.Get("/apps", api.GetAllServices)
	router.Get("/apps/:serviceName", api.GetService)

	// 服务实例相关路由
	router.Get("/apps/:serviceName/instances", api.GetServiceInstances)
	router.Get("/apps/:serviceName/instances/:instanceId", api.GetServiceInstance)
	router.Get("/apps/:serviceName/instances/:instanceId/metrics", api.GetServiceInstanceMetrics)

	// 服务接口相关路由
	router.Get("/apps/:serviceName/endpoints", api.GetServiceEndpoints)
	router.Get("/apps/:serviceName/endpoints/:endpoint", api.GetServiceEndpoint)
	router.Get("/apps/:serviceName/endpoints/:endpoint/metrics", api.GetServiceEndpointMetrics)

	// 服务汇总指标相关路由
	router.Get("/apps/:serviceName/metrics/summary", api.GetServiceMetricsSummary)
	router.Get("/apps/:serviceName/metrics/qps", api.GetServiceQPSMetrics)
	router.Get("/apps/:serviceName/metrics/error-rate", api.GetServiceErrorRateMetrics)
	router.Get("/apps/:serviceName/metrics/response-time", api.GetServiceResponseTimeMetrics)
}

// GetAllServices 获取所有应用服务列表
// @Summary 获取所有应用服务列表
// @Description 获取系统中所有已注册应用服务的列表
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param tag query string false "按标签筛选"
// @Param status query string false "按状态筛选 (up, down, warning, unknown)"
// @Param search query string false "搜索关键词"
// @Param limit query int false "限制返回数量"
// @Param offset query int false "分页偏移量"
// @Success 200 {array} models.Service "服务列表"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps [get]
func (api *ServiceAPI) GetAllServices(c *fiber.Ctx) error {
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

// GetService 获取指定服务的详细信息
// @Summary 获取指定服务的详细信息
// @Description 获取特定应用服务的详细信息，包括实例和接口概览
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Success 200 {object} models.Service "服务详情"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName} [get]
func (api *ServiceAPI) GetService(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务详情
	service, err := api.storage.GetService(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务不存在: %v", err))
	}

	return utils.SuccessResponse(c, service)
}

// GetServiceInstances 获取服务的所有实例
// @Summary 获取服务的所有实例
// @Description 获取特定应用服务的所有运行实例
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Success 200 {array} models.ServiceInstance "服务实例列表"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/instances [get]
func (api *ServiceAPI) GetServiceInstances(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务实例
	instances, err := api.storage.GetServiceInstances(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务不存在: %v", err))
	}

	return utils.SuccessResponse(c, instances)
}

// GetServiceInstance 获取服务的特定实例
// @Summary 获取服务的特定实例
// @Description 获取特定应用服务的特定实例的详细信息
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param instanceId path string true "实例ID"
// @Success 200 {object} models.ServiceInstance "服务实例详情"
// @Failure 404 {object} middleware.Response "服务或实例不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/instances/{instanceId} [get]
func (api *ServiceAPI) GetServiceInstance(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	instanceID := c.Params("instanceId")

	// 从存储中获取服务实例
	instance, err := api.storage.GetServiceInstance(serviceName, instanceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务实例不存在: %v", err))
	}

	return utils.SuccessResponse(c, instance)
}

// GetServiceInstanceMetrics 获取服务实例的指标数据
// @Summary 获取服务实例的指标数据
// @Description 获取特定服务实例的运行指标时间序列数据
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param instanceId path string true "实例ID"
// @Param start query string false "开始时间 (ISO 8601)"
// @Param end query string false "结束时间 (ISO 8601)"
// @Param interval query string false "时间间隔 (如 1m, 5m, 1h)"
// @Param aggregation query string false "聚合方式 (如 avg, max, min, sum)"
// @Success 200 {object} models.QueryResult "指标数据"
// @Failure 404 {object} middleware.Response "服务或实例不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/instances/{instanceId}/metrics [get]
func (api *ServiceAPI) GetServiceInstanceMetrics(c *fiber.Ctx) error {
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

// GetServiceEndpoints 获取服务的所有接口
// @Summary 获取服务的所有接口
// @Description 获取特定应用服务的所有接口列表
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Success 200 {array} models.ServiceEndpoint "服务接口列表"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/endpoints [get]
func (api *ServiceAPI) GetServiceEndpoints(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务接口
	endpoints, err := api.storage.GetServiceEndpoints(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务不存在: %v", err))
	}

	return utils.SuccessResponse(c, endpoints)
}

// GetServiceEndpoint 获取服务的特定接口
// @Summary 获取服务的特定接口
// @Description 获取特定应用服务的特定接口详情
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param endpoint path string true "接口路径"
// @Param method query string false "HTTP方法 (GET, POST等)"
// @Success 200 {object} models.ServiceEndpoint "服务接口详情"
// @Failure 404 {object} middleware.Response "服务或接口不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/endpoints/{endpoint} [get]
func (api *ServiceAPI) GetServiceEndpoint(c *fiber.Ctx) error {
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

// GetServiceEndpointMetrics 获取服务接口的指标数据
// @Summary 获取服务接口的指标数据
// @Description 获取特定服务接口的性能指标时间序列数据
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param endpoint path string true "接口路径"
// @Param method query string false "HTTP方法 (GET, POST等)"
// @Param start query string false "开始时间 (ISO 8601)"
// @Param end query string false "结束时间 (ISO 8601)"
// @Param interval query string false "时间间隔 (如 1m, 5m, 1h)"
// @Param aggregation query string false "聚合方式 (如 avg, max, min, sum)"
// @Success 200 {object} models.QueryResult "指标数据"
// @Failure 404 {object} middleware.Response "服务或接口不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/endpoints/{endpoint}/metrics [get]
func (api *ServiceAPI) GetServiceEndpointMetrics(c *fiber.Ctx) error {
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

// GetServiceMetricsSummary 获取服务的汇总指标
// @Summary 获取服务的汇总指标
// @Description 获取特定应用服务的汇总性能指标
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Success 200 {object} models.ServiceMetricsSummary "服务汇总指标"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/metrics/summary [get]
func (api *ServiceAPI) GetServiceMetricsSummary(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")

	// 从存储中获取服务汇总指标
	summary, err := api.storage.QueryServiceSummary(serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("查询服务汇总指标失败: %v", err))
	}

	return utils.SuccessResponse(c, summary)
}

// GetServiceQPSMetrics 获取服务的QPS指标
// @Summary 获取服务的QPS指标
// @Description 获取特定应用服务的QPS时间序列数据
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param start query string false "开始时间 (ISO 8601)"
// @Param end query string false "结束时间 (ISO 8601)"
// @Param interval query string false "时间间隔 (如 1m, 5m, 1h)"
// @Success 200 {object} models.QueryResult "QPS指标数据"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/metrics/qps [get]
func (api *ServiceAPI) GetServiceQPSMetrics(c *fiber.Ctx) error {
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

// GetServiceErrorRateMetrics 获取服务的错误率指标
// @Summary 获取服务的错误率指标
// @Description 获取特定应用服务的错误率时间序列数据
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param start query string false "开始时间 (ISO 8601)"
// @Param end query string false "结束时间 (ISO 8601)"
// @Param interval query string false "时间间隔 (如 1m, 5m, 1h)"
// @Success 200 {object} models.QueryResult "错误率指标数据"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/metrics/error-rate [get]
func (api *ServiceAPI) GetServiceErrorRateMetrics(c *fiber.Ctx) error {
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

// GetServiceResponseTimeMetrics 获取服务的响应时间指标
// @Summary 获取服务的响应时间指标
// @Description 获取特定应用服务的响应时间时间序列数据
// @Tags 服务监控
// @Accept json
// @Produce json
// @Param serviceName path string true "服务名称"
// @Param start query string false "开始时间 (ISO 8601)"
// @Param end query string false "结束时间 (ISO 8601)"
// @Param interval query string false "时间间隔 (如 1m, 5m, 1h)"
// @Success 200 {object} models.QueryResult "响应时间指标数据"
// @Failure 404 {object} middleware.Response "服务不存在"
// @Failure 500 {object} middleware.Response "服务器内部错误"
// @Router /api/monitoring/apps/{serviceName}/metrics/response-time [get]
func (api *ServiceAPI) GetServiceResponseTimeMetrics(c *fiber.Ctx) error {
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

// parseTimeParam 解析时间参数
func (api *ServiceAPI) parseTimeParam(value string, defaultValue time.Time) time.Time {
	if value == "" {
		return defaultValue
	}

	// 尝试解析ISO 8601格式
	t, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return t
	}

	// 如果解析失败，返回默认值
	return defaultValue
}

// parseIntParam 解析整数参数
func (api *ServiceAPI) parseIntParam(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}

	// 尝试解析整数
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	if err != nil {
		return defaultValue
	}

	return result
}
