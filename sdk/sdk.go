package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/utils"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/pkg/auth"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/distributed/idgen"
	"github.com/xsxdot/aio/pkg/distributed/lock"
	"github.com/xsxdot/aio/pkg/distributed/manager"
	"github.com/xsxdot/aio/pkg/distributed/state"
	network2 "github.com/xsxdot/aio/pkg/network"
	"github.com/xsxdot/aio/pkg/protocol"

	"github.com/nats-io/nats.go"
	"github.com/xsxdot/aio/internal/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ServerEndpoint 服务器端点
type ServerEndpoint struct {
	Host string
	Port int
}

// String 返回格式化的服务器地址
func (s ServerEndpoint) String() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// ClientOptions 客户端选项
type ClientOptions struct {
	ServiceInfo *ServiceInfo
	// 认证信息
	ClientID     string
	ClientSecret string
	Token        string
	// 网络选项
	ConnectionTimeout time.Duration
	RetryCount        int
	RetryInterval     time.Duration
	// 协议管理器选项
	ProtocolOptions *protocol.ProtocolManagerOptions
	// 是否自动连接到主节点
	AutoConnectToLeader bool
	// 是否自动注册到服务中心
	AutoRegisterService bool
	// 服务监听间隔
	ServiceWatchInterval time.Duration
	// Etcd服务选项
	EtcdOptions *EtcdServiceOptions
	// 调度器服务选项
	SchedulerOptions *SchedulerServiceOptions
	// 配置中心选项
	ConfigOptions *ConfigServiceOptions
	// Redis缓存服务选项
	RedisOptions *RedisClientOptions
	// NATS消息队列选项
	NatsOptions *NatsClientOptions
	// 指标收集器选项
	MetricsOptions *MetricsCollectorOptions
}

// DefaultClientOptions 默认客户端选项
var DefaultClientOptions = &ClientOptions{
	ConnectionTimeout:    10 * time.Second,
	RetryCount:           3,
	RetryInterval:        2 * time.Second,
	ProtocolOptions:      nil,
	AutoConnectToLeader:  true,
	AutoRegisterService:  true,
	ServiceWatchInterval: 5 * time.Second,
	EtcdOptions:          DefaultEtcdServiceOptions,
	SchedulerOptions:     DefaultSchedulerServiceOptions,
	ConfigOptions:        nil,
	RedisOptions:         DefaultRedisOptions,
	NatsOptions:          DefaultNatsOptions,
	MetricsOptions:       DefaultMetricsCollectorOptions(),
}

func (o *ClientOptions) WithService(serviceName string, port int) *ClientOptions {
	o.ServiceInfo = &ServiceInfo{
		Name: serviceName,
		Port: port,
	}
	return o
}

func (o *ClientOptions) WithAuth(clientID, clientSecret string) *ClientOptions {
	o.ClientID = clientID
	o.ClientSecret = clientSecret
	return o
}

type ServiceInfo struct {
	// 服务名称
	Name string
	// 服务ID
	ID string
	// 服务端口
	Port int
	// 服务元数据
	Metadata map[string]string
	LocalIP  string
	PublicIP string
}

// NodeInfo 扩展的节点信息，基于election.ElectionInfo
type NodeInfo struct {
	// 嵌入选举信息
	election.ElectionInfo
	// 连接ID
	ConnectionID string
	// 是否是主节点
	IsLeader bool
	// 额外元数据
	Metadata map[string]string
}

// Client 统一的SDK客户端
type Client struct {
	log         *zap.Logger
	serviceInfo *ServiceInfo
	// 初始服务器列表
	initialServers []ServerEndpoint
	// 协议管理器
	protocolMgr *protocol.ProtocolManager
	// 认证信息
	authInfo struct {
		ClientID     string
		ClientSecret string
		Token        string
	}
	// 连接选项
	options *ClientOptions

	// 连接互斥锁
	connectionLock sync.RWMutex
	// 连接映射 (connectionID -> Connection)
	connections map[string]*network2.Connection
	// 节点映射 (nodeID -> NodeInfo)
	nodes map[string]*NodeInfo

	// 主节点信息
	leaderLock sync.RWMutex
	leaderNode *NodeInfo

	// 服务发现组件
	Discovery *DiscoveryService

	// ETCD 客户端组件
	Etcd *EtcdService

	// 配置中心组件
	Config *ConfigService

	// Redis 缓存组件
	Redis *RedisService

	// NATS 消息队列组件
	Nats *NatsService

	// 指标收集器组件
	Metrics *MetricsCollector

	// 定时任务组件
	Scheduler *SchedulerService

	// 监听取消函数和控制
	watchCtx     context.Context
	cancelWatch  context.CancelFunc
	electionName string

	// 事件处理器
	leaderChangeHandlers     []func(oldLeader, newLeader *NodeInfo)
	connectionStatusHandlers []func(nodeID, connID string, connected bool)

	// TCP API 客户端
	tcpAPIClient *TCPAPIClient
}

