package dao

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
)

type WorkflowDefDao struct {
	mvc.IBaseDao[model.WorkflowDefModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

func NewWorkflowDefDao(db *gorm.DB, log *logger.Log) *WorkflowDefDao {
	return &WorkflowDefDao{
		IBaseDao: mvc.NewGormDao[model.WorkflowDefModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("WorkflowDefDao"),
		db:       db,
	}
}

// FindByCode 根据 code 获取最新版本的定义
func (d *WorkflowDefDao) FindByCode(ctx context.Context, code string) (*model.WorkflowDefModel, error) {
	var item model.WorkflowDefModel
	err := d.db.WithContext(ctx).Where("code = ?", code).Order("version desc").First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByIdWithTx 在事务内根据 ID 查询定义
func (d *WorkflowDefDao) FindByIdWithTx(ctx context.Context, tx *gorm.DB, id int64) (*model.WorkflowDefModel, error) {
	var item model.WorkflowDefModel
	err := tx.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}
