package ds

import (
	"encoding/binary"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"math"
	"sort"
	"sync"
)

// ZSet 有序集合值实现
type ZSet struct {
	mutex   sync.RWMutex
	dict    map[string]float64 // 成员到分数的映射
	skipped bool               // 是否跳过了排序（在进行范围查询前需要排序）
	entries []ZSetEntry        // 已排序的条目列表
}

// ZSetEntry 有序集合中的条目
type ZSetEntry struct {
	Member string
	Score  float64
}

// NewZSet 创建一个新的有序集合值
func NewZSet() *ZSet {
	return &ZSet{
		dict:    make(map[string]float64),
		skipped: true,
		entries: make([]ZSetEntry, 0),
	}
}

// Type 返回值的类型
func (z *ZSet) Type() cache.DataType {
	return cache.TypeZSet
}

// Encode 将值编码为字节数组
func (z *ZSet) Encode() ([]byte, error) {
	z.mutex.RLock()
	defer z.mutex.RUnlock()

	// 计算所需的缓冲区大小
	size := 5 // 类型(1字节) + 元素数量(4字节)

	// 计算所有成员字符串和分数所需的空间
	memberSizes := make(map[string]int, len(z.dict))
	for member := range z.dict {
		memberSizes[member] = len(member)
		size += 12 + len(member) // 每个条目: 分数(8字节) + 成员长度(4字节) + 成员内容
	}

	// 分配缓冲区
	buf := make([]byte, size)

	// 写入类型
	buf[0] = byte(cache.TypeZSet)

	// 写入元素数量
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(z.dict)))

	// 写入所有元素
	offset := 5
	for member, score := range z.dict {
		memberLen := memberSizes[member]

		// 写入分数
		bits := math.Float64bits(score)
		binary.BigEndian.PutUint64(buf[offset:offset+8], bits)
		offset += 8

		// 写入成员长度
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(memberLen))
		offset += 4

		// 写入成员内容
		copy(buf[offset:offset+memberLen], member)
		offset += memberLen
	}

	return buf, nil
}

// Size 返回值的大小（字节数）
func (z *ZSet) Size() int64 {
	z.mutex.RLock()
	defer z.mutex.RUnlock()

	// 估算有序集合内存占用: 基本结构 + 字典开销 + 每个条目的开销
	var totalSize int64 = 48 // 基本的有序集合结构开销

	// 字典开销
	totalSize += int64(16 * len(z.dict)) // 每个map条目约16字节

	// 遍历集合计算内存占用
	for member, _ := range z.dict {
		// 每个成员: 字符串开销 + float64开销
		totalSize += int64(len(member) + 8)
	}

	// 排序列表开销
	if !z.skipped {
		totalSize += int64(24 * len(z.entries)) // 每个条目约24字节（包括指针和结构体开销）
	}

	return totalSize
}

// DeepCopy 创建值的深度拷贝
func (z *ZSet) DeepCopy() cache.Value {
	z.mutex.RLock()
	defer z.mutex.RUnlock()

	newZSet := NewZSet()
	for member, score := range z.dict {
		newZSet.dict[member] = score
	}

	newZSet.skipped = true // 需要重新排序

	return newZSet
}

// Len 返回有序集合长度
func (z *ZSet) Len() int64 {
	z.mutex.RLock()
	defer z.mutex.RUnlock()

	return int64(len(z.dict))
}

// Add 添加元素到有序集合
func (z *ZSet) Add(score float64, member string) bool {
	z.mutex.Lock()
	defer z.mutex.Unlock()

	_, exists := z.dict[member]
	z.dict[member] = score
	z.skipped = true // 需要重新排序

	return !exists
}

// Score 获取有序集合中元素的分数
func (z *ZSet) Score(member string) (float64, bool) {
	z.mutex.RLock()
	defer z.mutex.RUnlock()

	score, exists := z.dict[member]
	return score, exists
}

// IncrBy 对有序集合成员的分数增加delta
func (z *ZSet) IncrBy(member string, delta float64) (float64, bool) {
	z.mutex.Lock()
	defer z.mutex.Unlock()

	score, exists := z.dict[member]
	if !exists {
		// 如果成员不存在，创建一个新的成员
		z.dict[member] = delta
		z.skipped = true // 需要重新排序
		return delta, false
	}

	// 增加分数
	newScore := score + delta
	z.dict[member] = newScore
	z.skipped = true // 需要重新排序

	return newScore, true
}

// sort 对有序集合进行排序（内部方法）
func (z *ZSet) sort() {
	if !z.skipped {
		return
	}

	// 重新创建条目切片
	z.entries = make([]ZSetEntry, 0, len(z.dict))
	for member, score := range z.dict {
		z.entries = append(z.entries, ZSetEntry{
			Member: member,
			Score:  score,
		})
	}

	// 按分数升序排序
	sort.Slice(z.entries, func(i, j int) bool {
		if z.entries[i].Score == z.entries[j].Score {
			// 如果分数相同，按成员名字典序排序
			return z.entries[i].Member < z.entries[j].Member
		}
		return z.entries[i].Score < z.entries[j].Score
	})

	z.skipped = false
}