// NewClient 创建新的客户端
func NewClient(servers []ServerEndpoint, options *ClientOptions) *Client {
	if options == nil {
		options = DefaultClientOptions
	}

	serviceInfo := options.ServiceInfo
	if serviceInfo != nil {
		if serviceInfo.LocalIP == "" {
			serviceInfo.LocalIP = utils.GetLocalIP()
		}
		if serviceInfo.PublicIP == "" {
			serviceInfo.PublicIP = utils.GetPublicIP()
		}
	}

	// 创建上下文
	watchCtx, cancelWatch := context.WithCancel(context.Background())

	client := &Client{
		log:            common.GetLogger().GetZapLogger("AIO-SDK"),
		serviceInfo:    serviceInfo,
		initialServers: servers,
		options:        options,
		connections:    make(map[string]*network2.Connection),
		nodes:          make(map[string]*NodeInfo),
		watchCtx:       watchCtx,
		cancelWatch:    cancelWatch,
		electionName:   "aio/election",
	}

	// 保存认证信息
	client.authInfo.ClientID = options.ClientID
	client.authInfo.ClientSecret = options.ClientSecret
	client.authInfo.Token = options.Token

	// 创建协议管理器
	var clientOptions *protocol.ClientOptions
	if options.ClientID != "" || options.ClientSecret != "" {
		clientOptions = &protocol.ClientOptions{
			EnableAuth:        true,
			ClientID:          options.ClientID,
			ClientSecret:      options.ClientSecret,
			ReadTimeout:       options.ConnectionTimeout,
			WriteTimeout:      options.ConnectionTimeout,
			IdleTimeout:       options.ConnectionTimeout * 2,
			MaxConnections:    100,
			BufferSize:        4096,
			EnableKeepAlive:   true,
			HeartbeatInterval: 30 * time.Second,
		}
	} else {
		clientOptions = &protocol.ClientOptions{
			EnableAuth:        false,
			ReadTimeout:       options.ConnectionTimeout,
			WriteTimeout:      options.ConnectionTimeout,
			IdleTimeout:       options.ConnectionTimeout * 2,
			MaxConnections:    100,
			BufferSize:        4096,
			EnableKeepAlive:   true,
			HeartbeatInterval: 30 * time.Second,
		}
	}

	// 使用新的纯客户端API
	client.protocolMgr = protocol.NewClientWithOptions(clientOptions)

	// 注册各种处理器
	client.registerHandlers()

	// 创建TCP API客户端
	client.tcpAPIClient = NewTCPAPIClient(client)

	// 创建服务发现组件（仅初始化服务发现组件）
	client.Discovery = NewDiscoveryService(client, &DiscoveryServiceOptions{
		ServiceWatchInterval: options.ServiceWatchInterval,
	})

	// 注册服务发现处理器
	client.Discovery.RegisterServiceDiscoveryHandler()

	if serviceInfo != nil && options.AutoRegisterService {
		id, err := client.Discovery.RegisterService(context.Background(), discovery.ServiceInfo{
			ID:      serviceInfo.ID,
			Name:    serviceInfo.Name,
			Address: serviceInfo.LocalIP,
			Port:    serviceInfo.Port,
			Metadata: map[string]string{
				"public_ip": serviceInfo.PublicIP,
			},
		})
		if err != nil {
			client.log.Error("服务注册失败", zap.Error(err))
		} else {
			serviceInfo.ID = id
		}
	}

	client.Config = NewConfigService(client, options.ConfigOptions)

	// 初始化Etcd服务组件
	if options.EtcdOptions != nil {
		client.Etcd = NewEtcdService(client, options.EtcdOptions)
	} else {
		client.Etcd = NewEtcdService(client, DefaultEtcdServiceOptions)
	}

	// 初始化调度器服务组件
	if options.SchedulerOptions != nil {
		client.Scheduler = NewSchedulerService(client, options.SchedulerOptions)
	} else {
		client.Scheduler = NewSchedulerService(client, DefaultSchedulerServiceOptions)
	}

	// 注册定时刷新token的任务（Token有效期为48小时，每47小时刷新一次）
	if client.Scheduler != nil && client.authInfo.Token != "" {
		client.registerTokenRefreshTask()
	}

	return client
}

// 注册所有事件处理器
func (c *Client) registerHandlers() {
	// 认证响应处理器
	c.registerAuthHandler()

	// 主节点事件处理器
	c.registerLeaderEventHandlers()
}

// 注册认证响应处理器
func (c *Client) registerAuthHandler() {
	handler := protocol.NewServiceHandler()

	// 注册认证响应处理函数
	handler.RegisterHandler(protocol.MsgTypeAuthResponse, func(connID string, msg *protocol.CustomMessage) error {
		// 处理认证响应
		// 解析认证响应
		var errorResp map[string]string
		if err := json.Unmarshal(msg.Payload(), &errorResp); err == nil {
			if errMsg, ok := errorResp["error"]; ok {
				fmt.Printf("认证失败: %s\n", errMsg)
				return fmt.Errorf("认证失败: %s", errMsg)
			}
		}

		// 解析为正常的令牌响应
		var token auth.Token
		err := json.Unmarshal(msg.Payload(), &token)
		if err != nil {
			return fmt.Errorf("解析认证响应失败: %w", err)
		}

		// 认证成功，保存令牌
		if token.AccessToken != "" {
			c.authInfo.Token = token.AccessToken
		}

		fmt.Println("认证成功")
		return nil
	})

	// 注册刷新token响应处理函数
	handler.RegisterHandler(protocol.MsgTypeRefreshTokenResponse, func(connID string, msg *protocol.CustomMessage) error {
		// 处理刷新token响应
		var errorResp map[string]string
		if err := json.Unmarshal(msg.Payload(), &errorResp); err == nil {
			if errMsg, ok := errorResp["error"]; ok {
				fmt.Printf("刷新token失败: %s\n", errMsg)
				return fmt.Errorf("刷新token失败: %s", errMsg)
			}
		}

		// 解析为正常的令牌响应
		var token auth.Token
		err := json.Unmarshal(msg.Payload(), &token)
		if err != nil {
			return fmt.Errorf("解析刷新token响应失败: %w", err)
		}

		// 更新令牌
		if token.AccessToken != "" {
			c.authInfo.Token = token.AccessToken
			fmt.Println("刷新token成功")
		}

		return nil
	})

	// 注册到协议管理器
	c.protocolMgr.RegisterService(protocol.ServiceTypeSystem, "auth-handler", handler)
}

