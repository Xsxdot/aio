package engine

import (
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExecZAdd 测试ZADD命令
func TestExecZAdd(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试在不存在的键上执行ZADD
	cmd := NewCommand("ZADD", []string{"zset1", "1.0", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZADD应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "添加1个成员应该返回1")

	// 验证值
	val, exists := db.Get("zset1")
	assert.True(t, exists, "zset1应该存在")
	assert.Equal(t, cache2.TypeZSet, val.Type(), "zset1应该是有序集合类型")
	assert.Equal(t, int64(1), val.(*ds2.ZSet).Len(), "zset1应该有1个成员")

	// 测试添加多个成员
	cmd = NewCommand("ZADD", []string{"zset1", "2.0", "member2", "3.0", "member3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZADD应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "成功添加2个成员应该返回2")

	// 测试更新已存在成员的分数
	cmd = NewCommand("ZADD", []string{"zset1", "4.0", "member1", "5.0", "member4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZADD应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "只有一个新成员被添加应该返回1")

	// 验证分数更新
	val, _ = db.Get("zset1")
	score, _ := val.(*ds2.ZSet).Score("member1")
	assert.Equal(t, 4.0, score, "member1的分数应该被更新为4.0")

	// 测试参数不足或格式错误
	cmd = NewCommand("ZADD", []string{"zset1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	cmd = NewCommand("ZADD", []string{"zset1", "score", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "分数格式错误应该返回错误")

	// 测试对非有序集合类型执行ZADD
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZADD", []string{"string1", "1.0", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZADD应该返回错误")
}

// TestExecZScore 测试ZSCORE命令
func TestExecZScore(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	db.Set("zset1", zsetVal, 0)

	// 测试获取存在成员的分数
	cmd := NewCommand("ZSCORE", []string{"zset1", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "ZSCORE应该返回批量回复")
	score, err := strconv.ParseFloat(reply.String(), 64)
	assert.Nil(t, err, "应该能成功解析分数")
	assert.Equal(t, 1.0, score, "member1的分数应该是1.0")

	// 测试获取不存在成员的分数
	cmd = NewCommand("ZSCORE", []string{"zset1", "member4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "不存在的成员应该返回nil回复")

	// 测试获取不存在键的成员分数
	cmd = NewCommand("ZSCORE", []string{"zset2", "member1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "不存在的键应该返回nil回复")

	// 测试参数不足
	cmd = NewCommand("ZSCORE", []string{"zset1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非有序集合类型执行ZSCORE
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZSCORE", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZSCORE应该返回错误")
}

// TestExecZIncrBy 测试ZINCRBY命令
func TestExecZIncrBy(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	db.Set("zset1", zsetVal, 0)

	// 测试增加已存在成员的分数
	cmd := NewCommand("ZINCRBY", []string{"zset1", "2.5", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "ZINCRBY应该返回批量回复")
	newScore, err := strconv.ParseFloat(reply.String(), 64)
	assert.Nil(t, err, "应该能成功解析新分数")
	assert.Equal(t, 3.5, newScore, "member1的新分数应该是3.5")

	// 测试增加不存在成员的分数（自动添加）
	cmd = NewCommand("ZINCRBY", []string{"zset1", "1.5", "member3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "ZINCRBY应该返回批量回复")
	newScore, err = strconv.ParseFloat(reply.String(), 64)
	assert.Nil(t, err, "应该能成功解析新分数")
	assert.Equal(t, 1.5, newScore, "member3的新分数应该是1.5")

	// 验证集合大小增加
	val, _ := db.Get("zset1")
	assert.Equal(t, int64(3), val.(*ds2.ZSet).Len(), "zset1应该有3个成员")

	// 测试负增量
	cmd = NewCommand("ZINCRBY", []string{"zset1", "-1.0", "member2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "ZINCRBY应该返回批量回复")
	newScore, err = strconv.ParseFloat(reply.String(), 64)
	assert.Nil(t, err, "应该能成功解析新分数")
	assert.Equal(t, 1.0, newScore, "member2的新分数应该是1.0")

	// 测试增加不存在键的成员分数（自动创建）
	cmd = NewCommand("ZINCRBY", []string{"zset2", "1.0", "member1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "ZINCRBY应该返回批量回复")

	// 验证新键已创建
	exists := db.Exists("zset2")
	assert.True(t, exists, "zset2应该被创建")

	// 测试参数不足
	cmd = NewCommand("ZINCRBY", []string{"zset1", "1.0"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试增量格式错误
	cmd = NewCommand("ZINCRBY", []string{"zset1", "invalid", "member1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "增量格式错误应该返回错误")

	// 测试对非有序集合类型执行ZINCRBY
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZINCRBY", []string{"string1", "1.0", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZINCRBY应该返回错误")
}

// TestExecZCard 测试ZCARD命令
func TestExecZCard(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	db.Set("zset1", zsetVal, 0)

	// 测试ZCARD
	cmd := NewCommand("ZCARD", []string{"zset1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZCARD应该返回整数回复")
	assert.Equal(t, "3", reply.String(), "zset1应该有3个成员")

	// 测试不存在的键
	cmd = NewCommand("ZCARD", []string{"zset2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZCARD应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "不存在的键应该返回0")

	// 测试参数不足
	cmd = NewCommand("ZCARD", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非有序集合类型执行ZCARD
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZCARD", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZCARD应该返回错误")
}

// TestExecZRange 测试ZRANGE命令
func TestExecZRange(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	zsetVal.Add(4.0, "member4")
	zsetVal.Add(5.0, "member5")
	db.Set("zset1", zsetVal, 0)

	// 测试ZRANGE获取全部范围
	cmd := NewCommand("ZRANGE", []string{"zset1", "0", "-1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGE应该返回多批量回复")

	// 测试ZRANGE获取部分范围
	cmd = NewCommand("ZRANGE", []string{"zset1", "1", "3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGE应该返回多批量回复")

	// 测试ZRANGE带WITHSCORES选项
	cmd = NewCommand("ZRANGE", []string{"zset1", "0", "1", "WITHSCORES"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGE应该返回多批量回复")

	// 测试索引超出范围
	cmd = NewCommand("ZRANGE", []string{"zset1", "10", "20"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGE应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("ZRANGE", []string{"zset2", "0", "-1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGE应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("ZRANGE", []string{"zset1", "0"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试索引格式错误
	cmd = NewCommand("ZRANGE", []string{"zset1", "invalid", "10"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "索引格式错误应该返回错误")

	// 测试对非有序集合类型执行ZRANGE
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZRANGE", []string{"string1", "0", "-1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZRANGE应该返回错误")
}

// TestExecZRevRange 测试ZREVRANGE命令
func TestExecZRevRange(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	zsetVal.Add(4.0, "member4")
	zsetVal.Add(5.0, "member5")
	db.Set("zset1", zsetVal, 0)

	// 测试ZREVRANGE获取全部范围
	cmd := NewCommand("ZREVRANGE", []string{"zset1", "0", "-1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZREVRANGE应该返回多批量回复")

	// 测试ZREVRANGE获取部分范围
	cmd = NewCommand("ZREVRANGE", []string{"zset1", "1", "3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZREVRANGE应该返回多批量回复")

	// 测试ZREVRANGE带WITHSCORES选项
	cmd = NewCommand("ZREVRANGE", []string{"zset1", "0", "1", "WITHSCORES"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZREVRANGE应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("ZREVRANGE", []string{"zset2", "0", "-1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZREVRANGE应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("ZREVRANGE", []string{"zset1", "0"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非有序集合类型执行ZREVRANGE
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZREVRANGE", []string{"string1", "0", "-1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZREVRANGE应该返回错误")
}

// TestExecZRangeByScore 测试ZRANGEBYSCORE命令
func TestExecZRangeByScore(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	zsetVal.Add(4.0, "member4")
	zsetVal.Add(5.0, "member5")
	db.Set("zset1", zsetVal, 0)

	// 测试ZRANGEBYSCORE获取全部范围
	cmd := NewCommand("ZRANGEBYSCORE", []string{"zset1", "-inf", "+inf"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGEBYSCORE应该返回多批量回复")

	// 测试ZRANGEBYSCORE获取部分范围
	cmd = NewCommand("ZRANGEBYSCORE", []string{"zset1", "2", "4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGEBYSCORE应该返回多批量回复")

	// 测试ZRANGEBYSCORE带WITHSCORES选项
	cmd = NewCommand("ZRANGEBYSCORE", []string{"zset1", "1", "3", "WITHSCORES"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGEBYSCORE应该返回多批量回复")

	// 测试ZRANGEBYSCORE带LIMIT选项
	cmd = NewCommand("ZRANGEBYSCORE", []string{"zset1", "1", "5", "LIMIT", "1", "2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGEBYSCORE应该返回多批量回复")

	// 测试ZRANGEBYSCORE带WITHSCORES和LIMIT选项
	cmd = NewCommand("ZRANGEBYSCORE", []string{"zset1", "1", "5", "WITHSCORES", "LIMIT", "1", "2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGEBYSCORE应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("ZRANGEBYSCORE", []string{"zset2", "-inf", "+inf"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "ZRANGEBYSCORE应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("ZRANGEBYSCORE", []string{"zset1", "1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非有序集合类型执行ZRANGEBYSCORE
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZRANGEBYSCORE", []string{"string1", "-inf", "+inf"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZRANGEBYSCORE应该返回错误")
}

// TestExecZRank 测试ZRANK和ZREVRANK命令
func TestExecZRank(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	zsetVal.Add(4.0, "member4")
	zsetVal.Add(5.0, "member5")
	db.Set("zset1", zsetVal, 0)

	// 测试ZRANK查询排名
	cmd := NewCommand("ZRANK", []string{"zset1", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZRANK应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "member1的排名应该是0")

	cmd = NewCommand("ZRANK", []string{"zset1", "member3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZRANK应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "member3的排名应该是2")

	// 测试ZRANK查询不存在的成员
	cmd = NewCommand("ZRANK", []string{"zset1", "nonexistent"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "不存在的成员应该返回nil回复")

	// 测试ZREVRANK查询排名
	cmd = NewCommand("ZREVRANK", []string{"zset1", "member5"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZREVRANK应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "member5的逆序排名应该是0")

	cmd = NewCommand("ZREVRANK", []string{"zset1", "member3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZREVRANK应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "member3的逆序排名应该是2")

	// 测试参数不足
	cmd = NewCommand("ZRANK", []string{"zset1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	cmd = NewCommand("ZREVRANK", []string{"zset1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非有序集合类型执行ZRANK和ZREVRANK
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZRANK", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZRANK应该返回错误")

	cmd = NewCommand("ZREVRANK", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZREVRANK应该返回错误")
}

// TestExecZRem 测试ZREM命令
func TestExecZRem(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	zsetVal := ds2.NewZSet()
	zsetVal.Add(1.0, "member1")
	zsetVal.Add(2.0, "member2")
	zsetVal.Add(3.0, "member3")
	zsetVal.Add(4.0, "member4")
	db.Set("zset1", zsetVal, 0)

	// 测试删除单个成员
	cmd := NewCommand("ZREM", []string{"zset1", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZREM应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功删除1个成员应该返回1")

	// 验证成员已删除
	val, _ := db.Get("zset1")
	_, exists := val.(*ds2.ZSet).Score("member1")
	assert.False(t, exists, "member1应该已被删除")

	// 测试删除多个成员，包括不存在的成员
	cmd = NewCommand("ZREM", []string{"zset1", "member2", "member3", "member5"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZREM应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "成功删除2个成员应该返回2")

	// 验证集合大小
	val, _ = db.Get("zset1")
	assert.Equal(t, int64(1), val.(*ds2.ZSet).Len(), "zset1应该只剩1个成员")

	// 测试删除所有剩余成员
	cmd = NewCommand("ZREM", []string{"zset1", "member4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZREM应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功删除1个成员应该返回1")

	// 验证集合为空后是否被删除
	exists = db.Exists("zset1")
	assert.False(t, exists, "所有成员删除后集合应该被删除")

	// 测试删除不存在的键的成员
	cmd = NewCommand("ZREM", []string{"zset2", "member1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "ZREM不存在的键应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "删除不存在的键的成员应该返回0")

	// 测试参数不足
	cmd = NewCommand("ZREM", []string{"zset1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非有序集合类型执行ZREM
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("ZREM", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非有序集合类型执行ZREM应该返回错误")
}
