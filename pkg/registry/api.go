package registry

import (
	"fmt"
	"time"

	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// API 服务注册中心的HTTP API
type API struct {
	registry Registry
	logger   *zap.Logger
}

// NewAPI 创建新的服务注册中心API
func NewAPI(registry Registry, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &API{
		registry: registry,
		logger:   logger,
	}
}

// RegisterRoutes 注册所有API路由
func (api *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建API组
	registryGroup := router.Group("/registry")

	// 服务实例基本操作
	registryGroup.Post("/services", authHandler, api.registerService)
	registryGroup.Delete("/services/:serviceID", authHandler, api.unregisterService)
	registryGroup.Put("/services/:serviceID/renew", authHandler, api.renewService)
	registryGroup.Get("/services/:serviceID", authHandler, api.getService)

	// 服务发现相关
	registryGroup.Get("/services", authHandler, api.getAllServicesAdmin)
	registryGroup.Get("/discovery/:serviceName", authHandler, api.discoverService)

	// 健康检查
	registryGroup.Get("/services/:serviceID/health", authHandler, api.checkServiceHealth)

	// 服务统计信息
	registryGroup.Get("/stats", authHandler, api.getRegistryStats)
	registryGroup.Get("/services/:serviceName/stats", authHandler, api.getServiceStats)

	// 管理员功能
	registryGroup.Get("/admin/all", authHandler, adminRoleHandler, api.getAllServicesAdmin)
	registryGroup.Delete("/admin/services/:serviceName", authHandler, adminRoleHandler, api.removeAllServiceInstances)

	api.logger.Info("服务注册中心API路由已注册")
}

// registerService 注册服务实例
func (api *API) registerService(c *fiber.Ctx) error {
	var request struct {
		Name     string            `json:"name"`
		Address  string            `json:"address"`
		Protocol string            `json:"protocol"`
		Env      string            `json:"env"`
		Metadata map[string]string `json:"metadata"`
		Weight   int               `json:"weight"`
		Status   string            `json:"status"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if request.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务名称不能为空")
	}

	if request.Address == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务地址不能为空")
	}

	// 设置默认值
	if request.Protocol == "" {
		request.Protocol = "http"
	}
	if request.Weight <= 0 {
		request.Weight = 100
	}
	if request.Status == "" {
		request.Status = "active"
	}
	// env的默认值将在注册中心的Register方法中处理

	instance := &ServiceInstance{
		Name:         request.Name,
		Address:      request.Address,
		Protocol:     request.Protocol,
		Env:          ParseEnvironment(request.Env),
		Metadata:     request.Metadata,
		Weight:       request.Weight,
		Status:       request.Status,
		RegisterTime: time.Now(),
		StartTime:    time.Now(),
	}

	err := api.registry.Register(c.Context(), instance)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("注册服务失败: %v", err))
	}

	return utils.SuccessResponse(c, instance)
}

// unregisterService 注销服务实例
func (api *API) unregisterService(c *fiber.Ctx) error {
	serviceID := c.Params("serviceID")
	if serviceID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务实例ID不能为空")
	}

	// 检查服务是否存在
	_, err := api.registry.GetService(c.Context(), serviceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("服务实例不存在: %v", err))
	}

	err = api.registry.Unregister(c.Context(), serviceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("注销服务失败: %v", err))
	}

	return utils.SuccessResponse(c, "服务注销成功")
}

// renewService 续约服务实例
func (api *API) renewService(c *fiber.Ctx) error {
	serviceID := c.Params("serviceID")
	if serviceID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务实例ID不能为空")
	}

	err := api.registry.Renew(c.Context(), serviceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("续约服务失败: %v", err))
	}

	return utils.SuccessResponse(c, "服务续约成功")
}

// getService 获取单个服务实例
func (api *API) getService(c *fiber.Ctx) error {
	serviceID := c.Params("serviceID")
	if serviceID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务实例ID不能为空")
	}

	instance, err := api.registry.GetService(c.Context(), serviceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取服务实例失败: %v", err))
	}

	return utils.SuccessResponse(c, instance)
}

// discoverService 发现服务实例列表
func (api *API) discoverService(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	if serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务名称不能为空")
	}

	// 获取查询参数
	env := c.Query("env")
	status := c.Query("status")
	protocol := c.Query("protocol")

	var instances []*ServiceInstance
	var err error

	// 根据是否指定环境使用不同的发现方法
	if env != "" {
		instances, err = api.registry.DiscoverByEnv(c.Context(), serviceName, ParseEnvironment(env))
	} else {
		instances, err = api.registry.Discover(c.Context(), serviceName)
	}

	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("发现服务失败: %v", err))
	}

	// 过滤其他查询参数
	var filteredInstances []*ServiceInstance
	for _, instance := range instances {
		// 状态过滤
		if status != "" && instance.Status != status {
			continue
		}
		// 协议过滤
		if protocol != "" && instance.Protocol != protocol {
			continue
		}
		filteredInstances = append(filteredInstances, instance)
	}

	return utils.SuccessResponse(c, filteredInstances)
}

// checkServiceHealth 检查服务健康状态
func (api *API) checkServiceHealth(c *fiber.Ctx) error {
	serviceID := c.Params("serviceID")
	if serviceID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务实例ID不能为空")
	}

	instance, err := api.registry.GetService(c.Context(), serviceID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取服务实例失败: %v", err))
	}

	health := map[string]interface{}{
		"service_id":        instance.ID,
		"service_name":      instance.Name,
		"status":            instance.Status,
		"healthy":           instance.IsHealthy(),
		"uptime":            instance.GetUptime().String(),
		"register_duration": instance.GetRegisterDuration().String(),
		"last_check":        time.Now(),
	}

	return utils.SuccessResponse(c, health)
}

// getRegistryStats 获取注册中心统计信息
func (api *API) getRegistryStats(c *fiber.Ctx) error {
	services, err := api.registry.ListServices(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取服务列表失败: %v", err))
	}

	totalInstances := 0
	healthyInstances := 0
	serviceStats := make(map[string]int)

	for _, serviceName := range services {
		instances, err := api.registry.Discover(c.Context(), serviceName)
		if err != nil {
			continue
		}

		serviceStats[serviceName] = len(instances)
		totalInstances += len(instances)

		for _, instance := range instances {
			if instance.IsHealthy() {
				healthyInstances++
			}
		}
	}

	stats := map[string]interface{}{
		"total_services":      len(services),
		"total_instances":     totalInstances,
		"healthy_instances":   healthyInstances,
		"unhealthy_instances": totalInstances - healthyInstances,
		"service_stats":       serviceStats,
		"timestamp":           time.Now(),
	}

	return utils.SuccessResponse(c, stats)
}

// getServiceStats 获取指定服务的统计信息
func (api *API) getServiceStats(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	if serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务名称不能为空")
	}

	instances, err := api.registry.Discover(c.Context(), serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("发现服务失败: %v", err))
	}

	healthyInstances := 0
	protocols := make(map[string]int)
	statuses := make(map[string]int)

	for _, instance := range instances {
		if instance.IsHealthy() {
			healthyInstances++
		}
		protocols[instance.Protocol]++
		statuses[instance.Status]++
	}

	stats := map[string]interface{}{
		"service_name":        serviceName,
		"total_instances":     len(instances),
		"healthy_instances":   healthyInstances,
		"unhealthy_instances": len(instances) - healthyInstances,
		"protocols":           protocols,
		"statuses":            statuses,
		"instances":           instances,
		"timestamp":           time.Now(),
	}

	return utils.SuccessResponse(c, stats)
}

// getAllServicesAdmin 管理员获取所有服务详细信息
func (api *API) getAllServicesAdmin(c *fiber.Ctx) error {
	services, err := api.registry.ListServices(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取服务列表失败: %v", err))
	}

	allServices := make(map[string][]*ServiceInstance)
	for _, serviceName := range services {
		instances, err := api.registry.Discover(c.Context(), serviceName)
		if err != nil {
			api.logger.Warn("获取服务实例失败",
				zap.String("service_name", serviceName),
				zap.Error(err))
			continue
		}
		allServices[serviceName] = instances
	}

	return utils.SuccessResponse(c, allServices)
}

// removeAllServiceInstances 管理员删除指定服务的所有实例
func (api *API) removeAllServiceInstances(c *fiber.Ctx) error {
	serviceName := c.Params("serviceName")
	if serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务名称不能为空")
	}

	instances, err := api.registry.Discover(c.Context(), serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("发现服务失败: %v", err))
	}

	var removedCount int
	var errors []string

	for _, instance := range instances {
		err := api.registry.Unregister(c.Context(), instance.ID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("删除实例 %s 失败: %v", instance.ID, err))
			api.logger.Warn("删除服务实例失败",
				zap.String("service_id", instance.ID),
				zap.Error(err))
		} else {
			removedCount++
		}
	}

	result := map[string]interface{}{
		"service_name":    serviceName,
		"total_instances": len(instances),
		"removed_count":   removedCount,
		"errors":          errors,
	}

	if len(errors) > 0 {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("部分删除失败，详情请查看响应数据"))
	}

	return utils.SuccessResponse(c, result)
}
