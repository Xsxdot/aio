package nginx

import (
	"net/url"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/utils"
)

// APIHandler nginx API处理器
type APIHandler struct {
	service server.NginxServiceManager
}

// NewAPIHandler 创建API处理器
func NewAPIHandler(service server.NginxServiceManager) *APIHandler {
	return &APIHandler{
		service: service,
	}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(router fiber.Router, baseAuthHandler func(c *fiber.Ctx) error, adminRoleHandler func(c *fiber.Ctx) error) {
	nginx := router.Group("/nginx")

	// nginx服务器管理
	servers := nginx.Group("/servers", baseAuthHandler)
	servers.Get("", h.ListNginxServers)
	servers.Post("", h.AddNginxServer)

	// 单个nginx服务器管理
	server := nginx.Group("/servers/:serverId", baseAuthHandler)
	server.Get("", h.GetNginxServer)
	server.Put("", h.UpdateNginxServer)
	server.Delete("", h.DeleteNginxServer)

	// nginx操作
	server.Post("/test", h.TestConfig)
	server.Post("/reload", h.ReloadConfig)
	server.Post("/restart", h.RestartNginx)
	server.Get("/status", h.GetNginxStatus)

	// 配置文件管理
	configs := server.Group("/configs")
	configs.Get("", h.ListConfigs)
	configs.Post("", h.CreateConfig)
	configs.Get("/*", h.GetConfig)
	configs.Put("/*", h.UpdateConfig)
	configs.Delete("/*", h.DeleteConfig)

	// 站点管理
	sites := server.Group("/sites")
	sites.Get("", h.ListSites)
	sites.Post("", h.CreateSite)

	// 单个站点管理
	site := sites.Group("/:siteName")
	site.Get("", h.GetSite)
	site.Put("", h.UpdateSite)
	site.Delete("", h.DeleteSite)
	site.Post("/enable", h.EnableSite)
	site.Post("/disable", h.DisableSite)
}

// ListNginxServers 获取nginx服务器列表
// @Summary 获取nginx服务器列表
// @Description 获取所有nginx服务器的列表，支持状态过滤和分页
// @Tags nginx
// @Accept json
// @Produce json
// @Param status query string false "服务器状态过滤"
// @Param limit query int false "分页大小" default(20)
// @Param offset query int false "分页偏移" default(0)
// @Success 200 {object} utils.Response{data=object{servers=[]server.NginxServer,total=int}}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers [get]
func (h *APIHandler) ListNginxServers(c *fiber.Ctx) error {
	req := &server.NginxServerListRequest{
		Status: c.Query("status"),
		Limit:  20,
		Offset: 0,
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

	servers, total, err := h.service.ListNginxServers(c.Context(), req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取nginx服务器列表失败: "+err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"servers": servers,
		"total":   total,
	})
}

// AddNginxServer 添加nginx服务器
// @Summary 添加nginx服务器
// @Description 添加新的nginx服务器到管理系统
// @Tags nginx
// @Accept json
// @Produce json
// @Param server body server.NginxServerCreateRequest true "nginx服务器配置"
// @Success 201 {object} utils.Response{data=server.NginxServer}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers [post]
func (h *APIHandler) AddNginxServer(c *fiber.Ctx) error {
	var req server.NginxServerCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	nginxServer, err := h.service.AddNginxServer(c.Context(), &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "添加nginx服务器失败: "+err.Error())
	}

	return utils.SuccessResponse(c, nginxServer)
}

// GetNginxServer 获取nginx服务器信息
// @Summary 获取nginx服务器详细信息
// @Description 根据服务器ID获取nginx服务器的详细信息
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=server.NginxServer}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId} [get]
func (h *APIHandler) GetNginxServer(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	nginxServer, err := h.service.GetNginxServer(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "获取nginx服务器失败: "+err.Error())
	}

	return utils.SuccessResponse(c, nginxServer)
}

// UpdateNginxServer 更新nginx服务器
// @Summary 更新nginx服务器配置
// @Description 更新指定nginx服务器的配置信息
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param server body server.NginxServerUpdateRequest true "nginx服务器更新配置"
// @Success 200 {object} utils.Response{data=server.NginxServer}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId} [put]
func (h *APIHandler) UpdateNginxServer(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	var req server.NginxServerUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	nginxServer, err := h.service.UpdateNginxServer(c.Context(), serverID, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "更新nginx服务器失败: "+err.Error())
	}

	return utils.SuccessResponse(c, nginxServer)
}

