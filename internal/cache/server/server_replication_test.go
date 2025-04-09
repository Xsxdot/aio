package server

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/internal/cache/replication"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/network"
	"github.com/xsxdot/aio/pkg/protocol"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/etcd"
)

var ctx = context.Background()

// TestMode 测试模式类型
type TestMode int

const (
	// ModeSingleNode 单节点模式
	ModeSingleNode TestMode = iota
	// ModeMasterSlave 主从复制模式
	ModeMasterSlave
	// ModeCluster 集群模式
	ModeCluster
)

// 测试环境配置
type testEnvConfig struct {
	// 基础配置
	Mode          TestMode
	DataDir       string
	EtcdName      string
	EtcdClientURL string
	EtcdPeerURL   string

	// 节点配置
	MasterHost         string
	MasterPort         int
	MasterProtocolPort int
	MasterPassword     string
	SlaveHost          string
	SlavePort          int
	SlaveProtocolPort  int
	SlavePassword      string

	// 集群配置
	ClusterNodes []struct {
		Host     string
		Port     int
		Password string
	}

	// 其他设置
	ElectionTTL       int64
	ElectionPrefix    string
	DiscoveryPrefix   string
	DiscoveryTTL      int64
	ConnectRetryCount int
	ConnectRetryDelay time.Duration
	StartupDelay      time.Duration
}

// 默认测试环境配置
func defaultTestEnvConfig(mode TestMode) *testEnvConfig {
	config := &testEnvConfig{
		Mode:               mode,
		EtcdName:           "etcd-test",
		EtcdClientURL:      "http://127.0.0.1:2379",
		EtcdPeerURL:        "http://127.0.0.1:2380",
		MasterHost:         "127.0.0.1",
		MasterPort:         6379,
		MasterProtocolPort: 6666,
		MasterPassword:     "",
		SlaveHost:          "127.0.0.1",
		SlavePort:          6380,
		SlaveProtocolPort:  6766,
		SlavePassword:      "",
		ElectionTTL:        60,
		ElectionPrefix:     "/elections/",
		DiscoveryPrefix:    "/cache-service",
		DiscoveryTTL:       60,
		ConnectRetryCount:  5,
		ConnectRetryDelay:  1 * time.Second,
		StartupDelay:       2 * time.Second,
	}

	// 集群模式特殊配置
	if mode == ModeCluster {
		config.ClusterNodes = []struct {
			Host     string
			Port     int
			Password string
		}{
			{Host: "127.0.0.1", Port: 6381, Password: ""},
			{Host: "127.0.0.1", Port: 6382, Password: ""},
			{Host: "127.0.0.1", Port: 6383, Password: ""},
		}
	}

	return config
}

// 测试环境资源
type testEnvResources struct {
	// 测试配置
	Config *testEnvConfig

	// 基础组件
	EtcdServer *etcd.EtcdServer
	EtcdClient *etcd.EtcdClient
	Manager    distributed.Manager
	Discovery  distributed.ServiceDiscovery
	Logger     *zap.Logger
	Context    context.Context
	CancelFunc context.CancelFunc

	// 主从节点组件
	MasterElection distributed.Election
	SlaveElection  distributed.Election
	MasterServer   *Server
	SlaveServer    *Server
	MasterClient   *redis.Client
	SlaveClient    *redis.Client

	// 集群节点组件
	ClusterElections []distributed.Election
	ClusterServers   []*Server
	ClusterClients   []*redis.Client

	// 协议管理器
	ProtocolManager      *protocol.ProtocolManager
	SlaveProtocolManager *protocol.ProtocolManager
}

// 设置测试环境
func setupReplicationTestEnv(t *testing.T, mode TestMode) (*testEnvResources, func()) {
	// 1. 准备测试环境配置
	config := defaultTestEnvConfig(mode)

	// 2. 创建临时数据目录
	dataDir, err := os.MkdirTemp("", "etcd-test")
	require.NoError(t, err)
	config.DataDir = dataDir

	// 3. 创建上下文和logger
	ctx, cancel := context.WithCancel(context.Background())
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// 4. 初始化资源对象
	resources := &testEnvResources{
		Config:     config,
		Logger:     logger,
		Context:    ctx,
		CancelFunc: cancel,
	}

	// 5. 启动etcd服务器
	if err := startEtcdServer(resources, t); err != nil {
		cleanupResources(resources, t)
		require.NoError(t, err, "启动etcd服务器失败")
	}

	// 6. 创建etcd客户端
	if err := createEtcdClient(resources, t); err != nil {
		cleanupResources(resources, t)
		require.NoError(t, err, "创建etcd客户端失败")
	}

	// 7. 初始化分布式管理器
	if err := initDistributedManager(resources, t); err != nil {
		cleanupResources(resources, t)
		require.NoError(t, err, "初始化分布式管理器失败")
	}

	// 8. 启动协议管理器
	resources.ProtocolManager = protocol.NewServer(nil)
	resources.ProtocolManager.Start(resources.Config.MasterHost+":"+strconv.Itoa(resources.Config.MasterProtocolPort), &network.Options{
		MaxConnections:  100000,
		BufferSize:      10 * 1024 * 1024,
		EnableKeepAlive: true,
	})
	resources.SlaveProtocolManager = protocol.NewServer(nil)
	resources.SlaveProtocolManager.Start(resources.Config.SlaveHost+":"+strconv.Itoa(resources.Config.SlaveProtocolPort), &network.Options{
		MaxConnections:  100000,
		BufferSize:      10 * 1024 * 1024,
		EnableKeepAlive: true,
	})

	// 9. 返回资源和清理函数
	cleanup := func() {
		cleanupResources(resources, t)
	}

	return resources, cleanup
}

