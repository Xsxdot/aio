package registry

import (
	"xiaozhizhang/base"
	"xiaozhizhang/system/registry/api/client"
	grpcsvc "xiaozhizhang/system/registry/external/grpc"
	"xiaozhizhang/system/registry/internal/app"
)

// Module 注册中心组件模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用注册中心能力
	Client *client.RegistryClient
	// GRPCService gRPC服务实例，供gRPC服务器注册使用
	GRPCService *grpcsvc.RegistryService
}

// NewModule 创建注册中心模块实例
func NewModule() *Module {
	internalApp := app.NewApp()
	registryClient := client.NewRegistryClient(internalApp)
	grpcService := grpcsvc.NewRegistryService(registryClient, base.Logger)

	return &Module{
		internalApp: internalApp,
		Client:      registryClient,
		GRPCService: grpcService,
	}
}
