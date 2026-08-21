package model

import "github.com/xsxdot/aio/pkg/core/model/common"

// WorkflowAppliedCallbackModel 工作流回调幂等标记表。
//
// 职责：为「一次 executor 任务完成 → 一次工作流推进」建立唯一性约束，
// 使承载回调的 outbox 任务在重试时不会重复推进同一个节点。
//
// 边界：只做幂等判定，不承载任何业务状态，也不参与 DAG 推进决策。
// 不能用 aio_workflow_checkpoint 表代替——loopback 边有意允许同一节点
// 产生多条 checkpoint，(instance_id, node_id) 维度不唯一。
type WorkflowAppliedCallbackModel struct {
	common.Model
	JobID      int64  `gorm:"column:job_id;not null;uniqueIndex:idx_wac_job_id" json:"job_id" comment:"来源 executor 任务ID，幂等键"`
	InstanceID int64  `gorm:"column:instance_id;not null;index:idx_wac_instance" json:"instance_id" comment:"工作流实例ID"`
	NodeID     string `gorm:"column:node_id;size:100;not null" json:"node_id" comment:"节点ID"`
}

// TableName 指定表名。
func (WorkflowAppliedCallbackModel) TableName() string {
	return "aio_workflow_applied_callback"
}