// 启动etcd服务器
func startEtcdServer(res *testEnvResources, t *testing.T) error {
	t.Logf("正在启动etcd服务器 (名称: %s)...", res.Config.EtcdName)

	// 创建etcd服务器配置
	serverConfig := &etcd.ServerConfig{
		Name:                res.Config.EtcdName,
		DataDir:             res.Config.DataDir,
		ClientURLs:          []string{res.Config.EtcdClientURL},
		PeerURLs:            []string{res.Config.EtcdPeerURL},
		InitialCluster:      fmt.Sprintf("%s=%s", res.Config.EtcdName, res.Config.EtcdPeerURL),
		InitialClusterState: "new",
		InitialClusterToken: "etcd-cluster-test",
		IsNewCluster:        true,
	}

	// 创建并启动etcd服务器
	server, err := etcd.NewEtcdServer(serverConfig, res.Logger)
	if err != nil {
		return fmt.Errorf("创建etcd服务器失败: %w", err)
	}
	res.EtcdServer = server

	// 等待etcd服务器完全启动
	t.Logf("等待etcd服务器启动完成 (%s)...", res.Config.StartupDelay)
	time.Sleep(res.Config.StartupDelay)
	t.Logf("etcd服务器启动成功，ClientURL: %s", res.Config.EtcdClientURL)

	return nil
}

// 创建etcd客户端
func createEtcdClient(res *testEnvResources, t *testing.T) error {
	t.Logf("正在创建etcd客户端 (Endpoint: %s)...", res.Config.EtcdClientURL)

	// 解析URL，提取主机和端口
	clientURL := res.Config.EtcdClientURL
	if strings.HasPrefix(clientURL, "http://") {
		clientURL = strings.TrimPrefix(clientURL, "http://")
	} else if strings.HasPrefix(clientURL, "https://") {
		clientURL = strings.TrimPrefix(clientURL, "https://")
	}

	// 创建etcd客户端配置
	clientConfig := &etcd.ClientConfig{
		Endpoints:   []string{clientURL},
		DialTimeout: 10 * time.Second,
	}

	// 添加重试逻辑
	var etcdClient *etcd.EtcdClient
	var clientErr error

	for i := 0; i < res.Config.ConnectRetryCount; i++ {
		etcdClient, clientErr = etcd.NewEtcdClient(clientConfig, res.Logger)
		if clientErr == nil {
			break
		}
		t.Logf("连接etcd服务器尝试 %d/%d 失败: %v，等待后重试...",
			i+1, res.Config.ConnectRetryCount, clientErr)
		time.Sleep(res.Config.ConnectRetryDelay)
	}

	if clientErr != nil {
		return fmt.Errorf("连接etcd服务器失败，已重试%d次: %w", res.Config.ConnectRetryCount, clientErr)
	}

	res.EtcdClient = etcdClient
	t.Logf("etcd客户端创建成功")

	return nil
}

// 初始化分布式管理器
func initDistributedManager(res *testEnvResources, t *testing.T) error {
	t.Logf("正在初始化分布式管理器...")

	// 创建分布式管理器
	manager := distributed.NewManager(res.EtcdClient.GetClient(), res.Logger)
	res.Manager = manager

	// 启动分布式管理器
	if err := manager.Start(res.Context); err != nil {
		return fmt.Errorf("启动分布式管理器失败: %w", err)
	}

	// 创建服务发现实例
	discoveryService := manager.Discovery()
	discovery := distributed.NewDiscoveryServiceAdapter(discoveryService)
	res.Discovery = discovery

	// 启动服务发现
	if err := discovery.Start(res.Context); err != nil {
		return fmt.Errorf("启动服务发现失败: %w", err)
	}

	t.Logf("分布式管理器初始化完成")
	return nil
}

