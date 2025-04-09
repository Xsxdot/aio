// Package server 提供缓存服务器实现
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/internal/cache/engine"
	"github.com/xsxdot/aio/internal/cache/protocol"
	"github.com/xsxdot/aio/internal/cache/replication"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/network"
	protocolmgr "github.com/xsxdot/aio/pkg/protocol"
)

// Server 表示缓存服务器
type Server struct {
	config     *cache.Config
	engine     *engine.MemoryEngine
	netManager *network.ConnectionManager
	handler    *network.MessageHandler
	protocol   network.Protocol
	cmdChan    chan cache.Command
	closeChan  chan struct{}
	wg         sync.WaitGroup
	logger     *common.Logger
	done       bool
	status     consts.ComponentStatus
	restartMu  sync.Mutex
	mutex      sync.Mutex
	stats      struct {
		startTime                 time.Time
		totalConnectionsReceived  int64
		totalCommandsProcessed    int64
		instantaneousOpsPerSecond float64
		lastOpsCount              int64
		lastOpsTime               time.Time
	}
	// 连接ID到数据库索引的映射
	connDbIndexMap sync.Map
	// 已认证的连接ID集合
	authenticatedConns sync.Map
	// 主从复制相关组件
	roleManager        replication.RoleManager
	replicationManager replication.ReplicationManager
	commandInterceptor replication.CommandInterceptor
}

func (s *Server) RegisterMetadata() (bool, int, map[string]string) {
	return true, s.config.Port, map[string]string{
		"port": strconv.Itoa(s.config.Port),
	}
}

func (s *Server) Name() string {
	return consts.ComponentCacheServer
}

func (s *Server) Status() consts.ComponentStatus {
	return s.status
}

// GetClientConfig 实现Component接口，返回客户端配置
func (s *Server) GetClientConfig() (bool, *config.ClientConfig) {
	value := map[string]interface{}{
		"password": config.NewEncryptedValue(s.config.Password),
	}

	return true, config.NewClientConfig("redis", value)
}

// DefaultConfig 返回组件的默认配置
func (s *Server) DefaultConfig(config *config.BaseConfig) interface{} {
	return s.genConfig(config)
}

func (s *Server) genConfig(config *config.BaseConfig) *cache.Config {
	defaultConfig := cache.DefaultConfig()
	defaultConfig.Host = config.Network.BindIP
	defaultConfig.NodeID = config.System.NodeId
	defaultConfig.RDBFilePath = filepath.Join(config.System.DataDir, defaultConfig.RDBFilePath, "rdb", fmt.Sprintf("%s.rdb", config.System.NodeId))
	defaultConfig.AOFFilePath = filepath.Join(config.System.DataDir, defaultConfig.AOFFilePath, "aof", fmt.Sprintf("%s.aof", config.System.NodeId))
	return &defaultConfig
}

func (s *Server) Init(config *config.BaseConfig, body []byte) error {
	defaultConfig := s.genConfig(config)
	s.config = defaultConfig

	if err := json.Unmarshal(body, &defaultConfig); err != nil {
		return err
	}

	s.status = consts.StatusInitialized

	return nil
}

func (s *Server) Restart(ctx context.Context) error {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()
	err := s.Stop(ctx)
	if err != nil {
		return err
	}
	return s.Start(ctx)
}

// ServerOption 服务器选项
type ServerOption func(*Server)

// WithReplication 设置复制管理器
func WithReplication(electionClient election.Election, discoveryClient discovery.DiscoveryService, protocolMgr *protocolmgr.ProtocolManager) ServerOption {
	return func(s *Server) {
		// 创建选举适配器
		electionAdapter := replication.NewElectionAdapter(electionClient)
		// 创建角色管理器
		s.roleManager = replication.NewRoleManager(electionAdapter)
		// 创建服务发现适配器
		discoveryAdapter := replication.NewServiceDiscoveryAdapter(discoveryClient, "cache-service")

		// 如果已有角色管理器和引擎，创建复制管理器
		if s.roleManager != nil && s.engine != nil {
			// 获取协议管理器，如果server.protocol是protocolAdapter类型，则使用其中的manager
			s.replicationManager = replication.NewReplicationManager(
				s.engine,
				s.roleManager,
				discoveryAdapter,
				s.config,
				protocolMgr, // 传递协议管理器
			)

			// 注册角色变更监听器
			s.roleManager.RegisterListener(s.replicationManager)

			// 创建命令拦截器
			s.commandInterceptor = replication.NewCommandInterceptor(
				s.roleManager,
				s.replicationManager,
			)
		}
	}
}

