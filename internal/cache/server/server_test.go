package server

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建测试服务器
func setupTestServer(t *testing.T) (*Server, *redis.Client, func()) {
	// 创建服务器配置
	config := cache.DefaultConfig()
	// 使用随机端口，避免端口冲突
	config.Port = 16379
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

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:    config.Password,
		DB:          0,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})

	// 等待连接成功，增加重试次数和间隔
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lastErr error
	for i := 0; i < 20; i++ {
		pong, err := client.Ping(ctx).Result()
		if err == nil {
			t.Logf("成功连接到Redis服务器: %s", pong)
			break
		}
		lastErr = err
		t.Logf("尝试 %d: 连接失败: %v", i+1, err)
		time.Sleep(250 * time.Millisecond)
	}

	// 如果最后一次尝试仍然失败，则报告错误
	if lastErr != nil {
		t.Fatalf("无法连接到Redis服务器: %v", lastErr)
	}

	// 返回清理函数
	cleanup := func() {
		client.Close()
		server.Stop(nil)
	}

	return server, client, cleanup
}

// 测试服务器启动和停止
func TestServerStartStop(t *testing.T) {
	// 创建服务器配置
	config := cache.DefaultConfig()
	config.Port = 16379
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

	// 给服务器启动时间
	time.Sleep(1 * time.Second)

	// 测试服务器是否正常运行
	stats := server.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "uptime_in_seconds")

	// 停止服务器
	err = server.Stop(nil)
	assert.NoError(t, err)
}

// 测试基本命令
func TestBasicCommands(t *testing.T) {
	// 创建服务器配置
	config := cache.DefaultConfig()
	config.Port = 16379
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

	// 给服务器启动时间
	time.Sleep(1 * time.Second)

	// 测试服务器是否正常运行
	stats := server.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "uptime_in_seconds")

	// 停止服务器
	err = server.Stop(nil)
	assert.NoError(t, err)
}

// 测试字符串操作
func TestStringOperations(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试INCR
	err := client.Set(ctx, "counter", "10", 0).Err()
	assert.NoError(t, err)

	n, err := client.Incr(ctx, "counter").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(11), n)

	// 测试INCRBY
	n, err = client.IncrBy(ctx, "counter", 5).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(16), n)

	// 测试DECR
	n, err = client.Decr(ctx, "counter").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(15), n)

	// 测试DECRBY
	n, err = client.DecrBy(ctx, "counter", 5).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(10), n)

	// 测试APPEND
	err = client.Set(ctx, "mystring", "Hello", 0).Err()
	assert.NoError(t, err)

	n, err = client.Append(ctx, "mystring", " World").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(11), n)

	val, err := client.Get(ctx, "mystring").Result()
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", val)

	// 测试STRLEN
	n, err = client.StrLen(ctx, "mystring").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(11), n)
}

// 测试哈希操作
func TestHashOperations(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试HSET和HGET
	err := client.HSet(ctx, "user:1", "name", "张三", "age", "25", "city", "北京").Err()
	assert.NoError(t, err)

	val, err := client.HGet(ctx, "user:1", "name").Result()
	assert.NoError(t, err)
	assert.Equal(t, "张三", val)

	// 测试HGETALL
	all, err := client.HGetAll(ctx, "user:1").Result()
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"name": "张三", "age": "25", "city": "北京"}, all)

	// 测试HEXISTS
	exists, err := client.HExists(ctx, "user:1", "name").Result()
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试HDEL
	n, err := client.HDel(ctx, "user:1", "city").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// 测试HINCRBY
	n, err = client.HIncrBy(ctx, "user:1", "age", 1).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(26), n)

	// 测试HLEN
	n, err = client.HLen(ctx, "user:1").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

// 测试列表操作
func TestListOperations(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试LPUSH和LRANGE
	n, err := client.LPush(ctx, "mylist", "world", "hello").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)

	vals, err := client.LRange(ctx, "mylist", 0, -1).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, vals)

	// 测试RPUSH
	n, err = client.RPush(ctx, "mylist", "ending").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)

	// 测试LPOP
	val, err := client.LPop(ctx, "mylist").Result()
	assert.NoError(t, err)
	assert.Equal(t, "hello", val)

	// 测试RPOP
	val, err = client.RPop(ctx, "mylist").Result()
	assert.NoError(t, err)
	assert.Equal(t, "ending", val)

	// 测试LLEN
	n, err = client.LLen(ctx, "mylist").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

