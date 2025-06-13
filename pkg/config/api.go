package config

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// API 配置中心的HTTP API
type API struct {
	service *Service
	logger  *zap.Logger
}

// NewAPI 创建新的配置中心API
func NewAPI(service *Service, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &API{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes 注册所有API路由
func (api *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建API组
	configGroup := router.Group("/config")

	// 配置项基本操作
	configGroup.Get("/", authHandler, api.listConfigs)
	configGroup.Get("/:key", authHandler, api.getConfig)
	configGroup.Post("/", authHandler, adminRoleHandler, api.createConfig)
	configGroup.Put("/:key", authHandler, adminRoleHandler, api.updateConfig)
	configGroup.Delete("/:key", authHandler, adminRoleHandler, api.deleteConfig)

	// 配置历史相关
	configGroup.Get("/:key/history", authHandler, api.getConfigHistory)
	configGroup.Get("/:key/revision/:revision", authHandler, api.getConfigByRevision)

	// 环境相关
	configGroup.Get("/:key/environments", authHandler, api.listEnvironmentConfigs)
	configGroup.Get("/:key/env/:env", authHandler, api.getConfigForEnvironment)
	configGroup.Post("/:key/env/:env", authHandler, adminRoleHandler, api.setConfigForEnvironment)

	// 组合配置相关
	configGroup.Get("/:key/composite", authHandler, api.getCompositeConfig)
	configGroup.Get("/:key/composite/env/:env", authHandler, api.getCompositeConfigForEnvironment)
	configGroup.Post("/merge", authHandler, adminRoleHandler, api.mergeCompositeConfigs)
	configGroup.Post("/merge/env/:env", authHandler, adminRoleHandler, api.mergeCompositeConfigsForEnvironment)

	// 导出配置
	configGroup.Get("/:key/export", authHandler, api.exportConfigAsJSON)
	configGroup.Get("/:key/export/env/:env", authHandler, api.exportConfigAsJSONForEnvironment)

	// 导出导入所有配置
	configGroup.Post("/export", authHandler, adminRoleHandler, api.exportAllConfigs)
	configGroup.Post("/import", authHandler, adminRoleHandler, api.importAllConfigs)

	api.logger.Info("配置中心API路由已注册")
}

// listConfigs 列出所有配置项
func (api *API) listConfigs(c *fiber.Ctx) error {
	configs, err := api.service.List(c.Context())
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取配置列表失败: %v", err))
	}

	return utils.SuccessResponse(c, configs)
}

// getConfig 获取单个配置项
func (api *API) getConfig(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	config, err := api.service.Get(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取配置失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// createConfig 创建配置项
func (api *API) createConfig(c *fiber.Ctx) error {
	var request struct {
		Key      string                  `json:"key"`
		Value    map[string]*ConfigValue `json:"value"`
		Metadata map[string]string       `json:"metadata"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if request.Key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	if len(request.Value) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置值不能为空")
	}

	err := api.service.Set(c.Context(), request.Key, request.Value, request.Metadata)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("创建配置失败: %v", err))
	}

	// 获取新创建的配置
	config, _ := api.service.Get(c.Context(), request.Key)

	return utils.SuccessResponse(c, config)
}

// updateConfig 更新配置项
func (api *API) updateConfig(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	var request struct {
		Value    map[string]*ConfigValue `json:"value"`
		Metadata map[string]string       `json:"metadata"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if len(request.Value) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置值不能为空")
	}

	// 检查配置是否存在
	_, err := api.service.Get(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("配置不存在: %v", err))
	}

	err = api.service.Set(c.Context(), key, request.Value, request.Metadata)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("更新配置失败: %v", err))
	}

	// 获取更新后的配置
	config, _ := api.service.Get(c.Context(), key)

	return utils.SuccessResponse(c, config)
}

// deleteConfig 删除配置项
func (api *API) deleteConfig(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	// 检查配置是否存在
	_, err := api.service.Get(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("配置不存在: %v", err))
	}

	err = api.service.Delete(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("删除配置失败: %v", err))
	}

	return utils.SuccessResponse(c, "配置删除成功")
}

// getConfigHistory 获取配置项历史版本
func (api *API) getConfigHistory(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	limitStr := c.Query("limit", "10")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的limit参数")
	}

	history, err := api.service.GetHistory(c.Context(), key, limit)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取配置历史失败: %v", err))
	}

	return utils.SuccessResponse(c, history)
}

