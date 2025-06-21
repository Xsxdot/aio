// Package notifier 提供邮件通知功能的实现
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"go.uber.org/zap"
)

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	config *EmailNotifierConfig
	logger *zap.Logger
	id     string
	name   string
}

// 默认的邮件主题模板
const defaultSubjectTemplate = "【{{.Level}}】{{.Title}}"

// 默认的邮件内容模板
const defaultBodyTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>通知消息</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { width: 100%; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f5f5f5; padding: 10px; border-radius: 5px; }
        .content { margin: 20px 0; }
        .footer { font-size: 12px; color: #999; margin-top: 30px; }
        .level-info { color: #2196F3; }
        .level-warning { color: #FF9800; }
        .level-error { color: #F44336; }
        .level-critical { color: #880E4F; }
        table { width: 100%; border-collapse: collapse; margin: 15px 0; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f5f5f5; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2 class="level-{{.Level}}">【{{.Level}}】{{.Title}}</h2>
        </div>
        <div class="content">
            <p><strong>消息内容:</strong></p>
            <div>{{.Content}}</div>
            <p><strong>发送时间:</strong> {{formatTime .CreatedAt}}</p>
            
            {{if .Labels}}
            <h3>标签信息</h3>
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
            
            {{if .Data}}
            <h3>附加数据</h3>
            <table>
                <tr>
                    <th>字段</th>
                    <th>值</th>
                </tr>
                {{range $key, $value := .Data}}
                <tr>
                    <td>{{$key}}</td>
                    <td>{{$value}}</td>
                </tr>
                {{end}}
            </table>
            {{end}}
        </div>
        <div class="footer">
            <p>此邮件由通知系统自动发送，请勿回复。</p>
        </div>
    </div>
</body>
</html>
`

// NewEmailNotifier 创建新的邮件通知器
func NewEmailNotifier(config *NotifierConfig) (Notifier, error) {
	if config.Type != NotifierTypeEmail {
		return nil, fmt.Errorf("通知器类型不是email: %s", config.Type)
	}

	// 解析配置
	var emailConfig EmailNotifierConfig
	configData, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := json.Unmarshal(configData, &emailConfig); err != nil {
		return nil, fmt.Errorf("解析邮件通知器配置失败: %w", err)
	}

	// 验证必要字段
	if len(emailConfig.Recipients) == 0 {
		return nil, fmt.Errorf("收件人列表不能为空")
	}
	if emailConfig.SMTPServer == "" {
		return nil, fmt.Errorf("SMTP服务器地址不能为空")
	}
	if emailConfig.SMTPPort == 0 {
		return nil, fmt.Errorf("SMTP服务器端口不能为0")
	}
	if emailConfig.FromAddress == "" {
		return nil, fmt.Errorf("发件人地址不能为空")
	}

	// 设置默认模板（如果没有提供）
	if emailConfig.SubjectTemplate == "" {
		emailConfig.SubjectTemplate = defaultSubjectTemplate
	}
	if emailConfig.BodyTemplate == "" {
		emailConfig.BodyTemplate = defaultBodyTemplate
	}

	logger, _ := zap.NewProduction()

	return &EmailNotifier{
		config: &emailConfig,
		logger: logger,
		id:     config.ID,
		name:   config.Name,
	}, nil
}

// Send 发送邮件通知
func (n *EmailNotifier) Send(notification *Notification) (*NotificationResult, error) {
	result := &NotificationResult{
		NotifierID:   n.id,
		NotifierName: n.name,
		NotifierType: NotifierTypeEmail,
		Success:      false,
		Timestamp:    time.Now().Unix(),
	}

	// 渲染主题
	subjectTmpl, err := template.New("subject").Parse(n.config.SubjectTemplate)
	if err != nil {
		result.Error = fmt.Sprintf("解析主题模板失败: %s", err.Error())
		return result, nil
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, notification); err != nil {
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
	if err := bodyTmpl.Execute(&bodyBuf, notification); err != nil {
		result.Error = fmt.Sprintf("渲染正文模板失败: %s", err.Error())
		return result, nil
	}
	body := bodyBuf.String()

	// 构建邮件消息
	message := n.buildMessage(subject, body)

	// 发送邮件
	if err := n.sendMail(message); err != nil {
		result.Error = fmt.Sprintf("发送邮件失败: %s", err.Error())
		return result, nil
	}

	result.Success = true
	n.logger.Info("邮件通知发送成功",
		zap.String("id", notification.ID),
		zap.String("title", notification.Title),
		zap.Strings("recipients", n.config.Recipients))

	return result, nil
}

// GetType 获取通知器类型
func (n *EmailNotifier) GetType() NotifierType {
	return NotifierTypeEmail
}

// GetID 获取通知器ID
func (n *EmailNotifier) GetID() string {
	return n.id
}

// GetName 获取通知器名称
func (n *EmailNotifier) GetName() string {
	return n.name
}

// buildMessage 构建邮件消息
func (n *EmailNotifier) buildMessage(subject, body string) string {
	var message bytes.Buffer

	message.WriteString(fmt.Sprintf("From: %s\r\n", n.config.FromAddress))
	message.WriteString(fmt.Sprintf("To: %s\r\n", joinRecipients(n.config.Recipients)))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)

	return message.String()
}

// sendMail 发送邮件
func (n *EmailNotifier) sendMail(message string) error {
	addr := fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)

	var auth smtp.Auth
	if n.config.SMTPUsername != "" && n.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", n.config.SMTPUsername, n.config.SMTPPassword, n.config.SMTPServer)
	}

	return smtp.SendMail(addr, auth, n.config.FromAddress, n.config.Recipients, []byte(message))
}

// joinRecipients 连接收件人列表
func joinRecipients(recipients []string) string {
	if len(recipients) == 0 {
		return ""
	}

	result := recipients[0]
	for i := 1; i < len(recipients); i++ {
		result += ", " + recipients[i]
	}
	return result
}
