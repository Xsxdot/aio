package workflow

import (
	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor"
	executorCallback "github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/workflow/api/client"
	grpcsvc "github.com/xsxdot/aio/system/workflow/external/grpc"
	"github.com/xsxdot/aio/system/workflow/internal/app"
	"github.com/xsxdot/aio/system/workflow/internal/dao"
	"github.com/xsxdot/aio/system/workflow/internal/service"
)

// Module 工作流模块门面
type Module struct {
	internalApp  *app.App
	Client       *client.WorkflowClient
	GRPCService  *grpcsvc.WorkflowService
}

// NewModule 创建工作流模块实例（依赖 ExecutorModule 提供任务执行能力）
func NewModule(executorModule *executor.Module) *Module {
	log := base.Logger.WithEntryName("Workflow")
	db := base.DB

	defDao := dao.NewWorkflowDefDao(db, log)
	instDao := dao.NewWorkflowInstanceDao(db, log)
	cpDao := dao.NewWorkflowCheckpointDao(db, log)

	defSvc := service.NewWorkflowDefService(defDao, log)
	instSvc := service.NewWorkflowInstanceService(instDao, log)
	cpSvc := service.NewWorkflowCheckpointService(cpDao, log)

	internalApp := app.NewApp(defSvc, instSvc, cpSvc, executorModule.Client)
	wfClient := client.NewWorkflowClient(internalApp)
	grpcService := grpcsvc.NewWorkflowService(wfClient, base.Logger)

	return &Module{
		internalApp: internalApp,
		Client:      wfClient,
		GRPCService: grpcService,
	}
}

// GetJobCompletionHandler 返回任务完成处理器实现（供 Executor 按 Source=workflow 注入，AckJob 成功时触发 Workflow.ReportNodeCompleted）
func (m *Module) GetJobCompletionHandler() executorCallback.JobCompletionHandler {
	return m.internalApp
}