// getConfigByRevision 根据修订版本获取配置项
func (api *API) getConfigByRevision(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	revisionStr := c.Params("revision")
	revision, err := strconv.ParseInt(revisionStr, 10, 64)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的修订版本号")
	}

	config, err := api.service.GetByRevision(c.Context(), key, revision)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取配置修订版本失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// listEnvironmentConfigs 列出配置项在所有环境中的版本
func (api *API) listEnvironmentConfigs(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	envConfigs, err := api.service.ListEnvironmentConfigs(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取环境配置列表失败: %v", err))
	}

	return utils.SuccessResponse(c, envConfigs)
}

// getConfigForEnvironment 获取特定环境的配置项
func (api *API) getConfigForEnvironment(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	env := c.Params("env")
	if env == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "环境参数不能为空")
	}

	// 构建环境配置
	fallbacksParam := c.Query("fallbacks")
	var fallbacks []string
	if fallbacksParam != "" {
		err := json.Unmarshal([]byte(fallbacksParam), &fallbacks)
		if err != nil {
			fallbacks = DefaultEnvironmentFallbacks(env)
		}
	} else {
		fallbacks = DefaultEnvironmentFallbacks(env)
	}

	envConfig := NewEnvironmentConfig(env, fallbacks...)
	config, err := api.service.GetForEnvironment(c.Context(), key, envConfig)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取环境配置失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// setConfigForEnvironment 为特定环境设置配置项
func (api *API) setConfigForEnvironment(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	env := c.Params("env")
	if env == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "环境参数不能为空")
	}

	var request struct {
		Value    map[string]*ConfigValue `json:"value"`
		Metadata map[string]string       `json:"metadata"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if len(request.Value) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置值不能为空")
	}

	err := api.service.SetForEnvironment(c.Context(), key, env, request.Value, request.Metadata)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("设置环境配置失败: %v", err))
	}

	// 构建环境配置
	envConfig := NewEnvironmentConfig(env)
	config, _ := api.service.GetForEnvironment(c.Context(), key, envConfig)

	return utils.SuccessResponse(c, config)
}

// getCompositeConfig 获取组合配置
func (api *API) getCompositeConfig(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	config, err := api.service.GetCompositeConfig(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取组合配置失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// getCompositeConfigForEnvironment 获取特定环境的组合配置
func (api *API) getCompositeConfigForEnvironment(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	env := c.Params("env")
	if env == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "环境参数不能为空")
	}

	// 构建环境配置
	fallbacksParam := c.Query("fallbacks")
	var fallbacks []string
	if fallbacksParam != "" {
		err := json.Unmarshal([]byte(fallbacksParam), &fallbacks)
		if err != nil {
			fallbacks = DefaultEnvironmentFallbacks(env)
		}
	} else {
		fallbacks = DefaultEnvironmentFallbacks(env)
	}

	envConfig := NewEnvironmentConfig(env, fallbacks...)
	config, err := api.service.GetCompositeConfigForEnvironment(c.Context(), key, envConfig)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取环境组合配置失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// mergeCompositeConfigs 合并多个组合配置
func (api *API) mergeCompositeConfigs(c *fiber.Ctx) error {
	var request struct {
		Keys []string `json:"keys"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if len(request.Keys) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "至少需要一个配置键")
	}

	config, err := api.service.MergeCompositeConfigs(c.Context(), request.Keys)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("合并组合配置失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// mergeCompositeConfigsForEnvironment 合并多个特定环境的组合配置
func (api *API) mergeCompositeConfigsForEnvironment(c *fiber.Ctx) error {
	env := c.Params("env")
	if env == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "环境参数不能为空")
	}

	var request struct {
		Keys []string `json:"keys"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if len(request.Keys) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "至少需要一个配置键")
	}

	// 构建环境配置
	fallbacksParam := c.Query("fallbacks")
	var fallbacks []string
	if fallbacksParam != "" {
		err := json.Unmarshal([]byte(fallbacksParam), &fallbacks)
		if err != nil {
			fallbacks = DefaultEnvironmentFallbacks(env)
		}
	} else {
		fallbacks = DefaultEnvironmentFallbacks(env)
	}

	envConfig := NewEnvironmentConfig(env, fallbacks...)
	config, err := api.service.MergeCompositeConfigsForEnvironment(c.Context(), request.Keys, envConfig)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("合并环境组合配置失败: %v", err))
	}

	return utils.SuccessResponse(c, config)
}

// exportConfigAsJSON 将配置导出为JSON
func (api *API) exportConfigAsJSON(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	jsonStr, err := api.service.ExportConfigAsJSON(c.Context(), key)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("导出配置失败: %v", err))
	}

	// 根据Accept头决定返回格式
	if c.Accepts("application/json") == "application/json" {
		return utils.SuccessResponse(c, json.RawMessage(jsonStr))
	}

	// 直接返回JSON字符串
	c.Set("Content-Type", "application/json")
	return c.SendString(jsonStr)
}

// exportConfigAsJSONForEnvironment 将特定环境的配置导出为JSON
func (api *API) exportConfigAsJSONForEnvironment(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置键不能为空")
	}

	env := c.Params("env")
	if env == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "环境参数不能为空")
	}

	// 构建环境配置
	fallbacksParam := c.Query("fallbacks")
	var fallbacks []string
	if fallbacksParam != "" {
		err := json.Unmarshal([]byte(fallbacksParam), &fallbacks)
		if err != nil {
			fallbacks = DefaultEnvironmentFallbacks(env)
		}
	} else {
		fallbacks = DefaultEnvironmentFallbacks(env)
	}

	envConfig := NewEnvironmentConfig(env, fallbacks...)
	jsonStr, err := api.service.ExportConfigAsJSONForEnvironment(c.Context(), key, envConfig)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("导出环境配置失败: %v", err))
	}

	// 根据Accept头决定返回格式
	if c.Accepts("application/json") == "application/json" {
		return utils.SuccessResponse(c, json.RawMessage(jsonStr))
	}

	// 直接返回JSON字符串
	c.Set("Content-Type", "application/json")
	return c.SendString(jsonStr)
}

// exportAllConfigs 导出所有配置
func (api *API) exportAllConfigs(c *fiber.Ctx) error {
	var request struct {
		Password string `json:"password"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if request.Password == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "加密密码不能为空")
	}

	// 导出所有配置并以Base64格式返回
	base64Data, err := api.service.ExportAllConfigsBase64(c.Context(), request.Password)
	if err != nil {
		api.logger.Error("导出所有配置失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("导出配置失败: %v", err))
	}

	// 返回统一的JSON响应格式
	return utils.SuccessResponse(c, base64Data)
}

