package ds

import (
	"container/list"
	"encoding/binary"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"sync"
)

// List 列表值实现
type List struct {
	mutex sync.RWMutex
	items *list.List
}

// NewList 创建一个新的列表值
func NewList() *List {
	return &List{
		items: list.New(),
	}
}

// Type 返回值的类型
func (l *List) Type() cache.DataType {
	return cache.TypeList
}

// Encode 将值编码为字节数组
func (l *List) Encode() ([]byte, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// 计算所需的缓冲区大小
	size := 5 // 类型(1字节) + 元素数量(4字节)

	// 首先计算所有字符串所需的空间
	stringSizes := make([]int, 0, l.items.Len())
	for e := l.items.Front(); e != nil; e = e.Next() {
		str := e.Value.(string)
		stringSizes = append(stringSizes, len(str))
		size += 4 + len(str) // 每个字符串: 长度(4字节) + 内容
	}

	// 分配缓冲区
	buf := make([]byte, size)

	// 写入类型
	buf[0] = byte(cache.TypeList)

	// 写入元素数量
	binary.BigEndian.PutUint32(buf[1:5], uint32(l.items.Len()))

	// 写入所有元素
	offset := 5
	i := 0
	for e := l.items.Front(); e != nil; e = e.Next() {
		str := e.Value.(string)
		strLen := stringSizes[i]

		// 写入字符串长度
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(strLen))
		offset += 4

		// 写入字符串内容
		copy(buf[offset:offset+strLen], str)
		offset += strLen

		i++
	}

	return buf, nil
}

// Size 返回值的大小（字节数）
func (l *List) Size() int64 {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// 估算列表内存占用: 基本结构 + 节点数量 * (节点开销 + 平均字符串长度)
	var totalSize int64 = 40 // 基本的列表结构开销

	if l.items.Len() > 0 {
		totalStrLen := int64(0)
		for e := l.items.Front(); e != nil; e = e.Next() {
			str := e.Value.(string)
			totalStrLen += int64(len(str))
		}

		// 每个节点有一个指针(8字节)开销，加上Element结构开销
		nodeOverhead := int64(24)
		totalSize += int64(l.items.Len()) * (nodeOverhead + (totalStrLen / int64(l.items.Len())))
	}

	return totalSize
}

// DeepCopy 创建值的深度拷贝
func (l *List) DeepCopy() cache.Value {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	newList := NewList()
	for e := l.items.Front(); e != nil; e = e.Next() {
		newList.items.PushBack(e.Value.(string))
	}

	return newList
}

// Len 返回列表长度
func (l *List) Len() int64 {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	return int64(l.items.Len())
}

// LPush 在列表左侧添加元素
func (l *List) LPush(vals ...string) int64 {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i := len(vals) - 1; i >= 0; i-- {
		l.items.PushFront(vals[i])
	}

	return int64(l.items.Len())
}

// RPush 在列表右侧添加元素
func (l *List) RPush(vals ...string) int64 {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for _, val := range vals {
		l.items.PushBack(val)
	}

	return int64(l.items.Len())
}

// LPop 从列表左侧移除元素
func (l *List) LPop() (string, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.items.Len() == 0 {
		return "", false
	}

	element := l.items.Front()
	l.items.Remove(element)
	return element.Value.(string), true
}

// RPop 从列表右侧移除元素
func (l *List) RPop() (string, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.items.Len() == 0 {
		return "", false
	}

	element := l.items.Back()
	l.items.Remove(element)
	return element.Value.(string), true
}