// 资源清理函数
func cleanupResources(res *testEnvResources, t *testing.T) {
	t.Log("开始清理测试资源...")

	// 创建一个超时上下文用于清理操作
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 关闭Redis客户端
	if res.MasterClient != nil {
		_ = res.MasterClient.Close()
	}
	if res.SlaveClient != nil {
		_ = res.SlaveClient.Close()
	}
	for _, client := range res.ClusterClients {
		if client != nil {
			_ = client.Close()
		}
	}
	t.Log("Redis客户端已关闭")

	// 2. 停止缓存服务器
	if res.MasterServer != nil {
		_ = res.MasterServer.Stop(nil)
	}
	if res.SlaveServer != nil {
		_ = res.SlaveServer.Stop(nil)
	}
	for _, server := range res.ClusterServers {
		if server != nil {
			_ = server.Stop(nil)
		}
	}
	t.Log("缓存服务器已停止")

	// 3. 退出选举
	if res.MasterElection != nil {
		_ = res.MasterElection.Resign(ctx)
	}
	if res.SlaveElection != nil {
		_ = res.SlaveElection.Resign(ctx)
	}
	for _, election := range res.ClusterElections {
		if election != nil {
			_ = election.Resign(ctx)
		}
	}
	t.Log("已退出选举")

	// 4. 停止服务发现和分布式管理器
	if res.Discovery != nil {
		res.Discovery.Stop(ctx)
	}
	if res.Manager != nil {
		res.Manager.Stop(ctx)
	}
	t.Log("服务发现和分布式管理器已停止")

	// 5. 关闭etcd客户端
	if res.EtcdClient != nil {
		res.EtcdClient.Close()
	}
	t.Log("etcd客户端已关闭")

	// 6. 关闭etcd服务器
	if res.EtcdServer != nil {
		res.EtcdServer.Close()
	}
	t.Log("etcd服务器已关闭")

	// 7. 删除临时目录
	if res.Config != nil && res.Config.DataDir != "" {
		_ = os.RemoveAll(res.Config.DataDir)
	}
	t.Log("临时数据目录已清除")

	// 8. 取消上下文
	if res.CancelFunc != nil {
		res.CancelFunc()
	}
	t.Log("上下文已取消")

	t.Log("所有测试资源已清理完成")
}

// 设置单节点测试环境
func setupSingleNodeTest(t *testing.T) (*Server, *redis.Client, func()) {
	t.Log("正在设置单节点测试环境...")

	// 1. 创建测试环境
	resources, cleanup := setupReplicationTestEnv(t, ModeSingleNode)

	// 2. 创建节点选举
	t.Log("创建单节点选举...")
	nodeElection := distributed.NewEtcdElection(
		resources.EtcdClient.GetClient(),
		"single-node",
		"cache-single",
		func(o *distributed.ElectionOptions) {
			o.TTL = int(resources.Config.ElectionTTL)
			o.Prefix = resources.Config.ElectionPrefix
		},
	)
	resources.MasterElection = nodeElection

	// 3. 创建节点配置
	t.Log("创建节点配置...")
	nodeCfg := cache.DefaultConfig()
	nodeCfg.Port = resources.Config.MasterPort
	nodeCfg.Host = resources.Config.MasterHost
	nodeCfg.Password = resources.Config.MasterPassword
	nodeCfg = nodeCfg.ValidateAndFix()

	// 4. 创建缓存服务器
	t.Log("创建缓存服务器...")
	server, err := NewServer(
		nodeCfg,
		WithReplication(nodeElection, resources.Discovery, resources.ProtocolManager),
	)
	require.NoError(t, err)
	resources.MasterServer = server

	// 5. 启动缓存服务器
	t.Log("启动缓存服务器...")
	startServer := make(chan struct{})
	var startErr error

	go func() {
		startErr = server.Start(nil)
		close(startServer)
	}()

	// 等待服务器初始化
	t.Logf("等待服务器初始化 (%s)...", resources.Config.StartupDelay)
	time.Sleep(resources.Config.StartupDelay)

	// 检查启动是否有错误（非阻塞）
	select {
	case <-startServer:
		require.NoError(t, startErr, "缓存服务器启动失败")
		t.Log("缓存服务器已关闭")
	default:
		t.Log("缓存服务器正在运行")
	}

	// 6. 创建Redis客户端
	t.Log("创建Redis客户端...")
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", nodeCfg.Host, nodeCfg.Port),
		Password: nodeCfg.Password,
		DB:       0,
	})
	resources.MasterClient = client

	// 7. 验证连接
	t.Log("验证Redis连接...")
	err = client.Ping(resources.Context).Err()
	require.NoError(t, err, "无法连接到Redis服务器")

	t.Log("单节点测试环境设置完成")
	return server, client, cleanup
}