// 注册主节点事件处理器
func (c *Client) registerLeaderEventHandlers() {
	handler := protocol.NewServiceHandler()

	// 注册主节点响应处理函数
	handler.RegisterHandler(distributed.MsgTypeLeaderResponse, func(connID string, msg *protocol.CustomMessage) error {
		// 解析为统一的API响应格式
		var apiResp protocol.APIResponse
		if err := json.Unmarshal(msg.Payload(), &apiResp); err != nil {
			return fmt.Errorf("解析主节点响应失败: %w", err)
		}

		// 检查操作是否成功
		if !apiResp.Success {
			return fmt.Errorf("获取主节点失败: %s", apiResp.Error)
		}

		// 数据不能为空
		if apiResp.Data == "" {
			return fmt.Errorf("获取主节点信息为空")
		}

		// 从Data字段解析LeaderInfo
		var leaderInfo distributed.LeaderInfo
		if err := json.Unmarshal([]byte(apiResp.Data), &leaderInfo); err != nil {
			return fmt.Errorf("解析主节点信息失败: %w", err)
		}

		// 更新主节点信息
		c.updateLeaderInfo(&leaderInfo, connID)
		return nil
	})

	// 注册到协议管理器
	c.protocolMgr.RegisterService(distributed.ServiceTypeElection, "election-handler", handler)
}

// connectToNode 连接到指定节点的通用方法
func (c *Client) connectToNode(nodeInfo *NodeInfo, networkOptions *network2.Options) (string, error) {
	// 如果已经连接到该节点，直接返回连接ID
	c.connectionLock.RLock()
	for _, node := range c.nodes {
		if node.NodeID == nodeInfo.NodeID {
			if conn, ok := c.connections[node.ConnectionID]; ok && conn != nil {
				connID := node.ConnectionID
				c.connectionLock.RUnlock()
				return connID, nil
			}
		}
	}
	c.connectionLock.RUnlock()

	// 构建地址
	addr := fmt.Sprintf("%s:%d", nodeInfo.IP, nodeInfo.ProtocolPort)

	// 直接使用协议管理器连接节点
	conn, err := c.protocolMgr.Connect(addr, networkOptions)
	if err != nil {
		return "", fmt.Errorf("连接节点 %s 失败: %w", addr, err)
	}

	// 保存连接信息
	c.connectionLock.Lock()
	// 双重检查，确保在获取锁的过程中没有其他goroutine已经建立了相同的连接
	for _, node := range c.nodes {
		if node.NodeID == nodeInfo.NodeID {
			if existingConn, ok := c.connections[node.ConnectionID]; ok && existingConn != nil {
				connID := node.ConnectionID
				c.connectionLock.Unlock()
				// 关闭新建的连接，使用已有的连接
				conn.Close()
				return connID, nil
			}
		}
	}
	// 保存新连接
	c.connections[conn.ID()] = conn
	nodeInfo.ConnectionID = conn.ID()
	nodeInfo.LastEventTime = time.Now()
	c.nodes[nodeInfo.NodeID] = nodeInfo
	c.connectionLock.Unlock()

	// 如果启用了认证，发送认证请求
	if c.options.ClientID != "" || c.options.ClientSecret != "" {
		// 直接使用协议管理器的认证功能
		newConn, token, err := c.protocolMgr.ConnectWithAuth(
			addr,
			networkOptions)
		if err != nil {
			// 认证失败，关闭连接
			c.connectionLock.Lock()
			c.protocolMgr.CloseConnection(conn.ID())
			delete(c.connections, conn.ID())
			delete(c.nodes, nodeInfo.NodeID)
			c.connectionLock.Unlock()
			return "", fmt.Errorf("节点 %s 认证失败: %w", nodeInfo.NodeID, err)
		}

		// 如果返回的是新连接，则替换原来的连接
		if newConn != nil && newConn.ID() != conn.ID() {
			c.connectionLock.Lock()
			c.protocolMgr.CloseConnection(conn.ID())
			delete(c.connections, conn.ID())
			c.connections[newConn.ID()] = newConn
			nodeInfo.ConnectionID = newConn.ID()
			c.connectionLock.Unlock()
		}

		// 保存认证令牌
		if token != nil && token.AccessToken != "" {
			c.authInfo.Token = token.AccessToken
		}
	}

	// 通知连接状态变更
	c.notifyConnectionStatusChange(nodeInfo.NodeID, conn.ID(), true)

	return conn.ID(), nil
}

