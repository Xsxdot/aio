package app

import (
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/config/internal/dao"
	"xiaozhizhang/system/config/internal/service"
)

// App 配置中心应用组合根
type App struct {
	ConfigItemService    *service.ConfigItemService
	ConfigHistoryService *service.ConfigHistoryService
	EncryptionService    *service.EncryptionService
	log                  *logger.Log
	err                  *errorc.ErrorBuilder
}

// NewApp 创建配置中心应用实例
func NewApp() *App {
	log := base.Logger.WithEntryName("ConfigApp")

	// 获取加密盐值
	salt := base.Configures.Config.ConfigCenter.EncryptionSalt

	// 创建 DAO
	configItemDao := dao.NewConfigItemDao(base.DB, log)
	configHistoryDao := dao.NewConfigHistoryDao(base.DB, log)

	// 创建 Service
	encryptionService := service.NewEncryptionService(salt, log)
	configItemService := service.NewConfigItemService(configItemDao, encryptionService, log)
	configHistoryService := service.NewConfigHistoryService(configHistoryDao, log)

	return &App{
		ConfigItemService:    configItemService,
		ConfigHistoryService: configHistoryService,
		EncryptionService:    encryptionService,
		log:                  log,
		err:                  errorc.NewErrorBuilder("ConfigApp"),
	}
}
