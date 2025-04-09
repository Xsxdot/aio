package engine

import (
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExecSAdd 测试SADD命令
func TestExecSAdd(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 测试在不存在的键上SADD
	cmd := NewCommand("SADD", []string{"set1", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SADD应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "添加1个成员应该返回1")

	// 验证值
	val, exists := db.Get("set1")
	assert.True(t, exists, "set1应该存在")
	assert.Equal(t, cache2.TypeSet, val.Type(), "set1应该是集合类型")
	assert.Equal(t, int64(1), val.(*ds2.Set).Len(), "set1应该有1个成员")

	// 测试添加多个成员
	cmd = NewCommand("SADD", []string{"set1", "member2", "member3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SADD应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "成功添加2个成员应该返回2")

	// 测试添加已存在的成员
	cmd = NewCommand("SADD", []string{"set1", "member1", "member4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SADD应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "只有一个新成员被添加应该返回1")

	// 验证集合大小
	val, _ = db.Get("set1")
	assert.Equal(t, int64(4), val.(*ds2.Set).Len(), "set1应该有4个成员")

	// 测试参数不足
	cmd = NewCommand("SADD", []string{"set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SADD
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SADD", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SADD应该返回错误")
}

// TestExecSRem 测试SREM命令
func TestExecSRem(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	setVal := ds2.NewSet()
	setVal.Add("member1", "member2", "member3", "member4")
	db.Set("set1", setVal, 0)

	// 测试删除单个成员
	cmd := NewCommand("SREM", []string{"set1", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SREM应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功删除1个成员应该返回1")

	// 验证成员已删除
	val, _ := db.Get("set1")
	assert.False(t, val.(*ds2.Set).IsMember("member1"), "member1应该已被删除")

	// 测试删除多个成员，包括不存在的成员
	cmd = NewCommand("SREM", []string{"set1", "member2", "member3", "member5"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SREM应该返回整数回复")
	assert.Equal(t, "2", reply.String(), "成功删除2个成员应该返回2")

	// 验证集合大小
	val, _ = db.Get("set1")
	assert.Equal(t, int64(1), val.(*ds2.Set).Len(), "set1应该只剩1个成员")

	// 测试删除所有剩余成员
	cmd = NewCommand("SREM", []string{"set1", "member4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SREM应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成功删除1个成员应该返回1")

	// 验证集合为空后是否被删除，不应该被删除
	exists := db.Exists("set1")
	assert.True(t, exists, "所有成员删除后集合不应该被删除")

	// 测试删除不存在的键
	cmd = NewCommand("SREM", []string{"set2", "member1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SREM不存在的键应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "删除不存在的键应该返回0")

	// 测试参数不足
	cmd = NewCommand("SREM", []string{"set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SREM
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SREM", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SREM应该返回错误")
}

// TestExecSIsMember 测试SISMEMBER命令
func TestExecSIsMember(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	setVal := ds2.NewSet()
	setVal.Add("member1", "member2", "member3")
	db.Set("set1", setVal, 0)

	// 测试成员存在
	cmd := NewCommand("SISMEMBER", []string{"set1", "member1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SISMEMBER应该返回整数回复")
	assert.Equal(t, "1", reply.String(), "成员存在应该返回1")

	// 测试成员不存在
	cmd = NewCommand("SISMEMBER", []string{"set1", "member4"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SISMEMBER应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "成员不存在应该返回0")

	// 测试键不存在
	cmd = NewCommand("SISMEMBER", []string{"set2", "member1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SISMEMBER应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "键不存在应该返回0")

	// 测试参数不足
	cmd = NewCommand("SISMEMBER", []string{"set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SISMEMBER
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SISMEMBER", []string{"string1", "member"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SISMEMBER应该返回错误")
}

// TestExecSMembers 测试SMEMBERS命令
func TestExecSMembers(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	setVal := ds2.NewSet()
	setVal.Add("member1", "member2", "member3")
	db.Set("set1", setVal, 0)

	// 测试SMEMBERS
	cmd := NewCommand("SMEMBERS", []string{"set1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SMEMBERS应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("SMEMBERS", []string{"set2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SMEMBERS应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("SMEMBERS", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SMEMBERS
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SMEMBERS", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SMEMBERS应该返回错误")
}

// TestExecSCard 测试SCARD命令
func TestExecSCard(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	setVal := ds2.NewSet()
	setVal.Add("member1", "member2", "member3")
	db.Set("set1", setVal, 0)

	// 测试SCARD
	cmd := NewCommand("SCARD", []string{"set1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SCARD应该返回整数回复")
	assert.Equal(t, "3", reply.String(), "set1应该有3个成员")

	// 测试不存在的键
	cmd = NewCommand("SCARD", []string{"set2"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyInteger, reply.Type(), "SCARD应该返回整数回复")
	assert.Equal(t, "0", reply.String(), "不存在的键应该返回0")

	// 测试参数不足
	cmd = NewCommand("SCARD", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SCARD
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SCARD", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SCARD应该返回错误")
}

// TestExecSPop 测试SPOP命令
func TestExecSPop(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	setVal := ds2.NewSet()
	setVal.Add("member1", "member2", "member3")
	db.Set("set1", setVal, 0)

	// 测试SPOP
	cmd := NewCommand("SPOP", []string{"set1"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "SPOP应该返回批量回复")

	// 验证集合大小减少
	val, _ := db.Get("set1")
	assert.Equal(t, int64(2), val.(*ds2.Set).Len(), "SPOP后set1应该有2个成员")

	// 测试继续SPOP
	cmd = NewCommand("SPOP", []string{"set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "SPOP应该返回批量回复")

	// 测试最后一个成员SPOP后集合是否被删除
	cmd = NewCommand("SPOP", []string{"set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyBulk, reply.Type(), "SPOP应该返回批量回复")

	// 验证集合为空后是否被删除
	exists := db.Exists("set1")
	assert.False(t, exists, "所有成员弹出后集合应该被删除")

	// 测试对空集合SPOP
	cmd = NewCommand("SPOP", []string{"set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyNil, reply.Type(), "对空集合SPOP应该返回nil回复")

	// 测试参数不足
	cmd = NewCommand("SPOP", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SPOP
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SPOP", []string{"string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SPOP应该返回错误")
}

// TestExecSInter 测试SINTER命令
func TestExecSInter(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	set1 := ds2.NewSet()
	set1.Add("a", "b", "c", "d")
	db.Set("set1", set1, 0)

	set2 := ds2.NewSet()
	set2.Add("c", "d", "e", "f")
	db.Set("set2", set2, 0)

	set3 := ds2.NewSet()
	set3.Add("a", "c", "e")
	db.Set("set3", set3, 0)

	// 测试两个集合的交集
	cmd := NewCommand("SINTER", []string{"set1", "set2"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SINTER应该返回多批量回复")

	// 测试三个集合的交集
	cmd = NewCommand("SINTER", []string{"set1", "set2", "set3"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SINTER应该返回多批量回复")

	// 测试与空集合的交集
	emptySet := ds2.NewSet()
	db.Set("emptySet", emptySet, 0)
	cmd = NewCommand("SINTER", []string{"set1", "emptySet"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SINTER应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("SINTER", []string{"set1", "nonexistent"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SINTER应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("SINTER", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SINTER
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SINTER", []string{"set1", "string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SINTER应该返回错误")
}

// TestExecSUnion 测试SUNION命令
func TestExecSUnion(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	set1 := ds2.NewSet()
	set1.Add("a", "b", "c")
	db.Set("set1", set1, 0)

	set2 := ds2.NewSet()
	set2.Add("c", "d", "e")
	db.Set("set2", set2, 0)

	// 测试两个集合的并集
	cmd := NewCommand("SUNION", []string{"set1", "set2"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SUNION应该返回多批量回复")

	// 测试与空集合的并集
	emptySet := ds2.NewSet()
	db.Set("emptySet", emptySet, 0)
	cmd = NewCommand("SUNION", []string{"set1", "emptySet"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SUNION应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("SUNION", []string{"set1", "nonexistent"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SUNION应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("SUNION", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SUNION
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SUNION", []string{"set1", "string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SUNION应该返回错误")
}

// TestExecSDiff 测试SDIFF命令
func TestExecSDiff(t *testing.T) {
	config := cache2.DefaultConfig()
	db := NewMemoryDatabase(0, &config)
	defer db.Close()

	// 准备测试数据
	set1 := ds2.NewSet()
	set1.Add("a", "b", "c", "d")
	db.Set("set1", set1, 0)

	set2 := ds2.NewSet()
	set2.Add("c", "d", "e")
	db.Set("set2", set2, 0)

	// 测试两个集合的差集
	cmd := NewCommand("SDIFF", []string{"set1", "set2"}, "client1")
	reply := db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SDIFF应该返回多批量回复")

	// 测试反向差集
	cmd = NewCommand("SDIFF", []string{"set2", "set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SDIFF应该返回多批量回复")

	// 测试与空集合的差集
	emptySet := ds2.NewSet()
	db.Set("emptySet", emptySet, 0)
	cmd = NewCommand("SDIFF", []string{"set1", "emptySet"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SDIFF应该返回多批量回复")

	// 测试空集合与其他集合的差集
	cmd = NewCommand("SDIFF", []string{"emptySet", "set1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SDIFF应该返回多批量回复")

	// 测试不存在的键
	cmd = NewCommand("SDIFF", []string{"set1", "nonexistent"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyMultiBulk, reply.Type(), "SDIFF应该返回多批量回复")

	// 测试参数不足
	cmd = NewCommand("SDIFF", []string{}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "参数不足应该返回错误")

	// 测试对非集合类型执行SDIFF
	db.Set("string1", ds2.NewString("value"), 0)
	cmd = NewCommand("SDIFF", []string{"set1", "string1"}, "client1")
	reply = db.ProcessCommand(cmd)
	assert.Equal(t, cache2.ReplyError, reply.Type(), "对非集合类型执行SDIFF应该返回错误")
}
