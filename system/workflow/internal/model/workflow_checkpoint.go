package model

import (
	"github.com/xsxdot/aio/pkg/core/model/common"
)

type WorkflowCheckpointModel struct {
	common.Model
	InstanceID int64  `gorm:"column:instance_id;not null;index" json:"instance_id" comment:"实例ID"`
	NodeID     string `gorm:"column:node_id;size:100;not null" json:"node_id" comment:"节点ID"`
	NodeOutput string `gorm:"column:node_output;type:json" json:"node_output" comment:"节点输出JSON"`
	StateAfter string `gorm:"column:state_after;type:json" json:"state_after" comment:"执行后完整状态JSON"`
}

func (WorkflowCheckpointModel) TableName() string {
	return "aio_workflow_checkpoint"
}
