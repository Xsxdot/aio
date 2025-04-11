package replication

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	cache2 "github.com/xsxdot/aio/internal/cache"
	engine2 "github.com/xsxdot/aio/internal/cache/engine"
	protocol2 "github.com/xsxdot/aio/internal/cache/protocol"
	"github.com/xsxdot/aio/pkg/common"
	network2 "github.com/xsxdot/aio/pkg/network"
	protocol3 "github.com/xsxdot/aio/pkg/protocol"
)

//todo 连接到主节点需要认证

// DefaultReplBufferSize 默认复制缓冲区大小 (1MB)
const DefaultReplBufferSize = 1024 * 1024

// ReplSyncTimeout 复制同步超时时间
const ReplSyncTimeout = 60 * time.Second

// 定义复制相关的消息类型和服务类型
const (
	// 复制消息类型
	MsgTypeReplSync      protocol3.MessageType = 100 // 同步请求
	MsgTypeReplSyncResp  protocol3.MessageType = 101 // 同步响应
	MsgTypeReplPSYNC     protocol3.MessageType = 102 // 部分同步请求
	MsgTypeReplPSYNCResp protocol3.MessageType = 103 // 部分同步响应
	MsgTypeReplData      protocol3.MessageType = 104 // 复制数据
	MsgTypeReplCommand   protocol3.MessageType = 105 // 命令传播
	MsgTypeReplACK       protocol3.MessageType = 106 // 确认消息
	MsgTypeReplHeartbeat protocol3.MessageType = 107 // 心跳消息
	MsgTypeGetMasterInfo protocol3.MessageType = 108 // 客户端请求主节点信息
	MsgTypeMasterChanged protocol3.MessageType = 109 // 主节点变更通知

	// 复制服务类型
	ServiceTypeReplication = protocol3.ServiceTypeReplication // 复制服务
)

// ReplCommand 复制命令消息结构体，用于在复制过程中传递数据库上下文
type ReplCommand struct {
	DbIndex uint32 // 数据库索引
	Command []byte // 原始命令数据
	Offset  int64  // 命令对应的复制偏移量
}

// Serialize 将ReplCommand序列化为二进制格式
// 格式: [4字节数据库索引][8字节偏移量][N字节命令数据]
func (rc *ReplCommand) Serialize() []byte {
	buf := make([]byte, 4+8+len(rc.Command))
	binary.BigEndian.PutUint32(buf[:4], rc.DbIndex)
	binary.BigEndian.PutUint64(buf[4:12], uint64(rc.Offset))
	copy(buf[12:], rc.Command)
	return buf
}

// DeserializeReplCommand 从二进制数据反序列化为ReplCommand
func DeserializeReplCommand(data []byte) (*ReplCommand, error) {
	if len(data) < 12 {
		return nil, errors.New("无效的复制命令格式：数据长度不足")
	}
	dbIndex := binary.BigEndian.Uint32(data[:4])
	offset := int64(binary.BigEndian.Uint64(data[4:12]))
	return &ReplCommand{
		DbIndex: dbIndex,
		Offset:  offset,
		Command: data[12:],
	}, nil
}

// DefaultReplicationManager 默认复制管理器实现
type DefaultReplicationManager struct {
	state         *ReplicationState
	engine        *engine2.MemoryEngine
	roleManager   RoleManager
	discovery     ServiceDiscover
	config        *cache2.Config
	protocolMgr   *protocol3.ProtocolManager
	stopChan      chan struct{}
	logger        *common.Logger
	mutex         sync.RWMutex
	processedCmds sync.Map // 用于存储已处理的命令hash，防止重复执行
	masterClients sync.Map // 用于存储需要主节点信息更新的客户端连接
}

// NewReplicationManager 创建新的复制管理器
func NewReplicationManager(engine *engine2.MemoryEngine, roleManager RoleManager, discovery ServiceDiscover, config *cache2.Config, protocolMgr *protocol3.ProtocolManager) ReplicationManager {
	// 创建复制缓冲区
	replBuffer := NewReplBuffer(DefaultReplBufferSize)

	// 生成复制ID
	replicationID := generateReplicationID()

	// 创建状态
	stat := &ReplicationState{
		Role:               RoleNone,
		ReplicationID:      replicationID,
		ReplicaOffset:      0,
		ReplBuffer:         replBuffer,
		ConnectingToMaster: false,
		ConnectedToMaster:  false,
	}

	// 如果没有传入协议管理器，则创建一个新的
	if protocolMgr == nil {
		protocolMgr = protocol3.NewServer(nil)
	}

	return &DefaultReplicationManager{
		state:       stat,
		engine:      engine,
		roleManager: roleManager,
		discovery:   discovery,
		config:      config,
		protocolMgr: protocolMgr,
		stopChan:    make(chan struct{}),
		logger:      common.GetLogger(),
	}
}

// Start 启动复制管理
func (rm *DefaultReplicationManager) Start() error {
	// 注册复制服务和处理器
	rm.registerReplicationHandlers()

	// 启动过期命令记录清理
	go rm.startCommandCleanup()

	// 启动客户端连接清理
	go rm.startClientConnectionCleanup()

	// 基于当前角色决定如何启动
	role := rm.roleManager.GetRole()
	rm.state.Role = role

	// 使用协议管理器的NetworkManager获取绑定的地址和端口
	var host string
	var port int

	// 尝试从协议管理器获取地址和端口
	host, port = rm.protocolMgr.GetServerConfig()

	if host == "" || port == 0 {
		println(rm.protocolMgr)
		return fmt.Errorf("无法获取协议管理器的地址和端口,%s,%d", host, port)
	}

	// 注册到服务发现
	err := rm.registerService(host, port)
	if err != nil {
		return fmt.Errorf("注册服务失败: %v", err)
	}

	// 监听主节点变更
	rm.discovery.WatchMasterChange(rm.handleMasterChange)

	// 如果是从节点，连接主节点
	if role == RoleSlave {
		go rm.connectToMaster()
	}

	return nil
}

// Stop 停止复制管理
// 注销服务时使用与registerService相同的服务ID生成逻辑，
// 优先使用config.NodeID，然后是环境变量AIO_NODE_ID
func (rm *DefaultReplicationManager) Stop() error {
	close(rm.stopChan)

	// 获取服务ID
	serviceID := rm.getServiceID(rm.config.Host, rm.config.Port)

	// 从服务发现中注销
	if err := rm.discovery.Deregister(serviceID); err != nil {
		rm.logger.Errorf("注销服务失败: %v", err)
	}

	// 清理所有客户端连接
	var clientCount int
	rm.masterClients.Range(func(key, value interface{}) bool {
		rm.masterClients.Delete(key)
		clientCount++
		return true
	})

	if clientCount > 0 {
		rm.logger.Infof("已清理 %d 个主节点信息订阅客户端", clientCount)
	}

	return nil
}

// ShouldReplicate 判断命令是否需要复制
func (rm *DefaultReplicationManager) ShouldReplicate(cmd cache2.Command) bool {
	if cmd == nil {
		return false
	}

	cmdName := strings.ToUpper(cmd.Name())

	// SELECT命令不需要复制，因为我们已经在命令中包含了数据库索引
	if cmdName == "SELECT" {
		return false
	}

	return isWriteCommand(cmdName)
}

