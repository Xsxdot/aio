// Package notifier 提供钉钉通知功能的实现
package notifier

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"net/http"
	"net/url"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// DingTalkNotifier 钉钉通知器
type DingTalkNotifier struct {
	config *models2.DingTalkNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的标题模板
const defaultDingTalkTitleTemplate = "【{{.Severity}}】告警: {{.RuleName}}"

// 默认的内容模板 (Markdown格式)
const defaultDingTalkContentTemplate = `
### 告警详情

- **告警描述**: {{.Description}}
- **告警状态**: {{if eq .EventType "triggered"}}已触发{{else}}已解决{{end}}
- **开始时间**: {{formatTime .StartsAt}}
{{if eq .EventType "resolved"}}
- **解决时间**: {{formatTime .EndsAt}}
{{end}}

### 指标信息

- **指标名称**: {{.Metric}}
- **当前值**: {{.Value}}
- **阈值**: {{.Threshold}}
- **条件**: {{.Condition}}

{{if .Labels}}
### 标签

{{range $key, $value := .Labels}}
- **{{$key}}**: {{$value}}
{{end}}
{{end}}
`

// NewDingTalkNotifier 创建新的钉钉通知器
func NewDingTalkNotifier(notifier *models2.Notifier) (Notifier, error) {
	if notifier.Type != models2.NotifierTypeDingTalk {
		return nil, fmt.Errorf("通知器类型不是dingtalk: %s", notifier.Type)
	}

	// 解析配置
	var config models2.DingTalkNotifierConfig
	configData, err := json.Marshal(notifier.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("解析钉钉通知器配置失败: %w", err)
	}

	// 验证必要字段
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("钉钉Webhook URL不能为空")
	}

	// 设置默认值
	if config.TitleTemplate == "" {
		config.TitleTemplate = defaultDingTalkTitleTemplate
	}
	if config.ContentTemplate == "" {
		config.ContentTemplate = defaultDingTalkContentTemplate
	}
	if !config.UseMarkdown {
		config.UseMarkdown = true // 总是使用Markdown格式
	}

	logger, _ := zap.NewProduction()

	return &DingTalkNotifier{
		config: &config,
		logger: logger,
		id:     notifier.ID,
		name:   notifier.Name,
	}, nil
}

// Send 发送钉钉通知
func (n *DingTalkNotifier) Send(alert *models2.Alert, eventType string) (*models2.NotificationResult, error) {
	result := &models2.NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: models2.NotifierTypeDingTalk,
		Success:      false,
		Timestamp:    time.Now().Unix(),
	}

	// 准备渲染的数据
	data := struct {
		*models2.Alert
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

	// 构造请求体
	requestBody := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  content,
		},
	}

	// 添加@功能
	at := map[string]interface{}{}
	if n.config.AtAll {
		at["isAtAll"] = true
	} else if len(n.config.AtMobiles) > 0 {
		at["atMobiles"] = n.config.AtMobiles
	}

	if len(at) > 0 {
		requestBody["at"] = at
	}

	requestData, err := json.Marshal(requestBody)
	if err != nil {
		result.Error = fmt.Sprintf("序列化请求体失败: %s", err.Error())
		return result, nil
	}

	// 构造请求URL，添加签名
	webhookURL := n.config.WebhookURL
	if n.config.Secret != "" {
		webhookURL = n.addSignature(webhookURL)
	}

	// 发送HTTP请求
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(requestData))
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
		result.Error = fmt.Sprintf("钉钉API返回错误: %s", responseData.ErrMsg)
		return result, nil
	}

	result.Success = true
	return result, nil
}

// addSignature 为钉钉Webhook URL添加签名
func (n *DingTalkNotifier) addSignature(webhookURL string) string {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := timestamp + "\n" + n.config.Secret

	// 计算HMAC-SHA256签名
	h := hmac.New(sha256.New, []byte(n.config.Secret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 构建带签名的URL
	u, _ := url.Parse(webhookURL)
	query := u.Query()
	query.Set("timestamp", timestamp)
	query.Set("sign", signature)
	u.RawQuery = query.Encode()

	return u.String()
}
