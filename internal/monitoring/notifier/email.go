// Package notifier 提供邮件通知功能的实现
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"html/template"
	"net/smtp"
	"time"

	"go.uber.org/zap"
)

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	config *models2.EmailNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的邮件主题模板
const defaultSubjectTemplate = "【{{.Severity}}】告警: {{.RuleName}}"

// 默认的邮件内容模板
const defaultBodyTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>告警通知</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { width: 100%; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f5f5f5; padding: 10px; border-radius: 5px; }
        .content { margin: 20px 0; }
        .footer { font-size: 12px; color: #999; margin-top: 30px; }
        .severity-info { color: #2196F3; }
        .severity-warning { color: #FF9800; }
        .severity-critical { color: #F44336; }
        .severity-emergency { color: #880E4F; }
        table { width: 100%; border-collapse: collapse; margin: 15px 0; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f5f5f5; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2 class="severity-{{.Severity}}">【{{.Severity}}】告警: {{.RuleName}}</h2>
        </div>
        <div class="content">
            <p><strong>告警描述:</strong> {{.Description}}</p>
            <p><strong>告警状态:</strong> {{if eq .EventType "triggered"}}已触发{{else}}已解决{{end}}</p>
            <p><strong>开始时间:</strong> {{formatTime .StartsAt}}</p>
            {{if eq .EventType "resolved"}}
            <p><strong>解决时间:</strong> {{formatTime .EndsAt}}</p>
            {{end}}
            
            <table>
                <tr>
                    <th>指标</th>
                    <th>当前值</th>
                    <th>阈值</th>
                    <th>条件</th>
                </tr>
                <tr>
                    <td>{{.Metric}}</td>
                    <td>{{.Value}}</td>
                    <td>{{.Threshold}}</td>
                    <td>{{.Condition}}</td>
                </tr>
            </table>
            
            {{if .Labels}}
            <h3>标签</h3>
            <table>
                <tr>
                    <th>名称</th>
                    <th>值</th>
                </tr>
                {{range $key, $value := .Labels}}
                <tr>
                    <td>{{$key}}</td>
                    <td>{{$value}}</td>
                </tr>
                {{end}}
            </table>
            {{end}}
        </div>
        <div class="footer">
            <p>此邮件由监控系统自动发送，请勿回复。</p>
        </div>
    </div>
</body>
</html>
`

// NewEmailNotifier 创建新的邮件通知器
func NewEmailNotifier(notifier *models2.Notifier) (Notifier, error) {
	if notifier.Type != models2.NotifierTypeEmail {
		return nil, fmt.Errorf("通知器类型不是email: %s", notifier.Type)
	}

	// 解析配置
	var config models2.EmailNotifierConfig
	configData, err := json.Marshal(notifier.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("解析邮件通知器配置失败: %w", err)
	}

	// 验证必要字段
	if len(config.Recipients) == 0 {
		return nil, fmt.Errorf("收件人列表不能为空")
	}
	if config.SMTPServer == "" {
		return nil, fmt.Errorf("SMTP服务器地址不能为空")
	}
	if config.SMTPPort == 0 {
		return nil, fmt.Errorf("SMTP服务器端口不能为0")
	}
	if config.FromAddress == "" {
		return nil, fmt.Errorf("发件人地址不能为空")
	}

	// 设置默认模板（如果没有提供）
	if config.SubjectTemplate == "" {
		config.SubjectTemplate = defaultSubjectTemplate
	}
	if config.BodyTemplate == "" {
		config.BodyTemplate = defaultBodyTemplate
	}

	logger, _ := zap.NewProduction()

	return &EmailNotifier{
		config: &config,
		logger: logger,
		id:     notifier.ID,
		name:   notifier.Name,
	}, nil
}

// Send 发送邮件通知
func (n *EmailNotifier) Send(alert *models2.Alert, eventType string) (*models2.NotificationResult, error) {
	result := &models2.NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: models2.NotifierTypeEmail,
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

	// 渲染主题
	subjectTmpl, err := template.New("subject").Parse(n.config.SubjectTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析主题模板失败: %s", err.Error())
		return result, nil
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		result.Error = fmt.Sprintf("渲染主题模板失败: %s", err.Error())
		return result, nil
	}
	subject := subjectBuf.String()

	// 渲染正文
	bodyTmpl := template.New("body").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
	})

	bodyTmpl, err = bodyTmpl.Parse(n.config.BodyTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析正文模板失败: %s", err.Error())
		return result, nil
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		result.Error = fmt.Sprintf("渲染正文模板失败: %s", err.Error())
		return result, nil
	}
	body := bodyBuf.String()

	// 组装完整邮件内容
	from := n.config.FromAddress
	to := n.config.Recipients

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to[0]
	if len(to) > 1 {
		for i := 1; i < len(to); i++ {
			header["To"] += ", " + to[i]
		}
	}
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// 发送邮件
	auth := smtp.PlainAuth("", n.config.SMTPUsername, n.config.SMTPPassword, n.config.SMTPServer)
	addr := fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)

	if err := smtp.SendMail(addr, auth, from, to, []byte(message)); err != nil {
		result.Error = fmt.Sprintf("发送邮件失败: %s", err.Error())
		return result, nil
	}

	result.Success = true
	return result, nil
}
