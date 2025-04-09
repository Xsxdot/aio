// Package cache 提供一个类Redis的内存缓存系统
package cache

import (
	"github.com/xsxdot/aio/pkg/network"
)

// Command 表示一个缓存命令
type Command interface {
	// Name 命令名称
	Name() string
	// Args 命令参数
	Args() []string
	// ClientID 客户端ID
	ClientID() string
	// SetReply 设置命令的回复
	SetReply(reply Reply)
	// GetReply 获取命令的回复通道
	GetReply() chan Reply
	// IsWriteCommand 判断命令是否会修改数据
	IsWriteCommand() bool
	// GetDbIndex 获取命令关联的数据库索引
	GetDbIndex() int
	// SetDbIndex 设置命令关联的数据库索引
	SetDbIndex(index int)
}

// Reply 表示命令的回复
type Reply interface {
	// Type 回复类型
	Type() ReplyType
	// String 返回回复的字符串形式
	String() string
	// Bytes 返回回复的字节数组形式
	Bytes() []byte
	// ToMessage 转换为网络消息
	ToMessage() network.Message
}

// Snapshotter 提供快照生成和加载功能
type Snapshotter interface {
	// SaveSnapshot 将数据快照保存到指定文件路径
	SaveSnapshot(filePath string) error

	// LoadSnapshot 从指定文件路径加载快照
	LoadSnapshot(filePath string) error
}
