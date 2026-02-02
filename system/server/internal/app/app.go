package app

import (
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/server/internal/dao"
	"github.com/xsxdot/aio/system/server/internal/service"
)

// App 服务器组件应用层
type App struct {
	ServerService          *service.ServerService
	ServerStatusService    *service.ServerStatusService
	ServerSSHCredentialSvc *service.ServerSSHCredentialService
	log                    *logger.Log
	errBuilder             *errorc.ErrorBuilder
	dialContext            DialContextFunc // 可注入的 TCP dial 函数（用于测试）
}

// NewApp 创建服务器组件应用层实例
func NewApp() *App {
	log := logger.GetLogger().WithEntryName("ServerApp")

	// 初始化 DAO
	sshCredentialDao := dao.NewServerSSHCredentialDao(base.DB, log)

	// 初始化 Service
	cryptoSvc := service.NewCryptoService(log)
	sshCredentialSvc := service.NewServerSSHCredentialService(sshCredentialDao, cryptoSvc, log)

	return &App{
		ServerService:          service.NewServerService(base.DB, log),
		ServerStatusService:    service.NewServerStatusService(base.DB, log),
		ServerSSHCredentialSvc: sshCredentialSvc,
		log:                    log,
		errBuilder:             errorc.NewErrorBuilder("ServerApp"),
		dialContext:            nil, // 默认使用 defaultDialContext
	}
}