// HandleReplicatedCommand 处理复制命令
func (rm *DefaultReplicationManager) HandleReplicatedCommand(cmd cache2.Command) cache2.Reply {
	// 只有主节点才处理复制
	if rm.state.Role != RoleMaster {
		rm.logger.Debugf("跳过复制命令，当前角色不是主节点: %s", rm.state.Role)
		return nil
	}

	// 检查是否有从节点连接
	var slaveCount int
	rm.state.Slaves.Range(func(_, _ interface{}) bool {
		slaveCount++
		return true
	})

	if slaveCount == 0 {
		rm.logger.Debugf("跳过复制命令，没有从节点连接")
		return nil
	}

	// 直接从命令对象获取数据库索引
	dbIndex := cmd.GetDbIndex()

	// 将命令转换为RESP格式字符串
	cmdStr := convertCommandToRESP(cmd)
	if cmdStr == "" {
		rm.logger.Warnf("命令转换为RESP格式失败: %s %v", cmd.Name(), cmd.Args())
		return nil
	}

	// 创建ReplCommand结构体
	replCmd := &ReplCommand{
		DbIndex: uint32(dbIndex),
		Command: []byte(cmdStr),
		Offset:  0, // 偏移量将在broadcastCommand中设置
	}

	// 序列化为二进制数据
	cmdBytes := replCmd.Serialize()

	// 写入复制缓冲区并记录偏移量
	rm.mutex.Lock()
	offset := rm.state.ReplBuffer.Write(cmdBytes)
	rm.state.ReplicaOffset = offset + int64(len(cmdBytes))
	rm.mutex.Unlock()

	rm.logger.Debugf("准备复制命令: %s %v, 数据库索引=%d, 长度=%d字节",
		cmd.Name(), cmd.Args(), dbIndex, len(cmdBytes))

	// 广播命令到所有从节点
	go rm.broadcastCommand(cmdBytes)

	// 返回nil表示命令需要继续正常处理
	return nil
}

// convertCommandToRESP 将命令转换为RESP格式字符串
func convertCommandToRESP(cmd cache2.Command) string {
	if cmd == nil {
		return ""
	}

	args := cmd.Args()
	name := cmd.Name()
	allArgs := append([]string{name}, args...)

	var sb strings.Builder
	// 数组长度
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(allArgs)))

	// 添加每个参数
	for _, arg := range allArgs {
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	return sb.String()
}

// SyncFromMaster 从主节点同步数据
func (rm *DefaultReplicationManager) SyncFromMaster() error {
	// 只有从节点才需要同步
	if rm.state.Role != RoleSlave {
		return fmt.Errorf("只有从节点才能进行同步")
	}

	// 尝试连接主节点
	return rm.connectToMaster()
}

// RegisterSlave 注册从节点
func (rm *DefaultReplicationManager) RegisterSlave(slaveInfo SlaveInfo) error {
	// 只有主节点才接受从节点注册
	if rm.state.Role != RoleMaster {
		return fmt.Errorf("只有主节点才能接受从节点注册")
	}

	// 获取当前的复制偏移量
	rm.mutex.RLock()
	currentOffset := rm.state.ReplicaOffset
	rm.mutex.RUnlock()

	// 初始化从节点的偏移量
	slaveInfo.Offset = 0
	slaveInfo.ExpectedOffset = 0

	// 存储从节点信息
	rm.state.Slaves.Store(slaveInfo.ID, &slaveInfo)

	rm.logger.Infof("从节点已注册: %s (%s:%d), 主节点当前偏移量: %d",
		slaveInfo.ID, slaveInfo.Host, slaveInfo.Port, currentOffset)

	return nil
}

// SyncCommand 处理同步命令
func (rm *DefaultReplicationManager) SyncCommand(cmd cache2.Command) cache2.Reply {
	// 只有主节点才处理同步请求
	if rm.roleManager.GetRole() != RoleMaster {
		return protocol2.NewErrorReply("ERR cannot sync with slave: not a master")
	}

	// 获取客户端ID
	clientID := cmd.ClientID()
	if clientID == "" {
		return protocol2.NewErrorReply("ERR client ID not found")
	}

	// 获取客户端连接
	conn, ok := rm.protocolMgr.GetConnection(clientID)
	if !ok {
		return protocol2.NewErrorReply("ERR client connection not found")
	}

	// 创建从节点信息
	slaveInfo := &SlaveInfo{
		ID:             clientID,
		LastACKTime:    time.Now(),
		Offset:         0,
		ExpectedOffset: 0,
		Connection:     conn,
	}

	// 存储从节点信息
	rm.state.Slaves.Store(clientID, slaveInfo)

	rm.logger.Infof("接收到从节点同步请求: %s", clientID)

	// 异步执行全量同步
	go rm.fullSync(slaveInfo)

	// 返回同步开始的响应
	return protocol2.NewStatusReply("FULLRESYNC " + rm.state.ReplicationID + " 0")
}

// ReplConfCommand 处理配置命令
func (rm *DefaultReplicationManager) ReplConfCommand(cmd cache2.Command) cache2.Reply {
	args := cmd.Args()
	if len(args) < 2 {
		return protocol2.NewErrorReply("ERR wrong number of arguments for REPLCONF command")
	}

	subCommand := strings.ToUpper(args[0])

	switch subCommand {
	case "LISTENING-PORT":
		// 从节点告知其监听端口
		port, err := strconv.Atoi(args[1])
		if err != nil {
			return protocol2.NewErrorReply("ERR invalid port")
		}

		// 获取从节点连接
		clientID := cmd.ClientID()
		slaveObj, ok := rm.state.Slaves.Load(clientID)
		if ok {
			slave := slaveObj.(*SlaveInfo)
			slave.Port = port
			rm.state.Slaves.Store(clientID, slave)
		}

		return protocol2.NewStatusReply("OK")

	case "ACK":
		// 从节点确认复制偏移量
		offset, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return protocol2.NewErrorReply("ERR invalid offset")
		}

		// 更新从节点复制偏移量
		clientID := cmd.ClientID()
		slaveObj, ok := rm.state.Slaves.Load(clientID)
		if ok {
			slave := slaveObj.(*SlaveInfo)
			previousOffset := slave.Offset
			slave.Offset = offset
			slave.LastACKTime = time.Now()

			// 记录与预期偏移量的差距
			if slave.ExpectedOffset > offset {
				lag := slave.ExpectedOffset - offset
				if lag > 1024*1024 { // 超过1MB的差距才记录警告
					rm.logger.Warnf("从节点 %s 偏移量落后: 当前=%d, 预期=%d, 差距=%d",
						clientID, offset, slave.ExpectedOffset, lag)
				}
			}

			// 计算复制速率
			if previousOffset > 0 && offset > previousOffset {
				progress := offset - previousOffset
				rm.logger.Debugf("从节点 %s 确认偏移量: %d, 进度: +%d 字节", clientID, offset, progress)
			} else {
				rm.logger.Debugf("从节点 %s 确认偏移量: %d", clientID, offset)
			}

			rm.state.Slaves.Store(clientID, slave)
		}

		return protocol2.NewStatusReply("OK")
	}

	return protocol2.NewErrorReply("ERR unknown REPLCONF subcommand: " + subCommand)
}

// OnRoleChange 角色变更通知
func (rm *DefaultReplicationManager) OnRoleChange(oldRole, newRole ReplicationRole) {
	rm.mutex.Lock()
	rm.state.Role = newRole
	rm.mutex.Unlock()

	rm.logger.Infof("节点角色变更: %s -> %s", oldRole, newRole)

	// 更新服务信息
	host, port := rm.protocolMgr.GetServerConfig()
	rm.registerService(host, port)

	if oldRole == RoleSlave && newRole == RoleMaster {
		// 从节点升级为主节点
		rm.handlePromotionToMaster()
	} else if oldRole == RoleMaster && newRole == RoleSlave {
		// 主节点降级为从节点
		rm.handleDemotionToSlave()
	}
}

