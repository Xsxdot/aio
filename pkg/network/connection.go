package network

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"
)

// Connection 表示一个网络连接
type Connection struct {
	id         string
	conn       net.Conn
	protocol   Protocol
	handler    *MessageHandler
	closeChan  chan struct{}
	state      atomic.Value // ConnectionState
	stats      *ConnectionStats
	formClient bool
	log        *zap.Logger
}

// NewConnection 创建新的连接
func NewConnection(conn net.Conn, protocol Protocol, handler *MessageHandler, formClient bool) *Connection {
	c := &Connection{
		id:         generateID(),
		conn:       conn,
		protocol:   protocol,
		handler:    handler,
		closeChan:  make(chan struct{}),
		stats:      &ConnectionStats{},
		formClient: formClient,
		log:        common.GetLogger().ZapLogger().Named("connection"),
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

		// 记录发送前的日志
		c.log.Debug("准备发送消息",
			zap.Int("dataLen", len(data)),
			zap.String("remoteAddr", c.conn.RemoteAddr().String()))

		if err := c.protocol.Write(c.conn, data); err != nil {
			// 检查是否是网络错误
			var isTimeout bool
			if netErr, ok := err.(net.Error); ok {
				isTimeout = netErr.Timeout()
			}

			c.log.Error("写入错误",
				zap.Error(err),
				zap.String("remoteAddr", c.conn.RemoteAddr().String()),
				zap.Int("dataLen", len(data)),
				zap.Bool("isTimeout", isTimeout))

			// 检查连接是否已关闭
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return fmt.Errorf("连接写入超时: %w", ErrWriteErr)
			} else if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return fmt.Errorf("连接已关闭: %w", ErrConnectionClosed)
			}

			return fmt.Errorf("写入错误: %w", ErrWriteErr)
		}

		// 记录发送成功的日志
		c.log.Debug("消息发送成功",
			zap.Int("dataLen", len(data)),
			zap.String("remoteAddr", c.conn.RemoteAddr().String()))

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
