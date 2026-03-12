package sdk

import (
	"context"
	"encoding/json"

	workflowpb "github.com/xsxdot/aio/system/workflow/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkflowClient 工作流客户端
type WorkflowClient struct {
	service workflowpb.WorkflowServiceClient
}

// newWorkflowClient 创建工作流客户端
func newWorkflowClient(conn *grpc.ClientConn) *WorkflowClient {
	return &WorkflowClient{
		service: workflowpb.NewWorkflowServiceClient(conn),
	}
}

// CreateDef 创建工作流定义
func (c *WorkflowClient) CreateDef(ctx context.Context, code, name, dagJSON string, version int32) (int64, error) {
	if version <= 0 {
		version = 1
	}

	resp, err := c.service.CreateDef(ctx, &workflowpb.CreateDefRequest{
		Code:    code,
		Name:    name,
		DagJson: dagJSON,
		Version: version,
	})
	if err != nil {
		return 0, WrapError(err, "create workflow def failed")
	}
	return resp.DefId, nil
}

// StartWorkflow 启动工作流
func (c *WorkflowClient) StartWorkflow(ctx context.Context, defCode string, initialData map[string]interface{}) (int64, error) {
	initialDataJSON := "{}"
	if initialData != nil {
		data, err := json.Marshal(initialData)
		if err != nil {
			return 0, WrapError(err, "marshal initial data failed")
		}
		initialDataJSON = string(data)
	}

	resp, err := c.service.StartWorkflow(ctx, &workflowpb.StartWorkflowRequest{
		DefCode:         defCode,
		InitialDataJson: initialDataJSON,
	})
	if err != nil {
		return 0, WrapError(err, "start workflow failed")
	}
	return resp.InstanceId, nil
}

// StartWorkflowWithJSON 启动工作流（传入已序列化的初始数据 JSON）
func (c *WorkflowClient) StartWorkflowWithJSON(ctx context.Context, defCode, initialDataJSON string) (int64, error) {
	if initialDataJSON == "" {
		initialDataJSON = "{}"
	}

	resp, err := c.service.StartWorkflow(ctx, &workflowpb.StartWorkflowRequest{
		DefCode:         defCode,
		InitialDataJson: initialDataJSON,
	})
	if err != nil {
		return 0, WrapError(err, "start workflow failed")
	}
	return resp.InstanceId, nil
}

// ReportNodeCompleted 报告节点完成
func (c *WorkflowClient) ReportNodeCompleted(ctx context.Context, instanceID int64, nodeID string, output map[string]interface{}) error {
	outputJSON := "{}"
	if output != nil {
		data, err := json.Marshal(output)
		if err != nil {
			return WrapError(err, "marshal output failed")
		}
		outputJSON = string(data)
	}

	resp, err := c.service.ReportNodeCompleted(ctx, &workflowpb.ReportNodeCompletedRequest{
		InstanceId: instanceID,
		NodeId:     nodeID,
		OutputJson: outputJSON,
		Env:        "",
	})
	if err != nil {
		return WrapError(err, "report node completed failed")
	}
	if !resp.Success {
		return WrapError(status.Error(codes.FailedPrecondition, resp.Message), "report node completed rejected")
	}
	return nil
}

// ReportNodeCompletedWithJSON 报告节点完成（传入已序列化的 output JSON）
func (c *WorkflowClient) ReportNodeCompletedWithJSON(ctx context.Context, instanceID int64, nodeID, outputJSON string) error {
	if outputJSON == "" {
		outputJSON = "{}"
	}

	resp, err := c.service.ReportNodeCompleted(ctx, &workflowpb.ReportNodeCompletedRequest{
		InstanceId: instanceID,
		NodeId:     nodeID,
		OutputJson: outputJSON,
		Env:        "",
	})
	if err != nil {
		return WrapError(err, "report node completed failed")
	}
	if !resp.Success {
		return WrapError(status.Error(codes.FailedPrecondition, resp.Message), "report node completed rejected")
	}
	return nil
}

// RollbackToNode 回滚到指定节点重新执行
func (c *WorkflowClient) RollbackToNode(ctx context.Context, instanceID int64, targetNodeID string) error {
	resp, err := c.service.RollbackToNode(ctx, &workflowpb.RollbackToNodeRequest{
		InstanceId:   instanceID,
		TargetNodeId: targetNodeID,
		Env:          "",
	})
	if err != nil {
		return WrapError(err, "rollback to node failed")
	}
	if !resp.Success {
		return WrapError(status.Error(codes.FailedPrecondition, resp.Message), "rollback rejected")
	}
	return nil
}

// ExecutionTrail 执行轨迹（SDK 友好版）
type ExecutionTrail struct {
	InstanceID    int64
	Status        string
	CurrentState  string                 // 当前状态 JSON
	ActiveNodeIDs string                 // 活动节点 ID 列表（JSON 字符串）
	Checkpoints   []ExecutionTrailCheckpoint
}

// ExecutionTrailCheckpoint 轨迹中的单步快照
type ExecutionTrailCheckpoint struct {
	NodeID     string
	NodeOutput map[string]interface{} // 节点输出
	StateAfter map[string]interface{} // 节点执行后状态
	CreatedAt  string
}

// GetExecutionTrail 获取执行轨迹
func (c *WorkflowClient) GetExecutionTrail(ctx context.Context, instanceID int64) (*ExecutionTrail, error) {
	resp, err := c.service.GetExecutionTrail(ctx, &workflowpb.GetExecutionTrailRequest{
		InstanceId: instanceID,
	})
	if err != nil {
		return nil, WrapError(err, "get execution trail failed")
	}

	checkpoints := make([]ExecutionTrailCheckpoint, len(resp.Checkpoints))
	for i, cp := range resp.Checkpoints {
		var nodeOutput, stateAfter map[string]interface{}
		if cp.NodeOutputJson != "" {
			_ = json.Unmarshal([]byte(cp.NodeOutputJson), &nodeOutput)
		}
		if cp.StateAfterJson != "" {
			_ = json.Unmarshal([]byte(cp.StateAfterJson), &stateAfter)
		}
		if nodeOutput == nil {
			nodeOutput = make(map[string]interface{})
		}
		if stateAfter == nil {
			stateAfter = make(map[string]interface{})
		}
		checkpoints[i] = ExecutionTrailCheckpoint{
			NodeID:     cp.NodeId,
			NodeOutput: nodeOutput,
			StateAfter: stateAfter,
			CreatedAt:  cp.CreatedAt,
		}
	}

	return &ExecutionTrail{
		InstanceID:    resp.InstanceId,
		Status:        resp.Status,
		CurrentState:  resp.CurrentState,
		ActiveNodeIDs: resp.ActiveNodeIds,
		Checkpoints:   checkpoints,
	}, nil
}
