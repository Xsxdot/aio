package ds

import (
	"github.com/xsxdot/aio/internal/cache"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewString 测试创建新的字符串值
func TestNewString(t *testing.T) {
	s := NewString("test")
	assert.NotNil(t, s, "NewString should not return nil")
	assert.Equal(t, "test", s.String(), "String value should be initialized correctly")
}

// TestStringType 测试字符串类型
func TestStringType(t *testing.T) {
	s := NewString("test")
	assert.Equal(t, cache.TypeString, s.Type(), "String type should be TypeString")
}

// TestStringString 测试获取字符串值
func TestStringString(t *testing.T) {
	s := NewString("test")
	assert.Equal(t, "test", s.String(), "String() should return the correct value")
}

// TestStringSetString 测试设置字符串值
func TestStringSetString(t *testing.T) {
	s := NewString("test")
	s.SetString("new value")
	assert.Equal(t, "new value", s.String(), "SetString should update the value")
}

// TestStringIncrBy 测试增加整数值
func TestStringIncrBy(t *testing.T) {
	// 测试有效整数
	s := NewString("10")
	val, err := s.IncrBy(5)
	assert.NoError(t, err, "IncrBy should not return error for valid integer")
	assert.Equal(t, int64(15), val, "IncrBy should return the new value")
	assert.Equal(t, "15", s.String(), "IncrBy should update the string value")

	// 测试负数增量
	val, err = s.IncrBy(-7)
	assert.NoError(t, err, "IncrBy should not return error for negative delta")
	assert.Equal(t, int64(8), val, "IncrBy should handle negative delta correctly")
	assert.Equal(t, "8", s.String(), "IncrBy should update the string value")

	// 测试非整数值
	s.SetString("not an integer")
	_, err = s.IncrBy(5)
	assert.Error(t, err, "IncrBy should return error for non-integer value")
}

// TestStringIncrByFloat 测试增加浮点数值
func TestStringIncrByFloat(t *testing.T) {
	// 测试有效浮点数
	s := NewString("10.5")
	val, err := s.IncrByFloat(2.5)
	assert.NoError(t, err, "IncrByFloat should not return error for valid float")
	assert.Equal(t, 13.0, val, "IncrByFloat should return the new value")
	assert.Equal(t, "13", s.String(), "IncrByFloat should update the string value")

	// 测试负数增量
	val, err = s.IncrByFloat(-5.5)
	assert.NoError(t, err, "IncrByFloat should not return error for negative delta")
	assert.Equal(t, 7.5, val, "IncrByFloat should handle negative delta correctly")
	assert.Equal(t, "7.5", s.String(), "IncrByFloat should update the string value")

	// 测试非浮点数值
	s.SetString("not a float")
	_, err = s.IncrByFloat(5.5)
	assert.Error(t, err, "IncrByFloat should return error for non-float value")
}

// TestStringAppend 测试追加字符串
func TestStringAppend(t *testing.T) {
	s := NewString("hello")
	length := s.Append(" world")
	assert.Equal(t, 11, length, "Append should return the new length")
	assert.Equal(t, "hello world", s.String(), "Append should update the string value")

	// 测试追加空字符串
	length = s.Append("")
	assert.Equal(t, 11, length, "Append with empty string should return the same length")
	assert.Equal(t, "hello world", s.String(), "Append with empty string should not change the value")
}

// TestStringEncode 测试编码字符串
func TestStringEncode(t *testing.T) {
	s := NewString("test")
	encoded, err := s.Encode()
	assert.NoError(t, err, "Encode should not return error")
	assert.NotNil(t, encoded, "Encoded data should not be nil")
	assert.Equal(t, byte(cache.TypeString), encoded[0], "First byte should be the type")

	// 检查长度字段
	length := int(encoded[1])<<24 | int(encoded[2])<<16 | int(encoded[3])<<8 | int(encoded[4])
	assert.Equal(t, 4, length, "Length field should be correct")

	// 检查内容
	assert.Equal(t, "test", string(encoded[5:]), "Content should be correct")
}

// TestStringDecode 测试解码字符串
func TestStringDecode(t *testing.T) {
	original := NewString("test")
	encoded, _ := original.Encode()

	decoded, err := DecodeString(encoded)
	assert.NoError(t, err, "DecodeString should not return error for valid data")
	assert.Equal(t, "test", decoded.String(), "Decoded string should match original")

	// 测试无效数据
	_, err = DecodeString([]byte{0})
	assert.Error(t, err, "DecodeString should return error for invalid data")

	// 测试错误类型
	invalidType := []byte{byte(cache.TypeHash), 0, 0, 0, 4, 't', 'e', 's', 't'}
	_, err = DecodeString(invalidType)
	assert.Error(t, err, "DecodeString should return error for invalid type")

	// 测试长度不足
	invalidLength := []byte{byte(cache.TypeString), 0, 0, 0, 10, 't', 'e', 's', 't'}
	_, err = DecodeString(invalidLength)
	assert.Error(t, err, "DecodeString should return error for invalid length")
}

// TestStringSize 测试字符串大小计算
func TestStringSize(t *testing.T) {
	s := NewString("test")
	size := s.Size()
	assert.Greater(t, size, int64(0), "Size should be greater than 0")

	// 测试空字符串
	empty := NewString("")
	emptySize := empty.Size()
	assert.Greater(t, emptySize, int64(0), "Size of empty string should be greater than 0")
	assert.Less(t, emptySize, size, "Size of empty string should be less than non-empty string")

	// 测试长字符串
	long := NewString(string(make([]byte, 1000)))
	longSize := long.Size()
	assert.Greater(t, longSize, size, "Size of long string should be greater than short string")
}

// TestStringDeepCopy 测试深度拷贝
func TestStringDeepCopy(t *testing.T) {
	original := NewString("test")
	copy := original.DeepCopy()

	// 检查值是否相同
	assert.Equal(t, original.String(), copy.(*String).String(), "DeepCopy should have the same value")

	// 修改原始值，确认副本不受影响
	original.SetString("modified")
	assert.NotEqual(t, original.String(), copy.(*String).String(), "Modifying original should not affect copy")
	assert.Equal(t, "test", copy.(*String).String(), "Copy should retain original value")
}
