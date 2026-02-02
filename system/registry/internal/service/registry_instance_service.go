package service

import (
	"context"
	"time"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/registry/internal/dao"
	"github.com/xsxdot/aio/system/registry/internal/model"

	"gorm.io/gorm"
)

// RegistryInstanceService 实例登记业务逻辑层
type RegistryInstanceService struct {
	mvc.IBaseService[model.RegistryInstance]
	dao *dao.RegistryInstanceDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

func NewRegistryInstanceService(dao *dao.RegistryInstanceDao, log *logger.Log) *RegistryInstanceService {
	return &RegistryInstanceService{
		IBaseService: mvc.NewBaseService[model.RegistryInstance](dao.IBaseDao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("RegistryInstanceService"),
	}
}

func (s *RegistryInstanceService) FindByServiceAndKey(ctx context.Context, serviceID int64, instanceKey string) (*model.RegistryInstance, error) {
	return s.dao.FindByServiceAndKey(ctx, serviceID, instanceKey)
}

func (s *RegistryInstanceService) ListByServiceID(ctx context.Context, serviceID int64) ([]*model.RegistryInstance, error) {
	return s.dao.ListByServiceID(ctx, serviceID)
}

func (s *RegistryInstanceService) ListByServiceIDAndEnv(ctx context.Context, serviceID int64, env string) ([]*model.RegistryInstance, error) {
	return s.dao.ListByServiceIDAndEnv(ctx, serviceID, env)
}

func (s *RegistryInstanceService) Upsert(ctx context.Context, inst *model.RegistryInstance) error {
	if inst.LastHeartbeatAt.IsZero() {
		inst.LastHeartbeatAt = time.Now()
	}
	return s.dao.Upsert(ctx, inst)
}

func (s *RegistryInstanceService) UpdateHeartbeat(ctx context.Context, serviceID int64, instanceKey string, at time.Time) error {
	return s.dao.UpdateHeartbeat(ctx, serviceID, instanceKey, at)
}

func (s *RegistryInstanceService) DeleteByServiceAndKey(ctx context.Context, serviceID int64, instanceKey string) error {
	return s.dao.DeleteByServiceAndKey(ctx, serviceID, instanceKey)
}

func (s *RegistryInstanceService) WithTx(tx *gorm.DB) *RegistryInstanceService {
	return &RegistryInstanceService{
		IBaseService: s.IBaseService.WithTx(tx),
		dao:          s.dao.WithTx(tx),
		log:          s.log,
		err:          s.err,
	}
}
