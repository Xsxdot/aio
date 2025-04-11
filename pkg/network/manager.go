package network

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/common"

	"go.uber.org/zap"
)

// ConnectionManager 连接管理器
type ConnectionManager struct {
	connections sync.Map
	protocol    Protocol
	handler     *MessageHandler
	options     *Options
	server      net.Listener
	closeChan   chan struct{}
	wg          sync.WaitGroup
	host        string
	port        int
	log         *zap.Logger
	// 添加连接拦截器
	interceptor ConnectionInterceptor
	onlyClient  bool
}

// ConnectionInterceptor 连接拦截器接口
type ConnectionInterceptor interface {
	// Intercept 拦截连接，返回错误表示拒绝连接
	Intercept(conn net.Conn) error
}

// SetConnectionInterceptor 设置连接拦截器
func (m *ConnectionManager) SetConnectionInterceptor(interceptor ConnectionInterceptor) {
	m.interceptor = interceptor
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(protocol Protocol, handler *MessageHandler, options *Options) *ConnectionManager {
	if protocol == nil {
		panic("protocol cannot be nil")
	}
	if handler == nil {
		panic("handler cannot be nil")
	}
	if options == nil {
		options = DefaultOptions
	}

	return &ConnectionManager{
		protocol:   protocol,
		handler:    handler,
		options:    options,
		onlyClient: options.OnlyClient,
		closeChan:  make(chan struct{}),
		log:        common.GetLogger().ZapLogger().Named("connection-manager"),
	}
}

// StartServer 启动服务器
func (m *ConnectionManager) StartServer(host string, port int) error {
	m.host = host
	m.port = port
	addr := fmt.Sprintf("%s:%d", host, port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen error: %w", err)
	}

	m.server = listener
	m.wg.Add(1)
	go m.acceptLoop()

	return nil
}

// GetServerConfig 获取服务器配置
func (m *ConnectionManager) GetServerConfig() (string, int) {
	return m.host, m.port
}

// Connect 连接到服务器
func (m *ConnectionManager) Connect(addr string) (*Connection, error) {
	// 使用设置的选项中的读取超时
	var timeout time.Duration
	if m.options != nil && m.options.ReadTimeout > 0 {
		timeout = m.options.ReadTimeout
	} else {
		timeout = 30 * time.Second // 默认超时
	}

	// 使用超时设置建立连接
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	return m.handleNewConnection(conn, false)
}

// Send 发送消息
func (m *ConnectionManager) Send(connID string, msg Message) error {
	conn, ok := m.connections.Load(connID)
	if !ok {
		return ErrConnectionNotFound
	}

	connection := conn.(*Connection)

	return connection.Send(msg)
}

// Broadcast 广播消息
func (m *ConnectionManager) Broadcast(msg Message) error {
	var errs []error
	m.connections.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)
		if err := conn.Send(msg); err != nil {
			errs = append(errs, fmt.Errorf("conn %s: %w", key, err))
		}
		return true
	})

	if len(errs) > 0 {
		return fmt.Errorf("broadcast errors: %v", errs)
	}
	return nil
}

// Close 关闭连接管理器
func (m *ConnectionManager) Close() error {
	select {
	case <-m.closeChan:
		return nil
	default:
		close(m.closeChan)

		// 关闭服务器
		if m.server != nil {
			if err := m.server.Close(); err != nil {
				return fmt.Errorf("close server error: %w", err)
			}
		}

		// 关闭所有连接
		m.connections.Range(func(key, value interface{}) bool {
			conn := value.(*Connection)
			if err := conn.Close(); err != nil {
				m.log.Error("close connection error",
					zap.String("connectionID", key.(string)),
					zap.Error(err))
			}
			return true
		})

		// 等待所有goroutine结束
		m.wg.Wait()

		return nil
	}
}

// GetConnection 获取连接
func (m *ConnectionManager) GetConnection(connID string) (*Connection, bool) {
	conn, ok := m.connections.Load(connID)
	if !ok {
		return nil, false
	}
	return conn.(*Connection), true
}

// GetConnectionCount 获取连接数量
func (m *ConnectionManager) GetConnectionCount() int {
	var count int
	m.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// acceptLoop 接受连接循环
func (m *ConnectionManager) acceptLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.closeChan:
			return
		default:
			conn, err := m.server.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					time.Sleep(time.Millisecond * 100)
					continue
				}
				m.log.Error("accept error", zap.Error(err))
				return
			}

			if _, err := m.handleNewConnection(conn, true); err != nil {
				m.log.Error("handle new connection error", zap.Error(err))
				conn.Close()
			}
		}
	}
}

