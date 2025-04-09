package engine

import (
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 辅助函数：将Reply转换为整数
func replyToInt(reply cache2.Reply) (int64, error) {
	return strconv.ParseInt(reply.String(), 10, 64)
}

// 辅助函数：将Reply转换为字符串数组
func replyToStringArray(reply cache2.Reply) ([]string, error) {
	if reply.Type() != cache2.ReplyMultiBulk {
		return nil, nil
	}

	// 这里简单实现，假设回复内容是以空格分隔的字符串
	result := make([]string, 0)
	for _, s := range reply.String()[1 : len(reply.String())-1] {
		if s != ' ' {
			result = append(result, string(s))
		}
	}
	return result, nil
}

// TestExecPing 测试PING命令
func TestExecPing(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试普通PING
	cmd := NewCommand("PING", []string{}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "PING应该返回状态回复")
	assert.Equal(t, "PONG", reply.String(), "PING应该返回PONG")

	// 测试带参数的PING
	cmd = NewCommand("PING", []string{"hello"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "带参数的PING应该返回批量回复")
	assert.Equal(t, "hello", reply.String(), "带参数的PING应该返回参数值")
}

// TestExecExists 测试EXISTS命令
func TestExecExists(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置一些测试键
	db.Set("key1", ds2.NewString("value1"), 0)
	db.Set("key2", ds2.NewString("value2"), 0)

	// 测试单键EXISTS
	cmd := NewCommand("EXISTS", []string{"key1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "EXISTS应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "key1存在，应该返回1")

	// 测试多键EXISTS
	cmd = NewCommand("EXISTS", []string{"key1", "key2", "key3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "EXISTS应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "应该返回存在的键的数量")

	// 测试不存在的键
	cmd = NewCommand("EXISTS", []string{"key3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "EXISTS应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "key3不存在，应该返回0")

	// 测试参数不足
	cmd = NewCommand("EXISTS", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}

// TestExecExpire 测试EXPIRE命令
func TestExecExpire(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置测试键
	db.Set("key1", ds2.NewString("value1"), 0)

	// 测试设置过期时间
	cmd := NewCommand("EXPIRE", []string{"key1", "1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "EXPIRE应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "设置成功应该返回1")

	// 测试TTL命令
	cmd = NewCommand("TTL", []string{"key1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "TTL应该返回整数回复")
	ttl, err := replyToInt(reply)
	assert.NoError(t, err, "TTL结果应该可以转换为整数")
	assert.True(t, ttl >= 0 && ttl <= 1, "TTL应该在有效范围内")

	// 测试对不存在的键设置过期时间
	cmd = NewCommand("EXPIRE", []string{"key3", "10"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "EXPIRE应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "键不存在时应该返回0")

	// 测试参数不足
	cmd = NewCommand("EXPIRE", []string{"key1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试无效的过期时间
	cmd = NewCommand("EXPIRE", []string{"key1", "invalid"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "无效的过期时间应该返回错误")
}

// TestExecTTL 测试TTL命令
func TestExecTTL(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置测试键
	db.Set("key1", ds2.NewString("value1"), 0)
	db.Set("key2", ds2.NewString("value2"), 1*time.Second)

	// 测试永久键TTL
	cmd := NewCommand("TTL", []string{"key1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "TTL应该返回整数回复")
	assert.Equal(t, "-1", reply.String(), "永久键应该返回-1")

	// 测试带过期时间的键TTL
	cmd = NewCommand("TTL", []string{"key2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "TTL应该返回整数回复")
	ttl, err := replyToInt(reply)
	assert.NoError(t, err, "TTL结果应该可以转换为整数")
	assert.True(t, ttl >= 0 && ttl <= 1, "TTL应该在有效范围内")

	// 测试不存在的键TTL
	cmd = NewCommand("TTL", []string{"key3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "TTL应该返回整数回复")
	assert.Equal(t, "-2", reply.String(), "不存在的键应该返回-2")

	// 测试参数不足
	cmd = NewCommand("TTL", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}

// TestExecType 测试TYPE命令
func TestExecType(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置不同类型的值
	db.Set("string-key", ds2.NewString("string-value"), 0)

	listVal := ds2.NewList()
	listVal.RPush("item1", "item2")
	db.Set("list-key", listVal, 0)

	hashVal := ds2.NewHash()
	hashVal.Set("field1", "value1")
	db.Set("hash-key", hashVal, 0)

	setVal := ds2.NewSet()
	setVal.Add("member1", "member2")
	db.Set("set-key", setVal, 0)

	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	db.Set("zset-key", zsetVal, 0)

	// 测试字符串类型
	cmd := NewCommand("TYPE", []string{"string-key"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "TYPE应该返回状态回复")
	assert.Equal(t, "string", reply.String(), "应该返回string")

	// 测试列表类型
	cmd = NewCommand("TYPE", []string{"list-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "TYPE应该返回状态回复")
	assert.Equal(t, "list", reply.String(), "应该返回list")

	// 测试哈希类型
	cmd = NewCommand("TYPE", []string{"hash-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "TYPE应该返回状态回复")
	assert.Equal(t, "hash", reply.String(), "应该返回hash")

	// 测试集合类型
	cmd = NewCommand("TYPE", []string{"set-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "TYPE应该返回状态回复")
	assert.Equal(t, "set", reply.String(), "应该返回set")

	// 测试有序集合类型
	cmd = NewCommand("TYPE", []string{"zset-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "TYPE应该返回状态回复")
	assert.Equal(t, "zset", reply.String(), "应该返回zset")

	// 测试不存在的键
	cmd = NewCommand("TYPE", []string{"non-existent-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "TYPE应该返回状态回复")
	assert.Equal(t, "none", reply.String(), "不存在的键应该返回none")

	// 测试参数不足
	cmd = NewCommand("TYPE", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}

// TestExecKeys 测试KEYS命令
func TestExecKeys(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置一些测试键
	keys := []string{"key1", "key2", "test1", "test2", "another"}
	for _, key := range keys {
		db.Set(key, ds2.NewString(key+"-value"), 0)
	}

	// 测试精确匹配
	cmd := NewCommand("KEYS", []string{"key1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "KEYS应该返回多批量回复")

	// 因为Reply接口没有直接获取数组的方法，我们这里只检查回复类型
	// 具体回复内容的验证需要依赖具体实现，这里简化处理

	// 测试前缀匹配
	cmd = NewCommand("KEYS", []string{"key.*"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "KEYS应该返回多批量回复")

	// 测试后缀匹配
	cmd = NewCommand("KEYS", []string{".*2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "KEYS应该返回多批量回复")

	// 测试通配符匹配所有
	cmd = NewCommand("KEYS", []string{".*"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "KEYS应该返回多批量回复")

	// 测试无效正则表达式
	cmd = NewCommand("KEYS", []string{"["}, "client1") // 无效的正则表达式
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "无效的正则表达式应该返回空的多批量回复")

	// 测试参数不足
	cmd = NewCommand("KEYS", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}

// TestExecGet 测试GET命令
func TestExecGet(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置测试键
	db.Set("key1", ds2.NewString("value1"), 0)

	// 测试获取存在的键
	cmd := NewCommand("GET", []string{"key1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "GET应该返回批量回复")
	assert.Equal(t, "value1", reply.String(), "GET应该返回键的值")

	// 测试获取不存在的键
	cmd = NewCommand("GET", []string{"non-existent-key"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "GET不存在的键应该返回nil回复")

	// 测试参数不足
	cmd = NewCommand("GET", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试参数过多
	cmd = NewCommand("GET", []string{"key1", "key2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数过多应该返回错误")
}

// TestExecSet 测试SET命令
func TestExecSet(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试基本SET
	cmd := NewCommand("SET", []string{"key1", "value1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "SET应该返回状态回复")
	assert.Equal(t, "OK", reply.String(), "SET应该返回OK")

	// 验证值已设置
	val, exists := db.Get("key1")
	assert.True(t, exists, "key1应该存在")
	assert.Equal(t, "value1", val.(*ds2.String).String(), "key1的值应该是value1")

	// 测试带过期时间的SET
	cmd = NewCommand("SET", []string{"key2", "value2", "EX", "1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyStatus, reply.Type(), "SET应该返回状态回复")
	assert.Equal(t, "OK", reply.String(), "SET应该返回OK")

	// 验证带过期时间的值
	val, exists = db.Get("key2")
	assert.True(t, exists, "key2应该存在")
	assert.Equal(t, "value2", val.(*ds2.String).String(), "key2的值应该是value2")

	// 测试TTL
	ttl := db.TTL("key2")
	assert.True(t, ttl > 0 && ttl <= time.Second, "TTL应该在有效范围内")

	// 测试参数不足
	cmd = NewCommand("SET", []string{"key3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试无效的过期时间
	cmd = NewCommand("SET", []string{"key3", "value3", "EX", "invalid"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "无效的过期时间应该返回错误")
}

// TestExecAppend 测试APPEND命令
func TestExecAppend(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置初始键
	db.Set("key1", ds2.NewString("Hello"), 0)

	// 测试在已存在键上APPEND
	cmd := NewCommand("APPEND", []string{"key1", " World"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "APPEND应该返回整数回复")
	length, err := replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(11), length, "返回的应该是追加后的长度")

	// 验证值
	val, exists := db.Get("key1")
	assert.True(t, exists, "key1应该存在")
	assert.Equal(t, "Hello World", val.(*ds2.String).String(), "值应该是Hello World")

	// 测试在不存在的键上APPEND（应该创建键）
	cmd = NewCommand("APPEND", []string{"key2", "value2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "APPEND应该返回整数回复")
	length, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(6), length, "返回的应该是追加后的长度")

	// 验证值
	val, exists = db.Get("key2")
	assert.True(t, exists, "key2应该存在")
	assert.Equal(t, "value2", val.(*ds2.String).String(), "值应该是value2")

	// 测试参数不足
	cmd = NewCommand("APPEND", []string{"key1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}

// TestExecIncr 测试INCR命令
func TestExecIncr(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置初始键
	db.Set("counter", ds2.NewString("10"), 0)

	// 测试INCR
	cmd := NewCommand("INCR", []string{"counter"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "INCR应该返回整数回复")
	val, err := replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(11), val, "INCR后的值应该是11")

	// 验证值
	strVal, exists := db.Get("counter")
	assert.True(t, exists, "counter应该存在")
	assert.Equal(t, "11", strVal.(*ds2.String).String(), "counter的值应该是11")

	// 测试不存在的键
	cmd = NewCommand("INCR", []string{"new-counter"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "INCR应该返回整数回复")
	val, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(1), val, "新键INCR后的值应该是1")

	// 测试非数字值
	db.Set("non-number", ds2.NewString("abc"), 0)
	cmd = NewCommand("INCR", []string{"non-number"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非数字值执行INCR应该返回错误")

	// 测试参数不足
	cmd = NewCommand("INCR", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")
}

// TestExecIncrBy 测试INCRBY命令
func TestExecIncrBy(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 设置初始键
	db.Set("counter", ds2.NewString("10"), 0)

	// 测试INCRBY
	cmd := NewCommand("INCRBY", []string{"counter", "5"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "INCRBY应该返回整数回复")
	val, err := replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(15), val, "INCRBY后的值应该是15")

	// 验证值
	strVal, exists := db.Get("counter")
	assert.True(t, exists, "counter应该存在")
	assert.Equal(t, "15", strVal.(*ds2.String).String(), "counter的值应该是15")

	// 测试不存在的键
	cmd = NewCommand("INCRBY", []string{"new-counter", "7"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "INCRBY应该返回整数回复")
	val, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(7), val, "新键INCRBY后的值应该是7")

	// 测试非数字值
	db.Set("non-number", ds2.NewString("abc"), 0)
	cmd = NewCommand("INCRBY", []string{"non-number", "5"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非数字值执行INCRBY应该返回错误")

	// 测试参数不足
	cmd = NewCommand("INCRBY", []string{"counter"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试无效增量
	cmd = NewCommand("INCRBY", []string{"counter", "invalid"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "无效增量应该返回错误")
}

// TestExecLPush 测试LPUSH命令
func TestExecLPush(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试在不存在的键上LPUSH
	cmd := NewCommand("LPUSH", []string{"list1", "value1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "LPUSH应该返回整数回复")
	length, err := replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(1), length, "LPUSH后长度应该是1")

	// 验证值
	val, exists := db.Get("list1")
	assert.True(t, exists, "list1应该存在")
	assert.Equal(t, cache2.TypeList, val.Type(), "list1应该是列表类型")
	assert.Equal(t, int64(1), val.(*ds2.List).Len(), "list1长度应该是1")

	// 测试在已存在列表上LPUSH
	cmd = NewCommand("LPUSH", []string{"list1", "value2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "LPUSH应该返回整数回复")
	length, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(2), length, "LPUSH后长度应该是2")

	// 测试LPUSH多个值
	cmd = NewCommand("LPUSH", []string{"list1", "value3", "value4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "LPUSH应该返回整数回复")
	length, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(4), length, "LPUSH多个值后长度应该是4")

	// 测试参数不足
	cmd = NewCommand("LPUSH", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非列表类型执行LPUSH
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("LPUSH", []string{"string1", "value"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非列表类型执行LPUSH应该返回错误")
}

// TestExecRPush 测试RPUSH命令
func TestExecRPush(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试在不存在的键上RPUSH
	cmd := NewCommand("RPUSH", []string{"list1", "value1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "RPUSH应该返回整数回复")
	length, err := replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(1), length, "RPUSH后长度应该是1")

	// 验证值
	val, exists := db.Get("list1")
	assert.True(t, exists, "list1应该存在")
	assert.Equal(t, cache2.TypeList, val.Type(), "list1应该是列表类型")
	assert.Equal(t, int64(1), val.(*ds2.List).Len(), "list1长度应该是1")

	// 测试在已存在列表上RPUSH
	cmd = NewCommand("RPUSH", []string{"list1", "value2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "RPUSH应该返回整数回复")
	length, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(2), length, "RPUSH后长度应该是2")

	// 测试RPUSH多个值
	cmd = NewCommand("RPUSH", []string{"list1", "value3", "value4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "RPUSH应该返回整数回复")
	length, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(4), length, "RPUSH多个值后长度应该是4")

	// 测试参数不足
	cmd = NewCommand("RPUSH", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非列表类型执行RPUSH
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("RPUSH", []string{"string1", "value"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非列表类型执行RPUSH应该返回错误")
}

// TestExecLPop 测试LPOP命令
func TestExecLPop(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	listVal := ds2.NewList()
	listVal.RPush("value1", "value2", "value3")
	db.Set("list1", listVal, 0)

	// 测试LPOP
	cmd := NewCommand("LPOP", []string{"list1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "LPOP应该返回批量回复")
	assert.Equal(t, "value1", reply.String(), "LPOP应该返回列表的头部元素")

	// 验证列表长度
	val, exists := db.Get("list1")
	assert.True(t, exists, "list1应该存在")
	assert.Equal(t, int64(2), val.(*ds2.List).Len(), "list1长度应该是2")

	// 测试再次LPOP
	cmd = NewCommand("LPOP", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "LPOP应该返回批量回复")
	assert.Equal(t, "value2", reply.String(), "LPOP应该返回列表的头部元素")

	// 测试最后一个元素LPOP后键应该被删除
	cmd = NewCommand("LPOP", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "LPOP应该返回批量回复")
	assert.Equal(t, "value3", reply.String(), "LPOP应该返回列表的头部元素")

	// 验证键是否被删除
	exists = db.Exists("list1")
	assert.False(t, exists, "列表为空后键应该被删除")

	// 测试对空列表LPOP
	cmd = NewCommand("LPOP", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "对空列表LPOP应该返回nil回复")

	// 测试参数不足
	cmd = NewCommand("LPOP", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非列表类型执行LPOP
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("LPOP", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非列表类型执行LPOP应该返回错误")
}

// TestExecRPop 测试RPOP命令
func TestExecRPop(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	listVal := ds2.NewList()
	listVal.RPush("value1", "value2", "value3")
	db.Set("list1", listVal, 0)

	// 测试RPOP
	cmd := NewCommand("RPOP", []string{"list1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "RPOP应该返回批量回复")
	assert.Equal(t, "value3", reply.String(), "RPOP应该返回列表的尾部元素")

	// 验证列表长度
	val, exists := db.Get("list1")
	assert.True(t, exists, "list1应该存在")
	assert.Equal(t, int64(2), val.(*ds2.List).Len(), "list1长度应该是2")

	// 测试再次RPOP
	cmd = NewCommand("RPOP", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "RPOP应该返回批量回复")
	assert.Equal(t, "value2", reply.String(), "RPOP应该返回列表的尾部元素")

	// 测试最后一个元素RPOP后键应该被删除
	cmd = NewCommand("RPOP", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "RPOP应该返回批量回复")
	assert.Equal(t, "value1", reply.String(), "RPOP应该返回列表的尾部元素")

	// 验证键是否被删除
	exists = db.Exists("list1")
	assert.False(t, exists, "列表为空后键应该被删除")

	// 测试对空列表RPOP
	cmd = NewCommand("RPOP", []string{"list1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "对空列表RPOP应该返回nil回复")

	// 测试参数不足
	cmd = NewCommand("RPOP", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非列表类型执行RPOP
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("RPOP", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非列表类型执行RPOP应该返回错误")
}

// TestExecLLen 测试LLEN命令
func TestExecLLen(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	listVal := ds2.NewList()
	listVal.RPush("value1", "value2", "value3")
	db.Set("list1", listVal, 0)

	// 测试LLEN
	cmd := NewCommand("LLEN", []string{"list1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "LLEN应该返回整数回复")
	length, err := replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(3), length, "list1长度应该是3")

	// 测试对不存在的键LLEN
	cmd = NewCommand("LLEN", []string{"non-existent-list"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "对不存在的键LLEN应该返回整数回复")
	length, err = replyToInt(reply)
	assert.NoError(t, err, "应该能够解析整数回复")
	assert.Equal(t, int64(0), length, "不存在的键长度应该是0")

	// 测试参数不足
	cmd = NewCommand("LLEN", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非列表类型执行LLEN
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("LLEN", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非列表类型执行LLEN应该返回错误")
}

// TestExecLRange 测试LRANGE命令
func TestExecLRange(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	listVal := ds2.NewList()
	listVal.RPush("value1", "value2", "value3", "value4", "value5")
	db.Set("list1", listVal, 0)

	// 测试LRANGE正常范围
	cmd := NewCommand("LRANGE", []string{"list1", "0", "2"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "LRANGE应该返回多批量回复")

	// 测试LRANGE负索引
	cmd = NewCommand("LRANGE", []string{"list1", "-2", "-1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "LRANGE应该返回多批量回复")

	// 测试LRANGE起始索引大于结束索引
	cmd = NewCommand("LRANGE", []string{"list1", "3", "1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "LRANGE应该返回多批量回复")

	// 测试LRANGE超出范围
	cmd = NewCommand("LRANGE", []string{"list1", "10", "20"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "LRANGE应该返回多批量回复")

	// 测试对不存在的键LRANGE
	cmd = NewCommand("LRANGE", []string{"non-existent-list", "0", "1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "对不存在的键LRANGE应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("LRANGE", []string{"list1", "0"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试无效的索引
	cmd = NewCommand("LRANGE", []string{"list1", "invalid", "1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "无效的索引应该返回错误")

	// 测试对非列表类型执行LRANGE
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("LRANGE", []string{"string1", "0", "1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非列表类型执行LRANGE应该返回错误")
}
