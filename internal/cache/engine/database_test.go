package engine

import (
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDatabaseBasicOperations 测试数据库基本操作
func TestDatabaseBasicOperations(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试初始状态
	assert.Equal(t, int64(0), db.Size(), "初始数据库大小应为0")

	// 测试Set和Get操作
	strVal := ds2.NewString("test-value")
	success := db.Set("test-key", strVal, 0)
	assert.True(t, success, "Set操作应该成功")

	val, exists := db.Get("test-key")
	assert.True(t, exists, "应该找到设置的键")
	assert.Equal(t, "test-value", val.(*ds2.String).String(), "获取的值应该与设置的值匹配")

	// 测试Exists操作
	assert.True(t, db.Exists("test-key"), "Exists应该返回true")
	assert.False(t, db.Exists("non-existent-key"), "Exists应该对不存在的键返回false")

	// 测试不同数据类型
	// 列表类型
	listVal := ds2.NewList()
	listVal.RPush("item1", "item2")
	success = db.Set("list-key", listVal, 0)
	assert.True(t, success, "设置列表应该成功")

	val, exists = db.Get("list-key")
	assert.True(t, exists, "应该找到列表键")
	assert.Equal(t, int64(2), val.(*ds2.List).Len(), "列表长度应为2")

	// 哈希表类型
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	success = db.Set("hash-key", hashVal, 0)
	assert.True(t, success, "设置哈希表应该成功")

	val, exists = db.Get("hash-key")
	assert.True(t, exists, "应该找到哈希表键")
	assert.Equal(t, int64(1), val.(*ds2.Hash).Len(), "哈希表长度应为1")

	// 测试Delete操作
	count := db.Delete("test-key", "non-existent-key")
	assert.Equal(t, int64(1), count, "Delete应该删除1个键")
	assert.False(t, db.Exists("test-key"), "键应该被删除")
	assert.Equal(t, int64(2), db.Size(), "数据库大小应为2")

	// 测试删除多个键
	count = db.Delete("list-key", "hash-key")
	assert.Equal(t, int64(2), count, "Delete应该删除2个键")
	assert.Equal(t, int64(0), db.Size(), "数据库大小应为0")
}

// TestDatabaseExpiry 测试过期策略
func TestDatabaseExpiry(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试设置过期时间
	strVal := ds2.NewString("test-value")
	success := db.Set("test-key", strVal, 100*time.Millisecond)
	assert.True(t, success, "Set操作应该成功")

	// 测试TTL
	ttl := db.TTL("test-key")
	assert.True(t, ttl > 0 && ttl <= 100*time.Millisecond, "TTL应该在有效范围内")

	// 测试未过期时键应该存在
	assert.True(t, db.Exists("test-key"), "未过期的键应该存在")

	// 等待键过期
	time.Sleep(200 * time.Millisecond)

	// 测试过期后键应该被删除
	assert.False(t, db.Exists("test-key"), "过期的键应该被删除")

	// 测试设置无过期时间
	success = db.Set("permanent-key", strVal, 0)
	assert.True(t, success, "设置永久键应该成功")

	// 测试永久键的TTL
	ttl = db.TTL("permanent-key")
	assert.Equal(t, -1*time.Second, ttl, "永久键的TTL应该是-1")

	// 测试更新过期时间
	success = db.Expire("permanent-key", 500*time.Millisecond)
	assert.True(t, success, "更新过期时间应该成功")

	// 测试更新后的TTL
	ttl = db.TTL("permanent-key")
	assert.True(t, ttl > 0 && ttl <= 500*time.Millisecond, "更新后的TTL应该在有效范围内")

	// 测试对不存在键设置过期时间
	success = db.Expire("non-existent-key", 1*time.Second)
	assert.False(t, success, "对不存在的键设置过期时间应该失败")
}

// TestDatabaseKeySpace 测试键空间操作
func TestDatabaseKeySpace(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 添加一些测试数据
	keys := []string{"key1", "key2", "test1", "test2", "another"}
	for _, key := range keys {
		db.Set(key, ds2.NewString(key+"-value"), 0)
	}

	// 测试键数量
	assert.Equal(t, int64(5), db.Size(), "数据库大小应为5")

	// 测试Keys操作，精确匹配
	result := db.Keys("key1")
	assert.Equal(t, 1, len(result), "应该只匹配一个键")
	assert.Equal(t, "key1", result[0], "应该匹配key1")

	// 测试前缀匹配
	result = db.Keys("key.*")
	assert.Equal(t, 2, len(result), "应该匹配2个键")
	assert.Contains(t, result, "key1", "应该包含key1")
	assert.Contains(t, result, "key2", "应该包含key2")

	// 测试后缀匹配
	result = db.Keys(".*2")
	assert.Equal(t, 2, len(result), "应该匹配2个键")
	assert.Contains(t, result, "key2", "应该包含key2")
	assert.Contains(t, result, "test2", "应该包含test2")

	// 测试包含匹配
	result = db.Keys(".*es.*")
	assert.Equal(t, 2, len(result), "应该匹配2个键")
	assert.Contains(t, result, "test1", "应该包含test1")
	assert.Contains(t, result, "test2", "应该包含test2")

	// 测试通配符匹配所有
	result = db.Keys(".*")
	assert.Equal(t, 5, len(result), "应该匹配所有5个键")

	// 测试Flush操作
	db.Flush()
	assert.Equal(t, int64(0), db.Size(), "Flush后数据库大小应为0")
	result = db.Keys("*")
	assert.Equal(t, 0, len(result), "Flush后应该没有键")
}

// TestDatabaseCommandProcessing 测试命令处理
func TestDatabaseCommandProcessing(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试PING命令
	cmd := NewCommand("PING", []string{}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "PING应该返回状态回复")
	assert.Equal(t, "PONG", reply.String(), "PING应该返回PONG")

	// 测试SET命令
	cmd = NewCommand("SET", []string{"test-key", "test-value"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "SET应该返回状态回复")
	assert.Equal(t, "OK", reply.String(), "SET应该返回OK")

	// 测试GET命令
	cmd = NewCommand("GET", []string{"test-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "GET应该返回批量回复")
	assert.Equal(t, "test-value", reply.String(), "GET应该返回设置的值")

	// 测试GET不存在的键
	cmd = NewCommand("GET", []string{"non-existent-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "GET不存在的键应该返回nil回复")

	// 测试DEL命令
	cmd = NewCommand("DEL", []string{"test-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "DEL应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "DEL应该返回删除的键数量")

	// 测试ERROR情况
	cmd = NewCommand("GET", []string{}, "client1") // 缺少参数
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}