// Range 获取列表范围内的元素
func (l *List) Range(start, stop int64) []string {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	size := l.items.Len()

	// 处理负索引
	if start < 0 {
		start = int64(size) + start
		if start < 0 {
			start = 0
		}
	}

	if stop < 0 {
		stop = int64(size) + stop
		if stop < 0 {
			stop = 0
		}
	}

	// 确保索引在范围内
	if start >= int64(size) {
		return []string{}
	}

	if stop >= int64(size) {
		stop = int64(size) - 1
	}

	// 确保 start <= stop
	if start > stop {
		return []string{}
	}

	// 获取指定范围的元素
	result := make([]string, 0, stop-start+1)
	var i int64 = 0
	var e *list.Element

	// 如果从头部开始遍历更近，则从头部开始
	if start < int64(size)/2 {
		e = l.items.Front()
		for ; i < start; i++ {
			e = e.Next()
		}
	} else { // 否则从尾部开始向前遍历
		e = l.items.Back()
		for i = int64(size) - 1; i > start; i-- {
			e = e.Prev()
		}
	}

	// 收集元素
	for ; i <= stop; i++ {
		result = append(result, e.Value.(string))
		e = e.Next()
	}

	return result
}

// Index 获取列表中指定位置的元素
func (l *List) Index(index int64) (string, bool) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	size := int64(l.items.Len())

	// 处理负索引
	if index < 0 {
		index = size + index
	}

	// 确保索引在范围内
	if index < 0 || index >= size {
		return "", false
	}

	// 如果从头部开始遍历更近，则从头部开始
	var i int64
	var e *list.Element

	if index < size/2 {
		e = l.items.Front()
		for i = 0; i < index; i++ {
			e = e.Next()
		}
	} else { // 否则从尾部开始向前遍历
		e = l.items.Back()
		for i = size - 1; i > index; i-- {
			e = e.Prev()
		}
	}

	return e.Value.(string), true
}

// SetItem 设置列表中指定位置的元素
func (l *List) SetItem(index int64, val string) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	size := int64(l.items.Len())

	// 处理负索引
	if index < 0 {
		index = size + index
	}

	// 确保索引在范围内
	if index < 0 || index >= size {
		return false
	}

	// 如果从头部开始遍历更近，则从头部开始
	var i int64
	var e *list.Element

	if index < size/2 {
		e = l.items.Front()
		for i = 0; i < index; i++ {
			e = e.Next()
		}
	} else { // 否则从尾部开始向前遍历
		e = l.items.Back()
		for i = size - 1; i > index; i-- {
			e = e.Prev()
		}
	}

	e.Value = val
	return true
}

// RemoveItem 移除指定的元素
func (l *List) RemoveItem(count int64, val string) int64 {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if count == 0 || l.items.Len() == 0 {
		return 0
	}

	var removed int64 = 0
	var next *list.Element

	// 从头部或尾部开始，取决于count的符号
	if count > 0 {
		// 从头部开始，移除最多count个匹配元素
		for e := l.items.Front(); e != nil && removed < count; {
			next = e.Next()
			if e.Value.(string) == val {
				l.items.Remove(e)
				removed++
			}
			e = next
		}
	} else if count < 0 {
		// 从尾部开始，移除最多|count|个匹配元素
		absCount := -count
		for e := l.items.Back(); e != nil && removed < absCount; {
			prev := e.Prev()
			if e.Value.(string) == val {
				l.items.Remove(e)
				removed++
			}
			e = prev
		}
	} else { // count == 0，特殊情况：移除所有匹配元素
		for e := l.items.Front(); e != nil; {
			next = e.Next()
			if e.Value.(string) == val {
				l.items.Remove(e)
				removed++
			}
			e = next
		}
	}

	return removed
}

// DecodeList 从字节数组解码列表值
func DecodeList(data []byte) (*List, error) {
	if len(data) < 5 || cache.DataType(data[0]) != cache.TypeList {
		return nil, fmt.Errorf("invalid list encoding")
	}

	count := binary.BigEndian.Uint32(data[1:5])
	list := NewList()

	offset := 5
	for i := uint32(0); i < count; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("invalid list encoding: unexpected end of data")
		}

		strLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(strLen) > len(data) {
			return nil, fmt.Errorf("invalid list encoding: unexpected end of string data")
		}

		str := string(data[offset : offset+int(strLen)])
		offset += int(strLen)

		list.items.PushBack(str)
	}

	return list, nil
}
