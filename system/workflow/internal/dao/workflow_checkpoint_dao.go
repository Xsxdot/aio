package dao

import (
	"context"

	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/pkg/db/dialect"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	errorc "github.com/xsxdot/gokit/err"
	"github.com/xsxdot/gokit/logger"

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
	err := mvc.ExtractDB(ctx, d.db).Where("instance_id = ?", instanceID).
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

// deleteFromIndexDB 按 index 删除 checkpoint。PostgreSQL/MySQL 8+ 使用 ROW_NUMBER；MySQL 5.7 使用 load-then-delete 兜底
func (d *WorkflowCheckpointDao) deleteFromIndexDB(ctx context.Context, gdb *gorm.DB, instanceID int64, fromIndex int) error {
	if dialect.IsPostgres(gdb) {
		// PostgreSQL 与 MySQL 8+ 支持 ROW_NUMBER()
		return mvc.ExtractDB(ctx, d.db).Exec(`
			DELETE FROM aio_workflow_checkpoint 
			WHERE id IN (
				SELECT id FROM (
					SELECT id, ROW_NUMBER() OVER (ORDER BY created_at ASC) - 1 as rn 
					FROM aio_workflow_checkpoint 
					WHERE instance_id = ?
				) ranked WHERE rn >= ?
			)`, instanceID, fromIndex).Error
	}
	// MySQL 5.7 不支持 ROW_NUMBER，使用 load-then-delete 兜底
	var ids []uint
	if err := mvc.ExtractDB(ctx, d.db).Model(&model.WorkflowCheckpointModel{}).
		Where("instance_id = ?", instanceID).
		Order("created_at ASC").
		Offset(fromIndex).
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	return mvc.ExtractDB(ctx, d.db).Where("id IN ?", ids).Delete(&model.WorkflowCheckpointModel{}).Error
}
