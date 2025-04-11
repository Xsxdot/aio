package protocol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/auth"
	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/network"
	"github.com/xsxdot/aio/pkg/utils"

	"github.com/xsxdot/aio/internal/authmanager"
)

// ProtocolManager 协议管理器
type ProtocolManager struct {
	host     string
	port     int
	status   consts.ComponentStatus
	mode     string
	httpPort int

	log            *zap.Logger
	handleManager  *HandleManager
	systemHandle   *SystemHandle
	RequestManager *RequestManager

	protocol   *CustomProtocol
	networkMgr *network.ConnectionManager
	options    *network.Options
	onlyClient bool
	// 客户端认证信息
	clientID     string
	clientSecret string
	token        *auth.Token
}

func (m *ProtocolManager) RegisterMetadata() (bool, int, map[string]string) {
	return true, m.port, map[string]string{
		"httpPort": strconv.Itoa(m.httpPort),
		"mode":     m.mode,
	}
}

func (m *ProtocolManager) Name() string {
	return consts.ComponentProtocolManager
}

func (m *ProtocolManager) Status() consts.ComponentStatus {
	return m.status
}

func (m *ProtocolManager) Init(cfg *config.BaseConfig, body []byte) error {
	protocolConfig := cfg.Protocol
	m.options = &network.Options{
		ReadTimeout:       utils.ParseDuration(protocolConfig.ReadTimeout, 15*time.Second),
		WriteTimeout:      utils.ParseDuration(protocolConfig.WriteTimeout, 15*time.Second),
		IdleTimeout:       utils.ParseDuration(protocolConfig.IdleTimeout, 60*time.Second),
		MaxConnections:    protocolConfig.MaxConnections,
		BufferSize:        protocolConfig.BufferSize,
		EnableKeepAlive:   protocolConfig.EnableKeepAlive,
		HeartbeatInterval: utils.ParseDuration(protocolConfig.HeartbeatTimeout, 30*time.Second),
	}

	m.host = cfg.Network.BindIP
	m.port = protocolConfig.Port
	m.status = consts.StatusInitialized
	m.mode = cfg.System.Mode
	m.httpPort = cfg.Network.HttpPort

	m.RequestManager = NewRequestManager(m.options.ReadTimeout, m)

	return nil
}

func (m *ProtocolManager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	return m.Start(ctx)
}

// 保护消息ID生成的互斥锁
var msgIDMutex sync.Mutex

// NewServer 创建协议管理器
func NewServer(authManager *authmanager.AuthManager) *ProtocolManager {
	systemHandle := NewSystemHandle(authManager)
	manager := NewHandleManager()

	if authManager != nil {
		manager.RegisterBaseHandle(systemHandle.ValidAuth)
		manager.RegisterHandle(ServiceTypeSystem, MsgTypeAuth, systemHandle.Auth)
	}
	manager.RegisterHandle(ServiceTypeSystem, MsgTypeHeartbeat, systemHandle.Heartbeat)

	m := &ProtocolManager{
		handleManager: manager,
		systemHandle:  systemHandle,
		protocol:      NewCustomProtocol(),
		log:           common.GetLogger().GetZapLogger("aio-protocol-server"),
		onlyClient:    false,
	}
	return m
}

func (m *ProtocolManager) RegisterBaseHandle(handles ...MessageHandler) {
	m.handleManager.RegisterBaseHandle(handles...)
}

func (m *ProtocolManager) RegisterHandle(svcType ServiceType, msgType MessageType, handler MessageHandler) {
	m.handleManager.RegisterHandle(svcType, msgType, handler)
}

// Start 启动服务器
func (m *ProtocolManager) Start(ctx context.Context) error {

	// 创建自定义网络处理器，包含连接管理
	networkHandler := m.createNetworkHandler()

	m.networkMgr = network.NewConnectionManager(m.protocol, networkHandler, m.options)

	fmt.Printf("Starting server on %s:%d\n", m.host, m.port)

	err := m.networkMgr.StartServer(m.host, m.port)
	if err != nil {
		return err
	}
	m.status = consts.StatusRunning
	return err
}

