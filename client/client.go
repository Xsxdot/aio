package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/protocol"
	"go.uber.org/zap"
)

type Client struct {
	mu          sync.Mutex
	log         *zap.Logger
	serviceInfo *ServiceInfo
	options     *ClientOptions
	once        sync.Once

	leaderInfo         *NodeInfo
	leaderChangeHandle []func(leaderInfo *NodeInfo)

	protocolService *ProtocolService
	requestService  *RequestService

	Discovery  *DiscoveryService
	Config     *ConfigService
	Etcd       *EtcdService
	Scheduler  *Scheduler
	Nats       *NatsClient
	Cache      *CacheService
	Monitoring *MonitoringClient
}

type Config struct {
	Endpoints []string
}

func (o *ClientOptions) NewClient() *Client {
	return &Client{
		log:         common.GetLogger().GetZapLogger("aio-client"),
		serviceInfo: o.serviceInfo,
		options:     o,
	}
}

func (c *Client) Start(ctx context.Context) error {
	protocolService := NewProtocolService(c, c.options.protocolOptions)
	err := protocolService.Start(ctx)
	if err != nil {
		c.log.Error("启动协议服务失败", zap.Error(err))
		panic(err)
	}
	c.protocolService = protocolService

	requestService := NewRequestService(c, protocolService)
	c.requestService = requestService

	// 初始化服务发现和配置服务
	c.Discovery = NewDiscoveryService(c)
	c.Config = NewConfigService(c)
	c.Etcd = NewEtcdService(c)
	err = c.Etcd.Connect(ctx)
	if err != nil {
		panic(err)
	}

	err = c.RequestLeader()
	if err != nil {
		c.log.Error("请求leader失败", zap.Error(err))
	}

	return nil
}

func (c *Client) GetScheduler() (*Scheduler, error) {
	if c.Scheduler != nil {
		return c.Scheduler, nil
	}
	// 初始化调度器服务
	c.Scheduler = NewScheduler(c, nil)
	if err := c.Scheduler.Start(); err != nil {
		c.log.Error("启动调度器服务失败", zap.Error(err))
		return nil, err
	}
	return c.Scheduler, nil
}

func (c *Client) GetProtocolService() *ProtocolService {
	return c.protocolService
}

func (c *Client) GetRequestService() *RequestService {
	return c.requestService
}

func (c *Client) GetServiceInfo() *ServiceInfo {
	return c.serviceInfo
}

func (c *Client) GetConfigService() *ConfigService {
	return c.Config
}

func (c *Client) InitNats(ctx context.Context, options *NatsClientOptions) (*nats.Conn, error) {
	client := NewNatsClient(c, options)
	c.Nats = client
	return client.GetOriginClient(ctx)
}

func (c *Client) InitCache(ctx context.Context, options *RedisClientOptions) (*RedisClient, error) {
	client := NewCacheClient(c, options)
	c.Cache = client
	return client.GetOriginClient(ctx)
}

func (c *Client) GetNatsService() *NatsClient {
	return c.Nats
}

func (c *Client) GetCacheService() *CacheService {
	return c.Cache
}

func (c *Client) GetLeaderInfo() *NodeInfo {
	return c.leaderInfo
}

func (c *Client) GetLeaderConnId() (string, error) {
	if c.leaderInfo == nil {
		return "", fmt.Errorf("leader info is nil")
	}

	return c.leaderInfo.ConnectionID, nil
}

func (c *Client) RequestLeader() error {
	leaderInfo := new(distributed.LeaderInfo)
	message := protocol.NewMessage(distributed.MsgTypeGetLeader, protocol.ServiceTypeElection, "", nil)
	err := c.requestService.Request(message, leaderInfo)
	if err != nil {
		return err
	}
	c.log.Info("获取leader信息成功", zap.Any("leaderInfo", leaderInfo))
	c.updateLeaderInfo(leaderInfo)

	c.once.Do(func() {
		c.protocolService.manager.RegisterHandle(protocol.ServiceTypeElection, distributed.MsgTypeLeaderNotify, func(connID string, msg *protocol.CustomMessage) (interface{}, error) {
			payload := msg.Payload()
			leaderInfo := new(distributed.LeaderInfo)
			err := json.Unmarshal(payload, leaderInfo)
			if err != nil {
				c.log.Error("解析leaderInfo失败", zap.Error(err), zap.String("payload", string(payload)))
				return nil, nil
			}
			c.updateLeaderInfo(leaderInfo)
			return nil, nil
		})
	})

	return nil
}

