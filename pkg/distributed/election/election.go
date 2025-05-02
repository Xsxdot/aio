package election

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xsxdot/aio/internal/etcd"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/distributed/common"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

// EventType 选举事件类型
type EventType string

const (
	// EventBecomeLeader 成为主节点事件
	EventBecomeLeader EventType = "become_leader"
	// EventBecomeFollower 成为从节点事件
	EventBecomeFollower EventType = "become_follower"
	// EventLeaderChanged 主节点变更事件
	EventLeaderChanged EventType = "leader_changed"
)

// ElectionEvent 选举事件
type ElectionEvent struct {
	// Type 事件类型
	Type EventType `json:"type"`
	// Leader 当前主节点ID
	Leader string `json:"leader"`
	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`
}

// ElectionEventHandler 选举事件处理函数
type ElectionEventHandler func(event ElectionEvent)

// ElectionInfo 选举信息
type ElectionInfo struct {
	// Name 选举名称
	Name string `json:"name"`
	// Leader 当前主节点
	Leader string `json:"leader"`
	// Status 选举状态
	Status common.ComponentStatus `json:"status"`
	// LastEventTime 最后事件时间
	LastEventTime time.Time `json:"lastEventTime"`
	// NodeID 当前节点ID
	NodeID string `json:"nodeId"`
	// CreateTime 创建时间
	CreateTime string `json:"createTime"`
	// IP 节点IP地址
	IP string `json:"ip"`
	// ProtocolPort 协议端口号
	ProtocolPort int `json:"protocolPort"`
	// CachePort 缓存中心端口号
	CachePort int `json:"cachePort"`
}

// Election 选举接口
type Election interface {
	// Campaign 参与选举
	Campaign(ctx context.Context, handler ElectionEventHandler) error
	// AddEventHandler 添加事件处理函数
	AddEventHandler(handler ElectionEventHandler) string
	// RemoveEventHandler 移除事件处理函数
	RemoveEventHandler(handlerID string) bool
	// Resign 放弃主节点身份
	Resign(ctx context.Context) error
	// GetLeader 获取当前主节点
	GetLeader(ctx context.Context) (string, error)
	// GetInfo 获取选举信息
	GetInfo() ElectionInfo
	// IsLeader 检查当前节点是否为主节点
	IsLeader() bool
	stopInternal()
}

// ElectionOption 选举配置选项函数类型
type ElectionOption func(*electionImpl)

// WithElectionTTL 设置选举TTL
func WithElectionTTL(ttl int) ElectionOption {
	return func(e *electionImpl) {
		e.ttl = ttl
	}
}

// WithElectionNodeID 设置节点ID
func WithElectionNodeID(nodeID string) ElectionOption {
	return func(e *electionImpl) {
		e.nodeID = nodeID
	}
}

// WithElectionIP 设置节点IP地址
func WithElectionIP(ip string) ElectionOption {
	return func(e *electionImpl) {
		e.ip = ip
	}
}

// WithElectionProtocolPort 设置协议端口号
func WithElectionProtocolPort(port int) ElectionOption {
	return func(e *electionImpl) {
		e.protocolPort = port
	}
}

// WithElectionCachePort 设置缓存中心端口号
func WithElectionCachePort(port int) ElectionOption {
	return func(e *electionImpl) {
		e.cachePort = port
	}
}

type ElectionConfig struct {
	Prefix        string `yaml:"prefix" json:"prefix"`
	TTL           int    `yaml:"ttl" json:"ttl"`
	RetryInterval string `yaml:"retry_interval" json:"retry_interval"`
	WatchTimeout  string `yaml:"watch_timeout" json:"watch_timeout"`
	IP            string `yaml:"ip" json:"ip"`
	CachePort     int    `yaml:"cache_port" json:"cache_port"`
	ProtocolPort  int    `yaml:"protocol_port" json:"protocol_port"`
	NodeId        string `yaml:"node_id" json:"node_id"`
}

// ElectionService 选举服务接口
type ElectionService interface {
	common.ServerComponent

	GetDefaultElection() Election
	// Create 创建选举实例
	Create(name string, options ...ElectionOption) (Election, error)
	// Get 获取选举实例
	Get(name string) (Election, bool)
	// List 列出所有选举
	List() []ElectionInfo
	// Delete 删除选举实例
	Delete(name string) error
	// Status 获取选举服务状态
	Status() consts.ComponentStatus
	// GetClientConfig 获取客户端配置
	GetClientConfig() (bool, *config.ClientConfig)
}

func (s *electionServiceImpl) RegisterMetadata() (bool, int, map[string]string) {
	return false, 0, nil
}

// 选举服务实现
type electionServiceImpl struct {
	etcdClient            *etcd.EtcdClient
	logger                *zap.Logger
	elections             map[string]Election
	mutex                 sync.RWMutex
	isRunning             bool
	status                consts.ComponentStatus
	defaultElectionConfig ElectionConfig
	defaultElection       Election
}

func (s *electionServiceImpl) GetDefaultElection() Election {
	return s.defaultElection
}

func (s *electionServiceImpl) Name() string {
	return consts.ComponentElection
}

func (s *electionServiceImpl) Status() consts.ComponentStatus {
	return s.status
}

// GetClientConfig 实现Component接口，返回客户端配置
func (s *electionServiceImpl) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}

// DefaultConfig 返回组件的默认配置
func (s *electionServiceImpl) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	return s.genConfig(baseConfig)
}

func (s *electionServiceImpl) genConfig(baseConfig *config.BaseConfig) ElectionConfig {
	return ElectionConfig{
		Prefix:        "aio/election",
		TTL:           5,
		RetryInterval: "1s",
		WatchTimeout:  "5s",
		IP:            baseConfig.Network.BindIP,
		NodeId:        baseConfig.System.NodeId,
		ProtocolPort:  baseConfig.Protocol.Port,
	}
}

func (s *electionServiceImpl) Init(config *config.BaseConfig, body []byte) error {
	defaultElectionConfig := s.genConfig(config)
	s.defaultElectionConfig = defaultElectionConfig
	s.status = consts.StatusInitialized

	return nil
}

func (s *electionServiceImpl) beforeStart(ctx context.Context) error {
	// 构建选举选项
	electionOpts := []ElectionOption{
		WithElectionNodeID(s.defaultElectionConfig.NodeId),
		WithElectionTTL(s.defaultElectionConfig.TTL),
		WithElectionIP(s.defaultElectionConfig.IP),
	}

	// 添加协议端口信息
	if s.defaultElectionConfig.ProtocolPort > 0 {
		electionOpts = append(electionOpts, WithElectionProtocolPort(s.defaultElectionConfig.ProtocolPort))
	}

	// 添加缓存中心端口信息
	if s.defaultElectionConfig.CachePort > 0 {
		electionOpts = append(electionOpts, WithElectionCachePort(s.defaultElectionConfig.CachePort))
	}

	election, err := s.Create(s.defaultElectionConfig.Prefix, electionOpts...)
	if err != nil {
		return err
	}
	s.defaultElection = election

	return nil
}

func (s *electionServiceImpl) Restart(ctx context.Context) error {
	err := s.Stop(ctx)
	if err != nil {
		return err
	}
	return s.Start(ctx)
}

// 选举实现
type electionImpl struct {
	name          string
	nodeID        string
	ttl           int
	ip            string
	protocolPort  int
	cachePort     int
	etcdClient    *etcd.EtcdClient
	logger        *zap.Logger
	session       *concurrency.Session
	election      *concurrency.Election
	leaderKey     string
	isLeader      bool
	stopCh        chan struct{}
	status        common.ComponentStatus
	createTime    time.Time
	lastEvent     time.Time
	eventHandlers map[string]ElectionEventHandler
	mutex         sync.RWMutex
}

// NewElectionService 创建选举服务
func NewElectionService(etcdClient *etcd.EtcdClient, logger *zap.Logger) (ElectionService, error) {
	return &electionServiceImpl{
		etcdClient: etcdClient,
		logger:     logger,
		elections:  make(map[string]Election),
		isRunning:  false,
	}, nil
}

// Start 启动选举服务
func (s *electionServiceImpl) Start(ctx context.Context) error {

	if s.isRunning {
		return nil
	}

	err := s.beforeStart(ctx)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.logger.Info("Starting election service")

	if s.defaultElection == nil {
		return errors.New("default election is not set")
	}

	// 5. 添加选举事件处理函数
	electionCompleteCh := make(chan struct{}, 1)

	handlerID := s.defaultElection.AddEventHandler(func(event ElectionEvent) {
		s.logger.Info(fmt.Sprintf("收到选举事件: %v, Leader: %s", event.Type, event.Leader))

		// 当收到选举结果事件（成为Leader或Follower），表示选举已完成
		if event.Type == EventBecomeLeader ||
			event.Type == EventBecomeFollower {
			// 使用非阻塞发送，确保只发送一次信号
			select {
			case electionCompleteCh <- struct{}{}:
				s.logger.Info(fmt.Sprintf("选举完成，结果: %v, Leader: %s", event.Type, event.Leader))
			default:
				// 已经发送过信号，不做任何处理
			}
		}
	})

	if handlerID == "" {
		s.logger.Warn("添加选举事件处理函数失败")
	}

	// 6. 参与选举
	campaignHandler := func(event ElectionEvent) {
		s.logger.Info(fmt.Sprintf("选举结果: %v, Leader: %s", event.Type, event.Leader))
	}

	// 使用goroutine调用Campaign，避免阻塞
	go func() {
		if err := s.defaultElection.Campaign(ctx, campaignHandler); err != nil {
			s.logger.Error(fmt.Sprintf("参与选举失败: %v", err))
		}
	}()
	s.logger.Info("已参与选举，等待选举结果")

	// 7. 等待选举完成或超时
	select {
	case <-electionCompleteCh:
		s.logger.Info("选举已完成")
	case <-time.After(120 * time.Second): // 将等待时间从30秒改为10秒
		s.logger.Warn("等待选举完成超时，这可能是因为存在旧的选举数据。将继续执行后续初始化，选举将在后台完成")
		// 即使选举超时也继续执行，不返回错误
	}

	s.isRunning = true
	s.status = consts.StatusRunning
	return nil
}

// Stop 停止选举服务
func (s *electionServiceImpl) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping election service")

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 停止所有选举
	for name, election := range s.elections {
		s.logger.Debug("Stopping election", zap.String("name", name))
		// 先主动放弃leader身份，然后再停止实例
		if election.IsLeader() {
			s.logger.Info("Resigning leadership before stopping", zap.String("name", name))
			_ = election.Resign(timeoutCtx)
		}
		election.stopInternal()
	}

	s.isRunning = false
	s.status = consts.StatusStopped
	return nil
}

// Create 创建选举实例
func (s *electionServiceImpl) Create(name string, options ...ElectionOption) (Election, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否已存在
	if _, exists := s.elections[name]; exists {
		return s.elections[name], nil
	}

	// 创建选举实现
	electionImpl := &electionImpl{
		name:          name,
		nodeID:        fmt.Sprintf("node-%d", time.Now().UnixNano()),
		ttl:           15, // 默认TTL 15秒
		ip:            "",
		protocolPort:  0,
		cachePort:     0,
		etcdClient:    s.etcdClient,
		logger:        s.logger.With(zap.String("election", name)),
		stopCh:        make(chan struct{}),
		status:        common.StatusCreated,
		createTime:    time.Now(),
		eventHandlers: make(map[string]ElectionEventHandler),
	}

	// 应用配置选项
	for _, option := range options {
		option(electionImpl)
	}

	s.logger.Info("Created election",
		zap.String("name", name),
		zap.String("nodeID", electionImpl.nodeID),
		zap.Int("ttl", electionImpl.ttl),
		zap.String("ip", electionImpl.ip),
		zap.Int("protocolPort", electionImpl.protocolPort),
		zap.Int("cachePort", electionImpl.cachePort))

	// 保存选举配置到etcd
	if err := s.saveElectionConfig(context.Background(), name, electionImpl); err != nil {
		s.logger.Error("Failed to save election config", zap.Error(err))
		return nil, err
	}

	// 保存到内存
	s.elections[name] = electionImpl

	return electionImpl, nil
}

// Get 获取选举实例
func (s *electionServiceImpl) Get(name string) (Election, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	election, exists := s.elections[name]
	return election, exists
}

// List 列出所有选举
func (s *electionServiceImpl) List() []ElectionInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]ElectionInfo, 0, len(s.elections))
	for _, election := range s.elections {
		result = append(result, election.GetInfo())
	}

	return result
}

// Delete 删除选举实例
func (s *electionServiceImpl) Delete(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	election, exists := s.elections[name]
	if !exists {
		return nil
	}

	// 停止选举
	electionImpl, ok := election.(*electionImpl)
	if ok {
		electionImpl.stopInternal()
	}

	// 从etcd删除选举配置
	key := fmt.Sprintf("/distributed/components/elections/%s/config", name)
	err := s.etcdClient.Delete(context.Background(), key)
	if err != nil {
		s.logger.Error("Failed to delete election config from etcd", zap.Error(err))
		return err
	}

	// 从内存中删除
	delete(s.elections, name)
	s.logger.Info("Deleted election", zap.String("name", name))

	return nil
}

// 保存选举配置到etcd
func (s *electionServiceImpl) saveElectionConfig(ctx context.Context, name string, election *electionImpl) error {
	config := map[string]interface{}{
		"nodeID":       election.nodeID,
		"ttl":          election.ttl,
		"createTime":   election.createTime.Format(time.RFC3339),
		"ip":           election.ip,
		"protocolPort": election.protocolPort,
		"cachePort":    election.cachePort,
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("/distributed/components/elections/%s/config", name)
	err = s.etcdClient.Put(ctx, key, string(data))
	return err
}

// Campaign 参与选举
func (e *electionImpl) Campaign(ctx context.Context, handler ElectionEventHandler) error {
	// 创建etcd会话
	session, err := concurrency.NewSession(e.etcdClient.Client, concurrency.WithTTL(e.ttl))
	if err != nil {
		e.logger.Error("Failed to create session", zap.Error(err))
		return err
	}

	e.session = session

	// 创建选举
	electionKey := fmt.Sprintf("/distributed/elections/%s", e.name)
	e.election = concurrency.NewElection(session, electionKey)
	e.leaderKey = electionKey + "/leader"
	// 启动选举处理
	go e.campaignLoop(ctx, handler)

	e.status = common.StatusRunning
	return nil
}

// campaignLoop 选举循环
func (e *electionImpl) campaignLoop(ctx context.Context, handler ElectionEventHandler) {
	// 监听当前会话
	sessionCh := e.session.Done()

	// 开始竞选
	campaignCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 参与选举
	e.logger.Info("Starting election campaign", zap.String("nodeID", e.nodeID))

	// 发送事件给所有处理函数
	notifyAllHandlers := func(event ElectionEvent) {
		// 首先通知传入的主处理函数
		if handler != nil {
			handler(event)
		}

		// 然后通知所有注册的处理函数
		e.mutex.RLock()
		handlers := make([]ElectionEventHandler, 0, len(e.eventHandlers))
		for _, h := range e.eventHandlers {
			handlers = append(handlers, h)
		}
		e.mutex.RUnlock()

		for _, h := range handlers {
			h(event)
		}
	}

	// 执行选举
	doElection := func() {
		// 使用节点ID作为选举值
		err := e.election.Campaign(campaignCtx, e.nodeID)
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			e.logger.Error("Campaign failed", zap.Error(err))
			return
		}

		// 成为主节点
		if err == nil {
			e.isLeader = true
			e.lastEvent = time.Now()

			e.logger.Info("Became leader", zap.String("nodeID", e.nodeID))

			// 发送成为主节点事件
			notifyAllHandlers(ElectionEvent{
				Type:      EventBecomeLeader,
				Leader:    e.nodeID,
				Timestamp: time.Now(),
			})
		}
	}

	// 尝试成为Leader
	go doElection()

	// 监听主节点变化
	watchCh := e.election.Observe(campaignCtx)

	for {
		select {
		case <-e.stopCh:
			e.logger.Info("Election campaign stopped")
			// 如果是主节点，主动放弃
			if e.isLeader {
				_ = e.Resign(context.Background())
			}
			return

		case <-sessionCh:
			// 会话过期，重新创建会话和选举
			e.logger.Warn("Session expired, recreating")
			e.isLeader = false

			// 通知会话过期
			notifyAllHandlers(ElectionEvent{
				Type:      EventBecomeFollower,
				Timestamp: time.Now(),
			})

			// 重新创建会话
			session, err := concurrency.NewSession(e.etcdClient.Client, concurrency.WithTTL(e.ttl))
			if err != nil {
				e.logger.Error("Failed to recreate session", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			// 更新会话和选举
			e.session = session
			sessionCh = e.session.Done()
			e.election = concurrency.NewElection(session, fmt.Sprintf("/distributed/elections/%s", e.name))

			// 重新参与选举
			cancel()
			campaignCtx, cancel = context.WithCancel(ctx)
			watchCh = e.election.Observe(campaignCtx)

			go doElection()

		case resp, ok := <-watchCh:
			if !ok {
				continue
			}

			newLeader := string(resp.Kvs[0].Value)
			e.lastEvent = time.Now()

			// 处理主节点变更
			if newLeader != e.nodeID && e.isLeader {
				e.isLeader = false
				e.logger.Info("No longer leader, new leader", zap.String("leader", newLeader))

				notifyAllHandlers(ElectionEvent{
					Type:      EventBecomeFollower,
					Leader:    newLeader,
					Timestamp: time.Now(),
				})

			} else if newLeader != e.nodeID {
				e.logger.Info("Leader changed", zap.String("leader", newLeader))

				notifyAllHandlers(ElectionEvent{
					Type:      EventLeaderChanged,
					Leader:    newLeader,
					Timestamp: time.Now(),
				})

			}
		}
	}
}

// AddEventHandler 添加事件处理函数
func (e *electionImpl) AddEventHandler(handler ElectionEventHandler) string {
	if handler == nil {
		return ""
	}

	handlerID := fmt.Sprintf("%p", handler)

	e.mutex.Lock()
	e.eventHandlers[handlerID] = handler
	e.mutex.Unlock()

	return handlerID
}

// RemoveEventHandler 移除事件处理函数
func (e *electionImpl) RemoveEventHandler(handlerID string) bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	_, exists := e.eventHandlers[handlerID]
	if exists {
		delete(e.eventHandlers, handlerID)
		return true
	}
	return false
}

// Resign 放弃主节点身份
func (e *electionImpl) Resign(ctx context.Context) error {
	if !e.isLeader || e.election == nil {
		return nil
	}

	e.logger.Info("Resigning leadership")

	// 添加重试逻辑，确保尽可能成功放弃leader身份
	var err error
	for i := 0; i < 3; i++ {
		err = e.election.Resign(ctx)
		if err == nil {
			e.isLeader = false
			e.logger.Info("Successfully resigned leadership")
			return nil
		}

		e.logger.Warn("Failed to resign, retrying", zap.Error(err), zap.Int("attempt", i+1))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// 短暂等待后重试
		}
	}

	if err != nil {
		e.logger.Error("Failed to resign after retries", zap.Error(err))
	}

	// 即使Resign失败，也强制设置isLeader为false
	e.isLeader = false
	return err
}

// GetLeader 获取当前主节点
func (e *electionImpl) GetLeader(ctx context.Context) (string, error) {
	if e.election == nil {
		return "", fmt.Errorf("election not started")
	}

	resp, err := e.election.Leader(ctx)
	if err != nil {
		if err == concurrency.ErrElectionNoLeader {
			return "", nil
		}
		return "", err
	}

	if len(resp.Kvs) == 0 {
		return "", nil
	}

	return string(resp.Kvs[0].Value), nil
}

// GetInfo 获取选举信息
func (e *electionImpl) GetInfo() ElectionInfo {
	leader, _ := e.GetLeader(context.Background())

	return ElectionInfo{
		Name:          e.name,
		Leader:        leader,
		Status:        e.status,
		LastEventTime: e.lastEvent,
		NodeID:        e.nodeID,
		CreateTime:    e.createTime.Format(time.RFC3339),
		IP:            e.ip,
		ProtocolPort:  e.protocolPort,
		CachePort:     e.cachePort,
	}
}

// IsLeader 检查当前节点是否为主节点
func (e *electionImpl) IsLeader() bool {
	return e.isLeader
}

// 内部停止方法
func (e *electionImpl) stopInternal() {
	// 关闭停止通道前先设置停止状态，防止竞态条件
	e.status = common.StatusStopped

	// 检查并关闭通道
	select {
	case <-e.stopCh:
		// 通道已关闭，无需操作
	default:
		close(e.stopCh)
	}

	// 如果是主节点，放弃主节点身份
	if e.isLeader && e.election != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		e.logger.Info("Force resigning leadership during stop")
		_ = e.Resign(ctx)
		e.isLeader = false
	}

	// 清理etcd中的选举数据
	if e.election != nil && e.leaderKey != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		electionKey := fmt.Sprintf("/distributed/elections/%s", e.name)
		e.logger.Info("清理选举数据", zap.String("key", electionKey))

		// 尝试删除选举键
		if _, err := e.etcdClient.Client.Delete(ctx, electionKey, clientv3.WithPrefix()); err != nil {
			e.logger.Warn("清理选举数据失败", zap.Error(err))
		} else {
			e.logger.Info("成功清理选举数据")
		}
	}

	// 关闭会话
	if e.session != nil {
		e.logger.Debug("Closing election session")
		e.session.Close()
		e.session = nil

		// 强制设置选举相关字段为nil，确保资源被释放
		e.election = nil
	}

	// 清理事件处理函数
	e.mutex.Lock()
	e.eventHandlers = make(map[string]ElectionEventHandler)
	e.mutex.Unlock()
}
