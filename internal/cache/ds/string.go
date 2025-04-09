// Package ds 提供缓存系统的数据结构实现
package ds

import (
	"encoding/binary"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"strconv"
	"sync"
)

// String 字符串值实现
type String struct {
	mutex sync.RWMutex
	value string
}

// NewString 创建一个新的字符串值
func NewString(val string) *String {
	return &String{
		value: val,
	}
}

// Type 返回值的类型
func (s *String) Type() cache.DataType {
	return cache.TypeString
}

// Encode 将值编码为字节数组
func (s *String) Encode() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 编码格式：类型(1字节) + 字符串长度(4字节) + 字符串内容
	buf := make([]byte, 5+len(s.value))
	buf[0] = byte(cache.TypeString)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(s.value)))
	copy(buf[5:], s.value)

	return buf, nil
}

// Size 返回值的大小（字节数）
func (s *String) Size() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Go 中的字符串由两部分组成：
	// 1. 指向底层字节数组的指针（8字节，在64位系统上）
	// 2. 表示字符串长度的整数（8字节，在64位系统上）
	// 再加上字符串内容本身所占的字节数
	stringHeaderSize := int64(16) // 字符串头部大小（指针+长度）
	stringContentSize := int64(len(s.value))

	// mutex 的大小约为 8 字节（取决于具体实现和平台）
	// 加上字符串在结构体中的引用（8字节）
	structOverhead := int64(16)

	return stringHeaderSize + stringContentSize + structOverhead
}

// DeepCopy 创建值的深度拷贝
func (s *String) DeepCopy() cache.Value {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return NewString(s.value)
}

// String 返回字符串值
func (s *String) String() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.value
}

// SetString 设置字符串值
func (s *String) SetString(val string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.value = val
}

// IncrBy 增加整数值
func (s *String) IncrBy(delta int64) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 尝试将字符串转换为整数
	val, err := strconv.ParseInt(s.value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("value is not an integer or out of range")
	}

	// 执行增加操作
	val += delta
	s.value = strconv.FormatInt(val, 10)

	return val, nil
}

// IncrByFloat 增加浮点数值
func (s *String) IncrByFloat(delta float64) (float64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 尝试将字符串转换为浮点数
	val, err := strconv.ParseFloat(s.value, 64)
	if err != nil {
		return 0, fmt.Errorf("value is not a valid float")
	}

	// 执行增加操作
	val += delta
	s.value = strconv.FormatFloat(val, 'f', -1, 64)

	return val, nil
}

// DecrBy 减少整数值
func (s *String) DecrBy(delta int64) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 尝试将字符串转换为整数
	val, err := strconv.ParseInt(s.value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("value is not an integer or out of range")
	}

	// 执行减少操作
	val -= delta
	s.value = strconv.FormatInt(val, 10)

	return val, nil
}

// Append 追加字符串
func (s *String) Append(val string) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.value += val
	return len(s.value)
}

// DecodeString 从字节数组解码字符串值
func DecodeString(data []byte) (*String, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("invalid string encoding: data too short")
	}

	// 检查类型字段是否匹配
	if cache.DataType(data[0]) != cache.TypeString {
		return nil, fmt.Errorf("invalid string encoding: expected type %d, got %d", cache.TypeString, data[0])
	}

	// 读取字符串长度
	length := binary.BigEndian.Uint32(data[1:5])

	// 确保数据长度充足
	if len(data) < 5+int(length) {
		return nil, fmt.Errorf("invalid string encoding: expected length %d, got %d", length, len(data)-5)
	}

	// 提取字符串内容
	value := string(data[5 : 5+length])

	return NewString(value), nil
}
