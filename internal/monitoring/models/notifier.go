// Package models 定义通知器相关的数据模型
package models

// NotifierType 表示通知器的类型
type NotifierType string

const (
	// NotifierTypeEmail 邮件通知器
	NotifierTypeEmail NotifierType = "email"
	// NotifierTypeWebhook Webhook通知器
	NotifierTypeWebhook NotifierType = "webhook"
	// NotifierTypeWeChat 企业微信通知器
	NotifierTypeWeChat NotifierType = "wechat"
	// NotifierTypeDingTalk 钉钉通知器
	NotifierTypeDingTalk NotifierType = "dingtalk"
)

// Notifier 表示一个通知器配置
type Notifier struct {
	// 通知器ID
	ID string `json:"id"`
	// 通知器类型
	Type NotifierType `json:"type"`
	// 通知器名称
	Name string `json:"name"`
	// 通知器配置
	Config interface{} `json:"config"`
	// 是否启用
	Enabled bool `json:"enabled"`
}

// EmailNotifierConfig 邮件通知器配置
type EmailNotifierConfig struct {
	// 收件人列表
	Recipients []string `json:"recipients"`
	// SMTP服务器地址
	SMTPServer string `json:"smtp_server"`
	// SMTP服务器端口
	SMTPPort int `json:"smtp_port"`
	// SMTP用户名
	SMTPUsername string `json:"smtp_username,omitempty"`
	// SMTP密码
	SMTPPassword string `json:"smtp_password,omitempty"`
	// 是否使用TLS
	UseTLS bool `json:"use_tls,omitempty"`
	// 发件人地址
	FromAddress string `json:"from_address"`
	// 邮件主题模板
	SubjectTemplate string `json:"subject_template,omitempty"`
	// 邮件内容模板
	BodyTemplate string `json:"body_template,omitempty"`
}

// WebhookNotifierConfig Webhook通知器配置
type WebhookNotifierConfig struct {
	// Webhook URL
	URL string `json:"url"`
	// HTTP方法
	Method string `json:"method"`
	// 自定义请求头
	Headers map[string]string `json:"headers,omitempty"`
	// 请求体模板
	BodyTemplate string `json:"body_template,omitempty"`
	// 超时时间（秒）
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// WeChatNotifierConfig 企业微信通知器配置
type WeChatNotifierConfig struct {
	// 企业微信机器人Webhook URL
	WebhookURL string `json:"webhook_url"`
	// 标题模板
	TitleTemplate string `json:"title_template,omitempty"`
	// 内容模板
	ContentTemplate string `json:"content_template,omitempty"`
	// 提及用户ID列表
	MentionedUserIDs []string `json:"mentioned_user_ids,omitempty"`
	// 是否提及所有人
	MentionAll bool `json:"mention_all,omitempty"`
}

// DingTalkNotifierConfig 钉钉通知器配置
type DingTalkNotifierConfig struct {
	// 钉钉机器人Webhook URL
	WebhookURL string `json:"webhook_url"`
	// 钉钉机器人安全设置密钥
	Secret string `json:"secret,omitempty"`
	// 标题模板
	TitleTemplate string `json:"title_template,omitempty"`
	// 内容模板
	ContentTemplate string `json:"content_template,omitempty"`
	// 是否使用Markdown格式
	UseMarkdown bool `json:"use_markdown,omitempty"`
	// @手机号列表
	AtMobiles []string `json:"at_mobiles,omitempty"`
	// 是否@所有人
	AtAll bool `json:"at_all,omitempty"`
}

// NotificationResult 表示通知发送结果
type NotificationResult struct {
	// 通知器ID
	NotifierID string `json:"notifier_id"`
	// 通知器名称
	NotifierName string `json:"notifier_name"`
	// 通知器类型
	NotifierType NotifierType `json:"notifier_type"`
	// 是否成功
	Success bool `json:"success"`
	// 错误信息（如果失败）
	Error string `json:"error,omitempty"`
	// 发送时间戳
	Timestamp int64 `json:"timestamp"`
}
