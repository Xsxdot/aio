// Package notifier 提供钉钉通知功能的实现
package notifier

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// DingTalkNotifier 钉钉通知器
type DingTalkNotifier struct {
	config *DingTalkNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的标题模板
const defaultDingTalkTitleTemplate = "【{{.Level}}】{{.Title}}"

// 默认的内容模板 (Markdown格式)
const defaultDingTalkContentTemplate = `
### 通知详情

- **消息标题**: {{.Title}}
- **消息级别**: {{.Level}}
- **发送时间**: {{formatTime .CreatedAt}}

### 消息内容

{{.Content}}

{{if .Labels}}
### 标签信息

{{range $key, $value := .Labels}}
- **{{$key}}**: {{$value}}
{{end}}
{{end}}

{{if .Data}}
### 附加数据

{{range $key, $value := .Data}}
- **{{$key}}**: {{$value}}
{{end}}
{{end}}
`

// NewDingTalkNotifier 创建新的钉钉通知器
func NewDingTalkNotifier(config *NotifierConfig) (Notifier, error) {
	if config.Type != NotifierTypeDingTalk {
		return nil, fmt.Errorf("通知器类型不是dingtalk: %s", config.Type)
	}

	// 解析配置
	var dingConfig DingTalkNotifierConfig
	configData, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &dingConfig); err != nil {
		return nil, fmt.Errorf("解析钉钉通知器配置失败: %w", err)
	}

	// 验证必要字段
	if dingConfig.WebhookURL == "" {
		return nil, fmt.Errorf("钉钉Webhook URL不能为空")
	}

	// 设置默认值
	if dingConfig.TitleTemplate == "" {
		dingConfig.TitleTemplate = defaultDingTalkTitleTemplate
	}
	if dingConfig.ContentTemplate == "" {
		dingConfig.ContentTemplate = defaultDingTalkContentTemplate
	}
	if !dingConfig.UseMarkdown {
		dingConfig.UseMarkdown = true // 总是使用Markdown格式
	}

	logger, _ := zap.NewProduction()

	return &DingTalkNotifier{
		config: &dingConfig,
		logger: logger,
		id:     config.ID,
		name:   config.Name,
	}, nil
}

// Send 发送钉钉通知
func (n *DingTalkNotifier) Send(notification *Notification) (*NotificationResult, error) {
	result := &NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: NotifierTypeDingTalk,
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
		result.Error = fmt.Sprintf("HTTP响应状态异常: %d", resp.StatusCode)
		return result, nil
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		result.Error = fmt.Sprintf("解析响应失败: %s", err.Error())
		return result, nil
	}

	// 检查钉钉接口返回
	if errCode, ok := response["errcode"]; ok && errCode.(float64) != 0 {
		errMsg := response["errmsg"]
		result.Error = fmt.Sprintf("钉钉接口返回错误: %v", errMsg)
		return result, nil
	}

	result.Success = true
	n.logger.Info("钉钉通知发送成功",
		zap.String("id", notification.ID),
		zap.String("title", notification.Title))

	return result, nil
}

// GetType 获取通知器类型
func (n *DingTalkNotifier) GetType() NotifierType {
	return NotifierTypeDingTalk
}

// GetID 获取通知器ID
func (n *DingTalkNotifier) GetID() string {
	return n.id
}

// GetName 获取通知器名称
func (n *DingTalkNotifier) GetName() string {
	return n.name
}

// addSignature 添加签名到Webhook URL
func (n *DingTalkNotifier) addSignature(webhookURL string) string {
	timestamp := time.Now().UnixNano() / 1e6 // 毫秒时间戳
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, n.config.Secret)

	// 使用HmacSHA256算法计算签名
	h := hmac.New(sha256.New, []byte(n.config.Secret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 对签名进行URL编码
	signatureEncoded := url.QueryEscape(signature)

	// 在URL中添加时间戳和签名参数
	if len(webhookURL) > 0 {
		if webhookURL[len(webhookURL)-1] == '&' {
			return fmt.Sprintf("%stimestamp=%d&sign=%s", webhookURL, timestamp, signatureEncoded)
		} else {
			return fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, signatureEncoded)
		}
	}

	return webhookURL
}
