// Package notifier 提供通知器的HTTP API接口
package notifier

import (
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// API 通知器系统的HTTP API
type API struct {
	manager *Manager
	logger  *zap.Logger
}

// NewAPI 创建新的通知器API
func NewAPI(manager *Manager, logger *zap.Logger) *API {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &API{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册所有API路由
func (api *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建通知器API组
	notifierGroup := router.Group("/notifiers")

	// 通知器CRUD操作
	notifierGroup.Get("/", authHandler, api.getNotifiers)
	notifierGroup.Get("/:id", authHandler, api.getNotifier)
	notifierGroup.Post("/", authHandler, adminRoleHandler, api.createNotifier)
	notifierGroup.Put("/:id", authHandler, adminRoleHandler, api.updateNotifier)
	notifierGroup.Delete("/:id", authHandler, adminRoleHandler, api.deleteNotifier)

	// 通知器测试和操作
	notifierGroup.Post("/:id/test", authHandler, adminRoleHandler, api.testNotifier)
	notifierGroup.Post("/:id/enable", authHandler, adminRoleHandler, api.enableNotifier)
	notifierGroup.Post("/:id/disable", authHandler, adminRoleHandler, api.disableNotifier)

	// 批量操作
	notifierGroup.Post("/batch/test", authHandler, adminRoleHandler, api.batchTestNotifiers)
	notifierGroup.Post("/batch/enable", authHandler, adminRoleHandler, api.batchEnableNotifiers)
	notifierGroup.Post("/batch/disable", authHandler, adminRoleHandler, api.batchDisableNotifiers)

	// 通知器类型和模板
	notifierGroup.Get("/types", authHandler, api.getSupportedTypes)
	notifierGroup.Get("/templates/:type", authHandler, api.getNotifierTemplate)

	// 发送通知
	notifierGroup.Post("/send", authHandler, adminRoleHandler, api.sendNotification)

	api.logger.Info("通知器API路由已注册")
}

// getNotifiers 获取所有通知器
func (api *API) getNotifiers(c *fiber.Ctx) error {
	// 支持查询参数过滤
	typeFilter := c.Query("type", "")
	enabledFilter := c.Query("enabled", "")

	configs := api.manager.GetNotifiers()

	// 应用过滤器
	var filtered []*NotifierConfig
	for _, config := range configs {
		// 类型过滤
		if typeFilter != "" && string(config.Type) != typeFilter {
			continue
		}

		// 启用状态过滤
		if enabledFilter != "" {
			if enabledFilter == "true" && !config.Enabled {
				continue
			}
			if enabledFilter == "false" && config.Enabled {
				continue
			}
		}

		filtered = append(filtered, config)
	}

	api.logger.Info("获取通知器列表",
		zap.Int("total_count", len(configs)),
		zap.Int("filtered_count", len(filtered)),
		zap.String("type_filter", typeFilter),
		zap.String("enabled_filter", enabledFilter))

	return utils.SuccessResponse(c, filtered)
}

// getNotifier 获取特定通知器
func (api *API) getNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器ID不能为空")
	}

	config, err := api.manager.GetNotifier(id)
	if err != nil {
		api.logger.Warn("获取通知器失败",
			zap.String("id", id),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("获取通知器失败: %v", err))
	}

	api.logger.Info("获取通知器成功", zap.String("id", id), zap.String("name", config.Name))
	return utils.SuccessResponse(c, config)
}

// createNotifier 创建通知器
func (api *API) createNotifier(c *fiber.Ctx) error {
	config := new(NotifierConfig)
	if err := c.BodyParser(config); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 验证必填字段
	if config.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器名称不能为空")
	}
	if config.Type == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器类型不能为空")
	}
	if config.Config == nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器配置不能为空")
	}

	// 检查ID是否为空，如果为空则生成UUID
	if config.ID == "" {
		config.ID = api.generateNotifierID(config.Name, string(config.Type))
	}

	// 设置创建时间
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now

	if err := api.manager.CreateNotifier(config); err != nil {
		api.logger.Error("创建通知器失败",
			zap.String("name", config.Name),
			zap.String("type", string(config.Type)),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("创建通知器失败: %v", err))
	}

	api.logger.Info("创建通知器成功",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("type", string(config.Type)))

	return utils.SuccessResponse(c, config)
}

