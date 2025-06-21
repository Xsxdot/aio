// Package notifier 提供Webhook通知功能的实现
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"net/http"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	config *models.WebhookNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的请求体模板
const defaultWebhookBodyTemplate = `{
  "alert": {
    "id": "{{.ID}}",
    "rule_id": "{{.RuleID}}",
    "rule_name": "{{.RuleName}}",
    "metric": "{{.Metric}}",
    "value": {{.Value}},
    "threshold": {{.Threshold}},
    "condition": "{{.Condition}}",
    "severity": "{{.Severity}}",
    "state": "{{.State}}",
    "starts_at": "{{formatTime .StartsAt}}",
    {{if .EndsAt}}"ends_at": "{{formatTime .EndsAt}}",{{end}}
    "description": "{{.Description}}"
  },
  "event_type": "{{.EventType}}",
  "timestamp": "{{formatTime .Timestamp}}"
}`

// NewWebhookNotifier 创建新的Webhook通知器
func NewWebhookNotifier(notifier *models.Notifier) (Notifier, error) {
	if notifier.Type != models.NotifierTypeWebhook {
		return nil, fmt.Errorf("通知器类型不是webhook: %s", notifier.Type)
	}

	// 解析配置
	var config models.WebhookNotifierConfig
	configData, err := json.Marshal(notifier.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("解析Webhook通知器配置失败: %w", err)
	}

	// 验证必要字段
	if config.URL == "" {
		return nil, fmt.Errorf("Webhook URL不能为空")
	}

	// 设置默认值
	if config.Method == "" {
		config.Method = "POST"
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 10
	}
	if config.BodyTemplate == "" {
		config.BodyTemplate = defaultWebhookBodyTemplate
	}

	logger, _ := zap.NewProduction()

	return &WebhookNotifier{
		config: &config,
		logger: logger,
		id:     notifier.ID,
		name:   notifier.Name,
	}, nil
}

// Send 发送Webhook通知
func (n *WebhookNotifier) Send(alert *models.Alert, eventType string) (*models.NotificationResult, error) {
	result := &models.NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: models.NotifierTypeWebhook,
		Success:      false,
		Timestamp:    time.Now().Unix(),
	}

	// 准备渲染的数据
	data := struct {
		*models.Alert
		EventType string
		EndsAt    *time.Time
		Timestamp time.Time
	}{
		Alert:     alert,
		EventType: eventType,
		EndsAt:    alert.EndsAt,
		Timestamp: time.Now(),
	}

	// 渲染请求体
	bodyTmpl, err := template.New("body").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
	}).Parse(n.config.BodyTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析请求体模板失败: %s", err.Error())
		return result, nil
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
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

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for key, value := range n.config.Headers {
		req.Header.Set(key, value)
	}

	// 设置超时
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
		result.Error = fmt.Sprintf("HTTP请求返回非成功状态码: %d", resp.StatusCode)
		return result, nil
	}

	result.Success = true
	return result, nil
}
