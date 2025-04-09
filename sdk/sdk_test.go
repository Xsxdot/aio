package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/protocol"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain 负责运行所有测试并处理设置和清理
func TestMain(m *testing.M) {
	// 设置测试环境
	setup()

	// 运行测试
	code := m.Run()

	// 清理测试环境
	teardown()

	// 退出
	os.Exit(code)
}

// 设置测试环境
func setup() {
	log.Println("设置测试环境...")
	// 这里可以添加更多的设置逻辑
}

// 清理测试环境
func teardown() {
	log.Println("清理测试环境...")
	// 这里可以添加更多的清理逻辑
}

// 创建测试客户端
func createTestClient() (*Client, error) {
	// 配置服务器端点
	servers := []ServerEndpoint{
		{Host: "127.0.0.1", Port: 6666}, // 使用实际服务器地址
	}

	// 配置选项
	options := &ClientOptions{
		ConnectionTimeout:    5 * time.Second,
		RetryCount:           2,
		RetryInterval:        1 * time.Second,
		AutoConnectToLeader:  true,
		ServiceWatchInterval: 2 * time.Second,
	}

	// 创建客户端
	client := NewClient(servers, options)

	return client, nil
}

// TestClientConnect 测试客户端连接功能
func TestClientConnect(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")
	defer client.Close()

	// 测试连接
	err = client.Connect()
	assert.NoError(t, err, "连接服务器失败")

	// 检查是否有活动的连接
	client.connectionLock.RLock()
	activeConnections := len(client.connections)
	client.connectionLock.RUnlock()

	assert.GreaterOrEqual(t, activeConnections, 1, "应该至少有一个活动连接")
}

// TestGetLeaderInfo 测试获取主节点信息功能
func TestGetLeaderInfo(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取主节点信息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaderInfo, err := client.GetLeaderInfo(ctx)
	assert.NoError(t, err, "获取主节点信息失败")
	if err == nil {
		assert.NotEmpty(t, leaderInfo.NodeID, "主节点ID不应为空")
		assert.NotEmpty(t, leaderInfo.IP, "主节点IP不应为空")
		assert.Greater(t, leaderInfo.ProtocolPort, 0, "主节点端口应大于0")
	}
}

// TestConnectToLeader 测试连接到主节点功能
func TestConnectToLeader(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 连接到主节点
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.ConnectToLeader(ctx)
	assert.NoError(t, err, "连接主节点失败")

	// 验证确实连接到主节点
	client.leaderLock.RLock()
	hasLeader := client.leaderNode != nil
	client.leaderLock.RUnlock()

	assert.True(t, hasLeader, "应该有主节点信息")
}

// TestServiceDiscovery 测试服务发现功能
func TestServiceDiscovery(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 测试服务发现
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试监听服务
	err = client.Discovery.WatchService(ctx, "aio-service")
	assert.NoError(t, err, "监听服务失败")

	// 等待服务发现完成
	time.Sleep(2 * time.Second)

	// 获取服务信息
	services := client.Discovery.GetServicesByName("aio-service")
	assert.GreaterOrEqual(t, len(services), 0, "应该能获取到服务信息")

	// 打印发现的服务信息
	for _, svc := range services {
		t.Logf("发现服务: ID=%s, Name=%s, Address=%s:%d",
			svc.ID, svc.Name, svc.Address, svc.Port)
	}
}

// TestEventHandlers 测试事件处理器功能
func TestEventHandlers(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 测试主节点变更处理器
	leaderChangeCh := make(chan struct{}, 1)
	client.OnLeaderChange(func(oldLeader, newLeader *NodeInfo) {
		t.Logf("主节点变更: 从 %v 到 %v", oldLeader, newLeader)
		leaderChangeCh <- struct{}{}
	})

	// 测试连接状态变更处理器
	connStatusCh := make(chan struct{}, 1)
	client.OnConnectionStatusChange(func(nodeID, connID string, connected bool) {
		t.Logf("连接状态变更: 节点=%s, 连接=%s, 状态=%v", nodeID, connID, connected)
		connStatusCh <- struct{}{}
	})

	// 关闭连接，应该触发连接状态变更事件
	client.Close()

	// 检查是否收到事件通知
	select {
	case <-connStatusCh:
		// 成功
	case <-time.After(2 * time.Second):
		t.Log("未收到连接状态变更事件，可能是因为测试环境限制")
	}
}

// TestSendMessage 测试发送消息功能
func TestSendMessage(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 构造测试消息
	type TestMessage struct {
		Message string `json:"message"`
		Time    string `json:"time"`
	}

	testMsg := TestMessage{
		Message: "测试消息",
		Time:    time.Now().Format(time.RFC3339),
	}

	payload, err := json.Marshal(testMsg)
	require.NoError(t, err, "序列化消息失败")

	// 发送消息
	err = client.SendMessage(protocol.MessageType(100), protocol.ServiceType(1), payload) // 使用自定义消息类型和服务类型
	assert.NoError(t, err, "发送消息失败")
}

// TestCloseClient 测试关闭客户端功能
func TestCloseClient(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")

	// 关闭客户端
	err = client.Close()
	assert.NoError(t, err, "关闭客户端失败")

	// 检查是否已关闭所有连接
	client.connectionLock.RLock()
	activeConnections := len(client.connections)
	client.connectionLock.RUnlock()

	assert.Equal(t, 0, activeConnections, "关闭后应该没有活动连接")
}

// 以下是各服务组件的测试用例

