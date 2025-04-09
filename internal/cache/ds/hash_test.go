package ds

import (
	"github.com/xsxdot/aio/internal/cache"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewHash 测试创建新的哈希表
func TestNewHash(t *testing.T) {
	h := NewHash()
	if h == nil {
		t.Fatal("NewHash() 返回 nil")
	}
	if h.fields == nil {
		t.Fatal("NewHash() 创建的哈希表 fields 字段为 nil")
	}
	if len(h.fields) != 0 {
		t.Errorf("NewHash() 创建的哈希表不是空的, 大小为 %d", len(h.fields))
	}
}

// TestHashType 测试哈希表类型
func TestHashType(t *testing.T) {
	h := NewHash()
	if h.Type() != cache.TypeHash {
		t.Errorf("Hash.Type() = %v, 期望 %v", h.Type(), cache.TypeHash)
	}
}

// TestHashLen 测试哈希表长度
func TestHashLen(t *testing.T) {
	h := NewHash()

	// 空哈希表
	if h.Len() != 0 {
		t.Errorf("空哈希表 Len() = %d, 期望 0", h.Len())
	}

	// 添加字段后
	h.Set("field1", "value1")
	h.Set("field2", "value2")
	if h.Len() != 2 {
		t.Errorf("添加2个字段后 Len() = %d, 期望 2", h.Len())
	}

	// 删除字段后
	h.Del("field1")
	if h.Len() != 1 {
		t.Errorf("删除1个字段后 Len() = %d, 期望 1", h.Len())
	}
}

// TestHashGet 测试获取哈希表字段值
func TestHashGet(t *testing.T) {
	h := NewHash()

	// 空哈希表
	val, ok := h.Get("field1")
	if ok {
		t.Errorf("空哈希表 Get(\"field1\") 返回 ok=true 和值 %s, 期望 ok=false", val)
	}

	// 添加字段后
	h.Set("field1", "value1")

	// 获取存在的字段
	val, ok = h.Get("field1")
	if !ok {
		t.Errorf("Get(\"field1\") 返回 ok=false, 期望 ok=true")
	}
	if val != "value1" {
		t.Errorf("Get(\"field1\") = %s, 期望 value1", val)
	}

	// 获取不存在的字段
	val, ok = h.Get("field2")
	if ok {
		t.Errorf("Get(\"field2\") 返回 ok=true 和值 %s, 期望 ok=false", val)
	}
}

// TestHashSet 测试设置哈希表字段
func TestHashSet(t *testing.T) {
	h := NewHash()

	// 设置新字段
	newField := h.Set("field1", "value1")
	if !newField {
		t.Errorf("Set(\"field1\", \"value1\") = false, 期望 true")
	}

	val, _ := h.Get("field1")
	if val != "value1" {
		t.Errorf("设置后 Get(\"field1\") = %s, 期望 value1", val)
	}

	// 更新字段
	newField = h.Set("field1", "newvalue1")
	if newField {
		t.Errorf("Set(\"field1\", \"newvalue1\") = true, 期望 false")
	}

	val, _ = h.Get("field1")
	if val != "newvalue1" {
		t.Errorf("更新后 Get(\"field1\") = %s, 期望 newvalue1", val)
	}
}

// TestHashSetNX 测试哈希表字段不存在时设置
func TestHashSetNX(t *testing.T) {
	h := NewHash()

	// 设置不存在的字段
	set := h.SetNX("field1", "value1")
	if !set {
		t.Errorf("SetNX(\"field1\", \"value1\") = false, 期望 true")
	}

	val, _ := h.Get("field1")
	if val != "value1" {
		t.Errorf("SetNX后 Get(\"field1\") = %s, 期望 value1", val)
	}

	// 设置已存在的字段
	set = h.SetNX("field1", "newvalue1")
	if set {
		t.Errorf("SetNX(\"field1\", \"newvalue1\") = true, 期望 false")
	}

	val, _ = h.Get("field1")
	if val != "value1" {
		t.Errorf("SetNX失败后 Get(\"field1\") = %s, 期望仍然是 value1", val)
	}
}

// TestHashDel 测试删除哈希表字段
func TestHashDel(t *testing.T) {
	h := NewHash()
	h.Set("field1", "value1")
	h.Set("field2", "value2")
	h.Set("field3", "value3")

	// 删除单个字段
	deleted := h.Del("field1")
	if deleted != 1 {
		t.Errorf("Del(\"field1\") = %d, 期望 1", deleted)
	}

	_, ok := h.Get("field1")
	if ok {
		t.Errorf("删除后 Get(\"field1\") 返回 ok=true, 期望 ok=false")
	}

	// 删除多个字段
	deleted = h.Del("field2", "field3", "field4")
	if deleted != 2 {
		t.Errorf("Del(\"field2\", \"field3\", \"field4\") = %d, 期望 2", deleted)
	}

	if h.Len() != 0 {
		t.Errorf("删除所有字段后 Len() = %d, 期望 0", h.Len())
	}
}

// TestHashGetAll 测试获取哈希表所有字段和值
func TestHashGetAll(t *testing.T) {
	h := NewHash()

	// 空哈希表
	all := h.GetAll()
	if len(all) != 0 {
		t.Errorf("空哈希表 GetAll() 返回 %v, 期望空映射", all)
	}

	// 添加字段后
	expected := map[string]string{
		"field1": "value1",
		"field2": "value2",
		"field3": "value3",
	}

	for field, value := range expected {
		h.Set(field, value)
	}

	all = h.GetAll()
	if !reflect.DeepEqual(all, expected) {
		t.Errorf("GetAll() = %v, 期望 %v", all, expected)
	}
}

// TestHashExists 测试哈希表字段是否存在
func TestHashExists(t *testing.T) {
	h := NewHash()

	// 空哈希表
	if h.Exists("field1") {
		t.Errorf("空哈希表 Exists(\"field1\") = true, 期望 false")
	}

	// 添加字段后
	h.Set("field1", "value1")

	// 检查存在的字段
	if !h.Exists("field1") {
		t.Errorf("Exists(\"field1\") = false, 期望 true")
	}

	// 检查不存在的字段
	if h.Exists("field2") {
		t.Errorf("Exists(\"field2\") = true, 期望 false")
	}
}

// TestHashIncrBy 测试哈希表字段整数增加
func TestHashIncrBy(t *testing.T) {
	h := NewHash()

	// 增加不存在的字段
	newVal, err := h.IncrBy("field1", 10)
	if err != nil {
		t.Errorf("IncrBy(\"field1\", 10) 返回错误: %v", err)
	}
	if newVal != 10 {
		t.Errorf("IncrBy(\"field1\", 10) = %d, 期望 10", newVal)
	}

	val, _ := h.Get("field1")
	if val != "10" {
		t.Errorf("IncrBy后 Get(\"field1\") = %s, 期望 \"10\"", val)
	}

	// 增加已存在的字段
	newVal, err = h.IncrBy("field1", 5)
	if err != nil {
		t.Errorf("IncrBy(\"field1\", 5) 返回错误: %v", err)
	}
	if newVal != 15 {
		t.Errorf("IncrBy(\"field1\", 5) = %d, 期望 15", newVal)
	}

	// 尝试增加非整数字段
	h.Set("field2", "notanumber")
	_, err = h.IncrBy("field2", 10)
	if err == nil {
		t.Errorf("IncrBy(\"field2\", 10) 没有返回错误，但期望返回错误")
	}
}

// TestHashIncrByFloat 测试哈希表字段浮点数增加
func TestHashIncrByFloat(t *testing.T) {
	h := NewHash()

	// 增加不存在的字段
	newVal, err := h.IncrByFloat("field1", 10.5)
	if err != nil {
		t.Errorf("IncrByFloat(\"field1\", 10.5) 返回错误: %v", err)
	}
	if newVal != 10.5 {
		t.Errorf("IncrByFloat(\"field1\", 10.5) = %f, 期望 10.5", newVal)
	}

	// 增加已存在的字段
	newVal, err = h.IncrByFloat("field1", 0.5)
	if err != nil {
		t.Errorf("IncrByFloat(\"field1\", 0.5) 返回错误: %v", err)
	}
	if newVal != 11.0 {
		t.Errorf("IncrByFloat(\"field1\", 0.5) = %f, 期望 11.0", newVal)
	}

	// 尝试增加非数字字段
	h.Set("field2", "notanumber")
	_, err = h.IncrByFloat("field2", 10.5)
	if err == nil {
		t.Errorf("IncrByFloat(\"field2\", 10.5) 没有返回错误，但期望返回错误")
	}
}

// TestHashSize 测试哈希表大小计算
func TestHashSize(t *testing.T) {
	h := NewHash()

	// 空哈希表
	emptySize := h.Size()
	if emptySize <= 0 {
		t.Errorf("空哈希表大小计算错误，返回 %d", emptySize)
	}

	// 添加字段后
	h.Set("field1", "value1")
	h.Set("field2", "value2")
	nonEmptySize := h.Size()
	if nonEmptySize <= emptySize {
		t.Errorf("添加字段后哈希表大小计算错误，返回 %d，应该大于 %d", nonEmptySize, emptySize)
	}

	// 添加大值测试
	bigValue := string(make([]byte, 1000))
	h.Set("field3", bigValue)
	biggerSize := h.Size()
	if biggerSize <= nonEmptySize {
		t.Errorf("添加大值后哈希表大小计算错误，返回 %d，应该大于 %d", biggerSize, nonEmptySize)
	}

	expectedIncrease := int64(1000) // 近似值
	if biggerSize-nonEmptySize < expectedIncrease {
		t.Errorf("添加1000字节值后大小增加 %d，应该至少增加 %d", biggerSize-nonEmptySize, expectedIncrease)
	}
}

// TestHashDeepCopy 测试哈希表深拷贝
func TestHashDeepCopy(t *testing.T) {
	h := NewHash()
	h.Set("field1", "value1")
	h.Set("field2", "value2")

	// 测试深拷贝
	copyValue := h.DeepCopy()
	copyHash, ok := copyValue.(*Hash)
	if !ok {
		t.Fatalf("DeepCopy() 返回类型 %T, 期望 *Hash", copyValue)
	}

	// 验证拷贝的内容相同
	originalAll := h.GetAll()
	copyAll := copyHash.GetAll()

	if !reflect.DeepEqual(copyAll, originalAll) {
		t.Errorf("拷贝后哈希表内容 = %v, 期望 %v", copyAll, originalAll)
	}

	// 验证是深拷贝，修改原哈希表不影响副本
	h.Set("field3", "value3")
	copyAll = copyHash.GetAll()
	if _, exists := copyAll["field3"]; exists {
		t.Errorf("修改原哈希表后，拷贝哈希表包含了新字段，违反深拷贝原则")
	}

	// 修改副本不影响原哈希表
	copyHash.Set("field4", "value4")
	originalAll = h.GetAll()
	if _, exists := originalAll["field4"]; exists {
		t.Errorf("修改拷贝哈希表后，原哈希表包含了新字段，违反深拷贝原则")
	}
}

// TestHashEncodeAndSize 测试哈希表编码和大小计算
func TestHashEncodeAndSize(t *testing.T) {
	h := NewHash()

	// 测试空哈希表编码
	emptyBytes, err := h.Encode()
	if err != nil {
		t.Fatalf("空哈希表 Encode() 返回错误: %v", err)
	}
	if len(emptyBytes) <= 0 {
		t.Errorf("空哈希表 Encode() 返回空字节数组")
	}
	if emptyBytes[0] != byte(cache.TypeHash) {
		t.Errorf("Encode() 首字节 = %d, 期望 %d (TypeHash)", emptyBytes[0], cache.TypeHash)
	}

	// 添加字段后测试编码
	h.Set("field1", "value1")
	h.Set("field2", "value2")
	nonEmptyBytes, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode() 返回错误: %v", err)
	}
	if len(nonEmptyBytes) <= len(emptyBytes) {
		t.Errorf("添加字段后 Encode() 返回大小 %d, 应该大于空哈希表的 %d", len(nonEmptyBytes), len(emptyBytes))
	}
}