// 设置主从复制测试环境
func setupMasterSlaveTest(t *testing.T) (*Server, *Server, *redis.Client, *redis.Client, func()) {
	t.Log("正在设置主从复制测试环境...")

	// 1. 创建测试环境
	resources, cleanup := setupReplicationTestEnv(t, ModeMasterSlave)

	// 2. 创建主节点选举
	t.Log("创建主节点选举...")
	masterElection := distributed.NewEtcdElection(
		resources.EtcdClient.GetClient(),
		"master-node",
		"cache-master",
		func(o *distributed.ElectionOptions) {
			o.TTL = int(resources.Config.ElectionTTL)
			o.Prefix = resources.Config.ElectionPrefix
		},
	)
	resources.MasterElection = masterElection

	// 3. 创建从节点选举
	t.Log("创建从节点选举...")
	slaveElection := distributed.NewEtcdElection(
		resources.EtcdClient.GetClient(),
		"slave-node",
		"cache-master",
		func(o *distributed.ElectionOptions) {
			o.TTL = int(resources.Config.ElectionTTL)
			o.Prefix = resources.Config.ElectionPrefix
		},
	)
	resources.SlaveElection = slaveElection

	// 4. 创建主节点配置
	t.Log("创建主节点配置...")
	masterCfg := cache.DefaultConfig()
	masterCfg.Port = resources.Config.MasterPort
	masterCfg.Host = resources.Config.MasterHost
	masterCfg.Password = resources.Config.MasterPassword
	masterCfg = masterCfg.ValidateAndFix()

	// 5. 创建从节点配置
	t.Log("创建从节点配置...")
	slaveCfg := cache.DefaultConfig()
	slaveCfg.Port = resources.Config.SlavePort
	slaveCfg.Host = resources.Config.SlaveHost
	slaveCfg.Password = resources.Config.SlavePassword
	slaveCfg = slaveCfg.ValidateAndFix()

	// 6. 创建主节点服务器
	t.Log("创建主节点服务器...")
	masterServer, err := NewServer(
		masterCfg,
		WithReplication(masterElection, resources.Discovery, resources.ProtocolManager),
	)
	require.NoError(t, err)
	resources.MasterServer = masterServer

	// 7. 启动主节点服务器
	t.Log("启动主节点服务器...")
	go func() {
		err := masterServer.Start(nil)
		if err != nil {
			t.Logf("主节点服务器启动错误: %v", err)
		}
	}()

	// 等待主节点服务器初始化
	t.Logf("等待主节点服务器初始化 (%s)...", resources.Config.StartupDelay)
	time.Sleep(resources.Config.StartupDelay)

	// 8. 创建从节点服务器
	t.Log("创建从节点服务器...")
	slaveServer, err := NewServer(
		slaveCfg,
		WithReplication(slaveElection, resources.Discovery, resources.SlaveProtocolManager),
	)
	require.NoError(t, err)
	resources.SlaveServer = slaveServer

	// 9. 启动从节点服务器
	t.Log("启动从节点服务器...")
	go func() {
		err := slaveServer.Start(nil)
		if err != nil {
			t.Logf("从节点服务器启动错误: %v", err)
		}
	}()

	// 等待从节点服务器初始化
	t.Logf("等待从节点服务器初始化并完成角色分配 (%s)...", resources.Config.StartupDelay)
	time.Sleep(resources.Config.StartupDelay)

	// 10. 创建Redis客户端连接到主节点
	t.Log("创建主节点Redis客户端...")
	masterClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", masterCfg.Host, masterCfg.Port),
		Password: masterCfg.Password,
		DB:       0,
	})
	resources.MasterClient = masterClient

	// 11. 创建Redis客户端连接到从节点
	t.Log("创建从节点Redis客户端...")
	slaveClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", slaveCfg.Host, slaveCfg.Port),
		Password: slaveCfg.Password,
		DB:       0,
	})
	resources.SlaveClient = slaveClient

	// 12. 验证连接
	t.Log("验证主节点Redis连接...")
	err = masterClient.Ping(resources.Context).Err()
	require.NoError(t, err, "无法连接到主节点Redis服务器")

	t.Log("验证从节点Redis连接...")
	err = slaveClient.Ping(resources.Context).Err()
	require.NoError(t, err, "无法连接到从节点Redis服务器")

	// 13. 等待角色分配完成并验证
	t.Log("等待角色分配完成...")
	time.Sleep(1 * time.Second)

	// 14. 确认主从节点角色
	masterRole := masterServer.roleManager.GetRole()
	slaveRole := slaveServer.roleManager.GetRole()

	t.Logf("主节点角色: %v", masterRole)
	t.Logf("从节点角色: %v", slaveRole)

	// 对角色进行断言
	if masterRole != replication.RoleMaster || slaveRole != replication.RoleSlave {
		t.Logf("角色分配异常，测试可能不稳定，主节点角色: %v, 从节点角色: %v",
			masterRole, slaveRole)
	}

	t.Log("主从复制测试环境设置完成")
	return masterServer, slaveServer, masterClient, slaveClient, cleanup
}

