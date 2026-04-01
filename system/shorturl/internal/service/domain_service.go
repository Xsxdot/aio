package service

import (
	"context"

	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/shorturl/internal/dao"
	"github.com/xsxdot/aio/system/shorturl/internal/model"
	errorc "github.com/xsxdot/gokit/err"
	"github.com/xsxdot/gokit/logger"
)

// DomainService 短域名业务逻辑层
type DomainService struct {
	mvc.IBaseService[model.ShortDomain]
	Dao *dao.DomainDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewDomainService 创建短域名服务实例
func NewDomainService(daoInstance *dao.DomainDao, log *logger.Log) *DomainService {
	return &DomainService{
		IBaseService: mvc.NewBaseService[model.ShortDomain](daoInstance),
		Dao:          daoInstance,
		log:          log.WithEntryName("DomainService"),
		err:          errorc.NewErrorBuilder("DomainService"),
	}
}

// FindByDomain 根据域名查找配置
func (s *DomainService) FindByDomain(ctx context.Context, host string) (*model.ShortDomain, error) {
	return s.Dao.FindByDomain(ctx, host)
}

// FindDefault 获取默认域名配置
func (s *DomainService) FindDefault(ctx context.Context) (*model.ShortDomain, error) {
	return s.Dao.FindDefault(ctx)
}







