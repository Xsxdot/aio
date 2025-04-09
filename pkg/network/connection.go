package network

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// Connection 表示一个网络连接
type Connection struct {
	id        string
	conn      net.Conn
	protocol  Protocol
	handler   *MessageHandler
	closeChan chan struct{}
	state     atomic.Value // ConnectionState
	stats     *ConnectionStats
}

// NewConnection 创建新的连接
func NewConnection(conn net.Conn, protocol Protocol, handler *MessageHandler) *Connection {
	c := &Connection{
		id:        generateID(),
		conn:      conn,
		protocol:  protocol,
		handler:   handler,
		closeChan: make(chan struct{}),
		stats:     &ConnectionStats{},
	}

	// 设置初始状态
	c.state.Store(ConnectionState{
		Connected:  true,
		LastActive: time.Now(),
	})

	return c
}

// ID 获取连接ID
func (c *Connection) ID() string {
	return c.id
}

// RemoteAddr 获取远程地址
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr 获取本地地址
func (c *Connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// State 获取连接状态
func (c *Connection) State() ConnectionState {
	return c.state.Load().(ConnectionState)
}

// Stats 获取连接统计信息
func (c *Connection) Stats() *ConnectionStats {
	return c.stats
}

// Send 发送消息
func (c *Connection) Send(msg Message) error {
	select {
	case <-c.closeChan:
		return ErrConnectionClosed
	default:
		data := msg.ToBytes()
		if err := c.protocol.Write(c.conn, data); err != nil {
			return fmt.Errorf("write error: %w", err)
		}

		// 更新统计信息
		c.stats.WriteBytes += uint64(len(data))
		c.stats.WriteCount++
		c.stats.LastActive = time.Now()

		// 更新状态
		state := c.State()
		state.WriteBytes += uint64(len(data))
		state.WriteCount++
		state.LastActive = time.Now()
		c.state.Store(state)

		return nil
	}
}

// Close 关闭连接
func (c *Connection) Close() error {
	select {
	case <-c.closeChan:
		return nil
	default:
		close(c.closeChan)
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("close error: %w", err)
		}

		// 更新状态
		state := c.State()
		state.Connected = false
		c.state.Store(state)

		return nil
	}
}

// generateID 生成唯一的连接ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Dial 创建到远程服务器的连接
func Dial(address string, protocol Protocol) (*Connection, error) {
	// 建立TCP连接
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// 创建消息处理器
	handler := &MessageHandler{
		Handle: func(_ *Connection, data []byte) error {
			// 客户端连接默认不处理服务器消息
			// 由调用者手动处理接收到的消息
			return nil
		},
	}

	// 创建连接对象
	c := NewConnection(conn, protocol, handler)

	return c, nil
}