// updateNotifier 更新通知器
func (api *API) updateNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器ID不能为空")
	}

	config := new(NotifierConfig)
	if err := c.BodyParser(config); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 确保ID匹配
	config.ID = id
	config.UpdatedAt = time.Now()

	// 验证必填字段
	if config.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器名称不能为空")
	}
	if config.Type == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器类型不能为空")
	}
	if config.Config == nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器配置不能为空")
	}

	if err := api.manager.UpdateNotifier(config); err != nil {
		api.logger.Error("更新通知器失败",
			zap.String("id", id),
			zap.String("name", config.Name),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("更新通知器失败: %v", err))
	}

	api.logger.Info("更新通知器成功",
		zap.String("id", id),
		zap.String("name", config.Name))

	return utils.SuccessResponse(c, config)
}

// deleteNotifier 删除通知器
func (api *API) deleteNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器ID不能为空")
	}

	if err := api.manager.DeleteNotifier(id); err != nil {
		api.logger.Error("删除通知器失败",
			zap.String("id", id),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("删除通知器失败: %v", err))
	}

	api.logger.Info("删除通知器成功", zap.String("id", id))
	return c.SendStatus(fiber.StatusNoContent)
}

// testNotifier 测试通知器
func (api *API) testNotifier(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器ID不能为空")
	}

	// 获取通知器配置，仅检查存在性
	config, err := api.manager.GetNotifier(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("通知器不存在: %v", err))
	}

	// 解析请求体，允许自定义测试消息
	testRequest := struct {
		Title   string                 `json:"title,omitempty"`
		Content string                 `json:"content,omitempty"`
		Level   NotificationLevel      `json:"level,omitempty"`
		Data    map[string]interface{} `json:"data,omitempty"`
	}{}

	if err := c.BodyParser(&testRequest); err != nil {
		// 如果解析失败，使用默认测试消息
		api.logger.Debug("使用默认测试消息", zap.Error(err))
	}

	// 创建测试通知
	now := time.Now()
	testNotification := &Notification{
		ID:        fmt.Sprintf("test-%d", now.Unix()),
		Title:     testRequest.Title,
		Content:   testRequest.Content,
		Level:     testRequest.Level,
		CreatedAt: now,
		Data:      testRequest.Data,
	}

	// 设置默认值
	if testNotification.Title == "" {
		testNotification.Title = "通知器测试消息"
	}
	if testNotification.Content == "" {
		testNotification.Content = fmt.Sprintf("这是一个测试通知，用于验证通知器 %s (%s) 的配置是否正确。\n发送时间：%s",
			config.Name, config.Type, now.Format("2006-01-02 15:04:05"))
	}
	if testNotification.Level == "" {
		testNotification.Level = NotificationLevelInfo
	}
	if testNotification.Data == nil {
		testNotification.Data = map[string]interface{}{
			"notifier_id":   id,
			"notifier_name": config.Name,
			"notifier_type": config.Type,
			"test_time":     now.Unix(),
		}
	}

	// 发送测试通知
	results := api.manager.SendNotification(testNotification)

	// 记录测试结果
	success := false
	for _, result := range results {
		if result.NotifierID == id && result.Success {
			success = true
			break
		}
	}

	if success {
		api.logger.Info("通知器测试成功",
			zap.String("id", id),
			zap.String("name", config.Name))
	} else {
		api.logger.Warn("通知器测试失败",
			zap.String("id", id),
			zap.String("name", config.Name))
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "测试通知已发送",
		"results": results,
	})
}

// enableNotifier 启用通知器
func (api *API) enableNotifier(c *fiber.Ctx) error {
	return api.toggleNotifier(c, true)
}

// disableNotifier 禁用通知器
func (api *API) disableNotifier(c *fiber.Ctx) error {
	return api.toggleNotifier(c, false)
}

