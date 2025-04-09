package engine

import (
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExecHSet 测试HSET命令
func TestExecHSet(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试在不存在的键上HSET
	cmd := NewCommand("HSET", []string{"hash1", "field1", "value1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HSET应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "首次设置字段应该返回1")

	// 验证值
	val, exists := db.Get("hash1")
	assert.True(t, exists, "hash1应该存在")
	assert.Equal(t, cache2.TypeHash, val.Type(), "hash1应该是哈希表类型")
	assert.Equal(t, int64(1), val.(*ds2.Hash).Len(), "hash1应该有1个字段")

	// 测试在已存在哈希表上HSET新字段
	cmd = NewCommand("HSET", []string{"hash1", "field2", "value2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HSET应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "设置新字段应该返回1")

	// 测试更新已存在的字段
	cmd = NewCommand("HSET", []string{"hash1", "field1", "new-value1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HSET应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "更新已存在的字段应该返回0")

	// 验证更新后的值
	val, _ = db.Get("hash1")
	fieldValue, exists := val.(*ds2.Hash).Get("field1")
	assert.True(t, exists, "field1应该存在")
	assert.Equal(t, "new-value1", fieldValue, "field1的值应该被更新")

	// 测试参数不足
	cmd = NewCommand("HSET", []string{"hash1", "field1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HSET
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HSET", []string{"string1", "field", "value"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HSET应该返回错误")
}

// TestExecHSetNX 测试HSETNX命令
func TestExecHSetNX(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试在不存在的键上HSETNX
	cmd := NewCommand("HSETNX", []string{"hash1", "field1", "value1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HSETNX应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功设置应该返回1")

	// 验证值
	val, exists := db.Get("hash1")
	assert.True(t, exists, "hash1应该存在")
	assert.Equal(t, cache2.TypeHash, val.Type(), "hash1应该是哈希表类型")
	fieldValue, exists := val.(*ds2.Hash).Get("field1")
	assert.True(t, exists, "field1应该存在")
	assert.Equal(t, "value1", fieldValue, "field1的值应该是value1")

	// 测试尝试更新已存在的字段
	cmd = NewCommand("HSETNX", []string{"hash1", "field1", "new-value1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HSETNX应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "字段已存在时应该返回0")

	// 验证值未被更新
	val, _ = db.Get("hash1")
	fieldValue, _ = val.(*ds2.Hash).Get("field1")
	assert.Equal(t, "value1", fieldValue, "field1的值不应该被更新")

	// 测试在已存在哈希表上HSETNX新字段
	cmd = NewCommand("HSETNX", []string{"hash1", "field2", "value2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HSETNX应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功设置新字段应该返回1")

	// 测试参数不足
	cmd = NewCommand("HSETNX", []string{"hash1", "field1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HSETNX
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HSETNX", []string{"string1", "field", "value"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HSETNX应该返回错误")
}

// TestExecHGet 测试HGET命令
func TestExecHGet(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	db.Set("hash1", hashVal, 0)

	// 测试获取存在的字段
	cmd := NewCommand("HGET", []string{"hash1", "field1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "HGET应该返回批量回复")
	assert.Equal(t, "value1", reply.String(), "HGET应该返回字段的值")

	// 测试获取不存在的字段
	cmd = NewCommand("HGET", []string{"hash1", "field3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "HGET不存在的字段应该返回nil回复")

	// 测试获取不存在的键
	cmd = NewCommand("HGET", []string{"hash2", "field1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "HGET不存在的键应该返回nil回复")

	// 测试参数不足
	cmd = NewCommand("HGET", []string{"hash1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HGET
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HGET", []string{"string1", "field"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HGET应该返回错误")
}

// TestExecHDel 测试HDEL命令
func TestExecHDel(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	hashVal.Set("field3", "value3")
	db.Set("hash1", hashVal, 0)

	// 测试删除单个字段
	cmd := NewCommand("HDEL", []string{"hash1", "field1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HDEL应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功删除1个字段应该返回1")

	// 验证字段已删除
	val, _ := db.Get("hash1")
	_, exists := val.(*ds2.Hash).Get("field1")
	assert.False(t, exists, "field1应该已被删除")

	// 测试删除多个字段，包括不存在的字段
	cmd = NewCommand("HDEL", []string{"hash1", "field2", "field3", "field4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HDEL应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "成功删除2个字段应该返回2")

	// 验证哈希表为空后是否被删除
	exists = db.Exists("hash1")
	assert.True(t, exists, "所有字段删除后哈希表不应该被删除")

	// 测试删除不存在的键
	cmd = NewCommand("HDEL", []string{"hash2", "field1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HDEL不存在的键应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "删除不存在的键应该返回0")

	// 测试参数不足
	cmd = NewCommand("HDEL", []string{"hash1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HDEL
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HDEL", []string{"string1", "field"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HDEL应该返回错误")
}

// TestExecHExists 测试HEXISTS命令
func TestExecHExists(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	db.Set("hash1", hashVal, 0)

	// 测试字段存在
	cmd := NewCommand("HEXISTS", []string{"hash1", "field1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HEXISTS应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "字段存在应该返回1")

	// 测试字段不存在
	cmd = NewCommand("HEXISTS", []string{"hash1", "field3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HEXISTS应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "字段不存在应该返回0")

	// 测试键不存在
	cmd = NewCommand("HEXISTS", []string{"hash2", "field1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HEXISTS应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "键不存在应该返回0")

	// 测试参数不足
	cmd = NewCommand("HEXISTS", []string{"hash1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HEXISTS
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HEXISTS", []string{"string1", "field"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HEXISTS应该返回错误")
}

// TestExecHLen 测试HLEN命令
func TestExecHLen(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	hashVal.Set("field3", "value3")
	db.Set("hash1", hashVal, 0)

	// 测试HLEN
	cmd := NewCommand("HLEN", []string{"hash1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HLEN应该返回整数回复")
	assert.Equal(t, "3", reply.String(), "hash1应该有3个字段")

	// 测试不存在的键
	cmd = NewCommand("HLEN", []string{"hash2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HLEN应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "不存在的键应该返回0")

	// 测试参数不足
	cmd = NewCommand("HLEN", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HLEN
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HLEN", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HLEN应该返回错误")
}

// TestExecHGetAll 测试HGETALL命令
func TestExecHGetAll(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	db.Set("hash1", hashVal, 0)

	// 测试HGETALL
	cmd := NewCommand("HGETALL", []string{"hash1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "HGETALL应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("HGETALL", []string{"hash2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "HGETALL应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("HGETALL", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HGETALL
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HGETALL", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HGETALL应该返回错误")
}

// TestExecHKeys 测试HKEYS命令
func TestExecHKeys(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	db.Set("hash1", hashVal, 0)

	// 测试HKEYS
	cmd := NewCommand("HKEYS", []string{"hash1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "HKEYS应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("HKEYS", []string{"hash2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "HKEYS应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("HKEYS", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HKEYS
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HKEYS", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HKEYS应该返回错误")
}

// TestExecHVals 测试HVALS命令
func TestExecHVals(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	hashVal.Set("field2", "value2")
	db.Set("hash1", hashVal, 0)

	// 测试HVALS
	cmd := NewCommand("HVALS", []string{"hash1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "HVALS应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("HVALS", []string{"hash2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "HVALS应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("HVALS", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HVALS
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HVALS", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HVALS应该返回错误")
}

// TestExecHIncrBy 测试HINCRBY命令
func TestExecHIncrBy(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	hashVal := ds2.NewHash()
	hashVal.Set("field1", "10")
	db.Set("hash1", hashVal, 0)

	// 测试HINCRBY
	cmd := NewCommand("HINCRBY", []string{"hash1", "field1", "5"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HINCRBY应该返回整数回复")
	assert.Equal(t, "15", reply.String(), "增加5后应该是15")

	// 验证值
	val, _ := db.Get("hash1")
	fieldValue, _ := val.(*ds2.Hash).Get("field1")
	assert.Equal(t, "15", fieldValue, "field1的值应该是15")

	// 测试对不存在的字段HINCRBY
	cmd = NewCommand("HINCRBY", []string{"hash1", "field2", "3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HINCRBY应该返回整数回复")
	assert.Equal(t, "3", reply.String(), "对不存在的字段增加3应该返回3")

	// 测试对不存在的键HINCRBY
	cmd = NewCommand("HINCRBY", []string{"hash2", "field1", "8"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "HINCRBY应该返回整数回复")
	assert.Equal(t, "8", reply.String(), "对不存在的键增加8应该返回8")

	// 验证哈希表已创建
	val, exists := db.Get("hash2")
	assert.True(t, exists, "hash2应该被创建")
	assert.Equal(t, cache2.TypeHash, val.Type(), "hash2应该是哈希表类型")

	// 测试非数字字段
	hashVal = ds2.NewHash()
	hashVal.Set("field1", "abc")
	db.Set("hash3", hashVal, 0)
	cmd = NewCommand("HINCRBY", []string{"hash3", "field1", "5"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非数字字段执行HINCRBY应该返回错误")

	// 测试无效增量
	cmd = NewCommand("HINCRBY", []string{"hash1", "field1", "invalid"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "无效增量应该返回错误")

	// 测试参数不足
	cmd = NewCommand("HINCRBY", []string{"hash1", "field1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非哈希表类型执行HINCRBY
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("HINCRBY", []string{"string1", "field", "5"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非哈希表类型执行HINCRBY应该返回错误")
}