// Connect 连接到初始服务器列表
func (c *Client) Connect() error {
	c.connectionLock.Lock()

	if len(c.initialServers) == 0 {
		c.connectionLock.Unlock()
		return fmt.Errorf("没有可用的服务器")
	}

	// 创建网络选项
	networkOptions := &network2.Options{
		ReadTimeout:  c.options.ConnectionTimeout,
		WriteTimeout: c.options.ConnectionTimeout,
		IdleTimeout:  c.options.ConnectionTimeout * 2,
	}

	// 1. 尝试连接初始服务器
	var firstConn string
	var firstNodeID string
	var err error
	var lastErr error

	// 依次尝试每个初始服务器
	for i, server := range c.initialServers {
		// 创建节点信息
		nodeInfo := &NodeInfo{
			ElectionInfo: election.ElectionInfo{
				Name:         fmt.Sprintf("node-initial-%d", i),
				IP:           server.Host,
				ProtocolPort: server.Port,
			},
			Metadata: map[string]string{"type": "initial"},
		}

		// 尝试连接，支持重试
		for retries := 0; retries <= c.options.RetryCount; retries++ {
			// 如果不是第一次尝试，等待一段时间
			if retries > 0 {
				time.Sleep(c.options.RetryInterval)
				fmt.Printf("重试连接 %s (尝试 %d/%d)\n", server.String(), retries, c.options.RetryCount)
			}

			// 暂时释放锁再尝试连接，避免在连接期间长时间持有锁
			c.connectionLock.Unlock()
			firstConn, err = c.connectToNode(nodeInfo, networkOptions)
			c.connectionLock.Lock()

			if err == nil {
				firstNodeID = nodeInfo.NodeID
				break
			}
			lastErr = err
		}

		if err == nil {
			break // 连接成功，跳出循环
		}
	}

	if firstConn == "" {
		c.connectionLock.Unlock()
		return fmt.Errorf("所有服务器连接失败，最后一个错误: %v", lastErr)
	}

	// 初始连接成功后释放锁
	c.connectionLock.Unlock()

	// 5. 获取主节点信息 - 不再需要加锁
	leaderInfo, err := c.refreshLeaderInfo()
	if err != nil {
		fmt.Printf("获取主节点信息失败: %v，但将继续使用已连接的节点\n", err)
		// 继续处理，即使没有主节点信息
	} else if leaderInfo.NodeID != firstNodeID {
		// 6. 如果当前连接的节点不是主节点，则连接主节点
		ctx, cancel := context.WithTimeout(context.Background(), c.options.ConnectionTimeout)
		defer cancel()
		err = c.ConnectToLeader(ctx)
		if err != nil {
			fmt.Printf("连接主节点失败: %v，但将继续使用已连接的节点\n", err)
		}
	}

	// 7. 初始化服务发现
	serviceWatchCtx, serviceWatchCancel := context.WithCancel(c.watchCtx)
	defer serviceWatchCancel()

	// 10. 开始监听服务 - 不需要在此处加锁
	err = c.Discovery.WatchService(serviceWatchCtx, "aio-service")
	if err != nil {
		fmt.Printf("监听服务 aio-service 失败: %v，但将继续使用已连接的节点\n", err)
	}

	// 11. 等待服务发现完成初始检索
	time.Sleep(1 * time.Second)

	// 12. 获取所有服务节点并连接
	c.connectToServiceNodes("aio-service", firstNodeID, networkOptions)

	// 13. 注册节点变更处理函数
	c.setupNodeChangeHandler("aio-service", networkOptions)

	return nil
}

// connectToServiceNodes 连接到服务的所有节点（排除指定的节点ID）
func (c *Client) connectToServiceNodes(serviceName, excludeNodeID string, networkOptions *network2.Options) {
	serviceNodes, err := c.Discovery.GetServiceNodes(serviceName)
	if err != nil {
		fmt.Printf("获取服务节点信息失败: %v，但将继续使用已连接的节点\n", err)
		return
	}

	// 获取主节点ID
	c.leaderLock.RLock()
	leaderID := ""
	if c.leaderNode != nil {
		leaderID = c.leaderNode.NodeID
	}
	c.leaderLock.RUnlock()

	// 连接到所有非排除节点
	for _, svc := range serviceNodes {
		// 跳过已排除的节点和主节点
		if svc.ID == excludeNodeID || svc.ID == leaderID {
			continue
		}

		// 创建节点信息
		nodeInfo := &NodeInfo{
			ElectionInfo: election.ElectionInfo{
				Name:         svc.Name,
				NodeID:       svc.ID,
				IP:           svc.Address,
				ProtocolPort: svc.Port,
			},
			Metadata: svc.Metadata,
		}

		// 尝试连接节点
		_, err := c.connectToNode(nodeInfo, networkOptions)
		if err != nil {
			fmt.Printf("连接服务节点 %s:%d 失败: %v\n", svc.Address, svc.Port, err)
		}
	}
}

// setupNodeChangeHandler 设置节点变更处理函数
func (c *Client) setupNodeChangeHandler(serviceName string, networkOptions *network2.Options) {
	c.Discovery.OnServiceNodesChange(func(svcName string, added, removed []discovery.ServiceInfo) {
		if svcName != serviceName {
			return
		}

		// 处理新增节点
		for _, svc := range added {
			// 创建节点信息
			nodeInfo := &NodeInfo{
				ElectionInfo: election.ElectionInfo{
					Name:         svc.Name,
					NodeID:       svc.ID,
					IP:           svc.Address,
					ProtocolPort: svc.Port,
				},
				Metadata: svc.Metadata,
			}

			_, err := c.connectToNode(nodeInfo, networkOptions)
			if err != nil {
				fmt.Printf("连接新增服务节点 %s:%d 失败: %v\n", svc.Address, svc.Port, err)
			}
		}

		// 处理移除节点
		for _, svc := range removed {
			c.connectionLock.Lock()

			existingNode, exists := c.nodes[svc.ID]
			if exists {
				connID := existingNode.ConnectionID
				conn, hasConn := c.connections[connID]

				if hasConn && conn != nil {
					// 关闭连接
					c.protocolMgr.CloseConnection(connID)
					delete(c.connections, connID)
				}

				delete(c.nodes, svc.ID)

				// 通知连接状态变更
				c.notifyConnectionStatusChange(svc.ID, connID, false)
			}

			c.connectionLock.Unlock()
		}

		// 节点变更后，重新获取主节点信息
		c.handleLeaderUpdate()
	})
}

