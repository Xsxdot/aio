package config

import (
	"xiaozhizhang/base"
	"xiaozhizhang/system/config/api/client"
	grpcsvc "xiaozhizhang/system/config/external/grpc"
	"xiaozhizhang/system/config/internal/app"
)

// Module 配置中心模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用配置中心能力
	Client *client.ConfigClient
	// GRPCService gRPC服务实例，供gRPC服务器注册使用
	GRPCService *grpcsvc.ConfigService
}

// NewModule 创建配置中心模块实例
func NewModule() *Module {
	internalApp := app.NewApp()
	configClient := client.NewConfigClient(internalApp)
	grpcService := grpcsvc.NewConfigService(configClient, internalApp, base.Logger)

	return &Module{
		internalApp: internalApp,
		Client:      configClient,
		GRPCService: grpcService,
	}
}
