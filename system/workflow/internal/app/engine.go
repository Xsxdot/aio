package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/xsxdot/aio/base"
	executorDto "github.com/xsxdot/aio/system/executor/api/dto"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StartWorkflow 启动一个新的工作流实例，任一起始节点触发失败则将实例标记为 FAILED
// env 用于 Executor 任务隔离，空则用 base.ENV
func (a *App) StartWorkflow(ctx context.Context, defCode string, initialData map[string]interface{}, env string) (int64, error) {
	if env == "" {
		env = base.ENV
	}
	def, err := a.DefService.FindByCode(ctx, defCode)
	if err != nil {
		return 0, err
	}

	var dag model.DAG
	if err := json.Unmarshal([]byte(def.DAGJSON), &dag); err != nil {
		return 0, a.err.New("解析DAG失败", err)
	}

	startNodes := dag.GetStartNodes()
	if len(startNodes) == 0 {
		return 0, a.err.New("工作流未定义起始节点", nil)
	}

	var activeNodeIDs []string
	for _, n := range startNodes {
		activeNodeIDs = append(activeNodeIDs, n.ID)
	}

	initialDataJSON, err := json.Marshal(initialData)
	if err != nil {
		return 0, a.err.New("序列化初始数据失败", err)
	}
	activeNodesJSON, err := json.Marshal(activeNodeIDs)
	if err != nil {
		return 0, a.err.New("序列化活跃节点失败", err)
	}

	instance := &model.WorkflowInstanceModel{
		DefID:         def.ID,
		DefVersion:    def.Version,
		Env:           env,
		Status:        model.InstanceStatusRunning,
		InitialState:  string(initialDataJSON),
		CurrentState:  string(initialDataJSON),
		ActiveNodeIDs: string(activeNodesJSON),
	}

	if err := a.InstanceService.Create(ctx, instance); err != nil {
		return 0, err
	}

	for _, n := range startNodes {
		nodeCopy := n
		if err := a.triggerNode(ctx, instance, &nodeCopy, &dag, env); err != nil {
			a.log.WithErr(err).Errorf("触发起始节点 %s 失败，将实例标记为 FAILED", n.ID)
			instance.Status = model.InstanceStatusFailed
			if _, updErr := a.InstanceService.UpdateById(ctx, instance.ID, instance); updErr != nil {
				a.log.WithErr(updErr).Errorf("更新实例状态为 FAILED 失败")
			}
			return 0, a.err.New("触发起始节点失败: "+err.Error(), err)
		}
	}

	return instance.ID, nil
}

