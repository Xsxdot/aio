package dao

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowInstanceDao struct {
	mvc.IBaseDao[model.WorkflowInstanceModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

func NewWorkflowInstanceDao(db *gorm.DB, log *logger.Log) *WorkflowInstanceDao {
	return &WorkflowInstanceDao{
		IBaseDao: mvc.NewGormDao[model.WorkflowInstanceModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("WorkflowInstanceDao"),
		db:       db,
	}
}

// FindByIdForUpdate 带行锁查询实例，必须在事务内使用
func (d *WorkflowInstanceDao) FindByIdForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.WorkflowInstanceModel, error) {
	var entity model.WorkflowInstanceModel
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// SaveWithTx 在事务内保存实例
func (d *WorkflowInstanceDao) SaveWithTx(ctx context.Context, tx *gorm.DB, entity *model.WorkflowInstanceModel) error {
	return tx.WithContext(ctx).Save(entity).Error
}
