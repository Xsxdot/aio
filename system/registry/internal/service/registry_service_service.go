package service

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/registry/internal/dao"
	"github.com/xsxdot/aio/system/registry/internal/model"

	"gorm.io/gorm"
)

// RegistryServiceService 服务定义业务逻辑层
type RegistryServiceService struct {
	mvc.IBaseService[model.RegistryService]
	dao *dao.RegistryServiceDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

func NewRegistryServiceService(dao *dao.RegistryServiceDao, log *logger.Log) *RegistryServiceService {
	return &RegistryServiceService{
		IBaseService: mvc.NewBaseService[model.RegistryService](dao.IBaseDao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("RegistryServiceService"),
	}
}

func (s *RegistryServiceService) FindByKey(ctx context.Context, project, name string) (*model.RegistryService, error) {
	return s.dao.FindByKey(ctx, project, name)
}

func (s *RegistryServiceService) ListByFilter(ctx context.Context, project string) ([]*model.RegistryService, error) {
	return s.dao.ListByFilter(ctx, project)
}

func (s *RegistryServiceService) WithTx(tx *gorm.DB) *RegistryServiceService {
	return &RegistryServiceService{
		IBaseService: s.IBaseService.WithTx(tx),
		dao:          s.dao.WithTx(tx),
		log:          s.log,
		err:          s.err,
	}
}