func (m *ProtocolManager) createNetworkHandler() *network.MessageHandler {
	handler := &network.MessageHandler{
		Handle: func(conn *network.Connection, data []byte) error {
			// 解析消息以便进行权限验证
			msg, err := m.protocol.ParseMessage(data)
			if err != nil {
				return fmt.Errorf("parse message failed: %w", err)
			}

			msg.header.ConnID = conn.ID()

			// 处理消息，传入连接对象
			response, err := m.handleManager.ProcessMsg(conn, msg)
			if err != nil {
				return err
			}

			if response != nil {
				// 发送消息
				err = conn.Send(response)
				if err != nil {
					m.log.Error("Failed to send response", zap.Error(err))
				}
			}

			return nil
		},
		GetHeartbeat: func() network.Message {
			// 创建简单的心跳消息
			return HeartbeatMsg
		},
		ConnectionClosed: func(connID string) {
			m.systemHandle.removeToken(connID)
		},
	}
	return handler
}

// GetServerConfig 获取服务器配置
func (m *ProtocolManager) GetServerConfig() (string, int) {
	return m.host, m.port
}

// Stop 停止服务器
func (m *ProtocolManager) Stop(ctx context.Context) error {
	if m.networkMgr != nil {
		return m.networkMgr.Close()
	}
	m.status = consts.StatusStopped
	return nil
}

// Connect 连接到服务器
func (m *ProtocolManager) Connect(addr string, options *network.Options) (*network.Connection, error) {
	if options == nil {
		options = network.DefaultOptions
	}

	if m.networkMgr == nil {
		// 创建自定义网络处理器
		networkHandler := m.createNetworkHandler()
		m.networkMgr = network.NewConnectionManager(m.protocol, networkHandler, options)

	} else {
		// 更新网络管理器的选项
		m.networkMgr.SetOptions(options)
	}

	// 调用网络管理器连接到服务器
	conn, err := m.networkMgr.Connect(addr)
	if err != nil {
		return nil, err
	}

	// 如果设置了客户端ID和Secret，进行认证
	if m.clientID != "" && m.clientSecret != "" {
		err := m.authenticate(conn.ID())
		if err != nil {
			return conn, fmt.Errorf("认证失败: %w", err)
		}

		// 启动令牌刷新协程
		go m.refreshTokenRoutine(conn)
	}

	return conn, nil
}

// authenticate 发送认证请求并获取令牌
func (m *ProtocolManager) authenticate(connID string) error {
	// 创建认证请求
	authReq := &authmanager.ClientAuthRequest{
		ClientID:     m.clientID,
		ClientSecret: m.clientSecret,
	}

	// 创建认证消息，直接传递认证请求对象
	authMsg := NewMessage(MsgTypeAuth, ServiceTypeSystem, connID, authReq)

	// 发送认证请求并等待响应
	var token auth.Token
	err := m.RequestManager.Request(authMsg, &token)
	if err != nil {
		return err
	}
	// 存储令牌
	m.token = &token

	return nil
}

// refreshTokenRoutine 定期刷新令牌的协程
func (m *ProtocolManager) refreshTokenRoutine(conn *network.Connection) {
	if m.token == nil {
		return
	}

	// 计算刷新间隔，通常为过期时间的80%
	refreshInterval := time.Duration(float64(m.token.ExpiresIn) * 0.8 * float64(time.Second))

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查连接是否关闭
			if !conn.State().Connected {
				return
			}

			// 执行认证并获取新令牌
			err := m.authenticate(conn.ID())
			if err != nil {
				m.log.Error("令牌刷新失败", zap.Error(err))
				continue
			}

			// 更新刷新间隔
			refreshInterval = time.Duration(float64(m.token.ExpiresIn) * 0.8 * float64(time.Second))
			ticker.Reset(refreshInterval)
		}
	}
}

func (m *ProtocolManager) GetConnection(connId string) (*network.Connection, bool) {
	return m.networkMgr.GetConnection(connId)
}

// generateMsgID 生成一个唯一的消息ID
// 使用UUID v4算法确保唯一性
func generateMsgID() string {
	msgIDMutex.Lock()
	defer msgIDMutex.Unlock()

	// 使用加密安全的随机数生成器生成16字节的随机数据 (UUID v4)
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		// 如果随机数生成器失败，使用基于时间的回退策略
		now := time.Now().UnixNano()
		// 将时间戳转换为字节
		byteTime := []byte(fmt.Sprintf("%016x", now))
		// 复制到UUID中
		copy(uuid, byteTime)
	}

	// 设置UUID版本 (4) 和变体
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // 版本 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // 变体 RFC4122

	// 将UUID转换为16字节的十六进制字符串
	id := hex.EncodeToString(uuid)[:16]
	return id
}

