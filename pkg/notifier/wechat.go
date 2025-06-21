// Package notifier 提供企业微信通知功能的实现
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

// WeChatNotifier 企业微信通知器
type WeChatNotifier struct {
	config *WeChatNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的标题模板
const defaultWeChatTitleTemplate = "【{{.Level}}】{{.Title}}"

// 默认的内容模板
const defaultWeChatContentTemplate = `通知详情:
标题: {{.Title}}
级别: {{.Level}}
内容: {{.Content}}
时间: {{formatTime .CreatedAt}}

{{if .Labels}}标签信息:
{{range $key, $value := .Labels}}{{$key}}: {{$value}}
{{end}}{{end}}

{{if .Data}}附加数据:
{{range $key, $value := .Data}}{{$key}}: {{$value}}
{{end}}{{end}}`

// NewWeChatNotifier 创建新的企业微信通知器
func NewWeChatNotifier(config *NotifierConfig) (Notifier, error) {
	if config.Type != NotifierTypeWeChat {
		return nil, fmt.Errorf("通知器类型不是wechat: %s", config.Type)
	}

	// 解析配置
	var wechatConfig WeChatNotifierConfig
	configData, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &wechatConfig); err != nil {
		return nil, fmt.Errorf("解析企业微信通知器配置失败: %w", err)
	}

	// 验证必要字段
	if wechatConfig.WebhookURL == "" {
		return nil, fmt.Errorf("企业微信Webhook URL不能为空")
	}

	// 设置默认值
	if wechatConfig.TitleTemplate == "" {
		wechatConfig.TitleTemplate = defaultWeChatTitleTemplate
	}
	if wechatConfig.ContentTemplate == "" {
		wechatConfig.ContentTemplate = defaultWeChatContentTemplate
	}

	logger, _ := zap.NewProduction()

	return &WeChatNotifier{
		config: &wechatConfig,
		logger: logger,
		id:     config.ID,
		name:   config.Name,
	}, nil
}

// Send 发送企业微信通知
func (n *WeChatNotifier) Send(notification *Notification) (*NotificationResult, error) {
	result := &NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: NotifierTypeWeChat,
		Success:      false,
		Timestamp:    time.Now().Unix(),
	}

	// 渲染标题
	titleTmpl, err := template.New("title").Parse(n.config.TitleTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析标题模板失败: %s", err.Error())
		return result, nil
	}

	var titleBuf bytes.Buffer
	if err := titleTmpl.Execute(&titleBuf, notification); err != nil {
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
	if err := contentTmpl.Execute(&contentBuf, notification); err != nil {
		result.Error = fmt.Sprintf("渲染内容模板失败: %s", err.Error())
		return result, nil
	}
	content := contentBuf.String()

	// 构造请求体
	requestBody := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": fmt.Sprintf("%s\n\n%s", title, content),
		},
	}

	// 添加@功能
	if n.config.MentionAll || len(n.config.MentionedUserIDs) > 0 {
		requestBody["text"].(map[string]interface{})["mentioned_list"] = n.config.MentionedUserIDs
		if n.config.MentionAll {
			requestBody["text"].(map[string]interface{})["mentioned_mobile_list"] = []string{"@all"}
		}
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
		result.Error = fmt.Sprintf("HTTP响应状态异常: %d", resp.StatusCode)
		return result, nil
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		result.Error = fmt.Sprintf("解析响应失败: %s", err.Error())
		return result, nil
	}

	// 检查企业微信接口返回
	if errCode, ok := response["errcode"]; ok && errCode.(float64) != 0 {
		errMsg := response["errmsg"]
		result.Error = fmt.Sprintf("企业微信接口返回错误: %v", errMsg)
		return result, nil
	}

	result.Success = true
	n.logger.Info("企业微信通知发送成功",
		zap.String("id", notification.ID),
		zap.String("title", notification.Title))

	return result, nil
}

// GetType 获取通知器类型
func (n *WeChatNotifier) GetType() NotifierType {
	return NotifierTypeWeChat
}

// GetID 获取通知器ID
func (n *WeChatNotifier) GetID() string {
	return n.id
}

// GetName 获取通知器名称
func (n *WeChatNotifier) GetName() string {
	return n.name
}
