package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	executorDto "github.com/xsxdot/aio/system/executor/api/dto"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// workflowStateData 和 workflowStateSys 用于新结构的 data、_sys 段
type workflowStateData map[string]interface{}
type workflowStateSys map[string]interface{}

// parseWorkflowState 解析 CurrentState：新结构 {"data":{...},"_sys":{...}} 或旧结构（整段为 data）
func parseWorkflowState(raw string) (workflowStateData, workflowStateSys, error) {
	var top map[string]interface{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &top); err != nil {
			return nil, nil, err
		}
	}
	if top == nil {
		top = make(map[string]interface{})
	}
	dataVal, hasData := top["data"]
	sysVal, hasSys := top["_sys"]
	if hasData {
		if m, ok := dataVal.(map[string]interface{}); ok {
			sys := make(workflowStateSys)
			if hasSys && sysVal != nil {
				if sm, ok := sysVal.(map[string]interface{}); ok {
					sys = sm
				}
			}
			return workflowStateData(m), sys, nil
		}
	}
	return workflowStateData(top), make(workflowStateSys), nil
}

// serializeWorkflowState 序列化 state 为 JSON
func serializeWorkflowState(data workflowStateData, sys workflowStateSys) (string, error) {
	if data == nil {
		data = make(workflowStateData)
	}
	if sys == nil {
		sys = make(workflowStateSys)
	}
	top := map[string]interface{}{"data": data, "_sys": sys}
	b, err := json.Marshal(top)
	return string(b), err
}

// stateUpdateMode 状态更新模式
const (
	stateUpdateOverwrite = "overwrite"
	stateUpdateAppend    = "append"
	stateUpdateDeepMerge = "deep_merge"
)

// applyStateReducer 根据节点配置将 output 合并进 data（overwrite/append/deep_merge）
func applyStateReducer(data workflowStateData, output map[string]interface{}, nodeConfig map[string]interface{}) {
	if data == nil || output == nil {
		return
	}
	mode := stateUpdateOverwrite
	if m, ok := nodeConfig["state_update_mode"].(string); ok && m != "" {
		mode = m
	}
	outputKey, hasOutputKey := nodeConfig["output_key"].(string)

	switch mode {
	case stateUpdateAppend:
		if hasOutputKey {
			existing := data[outputKey]
			var arr []interface{}
			if s, ok := existing.([]interface{}); ok {
				arr = s
			}
			data[outputKey] = append(arr, output)
		} else {
			for k, v := range output {
				existing := data[k]
				if s, ok := existing.([]interface{}); ok {
					data[k] = append(s, v)
				} else {
					// 修复：强制包装为数组，保证下一次能继续 append
					data[k] = []interface{}{v}
				}
			}
		}
	case stateUpdateDeepMerge:
		for k, v := range output {
			if existing, ok := data[k]; ok && existing != nil && v != nil {
				if em, ok := existing.(map[string]interface{}); ok {
					if vm, ok := v.(map[string]interface{}); ok {
						data[k] = deepMergeMaps(em, vm)
						continue
					}
				}
			}
			data[k] = v
		}
	default:
		for k, v := range output {
			data[k] = v
		}
	}
}