// NewServer 创建新的缓存服务器
func NewServer(opts ...ServerOption) (*Server, error) {
	// 创建服务器
	srv := &Server{
		closeChan: make(chan struct{}),
		logger:    common.GetLogger(),
	}
	// 应用选项
	for _, opt := range opts {
		opt(srv)
	}

	return srv, nil
}

// StartWithConfig 使用指定配置启动服务器
func (s *Server) StartWithConfig() error {
	s.logger.Infof("缓存服务器启动, 监听地址: %s:%d", s.config.Host, s.config.Port)

	// 如果启用了复制功能，先启动角色管理器
	if s.roleManager != nil {
		if err := s.roleManager.Start(); err != nil {
			s.logger.Errorf("启动角色管理器失败: %v", err)
			return err
		}

		// 启动复制管理器
		if s.replicationManager != nil {
			if err := s.replicationManager.Start(); err != nil {
				s.logger.Errorf("启动复制管理器失败: %v", err)
				return err
			}
		}
	}

	// 启动网络服务
	err := s.netManager.StartServer(s.config.Host, s.config.Port)
	if err != nil {
		s.logger.Errorf("启动网络服务失败: %v", err)
		return err
	}

	// 等待服务器完全启动
	time.Sleep(100 * time.Millisecond)
	s.logger.Info("服务器已成功启动")

	// 打印服务器角色
	if s.roleManager != nil {
		role := s.roleManager.GetRole()
		s.logger.Infof("服务器当前角色: %s", role)
	}

	s.status = consts.StatusRunning

	return nil
}

// Start 使用默认配置启动服务器
func (s *Server) Start(context.Context) error {
	// 创建内存引擎
	memEngine := engine.NewMemoryEngine(*s.config)
	s.engine = memEngine

	// 创建RESP协议
	respProtocol := protocol.NewRESPProtocol()
	s.protocol = respProtocol

	// 创建命令通道
	cmdChan := make(chan cache.Command, 10000)
	s.cmdChan = cmdChan

	// 初始化统计信息
	s.stats.startTime = time.Now()
	s.stats.lastOpsTime = time.Now()

	// 创建消息处理器
	handler := &network.MessageHandler{
		Handle: s.handleMessage,
		GetHeartbeat: func() network.Message {
			return protocol.NewRESPMessage(protocol.MessageHeartbeat, []byte("+PING\r\n"))
		},
	}

	// 创建连接管理器选项
	options := &network.Options{
		MaxConnections:    s.config.MaxClients,
		ReadTimeout:       time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(s.config.WriteTimeout) * time.Second,
		HeartbeatInterval: time.Duration(s.config.HeartbeatTimeout) * time.Second,
	}

	// 创建连接管理器
	s.netManager = network.NewConnectionManager(respProtocol, handler, options)
	s.handler = handler

	// 启动命令处理协程
	s.wg.Add(1)
	go s.processCommands()

	// 启动统计协程
	s.wg.Add(1)
	go s.updateStats()

	// 启动连接监控协程，用于清理断开连接的状态
	s.wg.Add(1)
	go s.monitorConnections()
	return s.StartWithConfig()
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	s.mutex.Lock()
	if s.done {
		s.mutex.Unlock()
		return nil
	}
	s.done = true
	s.status = consts.StatusStopped
	s.mutex.Unlock()

	s.logger.Info("服务器正在关闭...")

	// 关闭复制管理器
	if s.replicationManager != nil {
		if err := s.replicationManager.Stop(); err != nil {
			s.logger.Errorf("关闭复制管理器失败: %v", err)
		}
	}

	// 关闭角色管理器
	if s.roleManager != nil {
		if err := s.roleManager.Stop(); err != nil {
			s.logger.Errorf("关闭角色管理器失败: %v", err)
		}
	}

	// 关闭所有连接
	if err := s.netManager.Close(); err != nil {
		s.logger.Errorf("关闭连接管理器失败: %v", err)
	}

	// 关闭命令通道并通知协程退出
	close(s.closeChan)

	// 等待所有协程结束
	s.wg.Wait()

	// 关闭引擎
	if err := s.engine.Close(); err != nil {
		s.logger.Errorf("关闭缓存引擎失败: %v", err)
		return err
	}

	s.logger.Info("服务器已关闭")
	return nil
}

