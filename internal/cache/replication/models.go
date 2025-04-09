package replication

import (
	"encoding/binary"
	"github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/pkg/network"
	"sync"
	"time"
)

// ReplicationRole 定义复制角色类型
type ReplicationRole string

const (
	// RoleMaster 主节点角色
	RoleMaster ReplicationRole = "master"
	// RoleSlave 从节点角色
	RoleSlave ReplicationRole = "slave"
	// RoleNone 未确定角色
	RoleNone ReplicationRole = "none"
)

// ReplicationCommand 定义复制相关命令
const (
	CmdReplconf = "REPLCONF"
	CmdSync     = "SYNC"
	CmdPSync    = "PSYNC"
)

// ReplicationManager 复制管理接口
type ReplicationManager interface {
	// Start 启动复制管理
	Start() error
	// Stop 停止复制管理
	Stop() error
	// HandleReplicatedCommand 处理复制命令
	HandleReplicatedCommand(cmd cache.Command) cache.Reply
	// ShouldReplicate 判断命令是否需要复制
	ShouldReplicate(cmd cache.Command) bool
	// SyncFromMaster 从主节点同步数据（从节点调用）
	SyncFromMaster() error
	// RegisterSlave 注册从节点（主节点调用）
	RegisterSlave(slaveInfo SlaveInfo) error
	// SyncCommand 处理同步命令
	SyncCommand(cmd cache.Command) cache.Reply
	// ReplConfCommand 处理配置命令
	ReplConfCommand(cmd cache.Command) cache.Reply
	// OnRoleChange 角色变更通知
	OnRoleChange(oldRole, newRole ReplicationRole)
	// SetMaster 设置主节点信息
	SetMaster(host string, port int) error
	// GetConnectedSlaves 获取当前连接的从节点列表
	GetConnectedSlaves() [][]string
	// GetMasterAddr 获取主节点地址（格式：host:port）
	GetMasterAddr() string
}

// ReplChangeEvent 复制更改事件
type ReplChangeEvent struct {
	ChangeType string      // 事件类型
	Data       interface{} // 事件数据
}

// ReplicationState 复制状态
type ReplicationState struct {
	Role                ReplicationRole
	MasterHost          string
	MasterPort          int
	MasterConn          *network.Connection // 与主节点的连接
	ReplicationID       string              // 复制标识符
	ReplicaOffset       int64               // 复制偏移量
	ReplBuffer          *ReplBuffer         // 复制缓冲区
	Slaves              sync.Map            // 从节点信息映射 (string -> *SlaveInfo)
	SyncInProgress      bool                // 是否正在进行全量同步
	SyncStartOffset     int64               // 全量同步开始时的偏移量
	BufferedCmds        []*ReplCommand      // 全量同步期间缓存的命令
	BufferMutex         sync.Mutex          // 用于保护命令缓冲区的互斥锁
	ConnectingToMaster  bool                // 是否正在连接主节点
	ConnectedToMaster   bool                // 是否已经连接到主节点
	LastMasterHeartbeat time.Time           // 上次收到主节点心跳的时间
}

// ReplBuffer 复制缓冲区
type ReplBuffer struct {
	mutex     sync.RWMutex
	buffer    []byte // 环形缓冲区
	size      int    // 缓冲区大小
	writePos  int    // 写入位置
	minOffset int64  // 缓冲区起始偏移量
	maxOffset int64  // 缓冲区结束偏移量
}

// NewReplBuffer 创建新的复制缓冲区
func NewReplBuffer(size int) *ReplBuffer {
	return &ReplBuffer{
		buffer: make([]byte, size),
		size:   size,
	}
}

