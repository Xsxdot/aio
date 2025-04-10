package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/protocol"
)

// 定义TCP API相关的服务和消息类型
const (
	// 服务类型
	ServiceTypeElection  = protocol.ServiceTypeElection
	ServiceTypeDiscovery = protocol.ServiceTypeDiscovery

	// 选举服务消息类型
	MsgTypeGetLeader      protocol.MessageType = 1
	MsgTypeLeaderResponse protocol.MessageType = 2

	// 服务发现消息类型
	MsgTypeDiscoverService   protocol.MessageType = 1
	MsgTypeServiceResponse   protocol.MessageType = 2
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

// 使用统一的响应结构体
type ServiceResponse = protocol.APIResponse

// 请求结构体
type GetLeaderRequest struct {
	ElectionName string `json:"electionName"`
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

// 注册选举服务TCP处理器
func RegisterElectionTCPHandlers(electionService election.ElectionService, protocolMgr *protocol.ProtocolManager) error {
	serviceHandler := protocol.NewServiceHandler()

	// 注册获取主节点信息处理器
	serviceHandler.RegisterHandler(MsgTypeGetLeader, func(connID string, msg *protocol.CustomMessage) error {
		// 解析请求
		var req GetLeaderRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 获取选举实例
		elect, found := electionService.Get(req.ElectionName)
		if !found {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("election not found: %s", req.ElectionName))
		}

		// 获取选举信息
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		info := elect.GetInfo()
		leader, err := elect.GetLeader(ctx)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 构造响应
		leaderInfo := LeaderInfo{
			NodeID:       leader,
			IP:           info.IP,
			ProtocolPort: info.ProtocolPort,
			CachePort:    info.CachePort,
			LastUpdate:   time.Now(),
		}

		// 使用统一的响应方法
		return protocol.SendServiceResponse(
			protocolMgr,
			connID,
			msg.Header().MessageID,
			MsgTypeLeaderResponse,
			ServiceTypeElection,
			true,
			"leader",
			"获取主节点信息成功",
			leaderInfo,
			"",
		)
	})

	// 注册服务
	protocolMgr.RegisterService(ServiceTypeElection, "election", serviceHandler)
	return nil
}

// 注册服务发现TCP处理器
func RegisterDiscoveryTCPHandlers(discoveryService discovery.DiscoveryService, protocolMgr *protocol.ProtocolManager) error {
	serviceHandler := protocol.NewServiceHandler()

	// 连接ID到监听器ID的映射
	watcherMap := make(map[string]map[string]string) // connID -> serviceName -> watcherID

	// 注册服务发现处理器
	serviceHandler.RegisterHandler(MsgTypeDiscoverService, func(connID string, msg *protocol.CustomMessage) error {
		// 解析请求
		var req DiscoverServiceRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 获取服务列表
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		services, err := discoveryService.Discover(ctx, req.ServiceName)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 序列化服务列表为JSON字符串
		servicesJSON, err := json.Marshal(services)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("序列化服务列表失败: %v", err))
		}

		// 构造统一响应结构
		response := ServiceResponse{
			Success: true,
			Type:    "discover",
			Data:    string(servicesJSON),
		}

		// 序列化响应
		respPayload, err := json.Marshal(response)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 发送响应消息
		respMsg := protocol.NewMessage(
			MsgTypeServiceResponse,
			ServiceTypeDiscovery,
			connID,
			msg.Header().MessageID,
			respPayload,
		)

		conn, found := protocolMgr.GetConnection(connID)
		if !found {
			return fmt.Errorf("connection not found: %s", connID)
		}

		return conn.Send(respMsg)
	})

	// 注册服务监听处理器
	serviceHandler.RegisterHandler(MsgTypeWatchService, func(connID string, msg *protocol.CustomMessage) error {
		// 解析请求
		var req WatchServiceRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 添加服务变更监听器
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 服务变更事件处理函数
		eventHandler := func(event discovery.DiscoveryEvent) {
			// 将事件序列化为JSON字符串
			eventJSON, err := json.Marshal(event)
			if err != nil {
				fmt.Printf("Failed to marshal event: %v\n", err)
				return
			}

			// 构造事件消息
			eventData := ServiceResponse{
				Success: true,
				Type:    "event",
				Data:    string(eventJSON),
			}

			// 序列化事件内容
			payload, err := json.Marshal(eventData)
			if err != nil {
				fmt.Printf("Failed to marshal event: %v\n", err)
				return
			}

			// 发送事件消息
			eventMsg := protocol.NewMessage(
				MsgTypeServiceEvent,
				ServiceTypeDiscovery,
				connID,
				generateMessageID(),
				payload,
			)

			conn, found := protocolMgr.GetConnection(connID)
			if !found {
				// 连接已断开，可以移除监听器
				return
			}

			if err := conn.Send(eventMsg); err != nil {
				fmt.Printf("Failed to send event message: %v\n", err)
			}
		}

		watcherID, err := discoveryService.AddWatcher(ctx, req.ServiceName, eventHandler)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 保存监听器ID
		if _, ok := watcherMap[connID]; !ok {
			watcherMap[connID] = make(map[string]string)
		}
		watcherMap[connID][req.ServiceName] = watcherID

		// 将响应数据序列化为JSON字符串
		watcherDataJSON, err := json.Marshal(map[string]string{"watcherId": watcherID, "serviceName": req.ServiceName})
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("序列化监听器数据失败: %v", err))
		}

		// 构造统一响应结构
		response := ServiceResponse{
			Success: true,
			Type:    "watch",
			Data:    string(watcherDataJSON),
		}

		// 发送成功响应
		respPayload, _ := json.Marshal(response)
		respMsg := protocol.NewMessage(
			MsgTypeServiceResponse,
			ServiceTypeDiscovery,
			connID,
			msg.Header().MessageID,
			respPayload,
		)

		conn, found := protocolMgr.GetConnection(connID)
		if !found {
			return fmt.Errorf("connection not found: %s", connID)
		}

		return conn.Send(respMsg)
	})

	// 注册取消服务监听处理器
	serviceHandler.RegisterHandler(MsgTypeUnwatchService, func(connID string, msg *protocol.CustomMessage) error {
		// 解析请求
		var req UnwatchServiceRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 从映射中查找监听器ID
		watcherID := req.WatcherID
		if watcherID == "" {
			if connWatchers, ok := watcherMap[connID]; ok {
				if id, ok := connWatchers[req.ServiceName]; ok {
					watcherID = id
				}
			}
		}

		if watcherID == "" {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("watcher not found"))
		}

		// 移除监听器
		err := discoveryService.RemoveWatcher(req.ServiceName, watcherID)

		// 无论成功与否，都从映射中移除
		if connWatchers, ok := watcherMap[connID]; ok {
			delete(connWatchers, req.ServiceName)
			if len(connWatchers) == 0 {
				delete(watcherMap, connID)
			}
		}

		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 构造统一响应结构
		response := ServiceResponse{
			Success: true,
			Type:    "unwatch",
			Message: "服务监听已取消",
		}

		// 发送成功响应
		respPayload, _ := json.Marshal(response)
		respMsg := protocol.NewMessage(
			MsgTypeServiceResponse,
			ServiceTypeDiscovery,
			connID,
			msg.Header().MessageID,
			respPayload,
		)

		conn, found := protocolMgr.GetConnection(connID)
		if !found {
			return fmt.Errorf("connection not found: %s", connID)
		}

		return conn.Send(respMsg)
	})

	// 新增：注册服务注册处理器
	serviceHandler.RegisterHandler(MsgTypeRegisterService, func(connID string, msg *protocol.CustomMessage) error {
		// 解析请求
		var req RegisterServiceRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 验证服务信息
		if req.Service.ID == "" || req.Service.Name == "" || req.Service.Address == "" {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("invalid service info: missing required fields"))
		}

		// 设置注册时间
		if req.Service.RegisterTime.IsZero() {
			req.Service.RegisterTime = time.Now()
		}

		// 注册服务
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := discoveryService.Register(ctx, req.Service)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 将响应数据序列化为JSON字符串
		serviceIDJSON, err := json.Marshal(map[string]string{"serviceId": req.Service.ID})
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("序列化服务ID失败: %v", err))
		}

		// 构造统一响应结构
		response := ServiceResponse{
			Success: true,
			Type:    "register",
			Message: "服务注册成功",
			Data:    string(serviceIDJSON),
		}

		// 发送成功响应
		respPayload, _ := json.Marshal(response)

		respMsg := protocol.NewMessage(
			MsgTypeServiceResponse,
			ServiceTypeDiscovery,
			connID,
			msg.Header().MessageID,
			respPayload,
		)

		conn, found := protocolMgr.GetConnection(connID)
		if !found {
			return fmt.Errorf("connection not found: %s", connID)
		}

		return conn.Send(respMsg)
	})

	// 新增：注册服务注销处理器
	serviceHandler.RegisterHandler(MsgTypeDeregisterService, func(connID string, msg *protocol.CustomMessage) error {
		// 解析请求
		var req DeregisterServiceRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 验证服务ID
		if req.ServiceID == "" {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("invalid service ID: empty"))
		}

		// 注销服务
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := discoveryService.Deregister(ctx, req.ServiceID)
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, err)
		}

		// 将响应数据序列化为JSON字符串
		serviceIDJSON, err := json.Marshal(map[string]string{"serviceId": req.ServiceID})
		if err != nil {
			return sendErrorResponse(protocolMgr, msg, fmt.Errorf("序列化服务ID失败: %v", err))
		}

		// 构造统一响应结构
		response := ServiceResponse{
			Success: true,
			Type:    "deregister",
			Message: "服务已注销",
			Data:    string(serviceIDJSON),
		}

		// 发送成功响应
		respPayload, _ := json.Marshal(response)

		respMsg := protocol.NewMessage(
			MsgTypeServiceResponse,
			ServiceTypeDiscovery,
			connID,
			msg.Header().MessageID,
			respPayload,
		)

		conn, found := protocolMgr.GetConnection(connID)
		if !found {
			return fmt.Errorf("connection not found: %s", connID)
		}

		return conn.Send(respMsg)
	})

	// 注册服务
	protocolMgr.RegisterService(ServiceTypeDiscovery, "discovery", serviceHandler)

	// 添加连接关闭回调到服务处理逻辑
	serviceHandler.RegisterHandler(protocol.MsgTypeHeartbeatAck, func(connID string, msg *protocol.CustomMessage) error {
		// 当收到心跳响应时检查连接是否已关闭，如果关闭则清理监听器
		conn, found := protocolMgr.GetConnection(connID)
		if !found || !conn.State().Connected {
			if connWatchers, ok := watcherMap[connID]; ok {
				for serviceName, watcherID := range connWatchers {
					discoveryService.RemoveWatcher(serviceName, watcherID)
				}
				delete(watcherMap, connID)
			}
		}
		return nil
	})

	return nil
}