// handleLeaderUpdate 处理主节点更新
func (c *Client) handleLeaderUpdate() {
	go func() {
		// 添加延迟，避免频繁刷新
		time.Sleep(5 * time.Second)

		newLeaderInfo, err := c.refreshLeaderInfo()
		if err != nil {
			fmt.Printf("刷新主节点信息失败: %v\n", err)
			return
		}

		// 如果没有连接到新的主节点，则尝试连接
		c.leaderLock.RLock()
		leaderID := ""
		if c.leaderNode != nil {
			leaderID = c.leaderNode.NodeID
		}
		c.leaderLock.RUnlock()

		if newLeaderInfo.NodeID != leaderID {
			ctx, cancel := context.WithTimeout(context.Background(), c.options.ConnectionTimeout)
			defer cancel()
			err = c.ConnectToLeader(ctx)
			if err != nil {
				fmt.Printf("连接新主节点失败: %v\n", err)
			}
		}
	}()
}

// ConnectToLeader 连接到主节点
func (c *Client) ConnectToLeader(ctx context.Context) error {
	// 获取主节点信息
	leaderInfo, err := c.GetLeaderInfo(ctx)
	if err != nil {
		return err
	}

	// 创建网络选项
	networkOptions := &network2.Options{
		ReadTimeout:  c.options.ConnectionTimeout,
		WriteTimeout: c.options.ConnectionTimeout,
		IdleTimeout:  c.options.ConnectionTimeout * 2,
	}

	// 连接主节点
	_, err = c.connectToNode(leaderInfo, networkOptions)
	if err != nil {
		return fmt.Errorf("连接主节点失败: %w", err)
	}

	return nil
}

// GetLeaderInfo 获取主节点信息
func (c *Client) GetLeaderInfo(ctx context.Context) (*NodeInfo, error) {
	c.leaderLock.RLock()
	leader := c.leaderNode
	c.leaderLock.RUnlock()

	// 如果已经有主节点信息，直接返回
	if leader != nil {
		return leader, nil
	}

	// 发送获取主节点请求
	return c.refreshLeaderInfo()
}

// refreshLeaderInfo 刷新主节点信息
func (c *Client) refreshLeaderInfo() (*NodeInfo, error) {
	// 添加一个锁，避免多个goroutine同时刷新
	c.leaderLock.RLock()
	lastLeaderNode := c.leaderNode
	c.leaderLock.RUnlock()

	// 如果上次更新时间距现在小于3秒，则直接返回缓存的主节点信息
	if lastLeaderNode != nil && time.Since(lastLeaderNode.LastEventTime) < 3*time.Second {
		return lastLeaderNode, nil
	}

	// 构造获取主节点请求
	req := distributed.GetLeaderRequest{
		ElectionName: c.electionName,
	}

	// 序列化请求
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 向每个已连接的节点发送请求
	connList := make([]string, 0)
	c.connectionLock.RLock()
	for connID := range c.connections {
		connList = append(connList, connID)
	}
	c.connectionLock.RUnlock()

	// 使用复制的连接ID列表发送消息，避免长时间持有读锁
	for _, connID := range connList {
		err = c.protocolMgr.SendMessage(
			connID,
			distributed.MsgTypeGetLeader,
			distributed.ServiceTypeElection,
			reqData,
		)
		if err != nil {
			fmt.Printf("发送获取主节点请求到连接 %s 失败: %v\n", connID, err)
			continue
		}
	}

	// 等待响应处理器更新主节点信息
	// 简化处理，返回当前已知的主节点
	time.Sleep(500 * time.Millisecond)

	c.leaderLock.RLock()
	leader := c.leaderNode
	c.leaderLock.RUnlock()

	if leader == nil {
		return nil, fmt.Errorf("获取主节点信息失败")
	}

	return leader, nil
}

// updateLeaderInfo 更新主节点信息
func (c *Client) updateLeaderInfo(leaderInfo *distributed.LeaderInfo, connID string) {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()

	// 如果主节点信息相同，则不更新
	if c.leaderNode != nil && c.leaderNode.NodeID == leaderInfo.NodeID {
		// 检查其他关键信息是否相同
		if c.leaderNode.IP == leaderInfo.IP && c.leaderNode.ProtocolPort == leaderInfo.ProtocolPort {
			// 主节点信息没有变化，忽略此次更新
			return
		}
	}

	oldLeader := c.leaderNode

	// 创建新的主节点信息
	newLeader := &NodeInfo{
		ElectionInfo: election.ElectionInfo{
			Name:          c.electionName,
			Leader:        leaderInfo.NodeID,
			NodeID:        leaderInfo.NodeID,
			IP:            leaderInfo.IP,
			ProtocolPort:  leaderInfo.ProtocolPort,
			CachePort:     leaderInfo.CachePort,
			LastEventTime: time.Now(),
			Status:        "running", // 假设状态为运行中
			CreateTime:    time.Now().Format(time.RFC3339),
		},
		ConnectionID: connID,
		IsLeader:     true,
		Metadata:     map[string]string{"role": "leader"},
	}

	// 更新主节点信息
	c.leaderNode = newLeader

	// 同时更新节点映射
	c.connectionLock.Lock()
	c.nodes[leaderInfo.NodeID] = newLeader
	c.connectionLock.Unlock()

	// 通知主节点变更
	c.notifyLeaderChange(oldLeader, newLeader)

	fmt.Printf("更新主节点信息: NodeID=%s, IP=%s, Port=%d\n",
		leaderInfo.NodeID, leaderInfo.IP, leaderInfo.ProtocolPort)
}

