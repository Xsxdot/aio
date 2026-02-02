package service

import (
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/shorturl/internal/dao"
	"github.com/xsxdot/aio/system/shorturl/internal/model"
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