func deepMergeMaps(base, override map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		if v == nil {
			out[k] = v
			continue
		}
		if existing, ok := out[k]; ok && existing != nil {
			if em, ok := existing.(map[string]interface{}); ok {
				if vm, ok := v.(map[string]interface{}); ok {
					out[k] = deepMergeMaps(em, vm)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

// getValueAtPath 按简单点分隔路径从 data 中取值，支持数组
func getValueAtPath(data map[string]interface{}, path string) interface{} {
	if data == nil || path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	cur := interface{}(data)
	for _, key := range parts {
		if cur == nil {
			return nil
		}
		if m, ok := cur.(map[string]interface{}); ok {
			cur = m[key]
		} else {
			return nil
		}
	}
	return cur
}

// setValueAtPath 按简单点分隔路径设置 data 中的值，如 "state.key" 或 "key"
func setValueAtPath(data map[string]interface{}, path string, value interface{}) {
	if data == nil || path == "" {
		return
	}
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		data[path] = value
		return
	}
	cur := map[string]interface{}(data)
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		nextVal, ok := cur[key]
		if !ok || nextVal == nil {
			next := make(map[string]interface{})
			cur[key] = next
			cur = next
		} else if m, ok := nextVal.(map[string]interface{}); ok {
			cur = m
		} else {
			next := make(map[string]interface{})
			cur[key] = next
			cur = next
		}
	}
	cur[parts[len(parts)-1]] = value
}

// handleMapSubTaskCompleted 处理 Map 节点的子任务回调，聚合结果，计数器归零时写 output_path 并触发下游
func (a *App) handleMapSubTaskCompleted(tx *gorm.DB, instance *model.WorkflowInstanceModel, nodeID string, subID int, output map[string]interface{}, data workflowStateData, sys workflowStateSys, dag *model.DAG, nextNodeIDs *[]string) (bool, error) {
	_, hasErrorMsg := output["error_msg"]
	if hasErrorMsg {
		return false, nil
	}
	mapCounters, _ := sys["map_counters"].(map[string]interface{})
	mapResults, _ := sys["map_results"].(map[string]interface{})
	if mapCounters == nil || mapResults == nil {
		return true, nil // 修复：说明整个 Map 节点已被之前的 Error 熔断抛弃，安全吞掉后续的孤儿回调
	}
	countVal, exists := mapCounters[nodeID]
	if !exists {
		return true, nil // 修复：同上，安全吞掉
	}
	var count int
	switch v := countVal.(type) {
	case float64:
		count = int(v)
	case int:
		count = v
	case int64:
		count = int(v)
	default:
		return true, nil // 修复：类型异常也应抛弃，不能漏下去
	}
	resultsRaw := mapResults[nodeID]
	resultsArr, ok := resultsRaw.([]interface{})
	if !ok || resultsArr == nil || subID < 0 || subID >= len(resultsArr) {
		return true, nil // 修复：无效结果也应吞掉，防止幽灵触发
	}
	resultsArr[subID] = output
	count--
	mapCounters[nodeID] = count
	newStateStr, _ := serializeWorkflowState(data, sys)
	instance.CurrentState = newStateStr
	if count > 0 {
		return true, tx.Save(instance).Error
	}
	node := dag.GetNode(nodeID)
	if node == nil {
		return true, nil
	}
	outputPath, _ := node.Config["output_path"].(string)
	if outputPath != "" {
		setValueAtPath(data, outputPath, resultsArr)
	}
	delete(mapCounters, nodeID)
	delete(mapResults, nodeID)
	newStateStr, _ = serializeWorkflowState(data, sys)
	instance.CurrentState = newStateStr
	var activeNodes []string
	if instance.ActiveNodeIDs != "" {
		_ = json.Unmarshal([]byte(instance.ActiveNodeIDs), &activeNodes)
	}
	newActiveNodes := make([]string, 0)
	for _, n := range activeNodes {
		if n != nodeID {
			newActiveNodes = append(newActiveNodes, n)
		}
	}
	outputBytes, _ := json.Marshal(map[string]interface{}{"merged": resultsArr})
	if err := tx.Create(&model.WorkflowCheckpointModel{
		InstanceID: instance.ID,
		NodeID:     nodeID,
		NodeOutput: string(outputBytes),
		StateAfter: newStateStr,
	}).Error; err != nil {
		return true, err
	}
	outEdges := dag.GetOutgoingEdges(nodeID)
	for _, edge := range outEdges {
		if edge.Type == model.EdgeTypeError {
			continue
		}
		pass, err := a.evaluateCondition(edge.Condition, data)
		if err != nil {
			a.log.WithErr(err).Errorf("评估 map 下游边 %s->%s 条件失败", edge.From, edge.To)
			continue
		}
		if pass {
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
				*nextNodeIDs = append(*nextNodeIDs, edge.To)
				newActiveNodes = append(newActiveNodes, edge.To)
			}
		}
	}
	activeNodesBytes, _ := json.Marshal(newActiveNodes)
	instance.ActiveNodeIDs = string(activeNodesBytes)
	if len(newActiveNodes) == 0 {
		instance.Status = model.InstanceStatusCompleted
	}
	return true, tx.Save(instance).Error
}

// StartWorkflow 启动一个新的工作流实例，任一起始节点触发失败则将实例标记为 FAILED
// env 用于 Executor 任务隔离，空则用 base.ENV
func (a *App) StartWorkflow(ctx context.Context, defCode string, initialData map[string]interface{}, env string) (int64, error) {
	if env == "" {
		env = base.ENV
	}
	def, err := a.DefService.FindByCode(ctx, env, defCode)
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

	stateStr, err := serializeWorkflowState(workflowStateData(initialData), make(workflowStateSys))
	if err != nil {
		return 0, a.err.New("序列化初始状态失败", err)
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
		InitialState:  stateStr,
		CurrentState:  stateStr,
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
// subJobID 为 Map 子任务时传入（>=0），否则传 -1 或不传
func (a *App) ReportNodeCompleted(ctx context.Context, instanceID int64, nodeID string, output map[string]interface{}, env string, subJobID ...int) error {
	subID := -1
	if len(subJobID) > 0 {
		subID = subJobID[0]
	}
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

		// 2. 解析 CurrentState（data/_sys）、获取 DAG（需在边选择前加载）
		data, sys, err := parseWorkflowState(instance.CurrentState)
		if err != nil {
			return fmt.Errorf("解析 CurrentState 失败: %w", err)
		}
		if data == nil {
			data = make(workflowStateData)
		}
		if sys == nil {
			sys = make(workflowStateSys)
		}

		var def model.WorkflowDefModel
		if err := tx.Where("id = ?", instance.DefID).First(&def).Error; err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(def.DAGJSON), &dag); err != nil {
			return fmt.Errorf("解析 DAG 失败: %w", err)
		}

		// 3. Map 子任务回调：聚合结果，计数器归零时写 output_path 并触发下游
		if subID >= 0 {
			handled, err := a.handleMapSubTaskCompleted(tx, &instance, nodeID, subID, output, data, sys, &dag, &nextNodeIDs)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
		}

		// 4. 检测是否为 Executor 最终失败回调（带 error_msg），走 error 边或置实例 FAILED
		_, hasErrorMsg := output["error_msg"]
		if hasErrorMsg {
			// 修复：如果是 Map 节点的子任务报错，立即清除 Latch 计数，防止幽灵并发
			if subID >= 0 {
				if counters, ok := sys["map_counters"].(map[string]interface{}); ok {
					delete(counters, nodeID)
				}
				if results, ok := sys["map_results"].(map[string]interface{}); ok {
					delete(results, nodeID)
				}
			}
			outEdges := dag.GetOutgoingEdges(nodeID)
			var errorEdges []model.Edge
			for _, e := range outEdges {
				if e.Type == model.EdgeTypeError || e.Type == model.EdgeTypeAlways {
					errorEdges = append(errorEdges, e)
				}
			}
			if len(errorEdges) == 0 {
				instance.Status = model.InstanceStatusFailed
				activeNodesBytes, _ := json.Marshal([]string{})
				instance.ActiveNodeIDs = string(activeNodesBytes)
				outputBytes, _ := json.Marshal(output)
				_ = tx.Create(&model.WorkflowCheckpointModel{
					InstanceID: instance.ID,
					NodeID:     nodeID,
					NodeOutput: string(outputBytes),
					StateAfter: instance.CurrentState,
				}).Error
				if err := tx.Save(&instance).Error; err != nil {
					return err
				}
				return nil
			}
			for k, v := range output {
				data[k] = v
			}
			newStateStr, _ := serializeWorkflowState(data, sys)
			instance.CurrentState = newStateStr

			var activeNodes []string
			if instance.ActiveNodeIDs != "" {
				_ = json.Unmarshal([]byte(instance.ActiveNodeIDs), &activeNodes)
			}
			newActiveNodes := make([]string, 0)
			for _, n := range activeNodes {
				if n != nodeID {
					newActiveNodes = append(newActiveNodes, n)
				}
			}

			for _, edge := range errorEdges {
				pass, err := a.evaluateCondition(edge.Condition, data)
				if err != nil {
					a.log.WithErr(err).Errorf("评估 error 边 %s->%s 条件失败", edge.From, edge.To)
					continue
				}
				if pass {
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

			outputBytes, _ := json.Marshal(output)
			if err := tx.Create(&model.WorkflowCheckpointModel{
				InstanceID: instance.ID,
				NodeID:     nodeID,
				NodeOutput: string(outputBytes),
				StateAfter: newStateStr,
			}).Error; err != nil {
				return err
			}

			activeNodesBytes, _ := json.Marshal(newActiveNodes)
			instance.ActiveNodeIDs = string(activeNodesBytes)
			if len(newActiveNodes) == 0 {
				// 修复：因为是 error 触发的流转，如果无路可走，说明降级失败，应彻底宕机
				instance.Status = model.InstanceStatusFailed
			}
			if err := tx.Save(&instance).Error; err != nil {
				return err
			}
			return nil
		}

		// 4. 正常成功路径：使用 state_update_mode 合并状态，排除 error 边
		node := dag.GetNode(nodeID)
		var nodeConfig map[string]interface{}
		if node != nil {
			nodeConfig = node.Config
		}
		applyStateReducer(data, output, nodeConfig)
		newStateStr, err := serializeWorkflowState(data, sys)
		if err != nil {
			return fmt.Errorf("序列化状态失败: %w", err)
		}
		instance.CurrentState = newStateStr

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

		outputBytes, _ := json.Marshal(output)
		if err := tx.Create(&model.WorkflowCheckpointModel{
			InstanceID: instance.ID,
			NodeID:     nodeID,
			NodeOutput: string(outputBytes),
			StateAfter: newStateStr,
		}).Error; err != nil {
			return err
		}

		outEdges := dag.GetOutgoingEdges(nodeID)
		for _, edge := range outEdges {
			if edge.Type == model.EdgeTypeError {
				continue
			}
			pass, err := a.evaluateCondition(edge.Condition, data)
			if err != nil {
				a.log.WithErr(err).Errorf("评估边 %s->%s 条件失败", edge.From, edge.To)
				continue
			}
			if pass {
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

				// 修复：防假死机制。如果无法触发下个节点，将实例挂起为 FAILED
				instance.Status = model.InstanceStatusFailed
				if _, updErr := a.InstanceService.UpdateById(ctx, instance.ID, &instance); updErr != nil {
					a.log.WithErr(updErr).Errorf("尝试将假死实例置为 FAILED 失败: %d", instance.ID)
				}
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
			CallbackData:     callbackData,
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

	case model.NodeTypeMap:
		return a.triggerMapNode(ctx, instance, node, dag, env)
	}
	return nil
}

// triggerMapNode 触发 Map 节点：根据 items_path 获取数组，派发 N 个子任务，初始化 Latch
func (a *App) triggerMapNode(ctx context.Context, instance *model.WorkflowInstanceModel, node *model.Node, dag *model.DAG, env string) error {
	if env == "" {
		env = base.ENV
	}
	itemsPath, _ := node.Config["items_path"].(string)
	if itemsPath == "" {
		return a.err.New("Map 节点缺少 items_path 配置", nil)
	}
	data, sys, err := parseWorkflowState(instance.CurrentState)
	if err != nil {
		return a.err.New("解析状态失败", err)
	}
	if data == nil {
		data = make(workflowStateData)
	}
	if sys == nil {
		sys = make(workflowStateSys)
	}
	itemsRaw := getValueAtPath(data, itemsPath)
	itemsArr, ok := itemsRaw.([]interface{})
	if !ok || len(itemsArr) == 0 {
		return a.err.New("Map 节点 items_path 对应值非数组或为空", nil)
	}
	N := len(itemsArr)
	iteratorConf, _ := node.Config["iterator"].(map[string]interface{})
	var serviceName, methodName string
	if iteratorConf != nil {
		if s, ok := iteratorConf["service"].(string); ok {
			serviceName = s
		}
		if m, ok := iteratorConf["method"].(string); ok {
			methodName = m
		}
	}
	if serviceName == "" || methodName == "" {
		return a.err.New("Map 节点 iterator 缺少 service 或 method", nil)
	}
	itemAlias, _ := node.Config["item_alias"].(string)
	if itemAlias == "" {
		itemAlias = "item"
	}
	if mapCounters, ok := sys["map_counters"].(map[string]interface{}); ok {
		if mapCounters == nil {
			mapCounters = make(map[string]interface{})
			sys["map_counters"] = mapCounters
		}
		mapCounters[node.ID] = N
	} else {
		sys["map_counters"] = map[string]interface{}{node.ID: N}
	}
	results := make([]interface{}, N)
	if mapResults, ok := sys["map_results"].(map[string]interface{}); ok {
		if mapResults == nil {
			mapResults = make(map[string]interface{})
			sys["map_results"] = mapResults
		}
		mapResults[node.ID] = results
	} else {
		sys["map_results"] = map[string]interface{}{node.ID: results}
	}
	newStateStr, err := serializeWorkflowState(data, sys)
	if err != nil {
		return a.err.New("序列化状态失败", err)
	}
	instance.CurrentState = newStateStr
	if _, err := a.InstanceService.UpdateById(ctx, instance.ID, instance); err != nil {
		return a.err.New("更新实例状态失败", err)
	}
	for i := 0; i < N; i++ {
		item := itemsArr[i]
		subPayload := map[string]interface{}{
			"instance_id": instance.ID,
			"node_id":     node.ID,
			"state":       instance.CurrentState,
			"config":      node.Config,
			itemAlias:     item,
		}
		payloadBytes, _ := json.Marshal(subPayload)
		callbackDataBytes, _ := json.Marshal(map[string]interface{}{
			"instance_id": instance.ID,
			"node_id":     node.ID,
			"env":         env,
			"sub_job_id":  i,
		})
		callbackData := string(callbackDataBytes)
		dedupKey := fmt.Sprintf("wf_%d_node_%s_sub_%d", instance.ID, node.ID, i)
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
			CallbackData:     callbackData,
		})
		if err != nil {
			return a.err.New("提交 Map 子任务到 Executor 失败", err)
		}
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

		// 修复：从最新的检查点开始往回找，保留循环节点多次执行后的最新数据
		for i := len(checkpoints) - 1; i >= 0; i-- {
			cp := checkpoints[i]
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
	InstanceID    int64                      `json:"instance_id"`
	Status        string                     `json:"status"`
	CurrentState  string                     `json:"current_state"`
	ActiveNodeIDs string                     `json:"active_node_ids"`
	Checkpoints   []ExecutionTrailCheckpoint `json:"checkpoints"`
}

type ExecutionTrailCheckpoint struct {
	NodeID     string                 `json:"node_id"`
	NodeOutput map[string]interface{} `json:"node_output"`
	StateAfter map[string]interface{} `json:"state_after"`
	CreatedAt  string                 `json:"created_at"`
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
			CreatedAt:  cp.CreatedAt.Format(time.RFC3339),
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

// CancelInstance 取消运行中的实例（仅 RUNNING/WAITING 可取消）
func (a *App) CancelInstance(ctx context.Context, instanceID int64) error {
	instance, err := a.InstanceService.FindById(ctx, instanceID)
	if err != nil {
		return a.err.New("获取实例失败", err)
	}
	if instance.Status != model.InstanceStatusRunning && instance.Status != model.InstanceStatusWaiting {
		return a.err.New("只有运行中或等待中的实例才能取消", nil).WithCode(errorc.ErrorCodeValid)
	}
	instance.Status = model.InstanceStatusCanceled
	if _, err := a.InstanceService.UpdateById(ctx, instance.ID, instance); err != nil {
		return a.err.New("更新实例状态失败", err)
	}
	env := instance.Env
	if env == "" {
		env = base.ENV
	}
	var activeNodes []string
	if instance.ActiveNodeIDs != "" {
		_ = json.Unmarshal([]byte(instance.ActiveNodeIDs), &activeNodes)
	}
	for _, nodeID := range activeNodes {
		dedupKey := fmt.Sprintf("wf_%d_node_%s", instanceID, nodeID)
		_ = a.ExecutorClient.CancelJobByDedupKey(ctx, env, dedupKey)
	}
	return nil
}

// RetryNode 重试失败节点（仅 FAILED 实例可重试）
func (a *App) RetryNode(ctx context.Context, instanceID int64, nodeID string) error {
	var instance *model.WorkflowInstanceModel
	var dag model.DAG
	var stateToRestore string
	var deleteFromIndex int = -1

	err := base.DB.Transaction(func(tx *gorm.DB) error {
		inst, err := a.InstanceService.FindByIdForUpdate(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		instance = inst
		if instance.Status != model.InstanceStatusFailed {
			return fmt.Errorf("只有失败状态的实例才能重试节点: %s", instance.Status)
		}
		def, err := a.DefService.FindByIdWithTx(ctx, tx, instance.DefID)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(def.DAGJSON), &dag); err != nil {
			return fmt.Errorf("解析DAG失败: %w", err)
		}
		if dag.GetNode(nodeID) == nil {
			return fmt.Errorf("目标节点不存在: %s", nodeID)
		}
		checkpoints, err := a.CheckpointService.ListByInstanceIDOrderByCreatedAscWithTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		// 修复：RetryNode 也必须逆序查找最新检查点，与 RollbackToNode 一致
		for i := len(checkpoints) - 1; i >= 0; i-- {
			cp := checkpoints[i]
			if cp.NodeID == nodeID {
				deleteFromIndex = i
				if i == 0 {
					stateToRestore = instance.InitialState
				} else {
					stateToRestore = checkpoints[i-1].StateAfter
				}
				break
			}
		}
		startNodes := dag.GetStartNodes()
		for _, sn := range startNodes {
			if sn.ID == nodeID {
				stateToRestore = instance.InitialState
				deleteFromIndex = 0
				break
			}
		}
		if deleteFromIndex < 0 {
			return fmt.Errorf("目标节点尚未执行过，无法重试")
		}
		if deleteFromIndex < len(checkpoints) {
			if err := a.CheckpointService.DeleteFromIndexWithTx(ctx, tx, instanceID, deleteFromIndex); err != nil {
				return err
			}
		}
		instance.CurrentState = stateToRestore
		instance.ActiveNodeIDs = fmt.Sprintf(`["%s"]`, nodeID)
		instance.Status = model.InstanceStatusRunning
		return a.InstanceService.SaveWithTx(ctx, tx, instance)
	})
	if err != nil {
		return a.err.New("重试节点失败", err)
	}
	env := instance.Env
	if env == "" {
		env = base.ENV
	}
	node := dag.GetNode(nodeID)
	if node != nil {
		return a.triggerNode(ctx, instance, node, &dag, env)
	}
	return nil
}

// SendSignal 接收外部信号，合并 Payload 入 state，可选唤醒指定节点继续执行
func (a *App) SendSignal(ctx context.Context, instanceID int64, signalName string, payload map[string]interface{}, wakeupNode string, env string) error {
	var instance model.WorkflowInstanceModel
	var dag model.DAG

	err := base.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", instanceID).First(&instance).Error; err != nil {
			return err
		}
		if instance.Status != model.InstanceStatusCompleted && instance.Status != model.InstanceStatusWaiting {
			return fmt.Errorf("只有 COMPLETED 或 WAITING 状态的实例才能接收信号: %s", instance.Status)
		}
		if env == "" && instance.Env != "" {
			env = instance.Env
		}
		if env == "" {
			env = base.ENV
		}

		data, sys, err := parseWorkflowState(instance.CurrentState)
		if err != nil {
			return fmt.Errorf("解析状态失败: %w", err)
		}
		if data == nil {
			data = make(workflowStateData)
		}
		if sys == nil {
			sys = make(workflowStateSys)
		}

		for k, v := range payload {
			data[k] = v
		}
		newStateStr, err := serializeWorkflowState(data, sys)
		if err != nil {
			return err
		}
		instance.CurrentState = newStateStr
		if instance.Status == model.InstanceStatusCompleted {
			instance.Status = model.InstanceStatusWaiting
		}

		signalOutput := map[string]interface{}{
			"_signal":     true,
			"signal_name": signalName,
			"payload":     payload,
		}
		outputBytes, _ := json.Marshal(signalOutput)
		if err := tx.Create(&model.WorkflowCheckpointModel{
			InstanceID: instance.ID,
			NodeID:     "_signal_" + signalName,
			NodeOutput: string(outputBytes),
			StateAfter: newStateStr,
		}).Error; err != nil {
			return err
		}

		if wakeupNode != "" {
			var def model.WorkflowDefModel
			if err := tx.Where("id = ?", instance.DefID).First(&def).Error; err != nil {
				return err
			}
			if err := json.Unmarshal([]byte(def.DAGJSON), &dag); err != nil {
				return fmt.Errorf("解析 DAG 失败: %w", err)
			}
			node := dag.GetNode(wakeupNode)
			if node == nil {
				return fmt.Errorf("唤醒节点不存在: %s", wakeupNode)
			}
			instance.ActiveNodeIDs = fmt.Sprintf(`["%s"]`, wakeupNode)
			// 优化：节点已触发并投递给 Executor，语义上应为 RUNNING
			instance.Status = model.InstanceStatusRunning
		}
		return tx.Save(&instance).Error
	})
	if err != nil {
		return a.err.New("发送信号失败", err)
	}

	if wakeupNode != "" {
		node := dag.GetNode(wakeupNode)
		if node != nil {
			envToUse := env
			if envToUse == "" && instance.Env != "" {
				envToUse = instance.Env
			}
			if envToUse == "" {
				envToUse = base.ENV
			}
			return a.triggerNode(ctx, &instance, node, &dag, envToUse)
		}
	}
	return nil
}
