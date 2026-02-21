package executor

import (
	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor/api/client"
	grpcsvc "github.com/xsxdot/aio/system/executor/external/grpc"
	"github.com/xsxdot/aio/system/executor/internal/app"
)

// Module 任务执行器模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用任务执行器能力
	Client *client.ExecutorClient
	// GRPCService gRPC服务实例，供gRPC服务器注册使用
	GRPCService *grpcsvc.ExecutorService
}

// NewModule 创建任务执行器模块实例
func NewModule() *Module {
	internalApp := app.NewApp()
	executorClient := client.NewExecutorClient(internalApp)
	grpcService := grpcsvc.NewExecutorService(executorClient, internalApp, base.Logger)

	return &Module{
		internalApp: internalApp,
		Client:      executorClient,
		GRPCService: grpcService,
	}
}
