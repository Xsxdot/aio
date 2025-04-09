package replication

import (
	"context"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"sync"
)

// ElectionAdapter Election适配器
type ElectionAdapter struct {
	election election.Election
	logger   *common.Logger
	callback func(bool)
	ctx      context.Context
	cancel   context.CancelFunc
	isMaster bool
	mutex    sync.RWMutex
}

// NewElectionAdapter 创建新的Election适配器
func NewElectionAdapter(electionClient election.Election) RoleClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &ElectionAdapter{
		election: electionClient,
		logger:   common.GetLogger(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// IsMaster 判断当前节点是否为主节点
func (ea *ElectionAdapter) IsMaster() bool {
	ea.mutex.RLock()
	defer ea.mutex.RUnlock()
	return ea.isMaster
}

// WatchRoleChange 监听角色变更
func (ea *ElectionAdapter) WatchRoleChange(callback func(bool)) {
	ea.mutex.Lock()
	ea.callback = callback
	ea.mutex.Unlock()

	// 立即检查一次状态
	ea.setIsMaster(ea.election.IsLeader())
}

// setIsMaster 设置主节点状态
func (ea *ElectionAdapter) setIsMaster(isMaster bool) {
	ea.mutex.Lock()
	oldStatus := ea.isMaster
	changed := oldStatus != isMaster
	ea.isMaster = isMaster
	callback := ea.callback
	ea.mutex.Unlock()

	// 如果状态改变且有回调，则通知
	if changed && callback != nil {
		ea.logger.Infof("节点角色变更: 当前节点是否为主节点: %v", isMaster)
		callback(isMaster)
	}
}

// Start 启动选举客户端
func (ea *ElectionAdapter) Start() error {
	// 创建事件处理函数
	handler := func(event election.ElectionEvent) {
		ea.logger.Infof("接收到选举事件: %v, 领导者: %s", event.Type, event.Leader)

		switch event.Type {
		case election.EventBecomeLeader:
			ea.setIsMaster(true)
			ea.logger.Info("当前节点成为主节点")
		case election.EventBecomeFollower:
			ea.setIsMaster(false)
			ea.logger.Info("当前节点成为从节点")
		case election.EventLeaderChanged:
			// 领导者变更时，检查当前状态
			ea.logger.Info("领导者变更，检查当前节点状态")
			currentLeader, err := ea.election.GetLeader(ea.ctx)
			if err == nil {
				// 判断当前节点是否为主节点
				// 如果返回的主节点为空且当前节点为选举的主节点，或者当前节点ID与主节点ID相同
				isLeader := (currentLeader == "" && ea.election.IsLeader()) ||
					(currentLeader != "" && ea.election.IsLeader())
				ea.setIsMaster(isLeader)
				ea.logger.Infof("领导者变更检查结果: 当前节点是否为主节点: %v, 当前领导者: %s", isLeader, currentLeader)
			} else {
				ea.logger.Errorf("获取领导者信息失败: %v", err)
			}
		}
	}

	// 参与选举，使用新的Campaign方法
	err := ea.election.Campaign(ea.ctx, handler)
	if err != nil {
		return err
	}

	// 立即检查一次状态
	isLeader := ea.election.IsLeader()
	ea.setIsMaster(isLeader)
	ea.logger.Infof("初始化完成，当前节点是否为主节点: %v", isLeader)

	return nil
}

// Stop 停止选举客户端
func (ea *ElectionAdapter) Stop() error {
	// 如果是主节点，辞职
	if ea.IsMaster() {
		if err := ea.election.Resign(ea.ctx); err != nil {
			ea.logger.Errorf("辞职失败: %v", err)
		}
	}

	// 取消上下文
	ea.cancel()

	return nil
}