// 内部辅助方法

// getServiceID 获取服务ID
// 使用config.NodeID
// 格式为{nodeID}-cache，如果都未设置，则使用{host}:{port}作为服务ID
func (rm *DefaultReplicationManager) getServiceID(host string, port int) string {
	// 默认使用host:port格式
	serviceID := fmt.Sprintf("%s:%d", host, port)

	// 优先使用config.NodeID
	if rm.config != nil && rm.config.NodeID != "" {
		serviceID = fmt.Sprintf("%s-cache", rm.config.NodeID)
		return serviceID
	}

	return serviceID
}

// registerService 注册服务信息
// 该方法使用如下优先级确定服务ID：
// 1. config.NodeID（优先使用，格式为：{nodeID}-cache）
// 3. host:port格式（如果以上未设置）
// 通过使用唯一的节点ID可以防止多个节点使用相同的localhost导致的ID冲突问题
func (rm *DefaultReplicationManager) registerService(host string, port int) error {
	role := rm.state.Role

	// 获取协议管理器的实际端口
	_, protocolPort := rm.protocolMgr.GetServerConfig()

	// 获取服务ID
	serviceID := rm.getServiceID(host, port)

	info := ServiceInfo{
		ID:           serviceID,
		Host:         host,
		Port:         port,
		ProtocolPort: protocolPort,
		Role:         role,
		NodeID:       rm.config.NodeID,
	}

	// 如果是从节点且已知主节点信息，添加主节点ID
	if role == RoleSlave && rm.state.MasterHost != "" {
		info.MasterID = fmt.Sprintf("%s:%d", rm.state.MasterHost, rm.state.MasterPort)
	}

	return rm.discovery.Register(info)
}