// GetStats 获取服务器统计信息
func (s *Server) GetStats() map[string]interface{} {
	engineStats := s.engine.GetStats()

	// 合并引擎统计和服务器统计
	stats := make(map[string]interface{})
	for k, v := range engineStats {
		stats[k] = v
	}

	// 添加服务器特有的统计
	stats["uptime_in_seconds"] = int64(time.Since(s.stats.startTime).Seconds())
	stats["total_connections_received"] = s.stats.totalConnectionsReceived
	stats["total_commands_processed"] = s.stats.totalCommandsProcessed
	stats["instantaneous_ops_per_sec"] = s.stats.instantaneousOpsPerSecond
	stats["connected_clients"] = s.netManager.GetConnectionCount()

	return stats
}

// handleMessage 处理接收到的网络消息
func (s *Server) handleMessage(conn *network.Connection, data []byte) error {
	// 更新连接计数
	s.stats.totalConnectionsReceived++

	// 检查是否是PING命令的特殊处理
	if len(data) >= 1 && data[0] == protocol.SimpleString && bytes.HasPrefix(data, []byte("+PING\r\n")) {
		// 特殊格式的PING命令
		// 检查是否需要认证
		if s.config.Password != "" {
			if _, authenticated := s.authenticatedConns.Load(conn.ID()); !authenticated {
				// 未认证，返回错误
				errReply := protocol.NewErrorReply("NOAUTH Authentication required.")
				return conn.Send(errReply.ToMessage())
			}
		}
		// 直接回复PONG
		pongReply := protocol.NewSimpleStringReply("PONG")
		return conn.Send(pongReply.ToMessage())
	}

	// 将网络消息转换为命令
	parser := protocol.NewRESPCommandParser()
	cmd, err := parser.Parse(data, conn.ID())
	if err != nil {
		s.logger.Errorf("解析命令失败: %v", err)
		errReply := protocol.NewErrorReply(fmt.Sprintf("ERR %v", err))
		return conn.Send(errReply.ToMessage())
	}

	// 获取命令名并转换为大写以进行不区分大小写的比较
	cmdName := strings.ToUpper(cmd.Name())

	// 设置命令的数据库索引
	// 从连接数据库映射中获取数据库索引
	dbIndex := 0 // 默认为0
	if value, ok := s.connDbIndexMap.Load(conn.ID()); ok {
		if idx, ok := value.(int); ok {
			dbIndex = idx
		}
	}

	// 设置数据库索引到命令对象
	cmd.SetDbIndex(dbIndex)

	// 如果配置了密码，检查客户端是否已认证
	// 除了AUTH命令外，其他命令都需要认证
	if s.config.Password != "" && cmdName != "AUTH" {
		// 检查连接是否已认证
		if _, authenticated := s.authenticatedConns.Load(conn.ID()); !authenticated {
			// 未认证，返回错误
			errReply := protocol.NewErrorReply("NOAUTH Authentication required.")
			return conn.Send(errReply.ToMessage())
		}
	}

	// 对PING命令的特殊处理 (现在已经通过认证检查)
	if cmdName == "PING" {
		pongReply := protocol.NewSimpleStringReply("PONG")
		return conn.Send(pongReply.ToMessage())
	}

	// 如果有命令拦截器，先检查是否需要拦截
	if s.commandInterceptor != nil && s.commandInterceptor.ShouldIntercept(cmd) {
		reply := s.commandInterceptor.Process(cmd)
		if reply != nil {
			return conn.Send(reply.ToMessage())
		}
	}

	// 对特定命令的特殊处理
	switch cmdName {
	case "INFO":
		// 生成INFO信息
		reply := s.execInfo(cmd.Args())
		return conn.Send(reply.ToMessage())
	case "FLUSHALL":
		reply := s.execFlushAll(cmd.Args())
		return conn.Send(reply.ToMessage())
	case "SELECT":
		// 处理SELECT命令，将结果发送给客户端
		reply := s.execSelect(cmd.Args())
		err = conn.Send(reply.ToMessage())
		if err != nil {
			return err
		}

		// 如果SELECT成功，保存连接ID和数据库索引的关系
		if _, ok := reply.(*protocol.RESPReply); ok && reply.String() == "OK" {
			if clientID := cmd.ClientID(); clientID != "" {
				dbIndex, _ := parseDBIndex(cmd.Args()[0])
				s.connDbIndexMap.Store(clientID, dbIndex)

				// 更新命令对象中的数据库索引，确保命令中的索引与最新的一致
				cmd.SetDbIndex(dbIndex)
			}
		}
		return nil
	case "ROLE":
		// 处理ROLE命令，返回节点角色信息
		return s.execRole(conn)
	case "SLAVEOF":
		// 处理SLAVEOF命令，设置主从复制关系
		reply := s.execSlaveOf(cmd.Args())
		return conn.Send(reply.ToMessage())
	case "AUTH":
		// 处理AUTH命令
		reply := s.execAuth(cmd.ClientID(), cmd.Args())
		return conn.Send(reply.ToMessage())
	}

	// 发送到命令通道处理
	s.cmdChan <- cmd

	return nil
}

