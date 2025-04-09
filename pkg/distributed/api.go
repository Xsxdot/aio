package distributed

import (
	"fmt"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/utils"
	"go.uber.org/zap"

	"github.com/gofiber/fiber/v2"
)

// API 分布式组件API
type API struct {
	logger    *zap.Logger
	discovery discovery.DiscoveryService
	election  election.ElectionService
}

// NewAPI 创建分布式组件API
func NewAPI(discoveryService discovery.DiscoveryService, election election.ElectionService, logger *zap.Logger) *API {
	return &API{
		logger:    logger,
		discovery: discoveryService,
		election:  election,
	}
}

// SetupRoutes 设置API路由
func (a *API) SetupRoutes(app *fiber.App) {
	g := app.Group("/api/distributed")

	electionGroup := g.Group("/election")
	electionGroup.Get("/:name/leader", a.getElectionLeader)
	// 服务发现API
	discoveryGroup := g.Group("/discovery")
	discoveryGroup.Get("/services", a.getAllServices)
	discoveryGroup.Get("/services/:name", a.getServicesByName)
	discoveryGroup.Post("/services", a.registerService)
	discoveryGroup.Delete("/services/:id", a.deregisterService)

}

func (a *API) getElectionLeader(c *fiber.Ctx) error {
	election := a.election.GetDefaultElection()
	leader, err := election.GetLeader(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{"leader": leader})
}

// 服务发现API处理函数
func (a *API) getAllServices(c *fiber.Ctx) error {
	services, err := a.discovery.GetAllServices(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}
	return utils.SuccessResponse(c, services)
}

func (a *API) getServicesByName(c *fiber.Ctx) error {
	name := c.Params("name")
	services, err := a.discovery.Discover(c.Context(), name)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}
	return utils.SuccessResponse(c, services)
}

func (a *API) registerService(c *fiber.Ctx) error {
	var service discovery.ServiceInfo
	if err := c.BodyParser(&service); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, err.Error())
	}

	err := a.discovery.Register(c.Context(), service)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, service)
}

func (a *API) deregisterService(c *fiber.Ctx) error {
	id := c.Params("id")
	err := a.discovery.Deregister(c.Context(), id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}
	return utils.SuccessResponse(c, fiber.Map{"message": "服务已注销"})
}

func (a *API) updateService(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Address  string                 `json:"address"`
		Port     int                    `json:"port"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, err.Error())
	}

	// 获取所有服务
	services, err := a.discovery.GetAllServices(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	// 查找指定ID的服务
	var targetService *discovery.ServiceInfo
	for _, serviceList := range services {
		for i := range serviceList {
			if serviceList[i].ID == id {
				targetService = &serviceList[i]
				break
			}
		}
		if targetService != nil {
			break
		}
	}

	if targetService == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "服务不存在")
	}

	// 更新服务信息
	if req.Address != "" {
		targetService.Address = req.Address
	}
	if req.Port > 0 {
		targetService.Port = req.Port
	}
	if req.Metadata != nil {
		// 合并元数据，注意转换类型
		if targetService.Metadata == nil {
			targetService.Metadata = make(map[string]string)
		}
		for k, v := range req.Metadata {
			// 将interface{}转为string
			strValue, ok := v.(string)
			if ok {
				targetService.Metadata[k] = strValue
			} else {
				// 尝试用fmt转换为字符串
				targetService.Metadata[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// 因为我们没有Update方法，需要先注销再注册 todo
	err = a.discovery.Deregister(c.Context(), id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "注销服务失败: "+err.Error())
	}

	// 重新注册服务
	err = a.discovery.Register(c.Context(), *targetService)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "注册服务失败: "+err.Error())
	}

	// 返回结果，不设置UpdateTime，因为ServiceInfo中没有这个字段
	return utils.SuccessResponse(c, targetService)
}