// 注册各种事件处理器
func (c *Client) OnLeaderChange(handler func(oldLeader, newLeader *NodeInfo)) {
	c.leaderChangeHandlers = append(c.leaderChangeHandlers, handler)
}

func (c *Client) OnConnectionStatusChange(handler func(nodeID, connID string, connected bool)) {
	c.connectionStatusHandlers = append(c.connectionStatusHandlers, handler)
}

// 通知处理器
func (c *Client) notifyLeaderChange(oldLeader, newLeader *NodeInfo) {
	for _, handler := range c.leaderChangeHandlers {
		go handler(oldLeader, newLeader)
	}
}

func (c *Client) notifyConnectionStatusChange(nodeID, connID string, connected bool) {
	for _, handler := range c.connectionStatusHandlers {
		go handler(nodeID, connID, connected)
	}
}

// Close 关闭客户端
func (c *Client) Close() error {
	// 停止指标收集
	if c.Metrics != nil {
		c.Metrics.Stop()
	}

	// 停止服务监听
	c.cancelWatch()

	// 关闭所有连接
	c.connectionLock.Lock()
	for _, conn := range c.connections {
		conn.Close()
	}
	c.connections = make(map[string]*network2.Connection)
	c.connectionLock.Unlock()

	// 关闭已初始化的服务组件
	if c.Discovery != nil {
		if c.serviceInfo != nil && c.options.AutoRegisterService {
			err := c.Discovery.DeregisterService(context.Background(), c.serviceInfo.ID)
			if err != nil {
				c.log.Error("注销服务失败", zap.Error(err))
			}
		}
	}

	if c.Etcd != nil {
		c.Etcd.Close()
	}

	if c.Nats != nil {
		c.Nats.Close()
	}

	// 关闭协议管理器
	if c.protocolMgr != nil {
		// 简化版本没有实现关闭方法
	}

	// 关闭定时任务服务
	if c.Scheduler != nil {
		c.Scheduler.Stop()
	}

	return nil
}

// RegisterServiceHandler 注册服务处理器（供DiscoveryService使用）
func (c *Client) RegisterServiceHandler(svcType protocol.ServiceType, name string, handler *protocol.ServiceHandler) {
	c.protocolMgr.RegisterService(svcType, name, handler)
}

// SendMessage 发送消息（优先使用主节点）
func (c *Client) SendMessage(msgType protocol.MessageType, svcType protocol.ServiceType, payload []byte) error {
	// 使用TCP API客户端发送请求，但仍保留原有逻辑以保持向后兼容性

	// 优先使用主节点
	c.leaderLock.RLock()
	leaderNode := c.leaderNode
	c.leaderLock.RUnlock()

	// 如果有主节点信息，尝试向主节点发送
	if leaderNode != nil && leaderNode.ConnectionID != "" {
		// 检查与主节点的连接是否存在
		c.connectionLock.RLock()
		_, hasConn := c.connections[leaderNode.ConnectionID]
		c.connectionLock.RUnlock()

		if hasConn {
			err := c.protocolMgr.SendMessage(
				leaderNode.ConnectionID,
				msgType,
				svcType,
				payload,
			)

			if err == nil {
				return nil // 成功发送到主节点
			}

			fmt.Printf("向主节点发送消息失败: %v, 将尝试其他节点\n", err)
		}
	}

	// 如果没有主节点或发送失败，尝试向任一可用节点发送
	c.connectionLock.RLock()
	defer c.connectionLock.RUnlock()

	for connID := range c.connections {
		err := c.protocolMgr.SendMessage(
			connID,
			msgType,
			svcType,
			payload,
		)

		if err == nil {
			return nil // 成功发送到某个节点
		}

		fmt.Printf("向节点 %s 发送消息失败: %v\n", connID, err)
	}

	return fmt.Errorf("所有节点发送消息失败")
}

// GetEtcdClient 获取 ETCD 客户端
func (c *Client) GetEtcdClient() (*etcd.EtcdClient, error) {
	etcdService := c.getEtcdService()

	client := etcdService.GetClient()
	if client == nil {
		// 尝试连接
		etcdCtx, etcdCancel := context.WithTimeout(context.Background(), c.options.ConnectionTimeout)
		defer etcdCancel()
		err := etcdService.Connect(etcdCtx)
		if err != nil {
			return nil, fmt.Errorf("获取 ETCD 客户端失败: %w", err)
		}
		client = etcdService.GetClient()
	}

	return client, nil
}

// GetRawEtcdClient 获取原始的 ETCD 客户端
func (c *Client) GetRawEtcdClient() (*clientv3.Client, error) {
	etcdService := c.getEtcdService()

	client := etcdService.GetRawClient()
	if client == nil {
		// 尝试连接
		etcdCtx, etcdCancel := context.WithTimeout(context.Background(), c.options.ConnectionTimeout)
		defer etcdCancel()
		err := etcdService.Connect(etcdCtx)
		if err != nil {
			return nil, fmt.Errorf("获取原始 ETCD 客户端失败: %w", err)
		}
		client = etcdService.GetRawClient()
	}

	return client, nil
}

// GetDistributedManager 获取分布式服务管理器
func (c *Client) GetDistributedManager() (manager.DistributedManager, error) {
	etcdService := c.getEtcdService()
	return etcdService.GetDistributedManager(), nil
}