// registerReplicationHandlers 注册复制相关的消息处理器
func (rm *DefaultReplicationManager) registerReplicationHandlers() {
	// 创建复制服务处理器
	replHandler := rm.protocolMgr

	// 注册同步请求处理器
	syncHandle := func(connID string, msg *protocol3.CustomMessage) (interface{}, error) {
		// 处理同步请求
		rm.logger.Infof("接收到从节点同步请求: %s", connID)

		// 获取连接
		conn, ok := rm.protocolMgr.GetConnection(connID)
		if !ok || conn == nil {
			return nil, fmt.Errorf("连接不存在: %s", connID)
		}

		// 创建从节点信息
		slaveInfo := &SlaveInfo{
			ID:          connID,
			LastACKTime: time.Now(),
			Offset:      0,
			Connection:  conn,
		}

		// 存储从节点信息
		rm.state.Slaves.Store(connID, slaveInfo)

		// 执行全量同步
		go rm.fullSync(slaveInfo)

		return protocol3.OK, nil
	}

	// 注册同步响应处理器
	syncRespHandle := func(connID string, msg *protocol3.CustomMessage) (interface{}, error) {
		// 只有从节点才处理同步响应
		if rm.state.Role != RoleSlave {
			return nil, nil
		}

		// 解析同步响应消息
		respStr := string(msg.Payload())
		rm.logger.Infof("接收到主节点同步响应: %s", respStr)

		// 解析复制ID和偏移量
		parts := strings.Split(respStr, " ")
		if len(parts) >= 3 && parts[0] == "FULLRESYNC" {
			// 提取复制ID和偏移量
			replicationID := parts[1]
			offset, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("解析偏移量失败: %v", err)
			}

			// 更新复制状态
			rm.mutex.Lock()
			rm.state.ReplicationID = replicationID
			rm.state.ReplicaOffset = offset
			rm.mutex.Unlock()

			rm.logger.Infof("同步响应解析成功: 复制ID=%s, 偏移量=%d", replicationID, offset)
		} else {
			return nil, fmt.Errorf("无效的同步响应格式: %s", respStr)
		}

		return protocol3.OK, nil
	}

	// 注册数据处理器
	replDataHandle := func(connID string, msg *protocol3.CustomMessage) (interface{}, error) {
		// 只有从节点才处理复制数据
		if rm.state.Role != RoleSlave {
			return nil, nil

		}

		data := msg.Payload()
		dataSize := len(data)
		rm.logger.Infof("接收到主节点数据: %d 字节", dataSize)

		// 在实际实现中，这里应该解析RDB数据并加载到数据库
		// 这里简化处理，仅记录接收到数据

		if strings.HasPrefix(string(data), "EMPTY_RDB_DATA") {
			rm.logger.Info("接收到空RDB数据集")
		} else {
			// 实际应用中需要将RDB数据加载到引擎
			rm.logger.Info("需要解析并加载RDB数据")

			// 这里应该调用引擎的方法加载RDB数据
			// rm.engine.LoadRDB(data)
		}

		// 更新复制偏移量
		rm.mutex.Lock()
		rm.state.ReplicaOffset += int64(dataSize)
		offset := rm.state.ReplicaOffset
		rm.mutex.Unlock()

		// 发送确认消息到主节点
		offsetStr := fmt.Sprintf("%d", offset)
		err := rm.protocolMgr.SendMessage(connID, MsgTypeReplACK, ServiceTypeReplication, []byte(offsetStr))
		if err != nil {
			rm.logger.Errorf("发送数据确认失败: %v", err)
			return nil, err
		}

		rm.logger.Infof("数据接收完成，当前偏移量: %d", offset)
		return protocol3.OK, nil
	}

	// 注册确认消息处理器
	replAckHandle := func(connID string, msg *protocol3.CustomMessage) (interface{}, error) {
		// 解析复制确认信息
		offsetStr := string(msg.Payload())
		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析偏移量失败: %v", err)
		}

		// 更新从节点复制偏移量
		slaveObj, ok := rm.state.Slaves.Load(connID)
		if ok {
			slave := slaveObj.(*SlaveInfo)
			previousOffset := slave.Offset
			slave.Offset = offset
			slave.LastACKTime = time.Now()

			// 记录与预期偏移量的差距
			if slave.ExpectedOffset > offset {
				lag := slave.ExpectedOffset - offset
				if lag > 1024*1024 { // 超过1MB的差距才记录警告
					rm.logger.Warnf("从节点 %s 偏移量落后: 当前=%d, 预期=%d, 差距=%d",
						connID, offset, slave.ExpectedOffset, lag)
				}
			}

			// 计算复制速率
			if previousOffset > 0 && offset > previousOffset {
				progress := offset - previousOffset
				rm.logger.Debugf("从节点 %s 确认偏移量: %d, 进度: +%d 字节", connID, offset, progress)
			} else {
				rm.logger.Debugf("从节点 %s 确认偏移量: %d", connID, offset)
			}

			rm.state.Slaves.Store(connID, slave)
		}

		return protocol3.OK, nil
	}

	// 注册命令消息处理器
	replCmd := func(connID string, msg *protocol3.CustomMessage) (interface{}, error) {
		// 只有从节点才处理复制命令
		if rm.state.Role != RoleSlave {
			return nil, nil
		}

		// 获取命令数据
		cmdData := msg.Payload()
		if len(cmdData) == 0 {
			return nil, fmt.Errorf("空命令数据")
		}

		// 反序列化命令数据
		replCmd, err := DeserializeReplCommand(cmdData)
		if err != nil {
			rm.logger.Errorf("解析复制命令失败: %v", err)
			return nil, err
		}

		// 获取命令偏移量和数据库索引
		cmdOffset := replCmd.Offset
		dbIndex := int(replCmd.DbIndex)
		actualCmdData := replCmd.Command

		// 只使用命令内容计算哈希，不使用偏移量
		cmdHash := fmt.Sprintf("%x", md5.Sum(actualCmdData))

		// 检查是否已处理过该命令
		if _, exists := rm.processedCmds.Load(cmdHash); exists {
			rm.logger.Infof("跳过重复命令: hash=%s", cmdHash[:8])

			// 更新复制偏移量并发送确认消息
			rm.mutex.Lock()
			rm.state.ReplicaOffset = cmdOffset + int64(len(actualCmdData))
			offset := rm.state.ReplicaOffset
			rm.mutex.Unlock()

			// 发送确认消息
			offsetStr := fmt.Sprintf("%d", offset)
			return offsetStr, nil
		}

		// 检查当前是否正在进行全量同步
		rm.mutex.RLock()
		syncInProgress := rm.state.SyncInProgress
		rm.mutex.RUnlock()

		if syncInProgress {
			// 如果正在进行全量同步，缓存命令以便稍后执行
			rm.mutex.Lock()
			rm.state.BufferedCmds = append(rm.state.BufferedCmds, replCmd)
			rm.mutex.Unlock()

			rm.logger.Debugf("缓存复制命令: offset=%d, hash=%s, 数据库索引=%d",
				cmdOffset, cmdHash[:8], dbIndex)

			// 仍然更新偏移量并发送确认消息
			rm.mutex.Lock()
			rm.state.ReplicaOffset = cmdOffset + int64(len(actualCmdData))
			offset := rm.state.ReplicaOffset
			rm.mutex.Unlock()

			// 发送确认消息
			offsetStr := fmt.Sprintf("%d", offset)
			return offsetStr, nil
		}

		// 标记该命令已处理
		rm.processedCmds.Store(cmdHash, time.Now().UnixNano())

		// 记录处理信息
		cmdPreview := ""
		if len(actualCmdData) > 20 {
			cmdPreview = string(actualCmdData[:20]) + "..."
		} else {
			cmdPreview = string(actualCmdData)
		}
		rm.logger.Debugf("处理复制命令: cmd=%s, hash=%s, 数据库索引=%d, 偏移量=%d",
			cmdPreview, cmdHash[:8], dbIndex, cmdOffset)

		// 解析RESP格式命令
		parser := protocol2.NewRESPCommandParser()
		cmd, err := parser.Parse(actualCmdData, "master")
		if err != nil {
			rm.logger.Errorf("解析复制命令失败: %v", err)
			return nil, err
		}

		// 执行命令
		rm.logger.Debugf("从主节点接收到命令: %s %v, 数据库索引: %d", cmd.Name(), cmd.Args(), dbIndex)

		// 检查是否是FLUSHALL或FLUSHDB命令
		cmdName := strings.ToUpper(cmd.Name())
		if cmdName == "FLUSHALL" {
			// 直接调用引擎的FlushAll方法
			if rm.engine != nil {
				rm.engine.FlushAll()
				rm.logger.Info("执行复制的FLUSHALL命令")
			} else {
				rm.logger.Error("无法执行FLUSHALL：引擎为nil")
			}
		} else if cmdName == "FLUSHDB" {
			// 对于FLUSHDB，需要选择正确的数据库然后清空
			flushDbIndex := dbIndex // 默认使用解析出的数据库索引
			if len(cmd.Args()) > 0 {
				// 如果命令带有参数，尝试解析为数据库索引
				if index, err := strconv.Atoi(cmd.Args()[0]); err == nil {
					flushDbIndex = index
				}
			}

			// 选择对应的数据库并清空
			if rm.engine != nil {
				db, err := rm.engine.Select(flushDbIndex)
				if err != nil {
					rm.logger.Errorf("选择数据库失败: %v", err)
				} else {
					// 执行清空操作
					db.Flush()
					rm.logger.Infof("执行复制的FLUSHDB命令，数据库索引: %d", flushDbIndex)
				}
			} else {
				rm.logger.Error("无法执行FLUSHDB：引擎为nil")
			}
		} else {
			// 选择正确的数据库
			db, err := rm.engine.Select(dbIndex)
			if err != nil {
				rm.logger.Errorf("选择数据库失败: %v", err)
				return nil, err
			}

			// 处理其他命令
			reply := db.ProcessCommand(cmd)
			if reply.Type() == cache2.ReplyError {
				rm.logger.Errorf("执行复制命令失败: %s", reply.String())
			}
		}

		// 更新复制偏移量
		rm.mutex.Lock()
		rm.state.ReplicaOffset = cmdOffset + int64(len(actualCmdData))
		rm.mutex.Unlock()

		// 发送确认消息
		return rm.state.ReplicaOffset, nil
	}

	// 注册客户端请求主节点地址的消息处理器
	getMasterInfo := func(connID string, msg *protocol3.CustomMessage) (interface{}, error) {
		// 获取当前主节点信息
		var masterAddr string

		if rm.state.Role == RoleMaster {
			// 如果当前节点就是主节点，直接返回自己的地址
			masterAddr = fmt.Sprintf("%s:%d", rm.config.Host, rm.config.Port)
		} else {
			// 从本地缓存的主节点信息获取
			rm.mutex.RLock()
			masterHost := rm.state.MasterHost
			masterPort := rm.state.MasterPort
			rm.mutex.RUnlock()

			if masterHost != "" && masterPort > 0 {
				masterAddr = fmt.Sprintf("%s:%d", masterHost, masterPort)
			} else {
				// 如果本地没有缓存，尝试从服务发现获取
				masterInfo, err := rm.discovery.FindMaster()
				if err == nil && masterInfo.Host != "" && masterInfo.Port > 0 {
					masterAddr = fmt.Sprintf("%s:%d", masterInfo.Host, masterInfo.Port)

					// 更新本地缓存
					rm.mutex.Lock()
					rm.state.MasterHost = masterInfo.Host
					rm.state.MasterPort = masterInfo.Port
					rm.mutex.Unlock()
				} else {
					// 如果找不到主节点，返回错误信息
					masterAddr = "UNKNOWN"
				}
			}
		}

		// 保存连接，以便主节点变更时通知
		// 获取连接对象
		conn, ok := rm.protocolMgr.GetConnection(connID)
		if ok && conn != nil {
			rm.masterClients.Store(connID, conn)

			// 注册连接关闭事件处理器，在客户端断开连接时移除
			rm.registerConnectionCloseHandler(conn)

			rm.logger.Debugf("客户端[%s]请求主节点信息: %s", connID, masterAddr)
		}

		// 发送主节点地址信息给客户端
		return masterAddr, nil
	}
	replHandler.RegisterHandle(ServiceTypeReplication, MsgTypeReplCommand, replCmd)
	replHandler.RegisterHandle(ServiceTypeReplication, MsgTypeReplACK, replAckHandle)
	replHandler.RegisterHandle(ServiceTypeReplication, MsgTypeReplData, replDataHandle)
	replHandler.RegisterHandle(ServiceTypeReplication, MsgTypeReplSyncResp, syncRespHandle)
	replHandler.RegisterHandle(ServiceTypeReplication, MsgTypeReplSync, syncHandle)
	replHandler.RegisterHandle(ServiceTypeReplication, MsgTypeGetMasterInfo, getMasterInfo)

}