// DeleteNginxServer 删除nginx服务器
// @Summary 删除nginx服务器
// @Description 删除指定的nginx服务器及其所有关联数据
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId} [delete]
func (h *APIHandler) DeleteNginxServer(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	err := h.service.DeleteNginxServer(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除nginx服务器失败: "+err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{"message": "删除成功"})
}

// TestConfig 测试nginx配置
// @Summary 测试nginx配置
// @Description 测试指定服务器的nginx配置是否正确
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/test [post]
func (h *APIHandler) TestConfig(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := h.service.TestConfig(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "测试nginx配置失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ReloadConfig 重载nginx配置
// @Summary 重载nginx配置
// @Description 重载指定服务器的nginx配置
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/reload [post]
func (h *APIHandler) ReloadConfig(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := h.service.ReloadConfig(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重载nginx配置失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// RestartNginx 重启nginx
// @Summary 重启nginx服务
// @Description 重启指定服务器的nginx服务
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/restart [post]
func (h *APIHandler) RestartNginx(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := h.service.RestartNginx(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重启nginx失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// GetNginxStatus 获取nginx状态
// @Summary 获取nginx状态
// @Description 获取指定服务器的nginx运行状态
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Success 200 {object} utils.Response{data=server.NginxStatusResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/status [get]
func (h *APIHandler) GetNginxStatus(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := h.service.GetNginxStatus(c.Context(), serverID)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取nginx状态失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ListConfigs 获取配置文件列表
// @Summary 获取nginx配置文件列表
// @Description 获取指定服务器的nginx配置文件列表
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param path query string false "目录路径"
// @Param type query string false "配置文件类型过滤" Enums(main,site,module,custom)
// @Param limit query int false "分页大小" default(20)
// @Param offset query int false "分页偏移" default(0)
// @Success 200 {object} utils.Response{data=object{configs=[]server.NginxConfig,total=int}}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/configs [get]
func (h *APIHandler) ListConfigs(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	req := &server.NginxConfigListRequest{
		Path:   c.Query("path"),
		Type:   server.NginxConfigType(c.Query("type")),
		Limit:  20,
		Offset: 0,
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

	configs, total, err := h.service.ListConfigs(c.Context(), serverID, req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取配置文件列表失败: "+err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"configs": configs,
		"total":   total,
	})
}

// GetConfig 获取配置文件内容
// @Summary 获取nginx配置文件内容
// @Description 获取指定配置文件的详细内容
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param configPath path string true "配置文件路径"
// @Success 200 {object} utils.Response{data=server.NginxConfig}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/configs/{configPath} [get]
func (h *APIHandler) GetConfig(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	configPath := c.Params("*")

	if serverID == "" || configPath == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和配置文件路径不能为空")
	}

	unescapedPath, err := url.QueryUnescape(configPath)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置文件路径解析失败: "+err.Error())
	}

	config, err := h.service.GetConfig(c.Context(), serverID, unescapedPath)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "获取配置文件失败: "+err.Error())
	}

	return utils.SuccessResponse(c, config)
}

// CreateConfig 创建配置文件
// @Summary 创建nginx配置文件
// @Description 在指定服务器上创建新的nginx配置文件
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param config body server.NginxConfigCreateRequest true "配置文件信息"
// @Success 201 {object} utils.Response{data=server.NginxConfig}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/configs [post]
func (h *APIHandler) CreateConfig(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	var req server.NginxConfigCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	config, err := h.service.CreateConfig(c.Context(), serverID, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建配置文件失败: "+err.Error())
	}

	return utils.SuccessResponse(c, config)
}

// UpdateConfig 更新配置文件
// @Summary 更新nginx配置文件
// @Description 更新指定配置文件的内容
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param configPath path string true "配置文件路径"
// @Param config body server.NginxConfigUpdateRequest true "配置文件更新信息"
// @Success 200 {object} utils.Response{data=server.NginxConfig}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/configs/{configPath} [put]
func (h *APIHandler) UpdateConfig(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	configPath := c.Params("*")

	if serverID == "" || configPath == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和配置文件路径不能为空")
	}

	unescapedPath, err := url.QueryUnescape(configPath)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置文件路径解析失败: "+err.Error())
	}

	var req server.NginxConfigUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	config, err := h.service.UpdateConfig(c.Context(), serverID, unescapedPath, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "更新配置文件失败: "+err.Error())
	}

	return utils.SuccessResponse(c, config)
}

// DeleteConfig 删除配置文件
// @Summary 删除nginx配置文件
// @Description 删除指定的nginx配置文件
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param configPath path string true "配置文件路径"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/configs/{configPath} [delete]
func (h *APIHandler) DeleteConfig(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	configPath := c.Params("*")

	if serverID == "" || configPath == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和配置文件路径不能为空")
	}

	unescapedPath, err := url.QueryUnescape(configPath)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置文件路径解析失败: "+err.Error())
	}

	result, err := h.service.DeleteConfig(c.Context(), serverID, unescapedPath)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除配置文件失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ListSites 获取站点列表
// @Summary 获取nginx站点列表
// @Description 获取指定服务器的nginx站点列表
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param enabled query bool false "启用状态过滤"
// @Param ssl query bool false "SSL状态过滤"
// @Param pattern query string false "站点名称模式匹配"
// @Param limit query int false "分页大小" default(20)
// @Param offset query int false "分页偏移" default(0)
// @Success 200 {object} utils.Response{data=object{sites=[]server.NginxSite,total=int}}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites [get]
func (h *APIHandler) ListSites(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	req := &server.NginxSiteListRequest{
		Pattern: c.Query("pattern"),
		Limit:   20,
		Offset:  0,
	}

	// 解析布尔参数
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			req.Enabled = &enabled
		}
	}

	if sslStr := c.Query("ssl"); sslStr != "" {
		if ssl, err := strconv.ParseBool(sslStr); err == nil {
			req.SSL = &ssl
		}
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

	sites, total, err := h.service.ListSites(c.Context(), serverID, req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取站点列表失败: "+err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"sites": sites,
		"total": total,
	})
}

