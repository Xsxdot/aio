# 通知器组件 (Notifier)

本组件提供了一个独立的、可扩展的通知系统，支持多种通知渠道，可以被任何需要发送通知的模块使用。

## ✨ 特性

- 🔧 **插件化架构**: 支持多种通知器类型，易于扩展
- 📧 **多种通知渠道**: 支持邮件、钉钉、企业微信、Webhook
- 🔄 **动态配置**: 支持运行时添加、修改、删除通知器配置
- 💾 **持久化存储**: 基于ETCD的配置存储，支持集群部署
- ⚡ **高性能**: 并发发送通知，支持超时控制
- 🔍 **监控友好**: 详细的日志记录和发送结果统计
- 🛡️ **容错性**: 单个通知器失败不影响其他通知器

## 📖 快速开始

### 1. 基本使用

```go
package main

import (
    "time"
    "github.com/xsxdot/aio/pkg/notifier"
    "github.com/xsxdot/aio/pkg/notifier/storage"
    "github.com/xsxdot/aio/internal/etcd"
    "github.com/xsxdot/aio/app/config"
)

func main() {
    // 创建ETCD存储
    etcdClient, _ := etcd.NewClient(&config.EtcdConfig{
        Endpoints: "localhost:2379",
        DialTimeout: 5,
    })
    
    etcdStorage, _ := storage.NewEtcdStorage(storage.EtcdStorageConfig{
        Client: etcdClient,
        Prefix: "/notifiers",
    })

    // 创建通知器管理器
    manager, _ := notifier.NewManager(notifier.ManagerConfig{
        Storage:       etcdStorage,
        Factory:       notifier.NewDefaultFactory(),
        EnableWatcher: true,
        SendTimeout:   30 * time.Second,
    })

    // 启动管理器
    manager.Start()
    defer manager.Stop()

    // 创建邮件通知器
    emailConfig := &notifier.NotifierConfig{
        ID:   "email-1",
        Type: notifier.NotifierTypeEmail,
        Name: "邮件通知器",
        Enabled: true,
        Config: notifier.EmailNotifierConfig{
            Recipients:   []string{"admin@example.com"},
            SMTPServer:   "smtp.example.com",
            SMTPPort:     587,
            FromAddress:  "noreply@example.com",
            SMTPUsername: "user@example.com",
            SMTPPassword: "password",
            UseTLS:       true,
        },
    }
    
    manager.CreateNotifier(emailConfig)

    // 发送通知
    notification := &notifier.Notification{
        ID:      "test-1",
        Title:   "系统通知",
        Content: "这是一条测试通知",
        Level:   notifier.NotificationLevelInfo,
        CreatedAt: time.Now(),
    }
    
    results := manager.SendNotification(notification)
    // 处理发送结果...
}
```

### 2. 监控系统集成

```go
// 将监控告警转换为通用通知
import "github.com/xsxdot/aio/pkg/notifier/adapter"

adapter := adapter.NewMonitoringAdapter()
notification := adapter.AlertToNotification(alert, "triggered")

// 发送通知
results := manager.SendNotification(notification)
```

## 🛠️ 支持的通知器类型

### 1. 邮件通知器 (Email)

```go
emailConfig := notifier.EmailNotifierConfig{
    Recipients:      []string{"user@example.com"},
    SMTPServer:      "smtp.gmail.com",
    SMTPPort:        587,
    SMTPUsername:    "your-email@gmail.com",
    SMTPPassword:    "your-password",
    UseTLS:          true,
    FromAddress:     "noreply@example.com",
    SubjectTemplate: "【{{.Level}}】{{.Title}}",
    BodyTemplate:    "自定义HTML模板",
}
```

### 2. 钉钉通知器 (DingTalk)

```go
dingConfig := notifier.DingTalkNotifierConfig{
    WebhookURL:      "https://oapi.dingtalk.com/robot/send?access_token=xxx",
    Secret:          "SEC...", // 可选，用于签名验证
    TitleTemplate:   "【{{.Level}}】{{.Title}}",
    ContentTemplate: "## 通知内容\n{{.Content}}",
    UseMarkdown:     true,
    AtAll:           false,
    AtMobiles:       []string{"13800138000"},
}
```