// handleNewConnection 处理新连接
func (m *ConnectionManager) handleNewConnection(conn net.Conn, fromClient bool) (*Connection, error) {
	// 检查连接数量限制
	if m.options.MaxConnections > 0 && m.GetConnectionCount() >= m.options.MaxConnections {
		conn.Close()
		return nil, ErrMaxConnections
	}

	// 设置连接选项
	if err := m.setupConnection(conn); err != nil {
		return nil, err
	}

	// 如果设置了拦截器，先进行拦截处理
	if m.interceptor != nil {
		if err := m.interceptor.Intercept(conn); err != nil {
			conn.Close()
			m.log.Warn("连接被拦截",
				zap.String("remoteAddr", conn.RemoteAddr().String()),
				zap.Error(err))
			return nil, fmt.Errorf("connection intercepted: %w", err)
		}
	}

	// 创建新连接
	c := NewConnection(conn, m.protocol, m.handler, fromClient)

	// 保存连接
	m.connections.Store(c.ID(), c)

	// 启动消息处理
	m.wg.Add(1)
	go m.handleConnection(c)

	m.log.Info("新连接已建立",
		zap.String("connectionID", c.ID()),
		zap.String("remoteAddr", conn.RemoteAddr().String()))

	return c, nil
}

// handleConnection 处理连接
func (m *ConnectionManager) handleConnection(conn *Connection) {
	defer func() {
		if r := recover(); r != nil {
			m.log.Error("connection panic", zap.Any("recover", r))
		}
		conn.Close()
		m.connections.Delete(conn.ID())
		// 通知连接关闭
		if m.handler.ConnectionClosed != nil {
			m.handler.ConnectionClosed(conn.ID())
		}
		m.wg.Done()
	}()

	// 启动心跳
	if m.handler.GetHeartbeat != nil && !conn.formClient {
		m.wg.Add(1)
		go m.heartbeatLoop(conn)
	}

	for {
		select {
		case <-m.closeChan:
			m.log.Info("连接管理器关闭，关闭连接", zap.String("connectionID", conn.ID()))
			return
		case <-conn.closeChan:
			m.log.Info("连接已关闭", zap.String("connectionID", conn.ID()))
			return
		default:
			// 设置读取超时
			//if m.options.ReadTimeout > 0 && !m.onlyClient {
			//	if err := conn.conn.SetReadDeadline(time.Now().Add(m.options.ReadTimeout)); err != nil {
			//		m.log.Error("设置读取超时失败", zap.Error(err))
			//	}
			//}

			data, err := conn.protocol.Read(conn.conn)
			if err != nil {
				if err != io.EOF {
					m.log.Error("读取错误",
						zap.Error(err),
						zap.String("connectionID", conn.ID()))
				} else {
					m.log.Info("连接已关闭 (EOF)", zap.String("connectionID", conn.ID()))
				}
				return
			}

			// 更新统计信息
			conn.stats.ReadBytes += uint64(len(data))
			conn.stats.ReadCount++
			conn.stats.LastActive = time.Now()

			// 处理消息
			if err := m.handler.Handle(conn, data); err != nil {
				m.log.Error("处理消息错误",
					zap.Error(err),
					zap.String("connectionID", conn.ID()))
			}
		}
	}
}

// heartbeatLoop 心跳循环
func (m *ConnectionManager) heartbeatLoop(conn *Connection) {
	defer m.wg.Done()

	// 确保心跳间隔大于0，如果小于等于0则使用默认值30秒
	heartbeatInterval := m.options.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.closeChan:
			return
		case <-conn.closeChan:
			return
		case <-ticker.C:
			if m.handler.GetHeartbeat != nil {
				heartbeat := m.handler.GetHeartbeat()
				if err := conn.Send(heartbeat); err != nil {
					m.log.Error("send heartbeat error",
						zap.Error(err),
						zap.String("connectionID", conn.ID()))
					return
				}
			}
		}
	}
}

// setupConnection 设置连接选项
func (m *ConnectionManager) setupConnection(conn net.Conn) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(m.options.EnableKeepAlive); err != nil {
			return fmt.Errorf("set keepalive error: %w", err)
		}
		if err := tcpConn.SetKeepAlivePeriod(m.options.IdleTimeout); err != nil {
			return fmt.Errorf("set keepalive period error: %w", err)
		}
	}

	// 设置初始读取超时
	//if m.options.ReadTimeout > 0 {
	//	if err := conn.SetReadDeadline(time.Now().Add(m.options.ReadTimeout)); err != nil {
	//		return fmt.Errorf("set read deadline error: %w", err)
	//	}
	//}

	// 不在初始化时设置永久写入超时
	// 将在每次写入时单独设置

	return nil
}

// SetOptions 更新连接管理器选项
func (m *ConnectionManager) SetOptions(options *Options) {
	if options != nil {
		m.options = options
	}
}
