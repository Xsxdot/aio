// Package models 定义告警和通知相关的数据模型
package models

import (
	"time"
)

// AlertSeverity 表示告警的严重程度
type AlertSeverity string

const (
	// AlertSeverityInfo 信息级别的告警
	AlertSeverityInfo AlertSeverity = "info"
	// AlertSeverityWarning 警告级别的告警
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityCritical 严重级别的告警
	AlertSeverityCritical AlertSeverity = "critical"
	// AlertSeverityEmergency 紧急级别的告警
	AlertSeverityEmergency AlertSeverity = "emergency"
)

// AlertConditionType 表示告警条件的类型
type AlertConditionType string

const (
	// ConditionGreaterThan 大于条件
	ConditionGreaterThan AlertConditionType = "gt"
	// ConditionLessThan 小于条件
	ConditionLessThan AlertConditionType = "lt"
	// ConditionGreaterThanOrEqual 大于等于条件
	ConditionGreaterThanOrEqual AlertConditionType = "gte"
	// ConditionLessThanOrEqual 小于等于条件
	ConditionLessThanOrEqual AlertConditionType = "lte"
	// ConditionEqual 等于条件
	ConditionEqual AlertConditionType = "eq"
	// ConditionNotEqual 不等于条件
	ConditionNotEqual AlertConditionType = "neq"
	// ConditionChangeRate 变化率条件
	ConditionChangeRate AlertConditionType = "change_rate"
)

// AlertTargetType 表示告警目标的类型
type AlertTargetType string

const (
	// AlertTargetServer 服务器指标
	AlertTargetServer AlertTargetType = "server"
	// AlertTargetApplication 应用指标
	AlertTargetApplication AlertTargetType = "application"
)

// AlertState 表示告警的状态
type AlertState string

const (
	// AlertStatePending 待定状态
	AlertStatePending AlertState = "pending"
	// AlertStateFiring 触发状态
	AlertStateFiring AlertState = "firing"
	// AlertStateResolved 已解决状态
	AlertStateResolved AlertState = "resolved"
)

// AlertRule 表示一个告警规则
type AlertRule struct {
	// 规则ID
	ID string `json:"id"`
	// 规则名称
	Name string `json:"name"`
	// 目标类型（服务器/应用）
	TargetType AlertTargetType `json:"target_type"`
	// 指标名称
	Metric string `json:"metric"`
	// 标签匹配器（可选）
	LabelMatchers map[string]string `json:"label_matchers,omitempty"`
	// 条件类型
	Condition AlertConditionType `json:"condition"`
	// 阈值
	Threshold float64 `json:"threshold"`
	// 持续时间（例如：5m表示5分钟）
	Duration string `json:"duration"`
	// 告警严重程度
	Severity AlertSeverity `json:"severity"`
	// 通知者ID列表
	Notifiers []string `json:"notifiers"`
	// 告警描述
	Description string `json:"description"`
	// 是否启用
	Enabled bool `json:"enabled"`
}

// Alert 表示一个具体的告警实例
type Alert struct {
	// 告警ID
	ID string `json:"id"`
	// 关联的规则ID
	RuleID string `json:"rule_id"`
	// 规则名称
	RuleName string `json:"rule_name"`
	// 目标类型
	TargetType AlertTargetType `json:"target_type"`
	// 指标名称
	Metric string `json:"metric"`
	// 标签
	Labels map[string]string `json:"labels"`
	// 当前值
	Value float64 `json:"value"`
	// 阈值
	Threshold float64 `json:"threshold"`
	// 条件类型
	Condition AlertConditionType `json:"condition"`
	// 严重程度
	Severity AlertSeverity `json:"severity"`
	// 告警状态
	State AlertState `json:"state"`
	// 开始时间
	StartsAt time.Time `json:"starts_at"`
	// 结束时间（如果已解决）
	EndsAt *time.Time `json:"ends_at,omitempty"`
	// 告警描述
	Description string `json:"description"`
	// 最后更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertEvent 表示告警事件，用于通知
type AlertEvent struct {
	// 告警实例
	Alert Alert `json:"alert"`
	// 事件类型（triggered, resolved）
	EventType string `json:"event_type"`
	// 事件时间
	Timestamp time.Time `json:"timestamp"`
}
