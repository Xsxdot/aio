package model

import (
	"github.com/xsxdot/aio/pkg/core/model/common"
)

type WorkflowDefModel struct {
	common.Model
	Env     string `gorm:"column:env;size:50;not null;default:'dev';uniqueIndex:idx_env_code_version;index" json:"env" comment:"环境标识"`
	Code    string `gorm:"column:code;size:100;not null;uniqueIndex:idx_env_code_version" json:"code" comment:"模板唯一标识"`
	Version int32  `gorm:"column:version;not null;default:1;uniqueIndex:idx_env_code_version" json:"version" comment:"版本号"`
	Name    string `gorm:"column:name;size:255;not null" json:"name" comment:"名称"`
	DAGJSON string `gorm:"column:dag_json;type:json" json:"dag_json" comment:"DAG结构JSON"`
}

func (WorkflowDefModel) TableName() string {
	return "aio_workflow_def"
}
