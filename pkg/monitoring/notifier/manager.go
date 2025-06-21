// Package notifier 提供告警通知功能
package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"path"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/etcd"
)

// Manager 通知管理器
type Manager struct {
	etcdClient      *etcd.EtcdClient
	notifierPrefix  string
	logger          *zap.Logger
	notifiers       map[string]*models.Notifier
	notifiersMu     sync.RWMutex
	notifierFactory NotifierFactory

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NotifierFactory 通知器工厂接口
type NotifierFactory interface {
	// CreateNotifier 创建通知器实例
	CreateNotifier(notifier *models.Notifier) (Notifier, error)
}

// Notifier 通知器接口
type Notifier interface {
	// Send 发送通知
	Send(alert *models.Alert, eventType string) (*models.NotificationResult, error)
}

// Config 通知管理器配置
type Config struct {
	EtcdClient     *etcd.EtcdClient
	NotifierPrefix string
	Logger         *zap.Logger
}

// New 创建一个新的通知管理器
func New(config Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		logger, _ := zap.NewProduction()
		config.Logger = logger
	}

	return &Manager{
		etcdClient:      config.EtcdClient,
		notifierPrefix:  config.NotifierPrefix,
		logger:          config.Logger,
		notifiers:       make(map[string]*models.Notifier),
		notifierFactory: &DefaultNotifierFactory{},
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动通知管理器
func (m *Manager) Start() error {
	// 加载现有的通知器配置
	if err := m.loadNotifiers(); err != nil {
		return fmt.Errorf("加载通知器配置失败: %w", err)
	}

	// 启动etcd监听，以便在配置变更时进行更新
	m.wg.Add(1)
	go m.watchNotifiers()

	m.logger.Info("通知管理器已启动")
	return nil
}

// Stop 停止通知管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
	m.logger.Info("通知管理器已停止")
}

// loadNotifiers 从etcd加载通知器配置
func (m *Manager) loadNotifiers() error {
	m.notifiersMu.Lock()
	defer m.notifiersMu.Unlock()

	m.logger.Info("从etcd加载通知器配置", zap.String("prefix", m.notifierPrefix))

	// 获取所有通知器配置
	kvs, err := m.etcdClient.GetWithPrefix(context.Background(), m.notifierPrefix)
	if err != nil {
		return err
	}

	// 清空现有配置
	m.notifiers = make(map[string]*models.Notifier)

	// 解析配置
	for key, value := range kvs {
		notifierID := path.Base(key)
		notifier := &models.Notifier{}
		if err := json.Unmarshal([]byte(value), notifier); err != nil {
			m.logger.Error("解析通知器配置失败", zap.String("id", notifierID), zap.Error(err))
			continue
		}
		m.notifiers[notifierID] = notifier
		m.logger.Debug("加载通知器配置", zap.String("id", notifierID), zap.String("name", notifier.Name))
	}

	m.logger.Info("通知器配置加载完成", zap.Int("count", len(m.notifiers)))
	return nil
}

// watchNotifiers 监听etcd中的通知器配置变更
func (m *Manager) watchNotifiers() {
	defer m.wg.Done()

	m.logger.Info("开始监听通知器配置变更", zap.String("prefix", m.notifierPrefix))
	watchChan := m.etcdClient.WatchWithPrefix(m.ctx, m.notifierPrefix)

	for {
		select {
		case <-m.ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				m.logger.Warn("etcd监听通道已关闭，尝试重新监听")
				time.Sleep(5 * time.Second)
				watchChan = m.etcdClient.WatchWithPrefix(m.ctx, m.notifierPrefix)
				continue
			}

			for _, event := range resp.Events {
				notifierID := path.Base(string(event.Kv.Key))
				m.notifiersMu.Lock()

				switch event.Type {
				case clientv3.EventTypePut:
					notifier := &models.Notifier{}
					if err := json.Unmarshal(event.Kv.Value, notifier); err != nil {
						m.logger.Error("解析更新的通知器配置失败", zap.String("id", notifierID), zap.Error(err))
						m.notifiersMu.Unlock()
						continue
					}
					m.notifiers[notifierID] = notifier
					m.logger.Info("通知器配置已更新", zap.String("id", notifierID), zap.String("name", notifier.Name))

				case clientv3.EventTypeDelete:
					delete(m.notifiers, notifierID)
					m.logger.Info("通知器配置已删除", zap.String("id", notifierID))
				}

				m.notifiersMu.Unlock()
			}
		}
	}
}

