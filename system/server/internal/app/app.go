package app

import (
	"xiaozhizhang/base"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/server/internal/service"
)

// App 服务器组件应用层
type App struct {
	ServerService       *service.ServerService
	ServerStatusService *service.ServerStatusService
	log                 *logger.Log
}

// NewApp 创建服务器组件应用层实例
func NewApp() *App {
	log := logger.GetLogger().WithEntryName("ServerApp")

	return &App{
		ServerService:       service.NewServerService(base.DB, log),
		ServerStatusService: service.NewServerStatusService(base.DB, log),
		log:                 log,
	}
}