// ReportNodeCompleted 报告节点执行完成，引擎将自动推进状态机
// env 用于触发后续节点时 Executor 任务隔离，空则用实例存储的 Env，再空则用 base.ENV
func (a *App) ReportNodeCompleted(ctx context.Context, instanceID int64, nodeID string, output map[string]interface{}, env string) error {
	var nextNodeIDs []string
	var instance model.WorkflowInstanceModel
	var dag model.DAG

	err := base.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 获取实例并加锁
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", instanceID).First(&instance).Error; err != nil {
			return err
		}
		if env == "" && instance.Env != "" {
			env = instance.Env
		}
		if env == "" {
			env = base.ENV
		}

		if instance.Status != model.InstanceStatusRunning && instance.Status != model.InstanceStatusWaiting {
			return fmt.Errorf("实例状态不允许推进: %s", instance.Status)
		}
		if instance.Status == model.InstanceStatusWaiting {
			instance.Status = model.InstanceStatusRunning
		}

		// 2. 更新 CurrentState
		var currentState map[string]interface{}
		if instance.CurrentState != "" {
			if err := json.Unmarshal([]byte(instance.CurrentState), &currentState); err != nil {
				return fmt.Errorf("解析 CurrentState 失败: %w", err)
			}
		}
		if currentState == nil {
			currentState = make(map[string]interface{})
		}

		for k, v := range output {
			currentState[k] = v
		}
		newStateBytes, _ := json.Marshal(currentState)
		instance.CurrentState = string(newStateBytes)

		// 3. 更新 ActiveNodeIDs
		var activeNodes []string
		if instance.ActiveNodeIDs != "" {
			if err := json.Unmarshal([]byte(instance.ActiveNodeIDs), &activeNodes); err != nil {
				return fmt.Errorf("解析 ActiveNodeIDs 失败: %w", err)
			}
		}
		newActiveNodes := make([]string, 0)
		for _, n := range activeNodes {
			if n != nodeID {
				newActiveNodes = append(newActiveNodes, n)
			}
		}

		// 4. 保存 Checkpoint
		outputBytes, _ := json.Marshal(output)
		checkpoint := &model.WorkflowCheckpointModel{
			InstanceID: instance.ID,
			NodeID:     nodeID,
			NodeOutput: string(outputBytes),
			StateAfter: string(newStateBytes),
		}
		if err := tx.Create(checkpoint).Error; err != nil {
			return err
		}

		// 5. 获取 DAG 解析下一步
		var def model.WorkflowDefModel
		if err := tx.Where("id = ?", instance.DefID).First(&def).Error; err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(def.DAGJSON), &dag); err != nil {
			return fmt.Errorf("解析 DAG 失败: %w", err)
		}

		outEdges := dag.GetOutgoingEdges(nodeID)
		for _, edge := range outEdges {
			pass, err := a.evaluateCondition(edge.Condition, currentState)
			if err != nil {
				a.log.WithErr(err).Errorf("评估边 %s->%s 条件失败", edge.From, edge.To)
				continue
			}
			if pass {
				// 防止重复触发
				var cpCount int64
				tx.Model(&model.WorkflowCheckpointModel{}).
					Where("instance_id = ? AND node_id = ?", instance.ID, edge.To).
					Count(&cpCount)

				inActive := false
				for _, an := range newActiveNodes {
					if an == edge.To {
						inActive = true
						break
					}
				}

				if cpCount == 0 && !inActive {
					nextNodeIDs = append(nextNodeIDs, edge.To)
					newActiveNodes = append(newActiveNodes, edge.To)
				}
			}
		}

		// 6. 保存实例状态
		activeNodesBytes, _ := json.Marshal(newActiveNodes)
		instance.ActiveNodeIDs = string(activeNodesBytes)
		if len(newActiveNodes) == 0 {
			instance.Status = model.InstanceStatusCompleted
		}

		if err := tx.Save(&instance).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return a.err.New("完成节点流转失败", err)
	}

	// TX 提交成功后，触发新节点
	for _, nextNodeID := range nextNodeIDs {
		node := dag.GetNode(nextNodeID)
		if node != nil {
			if err := a.triggerNode(ctx, &instance, node, &dag, env); err != nil {
				a.log.WithErr(err).Errorf("触发后续节点 %s 失败", node.ID)
			}
		}
	}

	return nil
}

// triggerNode 根据节点类型执行具体动作，env 用于 Executor 任务隔离
func (a *App) triggerNode(ctx context.Context, instance *model.WorkflowInstanceModel, node *model.Node, dag *model.DAG, env string) error {
	if env == "" {
		env = base.ENV
	}
	switch node.Type {
	case model.NodeTypeTask:
		var serviceName, methodName string
		if svc, ok := node.Config["service"].(string); ok {
			serviceName = svc
		}
		if method, ok := node.Config["method"].(string); ok {
			methodName = method
		}

		payload := map[string]interface{}{
			"instance_id": instance.ID,
			"node_id":     node.ID,
			"state":       instance.CurrentState,
			"config":      node.Config,
		}
		payloadBytes, _ := json.Marshal(payload)
		callbackDataBytes, _ := json.Marshal(map[string]interface{}{"instance_id": instance.ID, "node_id": node.ID, "env": env})
		callbackData := string(callbackDataBytes)
		dedupKey := fmt.Sprintf("wf_%d_node_%s", instance.ID, node.ID)

		_, err := a.ExecutorClient.SubmitJob(ctx, &executorDto.SubmitJobInput{
			Env:              env,
			TargetService:    serviceName,
			Method:           methodName,
			ArgsJSON:         string(payloadBytes),
			RunAt:            0,
			MaxAttempts:      3,
			Priority:         0,
			DedupKey:         dedupKey,
			RetryBackoffType: executorDto.RetryBackoffExponential,
			Source:           "workflow",
			CallbackData:    callbackData,
		})
		if err != nil {
			return a.err.New("提交任务到Executor失败", err)
		}

	case model.NodeTypeApproval:
		// 等待外部接口调用 ReportNodeCompleted 推进，使用事务+行锁与 ReportNodeCompleted 并发安全
		return a.updateInstanceStatusToWaitingWithLock(ctx, instance.ID)

	case model.NodeTypeCondition:
		// 路由网关节点，不执行实际操作，立即计算后续分支
		return a.ReportNodeCompleted(ctx, instance.ID, node.ID, nil, env)
	}
	return nil
}

