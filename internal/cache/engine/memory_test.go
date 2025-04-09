package engine

import (
	"github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/internal/cache/ds"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEngineManagement 测试引擎管理功能
func TestEngineManagement(t *testing.T) {
	// 创建默认配置并设置数据库数量
	config := cache.DefaultConfig()
	config.DBCount = 4
	engine := NewMemoryEngine(config)
	defer engine.Close()

	// 测试Select有效数据库
	db0, err := engine.Select(0)
	assert.NoError(t, err, "选择0号数据库应该成功")
	assert.NotNil(t, db0, "0号数据库不应为nil")

	// 测试Select其他有效数据库
	db1, err := engine.Select(1)
	assert.NoError(t, err, "选择1号数据库应该成功")
	assert.NotNil(t, db1, "1号数据库不应为nil")

	// 测试Select无效数据库索引
	_, err = engine.Select(-1)
	assert.Error(t, err, "选择负数索引应该失败")

	_, err = engine.Select(4) // 配置中设置的最大索引是3
	assert.Error(t, err, "选择超出范围的索引应该失败")

	// 测试数据库隔离性
	// 在db0中添加键
	strVal := ds.NewString("db0-value")
	db0.Set("test-key", strVal, 0)

	// 确认db0中有该键
	assert.True(t, db0.Exists("test-key"), "db0中应该存在test-key")

	// 确认db1中没有该键
	assert.False(t, db1.Exists("test-key"), "db1中不应该存在test-key")

	// 测试FlushAll
	// 在db1中添加键
	strVal = ds.NewString("db1-value")
	db1.Set("another-key", strVal, 0)

	// 确认db1中有该键
	assert.True(t, db1.Exists("another-key"), "db1中应该存在another-key")

	// 执行FlushAll
	engine.FlushAll()

	// 确认所有数据库都被清空
	assert.False(t, db0.Exists("test-key"), "FlushAll后db0中不应该存在test-key")
	assert.False(t, db1.Exists("another-key"), "FlushAll后db1中不应该存在another-key")
	assert.Equal(t, int64(0), db0.Size(), "FlushAll后db0大小应为0")
	assert.Equal(t, int64(0), db1.Size(), "FlushAll后db1大小应为0")
}

// TestEngineStats 测试引擎统计信息
func TestEngineStats(t *testing.T) {
	config := cache.DefaultConfig()
	engine := NewMemoryEngine(config)
	defer engine.Close()

	// 获取引擎统计信息
	stats := engine.GetStats()
	assert.NotNil(t, stats, "统计信息不应为nil")

	// 检查基本字段是否存在
	assert.Contains(t, stats, "uptime_in_seconds", "统计信息应包含uptime_in_seconds")
	assert.Contains(t, stats, "used_memory", "统计信息应包含used_memory")
	assert.Contains(t, stats, "db_count", "统计信息应包含db_count")
	assert.Contains(t, stats, "total_commands_processed", "统计信息应包含total_commands_processed")
	assert.Contains(t, stats, "keyspace", "统计信息应包含keyspace")

	// 检查数据库数量
	assert.Equal(t, 1, stats["db_count"], "默认应只有1个数据库")

	// 检查keyspace信息
	keyspace, ok := stats["keyspace"].(map[string]interface{})
	assert.True(t, ok, "keyspace应该是一个映射")
	assert.Contains(t, keyspace, "db0", "keyspace应包含db0")

	// 添加一些数据并再次检查统计信息
	db0, _ := engine.Select(0)
	for i := 0; i < 5; i++ {
		key := "key" + string(rune('0'+i))
		db0.Set(key, ds.NewString("value"+string(rune('0'+i))), 0)
	}

	// 执行一些命令增加命令计数
	cmd := NewCommand("PING", []string{}, "client1")
	db0.ProcessCommand(cmd)
	cmd = NewCommand("GET", []string{"key0"}, "client1")
	db0.ProcessCommand(cmd)

	// 获取更新后的统计信息
	updatedStats := engine.GetStats()
	assert.NotNil(t, updatedStats, "更新后的统计信息不应为nil")

	// 检查内存使用是否增加
	assert.Greater(t, updatedStats["used_memory"], stats["used_memory"], "添加数据后内存使用应增加")

	// 检查命令计数是否增加
	assert.Greater(t, updatedStats["total_commands_processed"], stats["total_commands_processed"], "执行命令后命令计数应增加")

	// 检查数据库键数
	updatedKeyspace, _ := updatedStats["keyspace"].(map[string]interface{})
	db0Info, _ := updatedKeyspace["db0"].(map[string]interface{})
	assert.Equal(t, int64(5), db0Info["keys"], "db0应有5个键")
}

// TestEngineMemoryManagement 测试引擎内存管理
func TestEngineMemoryManagement(t *testing.T) {
	// 创建有内存限制的配置
	config := cache.DefaultConfig()
	config.MaxMemory = 10 // 10MB
	engine := NewMemoryEngine(config)
	defer engine.Close()

	db0, _ := engine.Select(0)

	// 添加一些小数据
	for i := 0; i < 100; i++ {
		key := "small-key" + string(rune('0'+i%10))
		db0.Set(key, ds.NewString("small-value"), 0)
	}

	// 检查键数量
	assert.Equal(t, int64(10), db0.Size(), "应有10个键（每个键被覆盖9次）")

	// 现在添加一些大数据，模拟接近内存限制的情况
	// 注意：实际测试中可能需要更大的数据量，这里只是示例
	largeBuf := make([]byte, 1024*1024) // 1MB
	for i := 0; i < 8; i++ {
		key := "large-key" + string(rune('0'+i))
		db0.Set(key, ds.NewString(string(largeBuf)), 0)
	}

	// 获取统计信息检查内存使用
	stats := engine.GetStats()
	usedMemory, _ := stats["used_memory"].(int64)
	assert.Greater(t, usedMemory, int64(0), "内存使用量应大于0")

	// 注意：由于内存限制的实际行为依赖于淘汰策略的实现
	// 以下断言可能需要根据实际实现调整
	if config.MaxMemory > 0 && usedMemory > config.MaxMemory*1024*1024 {
		assert.Fail(t, "内存使用量超过了配置的最大限制")
	}
}
