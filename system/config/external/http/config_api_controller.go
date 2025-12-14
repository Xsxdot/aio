package controller

import (
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/result"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/config/api/client"
	"xiaozhizhang/system/config/api/dto"
	"xiaozhizhang/utils"

	"github.com/gofiber/fiber/v2"
)

// ConfigAPIController 配置查询API控制器（对外接口）
type ConfigAPIController struct {
	client *client.ConfigClient
	err    *errorc.ErrorBuilder
}

// NewConfigAPIController 创建配置查询API控制器实例
func NewConfigAPIController(client *client.ConfigClient) *ConfigAPIController {
	return &ConfigAPIController{
		client: client,
		err:    errorc.NewErrorBuilder("ConfigAPIController"),
	}
}

// RegisterRoutes 注册路由
func (ctrl *ConfigAPIController) RegisterRoutes(api fiber.Router) {
	configRouter := api.Group("/configs")

	// 对外查询接口（不需要权限，可根据实际情况添加）
	configRouter.Post("/get", ctrl.GetConfig)
	configRouter.Post("/batch", ctrl.BatchGetConfigs)
}

// GetConfig 获取配置
func (ctrl *ConfigAPIController) GetConfig(ctx *fiber.Ctx) error {
	var req dto.GetConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx))
	}

	jsonStr, err := ctrl.client.GetConfigJSON(util.Context(ctx), req.Key, req.Env)
	return result.Once(ctx, jsonStr, err)
}

// BatchGetConfigs 批量获取配置
func (ctrl *ConfigAPIController) BatchGetConfigs(ctx *fiber.Ctx) error {
	var req dto.BatchGetConfigRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctrl.err.New("解析请求参数失败", err).WithTraceID(util.Context(ctx))
	}

	if errMsg, err := utils.Validate(&req); err != nil {
		return ctrl.err.New(errMsg, err).WithTraceID(util.Context(ctx))
	}

	configs, err := ctrl.client.GetConfigs(util.Context(ctx), req.Keys, req.Env)
	return result.Once(ctx, configs, err)
}