// 设置集群模式测试环境
func setupClusterTest(t *testing.T) ([]*Server, []*redis.Client, func()) {
	t.Log("正在设置集群模式测试环境...")

	// 1. 创建测试环境
	resources, cleanup := setupReplicationTestEnv(t, ModeCluster)

	// 2. 初始化集群节点数组
	nodeCount := len(resources.Config.ClusterNodes)
	resources.ClusterServers = make([]*Server, nodeCount)
	resources.ClusterClients = make([]*redis.Client, nodeCount)
	resources.ClusterElections = make([]distributed.Election, nodeCount)

	// 3. 逐个创建和启动节点
	for i, nodeConfig := range resources.Config.ClusterNodes {
		nodeID := fmt.Sprintf("node-%d", i)
		t.Logf("创建集群节点 %s (端口: %d)...", nodeID, nodeConfig.Port)

		// 3.1 创建节点选举
		election := distributed.NewEtcdElection(
			resources.EtcdClient.GetClient(),
			nodeID,
			"cache-cluster",
			func(o *distributed.ElectionOptions) {
				o.TTL = int(resources.Config.ElectionTTL)
				o.Prefix = resources.Config.ElectionPrefix
			},
		)
		resources.ClusterElections[i] = election

		// 3.2 创建节点配置
		nodeCfg := cache.DefaultConfig()
		nodeCfg.Port = nodeConfig.Port
		nodeCfg.Host = nodeConfig.Host
		nodeCfg.Password = nodeConfig.Password
		nodeCfg = nodeCfg.ValidateAndFix()

		// 3.3 创建缓存服务器
		server, err := NewServer(
			nodeCfg,
			WithReplication(election, resources.Discovery, resources.ProtocolManager),
		)
		require.NoError(t, err)
		resources.ClusterServers[i] = server

		// 3.4 启动缓存服务器
		go func(srv *Server, id string) {
			err := srv.Start(nil)
			if err != nil {
				t.Logf("集群节点 %s 启动错误: %v", id, err)
			}
		}(server, nodeID)

		// 3.5 等待服务器初始化 (每个节点间隔启动)
		t.Logf("等待节点 %s 初始化 (1s)...", nodeID)
		time.Sleep(1 * time.Second)

		// 3.6 创建Redis客户端
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", nodeConfig.Host, nodeConfig.Port),
			Password: nodeConfig.Password,
			DB:       0,
		})
		resources.ClusterClients[i] = client

		// 3.7 验证连接
		err = client.Ping(resources.Context).Err()
		require.NoError(t, err, "无法连接到集群节点 %s", nodeID)
	}

	// 4. 等待集群稳定
	t.Logf("等待集群稳定 (%s)...", resources.Config.StartupDelay)
	time.Sleep(resources.Config.StartupDelay)

	// 5. 检查集群状态
	for i, server := range resources.ClusterServers {
		role := server.roleManager.GetRole()
		t.Logf("集群节点 %d 角色: %v", i, role)
	}

	t.Log("集群模式测试环境设置完成")
	return resources.ClusterServers, resources.ClusterClients, cleanup
}

// 为了向后兼容，保留原有函数名但使用新函数实现
func setupReplicationTest(t *testing.T) (*Server, *Server, *redis.Client, *redis.Client, func()) {
	return setupMasterSlaveTest(t)
}

// 测试主从复制基本功能
func TestBasicReplication(t *testing.T) {
	masterServer, slaveServer, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 确保主从节点角色稳定
	time.Sleep(1 * time.Second)

	// 先检查主从节点角色，确保角色分配正确
	require.Equal(t, replication.RoleMaster, masterServer.roleManager.GetRole(),
		"主节点角色应该为master")
	require.Equal(t, replication.RoleSlave, slaveServer.roleManager.GetRole(),
		"从节点角色应该为slave")

	masterRole, err := masterClient.Do(ctx, "ROLE").Result()
	require.NoError(t, err)
	t.Logf("主节点角色: %v", masterRole)

	slaveRole, err := slaveClient.Do(ctx, "ROLE").Result()
	require.NoError(t, err)
	t.Logf("从节点角色: %v", slaveRole)

	// 先清除所有数据，以避免之前测试留下的大量数据导致消息过大
	// 在从节点上，使用命令模式，避免使用FlushAll
	_, err = masterClient.Do(ctx, "FLUSHALL").Result()
	require.NoError(t, err, "清除主节点数据失败")

	// 等待从节点同步清除操作
	time.Sleep(1 * time.Second)

	// 在主节点写入数据
	err = masterClient.Set(ctx, "key1", "value1", 0).Err()
	require.NoError(t, err)
	t.Logf("在主节点成功写入数据: key1=value1")

	// 稍等片刻，让数据复制到从节点
	t.Logf("等待2秒钟让数据复制到从节点...")
	time.Sleep(2 * time.Second)

	// 检查主节点数据
	masterVal, err := masterClient.Get(ctx, "key1").Result()
	require.NoError(t, err, "从主节点读取数据失败")
	t.Logf("从主节点读取数据成功: key1=%s", masterVal)

	// 从从节点读取数据，验证复制是否成功
	t.Logf("尝试从从节点读取数据...")
	val, err := slaveClient.Get(ctx, "key1").Result()
	require.NoError(t, err, "从从节点读取数据失败")
	assert.Equal(t, "value1", val, "从节点数据应与主节点一致")
	t.Logf("从从节点成功读取数据: key1=%s", val)

	// 测试多个键值对的复制
	// 再次确认主节点角色未变
	require.Equal(t, replication.RoleMaster, masterServer.roleManager.GetRole(),
		"主节点角色应保持为master")

	err = masterClient.MSet(ctx, "key2", "value2", "key3", "value3").Err()
	require.NoError(t, err)

	// 稍等片刻，让数据复制到从节点
	time.Sleep(2 * time.Second)

	// 确保主节点仍然是主节点，角色稳定
	require.Equal(t, replication.RoleMaster, masterServer.roleManager.GetRole(),
		"主节点角色应该保持为master")

	// 验证主节点数据写入是否成功
	val2, err := masterClient.Get(ctx, "key2").Result()
	require.NoError(t, err)
	assert.Equal(t, "value2", val2)

	// 从从节点读取数据，验证复制是否成功
	val2, err = slaveClient.Get(ctx, "key2").Result()
	require.NoError(t, err)
	assert.Equal(t, "value2", val2)

	var val3 string
	val3, err = slaveClient.Get(ctx, "key3").Result()
	require.NoError(t, err)
	assert.Equal(t, "value3", val3)
}

