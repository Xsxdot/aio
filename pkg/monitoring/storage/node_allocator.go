// Package storage 节点分配管理器
package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/registry"

	"go.uber.org/zap"
)

// NodeAllocation 表示服务的节点分配信息
type NodeAllocation struct {
	ServiceName   string    `json:"service_name"`    // 服务名称
	NodeID        string    `json:"node_id"`         // 分配的节点ID
	NodeAddress   string    `json:"node_address"`    // 节点地址
	AssignTime    time.Time `json:"assign_time"`     // 分配时间
	LastCheckTime time.Time `json:"last_check_time"` // 最后检查时间
}

// NodeAllocator 节点分配管理器
type NodeAllocator struct {
	registry       registry.Registry          // 服务注册中心
	logger         *zap.Logger                // 日志记录器
	allocations    map[string]*NodeAllocation // 服务名到节点分配的映射
	nodeCounts     map[string]int             // 节点ID到服务数量的映射
	mutex          sync.RWMutex               // 读写锁
	aioServiceName string                     // AIO服务名称
}

// NodeAllocatorConfig 节点分配器配置
type NodeAllocatorConfig struct {
	Registry       registry.Registry // 服务注册中心
	Logger         *zap.Logger       // 日志记录器
	AIOServiceName string            // AIO服务名称，默认为"aio"
}

// NewNodeAllocator 创建新的节点分配器
func NewNodeAllocator(config NodeAllocatorConfig) *NodeAllocator {
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	aioServiceName := config.AIOServiceName
	if aioServiceName == "" {
		aioServiceName = "aio-service"
	}

	return &NodeAllocator{
		registry:       config.Registry,
		logger:         logger,
		allocations:    make(map[string]*NodeAllocation),
		nodeCounts:     make(map[string]int),
		aioServiceName: aioServiceName,
	}
}

// GetStorageNode 获取服务的存储节点分配
func (na *NodeAllocator) GetStorageNode(ctx context.Context, serviceName string, forceReassign bool) (*NodeAllocation, error) {
	na.mutex.Lock()
	defer na.mutex.Unlock()

	na.logger.Debug("获取存储节点分配",
		zap.String("service_name", serviceName),
		zap.Bool("force_reassign", forceReassign))

	var needReassign bool
	var aioNodes []*registry.ServiceInstance
	var err error

	// 检查是否已有分配
	if allocation, exists := na.allocations[serviceName]; exists && !forceReassign {
		// 发现所有AIO节点来检查当前分配的节点状态
		aioNodes, err = na.discoverAIONodes(ctx)
		if err != nil {
			na.logger.Error("检查节点状态时发现AIO节点失败", zap.Error(err))
			return nil, fmt.Errorf("检查节点状态失败: %w", err)
		}

		// 检查当前分配的节点是否还存在
		if na.isNodeOnline(allocation.NodeID, aioNodes) {
			// 节点仍然存在，更新最后检查时间并返回现有分配
			allocation.LastCheckTime = time.Now()

			na.logger.Debug("使用现有节点分配",
				zap.String("service_name", serviceName),
				zap.String("node_id", allocation.NodeID),
				zap.String("node_address", allocation.NodeAddress))

			return allocation, nil
		} else {
			// 节点已被注销，需要重新分配
			na.logger.Info("检测到分配的节点已注销，将重新分配",
				zap.String("service_name", serviceName),
				zap.String("offline_node_id", allocation.NodeID))

			// 清理旧分配
			na.removeAllocation(serviceName)
			needReassign = true
		}
	} else {
		needReassign = true
	}

	// 需要分配新节点：1) 没有现有分配 2) 强制重新分配 3) 原分配节点已注销
	if needReassign || forceReassign {
		na.logger.Info("需要分配新节点",
			zap.String("service_name", serviceName),
			zap.Bool("force_reassign", forceReassign),
			zap.Bool("node_offline", needReassign))

		// 如果还没有发现节点，现在发现
		if aioNodes == nil {
			aioNodes, err = na.discoverAIONodes(ctx)
			if err != nil {
				na.logger.Error("发现AIO节点失败", zap.Error(err))
				return nil, fmt.Errorf("发现AIO节点失败: %w", err)
			}
		}

		if len(aioNodes) == 0 {
			return nil, fmt.Errorf("没有可用的AIO节点")
		}

		// 如果是强制重新分配，先清理旧分配
		if forceReassign && na.allocations[serviceName] != nil {
			na.logger.Info("强制重新分配，清理旧分配",
				zap.String("service_name", serviceName),
				zap.String("old_node_id", na.allocations[serviceName].NodeID))
			na.removeAllocation(serviceName)
		}

		// 进行新分配
		selectedNode := na.selectBestNode(aioNodes)
		if selectedNode == nil {
			return nil, fmt.Errorf("无法选择合适的节点：所有AIO节点都离线或不可用")
		}

		// 创建新分配
		allocation := &NodeAllocation{
			ServiceName:   serviceName,
			NodeID:        selectedNode.ID,
			NodeAddress:   selectedNode.Address,
			AssignTime:    time.Now(),
			LastCheckTime: time.Now(),
		}

		// 保存分配
		na.allocations[serviceName] = allocation
		na.nodeCounts[selectedNode.ID]++

		na.logger.Info("分配新存储节点",
			zap.String("service_name", serviceName),
			zap.String("node_id", selectedNode.ID),
			zap.String("node_address", selectedNode.Address))

		return allocation, nil
	}

	// 理论上不会到达这里
	return nil, fmt.Errorf("未知错误：无法确定节点分配状态")
}

