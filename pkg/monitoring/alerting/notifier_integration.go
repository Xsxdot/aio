// Package alerting 提供监控告警与通知器的集成
package alerting

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/notifier"
	"github.com/xsxdot/aio/pkg/notifier/adapter"
)

// NotifierIntegration 告警通知器集成
type NotifierIntegration struct {
	notifierManager   *notifier.Manager
	monitoringAdapter *adapter.MonitoringAdapter
	logger            *zap.Logger

	// 通知统计
	mu         sync.RWMutex
	statistics NotificationStatistics
}

// NotificationStatistics 通知统计信息
type NotificationStatistics struct {
	TotalSent    int64 `json:"total_sent"`
	TotalSuccess int64 `json:"total_success"`
	TotalFailed  int64 `json:"total_failed"`
	LastSentAt   int64 `json:"last_sent_at"`
}

// NewNotifierIntegration 创建通知器集成
func NewNotifierIntegration(notifierManager *notifier.Manager, logger *zap.Logger) *NotifierIntegration {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &NotifierIntegration{
		notifierManager:   notifierManager,
		monitoringAdapter: adapter.NewMonitoringAdapter(),
		logger:            logger,
		statistics:        NotificationStatistics{},
	}
}

// SendAlert 发送告警通知
func (ni *NotifierIntegration) SendAlert(alert *models.Alert, eventType string) []notifier.NotificationResult {
	if alert == nil {
		ni.logger.Error("告警为空，无法发送通知")
		return nil
	}

	// 转换告警为通用通知格式
	notification := ni.monitoringAdapter.AlertToNotification(alert, eventType)
	if notification == nil {
		ni.logger.Error("转换告警为通知失败",
			zap.String("alert_id", alert.ID),
			zap.String("event_type", eventType))
		return nil
	}

	ni.logger.Info("发送告警通知",
		zap.String("alert_id", alert.ID),
		zap.String("rule_name", alert.RuleName),
		zap.String("event_type", eventType),
		zap.String("severity", string(alert.Severity)),
		zap.String("notification_id", notification.ID))

	// 发送通知
	results := ni.notifierManager.SendNotification(notification)

	// 更新统计信息
	ni.updateStatistics(results)

	// 记录发送结果
	ni.logNotificationResults(results, alert)

	return results
}

// SendAlertBatch 批量发送告警通知
func (ni *NotifierIntegration) SendAlertBatch(alerts []*models.Alert, eventType string) map[string][]notifier.NotificationResult {
	if len(alerts) == 0 {
		return nil
	}

	results := make(map[string][]notifier.NotificationResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	ni.logger.Info("批量发送告警通知",
		zap.Int("alert_count", len(alerts)),
		zap.String("event_type", eventType))

	// 并发发送告警
	for _, alert := range alerts {
		wg.Add(1)
		go func(a *models.Alert) {
			defer wg.Done()

			alertResults := ni.SendAlert(a, eventType)
			if len(alertResults) > 0 {
				mu.Lock()
				results[a.ID] = alertResults
				mu.Unlock()
			}
		}(alert)
	}

	wg.Wait()
	return results
}

// SendCustomNotification 发送自定义通知（非告警）
func (ni *NotifierIntegration) SendCustomNotification(notification *notifier.Notification) []notifier.NotificationResult {
	if notification == nil {
		ni.logger.Error("通知为空，无法发送")
		return nil
	}

	ni.logger.Info("发送自定义通知",
		zap.String("notification_id", notification.ID),
		zap.String("title", notification.Title),
		zap.String("level", string(notification.Level)))

	results := ni.notifierManager.SendNotification(notification)
	ni.updateStatistics(results)

	return results
}

// updateStatistics 更新统计信息
func (ni *NotifierIntegration) updateStatistics(results []notifier.NotificationResult) {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	ni.statistics.TotalSent += int64(len(results))
	ni.statistics.LastSentAt = time.Now().Unix()

	for _, result := range results {
		if result.Success {
			ni.statistics.TotalSuccess++
		} else {
			ni.statistics.TotalFailed++
		}
	}
}

// logNotificationResults 记录通知发送结果
func (ni *NotifierIntegration) logNotificationResults(results []notifier.NotificationResult, alert *models.Alert) {
	successCount := 0
	failureCount := 0

	for _, result := range results {
		if result.Success {
			successCount++
			ni.logger.Debug("通知发送成功",
				zap.String("alert_id", alert.ID),
				zap.String("notifier_id", result.NotifierID),
				zap.String("notifier_name", result.NotifierName),
				zap.Int64("response_time", result.ResponseTime))
		} else {
			failureCount++
			ni.logger.Warn("通知发送失败",
				zap.String("alert_id", alert.ID),
				zap.String("notifier_id", result.NotifierID),
				zap.String("notifier_name", result.NotifierName),
				zap.String("error", result.Error),
				zap.Int64("response_time", result.ResponseTime))
		}
	}

	ni.logger.Info("告警通知发送完成",
		zap.String("alert_id", alert.ID),
		zap.Int("total_notifiers", len(results)),
		zap.Int("success_count", successCount),
		zap.Int("failure_count", failureCount))
}

// GetStatistics 获取通知统计信息
func (ni *NotifierIntegration) GetStatistics() NotificationStatistics {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	return ni.statistics
}

// ResetStatistics 重置统计信息
func (ni *NotifierIntegration) ResetStatistics() {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	ni.statistics = NotificationStatistics{}
	ni.logger.Info("通知统计信息已重置")
}

// HealthCheck 健康检查
func (ni *NotifierIntegration) HealthCheck(ctx context.Context) error {
	// 检查通知器管理器状态
	notifiers := ni.notifierManager.GetNotifiers()
	if len(notifiers) == 0 {
		ni.logger.Warn("没有配置任何通知器")
	}

	// 发送测试通知（可选）
	testNotification := &notifier.Notification{
		ID:        "health-check-" + time.Now().Format("20060102150405"),
		Title:     "健康检查",
		Content:   "这是一条健康检查通知",
		Level:     notifier.NotificationLevelInfo,
		CreatedAt: time.Now(),
		Labels: map[string]string{
			"type":   "health_check",
			"source": "monitoring",
		},
	}

	// 仅在测试模式下发送
	if ctx.Value("test_mode") == true {
		results := ni.notifierManager.SendNotification(testNotification)
		for _, result := range results {
			if !result.Success {
				ni.logger.Error("健康检查通知发送失败",
					zap.String("notifier_id", result.NotifierID),
					zap.String("error", result.Error))
			}
		}
	}

	return nil
}

// GetEnabledNotifiers 获取启用的通知器列表
func (ni *NotifierIntegration) GetEnabledNotifiers() []*notifier.NotifierConfig {
	return ni.notifierManager.GetNotifiers()
}