// connectToMaster 连接到主节点
func (rm *DefaultReplicationManager) connectToMaster() error {
	// 添加锁，防止并发调用
	rm.mutex.Lock()

	// 检查是否已经连接到主节点
	if rm.state.ConnectedToMaster && rm.state.MasterConn != nil {
		rm.logger.Info("已经连接到主节点，跳过本次连接")
		rm.mutex.Unlock()
		return nil // 已经连接到主节点，直接返回
	}

	// 检查是否已经在连接中
	if rm.state.ConnectingToMaster {
		rm.logger.Info("正在连接主节点，跳过本次连接")
		rm.mutex.Unlock()
		return nil // 正在连接中，直接返回
	}

	// 标记正在连接
	rm.state.ConnectingToMaster = true
	rm.mutex.Unlock()

	// 在连接主节点前，先通知引擎清空本地数据
	if rm.engine != nil {
		rm.engine.FlushAll() // 假设引擎有这个方法
		rm.logger.Info("已清空本地数据库，准备从主节点同步数据")
	}

	// 查找主节点，带重试
	var masterInfo ServiceInfo
	var err error

	// 重试最多5次，增加重试间隔
	for retries := 0; retries < 10; retries++ {
		masterInfo, err = rm.discovery.FindMaster()
		if err == nil && masterInfo.Host != "" && masterInfo.Port > 0 {
			break
		}

		if retries < 9 {
			waitTime := time.Duration(retries+1) * 500 * time.Millisecond
			rm.logger.Warnf("查找主节点失败，将在 %v 后重试 (%d/10): %v", waitTime, retries+1, err)
			time.Sleep(waitTime)
		} else {
			rm.logger.Errorf("查找主节点失败，已重试10次: %v", err)
			return fmt.Errorf("查找主节点失败: %v", err)
		}
	}

	// 确保主节点信息有效
	if masterInfo.Host == "" || masterInfo.Port <= 0 {
		return fmt.Errorf("主节点信息无效: %+v", masterInfo)
	}

	// 更新主节点信息
	rm.mutex.Lock()
	rm.state.MasterHost = masterInfo.Host
	rm.state.MasterPort = masterInfo.Port
	rm.state.ConnectedToMaster = false // 重置连接状态，允许重新连接
	if rm.state.MasterConn != nil {
		// 关闭旧的连接
		oldConn := rm.state.MasterConn
		rm.state.MasterConn = nil
		go oldConn.Close() // 异步关闭，避免阻塞
	}
	rm.mutex.Unlock()

	// 更新服务信息
	host, port := rm.protocolMgr.GetServerConfig()
	rm.registerService(host, port)

	// 使用主节点的协议端口连接，而不是缓存服务端口
	protocolPort := masterInfo.ProtocolPort
	if protocolPort <= 0 {
		// 如果协议端口未设置，回退到使用缓存服务端口
		protocolPort = masterInfo.Port
		rm.logger.Warnf("主节点未提供协议端口，回退使用缓存服务端口: %d", protocolPort)
	}

	// 打印更详细的连接信息用于诊断
	rm.logger.Infof("准备连接到主节点: %s:%d (协议端口: %d)", masterInfo.Host, masterInfo.Port, protocolPort)

	// 检查主节点地址是否是本机地址
	isLocalhost := false
	if masterInfo.Host == "localhost" || masterInfo.Host == "127.0.0.1" || masterInfo.Host == "0.0.0.0" {
		isLocalhost = true
		masterInfo.Host = "127.0.0.1" // 统一使用127.0.0.1
		rm.logger.Infof("主节点是本机地址，使用127.0.0.1进行连接")
	}

	// 构建连接地址
	masterReplAddr := fmt.Sprintf("%s:%d", masterInfo.Host, protocolPort)
	rm.logger.Infof("连接到主节点，最终地址: %s", masterReplAddr)

	// 设置连接选项
	options := &network2.Options{
		ReadTimeout:       60 * time.Second,  // 增加读取超时时间
		WriteTimeout:      30 * time.Second,  // 设置写入超时时间
		IdleTimeout:       120 * time.Second, // 设置空闲超时时间
		EnableKeepAlive:   true,
		HeartbeatInterval: 20 * time.Second, // 设置心跳间隔
		MaxConnections:    100,              // 设置最大连接数
		BufferSize:        10 * 1024 * 1024, // 设置缓冲区大小
	}

	// 使用协议管理器连接到主节点的协议端口
	var conn *network2.Connection

	// 增加连接重试逻辑
	for retries := 0; retries < 5; retries++ {
		rm.logger.Infof("开始连接主节点: %s (尝试 %d/5)", masterReplAddr, retries+1)
		conn, err = rm.protocolMgr.Connect(masterReplAddr, options)
		if err == nil {
			break // 连接成功
		}

		rm.logger.Errorf("连接主节点失败: %v", err)

		if retries < 4 {
			waitTime := time.Duration(retries+1) * 500 * time.Millisecond
			rm.logger.Infof("将在 %v 后重试", waitTime)
			time.Sleep(waitTime)
		}
	}

	// 如果所有尝试都失败，返回最后的错误
	if err != nil {
		// 如果是本地地址，尝试不同的地址进行连接
		if !isLocalhost && (err.Error() == "max connections reached" || err.Error() == "connection refused") {
			// 如果不是本地地址，尝试用127.0.0.1替代
			altAddr := fmt.Sprintf("127.0.0.1:%d", protocolPort)
			rm.logger.Infof("尝试使用替代地址连接主节点: %s", altAddr)
			for retries := 0; retries < 3; retries++ {
				conn, err = rm.protocolMgr.Connect(altAddr, options)
				if err == nil {
					break // 连接成功
				}

				if retries < 2 {
					waitTime := time.Duration(retries+1) * 500 * time.Millisecond
					rm.logger.Warnf("使用替代地址连接失败: %v, 将在 %v 后重试", err, waitTime)
					time.Sleep(waitTime)
				} else {
					rm.logger.Errorf("使用替代地址连接主节点也失败: %v", err)
				}
			}
		}

		// 如果仍然失败，返回错误
		if err != nil {
			return err
		}
	}

	// 保存主节点连接
	rm.mutex.Lock()
	rm.state.MasterConn = conn
	rm.mutex.Unlock()

	// 注册主节点连接关闭事件处理器
	go func() {
		// 创建一个通道用于检测连接关闭
		closeCh := make(chan struct{})

		// 创建一个定时器，定期检查连接状态
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 检查连接是否仍然存在
				if _, ok := rm.protocolMgr.GetConnection(conn.ID()); !ok {
					// 连接已关闭，重置连接状态
					rm.mutex.Lock()
					if rm.state.MasterConn != nil && rm.state.MasterConn.ID() == conn.ID() {
						rm.state.MasterConn = nil
						rm.state.ConnectedToMaster = false
						rm.logger.Warnf("主节点连接已断开")
					}
					rm.mutex.Unlock()
					close(closeCh)
					return
				}
			case <-rm.stopChan:
				return
			case <-closeCh:
				return
			}
		}
	}()

	// 发送同步请求
	// 创建SYNC消息
	syncPayload := []byte(fmt.Sprintf("%s:%d", rm.config.Host, rm.config.Port))

	// 添加发送重试逻辑
	var syncErr error
	for retries := 0; retries < 3; retries++ {
		syncErr = rm.protocolMgr.SendMessage(conn.ID(), MsgTypeReplSync, ServiceTypeReplication, syncPayload)
		if syncErr == nil {
			break // 发送成功
		}

		rm.logger.Errorf("发送同步请求失败: %v, 尝试 %d/3", syncErr, retries+1)
		if retries < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if syncErr != nil {
		rm.logger.Errorf("发送同步请求失败，放弃尝试: %v", syncErr)
		conn.Close()
		return syncErr
	}

	// 标记已连接到主节点
	rm.mutex.Lock()
	rm.state.ConnectedToMaster = true
	rm.mutex.Unlock()

	rm.logger.Infof("向主节点发送同步请求成功")
	return nil
}