// processCommands 处理命令通道中的命令
func (s *Server) processCommands() {
	defer s.wg.Done()

	for {
		select {
		case <-s.closeChan:
			return
		case cmd := <-s.cmdChan:
			// 更新命令计数
			s.stats.totalCommandsProcessed++
			s.stats.lastOpsCount++

			// 处理命令
			reply := s.executeCommand(cmd)

			// 查找连接并发送回复
			if conn, ok := s.netManager.GetConnection(cmd.ClientID()); ok {
				if err := conn.Send(reply.ToMessage()); err != nil {
					s.logger.Errorf("发送回复失败: %v", err)
				}
			} else {
				s.logger.Warnf("未找到客户端连接: %s", cmd.ClientID())
			}
		}
	}
}

// executeCommand 执行命令
func (s *Server) executeCommand(cmd cache.Command) cache.Reply {
	// 使用指标中间件处理命令
	ctx := context.Background()
	reply, err := s.Handle(ctx, cmd)
	if err != nil {
		return protocol.NewErrorReply(fmt.Sprintf("ERR %v", err))
	}
	return reply
}

// execSelect 处理SELECT命令
func (s *Server) execSelect(args []string) cache.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'select' command")
	}

	dbIndex, err := parseDBIndex(args[0])
	if err != nil {
		return protocol.NewErrorReply(fmt.Sprintf("ERR %v", err))
	}

	_, err = s.engine.Select(dbIndex)
	if err != nil {
		return protocol.NewErrorReply(fmt.Sprintf("ERR %v", err))
	}

	// 成功选择数据库，但不在这里存储客户端ID和数据库索引的关系
	// 这个操作在handleMessage中处理，因为那里有命令的ClientID

	return protocol.NewStatusReply("OK")
}

// execInfo 处理INFO命令
func (s *Server) execInfo(args []string) cache.Reply {
	stats := s.GetStats()

	// 构建INFO响应文本
	var infoText strings.Builder
	infoText.WriteString("# Server\r\n")
	infoText.WriteString(fmt.Sprintf("uptime_in_seconds:%v\r\n", stats["uptime_in_seconds"]))
	infoText.WriteString(fmt.Sprintf("connected_clients:%v\r\n", stats["connected_clients"]))
	infoText.WriteString(fmt.Sprintf("instantaneous_ops_per_sec:%v\r\n", stats["instantaneous_ops_per_sec"]))
	infoText.WriteString(fmt.Sprintf("total_connections_received:%v\r\n", stats["total_connections_received"]))
	infoText.WriteString(fmt.Sprintf("total_commands_processed:%v\r\n", stats["total_commands_processed"]))

	// 添加认证信息
	if s.config.Password != "" {
		infoText.WriteString("requirepass:yes\r\n")
	} else {
		infoText.WriteString("requirepass:no\r\n")
	}

	// 添加复制信息
	if s.replicationManager != nil && s.roleManager != nil {
		role := s.roleManager.GetRole()
		infoText.WriteString(fmt.Sprintf("role:%s\r\n", role))

		if role == replication.RoleMaster {
			// 获取连接的从节点信息
			slaveInfos := s.replicationManager.GetConnectedSlaves()
			infoText.WriteString(fmt.Sprintf("connected_slaves:%d\r\n", len(slaveInfos)))

			// 添加每个从节点的详细信息
			for i, slave := range slaveInfos {
				infoText.WriteString(fmt.Sprintf("slave%d:ip=%s,port=%s,state=online,offset=0\r\n",
					i, slave[0], slave[1]))
			}
		} else if role == replication.RoleSlave {
			// 添加主节点信息
			masterAddr := s.replicationManager.GetMasterAddr()
			if masterAddr != "" {
				parts := strings.Split(masterAddr, ":")
				if len(parts) == 2 {
					infoText.WriteString(fmt.Sprintf("master_host:%s\r\n", parts[0]))
					infoText.WriteString(fmt.Sprintf("master_port:%s\r\n", parts[1]))
					infoText.WriteString("master_link_status:up\r\n")
				}
			}
		}
	}

	infoText.WriteString("\r\n# Memory\r\n")
	infoText.WriteString(fmt.Sprintf("used_memory:%v\r\n", stats["used_memory"]))

	infoText.WriteString("\r\n# Stats\r\n")
	if keyspace, ok := stats["keyspace"].(map[string]interface{}); ok {
		infoText.WriteString("\r\n# Keyspace\r\n")
		for db, info := range keyspace {
			if dbInfo, ok := info.(map[string]interface{}); ok {
				infoText.WriteString(fmt.Sprintf("%s:keys=%v\r\n", db, dbInfo["keys"]))
			}
		}
	}

	return protocol.NewBulkReply(infoText.String())
}

