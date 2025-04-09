package protocol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/auth"
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

	protocol   *CustomProtocol
	networkMgr *network.ConnectionManager
	serviceMap map[ServiceType]string
	// 最后记录的连接数
	lastConnCount int
	// 添加认证管理器
	authManager *authmanager.AuthManager
	// 是否启用连接认证
	authEnabled bool
	// 连接令牌映射
	connTokens map[string]string
	// 临时消息处理器
	tempHandlers map[string]func(*CustomMessage)
	// 临时处理器互斥锁
	tempHandlerMutex sync.Mutex
	options          *network.Options
	onlyClient       bool
	// 客户端认证信息
	clientID     string
	clientSecret string
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

	// 启用认证
	if protocolConfig.EnableAuth && m.authManager != nil {
		m.authEnabled = true
	}
	m.host = cfg.Network.BindIP
	m.port = protocolConfig.Port
	m.status = consts.StatusInitialized
	m.mode = cfg.System.Mode
	m.httpPort = cfg.Network.HttpPort

	return nil
}

func (m *ProtocolManager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	return m.Start(ctx)
}

// ProtocolManagerOptions 协议管理器选项
type ProtocolManagerOptions struct {
}

// ConnectionInterceptor 连接拦截器接口
type ConnectionInterceptor interface {
	// Intercept 拦截连接
	Intercept(conn net.Conn) error
}

// AuthInterceptor 认证拦截器
type AuthInterceptor struct {
	authManager *authmanager.AuthManager
	manager     *ProtocolManager
}

// Intercept 实现连接拦截接口
func (a *AuthInterceptor) Intercept(conn net.Conn) error {
	// 如果未启用认证，直接通过
	if !a.manager.authEnabled || a.manager.authManager == nil {
		return nil
	}

	// 由于网络框架的设计，我们需要适当处理连接对象
	// 设置超时以等待认证请求
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // 重置超时

	// 读取认证消息
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read auth message failed: %w", err)
	}

	// 解析认证消息
	msg, err := a.manager.protocol.ParseMessage(buf[:n])
	if err != nil {
		return fmt.Errorf("parse auth message failed: %w", err)
	}

	// 验证消息类型
	if msg.Header().MessageType != MsgTypeAuth {
		return fmt.Errorf("expected auth message, got %d", msg.Header().MessageType)
	}

	// 解析认证请求
	var authReq authmanager.ClientAuthRequest
	err = json.Unmarshal(msg.Payload(), &authReq)
	if err != nil {
		return fmt.Errorf("unmarshal auth request failed: %w", err)
	}

	// 进行认证
	token, err := a.authManager.AuthenticateClient(authReq)
	if err != nil {
		return fmt.Errorf("authenticate client failed: %w", err)
	}

	// 将令牌与连接关联 - 使用连接对象的特性
	// 这里需要使用自定义方式存储令牌数据
	// 如果网络框架支持，也可以设置连接元数据
	a.manager.connTokens[msg.Header().ConnID] = token.AccessToken

	// 发送认证成功响应
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token failed: %w", err)
	}

	// 发送响应消息
	err = a.manager.SendMessage(msg.Header().ConnID, MsgTypeAuthResponse, ServiceTypeSystem, tokenBytes)
	if err != nil {
		return fmt.Errorf("send auth response failed: %w", err)
	}

	return nil
}

// 保护消息ID生成的互斥锁
var msgIDMutex sync.Mutex

// NewServer 创建协议管理器
func NewServer(authManager *authmanager.AuthManager) *ProtocolManager {
	m := &ProtocolManager{
		protocol:      NewCustomProtocol(),
		serviceMap:    make(map[ServiceType]string),
		lastConnCount: 0,
		authEnabled:   false,
		connTokens:    make(map[string]string),
		tempHandlers:  make(map[string]func(*CustomMessage)),
		authManager:   authManager,
	}
	// 注册心跳处理器
	RegisterHeartbeatHandlers(m)
	return m
}

