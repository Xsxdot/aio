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

type WorkflowDefService struct {
	mvc.IBaseService[model.WorkflowDefModel]
	dao *dao.WorkflowDefDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

func NewWorkflowDefService(dao *dao.WorkflowDefDao, log *logger.Log) *WorkflowDefService {
	return &WorkflowDefService{
		IBaseService: mvc.NewBaseService[model.WorkflowDefModel](dao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("WorkflowDefService"),
	}
}

func (s *WorkflowDefService) FindByCode(ctx context.Context, code string) (*model.WorkflowDefModel, error) {
	def, err := s.dao.FindByCode(ctx, code)
	if err != nil {
		return nil, s.err.New("查询工作流定义失败", err).DB()
	}
	return def, nil
}

// FindByIdWithTx 在事务内根据 ID 查询定义
func (s *WorkflowDefService) FindByIdWithTx(ctx context.Context, tx *gorm.DB, id int64) (*model.WorkflowDefModel, error) {
	def, err := s.dao.FindByIdWithTx(ctx, tx, id)
	if err != nil {
		return nil, s.err.New("查询工作流定义失败", err).DB()
	}
	return def, nil
}