// 测试集合操作
func TestSetOperations(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试SADD和SMEMBERS
	n, err := client.SAdd(ctx, "myset", "one", "two", "three").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)

	members, err := client.SMembers(ctx, "myset").Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"one", "two", "three"}, members)

	// 测试SISMEMBER
	isMember, err := client.SIsMember(ctx, "myset", "one").Result()
	assert.NoError(t, err)
	assert.True(t, isMember)

	// 测试SCARD
	n, err = client.SCard(ctx, "myset").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)

	// 测试SREM
	n, err = client.SRem(ctx, "myset", "one").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// 测试集合操作
	n, err = client.SAdd(ctx, "set1", "a", "b", "c").Result()
	assert.NoError(t, err)
	n, err = client.SAdd(ctx, "set2", "c", "d", "e").Result()
	assert.NoError(t, err)

	// 测试SUNION
	union, err := client.SUnion(ctx, "set1", "set2").Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b", "c", "d", "e"}, union)

	// 测试SINTER
	inter, err := client.SInter(ctx, "set1", "set2").Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"c"}, inter)

	// 测试SDIFF
	diff, err := client.SDiff(ctx, "set1", "set2").Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b"}, diff)
}

// 测试有序集合操作
func TestSortedSetOperations(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试ZADD和ZRANGE
	n, err := client.ZAdd(ctx, "scores", redis.Z{Score: 90, Member: "Alice"}, redis.Z{Score: 80, Member: "Bob"}, redis.Z{Score: 70, Member: "Charlie"}).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)

	members, err := client.ZRange(ctx, "scores", 0, -1).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"Charlie", "Bob", "Alice"}, members)

	// 测试ZREVRANGE
	members, err = client.ZRevRange(ctx, "scores", 0, -1).Result()
	assert.NoError(t, err)
	assert.Equal(t, []string{"Alice", "Bob", "Charlie"}, members)

	// 测试ZRANK
	rank, err := client.ZRank(ctx, "scores", "Bob").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rank)

	// 测试ZSCORE
	score, err := client.ZScore(ctx, "scores", "Alice").Result()
	assert.NoError(t, err)
	assert.Equal(t, float64(90), score)

	// 测试ZINCRBY
	score, err = client.ZIncrBy(ctx, "scores", 5, "Bob").Result()
	assert.NoError(t, err)
	assert.Equal(t, float64(85), score)

	// 测试ZCARD
	n, err = client.ZCard(ctx, "scores").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)

	// 测试ZREM
	n, err = client.ZRem(ctx, "scores", "Charlie").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

// 测试数据库选择
func TestDatabaseSelect(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 在DB 0中设置一个键
	err := client.Set(ctx, "db0_key", "db0_value", 0).Err()
	assert.NoError(t, err)

	// 切换到DB 1
	err = client.Do(ctx, "SELECT", 1).Err()
	assert.NoError(t, err)

	// 确认DB 0的键在DB 1中不可见
	_, err = client.Get(ctx, "db0_key").Result()
	assert.Equal(t, redis.Nil, err)

	// 在DB 1中设置一个键
	err = client.Set(ctx, "db1_key", "db1_value", 0).Err()
	assert.NoError(t, err)

	// 切回DB 0
	err = client.Do(ctx, "SELECT", 0).Err()
	assert.NoError(t, err)

	// 确认DB 0的键可见
	val, err := client.Get(ctx, "db0_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "db0_value", val)

	// 确认DB 1的键在DB 0中不可见
	_, err = client.Get(ctx, "db1_key").Result()
	assert.Equal(t, redis.Nil, err)
}

// 测试服务器命令
func TestServerCommands(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试INFO命令
	info, err := client.Info(ctx).Result()
	assert.NoError(t, err)
	assert.Contains(t, info, "uptime_in_seconds")
	assert.Contains(t, info, "connected_clients")

	// 设置一些键以测试FLUSHALL
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		err = client.Set(ctx, key, fmt.Sprintf("value%d", i), 0).Err()
		assert.NoError(t, err)
	}

	// 测试FLUSHALL
	err = client.FlushAll(ctx).Err()
	assert.NoError(t, err)

	// 验证所有键都被删除
	keys, err := client.Keys(ctx, "*").Result()
	assert.NoError(t, err)

	// 打印出剩余的键和长度
	t.Logf("FLUSHALL后剩余的键: %v, 长度: %d, 类型: %T", keys, len(keys), keys)

	// 直接检查每个键
	for i, key := range keys {
		t.Logf("键[%d]: '%s', 长度: %d", i, key, len(key))
	}

	// 过滤掉空字符串键
	var nonEmptyKeys []string
	for _, key := range keys {
		if len(key) > 0 {
			nonEmptyKeys = append(nonEmptyKeys, key)
		}
	}

	// 验证没有非空键
	assert.Equal(t, 0, len(nonEmptyKeys), "FLUSHALL后应该没有非空键")

	// 检查是否有空字符串键，如果有，这也是一个问题
	if len(keys) > 0 {
		// 检查是否有空字符串键
		hasEmptyKey := false
		for _, key := range keys {
			if len(key) == 0 {
				hasEmptyKey = true
				break
			}
		}

		// 如果有空字符串键，也应该报告错误
		assert.False(t, hasEmptyKey, "FLUSHALL后不应该有空字符串键")
	}
}