// SendMessage 发送消息
func (m *ProtocolManager) SendMessage(connID string, msgType MessageType, svcType ServiceType, payload interface{}) error {
	msg := NewMessage(msgType, svcType, connID, payload)
	err := m.networkMgr.Send(connID, msg)

	log.Printf("[管理器] 发送消息内容: %s", string(msg.payload))

	return err
}

func (m *ProtocolManager) Send(msg *CustomMessage) error {
	err := m.networkMgr.Send(msg.Header().ConnID, msg)

	log.Printf("[管理器] 发送消息内容: %s", string(msg.payload))

	return err
}

// BroadcastMessage 广播消息
func (m *ProtocolManager) BroadcastMessage(msgType MessageType, svcType ServiceType, payload interface{}) error {
	msg := NewMessage(msgType, svcType, "", payload)
	err := m.networkMgr.Broadcast(msg)
	return err
}

// GetConnectionCount 获取连接数量
func (m *ProtocolManager) GetConnectionCount() int {
	if m.networkMgr == nil {
		return 0
	}
	count := m.networkMgr.GetConnectionCount()

	return count
}

// CloseConnection 关闭指定连接
func (m *ProtocolManager) CloseConnection(connID string) error {
	// 检查网络管理器是否为nil
	if m.networkMgr == nil {
		return fmt.Errorf("network manager not initialized")
	}

	conn, ok := m.networkMgr.GetConnection(connID)
	if !ok {
		return network.ErrConnectionNotFound
	}

	return conn.Close()
}

// DefaultConfig 返回组件的默认配置
func (m *ProtocolManager) DefaultConfig(config *config.BaseConfig) interface{} {
	return nil
}

// GetClientConfig 实现Component接口，返回客户端配置
func (m *ProtocolManager) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}

// NewClient 创建只作为客户端使用的协议管理器
func NewClient() *ProtocolManager {
	m := &ProtocolManager{
		protocol:      NewCustomProtocol(),
		onlyClient:    true, // 设置为只作为客户端使用
		handleManager: NewHandleManager(),
		log:           common.GetLogger().GetZapLogger("aio-protocol-client"),
	}
	m.RequestManager = NewRequestManager(5*time.Second, m)

	return m
}

// ClientOptions 客户端选项
type ClientOptions struct {
	// 认证相关
	EnableAuth   bool   // 是否启用认证
	ClientID     string // 客户端ID
	ClientSecret string // 客户端密钥
	// 网络相关
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxConnections    int
	BufferSize        int
	EnableKeepAlive   bool
	HeartbeatInterval time.Duration
}

// DefaultClientOptions 默认客户端选项
var DefaultClientOptions = &ClientOptions{
	ReadTimeout:       15 * time.Second,
	WriteTimeout:      15 * time.Second,
	IdleTimeout:       60 * time.Second,
	MaxConnections:    100,
	BufferSize:        4096,
	EnableKeepAlive:   true,
	HeartbeatInterval: 30 * time.Second,
}

// NewClientWithOptions 使用选项创建只作为客户端的协议管理器
func NewClientWithOptions(options *ClientOptions) *ProtocolManager {
	if options == nil {
		options = DefaultClientOptions
	}

	m := NewClient()

	// 设置网络选项
	m.options = &network.Options{
		ReadTimeout:       options.ReadTimeout,
		WriteTimeout:      options.WriteTimeout,
		IdleTimeout:       options.IdleTimeout,
		MaxConnections:    options.MaxConnections,
		BufferSize:        options.BufferSize,
		EnableKeepAlive:   options.EnableKeepAlive,
		HeartbeatInterval: options.HeartbeatInterval,
	}

	// 设置认证选项
	if options.EnableAuth {
		m.clientID = options.ClientID
		m.clientSecret = options.ClientSecret
	}

	// 注册系统消息处理器
	m.handleManager.RegisterHandle(ServiceTypeResponse, MsgTypeResponseSuccess, m.RequestManager.HandleResponse)
	m.handleManager.RegisterHandle(ServiceTypeResponse, MsgTypeResponseFail, m.RequestManager.HandleResponse)

	return m
}