// SendAlert 发送告警通知
func (m *Manager) SendAlert(alert *models.Alert, eventType string) []models.NotificationResult {
	if alert == nil {
		m.logger.Error("尝试发送空告警")
		return nil
	}

	m.logger.Info("准备发送告警通知",
		zap.String("alertID", alert.ID),
		zap.String("ruleID", alert.RuleID),
		zap.String("eventType", eventType),
		zap.String("severity", string(alert.Severity)))

	// 获取要发送通知的通知器列表
	notifiers := m.getEnabledNotifiers()

	if len(notifiers) == 0 {
		m.logger.Warn("没有找到可用的通知器",
			zap.String("alertID", alert.ID),
			zap.String("ruleID", alert.RuleID))
		return nil
	}

	results := make([]models.NotificationResult, 0, len(notifiers))

	// 并行发送通知
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, n := range notifiers {
		wg.Add(1)
		go func(n *models.Notifier) {
			defer wg.Done()

			// 创建通知器实例
			notifierInstance, err := m.notifierFactory.CreateNotifier(n)
			if err != nil {
				result := models.NotificationResult{
					NotifierID:   n.ID,
					NotifierName: n.Name,
					NotifierType: n.Type,
					Success:      false,
					Error:        fmt.Sprintf("创建通知器实例失败: %s", err.Error()),
					Timestamp:    time.Now().Unix(),
				}

				mu.Lock()
				results = append(results, result)
				mu.Unlock()

				m.logger.Error("创建通知器实例失败",
					zap.String("notifier", n.ID),
					zap.String("alert", alert.ID),
					zap.Error(err))
				return
			}

			// 发送通知
			result, err := notifierInstance.Send(alert, eventType)
			if err != nil {
				result = &models.NotificationResult{
					NotifierID:   n.ID,
					NotifierName: n.Name,
					NotifierType: n.Type,
					Success:      false,
					Error:        fmt.Sprintf("发送通知失败: %s", err.Error()),
					Timestamp:    time.Now().Unix(),
				}
				m.logger.Error("发送通知失败",
					zap.String("notifier", n.ID),
					zap.String("alert", alert.ID),
					zap.Error(err))
			} else if !result.Success {
				m.logger.Warn("通知发送失败但无异常",
					zap.String("notifier", n.ID),
					zap.String("alert", alert.ID),
					zap.String("error", result.Error))
			} else {
				m.logger.Info("通知发送成功",
					zap.String("notifier", n.ID),
					zap.String("alert", alert.ID))
			}

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()
		}(n)
	}

	wg.Wait()
	return results
}

// getEnabledNotifiers 获取所有启用的通知器
func (m *Manager) getEnabledNotifiers() []*models.Notifier {
	m.notifiersMu.RLock()
	defer m.notifiersMu.RUnlock()

	notifiers := make([]*models.Notifier, 0)
	for _, notifier := range m.notifiers {
		if notifier.Enabled {
			notifiers = append(notifiers, notifier)
		}
	}

	return notifiers
}

// GetNotifiers 获取所有通知器配置
func (m *Manager) GetNotifiers() []*models.Notifier {
	m.notifiersMu.RLock()
	defer m.notifiersMu.RUnlock()

	notifiers := make([]*models.Notifier, 0, len(m.notifiers))
	for _, notifier := range m.notifiers {
		notifiers = append(notifiers, notifier)
	}
	return notifiers
}