// GetElectionService 获取选举服务
func (c *Client) GetElectionService() (election.ElectionService, error) {
	etcdService := c.getEtcdService()
	return etcdService.GetElectionService()
}

// GetLockService 获取锁服务
func (c *Client) GetLockService() (lock.LockService, error) {
	etcdService := c.getEtcdService()
	return etcdService.GetLockService()
}

// GetIDGeneratorService 获取ID生成器服务
func (c *Client) GetIDGeneratorService() (idgen.IDGeneratorService, error) {
	etcdService := c.getEtcdService()
	return etcdService.GetIDGeneratorService()
}

// GetStateManagerService 获取状态管理服务
func (c *Client) GetStateManagerService() (state.StateManagerService, error) {
	etcdService := c.getEtcdService()
	return etcdService.GetStateManagerService()
}

// CreateLock 创建一个命名的分布式锁
func (c *Client) CreateLock(ctx context.Context, name string, options ...lock.LockOption) (lock.Lock, error) {
	etcdService := c.getEtcdService()
	return etcdService.CreateLock(ctx, name, options...)
}

// CreateElection 创建一个命名的分布式选举
func (c *Client) CreateElection(ctx context.Context, name string, options ...election.ElectionOption) (election.Election, error) {
	etcdService := c.getEtcdService()
	return etcdService.CreateElection(ctx, name, options...)
}

// CreateIDGenerator 创建一个命名的ID生成器
func (c *Client) CreateIDGenerator(ctx context.Context, name string, options ...idgen.IDGenOption) (idgen.IDGenerator, error) {
	etcdService := c.getEtcdService()
	return etcdService.CreateIDGenerator(ctx, name, options...)
}

// GetNatsClient 获取NATS客户端
func (c *Client) GetNatsClient(ctx context.Context) (*nats.Conn, error) {
	natsService := c.getNatsService()
	return natsService.GetClient(ctx)
}

// GetRedisClient 获取Redis客户端
func (c *Client) GetRedisClient(ctx context.Context) (*RedisClient, error) {
	redisService := c.getRedisService()
	return redisService.Get()
}

// StartMetrics 启动指标收集服务
func (c *Client) StartMetrics(options *MetricsCollectorOptions) (*MetricsCollector, error) {
	metricsCollector := c.getMetricsCollector(options)
	return metricsCollector, metricsCollector.Start()
}

// StopMetrics 停止指标收集服务
func (c *Client) StopMetrics() error {
	if c.Metrics == nil {
		return nil
	}
	return c.Metrics.Stop()
}

// 获取ETCD服务组件，如果未初始化则进行初始化
func (c *Client) getEtcdService() *EtcdService {
	if c.Etcd == nil {
		c.Etcd = NewEtcdService(c, c.options.EtcdOptions)
	}
	return c.Etcd
}

// 获取配置中心服务组件，如果未初始化则进行初始化
func (c *Client) getConfigService() *ConfigService {
	if c.Config == nil {
		c.Config = NewConfigService(c, c.options.ConfigOptions)
	}
	return c.Config
}

// 获取Redis缓存组件，如果未初始化则进行初始化
func (c *Client) getRedisService() *RedisService {
	if c.Redis == nil {
		c.Redis = NewRedisService(c, c.options.RedisOptions)
	}
	return c.Redis
}

// 获取NATS消息队列组件，如果未初始化则进行初始化
func (c *Client) getNatsService() *NatsService {
	if c.Nats == nil {
		c.Nats = NewNatsService(c, c.options.NatsOptions)
	}
	return c.Nats
}

// 获取定时任务组件，如果未初始化则进行初始化
func (c *Client) getSchedulerService() *SchedulerService {
	if c.Scheduler == nil {
		c.Scheduler = NewSchedulerService(c, c.options.SchedulerOptions)
	}
	return c.Scheduler
}

// 获取指标收集器组件，如果未初始化则进行初始化
func (c *Client) getMetricsCollector(options *MetricsCollectorOptions) *MetricsCollector {
	if c.Metrics == nil {
		// 如果提供了选项参数，优先使用参数，否则使用客户端选项
		if options != nil {
			c.Metrics = NewMetricsCollector(c, options)
		} else {
			c.Metrics = NewMetricsCollector(c, c.options.MetricsOptions)
		}
	} else if options != nil {
		// 如果已经创建但提供了新的选项，则使用新选项重新创建
		c.Metrics = NewMetricsCollector(c, options)
	}
	return c.Metrics
}