// 辅助函数：发送错误响应
func sendErrorResponse(protocolMgr *protocol.ProtocolManager, msg *protocol.CustomMessage, err error) error {
	// 根据服务类型和请求类型确定响应消息类型
	var respMsgType protocol.MessageType
	var respType string

	switch msg.Header().ServiceType {
	case ServiceTypeElection:
		respMsgType = MsgTypeLeaderResponse
		respType = "leader"
	case ServiceTypeDiscovery:
		// 根据请求类型设置响应类型
		switch msg.Header().MessageType {
		case MsgTypeDiscoverService:
			respMsgType = MsgTypeServiceResponse
			respType = "discover"
		case MsgTypeWatchService:
			respMsgType = MsgTypeServiceResponse
			respType = "watch"
		case MsgTypeUnwatchService:
			respMsgType = MsgTypeServiceResponse
			respType = "unwatch"
		case MsgTypeRegisterService:
			respMsgType = MsgTypeServiceResponse
			respType = "register"
		case MsgTypeDeregisterService:
			respMsgType = MsgTypeServiceResponse
			respType = "deregister"
		default:
			respMsgType = MsgTypeServiceResponse
			respType = "unknown"
		}
	default:
		respMsgType = 0 // 通用响应类型
		respType = "unknown"
	}

	// 使用统一的错误响应函数
	return protocol.SendErrorResponse(
		protocolMgr,
		msg.Header().ConnID,
		msg.Header().MessageID,
		respMsgType,
		msg.Header().ServiceType,
		respType,
		err,
	)
}

// 生成唯一的消息ID
func generateMessageID() string {
	// 这里应该使用分布式ID生成器
	// 简单实现：使用时间戳+随机数
	return fmt.Sprintf("%d%06d", time.Now().UnixNano()/1000, time.Now().Nanosecond()%1000000)
}
