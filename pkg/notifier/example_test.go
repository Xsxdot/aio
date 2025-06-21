// Package notifier 提供通知器使用示例
package notifier_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/xsxdot/aio/app/config"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/notifier"
	"github.com/xsxdot/aio/pkg/notifier/storage"
	"go.uber.org/zap"
)

// ExampleManager_basic 基本使用示例
func ExampleManager_basic() {
	// 1. 创建ETCD客户端（假设ETCD已经启动）
	etcdClient, err := etcd.NewClient(&config.EtcdConfig{
		Endpoints:   "localhost:2379",
		DialTimeout: 5,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. 创建存储后端
	etcdStorage, err := storage.NewEtcdStorage(storage.EtcdStorageConfig{
		Client: etcdClient,
		Prefix: "/example/notifiers",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 3. 创建通知器管理器
	manager, err := notifier.NewManager(notifier.ManagerConfig{
		Storage:       etcdStorage,
		Factory:       notifier.NewDefaultFactory(),
		Logger:        zap.NewExample(),
		EnableWatcher: true,
		SendTimeout:   30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 4. 启动管理器
	if err := manager.Start(); err != nil {
		log.Fatal(err)
	}
	defer manager.Stop()

	// 5. 创建邮件通知器配置
	emailConfig := &notifier.NotifierConfig{
		ID:          "email-notifier-1",
		Type:        notifier.NotifierTypeEmail,
		Name:        "测试邮件通知器",
		Description: "用于测试的邮件通知器",
		Enabled:     true,
		Config: notifier.EmailNotifierConfig{
			Recipients:      []string{"test@example.com"},
			SMTPServer:      "smtp.example.com",
			SMTPPort:        587,
			SMTPUsername:    "user@example.com",
			SMTPPassword:    "password",
			UseTLS:          true,
			FromAddress:     "noreply@example.com",
			SubjectTemplate: "【{{.Level}}】{{.Title}}",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 6. 创建通知器
	if err := manager.CreateNotifier(emailConfig); err != nil {
		log.Fatal(err)
	}

	// 7. 发送通知
	notification := &notifier.Notification{
		ID:      "test-notification-1",
		Title:   "测试通知",
		Content: "这是一个测试通知消息",
		Level:   notifier.NotificationLevelInfo,
		Labels: map[string]string{
			"source": "example",
			"type":   "test",
		},
		CreatedAt: time.Now(),
		Data: map[string]interface{}{
			"additional_info": "额外信息",
		},
	}

	results := manager.SendNotification(notification)
	for _, result := range results {
		if result.Success {
			fmt.Printf("通知发送成功: %s\n", result.NotifierName)
		} else {
			fmt.Printf("通知发送失败: %s, 错误: %s\n", result.NotifierName, result.Error)
		}
	}

	fmt.Println("基本使用示例完成")
	// Output: 基本使用示例完成
}

// ExampleManager_multipleNotifiers 多通知器示例
func ExampleManager_multipleNotifiers() {
	// 这个示例展示了如何配置多种类型的通知器

	// 钉钉通知器配置
	dingTalkConfig := &notifier.NotifierConfig{
		ID:      "dingtalk-notifier-1",
		Type:    notifier.NotifierTypeDingTalk,
		Name:    "钉钉群通知",
		Enabled: true,
		Config: notifier.DingTalkNotifierConfig{
			WebhookURL:      "https://oapi.dingtalk.com/robot/send?access_token=your_token",
			Secret:          "your_secret",
			TitleTemplate:   "【{{.Level}}】{{.Title}}",
			ContentTemplate: "通知内容: {{.Content}}",
			UseMarkdown:     true,
			AtAll:           false,
			AtMobiles:       []string{"13800138000"},
		},
	}

	// Webhook通知器配置
	webhookConfig := &notifier.NotifierConfig{
		ID:      "webhook-notifier-1",
		Type:    notifier.NotifierTypeWebhook,
		Name:    "Webhook通知",
		Enabled: true,
		Config: notifier.WebhookNotifierConfig{
			URL:            "https://api.example.com/webhook",
			Method:         "POST",
			TimeoutSeconds: 30,
			Headers: map[string]string{
				"Authorization": "Bearer your_token",
				"Content-Type":  "application/json",
			},
			BodyTemplate: `{
				"title": "{{.Title}}",
				"content": "{{.Content}}",
				"level": "{{.Level}}",
				"timestamp": "{{formatTime .CreatedAt}}"
			}`,
		},
	}

	// 企业微信通知器配置
	wechatConfig := &notifier.NotifierConfig{
		ID:      "wechat-notifier-1",
		Type:    notifier.NotifierTypeWeChat,
		Name:    "企业微信通知",
		Enabled: true,
		Config: notifier.WeChatNotifierConfig{
			WebhookURL:       "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=your_key",
			TitleTemplate:    "【{{.Level}}】{{.Title}}",
			ContentTemplate:  "通知内容: {{.Content}}",
			MentionedUserIDs: []string{"@all"},
			MentionAll:       false,
		},
	}

	fmt.Printf("配置了 %d 种通知器类型\n", 3)
	fmt.Printf("- 钉钉通知器: %s\n", dingTalkConfig.Name)
	fmt.Printf("- Webhook通知器: %s\n", webhookConfig.Name)
	fmt.Printf("- 企业微信通知器: %s\n", wechatConfig.Name)

	// Output:
	// 配置了 3 种通知器类型
	// - 钉钉通知器: 钉钉群通知
	// - Webhook通知器: Webhook通知
	// - 企业微信通知器: 企业微信通知
}

// ExampleManager_monitoringIntegration 监控集成示例
func ExampleManager_monitoringIntegration() {
	ctx := context.Background()

	// 模拟告警数据
	// 这里展示了如何将监控告警转换为通用通知

	fmt.Println("监控系统集成示例:")
	fmt.Println("1. 监控系统检测到告警")
	fmt.Println("2. 将告警转换为通用通知格式")
	fmt.Println("3. 通过通知器管理器发送通知")
	fmt.Println("4. 支持多种通知渠道（邮件、钉钉、微信等）")

	// 使用context控制生命周期
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Output:
	// 监控系统集成示例:
	// 1. 监控系统检测到告警
	// 2. 将告警转换为通用通知格式
	// 3. 通过通知器管理器发送通知
	// 4. 支持多种通知渠道（邮件、钉钉、微信等）
}