### 3. 企业微信通知器 (WeChat)

```go
wechatConfig := notifier.WeChatNotifierConfig{
    WebhookURL:       "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx",
    TitleTemplate:    "【{{.Level}}】{{.Title}}",
    ContentTemplate:  "通知内容: {{.Content}}",
    MentionedUserIDs: []string{"@all"},
    MentionAll:       true,
}
```

### 4. Webhook通知器

```go
webhookConfig := notifier.WebhookNotifierConfig{
    URL:            "https://api.example.com/webhook",
    Method:         "POST",
    TimeoutSeconds: 30,
    Headers: map[string]string{
        "Authorization": "Bearer token",
        "Content-Type":  "application/json",
    },
    BodyTemplate: `{
        "title": "{{.Title}}",
        "content": "{{.Content}}",
        "level": "{{.Level}}"
    }`,
}
```

## 🔧 架构设计

### 核心组件

1. **Manager**: 通知器管理器，负责生命周期管理
2. **Storage**: 配置存储接口，支持ETCD等后端
3. **Factory**: 通知器工厂，负责创建具体的通知器实例
4. **Notifier**: 通知器接口，定义发送通知的标准
5. **Adapter**: 适配器，将不同数据源转换为通用通知格式

### 扩展性

- **自定义通知器**: 实现 `Notifier` 接口
- **自定义存储**: 实现 `Storage` 接口
- **自定义工厂**: 实现 `NotifierFactory` 接口

## 📝 模板系统

通知器支持Go template语法的模板系统，可以动态生成通知内容。

### 可用变量

```go
type Notification struct {
    ID        string                 // 通知ID
    Title     string                 // 通知标题
    Content   string                 // 通知内容
    Level     NotificationLevel      // 通知级别
    Labels    map[string]string      // 标签
    CreatedAt time.Time             // 创建时间
    Data      map[string]interface{} // 额外数据
}
```

### 内置函数

- `formatTime`: 格式化时间
- `toJSON`: 转换为JSON字符串

### 示例模板

```html
<!-- 邮件模板 -->
<h2>【{{.Level}}】{{.Title}}</h2>
<p>{{.Content}}</p>
<p>发送时间: {{formatTime .CreatedAt}}</p>

{{if .Labels}}
<h3>标签信息</h3>
<ul>
{{range $key, $value := .Labels}}
    <li>{{$key}}: {{$value}}</li>
{{end}}
</ul>
{{end}}
```

## 🔍 监控和日志

### 日志级别

- `DEBUG`: 详细的调试信息
- `INFO`: 一般信息（创建、更新、删除通知器等）
- `WARN`: 警告信息（发送失败等）
- `ERROR`: 错误信息（配置错误、连接失败等）

### 发送结果

每次发送通知后，会返回详细的发送结果：

```go
type NotificationResult struct {
    NotifierID   string    // 通知器ID
    NotifierName string    // 通知器名称
    NotifierType string    // 通知器类型
    Success      bool      // 是否成功
    Error        string    // 错误信息
    Timestamp    int64     // 发送时间戳
    ResponseTime int64     // 响应时间(毫秒)
}
```

## 🚀 最佳实践

### 1. 配置管理

- 使用环境变量或配置文件管理敏感信息
- 定期备份通知器配置
- 使用不同的通知器处理不同级别的通知

### 2. 错误处理

- 监控发送失败率
- 设置合理的超时时间
- 实现重试机制（在业务层）

### 3. 性能优化

- 合理设置并发数量
- 使用连接池
- 定期清理无效的通知器配置

### 4. 安全考虑

- 不在日志中记录敏感信息
- 使用HTTPS/TLS加密通信
- 定期更新访问凭证

## 🤝 贡献指南

欢迎贡献新的通知器类型！

1. 实现 `Notifier` 接口
2. 在 `Factory` 中注册新类型
3. 添加配置结构体
4. 编写测试用例
5. 更新文档

## 📄 许可证

本项目使用 MIT 许可证。 