func (c *Client) updateLeaderInfo(leaderInfo *distributed.LeaderInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.leaderInfo != nil && c.leaderInfo.NodeID == leaderInfo.NodeID {
		return
	}

	nodeInfo := &NodeInfo{
		IP:           leaderInfo.IP,
		ProtocolPort: leaderInfo.ProtocolPort,
		CachePort:    leaderInfo.CachePort,
		NodeID:       leaderInfo.NodeID,
		LastUpdate:   leaderInfo.LastUpdate,
		IsLeader:     true,
	}
	addr := fmt.Sprintf("%s:%d", nodeInfo.IP, nodeInfo.ProtocolPort)
	for address, connId := range c.protocolService.addr2conn {
		if address == addr {
			nodeInfo.ConnectionID = connId
			break
		}
	}
	c.leaderInfo = nodeInfo

	for _, handle := range c.leaderChangeHandle {
		handle(nodeInfo)
	}
}

func (c *Client) RegisterLeaderChangeHandle(handle func(leaderInfo *NodeInfo)) {
	c.leaderChangeHandle = append(c.leaderChangeHandle, handle)
}

func (c *Client) Connect(endpoints []string) error {
	c.options.protocolOptions.servers = endpoints
	return c.Start(context.Background())
}

func (c *Client) Close() error {
	var lastErr error

	// 先停止监控客户端
	if c.Monitoring != nil {
		c.Monitoring.Stop()
		c.Monitoring = nil
	}

	// 停止调度器服务
	if c.Scheduler != nil {
		if err := c.Scheduler.Stop(); err != nil {
			c.log.Error("停止调度器服务失败", zap.Error(err))
			lastErr = err
		}
		c.Scheduler = nil
	}

	// 关闭 Nats 客户端
	if c.Nats != nil && c.Nats.natsClient != nil {
		c.Nats.natsClient.Close() // NATS 客户端关闭不返回错误
		c.Nats = nil
	}

	// 关闭 Cache 客户端
	if c.Cache != nil && c.Cache.redisClient != nil && c.Cache.redisClient.Client != nil {
		if err := c.Cache.redisClient.Close(); err != nil {
			c.log.Error("关闭 Cache 客户端失败", zap.Error(err))
			lastErr = err
		}
		c.Cache = nil
	}

	// 关闭 Etcd 服务
	if c.Etcd != nil {
		c.Etcd.Close()
		c.Etcd = nil
	}

	// 关闭请求服务
	if c.requestService != nil {
		c.requestService = nil
	}

	// 清理 Discovery 和 Config 服务
	c.Discovery = nil
	c.Config = nil

	// 最后关闭协议服务
	if c.protocolService != nil {
		if err := c.protocolService.Stop(context.Background()); err != nil {
			c.log.Error("关闭协议服务失败", zap.Error(err))
			lastErr = err
		}
		c.protocolService = nil
	}

	c.log.Info("客户端资源已全部清理完毕")
	return lastErr
}

// InitMonitoring 初始化监控客户端
func (c *Client) InitMonitoring(ctx context.Context, options *MonitoringOptions) (*MonitoringClient, error) {
	if c.Nats == nil {
		return nil, fmt.Errorf("必须先初始化NATS客户端")
	}

	if options == nil {
		options = DefaultMonitoringOptions()
	}

	client, err := NewMonitoringClient(c, options)
	if err != nil {
		c.log.Error("初始化监控客户端失败", zap.Error(err))
		return nil, err
	}

	c.Monitoring = client
	return client, nil
}

// GetMonitoringClient 获取监控客户端
func (c *Client) GetMonitoringClient() *MonitoringClient {
	return c.Monitoring
}
