package replication

import (
	"github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/internal/cache/protocol"
	"github.com/xsxdot/aio/pkg/common"
	"strings"
)

// CommandInterceptor 命令拦截器接口
type CommandInterceptor interface {
	// ShouldIntercept 是否拦截该命令
	ShouldIntercept(cmd cache.Command) bool
	// Process 处理被拦截的命令
	Process(cmd cache.Command) cache.Reply
}

// ReplicationCommandInterceptor 复制相关命令拦截器
type ReplicationCommandInterceptor struct {
	roleManager        RoleManager
	replicationManager ReplicationManager
	logger             *common.Logger
}

// NewCommandInterceptor 创建新的命令拦截器
func NewCommandInterceptor(roleManager RoleManager, replicationManager ReplicationManager) CommandInterceptor {
	return &ReplicationCommandInterceptor{
		roleManager:        roleManager,
		replicationManager: replicationManager,
		logger:             common.GetLogger(),
	}
}

// ShouldIntercept 判断是否拦截该命令
func (ci *ReplicationCommandInterceptor) ShouldIntercept(cmd cache.Command) bool {
	if cmd == nil {
		return false
	}

	cmdName := strings.ToUpper(cmd.Name())

	// 复制相关命令需要特殊处理
	if cmdName == CmdReplconf || cmdName == CmdSync || cmdName == CmdPSync {
		return true
	}

	// 从节点不能执行写命令
	if !ci.roleManager.IsMaster() && isWriteCommand(cmdName) {
		return true
	}

	// 主节点需要传播写命令到从节点
	if ci.roleManager.IsMaster() && ci.replicationManager.ShouldReplicate(cmd) {
		return true
	}

	return false
}

// Process 处理被拦截的命令
func (ci *ReplicationCommandInterceptor) Process(cmd cache.Command) cache.Reply {
	if cmd == nil {
		return protocol.NewErrorReply("ERR nil command")
	}

	cmdName := strings.ToUpper(cmd.Name())

	// 处理复制相关命令
	if cmdName == CmdReplconf {
		return ci.replicationManager.ReplConfCommand(cmd)
	}

	if cmdName == CmdSync || cmdName == CmdPSync {
		return ci.replicationManager.SyncCommand(cmd)
	}

	// 从节点不能执行写命令
	if !ci.roleManager.IsMaster() && isWriteCommand(cmdName) {
		return protocol.NewErrorReply("ERR can't write against a read only slave")
	}

	// 主节点传播写命令到从节点
	if ci.roleManager.IsMaster() && ci.replicationManager.ShouldReplicate(cmd) {
		// 处理需要复制的命令，如果已经处理则返回结果
		if reply := ci.replicationManager.HandleReplicatedCommand(cmd); reply != nil {
			return reply
		}
	}

	// 命令可以继续正常处理
	return nil
}

// isWriteCommand 判断是否为写命令
func isWriteCommand(cmdName string) bool {
	// 统一转为大写，以便不区分大小写比较
	cmdName = strings.ToUpper(cmdName)

	// 常见的写命令列表
	writeCommands := map[string]bool{
		"SET":       true,
		"SETNX":     true,
		"SETEX":     true,
		"PSETEX":    true,
		"APPEND":    true,
		"DEL":       true,
		"UNLINK":    true,
		"INCR":      true,
		"DECR":      true,
		"INCRBY":    true,
		"DECRBY":    true,
		"LPUSH":     true,
		"RPUSH":     true,
		"LPUSHX":    true,
		"RPUSHX":    true,
		"LPOP":      true,
		"RPOP":      true,
		"LINSERT":   true,
		"LSET":      true,
		"LREM":      true,
		"LTRIM":     true,
		"RPOPLPUSH": true,
		"SADD":      true,
		"SREM":      true,
		"SPOP":      true,
		"ZADD":      true,
		"ZREM":      true,
		"HMSET":     true,
		"HSET":      true,
		"HDEL":      true,
		"EXPIRE":    true,
		"EXPIREAT":  true,
		"FLUSHDB":   true,
		"FLUSHALL":  true,
		"MSET":      true,
	}

	return writeCommands[cmdName]
}
