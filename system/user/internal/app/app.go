package app

import (
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/user/internal/dao"
	"github.com/xsxdot/aio/system/user/internal/service"

	"gorm.io/gorm"
)

// App 用户组件应用组合根
type App struct {
	AdminService            *service.AdminService
	ClientCredentialService *service.ClientCredentialService
	log                     *logger.Log
	err                     *errorc.ErrorBuilder
	db                      *gorm.DB
}

// NewApp 创建用户应用实例
func NewApp() *App {
	log := base.Logger.WithEntryName("UserApp")

	// 创建 DAO
	adminDao := dao.NewAdminDao(base.DB, log)
	clientCredentialDao := dao.NewClientCredentialDao(base.DB, log)

	// 创建 Service
	adminService := service.NewAdminService(adminDao, log)
	clientCredentialService := service.NewClientCredentialService(clientCredentialDao, log)

	return &App{
		AdminService:            adminService,
		ClientCredentialService: clientCredentialService,
		log:                     log,
		err:                     errorc.NewErrorBuilder("UserApp"),
		db:                      base.DB,
	}
}



