// Package adapter 提供不同数据源到通用通知的适配器
package adapter

import (
	"fmt"
	"time"

	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/notifier"
)

// MonitoringAdapter 监控告警适配器
type MonitoringAdapter struct{}

// NewMonitoringAdapter 创建监控告警适配器
func NewMonitoringAdapter() *MonitoringAdapter {
	return &MonitoringAdapter{}
}

// AlertToNotification 将告警转换为通知
func (a *MonitoringAdapter) AlertToNotification(alert *models.Alert, eventType string) *notifier.Notification {
	if alert == nil {
		return nil
	}

	// 确定通知级别
	level := a.mapSeverityToLevel(alert.Severity)

	// 构建标题
	title := fmt.Sprintf("%s - %s", alert.RuleName, alert.Description)
	if eventType == "resolved" {
		title = fmt.Sprintf("【已解决】%s", title)
	}

	// 构建内容
	content := a.buildContent(alert, eventType)

	// 构建标签
	labels := make(map[string]string)
	if alert.Labels != nil {
		for k, v := range alert.Labels {
			labels[k] = v
		}
	}
	labels["rule_id"] = alert.RuleID
	labels["target_type"] = string(alert.TargetType)
	labels["metric"] = alert.Metric

	// 构建额外数据
	data := map[string]interface{}{
		"alert_id":    alert.ID,
		"rule_id":     alert.RuleID,
		"rule_name":   alert.RuleName,
		"target_type": alert.TargetType,
		"metric":      alert.Metric,
		"value":       alert.Value,
		"threshold":   alert.Threshold,
		"condition":   alert.Condition,
		"severity":    alert.Severity,
		"state":       alert.State,
		"starts_at":   alert.StartsAt,
		"event_type":  eventType,
	}

	if alert.EndsAt != nil {
		data["ends_at"] = *alert.EndsAt
	}

	return &notifier.Notification{
		ID:        fmt.Sprintf("alert-%s-%d", alert.ID, time.Now().UnixNano()),
		Title:     title,
		Content:   content,
		Level:     level,
		Labels:    labels,
		CreatedAt: time.Now(),
		Data:      data,
	}
}

// mapSeverityToLevel 映射告警严重程度到通知级别
func (a *MonitoringAdapter) mapSeverityToLevel(severity models.AlertSeverity) notifier.NotificationLevel {
	switch severity {
	case models.AlertSeverityInfo:
		return notifier.NotificationLevelInfo
	case models.AlertSeverityWarning:
		return notifier.NotificationLevelWarning
	case models.AlertSeverityCritical:
		return notifier.NotificationLevelCritical
	case models.AlertSeverityEmergency:
		return notifier.NotificationLevelCritical
	default:
		return notifier.NotificationLevelInfo
	}
}

// buildContent 构建通知内容
func (a *MonitoringAdapter) buildContent(alert *models.Alert, eventType string) string {
	content := fmt.Sprintf("告警规则: %s\n", alert.RuleName)
	content += fmt.Sprintf("告警描述: %s\n", alert.Description)
	content += fmt.Sprintf("告警状态: %s\n", a.getEventTypeDisplay(eventType))
	content += fmt.Sprintf("严重程度: %s\n", alert.Severity)
	content += fmt.Sprintf("指标名称: %s\n", alert.Metric)
	content += fmt.Sprintf("当前值: %.2f\n", alert.Value)
	content += fmt.Sprintf("阈值: %.2f\n", alert.Threshold)
	content += fmt.Sprintf("条件: %s\n", alert.Condition)
	content += fmt.Sprintf("开始时间: %s\n", alert.StartsAt.Format("2006-01-02 15:04:05"))

	if alert.EndsAt != nil {
		content += fmt.Sprintf("结束时间: %s\n", alert.EndsAt.Format("2006-01-02 15:04:05"))
	}

	if len(alert.Labels) > 0 {
		content += "\n标签信息:\n"
		for k, v := range alert.Labels {
			content += fmt.Sprintf("- %s: %s\n", k, v)
		}
	}

	return content
}

// getEventTypeDisplay 获取事件类型显示名称
func (a *MonitoringAdapter) getEventTypeDisplay(eventType string) string {
	switch eventType {
	case "triggered":
		return "已触发"
	case "resolved":
		return "已解决"
	default:
		return eventType
	}
}
