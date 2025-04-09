// Package cache 提供一个类Redis的内存缓存系统
package cache

import (
	"time"
)

// DataType 表示缓存中的数据类型
type DataType uint8

const (
	// TypeString 字符串类型
	TypeString DataType = iota + 1
	// TypeList 列表类型
	TypeList
	// TypeHash 哈希表类型
	TypeHash
	// TypeSet 集合类型
	TypeSet
	// TypeZSet 有序集合类型
	TypeZSet
)

// Value 表示缓存中的值
type Value interface {
	// Type 返回值的类型
	Type() DataType
	// Encode 将值编码为字节数组
	Encode() ([]byte, error)
	// Size 返回值的大小（字节数）
	Size() int64
	// DeepCopy 创建值的深度拷贝
	DeepCopy() Value
}

// StringValue 字符串值接口
type StringValue interface {
	Value
	// String 返回字符串值
	String() string
	// SetString 设置字符串值
	SetString(val string)
	// IncrBy 增加整数值
	IncrBy(delta int64) (int64, error)
	// IncrByFloat 增加浮点数值
	IncrByFloat(delta float64) (float64, error)
	// DecrBy 减少整数值
	DecrBy(delta int64) (int64, error)
	// Append 追加字符串
	Append(val string) int
}

// ListValue 列表值接口
type ListValue interface {
	Value
	// Len 返回列表长度
	Len() int64
	// LPush 在列表左侧添加元素
	LPush(vals ...string) int64
	// RPush 在列表右侧添加元素
	RPush(vals ...string) int64
	// LPop 从列表左侧移除元素
	LPop() (string, bool)
	// RPop 从列表右侧移除元素
	RPop() (string, bool)
	// Range 获取列表范围内的元素
	Range(start, stop int64) []string
	// Index 获取列表中指定位置的元素
	Index(index int64) (string, bool)
	// SetItem 设置列表中指定位置的元素
	SetItem(index int64, val string) bool
	// RemoveItem 移除指定的元素
	RemoveItem(count int64, val string) int64
}

// HashValue 哈希表值接口
type HashValue interface {
	Value
	// Len 返回哈希表长度
	Len() int64
	// Get 获取哈希表中指定字段的值
	Get(field string) (string, bool)
	// Set 设置哈希表中指定字段的值
	Set(field, val string) bool
	// SetNX 当字段不存在时，设置哈希表中指定字段的值
	SetNX(field, val string) bool
	// Del 删除哈希表中指定的字段
	Del(fields ...string) int64
	// GetAll 获取哈希表中所有字段和值
	GetAll() map[string]string
	// Exists 判断字段是否存在
	Exists(field string) bool
	// IncrBy 增加哈希表中指定字段的整数值
	IncrBy(field string, delta int64) (int64, error)
	// IncrByFloat 增加哈希表中指定字段的浮点数值
	IncrByFloat(field string, delta float64) (float64, error)
}

// SetValue 集合值接口
type SetValue interface {
	Value
	// Len 返回集合长度
	Len() int64
	// Add 添加元素到集合
	Add(members ...string) int64
	// Members 获取集合中的所有元素
	Members() []string
	// IsMember 判断元素是否在集合中
	IsMember(member string) bool
	// Remove 从集合中移除元素
	Remove(members ...string) int64
	// Pop 随机移除并返回一个元素
	Pop() (string, bool)
	// Diff 返回与其他集合的差集
	Diff(others ...SetValue) []string
	// Inter 返回与其他集合的交集
	Inter(others ...SetValue) []string
	// Union 返回与其他集合的并集
	Union(others ...SetValue) []string
}

// ZSetValue 有序集合值接口
type ZSetValue interface {
	Value
	// Len 返回有序集合长度
	Len() int64
	// Add 添加元素到有序集合
	Add(score float64, member string) bool
	// Score 获取有序集合中元素的分数
	Score(member string) (float64, bool)
	// IncrBy 对有序集合成员的分数增加delta
	IncrBy(member string, delta float64) (float64, bool)
	// Range 获取有序集合范围内的元素
	Range(start, stop int64) []string
	// RangeWithScores 获取有序集合范围内的元素和分数
	RangeWithScores(start, stop int64) map[string]float64
	// RangeByScore 按分数获取有序集合范围内的元素
	RangeByScore(min, max float64) []string
	// RangeByScoreWithScores 按分数获取有序集合范围内的元素和分数
	RangeByScoreWithScores(min, max float64) map[string]float64
	// Rank 获取有序集合成员的排名
	Rank(member string) (int64, bool)
	// Remove 从有序集合中移除元素
	Remove(members ...string) int64
}

// Database 数据库接口
type Database interface {
	// Get 获取键对应的值
	Get(key string) (Value, bool)
	// Set 设置键值对
	Set(key string, value Value, expiration time.Duration) bool
	// Delete 删除键
	Delete(keys ...string) int64
	// Exists 判断键是否存在
	Exists(key string) bool
	// Expire 设置键的过期时间
	Expire(key string, expiration time.Duration) bool
	// TTL 获取键的剩余过期时间
	TTL(key string) time.Duration
	// Keys 按模式获取匹配的键
	Keys(pattern string) []string
	// Flush 清空数据库
	Flush()
	// Size 返回数据库中键的数量
	Size() int64
	// Close 关闭数据库
	Close() error
	// ProcessCommand 处理缓存命令
	ProcessCommand(cmd Command) Reply
}

// Engine 缓存引擎接口
type Engine interface {
	// Select 选择数据库
	Select(dbIndex int) (Database, error)
	// FlushAll 清空所有数据库
	FlushAll()
	// GetStats 获取引擎统计信息
	GetStats() map[string]interface{}
	// Close 关闭引擎
	Close() error
}

// ReplyType 表示回复类型
type ReplyType uint8

const (
	// ReplyStatus 状态回复
	ReplyStatus ReplyType = iota + 1
	// ReplyError 错误回复
	ReplyError
	// ReplyInteger 整数回复
	ReplyInteger
	// ReplyBulk 块回复
	ReplyBulk
	// ReplyMultiBulk 多块回复
	ReplyMultiBulk
	// ReplyNil 空回复
	ReplyNil
)