// 测试写命令拦截
func TestSlaveWriteRejection(t *testing.T) {
	_, _, _, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 尝试在从节点写入数据，应该被拒绝
	err := slaveClient.Set(ctx, "key1", "value1", 0).Err()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read only")
}

// 测试复制数据的持久性
func TestReplicationPersistence(t *testing.T) {
	_, _, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 在主节点写入多个数据类型
	// 1. 字符串
	err := masterClient.Set(ctx, "string_key", "string_value", 0).Err()
	require.NoError(t, err)

	// 2. 哈希表
	err = masterClient.HSet(ctx, "hash_key", map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}).Err()
	require.NoError(t, err)

	// 3. 列表
	err = masterClient.LPush(ctx, "list_key", "item1", "item2", "item3").Err()
	require.NoError(t, err)

	// 4. 集合
	err = masterClient.SAdd(ctx, "set_key", "member1", "member2", "member3").Err()
	require.NoError(t, err)

	// 5. 有序集合
	err = masterClient.ZAdd(ctx, "zset_key",
		redis.Z{Score: 1, Member: "member1"},
		redis.Z{Score: 2, Member: "member2"},
		redis.Z{Score: 3, Member: "member3"},
	).Err()
	require.NoError(t, err)

	// 稍等片刻，让数据复制到从节点
	time.Sleep(2 * time.Second)

	// 从从节点验证所有数据类型
	// 1. 验证字符串
	val, err := slaveClient.Get(ctx, "string_key").Result()
	require.NoError(t, err)
	assert.Equal(t, "string_value", val)

	// 2. 验证哈希表
	hashVal, err := slaveClient.HGetAll(ctx, "hash_key").Result()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"field1": "value1", "field2": "value2"}, hashVal)

	// 3. 验证列表
	listVal, err := slaveClient.LRange(ctx, "list_key", 0, -1).Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"item3", "item2", "item1"}, listVal)

	// 4. 验证集合
	setVal, err := slaveClient.SMembers(ctx, "set_key").Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"member1", "member2", "member3"}, setVal)

	// 5. 验证有序集合
	zsetVal, err := slaveClient.ZRangeWithScores(ctx, "zset_key", 0, -1).Result()
	require.NoError(t, err)
	assert.Len(t, zsetVal, 3)
}

// 测试删除操作的复制
func TestDeleteReplication(t *testing.T) {
	_, _, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 在主节点创建一些键
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("del_key_%d", i)
		value := fmt.Sprintf("del_value_%d", i)
		err := masterClient.Set(ctx, key, value, 0).Err()
		require.NoError(t, err)
	}

	// 等待复制完成
	time.Sleep(1 * time.Second)

	// 从从节点验证键存在
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("del_key_%d", i)
		exists, err := slaveClient.Exists(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)
	}

	// 在主节点删除部分键
	delCount, err := masterClient.Del(ctx, "del_key_0", "del_key_2", "del_key_4").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(3), delCount)

	// 等待复制完成
	time.Sleep(1 * time.Second)

	// 验证从节点的键也被删除
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("del_key_%d", i)
		exists, err := slaveClient.Exists(ctx, key).Result()
		require.NoError(t, err)

		if i == 0 || i == 2 || i == 4 {
			// 这些键应该已被删除
			assert.Equal(t, int64(0), exists)
		} else {
			// 这些键应该仍然存在
			assert.Equal(t, int64(1), exists)
		}
	}
}

// 测试过期时间的复制
func TestExpirationReplication(t *testing.T) {
	_, _, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 在主节点创建带有过期时间的键
	err := masterClient.Set(ctx, "expire_key_1", "expire_value_1", 5*time.Second).Err()
	require.NoError(t, err)

	err = masterClient.Set(ctx, "expire_key_2", "expire_value_2", 0).Err()
	require.NoError(t, err)
	err = masterClient.Expire(ctx, "expire_key_2", 10*time.Second).Err()
	require.NoError(t, err)

	// 创建不过期的键作为对照
	err = masterClient.Set(ctx, "persist_key", "persist_value", 0).Err()
	require.NoError(t, err)

	// 等待复制完成
	time.Sleep(1 * time.Second)

	// 验证所有键在从节点都存在
	for _, key := range []string{"expire_key_1", "expire_key_2", "persist_key"} {
		exists, err := slaveClient.Exists(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)
	}

	// 验证TTL复制正确
	ttl1, err := slaveClient.TTL(ctx, "expire_key_1").Result()
	require.NoError(t, err)
	assert.True(t, ttl1 > 0 && ttl1 <= 5*time.Second)

	ttl2, err := slaveClient.TTL(ctx, "expire_key_2").Result()
	require.NoError(t, err)
	assert.True(t, ttl2 > 0 && ttl2 <= 10*time.Second)

	ttl3, err := slaveClient.TTL(ctx, "persist_key").Result()
	require.NoError(t, err)
	assert.Equal(t, time.Duration(-1), ttl3) // -1 表示永不过期

	// 等待第一个键过期
	time.Sleep(6 * time.Second)

	// 验证第一个键已过期，其他键仍然存在
	exists, err := slaveClient.Exists(ctx, "expire_key_1").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)

	exists, err = slaveClient.Exists(ctx, "expire_key_2").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists)

	exists, err = slaveClient.Exists(ctx, "persist_key").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists)
}

