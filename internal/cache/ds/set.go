package ds

import (
	"encoding/binary"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"math/rand"
	"sync"
	"time"
)

// Set 集合值实现
type Set struct {
	mutex   sync.RWMutex
	members map[string]struct{} // 使用空结构体以节省内存
}

// NewSet 创建一个新的集合值
func NewSet() *Set {
	return &Set{
		members: make(map[string]struct{}),
	}
}

// Type 返回值的类型
func (s *Set) Type() cache.DataType {
	return cache.TypeSet
}

// Encode 将值编码为字节数组
func (s *Set) Encode() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 计算所需的缓冲区大小
	size := 5 // 类型(1字节) + 元素数量(4字节)

	// 计算所有成员字符串所需的空间
	memberSizes := make(map[string]int, len(s.members))
	for member := range s.members {
		memberSizes[member] = len(member)
		size += 4 + len(member) // 每个成员: 长度(4字节) + 内容
	}

	// 分配缓冲区
	buf := make([]byte, size)

	// 写入类型
	buf[0] = byte(cache.TypeSet)

	// 写入元素数量
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(s.members)))

	// 写入所有元素
	offset := 5
	for member := range s.members {
		memberLen := memberSizes[member]

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
func (s *Set) Size() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 估算集合内存占用: 基本结构 + 每个元素的开销
	var totalSize int64 = 32 // 基本的集合结构开销

	// 遍历集合计算内存占用
	for member := range s.members {
		// 每个成员: map条目开销 + 字符串开销
		totalSize += int64(16 + len(member))
	}

	return totalSize
}

// DeepCopy 创建值的深度拷贝
func (s *Set) DeepCopy() cache.Value {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	newSet := NewSet()
	for member := range s.members {
		newSet.members[member] = struct{}{}
	}

	return newSet
}

// Len 返回集合长度
func (s *Set) Len() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return int64(len(s.members))
}

// Add 添加元素到集合
func (s *Set) Add(members ...string) int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var added int64
	for _, member := range members {
		if _, exists := s.members[member]; !exists {
			s.members[member] = struct{}{}
			added++
		}
	}

	return added
}

// Members 获取集合中的所有元素
func (s *Set) Members() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]string, 0, len(s.members))
	for member := range s.members {
		result = append(result, member)
	}

	return result
}

// IsMember 判断元素是否在集合中
func (s *Set) IsMember(member string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	_, exists := s.members[member]
	return exists
}

// Remove 从集合中移除元素
func (s *Set) Remove(members ...string) int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var removed int64
	for _, member := range members {
		if _, exists := s.members[member]; exists {
			delete(s.members, member)
			removed++
		}
	}

	return removed
}

// Pop 随机移除并返回一个元素
func (s *Set) Pop() (string, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.members) == 0 {
		return "", false
	}

	// 随机选择一个元素
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	i := r.Intn(len(s.members))

	var member string
	var count int
	for m := range s.members {
		if count == i {
			member = m
			break
		}
		count++
	}

	delete(s.members, member)
	return member, true
}

// Diff 返回与其他集合的差集
func (s *Set) Diff(others ...cache.SetValue) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 创建一个临时结果集
	result := make(map[string]struct{})
	for member := range s.members {
		result[member] = struct{}{}
	}

	// 遍历其他集合，从结果中删除其他集合中存在的元素
	for _, other := range others {
		for _, member := range other.Members() {
			delete(result, member)
		}
	}

	// 将结果集转换为字符串数组
	members := make([]string, 0, len(result))
	for member := range result {
		members = append(members, member)
	}

	return members
}

// Inter 返回与其他集合的交集
func (s *Set) Inter(others ...cache.SetValue) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(others) == 0 {
		// 如果没有其他集合，则返回当前集合的所有元素
		return s.Members()
	}

	// 查找最小的集合作为基础集合（优化性能）
	minSetIndex := -1
	minSetSize := int64(-1)

	// 检查当前集合大小
	currentSize := int64(len(s.members))
	minSetSize = currentSize

	// 检查其他集合的大小
	for i, other := range others {
		size := other.Len()
		if size < minSetSize {
			minSetSize = size
			minSetIndex = i
		}
	}

	var baseMembers []string

	// 使用最小的集合作为基础集合
	if minSetIndex == -1 {
		// 当前集合是最小的
		baseMembers = s.Members()
	} else {
		// 其他集合是最小的
		baseMembers = others[minSetIndex].Members()
	}

	// 创建结果集
	result := make([]string, 0)

	// 检查最小集合中的每个元素是否存在于所有其他集合中
	for _, member := range baseMembers {
		inAllSets := true

		// 检查当前集合（如果不是基础集合）
		if minSetIndex != -1 {
			if _, ok := s.members[member]; !ok {
				inAllSets = false
			}
		}

		// 检查所有其他集合
		if inAllSets {
			for i, other := range others {
				if i != minSetIndex && !other.IsMember(member) {
					inAllSets = false
					break
				}
			}
		}

		if inAllSets {
			result = append(result, member)
		}
	}

	return result
}

// Union 返回与其他集合的并集
func (s *Set) Union(others ...cache.SetValue) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 创建结果集
	result := make(map[string]struct{}, len(s.members))

	// 添加当前集合的所有元素
	for member := range s.members {
		result[member] = struct{}{}
	}

	// 添加其他集合的所有元素
	for _, other := range others {
		for _, member := range other.Members() {
			result[member] = struct{}{}
		}
	}

	// 将结果集转换为字符串数组
	members := make([]string, 0, len(result))
	for member := range result {
		members = append(members, member)
	}

	return members
}

// DecodeSet 从字节数组解码集合值
func DecodeSet(data []byte) (*Set, error) {
	if len(data) < 5 || cache.DataType(data[0]) != cache.TypeSet {
		return nil, fmt.Errorf("invalid set encoding")
	}

	count := binary.BigEndian.Uint32(data[1:5])
	set := NewSet()

	offset := 5
	for i := uint32(0); i < count; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("invalid set encoding: unexpected end of data")
		}

		memberLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(memberLen) > len(data) {
			return nil, fmt.Errorf("invalid set encoding: unexpected end of member data")
		}

		member := string(data[offset : offset+int(memberLen)])
		offset += int(memberLen)

		set.members[member] = struct{}{}
	}

	return set, nil
}