// RegisterService 注册服务
func (m *ProtocolManager) RegisterService(svcType ServiceType, name string, handler *ServiceHandler) {
	m.protocol.RegisterService(svcType, handler)
	m.serviceMap[svcType] = name

}

// createNetworkHandler 创建网络处理器
func (m *ProtocolManager) createNetworkHandler() *network.MessageHandler {
	// 注册认证消息处理函数
	if m.authEnabled && m.authManager != nil {
		// 创建或获取系统服务处理器
		sysHandler, found := m.protocol.serviceHandlers[ServiceTypeSystem]
		if !found {
			sysHandler = NewServiceHandler()
			m.protocol.RegisterService(ServiceTypeSystem, sysHandler)
		}

		// 注册认证消息处理函数
		sysHandler.RegisterHandler(MsgTypeAuth, func(connID string, msg *CustomMessage) error {
			// 解析认证请求
			var authReq authmanager.ClientAuthRequest
			err := json.Unmarshal(msg.Payload(), &authReq)
			if err != nil {
				return fmt.Errorf("unmarshal auth request failed: %w", err)
			}

			// 进行认证
			token, err := m.authManager.AuthenticateClient(authReq)
			if err != nil {
				// 认证失败，发送失败响应
				errorResp := map[string]string{"error": err.Error()}
				respData, _ := json.Marshal(errorResp)
				m.SendMessage(connID, MsgTypeAuthResponse, ServiceTypeSystem, respData)
				return fmt.Errorf("authenticate client failed: %w", err)
			}

			// 认证成功，保存令牌
			m.connTokens[connID] = token.AccessToken

			// 发送认证成功响应
			tokenBytes, _ := json.Marshal(token)
			return m.SendMessage(connID, MsgTypeAuthResponse, ServiceTypeSystem, tokenBytes)
		})

		// 注册刷新token处理函数
		sysHandler.RegisterHandler(MsgTypeRefreshToken, func(connID string, msg *CustomMessage) error {
			// 解析刷新token请求
			var refreshReq struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
				RefreshToken string `json:"refresh_token"`
			}

			err := json.Unmarshal(msg.Payload(), &refreshReq)
			if err != nil {
				errorResp := map[string]string{"error": "无效的刷新请求格式"}
				respData, _ := json.Marshal(errorResp)
				m.SendMessage(connID, MsgTypeRefreshTokenResponse, ServiceTypeSystem, respData)
				return fmt.Errorf("unmarshal refresh token request failed: %w", err)
			}

			// 创建认证请求
			authReq := authmanager.ClientAuthRequest{
				ClientID:     refreshReq.ClientID,
				ClientSecret: refreshReq.ClientSecret,
			}

			// 进行认证刷新
			token, err := m.authManager.AuthenticateClient(authReq)
			if err != nil {
				// 刷新失败，发送失败响应
				errorResp := map[string]string{"error": err.Error()}
				respData, _ := json.Marshal(errorResp)
				m.SendMessage(connID, MsgTypeRefreshTokenResponse, ServiceTypeSystem, respData)
				return fmt.Errorf("refresh token failed: %w", err)
			}

			// 刷新成功，保存新令牌
			m.connTokens[connID] = token.AccessToken

			// 发送刷新成功响应
			tokenBytes, _ := json.Marshal(token)
			return m.SendMessage(connID, MsgTypeRefreshTokenResponse, ServiceTypeSystem, tokenBytes)
		})
	}

	handler := &network.MessageHandler{
		Handle: func(conn *network.Connection, data []byte) error {
			// 解析消息以便进行权限验证
			msg, err := m.protocol.ParseMessage(data)
			if err != nil {
				return fmt.Errorf("parse message failed: %w", err)
			}

			// 1. 更新连接ID
			msg.header.ConnID = conn.ID()

			// 2. 检查是否有临时处理器需要处理此消息
			m.tempHandlerMutex.Lock()
			for _, handler := range m.tempHandlers {
				if handler != nil {
					go handler(msg)
				}
			}
			m.tempHandlerMutex.Unlock()

			// 3. 如果启用了认证，并且不是认证消息，执行权限验证
			if m.authEnabled && m.authManager != nil {
				// 系统消息和认证消息不需要权限验证
				if msg.Header().ServiceType != ServiceTypeSystem ||
					(msg.Header().MessageType != MsgTypeAuth && msg.Header().MessageType != MsgTypeAuthResponse) {

					// 检查连接是否已认证
					if _, ok := m.connTokens[conn.ID()]; !ok {
						// 未认证的连接只能发送认证消息
						if msg.Header().MessageType != MsgTypeAuth {
							return fmt.Errorf("connection not authenticated")
						}
					} else {
						// 已认证的连接，验证操作权限
						resource := fmt.Sprintf("service.%s", msg.Header().ServiceType)
						action := fmt.Sprintf("message.%s", msg.Header().MessageType)

						// 验证权限
						verifyResp, err := m.VerifyPermission(conn.ID(), resource, action)
						if err != nil {
							return fmt.Errorf("verify permission failed: %w", err)
						}

						if !verifyResp.Allowed {
							return fmt.Errorf("permission denied: %s", verifyResp.Reason)
						}
					}
				}
			}

			// 处理消息，传入连接对象
			err = m.protocol.ProcessMessage(conn, data)

			return err
		},
		GetHeartbeat: func() network.Message {
			// 创建简单的心跳消息
			return NewMessage(MsgTypeHeartbeat, ServiceTypeSystem, "", generateMsgID(), CreateHeartbeatMessage())
		},
		ConnectionClosed: func(connID string) {
			// 移除连接令牌
			delete(m.connTokens, connID)
		},
	}

	return handler
}

