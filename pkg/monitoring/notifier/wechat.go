// Package notifier 提供企业微信通知功能的实现
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"net/http"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// WeChatNotifier 企业微信通知器
type WeChatNotifier struct {
	config *models.WeChatNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的标题模板
const defaultWeChatTitleTemplate = "【{{.Severity}}】告警: {{.RuleName}}"

// 默认的内容模板
const defaultWeChatContentTemplate = `
告警详情:
- 告警描述: {{.Description}}
- 告警状态: {{if eq .EventType "triggered"}}已触发{{else}}已解决{{end}}
- 开始时间: {{formatTime .StartsAt}}
{{if eq .EventType "resolved"}}
- 解决时间: {{formatTime .EndsAt}}
{{end}}

指标信息:
- 指标名称: {{.Metric}}
- 当前值: {{.Value}}
- 阈值: {{.Threshold}}
- 条件: {{.Condition}}

{{if .Labels}}
标签:
{{range $key, $value := .Labels}}
- {{$key}}: {{$value}}
{{end}}
{{end}}
`

// NewWeChatNotifier 创建新的企业微信通知器
func NewWeChatNotifier(notifier *models.Notifier) (Notifier, error) {
	if notifier.Type != models.NotifierTypeWeChat {
		return nil, fmt.Errorf("通知器类型不是wechat: %s", notifier.Type)
	}

	// 解析配置
	var config models.WeChatNotifierConfig
	configData, err := json.Marshal(notifier.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("解析企业微信通知器配置失败: %w", err)
	}

	// 验证必要字段
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("企业微信Webhook URL不能为空")
	}

	// 设置默认模板
	if config.TitleTemplate == "" {
		config.TitleTemplate = defaultWeChatTitleTemplate
	}
	if config.ContentTemplate == "" {
		config.ContentTemplate = defaultWeChatContentTemplate
	}

	logger, _ := zap.NewProduction()

	return &WeChatNotifier{
		config: &config,
		logger: logger,
		id:     notifier.ID,
		name:   notifier.Name,
	}, nil
}

// Send 发送企业微信通知
func (n *WeChatNotifier) Send(alert *models.Alert, eventType string) (*models.NotificationResult, error) {
	result := &models.NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: models.NotifierTypeWeChat,
		Success:      false,
		Timestamp:    time.Now().Unix(),
	}

	// 准备渲染的数据
	data := struct {
		*models.Alert
		EventType string
		EndsAt    time.Time
	}{
		Alert:     alert,
		EventType: eventType,
	}

	if alert.EndsAt != nil {
		data.EndsAt = *alert.EndsAt
	}

	// 渲染标题
	titleTmpl, err := template.New("title").Parse(n.config.TitleTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析标题模板失败: %s", err.Error())
		return result, nil
	}

	var titleBuf bytes.Buffer
	if err := titleTmpl.Execute(&titleBuf, data); err != nil {
		result.Error = fmt.Sprintf("渲染标题模板失败: %s", err.Error())
		return result, nil
	}
	title := titleBuf.String()

	// 渲染内容
	contentTmpl := template.New("content").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
	})

	contentTmpl, err = contentTmpl.Parse(n.config.ContentTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析内容模板失败: %s", err.Error())
		return result, nil
	}

	var contentBuf bytes.Buffer
	if err := contentTmpl.Execute(&contentBuf, data); err != nil {
		result.Error = fmt.Sprintf("渲染内容模板失败: %s", err.Error())
		return result, nil
	}
	content := contentBuf.String()

	// 准备@人员
	var mentionedList []string
	if n.config.MentionAll {
		mentionedList = append(mentionedList, "@all")
	} else if len(n.config.MentionedUserIDs) > 0 {
		for _, uid := range n.config.MentionedUserIDs {
			mentionedList = append(mentionedList, "@"+uid)
		}
	}

	// 如果有@人员，添加到内容末尾
	if len(mentionedList) > 0 {
		content = content + "\n\n" + strings.Join(mentionedList, " ")
	}

	// 构造请求体
	requestBody := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("### %s\n%s", title, content),
		},
	}

	requestData, err := json.Marshal(requestBody)
	if err != nil {
		result.Error = fmt.Sprintf("序列化请求体失败: %s", err.Error())
		return result, nil
	}

	// 发送HTTP请求
	resp, err := http.Post(n.config.WebhookURL, "application/json", bytes.NewBuffer(requestData))
	if err != nil {
		result.Error = fmt.Sprintf("发送HTTP请求失败: %s", err.Error())
		return result, nil
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("HTTP请求返回非成功状态码: %d", resp.StatusCode)
		return result, nil
	}

	// 解析响应
	var responseData struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		result.Error = fmt.Sprintf("解析响应失败: %s", err.Error())
		return result, nil
	}

	// 检查业务状态码
	if responseData.ErrCode != 0 {
		result.Error = fmt.Sprintf("企业微信API返回错误: %s", responseData.ErrMsg)
		return result, nil
	}

	result.Success = true
	return result, nil
}
