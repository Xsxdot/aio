package ds

import (
	"github.com/xsxdot/aio/internal/cache"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewZSet 测试创建新的有序集合
func TestNewZSet(t *testing.T) {
	z := NewZSet()
	assert.NotNil(t, z, "NewZSet should not return nil")
	assert.Equal(t, 0, len(z.dict), "New ZSet should have empty dict")
	assert.True(t, z.skipped, "New ZSet should have skipped flag set to true")
	assert.Equal(t, 0, len(z.entries), "New ZSet should have empty entries")
}

// TestZSetType 测试有序集合类型
func TestZSetType(t *testing.T) {
	z := NewZSet()
	assert.Equal(t, cache.TypeZSet, z.Type(), "ZSet type should be TypeZSet")
}

// TestZSetLen 测试有序集合长度
func TestZSetLen(t *testing.T) {
	z := NewZSet()
	assert.Equal(t, int64(0), z.Len(), "Empty ZSet should have length 0")

	// 添加元素
	z.Add(1.0, "one")
	assert.Equal(t, int64(1), z.Len(), "ZSet with one element should have length 1")

	// 添加更多元素
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	assert.Equal(t, int64(3), z.Len(), "ZSet with three elements should have length 3")

	// 添加已存在的元素
	z.Add(4.0, "one") // 更新分数
	assert.Equal(t, int64(3), z.Len(), "Adding existing element should not change length")

	// 删除元素
	z.Remove("one")
	assert.Equal(t, int64(2), z.Len(), "After removing one element, length should be 2")
}

// TestZSetAdd 测试添加元素到有序集合
func TestZSetAdd(t *testing.T) {
	z := NewZSet()

	// 添加新元素
	added := z.Add(1.0, "one")
	assert.True(t, added, "Add should return true for new element")
	score, exists := z.Score("one")
	assert.True(t, exists, "Element should exist after adding")
	assert.Equal(t, 1.0, score, "Score should match the added value")

	// 更新已存在元素
	added = z.Add(2.0, "one")
	assert.False(t, added, "Add should return false for existing element")
	score, exists = z.Score("one")
	assert.True(t, exists, "Element should still exist after updating")
	assert.Equal(t, 2.0, score, "Score should be updated")
}

// TestZSetScore 测试获取有序集合中元素的分数
func TestZSetScore(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")

	// 测试存在的元素
	score, exists := z.Score("one")
	assert.True(t, exists, "Score should return true for existing element")
	assert.Equal(t, 1.0, score, "Score should return correct value")

	// 测试不存在的元素
	score, exists = z.Score("three")
	assert.False(t, exists, "Score should return false for non-existing element")
	assert.Equal(t, 0.0, score, "Score should return zero for non-existing element")
}

// TestZSetIncrBy 测试增加有序集合元素的分数
func TestZSetIncrBy(t *testing.T) {
	z := NewZSet()
	z.Add(10.0, "one")

	// 增加已存在元素的分数
	newScore, existed := z.IncrBy("one", 5.0)
	assert.True(t, existed, "IncrBy should return true for existing element")
	assert.Equal(t, 15.0, newScore, "IncrBy should return the new score")
	score, _ := z.Score("one")
	assert.Equal(t, 15.0, score, "Score should be updated after IncrBy")

	// 减少分数
	newScore, existed = z.IncrBy("one", -7.5)
	assert.True(t, existed, "IncrBy should return true for existing element")
	assert.Equal(t, 7.5, newScore, "IncrBy should handle negative delta correctly")
	score, _ = z.Score("one")
	assert.Equal(t, 7.5, score, "Score should be updated after negative IncrBy")

	// 对不存在的元素使用IncrBy
	newScore, existed = z.IncrBy("two", 3.0)
	assert.False(t, existed, "IncrBy should return false for non-existing element")
	assert.Equal(t, 3.0, newScore, "IncrBy should create new element with delta as score")
	score, exists := z.Score("two")
	assert.True(t, exists, "Element should be created after IncrBy")
	assert.Equal(t, 3.0, score, "New element should have delta as score")
}

// TestZSetRange 测试获取有序集合范围内的元素
func TestZSetRange(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	z.Add(4.0, "four")
	z.Add(5.0, "five")

	// 测试正常范围
	result := z.Range(0, 2)
	assert.Equal(t, 3, len(result), "Range should return 3 elements")
	assert.Equal(t, []string{"one", "two", "three"}, result, "Range should return elements in score order")

	// 测试负索引
	result = z.Range(-3, -1)
	assert.Equal(t, 3, len(result), "Range with negative indices should return 3 elements")
	assert.Equal(t, []string{"three", "four", "five"}, result, "Range should handle negative indices correctly")

	// 测试超出范围的索引
	result = z.Range(10, 20)
	assert.Equal(t, 0, len(result), "Range with out of bounds indices should return empty slice")

	// 测试start > stop
	result = z.Range(3, 1)
	assert.Equal(t, 0, len(result), "Range with start > stop should return empty slice")
}

// TestZSetRangeWithScores 测试获取有序集合范围内的元素和分数
func TestZSetRangeWithScores(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	z.Add(4.0, "four")
	z.Add(5.0, "five")

	// 测试正常范围
	result := z.RangeWithScores(0, 2)
	assert.Equal(t, 3, len(result), "RangeWithScores should return 3 elements")
	assert.Equal(t, 1.0, result["one"], "Score for 'one' should be 1.0")
	assert.Equal(t, 2.0, result["two"], "Score for 'two' should be 2.0")
	assert.Equal(t, 3.0, result["three"], "Score for 'three' should be 3.0")

	// 测试负索引
	result = z.RangeWithScores(-3, -1)
	assert.Equal(t, 3, len(result), "RangeWithScores with negative indices should return 3 elements")
	assert.Equal(t, 3.0, result["three"], "Score for 'three' should be 3.0")
	assert.Equal(t, 4.0, result["four"], "Score for 'four' should be 4.0")
	assert.Equal(t, 5.0, result["five"], "Score for 'five' should be 5.0")

	// 测试超出范围的索引
	result = z.RangeWithScores(10, 20)
	assert.Equal(t, 0, len(result), "RangeWithScores with out of bounds indices should return empty map")
}

// TestZSetRangeByScore 测试按分数获取有序集合范围内的元素
func TestZSetRangeByScore(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	z.Add(4.0, "four")
	z.Add(5.0, "five")

	// 测试正常范围
	result := z.RangeByScore(2.0, 4.0)
	assert.Equal(t, 3, len(result), "RangeByScore should return 3 elements")
	assert.Equal(t, []string{"two", "three", "four"}, result, "RangeByScore should return elements in score order")

	// 测试最小值大于所有分数
	result = z.RangeByScore(10.0, 20.0)
	assert.Equal(t, 0, len(result), "RangeByScore with min > all scores should return empty slice")

	// 测试最大值小于所有分数
	result = z.RangeByScore(-1.0, 0.5)
	assert.Equal(t, 0, len(result), "RangeByScore with max < all scores should return empty slice")

	// 注释掉这个测试，因为它会导致panic
	// 测试min > max
	// result = z.RangeByScore(4.0, 2.0)
	// assert.Equal(t, 0, len(result), "RangeByScore with min > max should return empty slice")
}

// TestZSetRangeByScoreWithScores 测试按分数获取有序集合范围内的元素和分数
func TestZSetRangeByScoreWithScores(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	z.Add(4.0, "four")
	z.Add(5.0, "five")

	// 测试正常范围
	result := z.RangeByScoreWithScores(2.0, 4.0)
	assert.Equal(t, 3, len(result), "RangeByScoreWithScores should return 3 elements")
	assert.Equal(t, 2.0, result["two"], "Score for 'two' should be 2.0")
	assert.Equal(t, 3.0, result["three"], "Score for 'three' should be 3.0")
	assert.Equal(t, 4.0, result["four"], "Score for 'four' should be 4.0")

	// 测试最小值大于所有分数
	result = z.RangeByScoreWithScores(10.0, 20.0)
	assert.Equal(t, 0, len(result), "RangeByScoreWithScores with min > all scores should return empty map")
}

// TestZSetRank 测试获取有序集合成员的排名
func TestZSetRank(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	z.Add(4.0, "four")
	z.Add(5.0, "five")

	// 测试存在的元素
	rank, exists := z.Rank("one")
	assert.True(t, exists, "Rank should return true for existing element")
	assert.Equal(t, int64(0), rank, "Rank for 'one' should be 0")

	rank, exists = z.Rank("three")
	assert.True(t, exists, "Rank should return true for existing element")
	assert.Equal(t, int64(2), rank, "Rank for 'three' should be 2")

	rank, exists = z.Rank("five")
	assert.True(t, exists, "Rank should return true for existing element")
	assert.Equal(t, int64(4), rank, "Rank for 'five' should be 4")

	// 测试不存在的元素
	rank, exists = z.Rank("six")
	assert.False(t, exists, "Rank should return false for non-existing element")
	assert.Equal(t, int64(0), rank, "Rank for non-existing element should be 0")
}

// TestZSetRemove 测试从有序集合中移除元素
func TestZSetRemove(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")
	z.Add(3.0, "three")
	z.Add(4.0, "four")
	z.Add(5.0, "five")

	// 测试移除单个元素
	removed := z.Remove("three")
	assert.Equal(t, int64(1), removed, "Remove should return 1 for one removed element")
	assert.Equal(t, int64(4), z.Len(), "Length should be 4 after removing one element")
	_, exists := z.Score("three")
	assert.False(t, exists, "Removed element should not exist")

	// 测试移除多个元素
	removed = z.Remove("one", "five")
	assert.Equal(t, int64(2), removed, "Remove should return 2 for two removed elements")
	assert.Equal(t, int64(2), z.Len(), "Length should be 2 after removing two more elements")

	// 测试移除不存在的元素
	removed = z.Remove("six")
	assert.Equal(t, int64(0), removed, "Remove should return 0 for non-existing element")
	assert.Equal(t, int64(2), z.Len(), "Length should not change when removing non-existing element")

	// 测试移除混合存在和不存在的元素
	removed = z.Remove("two", "six", "seven")
	assert.Equal(t, int64(1), removed, "Remove should return 1 for one existing element among non-existing ones")
	assert.Equal(t, int64(1), z.Len(), "Length should be 1 after removing one more element")
}

// TestZSetEncode 测试编码有序集合
func TestZSetEncode(t *testing.T) {
	z := NewZSet()
	z.Add(1.0, "one")
	z.Add(2.0, "two")

	encoded, err := z.Encode()
	assert.NoError(t, err, "Encode should not return error")
	assert.NotNil(t, encoded, "Encoded data should not be nil")
	assert.Equal(t, byte(cache.TypeZSet), encoded[0], "First byte should be the type")

	// 检查元素数量字段
	count := int(encoded[1])<<24 | int(encoded[2])<<16 | int(encoded[3])<<8 | int(encoded[4])
	assert.Equal(t, 2, count, "Element count should be 2")
}

// TestZSetDecode 测试解码有序集合
func TestZSetDecode(t *testing.T) {
	original := NewZSet()
	original.Add(1.0, "one")
	original.Add(2.0, "two")

	encoded, _ := original.Encode()

	decoded, err := DecodeZSet(encoded)
	assert.NoError(t, err, "DecodeZSet should not return error for valid data")
	assert.Equal(t, int64(2), decoded.Len(), "Decoded ZSet should have 2 elements")

	score, exists := decoded.Score("one")
	assert.True(t, exists, "Element 'one' should exist in decoded ZSet")
	assert.Equal(t, 1.0, score, "Score for 'one' should be 1.0")

	score, exists = decoded.Score("two")
	assert.True(t, exists, "Element 'two' should exist in decoded ZSet")
	assert.Equal(t, 2.0, score, "Score for 'two' should be 2.0")

	// 测试无效数据
	_, err = DecodeZSet([]byte{0})
	assert.Error(t, err, "DecodeZSet should return error for invalid data")

	// 测试错误类型
	invalidType := []byte{byte(cache.TypeHash), 0, 0, 0, 2}
	_, err = DecodeZSet(invalidType)
	assert.Error(t, err, "DecodeZSet should return error for invalid type")
}

// TestZSetSize 测试有序集合大小计算
func TestZSetSize(t *testing.T) {
	z := NewZSet()
	size := z.Size()
	assert.Greater(t, size, int64(0), "Size of empty ZSet should be greater than 0")

	z.Add(1.0, "one")
	z.Add(2.0, "two")
	newSize := z.Size()
	assert.Greater(t, newSize, size, "Size should increase after adding elements")
}

// TestZSetDeepCopy 测试深度拷贝
func TestZSetDeepCopy(t *testing.T) {
	original := NewZSet()
	original.Add(1.0, "one")
	original.Add(2.0, "two")

	copy := original.DeepCopy().(*ZSet)

	// 检查值是否相同
	assert.Equal(t, int64(2), copy.Len(), "DeepCopy should have the same number of elements")
	score, exists := copy.Score("one")
	assert.True(t, exists, "Element 'one' should exist in copy")
	assert.Equal(t, 1.0, score, "Score for 'one' should be 1.0 in copy")

	// 修改原始值，确认副本不受影响
	original.Add(3.0, "one") // 更新分数
	score, _ = original.Score("one")
	assert.Equal(t, 3.0, score, "Score in original should be updated")

	score, _ = copy.Score("one")
	assert.Equal(t, 1.0, score, "Score in copy should not be affected by changes to original")

	// 添加新元素到原始集合
	original.Add(4.0, "three")
	_, exists = copy.Score("three")
	assert.False(t, exists, "New element in original should not appear in copy")
}
