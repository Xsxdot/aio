// Package notifier 提供通知器管理功能
package notifier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Storage 通知器配置存储接口
type Storage interface {
	// GetNotifier 获取单个通知器配置
	GetNotifier(ctx context.Context, id string) (*NotifierConfig, error)
	// GetNotifiers 获取所有通知器配置
	GetNotifiers(ctx context.Context) ([]*NotifierConfig, error)
	// SaveNotifier 保存通知器配置
	SaveNotifier(ctx context.Context, config *NotifierConfig) error
	// DeleteNotifier 删除通知器配置
	DeleteNotifier(ctx context.Context, id string) error
	// WatchNotifiers 监听通知器配置变更
	WatchNotifiers(ctx context.Context) (<-chan NotifierEvent, error)
}

// NotifierEvent 表示通知器配置变更事件
type NotifierEvent struct {
	Type   NotifierEventType `json:"type"`
	Config *NotifierConfig   `json:"config"`
}

// NotifierEventType 表示通知器事件类型
type NotifierEventType string

const (
	// NotifierEventAdd 添加通知器
	NotifierEventAdd NotifierEventType = "add"
	// NotifierEventUpdate 更新通知器
	NotifierEventUpdate NotifierEventType = "update"
	// NotifierEventDelete 删除通知器
	NotifierEventDelete NotifierEventType = "delete"
)

// Manager 通知器管理器
type Manager struct {
	storage   Storage
	factory   NotifierFactory
	logger    *zap.Logger
	notifiers map[string]Notifier
	configs   map[string]*NotifierConfig
	mu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 配置
	config ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// 存储后端
	Storage Storage
	// 通知器工厂
	Factory NotifierFactory
	// 日志器
	Logger *zap.Logger
	// 是否启用配置监听
	EnableWatcher bool
	// 发送超时时间
	SendTimeout time.Duration
}

// NewManager 创建新的通知器管理器
func NewManager(config ManagerConfig) (*Manager, error) {
	if config.Storage == nil {
		return nil, fmt.Errorf("存储后端不能为空")
	}

	if config.Factory == nil {
		config.Factory = NewDefaultFactory()
	}

	if config.Logger == nil {
		logger, _ := zap.NewProduction()
		config.Logger = logger
	}

	if config.SendTimeout == 0 {
		config.SendTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		storage:   config.Storage,
		factory:   config.Factory,
		logger:    config.Logger,
		notifiers: make(map[string]Notifier),
		configs:   make(map[string]*NotifierConfig),
		ctx:       ctx,
		cancel:    cancel,
		config:    config,
	}, nil
}

// Start 启动通知器管理器
func (m *Manager) Start() error {
	m.logger.Info("启动通知器管理器")

	// 加载现有配置
	if err := m.loadNotifiers(); err != nil {
		return fmt.Errorf("加载通知器配置失败: %w", err)
	}

	// 启动配置监听
	if m.config.EnableWatcher {
		m.wg.Add(1)
		go m.watchNotifiers()
	}

	m.logger.Info("通知器管理器启动完成", zap.Int("notifier_count", len(m.notifiers)))
	return nil
}

// Stop 停止通知器管理器
func (m *Manager) Stop() {
	m.logger.Info("停止通知器管理器")
	m.cancel()
	m.wg.Wait()
	m.logger.Info("通知器管理器已停止")
}

// loadNotifiers 加载通知器配置
func (m *Manager) loadNotifiers() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	configs, err := m.storage.GetNotifiers(m.ctx)
	if err != nil {
		return fmt.Errorf("从存储加载通知器配置失败: %w", err)
	}

	// 清空现有配置
	m.notifiers = make(map[string]Notifier)
	m.configs = make(map[string]*NotifierConfig)

	// 创建通知器实例
	for _, config := range configs {
		if !config.Enabled {
			m.logger.Debug("跳过禁用的通知器", zap.String("id", config.ID))
			continue
		}

		notifier, err := m.factory.CreateNotifier(config)
		if err != nil {
			m.logger.Error("创建通知器失败",
				zap.String("id", config.ID),
				zap.String("type", string(config.Type)),
				zap.Error(err))
			continue
		}

		m.notifiers[config.ID] = notifier
		m.configs[config.ID] = config
		m.logger.Debug("加载通知器",
			zap.String("id", config.ID),
			zap.String("name", config.Name),
			zap.String("type", string(config.Type)))
	}

	return nil
}

// watchNotifiers 监听通知器配置变更
func (m *Manager) watchNotifiers() {
	defer m.wg.Done()

	eventChan, err := m.storage.WatchNotifiers(m.ctx)
	if err != nil {
		m.logger.Error("启动通知器配置监听失败", zap.Error(err))
		return
	}

	m.logger.Info("开始监听通知器配置变更")

	for {
		select {
		case <-m.ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				m.logger.Warn("通知器配置监听通道已关闭")
				return
			}
			m.handleNotifierEvent(event)
		}
	}
}

