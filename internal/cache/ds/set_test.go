package ds

import (
	"github.com/xsxdot/aio/internal/cache"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewSet 测试创建新的集合
func TestNewSet(t *testing.T) {
	s := NewSet()
	if s == nil {
		t.Fatal("NewSet() 返回 nil")
	}
	if s.members == nil {
		t.Fatal("NewSet() 创建的集合 members 字段为 nil")
	}
	if len(s.members) != 0 {
		t.Errorf("NewSet() 创建的集合不是空的, 大小为 %d", len(s.members))
	}
}

// TestSetType 测试集合类型
func TestSetType(t *testing.T) {
	s := NewSet()
	if s.Type() != cache.TypeSet {
		t.Errorf("Set.Type() = %v, 期望 %v", s.Type(), cache.TypeSet)
	}
}

// TestSetSize 测试集合大小计算
func TestSetSize(t *testing.T) {
	s := NewSet()

	// 空集合
	emptySize := s.Size()
	if emptySize <= 0 {
		t.Errorf("空集合大小计算错误，返回 %d", emptySize)
	}

	// 添加元素后
	s.Add("key1", "key2", "key3")
	nonEmptySize := s.Size()
	if nonEmptySize <= emptySize {
		t.Errorf("添加元素后集合大小计算错误，返回 %d，应该大于 %d", nonEmptySize, emptySize)
	}

	// 验证大小随元素数量增加
	s.Add("key4", "key5")
	largerSize := s.Size()
	if largerSize <= nonEmptySize {
		t.Errorf("进一步添加元素后集合大小计算错误，返回 %d，应该大于 %d", largerSize, nonEmptySize)
	}
}

// TestSetAdd 测试集合添加元素
func TestSetAdd(t *testing.T) {
	s := NewSet()

	// 测试添加单个元素
	added := s.Add("key1")
	if added != 1 {
		t.Errorf("Add(\"key1\") 返回 %d, 期望 1", added)
	}
	if len(s.members) != 1 {
		t.Errorf("添加单个元素后集合大小 = %d, 期望 1", len(s.members))
	}

	// 测试添加多个元素
	added = s.Add("key2", "key3", "key4")
	if added != 3 {
		t.Errorf("Add(\"key2\", \"key3\", \"key4\") 返回 %d, 期望 3", added)
	}
	if len(s.members) != 4 {
		t.Errorf("添加3个元素后集合大小 = %d, 期望 4", len(s.members))
	}

	// 测试添加重复元素
	added = s.Add("key1", "key2")
	if added != 0 {
		t.Errorf("添加重复元素 Add(\"key1\", \"key2\") 返回 %d, 期望 0", added)
	}
	if len(s.members) != 4 {
		t.Errorf("添加重复元素后集合大小 = %d, 期望仍然是 4", len(s.members))
	}

	// 测试添加空字符串
	added = s.Add("")
	if added != 1 {
		t.Errorf("Add(\"\") 返回 %d, 期望 1", added)
	}
	if len(s.members) != 5 {
		t.Errorf("添加空字符串后集合大小 = %d, 期望 5", len(s.members))
	}
}

// TestSetMembers 测试获取集合成员
func TestSetMembers(t *testing.T) {
	s := NewSet()

	// 空集合
	members := s.Members()
	if len(members) != 0 {
		t.Errorf("空集合 Members() 返回 %v, 期望空切片", members)
	}

	// 添加元素后
	expected := []string{"key1", "key2", "key3"}
	s.Add(expected...)

	members = s.Members()
	if len(members) != len(expected) {
		t.Errorf("Members() 返回元素数量 %d, 期望 %d", len(members), len(expected))
	}

	// 排序后比较，因为集合没有顺序
	sort.Strings(members)
	sort.Strings(expected)
	if !reflect.DeepEqual(members, expected) {
		t.Errorf("Members() = %v, 期望 %v", members, expected)
	}
}

// TestSetIsMember 测试判断元素是否在集合中
func TestSetIsMember(t *testing.T) {
	s := NewSet()

	// 空集合
	if s.IsMember("key1") {
		t.Errorf("空集合中 IsMember(\"key1\") 返回 true, 期望 false")
	}

	// 添加元素后
	s.Add("key1", "key2")

	// 存在的元素
	if !s.IsMember("key1") {
		t.Errorf("IsMember(\"key1\") 返回 false, 期望 true")
	}

	// 不存在的元素
	if s.IsMember("key3") {
		t.Errorf("IsMember(\"key3\") 返回 true, 期望 false")
	}

	// 空字符串
	if s.IsMember("") {
		t.Errorf("IsMember(\"\") 返回 true, 期望 false")
	}

	// 添加空字符串
	s.Add("")
	if !s.IsMember("") {
		t.Errorf("添加空字符串后 IsMember(\"\") 返回 false, 期望 true")
	}
}

// TestSetRemove 测试从集合中移除元素
func TestSetRemove(t *testing.T) {
	s := NewSet()
	s.Add("key1", "key2", "key3", "key4")

	// 测试移除单个元素
	removed := s.Remove("key1")
	if removed != 1 {
		t.Errorf("Remove(\"key1\") 返回 %d, 期望 1", removed)
	}
	if len(s.members) != 3 {
		t.Errorf("移除单个元素后集合大小 = %d, 期望 3", len(s.members))
	}
	if s.IsMember("key1") {
		t.Errorf("移除元素后 IsMember(\"key1\") 返回 true, 期望 false")
	}

	// 测试移除多个元素
	removed = s.Remove("key2", "key3")
	if removed != 2 {
		t.Errorf("Remove(\"key2\", \"key3\") 返回 %d, 期望 2", removed)
	}
	if len(s.members) != 1 {
		t.Errorf("移除两个元素后集合大小 = %d, 期望 1", len(s.members))
	}

	// 测试移除不存在的元素
	removed = s.Remove("key5")
	if removed != 0 {
		t.Errorf("Remove(\"key5\") 返回 %d, 期望 0", removed)
	}
	if len(s.members) != 1 {
		t.Errorf("移除不存在元素后集合大小 = %d, 期望仍然是 1", len(s.members))
	}

	// 测试移除所有元素
	removed = s.Remove("key4")
	if removed != 1 {
		t.Errorf("Remove(\"key4\") 返回 %d, 期望 1", removed)
	}
	if len(s.members) != 0 {
		t.Errorf("移除所有元素后集合大小 = %d, 期望 0", len(s.members))
	}
}

// TestSetPop 测试随机弹出集合元素
func TestSetPop(t *testing.T) {
	s := NewSet()

	// 空集合
	member, ok := s.Pop()
	if ok {
		t.Errorf("空集合 Pop() 返回 ok=true 和元素 %s, 期望 ok=false", member)
	}

	// 单元素集合
	s.Add("key1")
	member, ok = s.Pop()
	if !ok {
		t.Errorf("Pop() 返回 ok=false, 期望 ok=true")
	}
	if member != "key1" {
		t.Errorf("Pop() 返回元素 %s, 期望 key1", member)
	}
	if len(s.members) != 0 {
		t.Errorf("Pop() 后集合大小 = %d, 期望 0", len(s.members))
	}

	// 多元素集合
	s.Add("key1", "key2", "key3")

	// 弹出全部元素
	validMembers := map[string]bool{"key1": true, "key2": true, "key3": true}
	for i := 0; i < 3; i++ {
		member, ok = s.Pop()
		if !ok {
			t.Errorf("第 %d 次 Pop() 返回 ok=false, 期望 ok=true", i+1)
		}
		if !validMembers[member] {
			t.Errorf("Pop() 返回无效元素 %s", member)
		}
		delete(validMembers, member)
	}

	// 弹出所有元素后
	member, ok = s.Pop()
	if ok {
		t.Errorf("弹出所有元素后 Pop() 返回 ok=true 和元素 %s, 期望 ok=false", member)
	}
}

// TestSetDiff 测试集合差集
func TestSetDiff(t *testing.T) {
	s1 := NewSet()
	s1.Add("key1", "key2", "key3", "key4")

	s2 := NewSet()
	s2.Add("key3", "key4", "key5")

	s3 := NewSet()
	s3.Add("key1", "key6")

	// 测试与空集合的差集
	diff := s1.Diff()
	sort.Strings(diff)
	expected := []string{"key1", "key2", "key3", "key4"}
	sort.Strings(expected)
	if !reflect.DeepEqual(diff, expected) {
		t.Errorf("Diff() = %v, 期望 %v", diff, expected)
	}

	// 测试与一个集合的差集
	diff = s1.Diff(s2)
	sort.Strings(diff)
	expected = []string{"key1", "key2"}
	sort.Strings(expected)
	if !reflect.DeepEqual(diff, expected) {
		t.Errorf("Diff(s2) = %v, 期望 %v", diff, expected)
	}

	// 测试与多个集合的差集
	diff = s1.Diff(s2, s3)
	sort.Strings(diff)
	expected = []string{"key2"}
	sort.Strings(expected)
	if !reflect.DeepEqual(diff, expected) {
		t.Errorf("Diff(s2, s3) = %v, 期望 %v", diff, expected)
	}

	// 测试与包含所有元素的集合的差集
	s4 := NewSet()
	s4.Add("key1", "key2", "key3", "key4", "key5", "key6")
	diff = s1.Diff(s4)
	if len(diff) != 0 {
		t.Errorf("Diff(s4) = %v, 期望空集合", diff)
	}
}

// TestSetInter 测试集合交集
func TestSetInter(t *testing.T) {
	s1 := NewSet()
	s1.Add("key1", "key2", "key3", "key4")

	s2 := NewSet()
	s2.Add("key3", "key4", "key5")

	s3 := NewSet()
	s3.Add("key1", "key4", "key6")

	// 测试与空参数的交集
	inter := s1.Inter()
	sort.Strings(inter)
	expected := []string{"key1", "key2", "key3", "key4"}
	sort.Strings(expected)
	if !reflect.DeepEqual(inter, expected) {
		t.Errorf("Inter() = %v, 期望 %v", inter, expected)
	}

	// 测试与一个集合的交集
	inter = s1.Inter(s2)
	sort.Strings(inter)
	expected = []string{"key3", "key4"}
	sort.Strings(expected)
	if !reflect.DeepEqual(inter, expected) {
		t.Errorf("Inter(s2) = %v, 期望 %v", inter, expected)
	}

	// 测试与多个集合的交集
	inter = s1.Inter(s2, s3)
	sort.Strings(inter)
	expected = []string{"key4"}
	sort.Strings(expected)
	if !reflect.DeepEqual(inter, expected) {
		t.Errorf("Inter(s2, s3) = %v, 期望 %v", inter, expected)
	}

	// 测试与无交集集合的交集
	s4 := NewSet()
	s4.Add("key5", "key6", "key7")
	inter = s1.Inter(s4)
	if len(inter) != 0 {
		t.Errorf("Inter(s4) = %v, 期望空集合", inter)
	}
}

// TestSetUnion 测试集合并集
func TestSetUnion(t *testing.T) {
	s1 := NewSet()
	s1.Add("a", "b", "c")

	s2 := NewSet()
	s2.Add("c", "d", "e")

	s3 := NewSet()
	s3.Add("e", "f", "g")

	// 测试两个集合的并集
	union := s1.Union(s2)
	assert.Equal(t, 5, len(union), "Union of two sets should have 5 elements")

	// 由于Union返回的是无序的字符串数组，我们需要检查所有元素是否都存在
	unionSet := make(map[string]bool)
	for _, member := range union {
		unionSet[member] = true
	}
	assert.True(t, unionSet["a"], "Union should contain 'a'")
	assert.True(t, unionSet["b"], "Union should contain 'b'")
	assert.True(t, unionSet["c"], "Union should contain 'c'")
	assert.True(t, unionSet["d"], "Union should contain 'd'")
	assert.True(t, unionSet["e"], "Union should contain 'e'")

	// 测试多个集合的并集
	union = s1.Union(s2, s3)
	assert.Equal(t, 7, len(union), "Union of three sets should have 7 elements")

	unionSet = make(map[string]bool)
	for _, member := range union {
		unionSet[member] = true
	}
	assert.True(t, unionSet["a"], "Union should contain 'a'")
	assert.True(t, unionSet["b"], "Union should contain 'b'")
	assert.True(t, unionSet["c"], "Union should contain 'c'")
	assert.True(t, unionSet["d"], "Union should contain 'd'")
	assert.True(t, unionSet["e"], "Union should contain 'e'")
	assert.True(t, unionSet["f"], "Union should contain 'f'")
	assert.True(t, unionSet["g"], "Union should contain 'g'")

	// 测试空集合的并集
	empty := NewSet()
	union = s1.Union(empty)
	assert.Equal(t, 3, len(union), "Union with empty set should equal original set")

	unionSet = make(map[string]bool)
	for _, member := range union {
		unionSet[member] = true
	}
	assert.True(t, unionSet["a"], "Union should contain 'a'")
	assert.True(t, unionSet["b"], "Union should contain 'b'")
	assert.True(t, unionSet["c"], "Union should contain 'c'")

	// 测试无参数的并集
	union = s1.Union()
	assert.Equal(t, 3, len(union), "Union with no arguments should equal original set")

	unionSet = make(map[string]bool)
	for _, member := range union {
		unionSet[member] = true
	}
	assert.True(t, unionSet["a"], "Union should contain 'a'")
	assert.True(t, unionSet["b"], "Union should contain 'b'")
	assert.True(t, unionSet["c"], "Union should contain 'c'")
}

// TestSetDeepCopy 测试集合深拷贝
func TestSetDeepCopy(t *testing.T) {
	s := NewSet()
	s.Add("key1", "key2", "key3")

	// 测试深拷贝
	copyValue := s.DeepCopy()
	copySet, ok := copyValue.(*Set)
	if !ok {
		t.Fatalf("DeepCopy() 返回类型 %T, 期望 *Set", copyValue)
	}

	// 验证拷贝的内容相同
	if len(copySet.members) != len(s.members) {
		t.Errorf("拷贝后集合大小 = %d, 期望 %d", len(copySet.members), len(s.members))
	}

	for member := range s.members {
		if _, exists := copySet.members[member]; !exists {
			t.Errorf("拷贝后集合缺少元素 %s", member)
		}
	}

	// 验证是深拷贝，修改原集合不影响副本
	s.Add("key4")
	if len(copySet.members) != 3 {
		t.Errorf("修改原集合后，拷贝集合大小变为 = %d, 期望仍然是 3", len(copySet.members))
	}
	if copySet.IsMember("key4") {
		t.Errorf("修改原集合后，拷贝集合包含了新元素，违反深拷贝原则")
	}

	// 修改副本不影响原集合
	copySet.Add("key5")
	if len(s.members) != 4 {
		t.Errorf("修改拷贝集合后，原集合大小变为 = %d, 期望仍然是 4", len(s.members))
	}
	if s.IsMember("key5") {
		t.Errorf("修改拷贝集合后，原集合包含了新元素，违反深拷贝原则")
	}
}

// TestSetEncodeAndSize 测试集合编码和大小计算
func TestSetEncodeAndSize(t *testing.T) {
	s := NewSet()

	// 测试空集合编码
	emptyBytes, err := s.Encode()
	if err != nil {
		t.Fatalf("空集合 Encode() 返回错误: %v", err)
	}
	if len(emptyBytes) <= 0 {
		t.Errorf("空集合 Encode() 返回空字节数组")
	}
	if emptyBytes[0] != byte(cache.TypeSet) {
		t.Errorf("Encode() 首字节 = %d, 期望 %d (TypeSet)", emptyBytes[0], cache.TypeSet)
	}

	// 添加元素后测试编码
	s.Add("key1", "key2", "key3")
	nonEmptyBytes, err := s.Encode()
	if err != nil {
		t.Fatalf("Encode() 返回错误: %v", err)
	}
	if len(nonEmptyBytes) <= len(emptyBytes) {
		t.Errorf("添加元素后 Encode() 返回大小 %d, 应该大于空集合的 %d", len(nonEmptyBytes), len(emptyBytes))
	}

	// 这里可以添加更多编码格式的验证，如果需要的话
}

// TestDecodeSet 测试解码集合
func TestDecodeSet(t *testing.T) {
	original := NewSet()
	original.Add("member1", "member2", "member3")

	encoded, _ := original.Encode()

	decoded, err := DecodeSet(encoded)
	assert.NoError(t, err, "DecodeSet should not return error for valid data")
	assert.Equal(t, int64(3), decoded.Len(), "Decoded Set should have 3 members")

	// 检查集合内容
	assert.True(t, decoded.IsMember("member1"), "Decoded Set should contain 'member1'")
	assert.True(t, decoded.IsMember("member2"), "Decoded Set should contain 'member2'")
	assert.True(t, decoded.IsMember("member3"), "Decoded Set should contain 'member3'")

	// 测试无效数据
	_, err = DecodeSet([]byte{0})
	assert.Error(t, err, "DecodeSet should return error for invalid data")

	// 测试错误类型
	invalidType := []byte{byte(cache.TypeString), 0, 0, 0, 2}
	_, err = DecodeSet(invalidType)
	assert.Error(t, err, "DecodeSet should return error for invalid type")

	// 测试数据长度不足
	invalidLength := []byte{byte(cache.TypeSet), 0, 0, 0, 10, 0, 0, 0, 7, 'm', 'e', 'm', 'b', 'e', 'r', '1'}
	_, err = DecodeSet(invalidLength)
	assert.Error(t, err, "DecodeSet should return error for invalid length")
}

// 我们还应该添加测试用例测试并发安全性，但需要确保测试环境支持并发测试
// 例如使用 t.Parallel() 和 go routines 进行并发操作测试
