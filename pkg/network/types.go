package network

import (
	"errors"
	"io"
	"time"
)

// 错误定义
var (
	ErrConnectionNotFound = errors.New("connection not found")
	ErrConnectionClosed   = errors.New("connection closed")
	ErrMessageTooLarge    = errors.New("message too large")
	ErrInvalidProtocol    = errors.New("invalid protocol")
	ErrInvalidHandler     = errors.New("invalid handler")
	ErrMaxConnections     = errors.New("max connections reached")
	ErrTimeout            = errors.New("operation timeout")
)

// Message 消息接口
type Message interface {
	// ToBytes 将消息转换为字节数组
	ToBytes() []byte
}

// Protocol 协议接口
type Protocol interface {
	// Read 读取消息
	Read(reader io.Reader) ([]byte, error)
	// Write 写入消息
	Write(writer io.Writer, data []byte) error
	// Name 获取协议名称
	Name() string
}

// MessageHandler 消息处理器
type MessageHandler struct {
	// Handle 处理消息
	Handle func(conn *Connection, data []byte) error
	// GetHeartbeat 获取心跳消息
	GetHeartbeat func() Message
	// ConnectionClosed 连接关闭回调
	ConnectionClosed func(connID string)
}

// ConnectionState 连接状态
type ConnectionState struct {
	Connected  bool
	LastActive time.Time
	ReadBytes  uint64
	WriteBytes uint64
	ReadCount  uint64
	WriteCount uint64
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadCount  uint64
	WriteCount uint64
	LastActive time.Time
}

// Options 连接管理器选项
type Options struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxConnections  int
	BufferSize      int
	EnableKeepAlive bool
	// 心跳间隔
	HeartbeatInterval time.Duration
}

// DefaultOptions 默认选项
var DefaultOptions = &Options{
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      30 * time.Second,
	IdleTimeout:       60 * time.Second,
	MaxConnections:    0, // 0表示不限制连接数量
	BufferSize:        4096,
	EnableKeepAlive:   true,
	HeartbeatInterval: 30 * time.Second,
}