// handleNotifierEvent 处理通知器配置变更事件
func (m *Manager) handleNotifierEvent(event NotifierEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch event.Type {
	case NotifierEventAdd, NotifierEventUpdate:
		if event.Config == nil {
			m.logger.Error("通知器配置为空")
			return
		}

		// 删除旧的通知器实例（如果存在）
		delete(m.notifiers, event.Config.ID)

		// 如果通知器被禁用，只更新配置
		if !event.Config.Enabled {
			m.configs[event.Config.ID] = event.Config
			m.logger.Info("通知器已禁用", zap.String("id", event.Config.ID))
			return
		}

		// 创建新的通知器实例
		notifier, err := m.factory.CreateNotifier(event.Config)
		if err != nil {
			m.logger.Error("创建通知器失败",
				zap.String("id", event.Config.ID),
				zap.Error(err))
			return
		}

		m.notifiers[event.Config.ID] = notifier
		m.configs[event.Config.ID] = event.Config
		m.logger.Info("通知器配置已更新",
			zap.String("id", event.Config.ID),
			zap.String("name", event.Config.Name))

	case NotifierEventDelete:
		if event.Config == nil {
			return
		}
		delete(m.notifiers, event.Config.ID)
		delete(m.configs, event.Config.ID)
		m.logger.Info("通知器已删除", zap.String("id", event.Config.ID))
	}
}

// SendNotification 发送通知
func (m *Manager) SendNotification(notification *Notification) []NotificationResult {
	if notification == nil {
		m.logger.Error("通知消息为空")
		return nil
	}

	m.mu.RLock()
	notifiers := make(map[string]Notifier)
	for id, notifier := range m.notifiers {
		notifiers[id] = notifier
	}
	m.mu.RUnlock()

	if len(notifiers) == 0 {
		m.logger.Warn("没有可用的通知器", zap.String("notification_id", notification.ID))
		return nil
	}

	m.logger.Info("发送通知",
		zap.String("id", notification.ID),
		zap.String("title", notification.Title),
		zap.String("level", string(notification.Level)),
		zap.Int("notifier_count", len(notifiers)))

	results := make([]NotificationResult, 0, len(notifiers))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 并行发送通知
	for _, notifier := range notifiers {
		wg.Add(1)
		go func(n Notifier) {
			defer wg.Done()

			// 设置发送超时
			ctx, cancel := context.WithTimeout(context.Background(), m.config.SendTimeout)
			defer cancel()

			// 记录开始时间
			startTime := time.Now()

			// 创建带超时的通道
			resultChan := make(chan *NotificationResult, 1)
			errorChan := make(chan error, 1)

			go func() {
				result, err := n.Send(notification)
				if err != nil {
					errorChan <- err
					return
				}
				resultChan <- result
			}()

			var result *NotificationResult
			select {
			case <-ctx.Done():
				result = &NotificationResult{
					NotifierID:   n.GetID(),
					NotifierName: n.GetName(),
					NotifierType: n.GetType(),
					Success:      false,
					Error:        "发送超时",
					Timestamp:    time.Now().Unix(),
					ResponseTime: time.Since(startTime).Milliseconds(),
				}
			case err := <-errorChan:
				result = &NotificationResult{
					NotifierID:   n.GetID(),
					NotifierName: n.GetName(),
					NotifierType: n.GetType(),
					Success:      false,
					Error:        err.Error(),
					Timestamp:    time.Now().Unix(),
					ResponseTime: time.Since(startTime).Milliseconds(),
				}
			case result = <-resultChan:
				result.ResponseTime = time.Since(startTime).Milliseconds()
			}

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()

			if result.Success {
				m.logger.Debug("通知发送成功",
					zap.String("notifier_id", result.NotifierID),
					zap.String("notifier_name", result.NotifierName))
			} else {
				m.logger.Warn("通知发送失败",
					zap.String("notifier_id", result.NotifierID),
					zap.String("notifier_name", result.NotifierName),
					zap.String("error", result.Error))
			}
		}(notifier)
	}

	wg.Wait()
	return results
}

// GetNotifiers 获取所有通知器配置
func (m *Manager) GetNotifiers() []*NotifierConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*NotifierConfig, 0, len(m.configs))
	for _, config := range m.configs {
		configs = append(configs, config)
	}
	return configs
}

// GetNotifier 获取指定的通知器配置
func (m *Manager) GetNotifier(id string) (*NotifierConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[id]
	if !exists {
		return nil, fmt.Errorf("通知器不存在: %s", id)
	}
	return config, nil
}

// CreateNotifier 创建新的通知器
func (m *Manager) CreateNotifier(config *NotifierConfig) error {
	if config == nil {
		return fmt.Errorf("通知器配置不能为空")
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if err := m.storage.SaveNotifier(m.ctx, config); err != nil {
		return fmt.Errorf("保存通知器配置失败: %w", err)
	}

	m.logger.Info("创建通知器",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("type", string(config.Type)))

	return nil
}

// UpdateNotifier 更新通知器配置
func (m *Manager) UpdateNotifier(config *NotifierConfig) error {
	if config == nil {
		return fmt.Errorf("通知器配置不能为空")
	}

	// 检查通知器是否存在
	_, err := m.GetNotifier(config.ID)
	if err != nil {
		return err
	}

	config.UpdatedAt = time.Now()

	if err := m.storage.SaveNotifier(m.ctx, config); err != nil {
		return fmt.Errorf("更新通知器配置失败: %w", err)
	}

	m.logger.Info("更新通知器",
		zap.String("id", config.ID),
		zap.String("name", config.Name))

	return nil
}

// DeleteNotifier 删除通知器
func (m *Manager) DeleteNotifier(id string) error {
	// 检查通知器是否存在
	_, err := m.GetNotifier(id)
	if err != nil {
		return err
	}

	if err := m.storage.DeleteNotifier(m.ctx, id); err != nil {
		return fmt.Errorf("删除通知器失败: %w", err)
	}

	m.logger.Info("删除通知器", zap.String("id", id))
	return nil
}