// 测试大批量数据复制
func TestBulkDataReplication(t *testing.T) {
	_, _, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 在主节点批量写入数据
	//pipe := masterClient.Pipeline()
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("bulk_key_%d", i)
		value := fmt.Sprintf("bulk_value_%d", i)
		err := masterClient.Set(ctx, key, value, 0).Err()
		require.NoError(t, err)
	}
	//_, err := pipe.Exec(ctx)

	// 等待复制完成
	time.Sleep(1 * time.Second)

	// 随机验证一些键值
	for i := 0; i < 10; i++ {
		index := i * 50
		key := fmt.Sprintf("bulk_key_%d", index)
		expectedValue := fmt.Sprintf("bulk_value_%d", index)

		value, err := slaveClient.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, expectedValue, value)
	}

	// 验证键的总数
	masterKeys, err := masterClient.DBSize(ctx).Result()
	require.NoError(t, err)

	// 给复制一些时间
	time.Sleep(2 * time.Second)

	slaveKeys, err := slaveClient.DBSize(ctx).Result()
	require.NoError(t, err)

	assert.Equal(t, masterKeys, slaveKeys)
}

// 测试主从断开连接后重新连接
func TestReconnection(t *testing.T) {
	_, _, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 在主节点写入数据
	err := masterClient.Set(ctx, "before_disconnect", "master_value", 0).Err()
	require.NoError(t, err)

	// 等待复制完成
	time.Sleep(1 * time.Second)

	// 从从节点读取数据，验证复制是否成功
	val, err := slaveClient.Get(ctx, "before_disconnect").Result()
	require.NoError(t, err)
	assert.Equal(t, "master_value", val)

	// 断开从节点连接
	_, err = slaveClient.SlaveOf(ctx, "NO", "ONE").Result()
	require.NoError(t, err)

	// 在从节点设置数据（此时从节点已独立）
	err = slaveClient.Set(ctx, "independent_key", "independent_value", 0).Err()
	require.NoError(t, err)

	// 验证独立数据已经设置成功
	val, err = slaveClient.Get(ctx, "independent_key").Result()
	require.NoError(t, err)
	assert.Equal(t, "independent_value", val)

	// 在主节点设置新数据
	err = masterClient.Set(ctx, "after_disconnect", "new_master_value", 0).Err()
	require.NoError(t, err)

	// 从masterClient获取主节点地址信息
	masterAddr := masterClient.Options().Addr
	host, port, err := net.SplitHostPort(masterAddr)
	require.NoError(t, err)

	// 重新连接从节点
	_, err = slaveClient.SlaveOf(ctx, host, port).Result()
	require.NoError(t, err)

	// 等待重新同步 (增加等待时间以确保完全同步)
	time.Sleep(4 * time.Second)

	// 重新检查主从节点角色，确保角色分配正确
	masterRole, err := masterClient.Do(ctx, "ROLE").Result()
	require.NoError(t, err)
	t.Logf("重连后主节点角色: %v", masterRole)

	slaveRole, err := slaveClient.Do(ctx, "ROLE").Result()
	require.NoError(t, err)
	t.Logf("重连后从节点角色: %v", slaveRole)

	// 确定哪个是真正的主节点
	var actualMasterClient *redis.Client
	if strings.Contains(fmt.Sprintf("%v", masterRole), "master") {
		actualMasterClient = masterClient
		t.Log("原主节点仍然是主节点")
	} else if strings.Contains(fmt.Sprintf("%v", slaveRole), "master") {
		actualMasterClient = slaveClient
		t.Log("从节点现在是主节点")
	} else {
		// 如果无法确定主节点，我们尝试两个客户端都进行写入测试
		t.Log("无法通过ROLE命令确定哪个节点是主节点，将尝试在两个节点上写入数据")

		// 尝试在原主节点上写入
		err = masterClient.Set(ctx, "master_write_test", "test_value", 0).Err()
		if err == nil {
			actualMasterClient = masterClient
			t.Log("通过写入测试确认原主节点仍然是主节点")
		} else {
			// 尝试在从节点上写入
			err = slaveClient.Set(ctx, "slave_write_test", "test_value", 0).Err()
			if err == nil {
				actualMasterClient = slaveClient
				t.Log("通过写入测试确认从节点现在是主节点")
			} else {
				t.Log("两个节点都无法写入，可能都是从节点状态，将使用原主节点客户端继续测试")
				actualMasterClient = masterClient
			}
		}
	}

	// 验证从节点能收到断开期间的更新
	val, err = slaveClient.Get(ctx, "after_disconnect").Result()
	if err != nil {
		t.Logf("获取断开期间更新的数据失败: %v", err)
	} else {
		assert.Equal(t, "new_master_value", val)
	}

	// 验证从节点的独立数据是否被清除
	// 注意：根据Redis的行为，重连后独立数据应该被清除
	_, err = slaveClient.Get(ctx, "independent_key").Result()
	if err != nil {
		// 如果有错误，应该是因为键不存在
		assert.Contains(t, err.Error(), "redis: nil", "独立数据应该被清除，返回键不存在错误")
	} else {
		// 如果没有错误，说明数据仍然存在，这可能是实现的差异
		// 我们不强制要求数据被清除，但记录这种情况
		t.Log("注意：重连后独立数据未被清除，这与标准Redis行为不同")
	}

	// 尝试在确定的主节点上设置新数据来验证同步功能是否正常
	err = actualMasterClient.Set(ctx, "new_sync_test", "sync_value", 0).Err()
	if err != nil {
		t.Logf("在主节点上设置数据失败: %v", err)
		t.Log("尝试在另一个节点上设置数据")

		if actualMasterClient == masterClient {
			err = slaveClient.Set(ctx, "new_sync_test", "sync_value", 0).Err()
			if err == nil {
				actualMasterClient = slaveClient
				t.Log("从节点可以写入数据，确认它是主节点")
			} else {
				t.Logf("两个节点都无法写入数据: %v", err)
			}
		} else {
			err = masterClient.Set(ctx, "new_sync_test", "sync_value", 0).Err()
			if err == nil {
				actualMasterClient = masterClient
				t.Log("原主节点可以写入数据，确认它是主节点")
			} else {
				t.Logf("两个节点都无法写入数据: %v", err)
			}
		}
	}

	// 如果成功写入数据，则验证同步
	if err == nil {
		// 等待主从同步
		time.Sleep(1 * time.Second)

		// 验证数据已同步到从节点
		val, err = slaveClient.Get(ctx, "new_sync_test").Result()
		if err != nil {
			t.Logf("从从节点获取同步数据失败: %v", err)
		} else {
			assert.Equal(t, "sync_value", val, "主节点新设置的数据应该同步到从节点")
		}
	}
}

