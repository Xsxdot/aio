package application

import (
	"xiaozhizhang/system/application/api/client"
	grpcsvc "xiaozhizhang/system/application/external/grpc"
	"xiaozhizhang/system/application/internal/app"
	"xiaozhizhang/system/nginx"
	"xiaozhizhang/system/registry"
	"xiaozhizhang/system/ssl"
	"xiaozhizhang/system/systemd"
)

// Module Application 组件模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用 Application 能力
	Client *client.ApplicationClient
	// GRPCService gRPC 服务实例，供 gRPC 服务器注册使用
	GRPCService *grpcsvc.ApplicationService
}

// NewModule 创建 Application 模块实例
// 需要传入依赖的其他组件 Module，以便在部署编排时调用
func NewModule(
	sslModule *ssl.Module,
	nginxModule *nginx.Module,
	systemdModule *systemd.Module,
	registryModule *registry.Module,
) *Module {
	internalApp := app.NewApp(sslModule, nginxModule, systemdModule, registryModule)
	applicationClient := client.NewApplicationClient(internalApp)
	grpcService := grpcsvc.NewApplicationService(internalApp)

	return &Module{
		internalApp: internalApp,
		Client:      applicationClient,
		GRPCService: grpcService,
	}
}