// exprStateWhitelist 表达式可访问的 state 字段前缀黑名单，以这些开头的 key 会被过滤
var exprStateBlacklistPrefixes = []string{"_", "password", "secret", "token", "key"}

// filterStateForExpr 过滤 state，移除敏感字段后传入表达式，降低注入泄露风险
func filterStateForExpr(state map[string]interface{}) map[string]interface{} {
	if state == nil {
		return map[string]interface{}{}
	}
	filtered := make(map[string]interface{})
	for k, v := range state {
		skip := false
		lower := strings.ToLower(k)
		for _, prefix := range exprStateBlacklistPrefixes {
			if strings.HasPrefix(lower, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			filtered[k] = v
		}
	}
	return filtered
}

// updateInstanceStatusToWaitingWithLock 带事务和行锁更新实例状态为 WAITING，与 ReportNodeCompleted 并发安全
func (a *App) updateInstanceStatusToWaitingWithLock(ctx context.Context, instanceID int64) error {
	return base.DB.Transaction(func(tx *gorm.DB) error {
		inst, err := a.InstanceService.FindByIdForUpdate(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if inst.Status != model.InstanceStatusRunning && inst.Status != model.InstanceStatusWaiting {
			return fmt.Errorf("实例状态不允许设为 WAITING: %s", inst.Status)
		}
		inst.Status = model.InstanceStatusWaiting
		return a.InstanceService.SaveWithTx(ctx, tx, inst)
	})
}

// evaluateCondition 评估表达式，仅传入过滤后的 state 降低敏感数据泄露风险
func (a *App) evaluateCondition(condition string, state map[string]interface{}) (bool, error) {
	if condition == "" {
		return true, nil
	}

	safeState := filterStateForExpr(state)
	env := map[string]interface{}{
		"state": safeState,
	}

	program, err := expr.Compile(condition, expr.Env(env))
	if err != nil {
		return false, err
	}
	output, err := expr.Run(program, env)
	if err != nil {
		return false, err
	}
	if b, ok := output.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("条件未返回布尔值")
}

// RollbackToNode 退回到指定节点重新执行（带事务和行锁，与 ReportNodeCompleted 并发安全）
func (a *App) RollbackToNode(ctx context.Context, instanceID int64, targetNodeID string, env string) error {
	var instance *model.WorkflowInstanceModel
	var dag model.DAG
	var stateToRestore string
	var deleteFromIndex int = -1

	err := base.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 获取实例并加行锁
		inst, err := a.InstanceService.FindByIdForUpdate(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		instance = inst
		if instance.Status != model.InstanceStatusRunning && instance.Status != model.InstanceStatusWaiting {
			return fmt.Errorf("只有运行中或等待中的实例才能回滚: %s", instance.Status)
		}

		// 2. 获取 DAG 定义
		def, err := a.DefService.FindByIdWithTx(ctx, tx, instance.DefID)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(def.DAGJSON), &dag); err != nil {
			return fmt.Errorf("解析DAG失败: %w", err)
		}
		if dag.GetNode(targetNodeID) == nil {
			return fmt.Errorf("目标节点不存在: %s", targetNodeID)
		}

		// 3. 获取 checkpoint 列表（事务内一致性读取）
		checkpoints, err := a.CheckpointService.ListByInstanceIDOrderByCreatedAscWithTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}

		for i, cp := range checkpoints {
			if cp.NodeID == targetNodeID {
				deleteFromIndex = i
				if i == 0 {
					stateToRestore = instance.InitialState
				} else {
					stateToRestore = checkpoints[i-1].StateAfter
				}
				break
			}
		}

		if deleteFromIndex < 0 {
			startNodes := dag.GetStartNodes()
			for _, sn := range startNodes {
				if sn.ID == targetNodeID {
					stateToRestore = instance.InitialState
					deleteFromIndex = 0
					break
				}
			}
			if deleteFromIndex < 0 {
				return fmt.Errorf("目标节点尚未执行过，无法回滚")
			}
		}

		// 4. 删除 checkpoint（子查询，不加载全表）
		if deleteFromIndex < len(checkpoints) {
			if err := a.CheckpointService.DeleteFromIndexWithTx(ctx, tx, instanceID, deleteFromIndex); err != nil {
				return err
			}
		}

		// 5. 更新实例状态
		instance.CurrentState = stateToRestore
		instance.ActiveNodeIDs = fmt.Sprintf(`["%s"]`, targetNodeID)
		instance.Status = model.InstanceStatusRunning
		if err := a.InstanceService.SaveWithTx(ctx, tx, instance); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return a.err.New("回滚失败", err)
	}

	// 6. TX 提交后，取消活跃节点的 Executor 任务并触达目标节点
	activeNodes := make([]string, 0)
	if instance.ActiveNodeIDs != "" {
		if parseErr := json.Unmarshal([]byte(instance.ActiveNodeIDs), &activeNodes); parseErr != nil {
			a.log.WithErr(parseErr).Warnf("解析 ActiveNodeIDs 失败，将跳过取消任务")
		}
	}
	envToUse := env
	if envToUse == "" && instance.Env != "" {
		envToUse = instance.Env
	}
	if envToUse == "" {
		envToUse = base.ENV
	}
	for _, nodeID := range activeNodes {
		dedupKey := fmt.Sprintf("wf_%d_node_%s", instanceID, nodeID)
		_ = a.ExecutorClient.CancelJobByDedupKey(ctx, envToUse, dedupKey)
	}
	node := dag.GetNode(targetNodeID)
	if node != nil {
		return a.triggerNode(ctx, instance, node, &dag, envToUse)
	}
	return nil
}

