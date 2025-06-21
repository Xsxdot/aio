// Package alerting 提供告警规则管理和评估功能
package alerting

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/etcd"
)

// Manager 告警规则管理器
type Manager struct {
	etcdClient  *etcd.EtcdClient
	storage     *storage.Storage
	alertPrefix string
	logger      *zap.Logger

	rules        map[string]*models.AlertRule
	activeAlerts map[string]*models.Alert
	alertsMu     sync.RWMutex

	notifierManager NotifierManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NotifierManager 通知管理器接口
type NotifierManager interface {
	SendAlert(alert *models.Alert, eventType string) []models.NotificationResult
}

// Config 告警管理器配置
type Config struct {
	EtcdClient  *etcd.EtcdClient
	Storage     *storage.Storage
	AlertPrefix string
	Logger      *zap.Logger
}

// New 创建一个新的告警管理器
func New(config Config, notifierManager NotifierManager) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		logger, _ := zap.NewProduction()
		config.Logger = logger
	}

	return &Manager{
		etcdClient:      config.EtcdClient,
		storage:         config.Storage,
		alertPrefix:     config.AlertPrefix,
		logger:          config.Logger,
		rules:           make(map[string]*models.AlertRule),
		activeAlerts:    make(map[string]*models.Alert),
		notifierManager: notifierManager,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动告警管理器
func (m *Manager) Start() error {
	// 加载现有的告警规则
	if err := m.loadRules(); err != nil {
		return fmt.Errorf("加载告警规则失败: %w", err)
	}

	// 启动告警规则评估协程
	m.wg.Add(1)
	go m.runEvaluation()

	// 启动etcd监听，以便在规则变更时进行更新
	m.wg.Add(1)
	go m.watchRules()

	m.logger.Info("告警管理器已启动")
	return nil
}

// Stop 停止告警管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
	m.logger.Info("告警管理器已停止")
}

// loadRules 从etcd加载告警规则
func (m *Manager) loadRules() error {
	m.alertsMu.Lock()
	defer m.alertsMu.Unlock()

	m.logger.Info("从etcd加载告警规则", zap.String("prefix", m.alertPrefix))

	// 获取所有规则
	kvs, err := m.etcdClient.GetWithPrefix(context.Background(), m.alertPrefix)
	if err != nil {
		return err
	}

	// 清空现有规则
	m.rules = make(map[string]*models.AlertRule)

	// 解析规则
	for key, value := range kvs {
		ruleID := path.Base(key)
		rule := &models.AlertRule{}
		if err := json.Unmarshal([]byte(value), rule); err != nil {
			m.logger.Error("解析告警规则失败", zap.String("id", ruleID), zap.Error(err))
			continue
		}
		m.rules[ruleID] = rule
		m.logger.Debug("加载告警规则", zap.String("id", ruleID), zap.String("name", rule.Name))
	}

	m.logger.Info("告警规则加载完成", zap.Int("count", len(m.rules)))
	return nil
}

// watchRules 监听etcd中的规则变更
func (m *Manager) watchRules() {
	defer m.wg.Done()

	m.logger.Info("开始监听告警规则变更", zap.String("prefix", m.alertPrefix))
	watchChan := m.etcdClient.WatchWithPrefix(m.ctx, m.alertPrefix)

	for {
		select {
		case <-m.ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				m.logger.Warn("etcd监听通道已关闭，尝试重新监听")
				time.Sleep(5 * time.Second)
				watchChan = m.etcdClient.WatchWithPrefix(m.ctx, m.alertPrefix)
				continue
			}

			for _, event := range resp.Events {
				ruleID := path.Base(string(event.Kv.Key))
				m.alertsMu.Lock()

				switch event.Type {
				case clientv3.EventTypePut:
					rule := &models.AlertRule{}
					if err := json.Unmarshal(event.Kv.Value, rule); err != nil {
						m.logger.Error("解析更新的告警规则失败", zap.String("id", ruleID), zap.Error(err))
						m.alertsMu.Unlock()
						continue
					}
					m.rules[ruleID] = rule
					m.logger.Info("告警规则已更新", zap.String("id", ruleID), zap.String("name", rule.Name))

				case clientv3.EventTypeDelete:
					delete(m.rules, ruleID)
					m.logger.Info("告警规则已删除", zap.String("id", ruleID))
				}

				m.alertsMu.Unlock()
			}
		}
	}
}

