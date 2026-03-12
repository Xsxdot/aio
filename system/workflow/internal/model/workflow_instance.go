package model

import (
	"github.com/xsxdot/aio/pkg/core/model/common"
)

type WorkflowInstanceStatus string

const (
	InstanceStatusRunning   WorkflowInstanceStatus = "RUNNING"
	InstanceStatusWaiting   WorkflowInstanceStatus = "WAITING"
	InstanceStatusCompleted WorkflowInstanceStatus = "COMPLETED"
	InstanceStatusFailed    WorkflowInstanceStatus = "FAILED"
	InstanceStatusCanceled  WorkflowInstanceStatus = "CANCELED"
)

type WorkflowInstanceModel struct {
	common.Model
	DefID         int64                  `gorm:"column:def_id;not null;index" json:"def_id" comment:"关联的定义ID"`
	DefVersion    int32                  `gorm:"column:def_version;not null" json:"def_version" comment:"关联的定义版本"`
	Env           string                 `gorm:"column:env;size:50;default:'';index" json:"env" comment:"环境标识（如 dev/prod/test），用于 Executor 任务隔离"`
	Status        WorkflowInstanceStatus `gorm:"column:status;size:20;not null;index" json:"status" comment:"实例状态"`
	InitialState  string                 `gorm:"column:initial_state;type:json" json:"initial_state" comment:"启动时初始状态JSON，用于回滚到起始节点"`
	CurrentState  string                 `gorm:"column:current_state;type:json" json:"current_state" comment:"当前全局状态JSON"`
	ActiveNodeIDs string                 `gorm:"column:active_node_ids;type:json" json:"active_node_ids" comment:"当前活跃节点列表JSON"`
}

func (WorkflowInstanceModel) TableName() string {
	return "aio_workflow_instance"
}
