package systemd

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/utils"
)

// APIHandler systemd API处理器
type APIHandler struct {
	service server.SystemdServiceManager
}

// NewAPIHandler 创建API处理器
func NewAPIHandler(service server.SystemdServiceManager) *APIHandler {
	return &APIHandler{
		service: service,
	}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(router fiber.Router, baseAuthHandler func(c *fiber.Ctx) error, adminRoleHandler func(c *fiber.Ctx) error) {
	systemd := router.Group("/systemd")

	// 服务列表查询
	systemd.Get("/servers/:serverId/services", baseAuthHandler, h.ListServices)

	// 单个服务管理
	services := systemd.Group("/servers/:serverId/services/:serviceName", baseAuthHandler)
	services.Get("", h.GetService)
	services.Post("", h.CreateService)
	services.Put("", h.UpdateService)
	services.Delete("", h.DeleteService)

	// 服务操作
	services.Post("/start", h.StartService)
	services.Post("/stop", h.StopService)
	services.Post("/restart", h.RestartService)
	services.Post("/reload", h.ReloadService)
	services.Post("/enable", h.EnableService)
	services.Post("/disable", h.DisableService)
	services.Get("/status", h.GetServiceStatus)
	services.Get("/logs", h.GetServiceLogs)
	services.Get("/file", h.GetServiceFileContent)

	// 系统操作
	systemd.Post("/servers/:serverId/daemon-reload", h.DaemonReload)
	systemd.Post("/servers/:serverId/reload", h.ReloadSystemd)
}

// ListServices 获取服务列表
// @Summary 获取systemd服务列表
// @Description 根据服务器ID获取systemd服务列表，支持状态过滤和分页
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param status query string false "服务状态过滤" Enums(active,inactive,failed,activating,deactivating)
// @Param enabled query bool false "启用状态过滤"
// @Param pattern query string false "服务名称模式匹配"
// @Param userOnly query bool false "仅显示用户创建的服务" default(false)
// @Param limit query int false "分页大小" default(20)
// @Param offset query int false "分页偏移" default(0)
// @Success 200 {object} utils.Response{data=object{services=[]SystemdService,total=int}}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services [get]
func (h *APIHandler) ListServices(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	req := &server.ServiceListRequest{
		Status:   server.ServiceState(c.Query("status")),
		Pattern:  c.Query("pattern"),
		UserOnly: false,
		Limit:    20,
		Offset:   0,
	}

	// 解析分页参数
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			req.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			req.Offset = offset
		}
	}

	// 解析enabled参数
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			req.Enabled = &enabled
		}
	}

	// 解析userOnly参数
	if userOnlyStr := c.Query("userOnly"); userOnlyStr != "" {
		if userOnly, err := strconv.ParseBool(userOnlyStr); err == nil {
			req.UserOnly = userOnly
		}
	}

	services, total, err := h.service.ListServices(c.Context(), serverID, req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取服务列表失败: "+err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"services": services,
		"total":    total,
	})
}

// GetService 获取单个服务信息
// @Summary 获取systemd服务详细信息
// @Description 根据服务器ID和服务名称获取systemd服务的详细信息
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=SystemdService}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName} [get]
func (h *APIHandler) GetService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	service, err := h.service.GetService(c.Context(), serverID, serviceName)
	if err != nil {
		// 检查是否为服务不存在错误
		if IsServiceNotFound(err) {
			return utils.FailResponse(c, utils.StatusNotFound, "服务不存在: "+err.Error())
		}
		return utils.FailResponse(c, utils.StatusInternalError, "获取服务信息失败: "+err.Error())
	}

	return utils.SuccessResponse(c, service)
}

// CreateService 创建服务
// @Summary 创建systemd服务
// @Description 在指定服务器上创建新的systemd服务
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Param service body ServiceCreateRequest true "服务配置"
// @Success 201 {object} utils.Response{data=SystemdService}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName} [post]
func (h *APIHandler) CreateService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	var req server.ServiceCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	// 使用路径中的服务名称
	req.Name = serviceName

	service, err := h.service.CreateService(c.Context(), serverID, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, service)
}

