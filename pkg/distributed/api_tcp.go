package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/network"
	"time"

	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/protocol"
)

// 定义TCP API相关的服务和消息类型
const (
	// 服务类型
	ServiceTypeSystem    = protocol.ServiceTypeSystem
	ServiceTypeElection  = protocol.ServiceTypeElection
	ServiceTypeDiscovery = protocol.ServiceTypeDiscovery

	// 系统消息类型
	MsgTypeHeartbeat = protocol.MsgTypeHeartbeat

	// 选举服务消息类型
	MsgTypeGetLeader    protocol.MessageType = 1
	MsgTypeLeaderNotify protocol.MessageType = 2

	// 服务发现消息类型
	MsgTypeDiscoverService   protocol.MessageType = 1
	MsgTypeWatchService      protocol.MessageType = 3
	MsgTypeServiceEvent      protocol.MessageType = 4
	MsgTypeUnwatchService    protocol.MessageType = 5
	MsgTypeRegisterService   protocol.MessageType = 6 // 新增：注册服务
	MsgTypeDeregisterService protocol.MessageType = 7 // 新增：注销服务
)

// LeaderInfo 主节点信息
type LeaderInfo struct {
	// NodeID 节点ID
	NodeID string `json:"nodeId"`
	// IP 节点IP地址
	IP string `json:"ip"`
	// ProtocolPort 协议端口号
	ProtocolPort int `json:"protocolPort"`
	// CachePort 缓存端口号
	CachePort int `json:"cachePort"`
	// LastUpdate 最后更新时间
	LastUpdate time.Time `json:"lastUpdate"`
}

// 请求结构体
type GetLeaderRequest struct {
	ElectionName string `json:"electionName"`
}

type ElectionTcpApi struct {
	// 选举服务
	electionService election.ElectionService

	// 协议管理器
	protocolMgr *protocol.ProtocolManager
}

func NewElectionTcpApi(electionService election.ElectionService, protocolMgr *protocol.ProtocolManager) *ElectionTcpApi {
	e := &ElectionTcpApi{
		electionService: electionService,
		protocolMgr:     protocolMgr,
	}

	protocolMgr.RegisterHandle(ServiceTypeElection, MsgTypeGetLeader, e.GetLeader)

	return e
}

func (e *ElectionTcpApi) GetLeader(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 获取选举实例
	elect := e.electionService.GetDefaultElection()

	// 获取选举信息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info := elect.GetInfo()
	leader, err := elect.GetLeader(ctx)
	if err != nil {
		return nil, err
	}

	// 构造响应
	leaderInfo := LeaderInfo{
		NodeID:       leader,
		IP:           info.IP,
		ProtocolPort: info.ProtocolPort,
		CachePort:    info.CachePort,
		LastUpdate:   time.Now(),
	}

	var handleIdP *string
	handlerId := elect.AddEventHandler(func(event election.ElectionEvent) {
		if leader == event.Leader {
			// 构造响应
			leaderInfo := LeaderInfo{
				NodeID:       leader,
				IP:           info.IP,
				ProtocolPort: info.ProtocolPort,
				CachePort:    info.CachePort,
				LastUpdate:   time.Now(),
			}
			err := e.protocolMgr.SendMessage(connID, MsgTypeLeaderNotify, ServiceTypeElection, leaderInfo)
			if err != nil {
				if network.IsUnavailable(err) {
					elect.RemoveEventHandler(*handleIdP)
				}
			}
		}
	})
	handleIdP = &handlerId

	// 使用统一的响应方法
	return &leaderInfo, nil
}

type DiscoverServiceRequest struct {
	ServiceName string `json:"serviceName"`
}

type WatchServiceRequest struct {
	ServiceName string `json:"serviceName"`
}

type UnwatchServiceRequest struct {
	ServiceName string `json:"serviceName"`
	WatcherID   string `json:"watcherId"`
}

// 新增：注册服务请求
type RegisterServiceRequest struct {
	// Service 要注册的服务信息
	Service discovery.ServiceInfo `json:"service"`
}

// 新增：注销服务请求
type DeregisterServiceRequest struct {
	// ServiceID 要注销的服务ID
	ServiceID string `json:"serviceId"`
}

// DiscoveryTcpApi 服务发现TCP API处理结构
type DiscoveryTcpApi struct {
	discoveryService discovery.DiscoveryService
	protocolMgr      *protocol.ProtocolManager
	watcherMap       map[string]map[string]string // connID -> serviceName -> watcherID
}

// NewDiscoveryTcpApi 创建服务发现TCP API处理器
func NewDiscoveryTcpApi(discoveryService discovery.DiscoveryService, protocolMgr *protocol.ProtocolManager) *DiscoveryTcpApi {
	d := &DiscoveryTcpApi{
		discoveryService: discoveryService,
		protocolMgr:      protocolMgr,
		watcherMap:       make(map[string]map[string]string),
	}

	// 注册处理函数
	protocolMgr.RegisterHandle(ServiceTypeDiscovery, MsgTypeDiscoverService, d.DiscoverService)
	protocolMgr.RegisterHandle(ServiceTypeDiscovery, MsgTypeWatchService, d.WatchService)
	protocolMgr.RegisterHandle(ServiceTypeDiscovery, MsgTypeUnwatchService, d.UnwatchService)
	protocolMgr.RegisterHandle(ServiceTypeDiscovery, MsgTypeRegisterService, d.RegisterService)
	protocolMgr.RegisterHandle(ServiceTypeDiscovery, MsgTypeDeregisterService, d.DeregisterService)
	protocolMgr.RegisterHandle(ServiceTypeSystem, MsgTypeHeartbeat, d.Heartbeat)

	return d
}

