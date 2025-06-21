// Package notifier 提供通知器工厂实现
package notifier

import (
	"fmt"
)

// DefaultFactory 默认通知器工厂
type DefaultFactory struct{}

// NewDefaultFactory 创建默认通知器工厂
func NewDefaultFactory() NotifierFactory {
	return &DefaultFactory{}
}

// CreateNotifier 创建通知器实例
func (f *DefaultFactory) CreateNotifier(config *NotifierConfig) (Notifier, error) {
	if config == nil {
		return nil, fmt.Errorf("通知器配置不能为空")
	}

	switch config.Type {
	case NotifierTypeEmail:
		return NewEmailNotifier(config)
	case NotifierTypeWebhook:
		return NewWebhookNotifier(config)
	case NotifierTypeWeChat:
		return NewWeChatNotifier(config)
	case NotifierTypeDingTalk:
		return NewDingTalkNotifier(config)
	default:
		return nil, fmt.Errorf("不支持的通知器类型: %s", config.Type)
	}
}

// SupportedTypes 获取支持的通知器类型
func (f *DefaultFactory) SupportedTypes() []NotifierType {
	return []NotifierType{
		NotifierTypeEmail,
		NotifierTypeWebhook,
		NotifierTypeWeChat,
		NotifierTypeDingTalk,
	}
}
