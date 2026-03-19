package service

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/dao"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
)

type WorkflowInstanceService struct {
	mvc.IBaseService[model.WorkflowInstanceModel]
	dao *dao.WorkflowInstanceDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

func NewWorkflowInstanceService(dao *dao.WorkflowInstanceDao, log *logger.Log) *WorkflowInstanceService {
	return &WorkflowInstanceService{
		IBaseService: mvc.NewBaseService[model.WorkflowInstanceModel](dao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("WorkflowInstanceService"),
	}
}

// FindByIdForUpdate 带行锁查询实例，必须在事务内使用
func (s *WorkflowInstanceService) FindByIdForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.WorkflowInstanceModel, error) {
	return s.dao.FindByIdForUpdate(ctx, tx, id)
}

// SaveWithTx 在事务内保存实例
func (s *WorkflowInstanceService) SaveWithTx(ctx context.Context, tx *gorm.DB, entity *model.WorkflowInstanceModel) error {
	return s.dao.SaveWithTx(ctx, tx, entity)
}

// ListInstances 分页列出实例
func (s *WorkflowInstanceService) ListInstances(ctx context.Context, filter *dao.ListInstancesFilter, pageNum, pageSize int32) ([]*model.WorkflowInstanceModel, int64, error) {
	return s.dao.ListInstances(ctx, filter, pageNum, pageSize)
}