// fullSync 执行全量同步
func (rm *DefaultReplicationManager) fullSync(slave *SlaveInfo) {
	// 标记全量同步开始
	rm.mutex.Lock()
	rm.state.SyncInProgress = true
	rm.state.SyncStartOffset = rm.state.ReplicaOffset
	syncStartOffset := rm.state.SyncStartOffset
	rm.state.BufferedCmds = make([]*ReplCommand, 0) // 清空命令缓冲
	rm.mutex.Unlock()

	rm.logger.Infof("开始全量同步，初始偏移量: %d", syncStartOffset)

	defer func() {
		// 全量同步结束后处理缓存的命令
		rm.mutex.Lock()
		syncInProgress := rm.state.SyncInProgress
		rm.state.SyncInProgress = false
		bufferedCmds := rm.state.BufferedCmds
		rm.state.BufferedCmds = make([]*ReplCommand, 0) // 清空命令缓冲
		rm.mutex.Unlock()

		// 处理在全量同步期间接收到的命令
		if syncInProgress {
			rm.processSyncBufferedCommands(slave, bufferedCmds)
		}
	}()

	// 获取当前主节点偏移量
	rm.mutex.RLock()
	currentOffset := rm.state.ReplicaOffset
	rm.mutex.RUnlock()

	// 创建临时文件来存储RDB数据
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "temp_rdb_*.rdb")
	if err != nil {
		rm.logger.Errorf("创建临时RDB文件失败: %v", err)
		return
	}
	tempFilePath := tempFile.Name()
	_ = tempFile.Close()          // 关闭文件，后面会重新打开
	defer os.Remove(tempFilePath) // 函数结束时删除临时文件

	rm.logger.Infof("开始从内存引擎导出RDB数据到临时文件: %s", tempFilePath)

	// 创建用于同步等待的WaitGroup
	var wg sync.WaitGroup
	wg.Add(1)

	// 为RDB管理器创建配置
	rdbConfig := *rm.config
	rdbConfig.EnableRDB = true
	rdbConfig.RDBFilePath = tempFilePath

	// 遍历所有数据库，收集数据
	allDBData := make(map[int]map[string]cache2.Value)
	allDBExpires := make(map[int]map[string]time.Time)

	// 获取数据库数量
	dbCount := rm.config.DBCount
	if dbCount <= 0 {
		dbCount = 16 // 默认16个数据库
	}

	// 遍历所有数据库，收集数据
	for dbIndex := 0; dbIndex < dbCount; dbIndex++ {
		db, err := rm.engine.Select(dbIndex)
		if err != nil {
			rm.logger.Errorf("选择数据库 %d 失败: %v", dbIndex, err)
			continue
		}

		// 尝试将db转换为MemoryDatabase以确保类型正确
		_, ok := db.(*engine2.MemoryDatabase)
		if !ok {
			rm.logger.Errorf("数据库 %d 不是MemoryDatabase类型", dbIndex)
			continue
		}

		// 获取数据库大小
		size := db.Size()
		if size == 0 {
			// 跳过空数据库
			continue
		}

		// 获取所有键值对
		keys := db.Keys("*")
		data := make(map[string]cache2.Value)
		expires := make(map[string]time.Time)

		for _, key := range keys {
			value, exists := db.Get(key)
			if !exists {
				continue
			}

			// 存储值（使用深拷贝以防止数据竞争）
			data[key] = value.DeepCopy()

			// 获取过期时间
			ttl := db.TTL(key)
			if ttl > 0 {
				expires[key] = time.Now().Add(ttl)
			}
		}

		// 存储这个数据库的数据
		if len(data) > 0 {
			allDBData[dbIndex] = data
			allDBExpires[dbIndex] = expires
		}
	}

	// 统计数据
	var totalKeys int
	for _, data := range allDBData {
		totalKeys += len(data)
	}

	// 打开临时文件进行写入
	file, err := os.Create(tempFilePath)
	if err != nil {
		rm.logger.Errorf("创建RDB临时文件失败: %v", err)
		return
	}

	// 写入RDB文件头部
	header := "REDIS001" // 8字节的头部
	if _, err := file.WriteString(header); err != nil {
		rm.logger.Errorf("写入RDB头部失败: %v", err)
		file.Close()
		return
	}

	// 遍历所有数据库，写入数据
	for dbIndex, data := range allDBData {
		expires := allDBExpires[dbIndex]

		// 写入数据库选择标记
		if err := binary.Write(file, binary.BigEndian, uint32(dbIndex)); err != nil {
			rm.logger.Errorf("写入数据库索引失败: %v", err)
			file.Close()
			return
		}

		// 写入键数量
		if err := binary.Write(file, binary.BigEndian, uint32(len(data))); err != nil {
			rm.logger.Errorf("写入键数量失败: %v", err)
			file.Close()
			return
		}

		// 写入数据
		now := time.Now()
		for key, value := range data {
			// 检查是否过期
			if expireTime, hasExpire := expires[key]; hasExpire && expireTime.Before(now) {
				continue
			}

			// 写入键长度和内容
			if err := binary.Write(file, binary.BigEndian, uint32(len(key))); err != nil {
				rm.logger.Errorf("写入键长度失败: %v", err)
				file.Close()
				return
			}
			if _, err := file.WriteString(key); err != nil {
				rm.logger.Errorf("写入键内容失败: %v", err)
				file.Close()
				return
			}

			// 写入值类型
			if err := binary.Write(file, binary.BigEndian, uint8(value.Type())); err != nil {
				rm.logger.Errorf("写入值类型失败: %v", err)
				file.Close()
				return
			}

			// 编码值
			encodedValue, err := value.Encode()
			if err != nil {
				rm.logger.Errorf("编码键 %s 的值失败: %v", key, err)
				file.Close()
				return
			}

			// 写入值长度和内容
			if err := binary.Write(file, binary.BigEndian, uint32(len(encodedValue))); err != nil {
				rm.logger.Errorf("写入值长度失败: %v", err)
				file.Close()
				return
			}
			if _, err := file.Write(encodedValue); err != nil {
				rm.logger.Errorf("写入值内容失败: %v", err)
				file.Close()
				return
			}

			// 写入过期时间（如果有）
			if expireTime, hasExpire := expires[key]; hasExpire {
				// 写入标记，表示有过期时间
				if err := binary.Write(file, binary.BigEndian, uint8(1)); err != nil {
					rm.logger.Errorf("写入过期标志失败: %v", err)
					file.Close()
					return
				}
				// 写入过期时间
				expireUnix := expireTime.UnixNano()
				if err := binary.Write(file, binary.BigEndian, expireUnix); err != nil {
					rm.logger.Errorf("写入过期时间失败: %v", err)
					file.Close()
					return
				}
			} else {
				// 写入标记，表示没有过期时间
				if err := binary.Write(file, binary.BigEndian, uint8(0)); err != nil {
					rm.logger.Errorf("写入过期标志失败: %v", err)
					file.Close()
					return
				}
			}
		}
	}

	// 写入文件尾部
	if _, err := file.WriteString("EOF"); err != nil {
		rm.logger.Errorf("写入RDB尾部失败: %v", err)
		file.Close()
		return
	}

	// 确保数据写入磁盘
	if err := file.Sync(); err != nil {
		rm.logger.Errorf("同步RDB文件失败: %v", err)
		file.Close()
		return
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		rm.logger.Errorf("关闭RDB文件失败: %v", err)
		return
	}

	// 读取临时文件数据
	rdbData, err := os.ReadFile(tempFilePath)
	if err != nil {
		rm.logger.Errorf("读取RDB文件失败: %v", err)
		return
	}

	rm.logger.Infof("RDB数据导出完成，总大小: %d 字节，共 %d 个键", len(rdbData), totalKeys)

	// 发送同步响应消息
	respPayload := []byte(fmt.Sprintf("FULLRESYNC %s %d", rm.state.ReplicationID, currentOffset))
	err = rm.protocolMgr.SendMessage(slave.ID, MsgTypeReplSyncResp, ServiceTypeReplication, respPayload)
	if err != nil {
		rm.logger.Errorf("发送同步响应失败: %v", err)
		return
	}

	// 发送RDB数据
	err = rm.protocolMgr.SendMessage(slave.ID, MsgTypeReplData, ServiceTypeReplication, rdbData)
	if err != nil {
		rm.logger.Errorf("发送RDB数据失败: %v", err)
		return
	}

	// 更新从节点偏移量和预期偏移量
	slave.Offset = currentOffset
	slave.ExpectedOffset = currentOffset + int64(len(rdbData))
	slave.LastACKTime = time.Now()

	// 更新从节点信息
	rm.state.Slaves.Store(slave.ID, slave)

	rm.logger.Infof("向从节点 %s 发送全量同步完成，共 %d 字节，当前偏移量: %d，预期偏移量: %d",
		slave.ID, len(rdbData), slave.Offset, slave.ExpectedOffset)
}