// toggleNotifier 切换通知器启用状态
func (api *API) toggleNotifier(c *fiber.Ctx, enabled bool) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器ID不能为空")
	}

	// 获取当前配置
	config, err := api.manager.GetNotifier(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("通知器不存在: %v", err))
	}

	// 更新启用状态
	config.Enabled = enabled
	config.UpdatedAt = time.Now()

	if err := api.manager.UpdateNotifier(config); err != nil {
		api.logger.Error("切换通知器状态失败",
			zap.String("id", id),
			zap.Bool("enabled", enabled),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("切换通知器状态失败: %v", err))
	}

	action := "禁用"
	if enabled {
		action = "启用"
	}

	api.logger.Info("通知器状态切换成功",
		zap.String("id", id),
		zap.String("action", action))

	return utils.SuccessResponse(c, config)
}

// batchTestNotifiers 批量测试通知器
func (api *API) batchTestNotifiers(c *fiber.Ctx) error {
	request := struct {
		IDs []string `json:"ids"`
	}{}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	if len(request.IDs) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须提供至少一个通知器ID")
	}

	// 创建测试通知
	now := time.Now()
	testNotification := &Notification{
		ID:        fmt.Sprintf("batch-test-%d", now.Unix()),
		Title:     "批量测试通知",
		Content:   fmt.Sprintf("这是一个批量测试通知，发送时间：%s", now.Format("2006-01-02 15:04:05")),
		Level:     NotificationLevelInfo,
		CreatedAt: now,
		Data: map[string]interface{}{
			"test_type": "batch",
			"test_time": now.Unix(),
		},
	}

	// 发送测试通知
	results := api.manager.SendNotification(testNotification)

	// 过滤结果，只返回请求的通知器结果
	filteredResults := make([]NotificationResult, 0)
	for _, result := range results {
		for _, id := range request.IDs {
			if result.NotifierID == id {
				filteredResults = append(filteredResults, result)
				break
			}
		}
	}

	api.logger.Info("批量测试通知器完成",
		zap.Int("requested_count", len(request.IDs)),
		zap.Int("result_count", len(filteredResults)))

	return utils.SuccessResponse(c, fiber.Map{
		"message": "批量测试通知已发送",
		"results": filteredResults,
	})
}

// batchEnableNotifiers 批量启用通知器
func (api *API) batchEnableNotifiers(c *fiber.Ctx) error {
	return api.batchToggleNotifiers(c, true)
}

// batchDisableNotifiers 批量禁用通知器
func (api *API) batchDisableNotifiers(c *fiber.Ctx) error {
	return api.batchToggleNotifiers(c, false)
}

// batchToggleNotifiers 批量切换通知器状态
func (api *API) batchToggleNotifiers(c *fiber.Ctx, enabled bool) error {
	request := struct {
		IDs []string `json:"ids"`
	}{}

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	if len(request.IDs) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "必须提供至少一个通知器ID")
	}

	results := make([]fiber.Map, 0, len(request.IDs))
	successCount := 0

	for _, id := range request.IDs {
		result := fiber.Map{
			"id":      id,
			"success": false,
		}

		// 获取配置
		config, err := api.manager.GetNotifier(id)
		if err != nil {
			result["error"] = fmt.Sprintf("通知器不存在: %v", err)
			results = append(results, result)
			continue
		}

		// 更新状态
		config.Enabled = enabled
		config.UpdatedAt = time.Now()

		if err := api.manager.UpdateNotifier(config); err != nil {
			result["error"] = fmt.Sprintf("更新失败: %v", err)
			results = append(results, result)
			continue
		}

		result["success"] = true
		results = append(results, result)
		successCount++
	}

	action := "禁用"
	if enabled {
		action = "启用"
	}

	api.logger.Info("批量切换通知器状态完成",
		zap.String("action", action),
		zap.Int("total_count", len(request.IDs)),
		zap.Int("success_count", successCount))

	return utils.SuccessResponse(c, fiber.Map{
		"message": fmt.Sprintf("批量%s操作完成", action),
		"results": results,
		"summary": fiber.Map{
			"total":   len(request.IDs),
			"success": successCount,
			"failed":  len(request.IDs) - successCount,
		},
	})
}

