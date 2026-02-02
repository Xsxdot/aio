package app

import (
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/registry/internal/dao"
	"github.com/xsxdot/aio/system/registry/internal/service"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// App 注册中心组件应用组合根
type App struct {
	ServiceDao  *dao.RegistryServiceDao
	InstanceDao *dao.RegistryInstanceDao
	ServiceSvc  *service.RegistryServiceService
	InstanceSvc *service.RegistryInstanceService
	log         *logger.Log
	err         *errorc.ErrorBuilder
	db          *gorm.DB
	rdb         *redis.Client
	cache       *cache.Cache
}

// NewApp 创建注册中心组件应用实例
func NewApp() *App {
	log := base.Logger.WithEntryName("RegistryApp")

	serviceDao := dao.NewRegistryServiceDao(base.DB, log)
	instanceDao := dao.NewRegistryInstanceDao(base.DB, log)

	serviceSvc := service.NewRegistryServiceService(serviceDao, log)
	instanceSvc := service.NewRegistryInstanceService(instanceDao, log)

	return &App{
		ServiceDao:  serviceDao,
		InstanceDao: instanceDao,
		ServiceSvc:  serviceSvc,
		InstanceSvc: instanceSvc,
		log:         log,
		err:         errorc.NewErrorBuilder("RegistryApp"),
		db:          base.DB,
		rdb:         base.RDB,
		cache:       base.Cache,
	}
}