// TestDecodeHash 测试解码哈希表
func TestDecodeHash(t *testing.T) {
	original := NewHash()
	original.Set("field1", "value1")
	original.Set("field2", "value2")

	encoded, _ := original.Encode()

	decoded, err := DecodeHash(encoded)
	assert.NoError(t, err, "DecodeHash should not return error for valid data")
	assert.Equal(t, int64(2), decoded.Len(), "Decoded Hash should have 2 fields")

	value, exists := decoded.Get("field1")
	assert.True(t, exists, "Field 'field1' should exist in decoded Hash")
	assert.Equal(t, "value1", value, "Value for 'field1' should be 'value1'")

	value, exists = decoded.Get("field2")
	assert.True(t, exists, "Field 'field2' should exist in decoded Hash")
	assert.Equal(t, "value2", value, "Value for 'field2' should be 'value2'")

	// 测试无效数据
	_, err = DecodeHash([]byte{0})
	assert.Error(t, err, "DecodeHash should return error for invalid data")

	// 测试错误类型
	invalidType := []byte{byte(cache.TypeString), 0, 0, 0, 2}
	_, err = DecodeHash(invalidType)
	assert.Error(t, err, "DecodeHash should return error for invalid type")

	// 测试数据长度不足
	invalidLength := []byte{byte(cache.TypeHash), 0, 0, 0, 10, 0, 0, 0, 5, 'f', 'i', 'e', 'l', 'd'}
	_, err = DecodeHash(invalidLength)
	assert.Error(t, err, "DecodeHash should return error for invalid length")
}
