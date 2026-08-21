package executor

import (
	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/api/client"
	grpcsvc "github.com/xsxdot/aio/system/executor/external/grpc"
	"github.com/xsxdot/aio/system/executor/internal/app"
	"github.com/xsxdot/aio/system/executor/internal/worker"
)

// Module 任务执行器模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	internalApp    *app.App
	Client         *client.ExecutorClient
	GRPCService    *grpcsvc.ExecutorService
	internalWorker *worker.InternalCallbackWorker
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

// RegisterJobCompletionHandler 注册任务完成处理器（按 Source 路由，AckJob 成功且 job.Source 非空时触发）
func (m *Module) RegisterJobCompletionHandler(source string, h callback.JobCompletionHandler) {
	m.internalApp.JobService.RegisterJobCompletionHandler(source, h)
}

// StartInternalWorker 启动进程内 outbox 回调消费者。
//
// 参数：
//   - env: 消费的环境标识，传 base.ENV
//   - consumerID: 实例标识，多实例下必须互不相同（优先用 registry 自注册的
//     instanceKey，未启用自注册时用 hostname-pid）
func (m *Module) StartInternalWorker(env, consumerID string) {
	m.internalWorker = worker.NewInternalCallbackWorker(
		m.internalApp.JobService, env, consumerID,
		base.Logger.WithEntryName("InternalCallbackWorker"),
	)
	m.internalWorker.Start()
}

// StopInternalWorker 停止内部回调消费者并等待在途回调结束。
func (m *Module) StopInternalWorker() {
	if m.internalWorker != nil {
		m.internalWorker.Stop()
	}
}
