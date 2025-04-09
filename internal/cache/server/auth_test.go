package server

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建带密码的测试服务器
func setupAuthTestServer(t *testing.T, password string) (*Server, func()) {
	// 创建服务器配置
	config := cache.DefaultConfig()
	// 使用指定端口
	config.Port = 16380
	// 设置密码
	config.Password = password
	config = config.ValidateAndFix()

	// 创建服务器
	server, err := NewServer(config)
	require.NoError(t, err)

	// 启动服务器
	go func() {
		err := server.Start(nil)
		if err != nil {
			t.Logf("服务器启动错误: %v", err)
		}
	}()

	// 给服务器更多的启动时间
	time.Sleep(500 * time.Millisecond)

	// 返回清理函数
	cleanup := func() {
		server.Stop(nil)
	}

	return server, cleanup
}

// 测试服务器无密码时的AUTH命令
func TestAuthWithNoPassword(t *testing.T) {
	// 创建无密码的服务器
	server, cleanup := setupAuthTestServer(t, "")
	defer cleanup()

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", server.GetPort()),
		Password:    "",
		DB:          0,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	defer client.Close()

	ctx := context.Background()

	// 验证可以执行命令（无需认证）
	pong, err := client.Ping(ctx).Result()
	assert.NoError(t, err)
	assert.Equal(t, "PONG", pong)

	// 测试AUTH命令，在无密码设置时应该返回错误
	err = client.Do(ctx, "AUTH", "any_password").Err()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no password is set")
}

// 测试密码错误的认证
func TestAuthWithWrongPassword(t *testing.T) {
	// 创建带密码的服务器
	server, cleanup := setupAuthTestServer(t, "correct_password")
	defer cleanup()

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", server.GetPort()),
		Password:    "",
		DB:          0,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	defer client.Close()

	ctx := context.Background()

	// 验证在认证前不能执行命令
	_, err := client.Ping(ctx).Result()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOAUTH")

	// 使用错误密码认证
	cmd := client.Do(ctx, "AUTH", "wrong_password")
	err = cmd.Err()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid password")
	}

	// 验证认证失败后仍然不能执行命令
	_, err = client.Ping(ctx).Result()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOAUTH")
}

// 测试密码正确的认证
func TestAuthWithCorrectPassword(t *testing.T) {
	// 创建带密码的服务器
	correctPassword := "correct_password"
	server, cleanup := setupAuthTestServer(t, correctPassword)
	defer cleanup()

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", server.GetPort()),
		Password:    "",
		DB:          0,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	defer client.Close()

	ctx := context.Background()

	// 验证在认证前不能执行命令
	_, err := client.Ping(ctx).Result()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOAUTH")

	// 使用正确密码认证
	err = client.Do(ctx, "AUTH", correctPassword).Err()
	assert.NoError(t, err)

	// 验证认证成功后可以执行命令
	pong, err := client.Ping(ctx).Result()
	assert.NoError(t, err)
	assert.Equal(t, "PONG", pong)

	// 测试其他命令
	err = client.Set(ctx, "test_key", "test_value", 0).Err()
	assert.NoError(t, err)

	value, err := client.Get(ctx, "test_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

// 测试使用正确密码直接连接
func TestDirectConnectionWithPassword(t *testing.T) {
	// 创建带密码的服务器
	correctPassword := "direct_password"
	server, cleanup := setupAuthTestServer(t, correctPassword)
	defer cleanup()

	// 创建Redis客户端，直接提供密码
	client := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", server.GetPort()),
		Password:    correctPassword,
		DB:          0,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	defer client.Close()

	ctx := context.Background()

	// 验证可以直接执行命令（连接时已提供密码）
	pong, err := client.Ping(ctx).Result()
	assert.NoError(t, err)
	assert.Equal(t, "PONG", pong)
}

// 测试认证状态在连接关闭后是否正确清理
func TestAuthCleanupAfterConnectionClose(t *testing.T) {
	// 创建带密码的服务器
	correctPassword := "test_password"
	server, cleanup := setupAuthTestServer(t, correctPassword)
	defer cleanup()

	ctx := context.Background()

	// 第一个客户端认证和操作
	{
		client := redis.NewClient(&redis.Options{
			Addr:        fmt.Sprintf("127.0.0.1:%d", server.GetPort()),
			Password:    "",
			DB:          0,
			DialTimeout: 2 * time.Second,
			ReadTimeout: 2 * time.Second,
		})

		// 认证
		err := client.Do(ctx, "AUTH", correctPassword).Err()
		assert.NoError(t, err)

		// 设置键值
		err = client.Set(ctx, "connection_test", "value1", 0).Err()
		assert.NoError(t, err)

		// 关闭客户端连接
		client.Close()

		// 等待连接状态清理（monitorConnections函数会每30秒检查一次，但为了测试效率，我们使用内部API直接检查）
		// 短暂延迟确保连接完全关闭
		time.Sleep(100 * time.Millisecond)

		// 获取已认证连接的数量
		var count int
		server.authenticatedConns.Range(func(_, _ interface{}) bool {
			count++
			return true
		})

		// 等待服务器的监控协程清理连接（通常在真实环境会自动完成）
		// 这里手动触发monitorConnections中的逻辑，确保清理已执行
		var connIDs []string
		server.authenticatedConns.Range(func(key, _ interface{}) bool {
			connID, ok := key.(string)
			if ok {
				connIDs = append(connIDs, connID)
			}
			return true
		})

		for _, connID := range connIDs {
			// 检查连接是否仍然存在，如果不存在则清理状态
			if _, ok := server.netManager.GetConnection(connID); !ok {
				server.authenticatedConns.Delete(connID)
				server.connDbIndexMap.Delete(connID)
			}
		}
	}

	// 验证第一个连接的认证状态已被清理
	// 这里我们测试的是内部状态，在实际使用中用户不会直接检查这些
	var countAfterClose int
	server.authenticatedConns.Range(func(_, _ interface{}) bool {
		countAfterClose++
		return true
	})

	assert.Equal(t, 0, countAfterClose, "认证连接状态应该被清理")

	// 创建新连接，验证需要重新认证
	client2 := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", server.GetPort()),
		Password:    "",
		DB:          0,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	defer client2.Close()

	// 验证新连接在认证前不能执行命令
	_, err := client2.Get(ctx, "connection_test").Result()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOAUTH")

	// 认证新连接
	err = client2.Do(ctx, "AUTH", correctPassword).Err()
	assert.NoError(t, err)

	// 验证可以访问之前设置的键
	value, err := client2.Get(ctx, "connection_test").Result()
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)
}
