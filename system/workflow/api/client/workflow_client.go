package client

import (
	"context"

	"github.com/xsxdot/aio/system/workflow/api/dto"
	"github.com/xsxdot/aio/system/workflow/internal/app"
)

// WorkflowClient 工作流客户端（供其他组件调用）
type WorkflowClient struct {
	app *app.App
}

// NewWorkflowClient 创建客户端实例6
func NewWorkflowClient(a *app.App) *WorkflowClient {
	return &WorkflowClient{app: a}
}

// CreateDef 创建工作流定义
func (c *WorkflowClient) CreateDef(ctx context.Context, env, code, name, dagJSON string, version int32) (int64, error) {
	return c.app.CreateDef(ctx, env, code, name, dagJSON, version)
}

// StartWorkflow 启动工作流，env 用于 Executor 任务隔离
func (c *WorkflowClient) StartWorkflow(ctx context.Context, defCode string, initialData map[string]interface{}, env string) (int64, error) {
	return c.app.StartWorkflow(ctx, defCode, initialData, env)
}

// ReportNodeCompleted 报告节点执行完成，env 用于后续节点 Executor 任务隔离
func (c *WorkflowClient) ReportNodeCompleted(ctx context.Context, instanceID int64, nodeID string, output map[string]interface{}, env string) error {
	return c.app.ReportNodeCompleted(ctx, instanceID, nodeID, output, env)
}

// RollbackToNode 退回到指定节点重新执行，env 用于 Executor 任务隔离
func (c *WorkflowClient) RollbackToNode(ctx context.Context, instanceID int64, targetNodeID string, env string) error {
	return c.app.RollbackToNode(ctx, instanceID, targetNodeID, env)
}

// GetExecutionTrail 获取执行轨迹
func (c *WorkflowClient) GetExecutionTrail(ctx context.Context, instanceID int64) (*dto.ExecutionTrail, error) {
	trail, err := c.app.GetExecutionTrail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return toDTO(trail), nil
}

// GetDef 查询定义，version=0 表示最新版本
func (c *WorkflowClient) GetDef(ctx context.Context, env, code string, version int32) (*app.WorkflowDefModel, error) {
	return c.app.GetDefByCodeAndVersion(ctx, env, code, version)
}

// ListDefs 分页列出定义
func (c *WorkflowClient) ListDefs(ctx context.Context, env, codeLike string, pageNum, pageSize int32) ([]*app.WorkflowDefModel, int64, error) {
	return c.app.ListDefs(ctx, env, codeLike, pageNum, pageSize)
}

// CreateIfNotExists 幂等创建定义
func (c *WorkflowClient) CreateIfNotExists(ctx context.Context, env, code, name, dagJSON string, version int32) (defID int64, created bool, err error) {
	return c.app.CreateIfNotExists(ctx, env, code, name, dagJSON, version)
}

// GetInstance 获取实例详情（含 def_code）
func (c *WorkflowClient) GetInstance(ctx context.Context, instanceID int64) (*app.WorkflowInstanceModel, string, error) {
	inst, err := c.app.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, "", err
	}
	if inst == nil {
		return nil, "", nil
	}
	def, err := c.app.GetDefByID(ctx, inst.DefID)
	if err != nil {
		return inst, "", nil
	}
	defCode := ""
	if def != nil {
		defCode = def.Code
	}
	return inst, defCode, nil
}

// GetInstanceStatus 获取实例状态
func (c *WorkflowClient) GetInstanceStatus(ctx context.Context, instanceID int64) (string, error) {
	inst, err := c.app.GetInstance(ctx, instanceID)
	if err != nil {
		return "", err
	}
	if inst == nil {
		return "", nil
	}
	return string(inst.Status), nil
}

// ListInstances 分页列出实例
func (c *WorkflowClient) ListInstances(ctx context.Context, filter *app.ListInstancesFilter, pageNum, pageSize int32) ([]*app.WorkflowInstanceModel, int64, error) {
	return c.app.ListInstances(ctx, filter, pageNum, pageSize)
}

// CancelInstance 取消实例
func (c *WorkflowClient) CancelInstance(ctx context.Context, instanceID int64) error {
	return c.app.CancelInstance(ctx, instanceID)
}

// RetryNode 重试节点
func (c *WorkflowClient) RetryNode(ctx context.Context, instanceID int64, nodeID string) error {
	return c.app.RetryNode(ctx, instanceID, nodeID)
}

// SendSignal 发送信号（Human-in-the-loop），合并 Payload 入状态，可选唤醒指定节点
func (c *WorkflowClient) SendSignal(ctx context.Context, instanceID int64, signalName string, payload map[string]interface{}, wakeupNode string, env string) error {
	return c.app.SendSignal(ctx, instanceID, signalName, payload, wakeupNode, env)
}

func toDTO(t *app.ExecutionTrail) *dto.ExecutionTrail {
	if t == nil {
		return nil
	}
	checkpoints := make([]dto.ExecutionTrailCheckpoint, len(t.Checkpoints))
	for i, cp := range t.Checkpoints {
		checkpoints[i] = dto.ExecutionTrailCheckpoint{
			NodeID:     cp.NodeID,
			NodeOutput: cp.NodeOutput,
			StateAfter: cp.StateAfter,
			CreatedAt:  cp.CreatedAt,
		}
	}
	return &dto.ExecutionTrail{
		InstanceID:    t.InstanceID,
		Status:        t.Status,
		CurrentState:  t.CurrentState,
		ActiveNodeIDs: t.ActiveNodeIDs,
		Checkpoints:   checkpoints,
	}
}