// 测试错误处理
func TestErrorHandling(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试对无效命令的错误处理
	err := client.Do(ctx, "INVALID_COMMAND").Err()
	assert.Error(t, err)

	// 测试参数错误
	err = client.Do(ctx, "GET").Err()
	assert.Error(t, err)

	// 测试类型错误
	err = client.Set(ctx, "string_key", "value", 0).Err()
	assert.NoError(t, err)

	err = client.HGet(ctx, "string_key", "field").Err()
	assert.Error(t, err)
}

// 测试性能和并发
func TestPerformanceAndConcurrency(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 并发写入和读取
	const goroutines = 10
	const keysPerGoroutine = 100

	// 创建等待组
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// 启动多个goroutine进行写入
	for g := 0; g < goroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < keysPerGoroutine; i++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, i)
				value := fmt.Sprintf("value_%d_%d", goroutineID, i)

				err := client.Set(ctx, key, value, 0).Err()
				if err != nil {
					t.Errorf("并发写入错误: %v", err)
					return
				}

				// 读取刚写入的值
				val, err := client.Get(ctx, key).Result()
				if err != nil {
					t.Errorf("并发读取错误: %v", err)
					return
				}
				if val != value {
					t.Errorf("期望值 %s, 实际值 %s", value, val)
					return
				}
			}
		}(g)
	}

	// 等待所有goroutine完成
	wg.Wait()

	// 检查所有键是否存在
	for g := 0; g < goroutines; g++ {
		for i := 0; i < keysPerGoroutine; i++ {
			key := fmt.Sprintf("key_%d_%d", g, i)
			exists, err := client.Exists(ctx, key).Result()
			assert.NoError(t, err)
			assert.Equal(t, int64(1), exists)
		}
	}
}

// 测试边界情况
func TestEdgeCases(t *testing.T) {
	_, client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// 测试空键和空值
	err := client.Set(ctx, "", "empty_key", 0).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, "").Result()
	assert.NoError(t, err)
	assert.Equal(t, "empty_key", val)

	err = client.Set(ctx, "empty_value", "", 0).Err()
	assert.NoError(t, err)

	val, err = client.Get(ctx, "empty_value").Result()
	assert.NoError(t, err)
	assert.Equal(t, "", val)

	// 测试大键和大值
	largeValue := make([]byte, 1024*1024) // 1MB
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	err = client.Set(ctx, "large_value", largeValue, 0).Err()
	assert.NoError(t, err)

	// 测试极短超时
	err = client.Set(ctx, "short_ttl", "value", 1*time.Millisecond).Err()
	assert.NoError(t, err)

	// 等待过期
	time.Sleep(10 * time.Millisecond)

	_, err = client.Get(ctx, "short_ttl").Result()
	assert.Equal(t, redis.Nil, err)

	// 测试极长键名
	longKey := string(make([]byte, 1024)) // 1KB键名
	err = client.Set(ctx, longKey, "long_key_value", 0).Err()
	assert.NoError(t, err)

	val, err = client.Get(ctx, longKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "long_key_value", val)
}