// Range 获取有序集合范围内的元素
func (z *ZSet) Range(start, stop int64) []string {
	z.mutex.Lock()
	// 先排序
	z.sort()
	z.mutex.Unlock()

	z.mutex.RLock()
	defer z.mutex.RUnlock()

	size := int64(len(z.entries))

	// 处理负索引
	if start < 0 {
		start = size + start
		if start < 0 {
			start = 0
		}
	}

	if stop < 0 {
		stop = size + stop
		if stop < 0 {
			stop = 0
		}
	}

	// 确保索引在范围内
	if start >= size {
		return []string{}
	}

	if stop >= size {
		stop = size - 1
	}

	// 确保 start <= stop
	if start > stop {
		return []string{}
	}

	// 获取指定范围的元素
	result := make([]string, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		result = append(result, z.entries[i].Member)
	}

	return result
}

// RangeWithScores 获取有序集合范围内的元素和分数
func (z *ZSet) RangeWithScores(start, stop int64) map[string]float64 {
	z.mutex.Lock()
	// 先排序
	z.sort()
	z.mutex.Unlock()

	z.mutex.RLock()
	defer z.mutex.RUnlock()

	size := int64(len(z.entries))

	// 处理负索引
	if start < 0 {
		start = size + start
		if start < 0 {
			start = 0
		}
	}

	if stop < 0 {
		stop = size + stop
		if stop < 0 {
			stop = 0
		}
	}

	// 确保索引在范围内
	if start >= size {
		return make(map[string]float64)
	}

	if stop >= size {
		stop = size - 1
	}

	// 确保 start <= stop
	if start > stop {
		return make(map[string]float64)
	}

	// 获取指定范围的元素和分数
	result := make(map[string]float64, stop-start+1)
	for i := start; i <= stop; i++ {
		result[z.entries[i].Member] = z.entries[i].Score
	}

	return result
}

// RangeByScore 按分数获取有序集合范围内的元素
func (z *ZSet) RangeByScore(min, max float64) []string {
	z.mutex.Lock()
	// 先排序
	z.sort()
	z.mutex.Unlock()

	z.mutex.RLock()
	defer z.mutex.RUnlock()

	// 找到第一个分数大于等于min的索引
	start := sort.Search(len(z.entries), func(i int) bool {
		return z.entries[i].Score >= min
	})

	// 如果所有分数都小于min，则返回空结果
	if start == len(z.entries) {
		return []string{}
	}

	// 找到第一个分数大于max的索引
	end := sort.Search(len(z.entries), func(i int) bool {
		return z.entries[i].Score > max
	})

	// 获取指定范围的元素
	result := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, z.entries[i].Member)
	}

	return result
}

// RangeByScoreWithScores 按分数获取有序集合范围内的元素和分数
func (z *ZSet) RangeByScoreWithScores(min, max float64) map[string]float64 {
	z.mutex.Lock()
	// 先排序
	z.sort()
	z.mutex.Unlock()

	z.mutex.RLock()
	defer z.mutex.RUnlock()

	// 找到第一个分数大于等于min的索引
	start := sort.Search(len(z.entries), func(i int) bool {
		return z.entries[i].Score >= min
	})

	// 如果所有分数都小于min，则返回空结果
	if start == len(z.entries) {
		return make(map[string]float64)
	}

	// 找到第一个分数大于max的索引
	end := sort.Search(len(z.entries), func(i int) bool {
		return z.entries[i].Score > max
	})

	// 获取指定范围的元素和分数
	result := make(map[string]float64, end-start)
	for i := start; i < end; i++ {
		result[z.entries[i].Member] = z.entries[i].Score
	}

	return result
}

// Rank 获取有序集合成员的排名
func (z *ZSet) Rank(member string) (int64, bool) {
	// 先检查成员是否存在
	if _, exists := z.Score(member); !exists {
		return 0, false
	}

	z.mutex.Lock()
	// 先排序
	z.sort()
	z.mutex.Unlock()

	z.mutex.RLock()
	defer z.mutex.RUnlock()

	// 查找成员的排名
	for i, entry := range z.entries {
		if entry.Member == member {
			return int64(i), true
		}
	}

	// 这种情况不应该发生，因为我们已经检查了成员是否存在
	return 0, false
}

// Remove 从有序集合中移除元素
func (z *ZSet) Remove(members ...string) int64 {
	z.mutex.Lock()
	defer z.mutex.Unlock()

	var removed int64
	for _, member := range members {
		if _, exists := z.dict[member]; exists {
			delete(z.dict, member)
			removed++
		}
	}

	if removed > 0 {
		z.skipped = true // 需要重新排序
	}

	return removed
}

// DecodeZSet 从字节数组解码有序集合值
func DecodeZSet(data []byte) (*ZSet, error) {
	if len(data) < 5 || cache.DataType(data[0]) != cache.TypeZSet {
		return nil, fmt.Errorf("invalid zset encoding")
	}

	count := binary.BigEndian.Uint32(data[1:5])
	zset := NewZSet()

	offset := 5
	for i := uint32(0); i < count; i++ {
		if offset+12 > len(data) {
			return nil, fmt.Errorf("invalid zset encoding: unexpected end of data")
		}

		// 读取分数
		bits := binary.BigEndian.Uint64(data[offset : offset+8])
		score := math.Float64frombits(bits)
		offset += 8

		// 读取成员长度
		memberLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(memberLen) > len(data) {
			return nil, fmt.Errorf("invalid zset encoding: unexpected end of member data")
		}

		// 读取成员
		member := string(data[offset : offset+int(memberLen)])
		offset += int(memberLen)

		zset.dict[member] = score
	}

	zset.skipped = true // 需要对新解码的集合进行排序

	return zset, nil
}