// broadcastCommand 广播命令到所有从节点
func (rm *DefaultReplicationManager) broadcastCommand(cmdBytes []byte) {
	// 反序列化命令，获取命令信息
	cmd, err := DeserializeReplCommand(cmdBytes)
	if err != nil {
		rm.logger.Errorf("反序列化命令失败: %v", err)
		return
	}

	// 增加复制偏移量
	rm.mutex.Lock()
	// 记录当前偏移量作为命令的偏移量
	cmd.Offset = rm.state.ReplicaOffset
	// 重新序列化命令（包含偏移量）
	cmdBytes = cmd.Serialize()
	// 更新复制偏移量（包含新序列化后的命令长度）
	currentOffset := rm.state.ReplicaOffset + int64(len(cmdBytes))
	rm.state.ReplicaOffset = currentOffset

	// 检查是否有正在进行全量同步的从节点
	if rm.state.SyncInProgress {
		// 缓存命令供全量同步后使用
		rm.state.BufferedCmds = append(rm.state.BufferedCmds, cmd)
		rm.logger.Debugf("在全量同步期间缓存命令: dbIndex=%d, offset=%d, 长度=%d字节",
			cmd.DbIndex, cmd.Offset, len(cmdBytes))
	}
	rm.mutex.Unlock()

	// 遍历所有从节点，仅向它们发送命令
	successCount := 0
	failCount := 0

	var count int

	rm.state.Slaves.Range(func(key, value interface{}) bool {
		count++
		slaveInfo, ok := value.(*SlaveInfo)
		if !ok || slaveInfo == nil || slaveInfo.Connection == nil {
			return true // 跳过无效的从节点
		}

		// 向该从节点发送命令
		err := rm.protocolMgr.SendMessage(slaveInfo.Connection.ID(), MsgTypeReplCommand, ServiceTypeReplication, cmdBytes)
		if err != nil {
			rm.logger.Warnf("向从节点 %s 发送命令失败: %v", slaveInfo.ID, err)
			failCount++
		} else {
			successCount++
			// 更新从节点的预期偏移量
			slaveInfo.ExpectedOffset = currentOffset
		}

		return true // 继续遍历下一个从节点
	})

	if failCount > 0 {
		rm.logger.Warnf("命令复制结果: 成功=%d, 失败=%d", successCount, failCount)
	} else if successCount > 0 {
		rm.logger.Debugf("命令已复制到 %d 个从节点", successCount)
	}
}

// handlePromotionToMaster 处理从节点升级为主节点
func (rm *DefaultReplicationManager) handlePromotionToMaster() {
	rm.logger.Info("节点已升级为主节点")
	// 生成新的复制ID
	rm.mutex.Lock()
	rm.state.ReplicationID = generateReplicationID()
	rm.mutex.Unlock()
}

// handleDemotionToSlave 处理主节点降级为从节点
func (rm *DefaultReplicationManager) handleDemotionToSlave() {
	rm.logger.Info("节点已降级为从节点")

	// 重置连接状态，以便可以连接到主节点
	rm.mutex.Lock()
	rm.state.ConnectedToMaster = false
	rm.mutex.Unlock()

	// 清空已处理命令记录，以便重新接收主节点的命令
	rm.processedCmds = sync.Map{}

	// 连接到新的主节点
	go rm.connectToMaster()
}

// handleMasterChange 处理主节点变更
func (rm *DefaultReplicationManager) handleMasterChange(masterInfo ServiceInfo) {
	// 只有从节点才需要处理主节点变更
	if rm.roleManager.GetRole() != RoleSlave {
		return
	}

	// 检查是否真的变更了
	rm.mutex.RLock()
	oldHost := rm.state.MasterHost
	oldPort := rm.state.MasterPort
	rm.mutex.RUnlock()

	if oldHost == masterInfo.Host && oldPort == masterInfo.Port {
		return
	}

	rm.logger.Infof("主节点变更: %s:%d -> %s:%d", oldHost, oldPort, masterInfo.Host, masterInfo.Port)

	// 获取当前订阅主节点信息的客户端数量
	subscribers := rm.GetMasterInfoSubscribers()
	if len(subscribers) > 0 {
		rm.logger.Infof("当前有 %d 个客户端订阅主节点信息变更", len(subscribers))
	}

	// 更新主节点信息并重新连接
	rm.mutex.Lock()
	rm.state.MasterHost = masterInfo.Host
	rm.state.MasterPort = masterInfo.Port
	rm.state.ConnectedToMaster = false // 重置连接状态，允许重新连接
	if rm.state.MasterConn != nil {
		// 关闭旧的连接
		oldConn := rm.state.MasterConn
		rm.state.MasterConn = nil
		go oldConn.Close() // 异步关闭，避免阻塞
	}
	rm.mutex.Unlock()

	// 重新连接到新的主节点
	go rm.connectToMaster()

	// 通知所有订阅主节点变更的客户端
	go rm.notifyMasterChange(masterInfo)
}