// UpdateService 更新服务
// @Summary 更新systemd服务
// @Description 更新指定服务器上的systemd服务配置
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Param service body ServiceUpdateRequest true "服务更新配置"
// @Success 200 {object} utils.Response{data=SystemdService}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName} [put]
func (h *APIHandler) UpdateService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	var req server.ServiceUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	service, err := h.service.UpdateService(c.Context(), serverID, serviceName, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "更新服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, service)
}

// DeleteService 删除服务
// @Summary 删除systemd服务
// @Description 删除指定服务器上的systemd服务
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName} [delete]
func (h *APIHandler) DeleteService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.DeleteService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// StartService 启动服务
// @Summary 启动systemd服务
// @Description 启动指定服务器上的systemd服务
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/start [post]
func (h *APIHandler) StartService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.StartService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "启动服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// StopService 停止服务
// @Summary 停止systemd服务
// @Description 停止指定服务器上的systemd服务
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/stop [post]
func (h *APIHandler) StopService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.StopService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "停止服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// RestartService 重启服务
// @Summary 重启systemd服务
// @Description 重启指定服务器上的systemd服务
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/restart [post]
func (h *APIHandler) RestartService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.RestartService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重启服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ReloadService 重载服务
// @Summary 重载systemd服务
// @Description 重载指定服务器上的systemd服务配置
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/reload [post]
func (h *APIHandler) ReloadService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.ReloadService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重载服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// EnableService 启用服务
// @Summary 启用systemd服务开机自启
// @Description 启用指定服务器上的systemd服务开机自启动
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/enable [post]
func (h *APIHandler) EnableService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.EnableService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "启用服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// DisableService 禁用服务
// @Summary 禁用systemd服务开机自启
// @Description 禁用指定服务器上的systemd服务开机自启动
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/disable [post]
func (h *APIHandler) DisableService(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.DisableService(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "禁用服务失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// GetServiceStatus 获取服务状态
// @Summary 获取systemd服务状态
// @Description 获取指定服务器上的systemd服务运行状态
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/status [get]
func (h *APIHandler) GetServiceStatus(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.GetServiceStatus(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取服务状态失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// GetServiceLogs 获取服务日志
// @Summary 获取systemd服务日志
// @Description 获取指定服务器上的systemd服务日志
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Param lines query int false "获取日志行数" default(100)
// @Param follow query bool false "是否跟踪日志" default(false)
// @Success 200 {object} utils.Response{data=ServiceLogResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/logs [get]
func (h *APIHandler) GetServiceLogs(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	req := &server.ServiceLogRequest{
		ServerID: serverID,
		Name:     serviceName,
		Lines:    100,
		Follow:   false,
	}

	// 解析查询参数
	if linesStr := c.Query("lines"); linesStr != "" {
		if lines, err := strconv.Atoi(linesStr); err == nil && lines > 0 {
			req.Lines = lines
		}
	}

	if followStr := c.Query("follow"); followStr != "" {
		if follow, err := strconv.ParseBool(followStr); err == nil {
			req.Follow = follow
		}
	}

	result, err := h.service.GetServiceLogs(c.Context(), req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取服务日志失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// DaemonReload 重载systemd守护进程
// @Summary 重载systemd守护进程
// @Description 重载指定服务器上的systemd守护进程配置
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/daemon-reload [post]
func (h *APIHandler) DaemonReload(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := h.service.DaemonReload(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重载systemd守护进程失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ReloadSystemd 重载systemd
// @Summary 重载systemd
// @Description 重载指定服务器上的systemd配置
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=ServiceOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/reload [post]
func (h *APIHandler) ReloadSystemd(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := h.service.ReloadSystemd(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重载systemd失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// GetServiceFileContent 获取服务文件内容
// @Summary 获取systemd服务文件内容
// @Description 获取指定服务器上的systemd服务配置文件内容
// @Tags systemd
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param serviceName path string true "服务名称"
// @Success 200 {object} utils.Response{data=ServiceFileResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/systemd/servers/{serverId}/services/{serviceName}/file [get]
func (h *APIHandler) GetServiceFileContent(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	serviceName := c.Params("serviceName")

	if serverID == "" || serviceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和服务名称不能为空")
	}

	result, err := h.service.GetServiceFileContent(c.Context(), serverID, serviceName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取服务文件内容失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}
