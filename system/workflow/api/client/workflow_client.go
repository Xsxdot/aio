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
func (c *WorkflowClient) CreateDef(ctx context.Context, code, name, dagJSON string, version int32) (int64, error) {
	return c.app.CreateDef(ctx, code, name, dagJSON, version)
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
