// Package server 服务器管理 HTTP API 实现
package service

import (
	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/utils"
	"go.uber.org/zap"
)

// API 服务器管理 HTTP API
type API struct {
	service server.Service
	logger  *zap.Logger
}

// NewAPI 创建服务器管理 API
func NewAPI(service server.Service, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &API{
		service: service,
		logger:  logger.With(zap.String("component", "server_api")),
	}
}

// RegisterRoutes 注册服务器管理相关路由
func (a *API) RegisterRoutes(app fiber.Router, baseAuthHandler func(c *fiber.Ctx) error, adminRoleHandler func(c *fiber.Ctx) error) {
	// 服务器管理路由
	serverGroup := app.Group("/servers")
	serverGroup.Post("/", baseAuthHandler, a.CreateServer)
	serverGroup.Get("/", baseAuthHandler, a.ListServers)
	serverGroup.Get("/:id", baseAuthHandler, a.GetServer)
	serverGroup.Put("/:id", baseAuthHandler, a.UpdateServer)
	serverGroup.Delete("/:id", baseAuthHandler, a.DeleteServer)
	serverGroup.Post("/:id/test", baseAuthHandler, a.TestConnection)
	serverGroup.Post("/:id/health", baseAuthHandler, a.HealthCheck)
	serverGroup.Get("/:id/monitor-node", baseAuthHandler, a.GetMonitorNodeIP)
	serverGroup.Get("/:id/monitor-assignment", baseAuthHandler, a.GetMonitorAssignment)
	serverGroup.Post("/:id/monitor-reassign", baseAuthHandler, a.ReassignMonitorNode)
}

// CreateServer 创建服务器
func (a *API) CreateServer(c *fiber.Ctx) error {
	var req server.ServerCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	result, err := a.service.CreateServer(c.Context(), &req)
	if err != nil {
		a.logger.Error("创建服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// ListServers 获取服务器列表
func (a *API) ListServers(c *fiber.Ctx) error {
	req := &server.ServerListRequest{
		Limit:  20,
		Offset: 0,
	}

	// 解析查询参数
	if limit := c.QueryInt("limit", 20); limit > 0 && limit <= 100 {
		req.Limit = limit
	}
	if offset := c.QueryInt("offset", 0); offset >= 0 {
		req.Offset = offset
	}

	servers, total, err := a.service.ListServers(c.Context(), req)
	if err != nil {
		a.logger.Error("获取服务器列表失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"servers": servers,
		"total":   total,
	})
}

// GetServer 获取服务器详情
func (a *API) GetServer(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	server, err := a.service.GetServer(c.Context(), serverID)
	if err != nil {
		a.logger.Error("获取服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, server)
}

// UpdateServer 更新服务器
func (a *API) UpdateServer(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	var req server.ServerUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	result, err := a.service.UpdateServer(c.Context(), serverID, &req)
	if err != nil {
		a.logger.Error("更新服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// DeleteServer 删除服务器
func (a *API) DeleteServer(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	err := a.service.DeleteServer(c.Context(), serverID)
	if err != nil {
		a.logger.Error("删除服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]string{"message": "服务器删除成功"})
}

// TestConnection 测试连接
func (a *API) TestConnection(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	var req server.ServerTestConnectionRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	result, err := a.service.TestConnection(c.Context(), &req)
	if err != nil {
		a.logger.Error("测试连接失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// HealthCheck 健康检查
func (a *API) HealthCheck(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	result, err := a.service.PerformHealthCheck(c.Context(), serverID)
	if err != nil {
		a.logger.Error("健康检查失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// GetMonitorNodeIP 获取监控节点IP和端口
func (a *API) GetMonitorNodeIP(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	ip, port, err := a.service.GetMonitorNodeIP(c.Context(), serverID)
	if err != nil {
		a.logger.Error("获取监控节点IP失败", zap.String("server_id", serverID), zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]string{
		"ip":   ip,
		"port": port,
	})
}

// GetMonitorAssignment 获取监控分配信息
func (a *API) GetMonitorAssignment(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	assignment, err := a.service.GetMonitorAssignment(c.Context(), serverID)
	if err != nil {
		a.logger.Error("获取监控分配信息失败", zap.String("server_id", serverID), zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, assignment)
}

// ReassignMonitorNodeRequest 重新分配监控节点请求
type ReassignMonitorNodeRequest struct {
	NodeID string `json:"nodeId" validate:"required"` // 节点ID
}

// ReassignMonitorNode 重新分配监控节点
func (a *API) ReassignMonitorNode(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}

	var req ReassignMonitorNodeRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "解析请求参数失败")
	}

	if req.NodeID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "节点ID不能为空")
	}

	err := a.service.ReassignMonitorNode(c.Context(), serverID, req.NodeID)
	if err != nil {
		a.logger.Error("重新分配监控节点失败",
			zap.String("server_id", serverID),
			zap.String("node_id", req.NodeID),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, map[string]string{
		"message":  "监控节点重新分配成功",
		"serverId": serverID,
		"nodeId":   req.NodeID,
	})
}