// Start 启动服务器
func (m *ProtocolManager) Start(ctx context.Context) error {

	// 创建自定义网络处理器，包含连接管理
	networkHandler := m.createNetworkHandler()

	m.networkMgr = network.NewConnectionManager(m.protocol, networkHandler, m.options)

	// 启用认证时，我们将在消息处理逻辑中集成认证机制，而不是在连接建立时
	if m.authEnabled && m.authManager != nil {
		log.Printf("认证功能已启用，客户端需要发送认证消息")
	}

	fmt.Printf("Starting server on %s:%d\n", m.host, m.port)

	err := m.networkMgr.StartServer(m.host, m.port)
	if err != nil {
		return err
	}
	m.status = consts.StatusRunning
	return err
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

	return conn, nil
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
func (m *ProtocolManager) SendMessage(connID string, msgType MessageType, svcType ServiceType, payload []byte) error {
	// 生成消息ID
	msgID := generateMsgID()

	msg := NewMessage(msgType, svcType, connID, msgID, payload)
	err := m.networkMgr.Send(connID, msg)

	log.Printf("[管理器] 发送消息内容: %s", string(payload))

	return err
}

// BroadcastMessage 广播消息
func (m *ProtocolManager) BroadcastMessage(msgType MessageType, svcType ServiceType, payload []byte) error {
	// 生成消息ID (广播消息共用一个ID)
	msgID := generateMsgID()

	// 创建消息，连接ID为空
	msg := NewMessage(msgType, svcType, "", msgID, payload)
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

// EnableAuth 启用认证
func (m *ProtocolManager) EnableAuth(authManager *authmanager.AuthManager) {
	if authManager != nil {
		m.authEnabled = true
		m.authManager = authManager

		// 初始化认证相关结构
		m.connTokens = make(map[string]string)

		// 记录日志
		log.Printf("已启用认证功能")

		// 注册认证消息处理函数
		// 这里重新调用一次createNetworkHandler来确保认证消息处理器被注册
		m.createNetworkHandler()
	}
}

// 客户端认证流程说明:
//
// 1. 客户端连接流程:
//    - 客户端通过Connect方法连接到服务器
//    - 连接建立后，客户端需要发送认证消息(MsgTypeAuth)
//    - 服务器验证认证后，返回认证响应(MsgTypeAuthResponse)
//    - 认证成功后，客户端收到JWT令牌，可以继续发送其他消息
//
// 2. 服务端认证流程:
//    - 服务器收到消息后，首先检查是否是认证消息
//    - 如果是认证消息，则进行认证验证
//    - 如果不是认证消息，则检查连接是否已认证
//    - 如果连接未认证，则拒绝非认证消息
//    - 如果连接已认证，则验证客户端权限
//
// 3. 权限验证流程:
//    - 根据消息类型和服务类型确定资源和操作
//    - 调用认证管理器验证客户端是否有权限执行该操作
//    - 如果有权限，则处理消息
//    - 如果没有权限，则拒绝消息
//
// 4. 角色和权限:
//    - 客户端在创建时分配角色
//    - 角色决定了客户端可以执行的操作
//    - 权限定义为资源和操作的组合
//    - 资源格式为"service.{ServiceType}"
//    - 操作格式为"message.{MessageType}"

// AuthenticateClient 客户端认证方法
func (m *ProtocolManager) AuthenticateClient(connID string, req *authmanager.ClientAuthRequest) (*auth.Token, error) {
	// 检查认证是否启用
	if !m.authEnabled || m.authManager == nil {
		return nil, fmt.Errorf("authentication not enabled")
	}

	// 调用认证管理器进行认证
	token, err := m.authManager.AuthenticateClient(*req)
	if err != nil {
		return nil, err
	}

	// 存储令牌
	m.connTokens[connID] = token.AccessToken

	return token, nil
}

// VerifyPermission 验证客户端权限
func (m *ProtocolManager) VerifyPermission(connID string, resource string, action string) (*auth.VerifyResponse, error) {
	// 检查认证是否启用
	if !m.authEnabled || m.authManager == nil {
		// 如果未启用认证，默认允许所有操作
		return &auth.VerifyResponse{Allowed: true}, nil
	}

	// 获取连接令牌
	token, ok := m.connTokens[connID]
	if !ok {
		return &auth.VerifyResponse{Allowed: false, Reason: "no auth token found"}, nil
	}

	// 调用认证管理器验证权限
	return m.authManager.VerifyPermission(token, resource, action)
}

// AuthenticateConnection 对已建立的连接进行认证
func (m *ProtocolManager) AuthenticateConnection(conn *network.Connection, clientID, clientSecret string) (*auth.Token, error) {
	if !m.authEnabled || m.authManager == nil {
		return nil, fmt.Errorf("authentication not enabled")
	}

	// 创建认证请求
	authReq := authmanager.ClientAuthRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	// 序列化认证请求
	authData, err := json.Marshal(authReq)
	if err != nil {
		return nil, fmt.Errorf("marshal auth request failed: %w", err)
	}

	// 创建认证消息
	authMsg := NewMessage(MsgTypeAuth, ServiceTypeSystem, conn.ID(), generateMsgID(), authData)

	// 创建接收通道
	respChan := make(chan *auth.Token, 1)
	errChan := make(chan error, 1)

	// 注册临时处理器等待认证响应
	handlerID := fmt.Sprintf("auth_response_%s", conn.ID())

	m.RegisterTemporaryHandler(handlerID, func(msg *CustomMessage) {
		if msg.Header().MessageType == MsgTypeAuthResponse && msg.Header().ServiceType == ServiceTypeSystem {
			// 检查是否是错误响应
			var errorResp map[string]string
			if err := json.Unmarshal(msg.Payload(), &errorResp); err == nil {
				if errMsg, ok := errorResp["error"]; ok {
					errChan <- fmt.Errorf("authentication failed: %s", errMsg)
					return
				}
			}

			// 解析为正常的令牌响应
			var token auth.Token
			err := json.Unmarshal(msg.Payload(), &token)
			if err != nil {
				errChan <- fmt.Errorf("unmarshal auth response failed: %w", err)
				return
			}

			// 存储令牌
			m.connTokens[conn.ID()] = token.AccessToken

			respChan <- &token
		}
	})

	// 确保在函数返回时取消注册处理器
	defer m.UnregisterTemporaryHandler(handlerID)

	// 发送认证请求
	err = conn.Send(authMsg)
	if err != nil {
		return nil, fmt.Errorf("send auth request failed: %w", err)
	}

	// 等待认证响应或超时
	select {
	case token := <-respChan:
		return token, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("authentication timed out")
	}
}

// RefreshToken 刷新认证令牌
func (m *ProtocolManager) RefreshToken(conn *network.Connection) (*auth.Token, error) {
	// 检查是否启用认证
	if !m.authEnabled {
		return nil, fmt.Errorf("认证未启用")
	}

	// 检查客户端认证信息是否完整
	if m.clientID == "" || m.clientSecret == "" {
		return nil, fmt.Errorf("客户端认证信息不完整")
	}

	// 创建刷新令牌请求
	refreshReq := struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}{
		ClientID:     m.clientID,
		ClientSecret: m.clientSecret,
		RefreshToken: m.connTokens[conn.ID()], // 使用当前连接的令牌
	}

	// 序列化刷新令牌请求
	refreshData, err := json.Marshal(refreshReq)
	if err != nil {
		return nil, fmt.Errorf("序列化刷新令牌请求失败: %w", err)
	}

	// 创建刷新令牌消息
	refreshMsg := NewMessage(MsgTypeRefreshToken, ServiceTypeSystem, conn.ID(), generateMsgID(), refreshData)

	// 创建接收通道
	respChan := make(chan *auth.Token, 1)
	errChan := make(chan error, 1)

	// 注册临时处理器等待刷新令牌响应
	handlerID := fmt.Sprintf("refresh_token_response_%s", conn.ID())

	m.RegisterTemporaryHandler(handlerID, func(msg *CustomMessage) {
		if msg.Header().MessageType == MsgTypeRefreshTokenResponse && msg.Header().ServiceType == ServiceTypeSystem {
			// 检查是否是错误响应
			var errorResp map[string]string
			if err := json.Unmarshal(msg.Payload(), &errorResp); err == nil {
				if errMsg, ok := errorResp["error"]; ok {
					errChan <- fmt.Errorf("刷新令牌失败: %s", errMsg)
					return
				}
			}

			// 解析为正常的令牌响应
			var token auth.Token
			err := json.Unmarshal(msg.Payload(), &token)
			if err != nil {
				errChan <- fmt.Errorf("解析刷新令牌响应失败: %w", err)
				return
			}

			// 更新存储的令牌
			m.connTokens[conn.ID()] = token.AccessToken

			respChan <- &token
		}
	})

	// 确保在函数返回时取消注册处理器
	defer m.UnregisterTemporaryHandler(handlerID)

	// 发送刷新令牌请求
	err = conn.Send(refreshMsg)
	if err != nil {
		return nil, fmt.Errorf("发送刷新令牌请求失败: %w", err)
	}

	// 等待刷新令牌响应或超时
	select {
	case token := <-respChan:
		return token, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("刷新令牌超时")
	}
}

// ConnectWithAuth 连接到服务器并自动进行认证
func (m *ProtocolManager) ConnectWithAuth(addr string, options *network.Options) (*network.Connection, *auth.Token, error) {
	// 检查是否启用认证
	if !m.authEnabled {
		// 如果未启用认证，则直接连接
		conn, err := m.Connect(addr, options)
		return conn, nil, err
	}

	// 检查客户端认证信息是否完整
	if m.clientID == "" || m.clientSecret == "" {
		return nil, nil, fmt.Errorf("客户端认证信息不完整，需要提供ClientID和ClientSecret")
	}

	// 先建立连接
	conn, err := m.Connect(addr, options)
	if err != nil {
		return nil, nil, fmt.Errorf("连接服务器失败: %w", err)
	}

	// 构造认证请求
	authReq := authmanager.ClientAuthRequest{
		ClientID:     m.clientID,
		ClientSecret: m.clientSecret,
	}

	// 序列化认证请求
	authData, err := json.Marshal(authReq)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("序列化认证请求失败: %w", err)
	}

	// 创建认证消息
	authMsg := NewMessage(MsgTypeAuth, ServiceTypeSystem, conn.ID(), generateMsgID(), authData)

	// 创建接收通道
	respChan := make(chan *auth.Token, 1)
	errChan := make(chan error, 1)

	// 注册临时处理器等待认证响应
	handlerID := fmt.Sprintf("auth_response_%s", conn.ID())

	m.RegisterTemporaryHandler(handlerID, func(msg *CustomMessage) {
		if msg.Header().MessageType == MsgTypeAuthResponse && msg.Header().ServiceType == ServiceTypeSystem {
			// 检查是否是错误响应
			var errorResp map[string]string
			if err := json.Unmarshal(msg.Payload(), &errorResp); err == nil {
				if errMsg, ok := errorResp["error"]; ok {
					errChan <- fmt.Errorf("认证失败: %s", errMsg)
					return
				}
			}

			// 解析为正常的令牌响应
			var token auth.Token
			err := json.Unmarshal(msg.Payload(), &token)
			if err != nil {
				errChan <- fmt.Errorf("解析认证响应失败: %w", err)
				return
			}

			// 存储令牌
			m.connTokens[conn.ID()] = token.AccessToken

			respChan <- &token
		}
	})

	// 确保在函数返回时取消注册处理器
	defer m.UnregisterTemporaryHandler(handlerID)

	// 发送认证请求
	err = conn.Send(authMsg)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("发送认证请求失败: %w", err)
	}

	// 等待认证响应或超时
	select {
	case token := <-respChan:
		return conn, token, nil
	case err := <-errChan:
		conn.Close()
		return nil, nil, err
	case <-time.After(10 * time.Second):
		conn.Close()
		return nil, nil, fmt.Errorf("认证超时")
	}
}

