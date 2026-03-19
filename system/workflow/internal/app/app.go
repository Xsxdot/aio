package app

import (
	"context"
	"encoding/json"
	"fmt"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	executorClient "github.com/xsxdot/aio/system/executor/api/client"
	"github.com/xsxdot/aio/system/workflow/internal/dao"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"github.com/xsxdot/aio/system/workflow/internal/service"
)

// 类型别名，供 api/client 等层使用
type WorkflowDefModel = model.WorkflowDefModel
type WorkflowInstanceModel = model.WorkflowInstanceModel
type ListInstancesFilter = dao.ListInstancesFilter

// App 工作流内部应用层编排
type App struct {
	DefService        *service.WorkflowDefService
	InstanceService   *service.WorkflowInstanceService
	CheckpointService *service.WorkflowCheckpointService
	ExecutorClient    *executorClient.ExecutorClient
	log               *logger.Log
	err               *errorc.ErrorBuilder
}

// NewApp 创建内部 App
func NewApp(
	defSvc *service.WorkflowDefService,
	instSvc *service.WorkflowInstanceService,
	cpSvc *service.WorkflowCheckpointService,
	execClient *executorClient.ExecutorClient,
) *App {
	return &App{
		DefService:        defSvc,
		InstanceService:   instSvc,
		CheckpointService: cpSvc,
		ExecutorClient:    execClient,
		log:               logger.GetLogger().WithEntryName("WorkflowApp"),
		err:               errorc.NewErrorBuilder("WorkflowApp"),
	}
}

// CreateDef 创建工作流定义
func (a *App) CreateDef(ctx context.Context, code, name, dagJSON string, version int32) (int64, error) {
	if version <= 0 {
		version = 1
	}
	var dag model.DAG
	if err := json.Unmarshal([]byte(dagJSON), &dag); err != nil {
		return 0, a.err.New("解析DAG失败", err)
	}
	if err := dag.Validate(); err != nil {
		return 0, a.err.New("DAG 验证失败: "+err.Error(), err)
	}
	def := &model.WorkflowDefModel{
		Code:    code,
		Version: version,
		Name:    name,
		DAGJSON: dagJSON,
	}
	if err := a.DefService.Create(ctx, def); err != nil {
		return 0, err
	}
	return def.ID, nil
}

// GetInstance 获取工作流实例
func (a *App) GetInstance(ctx context.Context, instanceID int64) (*model.WorkflowInstanceModel, error) {
	return a.InstanceService.FindById(ctx, instanceID)
}

// GetDefByCode 根据 code 获取工作流定义（最新版本）
func (a *App) GetDefByCode(ctx context.Context, code string) (*model.WorkflowDefModel, error) {
	return a.DefService.FindByCode(ctx, code)
}

// GetDefByID 根据 ID 获取工作流定义
func (a *App) GetDefByID(ctx context.Context, id int64) (*model.WorkflowDefModel, error) {
	return a.DefService.FindById(ctx, id)
}

// GetDefByCodeAndVersion 根据 code 和 version 查询定义，version=0 表示最新版本
func (a *App) GetDefByCodeAndVersion(ctx context.Context, code string, version int32) (*model.WorkflowDefModel, error) {
	if version <= 0 {
		return a.DefService.FindByCode(ctx, code)
	}
	return a.DefService.FindByCodeAndVersion(ctx, code, version)
}

// ListDefs 分页列出定义
func (a *App) ListDefs(ctx context.Context, codeLike string, pageNum, pageSize int32) ([]*model.WorkflowDefModel, int64, error) {
	return a.DefService.ListDefs(ctx, codeLike, pageNum, pageSize)
}

// ListInstances 分页列出实例
func (a *App) ListInstances(ctx context.Context, filter *dao.ListInstancesFilter, pageNum, pageSize int32) ([]*model.WorkflowInstanceModel, int64, error) {
	return a.InstanceService.ListInstances(ctx, filter, pageNum, pageSize)
}

// CreateIfNotExists 幂等创建定义，存在则返回已有 def_id
func (a *App) CreateIfNotExists(ctx context.Context, code, name, dagJSON string, version int32) (defID int64, created bool, err error) {
	if version <= 0 {
		version = 1
	}
	existing, findErr := a.DefService.FindByCodeAndVersion(ctx, code, version)
	if findErr == nil && existing != nil {
		return existing.ID, false, nil
	}
	if findErr != nil && !errorc.IsNotFound(findErr) {
		return 0, false, findErr
	}
	id, err := a.CreateDef(ctx, code, name, dagJSON, version)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// OnJobCompleted 实现 executor 任务完成处理器，在 AckJob 成功且 Source=workflow 时触发
func (a *App) OnJobCompleted(ctx context.Context, jobID uint64, callbackData, resultJSON string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(callbackData), &data); err != nil {
		a.log.WithErr(err).Errorf("解析任务 %d callback_data 失败", jobID)
		return
	}

	instanceID, nodeID, callbackEnv, subJobID, err := parseWorkflowCallbackData(data)
	if err != nil {
		a.log.WithErr(err).Errorf("任务 %d callback_data 格式无效，跳过回调", jobID)
		return
	}

	var output map[string]interface{}
	if resultJSON != "" {
		if parseErr := json.Unmarshal([]byte(resultJSON), &output); parseErr != nil {
			a.log.WithErr(parseErr).Warnf("任务 %d result_json 解析失败，将使用空 output", jobID)
		}
	}
	if output == nil {
		output = make(map[string]interface{})
	}

	if err := a.ReportNodeCompleted(ctx, instanceID, nodeID, output, callbackEnv, subJobID); err != nil {
		a.log.WithErr(err).Errorf("任务 %d 回调 ReportNodeCompleted 失败", jobID)
	}
}

// parseWorkflowCallbackData 从 callback_data 中解析 instance_id、node_id、env、sub_job_id（Map 子任务时有值）
func parseWorkflowCallbackData(data map[string]interface{}) (instanceID int64, nodeID string, env string, subJobID int, err error) {
	instVal, ok := data["instance_id"]
	if !ok || instVal == nil {
		return 0, "", "", -1, fmt.Errorf("缺少 instance_id")
	}
	switch v := instVal.(type) {
	case float64:
		instanceID = int64(v)
	case int:
		instanceID = int64(v)
	case int64:
		instanceID = v
	default:
		return 0, "", "", -1, fmt.Errorf("instance_id 类型无效: %T", instVal)
	}

	nodeVal, ok := data["node_id"]
	if !ok || nodeVal == nil {
		return 0, "", "", -1, fmt.Errorf("缺少 node_id")
	}
	nodeID, ok = nodeVal.(string)
	if !ok {
		return 0, "", "", -1, fmt.Errorf("node_id 类型无效: %T", nodeVal)
	}
	if envVal, ok := data["env"]; ok && envVal != nil {
		if s, ok := envVal.(string); ok {
			env = s
		}
	}
	subJobID = -1
	if sjVal, ok := data["sub_job_id"]; ok && sjVal != nil {
		switch v := sjVal.(type) {
		case float64:
			subJobID = int(v)
		case int:
			subJobID = v
		case int64:
			subJobID = int(v)
		}
	}
	return instanceID, nodeID, env, subJobID, nil
}