// importAllConfigs 导入所有配置
func (api *API) importAllConfigs(c *fiber.Ctx) error {
	var request struct {
		Data         string `json:"data"`
		Password     string `json:"password"`
		SkipExisting bool   `json:"skipExisting"`
		OriginalSalt string `json:"originalSalt"`
	}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
	}

	if request.Data == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "导入数据不能为空")
	}

	if request.Password == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "解密密码不能为空")
	}

	// 默认跳过已存在的配置
	skipExisting := true
	if c.Body() != nil && !request.SkipExisting {
		skipExisting = false
	}

	api.logger.Info("导入配置",
		zap.Bool("skipExisting", skipExisting),
		zap.Bool("hasOriginalSalt", request.OriginalSalt != ""))

	// 从Base64字符串导入所有配置
	skippedConfigs, err := api.service.ImportAllConfigsBase64(c.Context(), request.Data, request.Password, skipExisting, request.OriginalSalt)
	if err != nil {
		api.logger.Error("导入所有配置失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("导入配置失败: %v", err))
	}

	api.logger.Info("导入配置完成",
		zap.Int("skipped_count", len(skippedConfigs)),
		zap.Strings("skipped_configs", skippedConfigs))

	// 确保返回空数组而不是null
	if skippedConfigs == nil {
		skippedConfigs = []string{}
	}

	// 返回导入结果，包括跳过的配置列表
	result := map[string]interface{}{
		"message": "配置导入成功",
		"skipped": skippedConfigs,
	}

	// 由于返回数据可能比较大，添加详细的日志
	resultJSON, _ := json.Marshal(result)
	api.logger.Info("返回导入结果", zap.String("result", string(resultJSON)))

	return utils.SuccessResponse(c, result)
}