// execFlushAll 处理FLUSHALL命令
func (s *Server) execFlushAll(args []string) cache.Reply {
	// 调用引擎的FlushAll方法清除所有数据库中的所有键
	s.engine.FlushAll()

	// 记录操作
	s.logger.Info("执行FLUSHALL命令，清除所有数据库")

	return protocol.NewStatusReply("OK")
}

// execShutdown 处理SHUTDOWN命令
func (s *Server) execShutdown(args []string) cache.Reply {
	// 异步关闭服务器
	go func() {
		time.Sleep(time.Millisecond * 100) // 给回复一些发送的时间
		if err := s.Stop(nil); err != nil {
			s.logger.Errorf("关闭服务器失败: %v", err)
		}
	}()

	return protocol.NewStatusReply("OK")
}

// updateStats 定期更新统计信息
func (s *Server) updateStats() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeChan:
			return
		case now := <-ticker.C:
			elapsed := now.Sub(s.stats.lastOpsTime).Seconds()
			s.stats.instantaneousOpsPerSecond = float64(s.stats.lastOpsCount) / elapsed
			s.stats.lastOpsCount = 0
			s.stats.lastOpsTime = now
		}
	}
}

// 辅助函数

// parseDBIndex 解析数据库索引
func parseDBIndex(s string) (int, error) {
	var index int
	_, err := fmt.Sscanf(s, "%d", &index)
	if err != nil {
		return 0, fmt.Errorf("invalid DB index: %s", s)
	}
	if index < 0 {
		return 0, fmt.Errorf("DB index cannot be negative: %d", index)
	}
	return index, nil
}

// RegisterMessageHandler 注册自定义的消息处理函数
func (s *Server) RegisterMessageHandler(handler func(*network.Connection, []byte) error) {
	// 创建新的消息处理器
	msgHandler := &network.MessageHandler{
		Handle: handler,
		GetHeartbeat: func() network.Message {
			return protocol.NewRESPMessage(protocol.MessageHeartbeat, []byte("+PING\r\n"))
		},
	}

	// 更新处理器
	s.handler = msgHandler
}

// execRole 处理ROLE命令
func (s *Server) execRole(conn *network.Connection) error {
	var roleReply *protocol.RESPReply

	if s.replicationManager != nil {
		role := s.roleManager.GetRole()

		if role == replication.RoleMaster {
			// 返回主节点信息: master, replication_offset, slaves_array
			// Redis ROLE命令期望的格式为: master replication_offset connected_slaves
			values := []string{"master", "0"}

			// 添加从节点信息 (作为空字符串表示没有从节点，这样客户端会解析为空数组)
			// 注意: 实际实现中应该添加从节点信息，但为了简化，我们这里不添加

			roleReply = protocol.NewMultiBulkReply(values)
		} else if role == replication.RoleSlave {
			// 返回从节点信息: slave, master_host, master_port, replication_state, replication_offset
			masterAddr := s.replicationManager.GetMasterAddr()
			host := "127.0.0.1"
			port := "6379"

			if masterAddr != "" {
				parts := strings.Split(masterAddr, ":")
				if len(parts) == 2 {
					host = parts[0]
					port = parts[1]
				}
			}

			values := []string{"slave", host, port, "connected", "0"}
			roleReply = protocol.NewMultiBulkReply(values)
		} else {
			// 未知角色，返回单个元素数组
			roleReply = protocol.NewMultiBulkReply([]string{"none"})
		}
	} else {
		// 未启用复制功能，返回单个元素数组
		roleReply = protocol.NewMultiBulkReply([]string{"none"})
	}

	return conn.Send(roleReply.ToMessage())
}

