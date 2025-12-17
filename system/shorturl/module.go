package shorturl

import (
	"xiaozhizhang/pkg/core/logger"
	grpcservice "xiaozhizhang/system/shorturl/external/grpc"
	internalapp "xiaozhizhang/system/shorturl/internal/app"
)

// Module 短网址组件模块
type Module struct {
	internalApp *internalapp.App
	GRPCService *grpcservice.ShortURLService
}

// NewModule 创建短网址组件模块
func NewModule() *Module {
	log := logger.GetLogger().WithEntryName("ShortURLModule")

	// 创建内部 App
	app := internalapp.NewApp()

	// 创建 gRPC 服务
	grpcSvc := grpcservice.NewShortURLService(app, log)

	return &Module{
		internalApp: app,
		GRPCService: grpcSvc,
	}
}