// Write 写入数据到缓冲区
func (b *ReplBuffer) Write(data []byte) int64 {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	offset := b.maxOffset

	// 写入数据
	for i := 0; i < len(data); i++ {
		b.buffer[b.writePos] = data[i]
		b.writePos = (b.writePos + 1) % b.size
		b.maxOffset++

		// 如果覆盖了最小偏移量，更新最小偏移量
		if b.maxOffset-b.minOffset > int64(b.size) {
			b.minOffset = b.maxOffset - int64(b.size)
		}
	}

	return offset
}

// Read 从指定偏移量读取数据
func (b *ReplBuffer) Read(offset int64, maxLen int) ([]byte, int64, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// 检查偏移量是否在缓冲区范围内
	if offset < b.minOffset || offset > b.maxOffset {
		return nil, b.minOffset, false
	}

	// 计算读取位置和长度
	startPos := int((offset - b.minOffset) % int64(b.size))
	available := int(b.maxOffset - offset)
	if available > maxLen {
		available = maxLen
	}

	// 读取数据
	result := make([]byte, available)
	for i := 0; i < available; i++ {
		pos := (startPos + i) % b.size
		result[i] = b.buffer[pos]
	}

	return result, offset + int64(available), true
}

// ReadRange 读取指定偏移量范围内的所有命令数据
// 返回值：
// - [][]byte：每个元素是一个完整的命令数据
// - bool：是否成功读取
func (b *ReplBuffer) ReadRange(startOffset, endOffset int64) ([][]byte, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// 检查偏移量是否在缓冲区范围内
	if startOffset < b.minOffset {
		// 起始偏移量已经超出缓冲区范围
		return nil, false
	}

	if startOffset >= endOffset {
		// 无数据可读
		return [][]byte{}, true
	}

	// 确保结束偏移量不超过当前最大偏移量
	if endOffset > b.maxOffset {
		endOffset = b.maxOffset
	}

	// 读取范围内的所有数据
	totalBytes := int(endOffset - startOffset)
	if totalBytes <= 0 {
		return [][]byte{}, true
	}

	// 首先读取所有数据为一个连续的字节数组
	startPos := int((startOffset - b.minOffset) % int64(b.size))
	rawData := make([]byte, totalBytes)

	for i := 0; i < totalBytes; i++ {
		pos := (startPos + i) % b.size
		rawData[i] = b.buffer[pos]
	}

	// 将原始数据根据命令格式进行分割
	// 注意：这里假设每个命令的格式为 [4字节长度][命令数据]
	// 实际实现中可能需要根据具体的命令格式调整
	var commands [][]byte
	offset := 0

	for offset < len(rawData) {
		// 确保有足够的数据读取长度
		if offset+4 > len(rawData) {
			break
		}

		// 读取命令长度
		length := int(binary.BigEndian.Uint32(rawData[offset : offset+4]))
		offset += 4

		// 确保有足够的数据读取命令
		if offset+length > len(rawData) {
			break
		}

		// 提取命令数据
		command := make([]byte, length)
		copy(command, rawData[offset:offset+length])
		commands = append(commands, command)

		offset += length
	}

	return commands, true
}

// SlaveInfo 从节点信息
type SlaveInfo struct {
	ID             string
	Host           string
	Port           int
	LastACKTime    time.Time           // 上次确认时间
	Offset         int64               // 当前偏移量（已确认）
	ExpectedOffset int64               // 预期偏移量（已发送但未确认）
	Connection     *network.Connection // 与从节点的连接
}

// RoleChangeEvent 角色变更事件
type RoleChangeEvent struct {
	OldRole ReplicationRole
	NewRole ReplicationRole
}

// RoleChangeListener 角色变更监听接口
type RoleChangeListener interface {
	// OnRoleChange 角色变更通知
	OnRoleChange(oldRole, newRole ReplicationRole)
}

// ReplicationCommands 复制相关命令处理
type ReplicationCommands interface {
	// SyncCommand 处理同步命令
	SyncCommand(cmd cache.Command) cache.Reply
	// ReplConfCommand 处理配置命令
	ReplConfCommand(cmd cache.Command) cache.Reply
}
