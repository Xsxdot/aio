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

type WorkflowCheckpointService struct {
	mvc.IBaseService[model.WorkflowCheckpointModel]
	dao *dao.WorkflowCheckpointDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

func NewWorkflowCheckpointService(dao *dao.WorkflowCheckpointDao, log *logger.Log) *WorkflowCheckpointService {
	return &WorkflowCheckpointService{
		IBaseService: mvc.NewBaseService[model.WorkflowCheckpointModel](dao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("WorkflowCheckpointService"),
	}
}

func (s *WorkflowCheckpointService) ListByInstanceIDOrderByCreatedAsc(ctx context.Context, instanceID int64) ([]*model.WorkflowCheckpointModel, error) {
	return s.dao.ListByInstanceIDOrderByCreatedAsc(ctx, instanceID)
}

// ListByInstanceIDOrderByCreatedAscWithTx 在事务内列出 checkpoint
func (s *WorkflowCheckpointService) ListByInstanceIDOrderByCreatedAscWithTx(ctx context.Context, tx *gorm.DB, instanceID int64) ([]*model.WorkflowCheckpointModel, error) {
	return s.dao.ListByInstanceIDOrderByCreatedAscWithTx(ctx, tx, instanceID)
}

func (s *WorkflowCheckpointService) DeleteFromIndex(ctx context.Context, instanceID int64, fromIndex int) error {
	return s.dao.DeleteFromIndex(ctx, instanceID, fromIndex)
}

// DeleteFromIndexWithTx 在事务内按 index 删除 checkpoint
func (s *WorkflowCheckpointService) DeleteFromIndexWithTx(ctx context.Context, tx *gorm.DB, instanceID int64, fromIndex int) error {
	return s.dao.DeleteFromIndexWithTx(ctx, tx, instanceID, fromIndex)
}
