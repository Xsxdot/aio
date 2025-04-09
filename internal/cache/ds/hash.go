package ds

import (
	"encoding/binary"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"strconv"
	"sync"
)

// Hash 哈希表值实现
type Hash struct {
	mutex  sync.RWMutex
	fields map[string]string
}

// NewHash 创建一个新的哈希表值
func NewHash() *Hash {
	return &Hash{
		fields: make(map[string]string),
	}
}

// Type 返回值的类型
func (h *Hash) Type() cache.DataType {
	return cache.TypeHash
}

// Encode 将值编码为字节数组
func (h *Hash) Encode() ([]byte, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// 计算所需的缓冲区大小
	size := 5 // 类型(1字节) + 字段数量(4字节)

	// 计算所有字段名和值所需的空间
	fieldSizes := make(map[string]int, len(h.fields))
	valueSizes := make(map[string]int, len(h.fields))

	for field, value := range h.fields {
		fieldSizes[field] = len(field)
		valueSizes[field] = len(value)
		size += 8 + len(field) + len(value) // 字段长度(4字节) + 值长度(4字节) + 字段 + 值
	}

	// 分配缓冲区
	buf := make([]byte, size)

	// 写入类型
	buf[0] = byte(cache.TypeHash)

	// 写入字段数量
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(h.fields)))

	// 写入所有字段和值
	offset := 5
	for field, value := range h.fields {
		fieldLen := fieldSizes[field]
		valueLen := valueSizes[field]

		// 写入字段长度
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(fieldLen))
		offset += 4

		// 写入字段
		copy(buf[offset:offset+fieldLen], field)
		offset += fieldLen

		// 写入值长度
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(valueLen))
		offset += 4

		// 写入值
		copy(buf[offset:offset+valueLen], value)
		offset += valueLen
	}

	return buf, nil
}

// Size 返回值的大小（字节数）
func (h *Hash) Size() int64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// 估算哈希表内存占用: 基本结构 + 哈希表开销 + 字段数 * (字段开销 + 值开销)
	var totalSize int64 = 48 // 基本的哈希表结构开销

	// 计算所有字段和值的总大小
	for field, value := range h.fields {
		// 每个字段和值: 哈希表条目开销(约16字节) + 字段字符串 + 值字符串
		totalSize += int64(16 + len(field) + len(value))
	}

	return totalSize
}

// DeepCopy 创建值的深度拷贝
func (h *Hash) DeepCopy() cache.Value {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	newHash := NewHash()
	for field, value := range h.fields {
		newHash.fields[field] = value
	}

	return newHash
}

// Len 返回哈希表长度
func (h *Hash) Len() int64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return int64(len(h.fields))
}

// Get 获取哈希表中指定字段的值
func (h *Hash) Get(field string) (string, bool) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	value, exists := h.fields[field]
	return value, exists
}

// Set 设置哈希表中指定字段的值
func (h *Hash) Set(field, val string) bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	_, exists := h.fields[field]
	h.fields[field] = val
	return !exists
}

// SetNX 当字段不存在时，设置哈希表中指定字段的值
func (h *Hash) SetNX(field, val string) bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, exists := h.fields[field]; !exists {
		h.fields[field] = val
		return true
	}

	return false
}

// Del 删除哈希表中指定的字段
func (h *Hash) Del(fields ...string) int64 {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	var deleted int64 = 0
	for _, field := range fields {
		if _, exists := h.fields[field]; exists {
			delete(h.fields, field)
			deleted++
		}
	}

	return deleted
}

// GetAll 获取哈希表中所有字段和值
func (h *Hash) GetAll() map[string]string {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string]string, len(h.fields))
	for field, value := range h.fields {
		result[field] = value
	}

	return result
}

// Exists 判断字段是否存在
func (h *Hash) Exists(field string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	_, exists := h.fields[field]
	return exists
}

// IncrBy 增加哈希表中指定字段的整数值
func (h *Hash) IncrBy(field string, delta int64) (int64, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	value, exists := h.fields[field]
	if !exists {
		// 如果字段不存在，则创建它并设置为增量值
		h.fields[field] = strconv.FormatInt(delta, 10)
		return delta, nil
	}

	// 尝试将字段值转换为整数
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("value is not an integer or out of range")
	}

	// 执行增加操作
	val += delta
	h.fields[field] = strconv.FormatInt(val, 10)

	return val, nil
}

// IncrByFloat 增加哈希表中指定字段的浮点数值
func (h *Hash) IncrByFloat(field string, delta float64) (float64, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	value, exists := h.fields[field]
	if !exists {
		// 如果字段不存在，则创建它并设置为增量值
		h.fields[field] = strconv.FormatFloat(delta, 'f', -1, 64)
		return delta, nil
	}

	// 尝试将字段值转换为浮点数
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("value is not a valid float")
	}

	// 执行增加操作
	val += delta
	h.fields[field] = strconv.FormatFloat(val, 'f', -1, 64)

	return val, nil
}

// DecodeHash 从字节数组解码哈希表值
func DecodeHash(data []byte) (*Hash, error) {
	if len(data) < 5 || cache.DataType(data[0]) != cache.TypeHash {
		return nil, fmt.Errorf("invalid hash encoding")
	}

	count := binary.BigEndian.Uint32(data[1:5])
	hash := NewHash()

	offset := 5
	for i := uint32(0); i < count; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("invalid hash encoding: unexpected end of data")
		}

		// 读取字段长度
		fieldLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(fieldLen) > len(data) {
			return nil, fmt.Errorf("invalid hash encoding: unexpected end of field data")
		}

		// 读取字段
		field := string(data[offset : offset+int(fieldLen)])
		offset += int(fieldLen)

		if offset+4 > len(data) {
			return nil, fmt.Errorf("invalid hash encoding: unexpected end of data")
		}

		// 读取值长度
		valueLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(valueLen) > len(data) {
			return nil, fmt.Errorf("invalid hash encoding: unexpected end of value data")
		}

		// 读取值
		value := string(data[offset : offset+int(valueLen)])
		offset += int(valueLen)

		hash.fields[field] = value
	}

	return hash, nil
}