// runEvaluation 运行告警规则评估循环
func (m *Manager) runEvaluation() {
	defer m.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.evaluateAllRules()
		}
	}
}

// evaluateAllRules 评估所有告警规则
func (m *Manager) evaluateAllRules() {
	m.alertsMu.RLock()
	rules := make([]*models.AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	m.alertsMu.RUnlock()

	for _, rule := range rules {
		if err := m.evaluateRule(rule); err != nil {
			m.logger.Error("评估告警规则失败", zap.String("id", rule.ID), zap.Error(err))
		}
	}
}

// evaluateRule 评估单个告警规则
func (m *Manager) evaluateRule(rule *models.AlertRule) error {
	// 解析持续时间
	duration, err := time.ParseDuration(rule.Duration)
	if err != nil {
		return fmt.Errorf("解析持续时间失败: %w", err)
	}

	now := time.Now()
	startTime := now.Add(-duration)

	// 构建统一的查询结构
	query := storage.MetricQuery{
		StartTime:   startTime,
		EndTime:     now,
		MetricNames: []string{string(rule.Metric)},
	}

	// 根据目标类型设置查询参数
	var results *models.QueryResult
	if rule.TargetType == models.AlertTargetServer {
		// 查询服务器指标
		query.Categories = []models.MetricCategory{models.CategoryServer}
		results, err = m.storage.QueryTimeSeries(query)
	} else if rule.TargetType == models.AlertTargetApplication {
		// 查询应用指标
		query.Categories = []models.MetricCategory{models.CategoryApp}
		// 添加标签匹配器
		if len(rule.LabelMatchers) > 0 {
			query.LabelMatchers = rule.LabelMatchers
		}
		results, err = m.storage.QueryTimeSeries(query)
	} else {
		return fmt.Errorf("未知的目标类型: %s", rule.TargetType)
	}

	if err != nil {
		return fmt.Errorf("查询指标数据失败: %w", err)
	}

	// 检查是否有数据返回
	if len(results.Series) == 0 {
		m.logger.Debug("没有匹配的指标数据", zap.String("rule", rule.ID), zap.String("metric", rule.Metric))
		return nil
	}

	// 对每个时间序列进行评估
	for _, series := range results.Series {
		if len(series.Points) == 0 {
			continue
		}

		// 获取最新的数据点
		latestPoint := series.Points[len(series.Points)-1]

		// 创建告警键
		alertKey := fmt.Sprintf("%s:%s", rule.ID, series.Name)
		for k, v := range series.Labels {
			alertKey += fmt.Sprintf(":%s=%s", k, v)
		}

		// 检查是否满足告警条件
		if m.checkCondition(rule.Condition, latestPoint.Value, rule.Threshold) {
			// 检查是否所有数据点都满足条件（持续时间检查）
			allPointsTriggered := true
			for _, point := range series.Points {
				if !m.checkCondition(rule.Condition, point.Value, rule.Threshold) {
					allPointsTriggered = false
					break
				}
			}

			if allPointsTriggered {
				// 触发或更新告警
				m.triggerAlert(alertKey, rule, series.Labels, latestPoint.Value)
			}
		} else {
			// 解决告警
			m.resolveAlert(alertKey)
		}
	}

	return nil
}

// checkCondition 检查是否满足告警条件
func (m *Manager) checkCondition(condition models.AlertConditionType, value, threshold float64) bool {
	switch condition {
	case models.ConditionGreaterThan:
		return value >= threshold
	case models.ConditionLessThan:
		return value <= threshold
	case models.ConditionEqual:
		return value == threshold
	case models.ConditionNotEqual:
		return value != threshold
	default:
		m.logger.Warn("未知的条件类型", zap.String("condition", string(condition)))
		return false
	}
}

// triggerAlert 触发或更新告警
func (m *Manager) triggerAlert(alertKey string, rule *models.AlertRule, labels map[string]string, value float64) {
	m.alertsMu.Lock()
	defer m.alertsMu.Unlock()

	now := time.Now()

	// 检查是否已存在相同的告警
	alert, exists := m.activeAlerts[alertKey]
	if exists {
		// 更新现有告警
		alert.Value = value
		alert.UpdatedAt = now
		m.logger.Debug("更新现有告警", zap.String("id", alert.ID), zap.String("rule", rule.Name))
	} else {
		// 创建新告警
		alertID := uuid.New().String()
		alert = &models.Alert{
			ID:          alertID,
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			TargetType:  rule.TargetType,
			Metric:      rule.Metric,
			Labels:      labels,
			Value:       value,
			Threshold:   rule.Threshold,
			Condition:   rule.Condition,
			Severity:    rule.Severity,
			State:       models.AlertStateFiring,
			StartsAt:    now,
			Description: rule.Description,
			UpdatedAt:   now,
		}
		m.activeAlerts[alertKey] = alert

		// 发送告警通知
		m.notifierManager.SendAlert(alert, "triggered")
		m.logger.Info("触发新告警", zap.String("id", alertID), zap.String("rule", rule.Name))
	}
}

// resolveAlert 解决告警
func (m *Manager) resolveAlert(alertKey string) {
	m.alertsMu.Lock()
	defer m.alertsMu.Unlock()

	alert, exists := m.activeAlerts[alertKey]
	if !exists {
		return
	}

	now := time.Now()
	alert.State = models.AlertStateResolved
	alert.EndsAt = &now
	alert.UpdatedAt = now

	// 发送告警解决通知
	m.notifierManager.SendAlert(alert, "resolved")

	// 从活动告警中移除
	delete(m.activeAlerts, alertKey)
	m.logger.Info("告警已解决", zap.String("id", alert.ID), zap.String("rule", alert.RuleName))
}

// GetRules 获取所有告警规则
func (m *Manager) GetRules() []*models.AlertRule {
	m.alertsMu.RLock()
	defer m.alertsMu.RUnlock()

	rules := make([]*models.AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRule 获取指定ID的告警规则
func (m *Manager) GetRule(id string) (*models.AlertRule, error) {
	m.alertsMu.RLock()
	defer m.alertsMu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("告警规则不存在: %s", id)
	}
	return rule, nil
}

// CreateRule 创建新的告警规则
func (m *Manager) CreateRule(rule *models.AlertRule) error {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// 检查ID是否已存在
	existing, _ := m.GetRule(rule.ID)
	if existing != nil {
		return fmt.Errorf("告警规则ID已存在: %s", rule.ID)
	}

	// 序列化规则
	data, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("序列化告警规则失败: %w", err)
	}

	// 存储到etcd
	key := path.Join(m.alertPrefix, rule.ID)
	if err := m.etcdClient.Put(context.Background(), key, string(data)); err != nil {
		return fmt.Errorf("存储告警规则到etcd失败: %w", err)
	}

	return nil
}

// UpdateRule 更新告警规则
func (m *Manager) UpdateRule(rule *models.AlertRule) error {
	if rule.ID == "" {
		return fmt.Errorf("告警规则ID不能为空")
	}

	// 检查规则是否存在
	_, err := m.GetRule(rule.ID)
	if err != nil {
		return err
	}

	// 序列化规则
	data, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("序列化告警规则失败: %w", err)
	}

	// 存储到etcd
	key := path.Join(m.alertPrefix, rule.ID)
	if err := m.etcdClient.Put(context.Background(), key, string(data)); err != nil {
		return fmt.Errorf("更新告警规则到etcd失败: %w", err)
	}

	return nil
}

// DeleteRule 删除告警规则
func (m *Manager) DeleteRule(id string) error {
	// 检查规则是否存在
	_, err := m.GetRule(id)
	if err != nil {
		return err
	}

	// 从etcd中删除
	key := path.Join(m.alertPrefix, id)
	if err := m.etcdClient.Delete(context.Background(), key); err != nil {
		return fmt.Errorf("从etcd删除告警规则失败: %w", err)
	}

	return nil
}

// GetActiveAlerts 获取当前活动的告警
func (m *Manager) GetActiveAlerts() []*models.Alert {
	m.alertsMu.RLock()
	defer m.alertsMu.RUnlock()

	alerts := make([]*models.Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		alerts = append(alerts, alert)
	}
	return alerts
}
