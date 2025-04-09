package engine

import (
	"github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestPersistenceManagerCreation 测试持久化管理器的创建
func TestPersistenceManagerCreation(t *testing.T) {
	// 准备测试环境
	config := cache.DefaultConfig()
	config.EnableRDB = true
	config.EnableAOF = true
	config.RDBFilePath = "./test_rdb.rdb"
	config.AOFFilePath = "./test_aof.aof"

	// 先尝试删除可能存在的旧文件
	os.Remove(config.RDBFilePath)
	os.Remove(config.AOFFilePath)

	db := NewMemoryDatabase(0, &config)
	defer db.Close()
	defer os.Remove(config.RDBFilePath)
	defer os.Remove(config.AOFFilePath)

	// 创建持久化管理器
	var wg sync.WaitGroup
	pm, err := NewPersistenceManager(db, &wg)
	assert.NoError(t, err, "创建持久化管理器不应返回错误")
	assert.NotNil(t, pm, "持久化管理器不应为nil")
	assert.NotNil(t, pm.rdbManager, "RDB管理器不应为nil")
	assert.NotNil(t, pm.aofManager, "AOF管理器不应为nil")

	// 测试启动和关闭
	err = pm.Start()
	assert.NoError(t, err, "启动持久化管理器不应返回错误")

	err = pm.Close()
	assert.NoError(t, err, "关闭持久化管理器不应返回错误")
}

// TestRDBSaveAndLoad 测试RDB保存和加载功能
func TestRDBSaveAndLoad(t *testing.T) {
	// 准备测试环境
	config := cache.DefaultConfig()
	config.EnableRDB = true
	config.EnableAOF = false
	config.RDBFilePath = "./test_rdb_save_load.rdb"

	// 先尝试删除可能存在的旧文件
	os.Remove(config.RDBFilePath)

	db := NewMemoryDatabase(0, &config)
	defer db.Close()
	defer os.Remove(config.RDBFilePath)

	// 创建持久化管理器
	var wg sync.WaitGroup
	pm, err := NewPersistenceManager(db, &wg)
	assert.NoError(t, err, "创建持久化管理器不应返回错误")

	// 添加一些测试数据
	db.Set("string1", ds2.NewString("value1"), 0)

	// 设置带过期时间的键，在1小时后过期
	db.Set("string2", ds2.NewString("value2"), time.Hour)

	list := ds2.NewList()
	list.RPush("item1", "item2", "item3")
	db.Set("list1", list, 0)

	hash := ds2.NewHash()
	hash.Set("field1", "value1")
	hash.Set("field2", "value2")
	db.Set("hash1", hash, 0)

	set := ds2.NewSet()
	set.Add("member1", "member2", "member3")
	db.Set("set1", set, 0)

	zset := ds2.NewZSet()
	zset.Add(1.0, "member1")
	zset.Add(2.0, "member2")
	zset.Add(3.0, "member3")
	db.Set("zset1", zset, 0)

	// 保存RDB
	err = pm.SaveRDB()
	assert.NoError(t, err, "保存RDB不应返回错误")

	// 创建一个新的数据库实例
	db2 := NewMemoryDatabase(0, &config)
	defer db2.Close()

	// 为新数据库创建持久化管理器
	pm2, err := NewPersistenceManager(db2, &wg)
	assert.NoError(t, err, "创建第二个持久化管理器不应返回错误")

	// 启动管理器(会从RDB加载数据)
	err = pm2.Start()
	assert.NoError(t, err, "启动第二个持久化管理器不应返回错误")

	// 验证数据加载正确
	val, exists := db2.Get("string1")
	assert.True(t, exists, "string1应该存在")
	assert.Equal(t, "value1", val.(*ds2.String).String(), "string1的值应该是value1")

	val, exists = db2.Get("list1")
	assert.True(t, exists, "list1应该存在")
	assert.Equal(t, int64(3), val.(*ds2.List).Len(), "list1应该有3个元素")

	val, exists = db2.Get("hash1")
	assert.True(t, exists, "hash1应该存在")
	assert.Equal(t, int64(2), val.(*ds2.Hash).Len(), "hash1应该有2个字段")

	val, exists = db2.Get("set1")
	assert.True(t, exists, "set1应该存在")
	assert.Equal(t, int64(3), val.(*ds2.Set).Len(), "set1应该有3个成员")

	val, exists = db2.Get("zset1")
	assert.True(t, exists, "zset1应该存在")
	assert.Equal(t, int64(3), val.(*ds2.ZSet).Len(), "zset1应该有3个成员")

	// 验证过期时间设置正确
	assert.True(t, db2.Exists("string2"), "string2应该存在")

	// 检查TTL是否大于0
	ttl := db2.TTL("string2")
	assert.True(t, ttl > 0, "string2的TTL应该大于0")

	// 测试关闭
	err = pm2.Close()
	assert.NoError(t, err, "关闭第二个持久化管理器不应返回错误")
}

// TestAOFWriteAndReplay 测试AOF写入和重放
func TestAOFWriteAndReplay(t *testing.T) {
	// 准备测试环境
	config := cache.DefaultConfig()
	config.EnableAOF = true
	config.EnableRDB = false
	config.AOFFilePath = "./test_aof_write_replay.aof"
	config.AOFSyncStrategy = 2

	// 先尝试删除可能存在的旧文件
	os.Remove(config.AOFFilePath)
	defer os.Remove(config.AOFFilePath)

	var wg sync.WaitGroup

	// 创建一个新的数据库实例
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 清空数据库以确保测试环境干净
	db.Flush()

	// 为数据库创建持久化管理器
	pm, err := NewPersistenceManager(db, &wg)
	assert.NoError(t, err, "创建持久化管理器不应返回错误")

	// 启动管理器
	err = pm.Start()
	assert.NoError(t, err, "启动持久化管理器不应返回错误")

	// 添加一些测试数据
	cmd1 := NewCommand("SET", []string{"key1", "value1"}, "client1")
	db.ProcessCommand(cmd1)

	cmd2 := NewCommand("LPUSH", []string{"list1", "a", "b"}, "client1")
	db.ProcessCommand(cmd2)

	cmd3 := NewCommand("SADD", []string{"set1", "x", "y"}, "client1")
	db.ProcessCommand(cmd3)

	cmd4 := NewCommand("HSET", []string{"hash1", "field1", "value1"}, "client1")
	db.ProcessCommand(cmd4)

	cmd5 := NewCommand("ZADD", []string{"zset1", "1", "member1"}, "client1")
	db.ProcessCommand(cmd5)

	// 确保所有数据都已写入AOF文件
	err = pm.SyncAOF()
	assert.NoError(t, err, "同步AOF文件不应返回错误")

	// 关闭持久化管理器
	err = pm.Close()
	assert.NoError(t, err, "关闭持久化管理器不应返回错误")

	// 等待一段时间，确保文件操作完成
	time.Sleep(100 * time.Millisecond)

	// 创建一个新的数据库实例
	db2 := NewMemoryDatabase(0, &config)
	defer db2.Close()

	// 清空数据库以确保测试环境干净
	db2.Flush()

	// 为新数据库创建持久化管理器
	pm2, err := NewPersistenceManager(db2, &wg)
	assert.NoError(t, err, "创建第二个持久化管理器不应返回错误")

	// 启动管理器(会重放AOF文件)
	err = pm2.Start()
	assert.NoError(t, err, "启动第二个持久化管理器不应返回错误")

	// 添加延迟，确保AOF重放完成
	time.Sleep(200 * time.Millisecond)

	// 验证数据重放正确
	assert.True(t, db2.Exists("key1"), "key1应该存在")
	assert.True(t, db2.Exists("list1"), "list1应该存在")
	assert.True(t, db2.Exists("set1"), "set1应该存在")
	assert.True(t, db2.Exists("hash1"), "hash1应该存在")
	assert.True(t, db2.Exists("zset1"), "zset1应该存在")

	// 验证值正确
	val, exists := db2.Get("key1")
	assert.True(t, exists, "key1应该能够被获取")
	if exists {
		assert.Equal(t, "value1", val.(*ds2.String).String(), "key1的值应该是value1")
	}

	val, exists = db2.Get("list1")
	assert.True(t, exists, "list1应该能够被获取")
	if exists {
		assert.Equal(t, int64(2), val.(*ds2.List).Len(), "list1应该有2个元素")
	}

	val, exists = db2.Get("set1")
	assert.True(t, exists, "set1应该能够被获取")
	if exists {
		assert.Equal(t, int64(2), val.(*ds2.Set).Len(), "set1应该有2个成员")
	}

	val, exists = db2.Get("hash1")
	assert.True(t, exists, "hash1应该能够被获取")
	if exists {
		assert.Equal(t, int64(1), val.(*ds2.Hash).Len(), "hash1应该有1个字段")
	}

	val, exists = db2.Get("zset1")
	assert.True(t, exists, "zset1应该能够被获取")
	if exists {
		assert.Equal(t, int64(1), val.(*ds2.ZSet).Len(), "zset1应该有1个成员")
	}

	// 测试AOF重写
	rewriteDone := make(chan struct{})
	go func() {
		pm2.TriggerAOFRewrite()
		time.Sleep(200 * time.Millisecond) // 延长等待时间，确保重写完成
		close(rewriteDone)
	}()

	// 等待重写完成
	select {
	case <-rewriteDone:
		// 重写完成
	case <-time.After(5 * time.Second):
		t.Fatal("AOF重写超时")
	}

	// 添加一些新数据
	cmd6 := NewCommand("SET", []string{"key2", "value2"}, "client1")
	db2.ProcessCommand(cmd6)

	// 确保该命令写入AOF文件并同步到磁盘
	err = pm2.WriteCommandAndSync("SET", []string{"key2", "value2"})
	assert.NoError(t, err, "写入并同步命令不应返回错误")

	// 延长等待时间，确保文件操作完成
	time.Sleep(200 * time.Millisecond)

	// 关闭第二个持久化管理器
	err = pm2.Close()
	assert.NoError(t, err, "关闭第二个持久化管理器不应返回错误")

	// 等待文件操作完成
	time.Sleep(200 * time.Millisecond)

	// 创建第三个数据库和持久化管理器
	db3 := NewMemoryDatabase(0, &config)
	defer db3.Close()

	// 清空数据库以确保测试环境干净
	db3.Flush()

	pm3, err := NewPersistenceManager(db3, &wg)
	assert.NoError(t, err, "创建第三个持久化管理器不应返回错误")

	// 启动第三个管理器(重放重写后的AOF)
	err = pm3.Start()
	assert.NoError(t, err, "启动第三个持久化管理器不应返回错误")

	// 延长等待时间，确保AOF重放完成
	time.Sleep(200 * time.Millisecond)

	// 验证所有数据都存在
	assert.True(t, db3.Exists("key1"), "key1应该存在")
	assert.True(t, db3.Exists("key2"), "key2应该存在")
	assert.True(t, db3.Exists("list1"), "list1应该存在")
	assert.True(t, db3.Exists("set1"), "set1应该存在")
	assert.True(t, db3.Exists("hash1"), "hash1应该存在")
	assert.True(t, db3.Exists("zset1"), "zset1应该存在")

	// 关闭第三个持久化管理器
	err = pm3.Close()
	assert.NoError(t, err, "关闭第三个持久化管理器不应返回错误")
}

// TestRDBAndAOFTogether 测试RDB和AOF同时启用
func TestRDBAndAOFTogether(t *testing.T) {
	// 准备测试环境
	config := cache.DefaultConfig()
	config.EnableRDB = true
	config.EnableAOF = true
	config.RDBFilePath = "./test_both.rdb"
	config.AOFFilePath = "./test_both.aof"

	// 先尝试删除可能存在的旧文件
	os.Remove(config.RDBFilePath)
	os.Remove(config.AOFFilePath)

	db := NewMemoryDatabase(0, &config)
	defer db.Close()
	defer os.Remove(config.RDBFilePath)
	defer os.Remove(config.AOFFilePath)

	// 创建持久化管理器
	var wg sync.WaitGroup
	pm, err := NewPersistenceManager(db, &wg)
	assert.NoError(t, err, "创建持久化管理器不应返回错误")

	// 启动管理器
	err = pm.Start()
	assert.NoError(t, err, "启动持久化管理器不应返回错误")

	// 添加一些测试数据
	db.Set("key1", ds2.NewString("value1"), 0)

	// 等待一段时间确保RDB保存
	time.Sleep(100 * time.Millisecond)

	// 执行命令，会写入AOF
	cmd := NewCommand("SET", []string{"key2", "value2"}, "client1")
	db.ProcessCommand(cmd)

	// 等待一段时间确保AOF写入
	time.Sleep(100 * time.Millisecond)

	// 关闭持久化管理器
	err = pm.Close()
	assert.NoError(t, err, "关闭持久化管理器不应返回错误")

	// 创建第二个数据库实例
	db2 := NewMemoryDatabase(0, &config)
	defer db2.Close()

	// 为新数据库创建持久化管理器
	pm2, err := NewPersistenceManager(db2, &wg)
	assert.NoError(t, err, "创建第二个持久化管理器不应返回错误")

	// 启动管理器(将从RDB加载数据并从AOF重放命令)
	err = pm2.Start()
	assert.NoError(t, err, "启动第二个持久化管理器不应返回错误")

	// 等待一段时间确保加载完成
	time.Sleep(100 * time.Millisecond)

	// 验证数据加载正确
	assert.True(t, db2.Exists("key1"), "key1应该存在")
	assert.True(t, db2.Exists("key2"), "key2应该存在")

	// 验证值正确
	val, exists := db2.Get("key1")
	assert.True(t, exists, "key1应该能够被获取")
	if exists {
		assert.Equal(t, "value1", val.(*ds2.String).String(), "key1的值应该是value1")
	}

	val, exists = db2.Get("key2")
	assert.True(t, exists, "key2应该能够被获取")
	if exists {
		assert.Equal(t, "value2", val.(*ds2.String).String(), "key2的值应该是value2")
	}

	// 关闭第二个持久化管理器
	err = pm2.Close()
	assert.NoError(t, err, "关闭第二个持久化管理器不应返回错误")
}

// TestDisabledPersistence 测试禁用持久化
func TestDisabledPersistence(t *testing.T) {
	// 准备测试环境
	config := cache.DefaultConfig()
	config.EnableRDB = false
	config.EnableAOF = false
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 创建持久化管理器
	var wg sync.WaitGroup
	pm, err := NewPersistenceManager(db, &wg)
	assert.NoError(t, err, "创建持久化管理器不应返回错误")

	// 启动管理器
	err = pm.Start()
	assert.NoError(t, err, "启动持久化管理器不应返回错误")

	// 添加一些测试数据
	db.Set("key1", ds2.NewString("value1"), 0)

	// 尝试保存RDB(应该不做任何事)
	err = pm.SaveRDB()
	assert.NoError(t, err, "当RDB禁用时，SaveRDB不应返回错误")

	// 尝试写入AOF(应该不做任何事)
	err = pm.WriteAOF("SET", []string{"key1", "value1"})
	assert.NoError(t, err, "当AOF禁用时，WriteAOF不应返回错误")

	// 关闭管理器
	err = pm.Close()
	assert.NoError(t, err, "关闭持久化管理器不应返回错误")
}