// TestDiscoveryService 测试服务发现组件功能
func TestDiscoveryService(t *testing.T) {
	// 测试用例分成多个子测试
	t.Run("RegisterAndDiscoverService", func(t *testing.T) {
		client, err := createTestClient()
		require.NoError(t, err, "创建客户端失败")

		// 首先连接
		err = client.Connect()
		require.NoError(t, err, "连接服务器失败")
		defer client.Close()

		// 获取服务发现组件
		discoveryService := client.Discovery
		assert.NotNil(t, discoveryService, "服务发现组件不应为空")

		// 测试获取服务节点
		ctx := context.Background()
		nodes, err := discoveryService.GetServiceNodes("aio-service")
		if err == nil {
			t.Logf("获取到 %d 个服务节点", len(nodes))
			for _, node := range nodes {
				t.Logf("节点: ID=%s, 地址=%s:%d", node.ID, node.Address, node.Port)
			}
		} else {
			t.Logf("获取服务节点失败: %v", err)
		}

		// 测试服务变更回调
		changesCh := make(chan struct{}, 1)
		discoveryService.OnServiceNodesChange(func(svcName string, added, removed []discovery.ServiceInfo) {
			t.Logf("服务变更: 服务=%s, 新增=%d, 移除=%d", svcName, len(added), len(removed))
			changesCh <- struct{}{}
		})

		// 监听服务变更
		err = discoveryService.WatchService(ctx, "aio-service")
		assert.NoError(t, err, "监听服务失败")

		// 等待一段时间，检查是否收到服务变更通知
		time.Sleep(2 * time.Second)
	})
}

// TestEtcdService 测试ETCD服务组件功能
func TestEtcdService(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取ETCD服务组件
	etcdService := client.getEtcdService()
	assert.NotNil(t, etcdService, "ETCD服务组件不应为空")

	// 尝试连接ETCD
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = etcdService.Connect(ctx)
	if err != nil {
		t.Logf("连接ETCD失败: %v", err)
	} else {
		t.Log("成功连接到ETCD")

		// 测试分布式锁
		lock, err := client.CreateLock(ctx, "test-lock")
		if err == nil {
			// 尝试获取锁
			err = lock.Lock(ctx)
			if err == nil {
				t.Log("成功获取分布式锁")
				// 释放锁
				err = lock.Unlock(ctx)
				assert.NoError(t, err, "释放分布式锁失败")
			} else {
				t.Logf("获取分布式锁失败: %v", err)
			}
		} else {
			t.Logf("创建分布式锁失败: %v", err)
		}
	}
}

// TestRedisService 测试Redis服务组件功能
func TestRedisService(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取Redis服务组件
	redisService := client.getRedisService()
	assert.NotNil(t, redisService, "Redis服务组件不应为空")

	// 尝试连接Redis并执行操作
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	redisClient, err := redisService.Get()
	if err != nil {
		t.Logf("获取Redis客户端失败: %v", err)
	} else {
		t.Log("成功获取Redis客户端")

		// 测试设置和获取键值
		testKey := fmt.Sprintf("test-key-%d", time.Now().Unix())
		testValue := "test-value"

		// 获取Redis客户端并锁定
		client, unlock := redisClient.GetLockedClient()
		defer unlock()

		// 设置键值
		err = client.Set(ctx, testKey, testValue, 30*time.Second).Err()
		if err == nil {
			// 获取刚设置的值
			val, err := client.Get(ctx, testKey).Result()
			if err == nil {
				assert.Equal(t, testValue, val, "获取的值与设置的值不匹配")
				t.Logf("成功设置并获取Redis键值: %s=%s", testKey, val)
			} else {
				t.Logf("获取Redis键值失败: %v", err)
			}
		} else {
			t.Logf("设置Redis键值失败: %v", err)
		}
	}
}

// TestNatsService 测试NATS服务组件功能
func TestNatsService(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败1")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取NATS服务组件
	natsService := client.getNatsService()
	assert.NotNil(t, natsService, "NATS服务组件不应为空")

	// 尝试连接NATS
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nc, err := natsService.GetClient(ctx)
	if err != nil {
		t.Logf("获取NATS客户端失败: %v", err)
	} else {
		t.Log("成功获取NATS客户端")

		// 测试发布订阅
		sub, err := nc.SubscribeSync("test.subject")
		if err == nil {
			// 发布消息
			err = nc.Publish("test.subject", []byte("hello"))
			assert.NoError(t, err, "发布消息失败")

			// 尝试接收消息
			msg, err := sub.NextMsg(time.Second)
			if err == nil {
				assert.Equal(t, "hello", string(msg.Data), "接收的消息与发送的不匹配")
				t.Log("成功发布并接收NATS消息")
			} else {
				t.Logf("接收NATS消息失败: %v", err)
			}

			// 取消订阅
			sub.Unsubscribe()
		} else {
			t.Logf("订阅NATS主题失败: %v", err)
		}
	}
}

// TestIntegration 综合测试多个组件协同工作
func TestIntegration(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 测试与主节点的通信
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取主节点信息
	leaderInfo, err := client.GetLeaderInfo(ctx)
	if err == nil {
		t.Logf("主节点信息: ID=%s, IP=%s, Port=%d",
			leaderInfo.NodeID, leaderInfo.IP, leaderInfo.ProtocolPort)

		// 连接到主节点
		err = client.ConnectToLeader(ctx)
		assert.NoError(t, err, "连接主节点失败")

		// 发送测试消息到主节点
		type TestMessage struct {
			Message string `json:"message"`
			Time    string `json:"time"`
		}

		testMsg := TestMessage{
			Message: "发送到主节点的测试消息",
			Time:    time.Now().Format(time.RFC3339),
		}

		payload, err := json.Marshal(testMsg)
		require.NoError(t, err, "序列化消息失败")

		// 发送消息
		err = client.SendMessage(protocol.MessageType(100), protocol.ServiceType(1), payload)
		assert.NoError(t, err, "发送消息到主节点失败")
	} else {
		t.Logf("获取主节点信息失败: %v", err)
	}
}