// ConnectWithAuth 连接到节点并自动进行认证
func (c *Client) ConnectWithAuth() error {
	c.connectionLock.Lock()

	if len(c.initialServers) == 0 {
		c.connectionLock.Unlock()
		return fmt.Errorf("没有可用的服务器")
	}

	// 检查认证信息是否完整
	if c.authInfo.ClientID == "" || c.authInfo.ClientSecret == "" {
		c.connectionLock.Unlock()
		return fmt.Errorf("认证信息不完整，需要提供 ClientID 和 ClientSecret")
	}

	// 创建网络选项
	networkOptions := &network2.Options{
		ReadTimeout:  c.options.ConnectionTimeout,
		WriteTimeout: c.options.ConnectionTimeout,
		IdleTimeout:  c.options.ConnectionTimeout * 2,
	}

	var firstConn *network2.Connection
	var firstToken *auth.Token
	var err error
	var lastErr error

	// 依次尝试每个初始服务器
	for _, server := range c.initialServers {
		addr := server.String()

		// 尝试连接并认证，支持重试
		for retries := 0; retries <= c.options.RetryCount; retries++ {
			// 如果不是第一次尝试，等待一段时间
			if retries > 0 {
				time.Sleep(c.options.RetryInterval)
				fmt.Printf("重试连接并认证 %s (尝试 %d/%d)\n", addr, retries, c.options.RetryCount)
			}

			// 暂时释放锁再尝试连接，避免在连接期间长时间持有锁
			c.connectionLock.Unlock()

			// 使用协议管理器的ConnectWithAuth方法
			firstConn, firstToken, err = c.protocolMgr.ConnectWithAuth(
				addr,
				networkOptions)

			c.connectionLock.Lock()

			if err == nil {
				break
			}
			lastErr = err
		}

		if err == nil {
			break // 连接成功，跳出循环
		}
	}

	if firstConn == nil || firstToken == nil {
		c.connectionLock.Unlock()
		return fmt.Errorf("所有服务器连接或认证失败，最后一个错误: %v", lastErr)
	}

	// 保存连接
	c.connections[firstConn.ID()] = firstConn

	// 保存认证令牌
	c.authInfo.Token = firstToken.AccessToken

	// 释放锁
	c.connectionLock.Unlock()

	// 创建节点信息（简化版本，省略了额外属性的初始化）
	nodeInfo := &NodeInfo{
		ElectionInfo: election.ElectionInfo{
			Name:         "autoauth-node",
			NodeID:       fmt.Sprintf("node-%s", firstConn.ID()),
			IP:           firstConn.RemoteAddr().String(),
			ProtocolPort: 0, // 端口信息不可用，设为0
		},
		ConnectionID: firstConn.ID(),
		Metadata:     map[string]string{"type": "initial", "auth": "auto"},
	}

	// 保存节点信息
	c.connectionLock.Lock()
	c.nodes[nodeInfo.NodeID] = nodeInfo
	c.connectionLock.Unlock()

	// 通知连接状态变更
	c.notifyConnectionStatusChange(nodeInfo.NodeID, firstConn.ID(), true)

	// 继续进行后续初始化和服务发现等操作
	// 与Connect方法相同...

	// 获取主节点信息
	_, err = c.refreshLeaderInfo()
	if err != nil {
		fmt.Printf("获取主节点信息失败: %v，但将继续使用已连接的节点\n", err)
	}

	// 初始化服务发现
	serviceWatchCtx, serviceWatchCancel := context.WithCancel(c.watchCtx)
	defer serviceWatchCancel()

	// 开始监听服务
	err = c.Discovery.WatchService(serviceWatchCtx, "aio-service")
	if err != nil {
		fmt.Printf("监听服务 aio-service 失败: %v，但将继续使用已连接的节点\n", err)
	}

	// 等待服务发现完成初始检索
	time.Sleep(1 * time.Second)

	// 获取所有服务节点并连接
	c.connectToServiceNodes("aio-service", nodeInfo.NodeID, networkOptions)

	// 注册节点变更处理函数
	c.setupNodeChangeHandler("aio-service", networkOptions)

	return nil
}

// GetAllConnections 获取所有当前的连接
func (c *Client) GetAllConnections() []*network2.Connection {
	c.connectionLock.RLock()
	defer c.connectionLock.RUnlock()

	connections := make([]*network2.Connection, 0, len(c.connections))
	for _, conn := range c.connections {
		connections = append(connections, conn)
	}

	return connections
}

// registerTokenRefreshTask 注册定时刷新token的任务
func (c *Client) registerTokenRefreshTask() {
	// 每47小时刷新一次token（Token有效期为48小时）
	refreshInterval := 47 * time.Hour

	// 注册定时任务
	_, err := c.Scheduler.AddIntervalTask(
		"token-refresh",
		refreshInterval,
		false,
		func(ctx context.Context, params map[string]interface{}) error {
			return c.RefreshToken(ctx)
		},
		nil,
		false,
	)

	if err != nil {
		fmt.Printf("注册token刷新任务失败: %v\n", err)
	} else {
		fmt.Println("已注册token自动刷新任务，间隔为47小时")
	}
}

// RefreshToken 刷新令牌
func (c *Client) RefreshToken(ctx context.Context) error {
	c.leaderLock.RLock()
	token := c.authInfo.Token
	c.leaderLock.RUnlock()

	if token == "" {
		return fmt.Errorf("无法刷新令牌: 当前无令牌")
	}

	// 构造刷新令牌的请求
	req := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}

	// 使用TCP API客户端发送请求
	resp, err := c.tcpAPIClient.Send(
		ctx,
		protocol.MsgTypeRefreshToken,
		protocol.ServiceTypeSystem,
		protocol.MsgTypeRefreshTokenResponse,
		req,
	)
	if err != nil {
		return fmt.Errorf("刷新令牌请求失败: %w", err)
	}

	// 从响应中提取新令牌
	var tokenResp struct {
		NewToken string `json:"newToken"`
	}
	if err := json.Unmarshal([]byte(resp.Data), &tokenResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if tokenResp.NewToken == "" {
		return fmt.Errorf("服务器返回的令牌为空")
	}

	// 更新客户端的令牌
	c.authInfo.Token = tokenResp.NewToken
	fmt.Println("令牌刷新成功")

	return nil
}

// GetTCPAPIClient 获取TCP API客户端
func (c *Client) GetTCPAPIClient() *TCPAPIClient {
	if c.tcpAPIClient == nil {
		c.tcpAPIClient = NewTCPAPIClient(c)
	}
	return c.tcpAPIClient
}