// getSupportedTypes 获取支持的通知器类型
func (api *API) getSupportedTypes(c *fiber.Ctx) error {
	supportedTypes := []fiber.Map{
		{
			"type":        NotifierTypeEmail,
			"name":        "邮件通知",
			"description": "通过SMTP发送邮件通知",
			"icon":        "mail",
		},
		{
			"type":        NotifierTypeWebhook,
			"name":        "Webhook通知",
			"description": "通过HTTP请求发送通知到指定URL",
			"icon":        "link",
		},
		{
			"type":        NotifierTypeWeChat,
			"name":        "企业微信通知",
			"description": "通过企业微信机器人发送通知",
			"icon":        "wechat",
		},
		{
			"type":        NotifierTypeDingTalk,
			"name":        "钉钉通知",
			"description": "通过钉钉机器人发送通知",
			"icon":        "dingtalk",
		},
	}

	return utils.SuccessResponse(c, supportedTypes)
}

// getNotifierTemplate 获取通知器配置模板
func (api *API) getNotifierTemplate(c *fiber.Ctx) error {
	notifierType := c.Params("type")
	if notifierType == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知器类型不能为空")
	}

	var template interface{}
	var description string

	switch NotifierType(notifierType) {
	case NotifierTypeEmail:
		template = EmailNotifierConfig{
			Recipients:      []string{"user@example.com"},
			SMTPServer:      "smtp.example.com",
			SMTPPort:        587,
			SMTPUsername:    "username",
			SMTPPassword:    "password",
			UseTLS:          true,
			FromAddress:     "noreply@example.com",
			SubjectTemplate: "{{ .Title }}",
			BodyTemplate:    "{{ .Content }}",
		}
		description = "邮件通知器配置模板"

	case NotifierTypeWebhook:
		template = WebhookNotifierConfig{
			URL:            "https://your-webhook-url.com/notify",
			Method:         "POST",
			Headers:        map[string]string{"Content-Type": "application/json"},
			BodyTemplate:   `{"title": "{{ .Title }}", "content": "{{ .Content }}", "level": "{{ .Level }}"}`,
			TimeoutSeconds: 30,
		}
		description = "Webhook通知器配置模板"

	case NotifierTypeWeChat:
		template = WeChatNotifierConfig{
			WebhookURL:       "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY",
			TitleTemplate:    "{{ .Title }}",
			ContentTemplate:  "{{ .Content }}",
			MentionedUserIDs: []string{"@all"},
			MentionAll:       false,
		}
		description = "企业微信通知器配置模板"

	case NotifierTypeDingTalk:
		template = DingTalkNotifierConfig{
			WebhookURL:      "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN",
			Secret:          "YOUR_SECRET",
			TitleTemplate:   "{{ .Title }}",
			ContentTemplate: "{{ .Content }}",
			UseMarkdown:     true,
			AtMobiles:       []string{},
			AtAll:           false,
		}
		description = "钉钉通知器配置模板"

	default:
		return utils.FailResponse(c, utils.StatusBadRequest, "不支持的通知器类型")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"type":        notifierType,
		"description": description,
		"template":    template,
	})
}

// sendNotification 发送通知
func (api *API) sendNotification(c *fiber.Ctx) error {
	notification := new(Notification)
	if err := c.BodyParser(notification); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无法解析请求体: %v", err))
	}

	// 验证必填字段
	if notification.Title == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知标题不能为空")
	}
	if notification.Content == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "通知内容不能为空")
	}

	// 设置默认值
	if notification.ID == "" {
		notification.ID = fmt.Sprintf("manual-%d", time.Now().Unix())
	}
	if notification.Level == "" {
		notification.Level = NotificationLevelInfo
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	// 发送通知
	results := api.manager.SendNotification(notification)

	// 统计结果
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	api.logger.Info("手动发送通知完成",
		zap.String("notification_id", notification.ID),
		zap.String("title", notification.Title),
		zap.Int("total_notifiers", len(results)),
		zap.Int("success_count", successCount))

	return utils.SuccessResponse(c, fiber.Map{
		"message": "通知已发送",
		"results": results,
		"summary": fiber.Map{
			"total":   len(results),
			"success": successCount,
			"failed":  len(results) - successCount,
		},
	})
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