// ExecutionTrail 执行轨迹
type ExecutionTrail struct {
	InstanceID    int64                    `json:"instance_id"`
	Status        string                   `json:"status"`
	CurrentState  string                   `json:"current_state"`
	ActiveNodeIDs string                   `json:"active_node_ids"`
	Checkpoints   []ExecutionTrailCheckpoint `json:"checkpoints"`
}

type ExecutionTrailCheckpoint struct {
	NodeID    string                 `json:"node_id"`
	NodeOutput map[string]interface{} `json:"node_output"`
	StateAfter map[string]interface{} `json:"state_after"`
	CreatedAt string                 `json:"created_at"`
}

// GetExecutionTrail 获取工作流完整执行轨迹（用于前端绘制执行过程）
func (a *App) GetExecutionTrail(ctx context.Context, instanceID int64) (*ExecutionTrail, error) {
	instance, err := a.InstanceService.FindById(ctx, instanceID)
	if err != nil {
		return nil, a.err.New("获取实例失败", err)
	}

	checkpoints, err := a.CheckpointService.ListByInstanceIDOrderByCreatedAsc(ctx, instanceID)
	if err != nil {
		return nil, a.err.New("获取执行快照失败", err)
	}

	trail := make([]ExecutionTrailCheckpoint, 0, len(checkpoints))
	for _, cp := range checkpoints {
		var nodeOutput, stateAfter map[string]interface{}
		if cp.NodeOutput != "" {
			if err := json.Unmarshal([]byte(cp.NodeOutput), &nodeOutput); err != nil {
				a.log.WithErr(err).Warnf("解析 checkpoint node_output 失败，node_id=%s", cp.NodeID)
			}
		}
		if cp.StateAfter != "" {
			if err := json.Unmarshal([]byte(cp.StateAfter), &stateAfter); err != nil {
				a.log.WithErr(err).Warnf("解析 checkpoint state_after 失败，node_id=%s", cp.NodeID)
			}
		}
		if nodeOutput == nil {
			nodeOutput = make(map[string]interface{})
		}
		if stateAfter == nil {
			stateAfter = make(map[string]interface{})
		}
		trail = append(trail, ExecutionTrailCheckpoint{
			NodeID:     cp.NodeID,
			NodeOutput: nodeOutput,
			StateAfter: stateAfter,
			CreatedAt:  cp.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &ExecutionTrail{
		InstanceID:    instance.ID,
		Status:        string(instance.Status),
		CurrentState:  instance.CurrentState,
		ActiveNodeIDs: instance.ActiveNodeIDs,
		Checkpoints:   trail,
	}, nil
}