// 使用JWT令牌创建客户端
func (m *ProtocolManager) CreateServiceClient(serviceName, description string, roles []string) (string, string, error) {
	if !m.authEnabled || m.authManager == nil {
		return "", "", fmt.Errorf("authentication not enabled")
	}

	// 创建服务客户端
	clientID, clientSecret, err := m.authManager.CreateServiceClient(serviceName, roles, description)
	if err != nil {
		return "", "", fmt.Errorf("create service client failed: %w", err)
	}

	return clientID, clientSecret, nil
}

// 使用客户端ID和密钥获取JWT令牌
func (m *ProtocolManager) GetClientToken(clientID, clientSecret string) (*auth.Token, error) {
	if !m.authEnabled || m.authManager == nil {
		return nil, fmt.Errorf("authentication not enabled")
	}

	// 创建认证请求
	authReq := authmanager.ClientAuthRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	// 进行认证
	return m.authManager.AuthenticateClient(authReq)
}

// RegisterTemporaryHandler 注册临时消息处理器
func (m *ProtocolManager) RegisterTemporaryHandler(handlerID string, handler func(*CustomMessage)) {
	m.tempHandlerMutex.Lock()
	defer m.tempHandlerMutex.Unlock()
	m.tempHandlers[handlerID] = handler
}

// UnregisterTemporaryHandler 注销临时消息处理器
func (m *ProtocolManager) UnregisterTemporaryHandler(handlerID string) {
	m.tempHandlerMutex.Lock()
	defer m.tempHandlerMutex.Unlock()
	delete(m.tempHandlers, handlerID)
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
		serviceMap:    make(map[ServiceType]string),
		lastConnCount: 0,
		authEnabled:   false,
		connTokens:    make(map[string]string),
		tempHandlers:  make(map[string]func(*CustomMessage)),
		onlyClient:    true, // 设置为只作为客户端使用
	}
	// 注册心跳处理器
	RegisterHeartbeatHandlers(m)
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
		m.authEnabled = true
		m.clientID = options.ClientID
		m.clientSecret = options.ClientSecret
	}

	return m
}