// GetNotifier 获取指定ID的通知器配置
func (m *Manager) GetNotifier(id string) (*models.Notifier, error) {
	m.notifiersMu.RLock()
	defer m.notifiersMu.RUnlock()

	notifier, exists := m.notifiers[id]
	if !exists {
		return nil, fmt.Errorf("通知器配置不存在: %s", id)
	}
	return notifier, nil
}

// CreateNotifier 创建新的通知器配置
func (m *Manager) CreateNotifier(notifier *models.Notifier) error {
	// 检查通知器类型是否受支持
	if !isSupportedNotifierType(notifier.Type) {
		return fmt.Errorf("不支持的通知器类型: %s", notifier.Type)
	}

	// 序列化配置
	data, err := json.Marshal(notifier)
	if err != nil {
		return fmt.Errorf("序列化通知器配置失败: %w", err)
	}

	// 存储到etcd
	key := path.Join(m.notifierPrefix, notifier.ID)
	if err := m.etcdClient.Put(context.Background(), key, string(data)); err != nil {
		return fmt.Errorf("存储通知器配置到etcd失败: %w", err)
	}

	return nil
}

// UpdateNotifier 更新通知器配置
func (m *Manager) UpdateNotifier(notifier *models.Notifier) error {
	if notifier.ID == "" {
		return fmt.Errorf("通知器ID不能为空")
	}

	// 检查配置是否存在
	_, err := m.GetNotifier(notifier.ID)
	if err != nil {
		return err
	}

	// 检查通知器类型是否受支持
	if !isSupportedNotifierType(notifier.Type) {
		return fmt.Errorf("不支持的通知器类型: %s", notifier.Type)
	}

	// 序列化配置
	data, err := json.Marshal(notifier)
	if err != nil {
		return fmt.Errorf("序列化通知器配置失败: %w", err)
	}

	// 存储到etcd
	key := path.Join(m.notifierPrefix, notifier.ID)
	if err := m.etcdClient.Put(context.Background(), key, string(data)); err != nil {
		return fmt.Errorf("更新通知器配置到etcd失败: %w", err)
	}

	return nil
}

// DeleteNotifier 删除通知器配置
func (m *Manager) DeleteNotifier(id string) error {
	// 检查配置是否存在
	_, err := m.GetNotifier(id)
	if err != nil {
		return err
	}

	// 从etcd中删除
	key := path.Join(m.notifierPrefix, id)
	if err := m.etcdClient.Delete(context.Background(), key); err != nil {
		return fmt.Errorf("从etcd删除通知器配置失败: %w", err)
	}

	return nil
}

// 辅助函数

// isSupportedNotifierType 检查通知器类型是否受支持
func isSupportedNotifierType(t models.NotifierType) bool {
	supportedTypes := []models.NotifierType{
		models.NotifierTypeEmail,
		models.NotifierTypeWebhook,
		models.NotifierTypeWeChat,
		models.NotifierTypeDingTalk,
	}

	for _, st := range supportedTypes {
		if t == st {
			return true
		}
	}
	return false
}

// contains 检查字符串数组中是否包含特定值
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

// DefaultNotifierFactory 默认的通知器工厂
type DefaultNotifierFactory struct{}

// CreateNotifier 创建通知器实例
func (f *DefaultNotifierFactory) CreateNotifier(notifier *models.Notifier) (Notifier, error) {
	switch notifier.Type {
	case models.NotifierTypeEmail:
		return NewEmailNotifier(notifier)
	case models.NotifierTypeWebhook:
		return NewWebhookNotifier(notifier)
	case models.NotifierTypeWeChat:
		return NewWeChatNotifier(notifier)
	case models.NotifierTypeDingTalk:
		return NewDingTalkNotifier(notifier)
	default:
		return nil, fmt.Errorf("不支持的通知器类型: %s", notifier.Type)
	}
}
