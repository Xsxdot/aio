package dao

import (
	"context"
	"time"

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

// ListInstancesFilter 实例列表筛选条件
type ListInstancesFilter struct {
	DefID         *int64  // 按 def_id 筛选
	DefCode       string  // 按 def code 筛选（需 join def 表）
	Status        string  // 按状态筛选
	CreatedAfter  int64   // 创建时间戳（秒）之后
	CreatedBefore int64   // 创建时间戳（秒）之前
}

// ListInstances 分页列出实例，支持 def_code、status、时间范围筛选
func (d *WorkflowInstanceDao) ListInstances(ctx context.Context, filter *ListInstancesFilter, pageNum, pageSize int32) ([]*model.WorkflowInstanceModel, int64, error) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	db := d.db.WithContext(ctx).Model(&model.WorkflowInstanceModel{})

	if filter != nil {
		if filter.DefID != nil {
			db = db.Where("def_id = ?", *filter.DefID)
		}
		if filter.DefCode != "" {
			db = db.Joins("INNER JOIN aio_workflow_def ON aio_workflow_def.id = aio_workflow_instance.def_id").
				Where("aio_workflow_def.code = ?", filter.DefCode)
		}
		if filter.Status != "" {
			db = db.Where("aio_workflow_instance.status = ?", filter.Status)
		}
		if filter.CreatedAfter > 0 {
			db = db.Where("aio_workflow_instance.created_at >= ?", time.Unix(filter.CreatedAfter, 0))
		}
		if filter.CreatedBefore > 0 {
			db = db.Where("aio_workflow_instance.created_at < ?", time.Unix(filter.CreatedBefore, 0))
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.WorkflowInstanceModel
	offset := (pageNum - 1) * pageSize
	if err := db.Order("aio_workflow_instance.created_at desc").Offset(int(offset)).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
