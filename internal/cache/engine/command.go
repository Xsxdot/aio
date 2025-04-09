package engine

import (
	"github.com/xsxdot/aio/internal/cache"
	"strings"
)

// Command 是Command接口的实现
type Command interface {
	cache.Command
}

// NewCommand 创建一个新的命令
func NewCommand(name string, args []string, clientID string) cache.Command {
	return &simpleCommand{
		name:     name,
		args:     args,
		clientID: clientID,
		replyCh:  make(chan cache.Reply, 1),
	}
}

// simpleCommand 是Command接口的简单实现
type simpleCommand struct {
	name     string
	args     []string
	clientID string
	replyCh  chan cache.Reply
	dbIndex  int // 新增的数据库索引字段
}

// Name 返回命令名称
func (c *simpleCommand) Name() string {
	return c.name
}

// Args 返回命令参数
func (c *simpleCommand) Args() []string {
	return c.args
}

// ClientID 返回客户端ID
func (c *simpleCommand) ClientID() string {
	return c.clientID
}

// SetReply 设置命令响应
func (c *simpleCommand) SetReply(reply cache.Reply) {
	c.replyCh <- reply
}

// GetReply 返回命令回复通道
func (c *simpleCommand) GetReply() chan cache.Reply {
	return c.replyCh
}

// IsWriteCommand 判断命令是否会修改数据
func (c *simpleCommand) IsWriteCommand() bool {
	cmdName := strings.ToUpper(c.name)
	return isWriteCommand(cmdName)
}

// GetDbIndex 获取命令关联的数据库索引
func (c *simpleCommand) GetDbIndex() int {
	return c.dbIndex
}

// SetDbIndex 设置命令关联的数据库索引
func (c *simpleCommand) SetDbIndex(index int) {
	c.dbIndex = index
}

// isWriteCommand 判断命令是否为写命令
func isWriteCommand(cmdName string) bool {
	// 定义写入命令列表
	writeCommands := map[string]bool{
		"SET":              true,
		"SETNX":            true,
		"SETEX":            true,
		"PSETEX":           true,
		"MSET":             true,
		"MSETNX":           true,
		"APPEND":           true,
		"DEL":              true,
		"UNLINK":           true,
		"INCR":             true,
		"INCRBY":           true,
		"INCRBYFLOAT":      true,
		"DECR":             true,
		"DECRBY":           true,
		"RPUSH":            true,
		"LPUSH":            true,
		"RPUSHX":           true,
		"LPUSHX":           true,
		"LINSERT":          true,
		"LSET":             true,
		"LREM":             true,
		"LTRIM":            true,
		"RPOP":             true,
		"LPOP":             true,
		"RPOPLPUSH":        true,
		"LMOVE":            true,
		"BLMOVE":           true,
		"SADD":             true,
		"SREM":             true,
		"SMOVE":            true,
		"SPOP":             true,
		"SINTERSTORE":      true,
		"SUNIONSTORE":      true,
		"SDIFFSTORE":       true,
		"ZADD":             true,
		"ZINCRBY":          true,
		"ZREM":             true,
		"ZREMRANGEBYRANK":  true,
		"ZREMRANGEBYSCORE": true,
		"ZREMRANGEBYLEX":   true,
		"ZINTERSTORE":      true,
		"ZUNIONSTORE":      true,
		"HSET":             true,
		"HSETNX":           true,
		"HMSET":            true,
		"HINCRBY":          true,
		"HINCRBYFLOAT":     true,
		"HDEL":             true,
		"EXPIRE":           true,
		"EXPIREAT":         true,
		"PEXPIRE":          true,
		"PEXPIREAT":        true,
		"FLUSHDB":          true,
		"FLUSHALL":         true,
		"RENAME":           true,
		"RENAMENX":         true,
	}

	return writeCommands[cmdName]
}