// execSlaveOf 处理SLAVEOF命令
func (s *Server) execSlaveOf(args []string) cache.Reply {
	// 检查参数数量
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'slaveof' command")
	}

	// 检查是否启用了复制功能
	if s.replicationManager == nil || s.roleManager == nil {
		return protocol.NewErrorReply("ERR replication not enabled on this server")
	}

	host := args[0]
	portStr := args[1]

	// 检查是否是SLAVEOF NO ONE命令（断开复制）
	if strings.ToUpper(host) == "NO" && strings.ToUpper(portStr) == "ONE" {
		// 将节点角色设置为主节点
		s.roleManager.SetRole(replication.RoleMaster)
		return protocol.NewStatusReply("OK")
	}

	// 解析端口
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return protocol.NewErrorReply(fmt.Sprintf("ERR invalid port: %s", portStr))
	}

	// 设置主节点信息
	s.replicationManager.SetMaster(host, port)

	// 将节点角色设置为从节点
	s.roleManager.SetRole(replication.RoleSlave)

	// 开始同步
	go func() {
		if err := s.replicationManager.SyncFromMaster(); err != nil {
			s.logger.Errorf("同步主节点失败: %v", err)
		}
	}()

	return protocol.NewStatusReply("OK")
}

// execAuth 处理AUTH命令
func (s *Server) execAuth(clientID string, args []string) cache.Reply {
	// 检查参数数量
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'auth' command")
	}

	// 获取密码参数
	password := args[0]

	// 如果没有设置密码，返回错误
	if s.config.Password == "" {
		return protocol.NewErrorReply("ERR Client sent AUTH, but no password is set")
	}

	// 验证密码
	if password != s.config.Password {
		// 密码错误，记录警告日志
		s.logger.Warnf("客户端 %s 认证失败: 密码错误", clientID)
		return protocol.NewErrorReply("ERR invalid password")
	}

	// 认证成功，将客户端ID添加到已认证连接集合
	s.authenticatedConns.Store(clientID, true)
	s.logger.Infof("客户端 %s 认证成功", clientID)

	return protocol.NewStatusReply("OK")
}

// GetPort 返回服务器的实际监听端口
func (s *Server) GetPort() int {
	return s.config.Port
}

// monitorConnections 监控连接状态，清理断开连接的状态
func (s *Server) monitorConnections() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeChan:
			return
		case <-ticker.C:
			// 获取所有已认证连接的ID
			var connIDs []string
			s.authenticatedConns.Range(func(key, value interface{}) bool {
				connID, ok := key.(string)
				if ok {
					connIDs = append(connIDs, connID)
				}
				return true
			})

			// 检查每个连接是否仍然存在
			for _, connID := range connIDs {
				if _, ok := s.netManager.GetConnection(connID); !ok {
					// 连接已关闭，清理认证状态
					s.authenticatedConns.Delete(connID)
					s.connDbIndexMap.Delete(connID)
					s.logger.Debugf("检测到客户端断开连接，清理状态: %s", connID)
				}
			}
		}
	}
}

// Handle 实现middleware.Handler接口，处理缓存命令
func (s *Server) Handle(ctx context.Context, cmd cache.Command) (cache.Reply, error) {
	// 获取连接的数据库索引，默认使用数据库 0
	dbIndex := 0

	// 从映射中获取连接ID对应的数据库索引
	if clientID := cmd.ClientID(); clientID != "" {
		if value, ok := s.connDbIndexMap.Load(clientID); ok {
			if index, ok := value.(int); ok {
				dbIndex = index
			}
		}
	}

	// 获取数据库实例
	db, err := s.engine.Select(dbIndex)
	if err != nil {
		return protocol.NewErrorReply(fmt.Sprintf("ERR %v", err)), err
	}

	// 交由数据库处理命令
	return db.ProcessCommand(cmd), nil
}