// notifyMasterChange 通知所有客户端主节点变更
func (rm *DefaultReplicationManager) notifyMasterChange(masterInfo ServiceInfo) {
	// 准备通知消息
	masterAddr := fmt.Sprintf("%s:%d", masterInfo.Host, masterInfo.Port)
	payload := []byte(masterAddr)

	// 计数器
	var notifiedCount int
	var failedCount int

	rm.logger.Infof("开始通知客户端主节点变更: %s", masterAddr)

	// 遍历所有保存的客户端连接并通知
	rm.masterClients.Range(func(key, value interface{}) bool {
		connID, ok := key.(string)
		if !ok {
			return true // 继续遍历
		}

		// 检查连接是否仍然有效
		conn, ok := rm.protocolMgr.GetConnection(connID)
		if !ok || conn == nil {
			rm.logger.Debugf("客户端连接已失效，跳过通知: %s", connID)
			rm.masterClients.Delete(connID)
			failedCount++
			return true
		}

		// 发送通知消息
		err := rm.protocolMgr.SendMessage(connID, MsgTypeMasterChanged, ServiceTypeReplication, payload)
		if err != nil {
			rm.logger.Warnf("通知客户端[%s]主节点变更失败: %v", connID, err)
			// 删除无效连接
			rm.masterClients.Delete(connID)
			failedCount++
		} else {
			rm.logger.Debugf("成功通知客户端[%s]主节点变更: %s", connID, masterAddr)
			notifiedCount++
		}

		return true // 继续遍历
	})

	if notifiedCount > 0 || failedCount > 0 {
		rm.logger.Infof("主节点变更通知完成: 成功=%d, 失败=%d", notifiedCount, failedCount)
	}
}

// generateReplicationID 生成复制ID
func generateReplicationID() string {
	// 生成一个40字符的随机字符串
	id := make([]byte, 40)
	for i := 0; i < 40; i++ {
		// 使用简单的随机生成，实际应该使用更安全的随机函数
		id[i] = byte('a' + (time.Now().UnixNano() % 26))
		time.Sleep(time.Nanosecond)
	}
	return string(id)
}

// SetMaster 设置主节点信息
func (rm *DefaultReplicationManager) SetMaster(host string, port int) error {
	rm.mutex.Lock()
	rm.state.MasterHost = host
	rm.state.MasterPort = port
	rm.mutex.Unlock()

	rm.logger.Infof("设置主节点: %s:%d", host, port)
	return nil
}

// startCommandCleanup 定期清理过期的命令记录
func (rm *DefaultReplicationManager) startCommandCleanup() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			expiredTime := time.Now().Add(-1 * time.Hour).UnixNano()
			var count int

			rm.processedCmds.Range(func(key, value interface{}) bool {
				timestamp, ok := value.(int64)
				if ok && timestamp < expiredTime {
					rm.processedCmds.Delete(key)
					count++
				}
				return true
			})

			if count > 0 {
				rm.logger.Infof("已清理 %d 条过期的命令记录", count)
			}
		case <-rm.stopChan:
			return
		}
	}
}

// registerConnectionCloseHandler 注册连接关闭事件处理器
func (rm *DefaultReplicationManager) registerConnectionCloseHandler(conn *network2.Connection) {
	// 客户端连接是否已经注册过关闭事件处理器，使用sync.Map避免重复注册
	if _, loaded := rm.processedCmds.LoadOrStore("conn_close_"+conn.ID(), true); loaded {
		return
	}

	// 使用goroutine监听连接关闭
	go func() {
		// 创建一个通道用于检测连接关闭
		closeCh := make(chan struct{})

		// 创建一个定时器，定期检查连接状态
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 检查连接是否仍然存在
				if _, ok := rm.protocolMgr.GetConnection(conn.ID()); !ok {
					// 连接已关闭，移除保存的客户端
					rm.masterClients.Delete(conn.ID())
					rm.processedCmds.Delete("conn_close_" + conn.ID())
					rm.logger.Debugf("客户端连接已关闭，移除主节点信息订阅: %s", conn.ID())
					close(closeCh)
					return
				}
			case <-rm.stopChan:
				return
			case <-closeCh:
				return
			}
		}
	}()
}

// GetConnectedSlaves 获取当前连接的从节点列表
func (rm *DefaultReplicationManager) GetConnectedSlaves() [][]string {
	slavesList := make([][]string, 0)

	rm.state.Slaves.Range(func(key, value interface{}) bool {
		if slaveInfo, ok := value.(*SlaveInfo); ok {
			// 检查连接是否仍然有效（不为nil且最近有活动）
			if slaveInfo.Connection != nil && time.Since(slaveInfo.LastACKTime) < time.Minute*5 {
				// 连接有效且最近有活动
				port := strconv.Itoa(slaveInfo.Port)
				slavesList = append(slavesList, []string{slaveInfo.Host, port})
			}
		}
		return true
	})

	return slavesList
}

// GetMasterAddr 获取主节点地址（格式：host:port）
func (rm *DefaultReplicationManager) GetMasterAddr() string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if rm.state.MasterHost != "" && rm.state.MasterPort > 0 {
		return fmt.Sprintf("%s:%d", rm.state.MasterHost, rm.state.MasterPort)
	}

	return ""
}

// processSyncBufferedCommands 处理全量同步期间缓存的命令
func (rm *DefaultReplicationManager) processSyncBufferedCommands(slave *SlaveInfo, commands []*ReplCommand) {
	if len(commands) == 0 {
		rm.logger.Infof("全量同步期间没有缓存命令需要处理")
		return
	}

	currentOffset := slave.Offset
	processedCount := 0
	skippedCount := 0

	// 按照偏移量排序命令
	// 注意：实际实现中可能需要确保命令已按偏移量排序
	for _, cmd := range commands {
		// 只处理RDB数据之后的命令（偏移量大于同步完成时的偏移量）
		if cmd.Offset <= currentOffset {
			skippedCount++
			continue
		}

		// 向从节点发送命令
		err := rm.protocolMgr.SendMessage(slave.ID, MsgTypeReplCommand, ServiceTypeReplication, cmd.Serialize())
		if err != nil {
			rm.logger.Errorf("发送缓存命令到从节点失败: %v", err)
			break
		}

		// 更新预期偏移量
		slave.ExpectedOffset = cmd.Offset + int64(len(cmd.Command))
		processedCount++
	}

	rm.logger.Infof("全量同步后处理缓存命令: 总数=%d, 处理=%d, 跳过=%d",
		len(commands), processedCount, skippedCount)
}

// startClientConnectionCleanup 定期清理无效的客户端连接
func (rm *DefaultReplicationManager) startClientConnectionCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var invalidCount int

			rm.masterClients.Range(func(key, value interface{}) bool {
				connID, ok := key.(string)
				if !ok {
					rm.masterClients.Delete(key)
					invalidCount++
					return true
				}

				// 检查连接是否仍然有效
				_, ok = rm.protocolMgr.GetConnection(connID)
				if !ok {
					rm.masterClients.Delete(connID)
					invalidCount++
					rm.logger.Debugf("清理无效的客户端连接: %s", connID)
				}

				return true
			})

			if invalidCount > 0 {
				rm.logger.Infof("已清理 %d 个无效的客户端连接", invalidCount)
			}
		case <-rm.stopChan:
			return
		}
	}
}

// GetMasterInfoSubscribers 获取订阅主节点信息的客户端列表
func (rm *DefaultReplicationManager) GetMasterInfoSubscribers() []string {
	var subscribers []string

	rm.masterClients.Range(func(key, value interface{}) bool {
		if connID, ok := key.(string); ok {
			// 检查连接是否仍然有效
			if _, ok := rm.protocolMgr.GetConnection(connID); ok {
				subscribers = append(subscribers, connID)
			}
		}
		return true
	})

	return subscribers
}
