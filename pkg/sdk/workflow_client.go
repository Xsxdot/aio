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
	CurrentState  string // 当前状态 JSON
	ActiveNodeIDs string // 活动节点 ID 列表（JSON 字符串）
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

// WorkflowDef 工作流定义（SDK 友好版）
type WorkflowDef struct {
	ID      int64
	Code    string
	Version int32
	Name    string
	DAGJSON string
}

// GetDef 查询工作流定义，version=0 表示最新版本
func (c *WorkflowClient) GetDef(ctx context.Context, code string, version int32) (*WorkflowDef, error) {
	resp, err := c.service.GetDef(ctx, &workflowpb.GetDefRequest{
		Code:    code,
		Version: version,
	})
	if err != nil {
		return nil, WrapError(err, "get workflow def failed")
	}
	if resp.NotFound {
		return nil, nil
	}
	return &WorkflowDef{
		ID:      resp.DefId,
		Code:    resp.Code,
		Version: resp.Version,
		Name:    resp.Name,
		DAGJSON: resp.DagJson,
	}, nil
}

// ListDefs 分页列出工作流定义
func (c *WorkflowClient) ListDefs(ctx context.Context, codeLike string, pageNum, pageSize int32) ([]*WorkflowDef, int64, error) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	resp, err := c.service.ListDefs(ctx, &workflowpb.ListDefsRequest{
		CodeLike: codeLike,
		PageNum:  pageNum,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, WrapError(err, "list workflow defs failed")
	}
	items := make([]*WorkflowDef, len(resp.Items))
	for i, d := range resp.Items {
		items[i] = &WorkflowDef{
			ID:      d.DefId,
			Code:    d.Code,
			Version: d.Version,
			Name:    d.Name,
			DAGJSON: d.DagJson,
		}
	}
	return items, resp.Total, nil
}

// CreateIfNotExists 幂等创建工作流定义，已存在则返回已有 def_id
func (c *WorkflowClient) CreateIfNotExists(ctx context.Context, code, name, dagJSON string, version int32) (defID int64, created bool, err error) {
	if version <= 0 {
		version = 1
	}
	resp, err := c.service.CreateIfNotExists(ctx, &workflowpb.CreateIfNotExistsRequest{
		Code:    code,
		Name:    name,
		DagJson: dagJSON,
		Version: version,
	})
	if err != nil {
		return 0, false, WrapError(err, "create workflow def if not exists failed")
	}
	return resp.DefId, resp.Created, nil
}

// WorkflowInstance 工作流实例（SDK 友好版）
type WorkflowInstance struct {
	ID            int64
	DefID         int64
	DefCode       string
	DefVersion    int32
	Env           string
	Status        string
	InitialState  string // 初始状态 JSON
	CurrentState  string // 当前状态 JSON
	ActiveNodeIDs string // 活动节点 ID 列表（JSON 字符串）
	CreatedAt     string
}

// GetInstance 获取工作流实例详情
func (c *WorkflowClient) GetInstance(ctx context.Context, instanceID int64) (*WorkflowInstance, error) {
	resp, err := c.service.GetInstance(ctx, &workflowpb.GetInstanceRequest{
		InstanceId: instanceID,
	})
	if err != nil {
		return nil, WrapError(err, "get workflow instance failed")
	}
	if resp.NotFound {
		return nil, nil
	}
	return &WorkflowInstance{
		ID:            resp.InstanceId,
		DefID:         resp.DefId,
		DefCode:       resp.DefCode,
		DefVersion:    resp.DefVersion,
		Env:           resp.Env,
		Status:        resp.Status,
		InitialState:  resp.InitialState,
		CurrentState:  resp.CurrentState,
		ActiveNodeIDs: resp.ActiveNodeIds,
		CreatedAt:     resp.CreatedAt,
	}, nil
}

// GetInstanceStatus 轻量查询实例状态
func (c *WorkflowClient) GetInstanceStatus(ctx context.Context, instanceID int64) (string, error) {
	resp, err := c.service.GetInstanceStatus(ctx, &workflowpb.GetInstanceStatusRequest{
		InstanceId: instanceID,
	})
	if err != nil {
		return "", WrapError(err, "get workflow instance status failed")
	}
	if resp.NotFound {
		return "", nil
	}
	return resp.Status, nil
}

// ListInstancesFilter 实例列表过滤条件
type ListInstancesFilter struct {
	DefCode       string
	Status        string
	CreatedAfter  int64 // Unix 毫秒
	CreatedBefore int64 // Unix 毫秒
}

// ListInstances 分页列出工作流实例
func (c *WorkflowClient) ListInstances(ctx context.Context, filter *ListInstancesFilter, pageNum, pageSize int32) ([]*WorkflowInstance, int64, error) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	req := &workflowpb.ListInstancesRequest{
		PageNum:  pageNum,
		PageSize: pageSize,
	}
	if filter != nil {
		req.DefCode = filter.DefCode
		req.Status = filter.Status
		req.CreatedAfter = filter.CreatedAfter
		req.CreatedBefore = filter.CreatedBefore
	}
	resp, err := c.service.ListInstances(ctx, req)
	if err != nil {
		return nil, 0, WrapError(err, "list workflow instances failed")
	}
	items := make([]*WorkflowInstance, len(resp.Items))
	for i, inst := range resp.Items {
		items[i] = &WorkflowInstance{
			ID:            inst.InstanceId,
			DefID:         inst.DefId,
			DefCode:       inst.DefCode,
			DefVersion:    inst.DefVersion,
			Env:           inst.Env,
			Status:        inst.Status,
			InitialState:  inst.InitialState,
			CurrentState:  inst.CurrentState,
			ActiveNodeIDs: inst.ActiveNodeIds,
			CreatedAt:     inst.CreatedAt,
		}
	}
	return items, resp.Total, nil
}

// CancelInstance 取消工作流实例
func (c *WorkflowClient) CancelInstance(ctx context.Context, instanceID int64) error {
	resp, err := c.service.CancelInstance(ctx, &workflowpb.CancelInstanceRequest{
		InstanceId: instanceID,
	})
	if err != nil {
		return WrapError(err, "cancel workflow instance failed")
	}
	if !resp.Success {
		return WrapError(status.Error(codes.FailedPrecondition, resp.Message), "cancel rejected")
	}
	return nil
}

// RetryNode 重试失败节点
func (c *WorkflowClient) RetryNode(ctx context.Context, instanceID int64, nodeID string) error {
	resp, err := c.service.RetryNode(ctx, &workflowpb.RetryNodeRequest{
		InstanceId: instanceID,
		NodeId:     nodeID,
	})
	if err != nil {
		return WrapError(err, "retry node failed")
	}
	if !resp.Success {
		return WrapError(status.Error(codes.FailedPrecondition, resp.Message), "retry rejected")
	}
	return nil
}
