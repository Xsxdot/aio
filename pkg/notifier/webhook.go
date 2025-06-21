// Package notifier 提供Webhook通知功能的实现
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	config *WebhookNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的请求体模板
const defaultWebhookBodyTemplate = `{
  "id": "{{.ID}}",
  "title": "{{.Title}}",
  "content": "{{.Content}}",
  "level": "{{.Level}}",
  "created_at": "{{formatTime .CreatedAt}}",
  "labels": {{toJSON .Labels}},
  "data": {{toJSON .Data}}
}`

// NewWebhookNotifier 创建新的Webhook通知器
func NewWebhookNotifier(config *NotifierConfig) (Notifier, error) {
	if config.Type != NotifierTypeWebhook {
		return nil, fmt.Errorf("通知器类型不是webhook: %s", config.Type)
	}

	// 解析配置
	var webhookConfig WebhookNotifierConfig
	configData, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &webhookConfig); err != nil {
		return nil, fmt.Errorf("解析Webhook通知器配置失败: %w", err)
	}

	// 验证必要字段
	if webhookConfig.URL == "" {
		return nil, fmt.Errorf("Webhook URL不能为空")
	}

	// 设置默认值
	if webhookConfig.Method == "" {
		webhookConfig.Method = "POST"
	}
	if webhookConfig.BodyTemplate == "" {
		webhookConfig.BodyTemplate = defaultWebhookBodyTemplate
	}
	if webhookConfig.TimeoutSeconds == 0 {
		webhookConfig.TimeoutSeconds = 30
	}

	logger, _ := zap.NewProduction()

	return &WebhookNotifier{
		config: &webhookConfig,
		logger: logger,
		id:     config.ID,
		name:   config.Name,
	}, nil
}

// Send 发送Webhook通知
func (n *WebhookNotifier) Send(notification *Notification) (*NotificationResult, error) {
	result := &NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: NotifierTypeWebhook,
		Success:      false,
		Timestamp:    time.Now().Unix(),
	}

	// 渲染请求体
	bodyTmpl := template.New("body").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02T15:04:05Z07:00")
		},
		"toJSON": func(v interface{}) string {
			if v == nil {
				return "null"
			}
			b, err := json.Marshal(v)
			if err != nil {
				return "null"
			}
			return string(b)
		},
	})

	bodyTmpl, err := bodyTmpl.Parse(n.config.BodyTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析请求体模板失败: %s", err.Error())
		return result, nil
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, notification); err != nil {
		result.Error = fmt.Sprintf("渲染请求体模板失败: %s", err.Error())
		return result, nil
	}
	body := bodyBuf.String()

	// 创建HTTP请求
	req, err := http.NewRequest(n.config.Method, n.config.URL, bytes.NewBufferString(body))
	if err != nil {
		result.Error = fmt.Sprintf("创建HTTP请求失败: %s", err.Error())
		return result, nil
	}

	// 设置默认请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AIO-Notifier/1.0")

	// 设置自定义请求头
	for key, value := range n.config.Headers {
		req.Header.Set(key, value)
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: time.Duration(n.config.TimeoutSeconds) * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("发送HTTP请求失败: %s", err.Error())
		return result, nil
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("HTTP响应状态异常: %d", resp.StatusCode)
		return result, nil
	}

	result.Success = true
	n.logger.Info("Webhook通知发送成功",
		zap.String("id", notification.ID),
		zap.String("title", notification.Title),
		zap.String("url", n.config.URL),
		zap.Int("status_code", resp.StatusCode))

	return result, nil
}

// GetType 获取通知器类型
func (n *WebhookNotifier) GetType() NotifierType {
	return NotifierTypeWebhook
}

// GetID 获取通知器ID
func (n *WebhookNotifier) GetID() string {
	return n.id
}

// GetName 获取通知器名称
func (n *WebhookNotifier) GetName() string {
	return n.name
}