// discoverAIONodes 发现所有AIO节点
func (na *NodeAllocator) discoverAIONodes(ctx context.Context) ([]*registry.ServiceInstance, error) {
	instances, err := na.registry.DiscoverAll(ctx, na.aioServiceName)
	if err != nil {
		return nil, fmt.Errorf("发现AIO服务实例失败: %w", err)
	}

	na.logger.Debug("发现AIO节点", zap.Int("total_nodes", len(instances)))

	return instances, nil
}

// isNodeOnline 检查节点是否在线
func (na *NodeAllocator) isNodeOnline(nodeID string, nodes []*registry.ServiceInstance) bool {
	for _, node := range nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

// selectBestNode 选择最佳节点（负载最少的节点）
func (na *NodeAllocator) selectBestNode(nodes []*registry.ServiceInstance) *registry.ServiceInstance {
	if len(nodes) == 0 {
		return nil
	}

	// 创建节点负载信息
	type nodeLoad struct {
		node *registry.ServiceInstance
		load int
	}

	var nodeLoads []nodeLoad
	for _, node := range nodes {
		if !node.IsOnline() {
			continue
		}
		load := na.nodeCounts[node.ID]
		nodeLoads = append(nodeLoads, nodeLoad{
			node: node,
			load: load,
		})
	}

	// 检查是否有可用的在线节点
	if len(nodeLoads) == 0 {
		na.logger.Warn("没有在线的AIO节点可用")
		return nil
	}

	// 按负载排序（负载少的在前）
	sort.Slice(nodeLoads, func(i, j int) bool {
		if nodeLoads[i].load == nodeLoads[j].load {
			// 负载相同时，按节点ID排序确保一致性
			return nodeLoads[i].node.ID < nodeLoads[j].node.ID
		}
		return nodeLoads[i].load < nodeLoads[j].load
	})

	selectedNode := nodeLoads[0].node
	na.logger.Debug("选择最佳节点",
		zap.String("node_id", selectedNode.ID),
		zap.String("node_address", selectedNode.Address),
		zap.Int("current_load", nodeLoads[0].load))

	return selectedNode
}

// removeAllocation 移除分配
func (na *NodeAllocator) removeAllocation(serviceName string) {
	if allocation, exists := na.allocations[serviceName]; exists {
		// 减少节点计数
		if count, nodeExists := na.nodeCounts[allocation.NodeID]; nodeExists && count > 0 {
			na.nodeCounts[allocation.NodeID]--
			if na.nodeCounts[allocation.NodeID] == 0 {
				delete(na.nodeCounts, allocation.NodeID)
			}
		}

		// 删除分配记录
		delete(na.allocations, serviceName)

		na.logger.Debug("移除节点分配",
			zap.String("service_name", serviceName),
			zap.String("node_id", allocation.NodeID))
	}
}

// GetAllAllocations 获取所有分配信息（用于监控和调试）
func (na *NodeAllocator) GetAllAllocations() map[string]*NodeAllocation {
	na.mutex.RLock()
	defer na.mutex.RUnlock()

	result := make(map[string]*NodeAllocation)
	for k, v := range na.allocations {
		// 创建副本避免并发问题
		allocation := *v
		result[k] = &allocation
	}

	return result
}

// GetNodeStats 获取节点统计信息
func (na *NodeAllocator) GetNodeStats() map[string]int {
	na.mutex.RLock()
	defer na.mutex.RUnlock()

	result := make(map[string]int)
	for k, v := range na.nodeCounts {
		result[k] = v
	}

	return result
}

// CleanupOfflineAllocations 清理离线节点的分配
func (na *NodeAllocator) CleanupOfflineAllocations(ctx context.Context) error {
	na.mutex.Lock()
	defer na.mutex.Unlock()

	// 发现当前在线节点
	onlineNodes, err := na.discoverAIONodes(ctx)
	if err != nil {
		return fmt.Errorf("发现在线节点失败: %w", err)
	}

	onlineNodeIDs := make(map[string]bool)
	for _, node := range onlineNodes {
		onlineNodeIDs[node.ID] = true
	}

	// 检查所有分配，移除离线节点的分配
	var toRemove []string
	for serviceName, allocation := range na.allocations {
		if !onlineNodeIDs[allocation.NodeID] {
			toRemove = append(toRemove, serviceName)
			na.logger.Info("检测到离线节点分配",
				zap.String("service_name", serviceName),
				zap.String("offline_node_id", allocation.NodeID))
		}
	}

	// 移除离线节点的分配
	for _, serviceName := range toRemove {
		na.removeAllocation(serviceName)
	}

	if len(toRemove) > 0 {
		na.logger.Info("清理离线节点分配完成",
			zap.Int("cleaned_count", len(toRemove)))
	}

	return nil
}