// GetSite 获取站点信息
// @Summary 获取nginx站点详细信息
// @Description 获取指定站点的详细信息
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param siteName path string true "站点名称"
// @Success 200 {object} utils.Response{data=server.NginxSite}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites/{siteName} [get]
func (h *APIHandler) GetSite(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteName := c.Params("siteName")

	if serverID == "" || siteName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和站点名称不能为空")
	}

	site, err := h.service.GetSite(c.Context(), serverID, siteName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "获取站点信息失败: "+err.Error())
	}

	return utils.SuccessResponse(c, site)
}

// CreateSite 创建站点
// @Summary 创建nginx站点
// @Description 在指定服务器上创建新的nginx站点
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param site body server.NginxSiteCreateRequest true "站点配置"
// @Success 201 {object} utils.Response{data=server.NginxSite}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites [post]
func (h *APIHandler) CreateSite(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	var req server.NginxSiteCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	site, err := h.service.CreateSite(c.Context(), serverID, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建站点失败: "+err.Error())
	}

	return utils.SuccessResponse(c, site)
}

// UpdateSite 更新站点
// @Summary 更新nginx站点
// @Description 更新指定站点的配置
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param siteName path string true "站点名称"
// @Param site body server.NginxSiteUpdateRequest true "站点更新配置"
// @Success 200 {object} utils.Response{data=server.NginxSite}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites/{siteName} [put]
func (h *APIHandler) UpdateSite(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteName := c.Params("siteName")

	if serverID == "" || siteName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和站点名称不能为空")
	}

	var req server.NginxSiteUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求参数解析失败: "+err.Error())
	}

	site, err := h.service.UpdateSite(c.Context(), serverID, siteName, &req)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "更新站点失败: "+err.Error())
	}

	return utils.SuccessResponse(c, site)
}

// DeleteSite 删除站点
// @Summary 删除nginx站点
// @Description 删除指定的nginx站点
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param siteName path string true "站点名称"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites/{siteName} [delete]
func (h *APIHandler) DeleteSite(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteName := c.Params("siteName")

	if serverID == "" || siteName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和站点名称不能为空")
	}

	result, err := h.service.DeleteSite(c.Context(), serverID, siteName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除站点失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// EnableSite 启用站点
// @Summary 启用nginx站点
// @Description 启用指定的nginx站点
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param siteName path string true "站点名称"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites/{siteName}/enable [post]
func (h *APIHandler) EnableSite(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteName := c.Params("siteName")

	if serverID == "" || siteName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和站点名称不能为空")
	}

	result, err := h.service.EnableSite(c.Context(), serverID, siteName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "启用站点失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// DisableSite 禁用站点
// @Summary 禁用nginx站点
// @Description 禁用指定的nginx站点
// @Tags nginx
// @Accept json
// @Produce json
// @Param serverId path string true "服务器ID"
// @Param siteName path string true "站点名称"
// @Success 200 {object} utils.Response{data=server.NginxOperationResult}
// @Failure 400 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/nginx/servers/{serverId}/sites/{siteName}/disable [post]
func (h *APIHandler) DisableSite(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteName := c.Params("siteName")

	if serverID == "" || siteName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID和站点名称不能为空")
	}

	result, err := h.service.DisableSite(c.Context(), serverID, siteName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "禁用站点失败: "+err.Error())
	}

	return utils.SuccessResponse(c, result)
}