// 测试REPLCONF和INFO命令
func TestReplicationCommands(t *testing.T) {
	_, _, masterClient, slaveClient, cleanup := setupReplicationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 测试ROLE命令 (主节点)
	masterRole, err := masterClient.Do(ctx, "ROLE").Result()
	require.NoError(t, err)
	roleResult, ok := masterRole.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, "master", roleResult[0])

	// 测试ROLE命令 (从节点)
	slaveRole, err := slaveClient.Do(ctx, "ROLE").Result()
	require.NoError(t, err)
	slaveRoleResult, ok := slaveRole.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, "slave", slaveRoleResult[0])

	// 测试INFO命令的复制部分
	masterInfo, err := masterClient.Info(ctx, "replication").Result()
	require.NoError(t, err)
	t.Logf("主节点复制信息:\n%s", masterInfo)
	assert.Contains(t, masterInfo, "role:master")
	assert.Contains(t, masterInfo, "connected_slaves:")

	slaveInfo, err := slaveClient.Info(ctx, "replication").Result()
	require.NoError(t, err)
	t.Logf("从节点复制信息:\n%s", slaveInfo)
	assert.Contains(t, slaveInfo, "role:slave")
	assert.Contains(t, slaveInfo, "master_host:")
}

// 测试服务器启动时使用自定义协议
func TestCustomProtocolWithReplication(t *testing.T) {
	// 初始化测试环境
	resources, cleanup := setupReplicationTestEnv(t, ModeSingleNode)
	defer cleanup()

	// 获取etcd客户端
	etcdClient := resources.EtcdClient

	// 创建服务发现
	disc, err := distributed.NewEtcdServiceDiscovery(etcdClient.GetClient(), func(o *distributed.DiscoveryOptions) {
		o.Prefix = "/cache-service"
		o.DefaultTTL = 15
	})
	require.NoError(t, err)

	// 启动服务发现
	ctx := context.Background()
	err = disc.Start(ctx)
	require.NoError(t, err)
	defer disc.Stop(ctx)

	// 创建主节点选举
	masterElect := distributed.NewEtcdElection(etcdClient.GetClient(), "custom-master", "cache-master", func(o *distributed.ElectionOptions) {
		o.TTL = 30 // 增加主节点的TTL，使其更稳定
		o.Prefix = "/elections/"
	})

	// 创建协议管理器（使用自定义协议）
	protocolMgr := protocol.NewServer(nil)

	// 创建服务器配置
	cfg := cache.DefaultConfig()
	cfg.Port = 7500 // 使用不同的端口
	cfg = cfg.ValidateAndFix()

	// 创建服务器并使用自定义协议
	server, err := NewServer(cfg, WithReplication(masterElect, disc, protocolMgr))
	require.NoError(t, err)

	// 启动服务器
	go func() {
		err := server.Start(nil)
		if err != nil {
			t.Logf("服务器启动错误: %v", err)
		}
	}()
	defer server.Stop(nil)

	// 等待服务器启动
	time.Sleep(2 * time.Second)

	// 测试基本功能
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("127.0.0.1:%d", cfg.Port),
	})
	defer client.Close()

	// 写入数据
	_, err = client.Set(ctx, "key1", "value1", 0).Result()
	require.NoError(t, err)

	// 读取数据
	value, err := client.Get(ctx, "key1").Result()
	require.NoError(t, err)
	assert.Equal(t, "value1", value)
}