// DiscoverService 发现服务处理方法
func (d *DiscoveryTcpApi) DiscoverService(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 解析请求
	var req DiscoverServiceRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		return nil, err
	}

	// 获取服务列表
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services, err := d.discoveryService.Discover(ctx, req.ServiceName)
	if err != nil {
		return nil, err
	}

	return services, nil
}

// WatchService 监听服务处理方法
func (d *DiscoveryTcpApi) WatchService(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 解析请求
	var req WatchServiceRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		return nil, err
	}

	// 添加服务变更监听器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var watcher *string

	// 服务变更事件处理函数
	eventHandler := func(event discovery.DiscoveryEvent) {
		// 发送事件消息
		eventMsg := protocol.NewMessage(
			MsgTypeServiceEvent,
			ServiceTypeDiscovery,
			connID,
			event,
		)

		conn, found := d.protocolMgr.GetConnection(connID)
		if !found {
			// 连接已断开，可以移除监听器
			d.discoveryService.RemoveWatcher(req.ServiceName, *watcher)
			return
		}

		if err := conn.Send(eventMsg); err != nil {
			fmt.Printf("Failed to send event message: %v\n", err)
		}
	}

	watcherID, err := d.discoveryService.AddWatcher(ctx, req.ServiceName, eventHandler)
	if err != nil {
		return nil, err
	}
	watcher = &watcherID

	// 保存watcherID到连接映射
	if _, ok := d.watcherMap[connID]; !ok {
		d.watcherMap[connID] = make(map[string]string)
	}
	d.watcherMap[connID][req.ServiceName] = watcherID

	return watcherID, nil
}

// UnwatchService 取消服务监听处理方法
func (d *DiscoveryTcpApi) UnwatchService(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 解析请求
	var req UnwatchServiceRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		return nil, err
	}

	// 获取监听器ID
	var watcherID string
	if req.WatcherID != "" {
		watcherID = req.WatcherID
	} else {
		// 从连接映射中获取监听器ID
		if connWatchers, ok := d.watcherMap[connID]; ok {
			if id, ok := connWatchers[req.ServiceName]; ok {
				watcherID = id
			}
		}
	}

	if watcherID == "" {
		return nil, fmt.Errorf("watcher not found")
	}

	// 移除监听器
	err := d.discoveryService.RemoveWatcher(req.ServiceName, watcherID)

	// 无论成功与否，都从映射中移除
	if connWatchers, ok := d.watcherMap[connID]; ok {
		delete(connWatchers, req.ServiceName)
		if len(connWatchers) == 0 {
			delete(d.watcherMap, connID)
		}
	}

	if err != nil {
		return nil, err
	}

	return protocol.OK, nil
}

// RegisterService 注册服务处理方法
func (d *DiscoveryTcpApi) RegisterService(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 解析请求
	var req RegisterServiceRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		return nil, err
	}

	// 验证服务信息
	if req.Service.ID == "" || req.Service.Name == "" || req.Service.Address == "" {
		return nil, fmt.Errorf("invalid service info: missing required fields")
	}

	// 注册服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.discoveryService.Register(ctx, req.Service)
	if err != nil {
		return nil, err
	}

	return req.Service.ID, nil
}

// DeregisterService 注销服务处理方法
func (d *DiscoveryTcpApi) DeregisterService(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 解析请求
	var req DeregisterServiceRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		return nil, err
	}

	// 验证服务ID
	if req.ServiceID == "" {
		return nil, fmt.Errorf("invalid service ID: empty")
	}

	// 注销服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.discoveryService.Deregister(ctx, req.ServiceID)
	if err != nil {
		return nil, err
	}

	return protocol.OK, nil
}

// Heartbeat 处理心跳消息，检查并清理已关闭连接的监听器
func (d *DiscoveryTcpApi) Heartbeat(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	// 当收到心跳响应时检查连接是否已关闭，如果关闭则清理监听器
	_, found := d.protocolMgr.GetConnection(connID)
	if !found {
		// 连接已关闭，清理监听器
		if watchersByConn, ok := d.watcherMap[connID]; ok {
			// 遍历该连接的所有监听器
			for serviceName, watcherID := range watchersByConn {
				// 取消服务监听
				err := d.discoveryService.RemoveWatcher(serviceName, watcherID)
				if err != nil {
					// 记录错误，但继续清理其他监听器
					fmt.Printf("Error removing watcher: %v\n", err)
				}
			}
			// 删除该连接的监听器映射
			delete(d.watcherMap, connID)
		}
	}
	return nil, nil
}

// RegisterDiscoveryTCPHandlers 原有函数保留，但实现改为使用DiscoveryTcpApi
func RegisterDiscoveryTCPHandlers(discoveryService discovery.DiscoveryService, protocolMgr *protocol.ProtocolManager) error {
	_ = NewDiscoveryTcpApi(discoveryService, protocolMgr)
	return nil
}

// RegisterElectionTCPHandlers 注册选举TCP处理器，使用ElectionTcpApi结构
func RegisterElectionTCPHandlers(electionService election.ElectionService, protocolMgr *protocol.ProtocolManager) error {
	_ = NewElectionTcpApi(electionService, protocolMgr)
	return nil
}

// 生成唯一的消息ID
func generateMessageID() string {
	// 这里应该使用分布式ID生成器
	// 简单实现：使用时间戳+随机数
	return fmt.Sprintf("%d%06d", time.Now().UnixNano()/1000, time.Now().Nanosecond()%1000000)
}
