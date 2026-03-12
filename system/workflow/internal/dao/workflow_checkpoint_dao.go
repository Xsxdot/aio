package dao

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
)

type WorkflowCheckpointDao struct {
	mvc.IBaseDao[model.WorkflowCheckpointModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

func NewWorkflowCheckpointDao(db *gorm.DB, log *logger.Log) *WorkflowCheckpointDao {
	return &WorkflowCheckpointDao{
		IBaseDao: mvc.NewGormDao[model.WorkflowCheckpointModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("WorkflowCheckpointDao"),
		db:       db,
	}
}

func (d *WorkflowCheckpointDao) WithTx(tx interface{}) mvc.IBaseDao[model.WorkflowCheckpointModel] {
	if gtx, ok := tx.(*gorm.DB); ok {
		return &WorkflowCheckpointDao{
			IBaseDao: mvc.NewGormDao[model.WorkflowCheckpointModel](gtx),
			log:      d.log,
			err:      d.err,
			db:       gtx,
		}
	}
	return d
}

func (d *WorkflowCheckpointDao) ListByInstanceIDOrderByCreatedAsc(ctx context.Context, instanceID int64) ([]*model.WorkflowCheckpointModel, error) {
	return d.listByInstanceIDOrderByCreatedAscDB(ctx, d.db, instanceID)
}

// ListByInstanceIDOrderByCreatedAscWithTx 在事务内按创建时间升序列出 checkpoint
func (d *WorkflowCheckpointDao) ListByInstanceIDOrderByCreatedAscWithTx(ctx context.Context, tx *gorm.DB, instanceID int64) ([]*model.WorkflowCheckpointModel, error) {
	return d.listByInstanceIDOrderByCreatedAscDB(ctx, tx, instanceID)
}

func (d *WorkflowCheckpointDao) listByInstanceIDOrderByCreatedAscDB(ctx context.Context, db *gorm.DB, instanceID int64) ([]*model.WorkflowCheckpointModel, error) {
	var list []*model.WorkflowCheckpointModel
	err := db.WithContext(ctx).Where("instance_id = ?", instanceID).
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *WorkflowCheckpointDao) DeleteFromIndex(ctx context.Context, instanceID int64, fromIndex int) error {
	return d.deleteFromIndexDB(ctx, d.db, instanceID, fromIndex)
}

// DeleteFromIndexWithTx 在事务内按 index 删除 checkpoint，使用子查询避免全量加载
func (d *WorkflowCheckpointDao) DeleteFromIndexWithTx(ctx context.Context, tx *gorm.DB, instanceID int64, fromIndex int) error {
	return d.deleteFromIndexDB(ctx, tx, instanceID, fromIndex)
}

// deleteFromIndexDB 使用 ROW_NUMBER 子查询直接删除，避免加载全表到内存
func (d *WorkflowCheckpointDao) deleteFromIndexDB(ctx context.Context, db *gorm.DB, instanceID int64, fromIndex int) error {
	return db.WithContext(ctx).Exec(`
		DELETE FROM aio_workflow_checkpoint 
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (ORDER BY created_at ASC) - 1 as rn 
				FROM aio_workflow_checkpoint 
				WHERE instance_id = ?
			) ranked WHERE rn >= ?
		)`, instanceID, fromIndex).Error
}
