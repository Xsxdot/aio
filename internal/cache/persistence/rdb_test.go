package persistence

import (
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 模拟数据库访问器
type mockDBAccessor struct {
	data    map[string]cache2.Value
	expires map[string]time.Time
	mutex   sync.RWMutex
}

func newMockDBAccessor() *mockDBAccessor {
	return &mockDBAccessor{
		data:    make(map[string]cache2.Value),
		expires: make(map[string]time.Time),
	}
}

func (m *mockDBAccessor) GetAllData() (map[string]cache2.Value, map[string]time.Time) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建数据副本
	dataCopy := make(map[string]cache2.Value, len(m.data))
	expiresCopy := make(map[string]time.Time, len(m.expires))

	for k, v := range m.data {
		dataCopy[k] = v.DeepCopy()
	}

	for k, v := range m.expires {
		expiresCopy[k] = v
	}

	return dataCopy, expiresCopy
}

func (m *mockDBAccessor) LoadData(data map[string]cache2.Value, expires map[string]time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 清空当前数据
	m.data = make(map[string]cache2.Value, len(data))
	m.expires = make(map[string]time.Time, len(expires))

	// 加载新数据
	for k, v := range data {
		m.data[k] = v
	}

	// 加载过期时间
	for k, v := range expires {
		m.expires[k] = v
	}

	return nil
}

// 准备测试环境
func setupRDBTest(t *testing.T) (*RDBManager, *mockDBAccessor, string) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "rdb_test")
	require.NoError(t, err)

	// 创建配置
	config := &cache2.Config{
		EnableRDB:       true,
		RDBFilePath:     filepath.Join(tempDir, "dump.rdb"),
		RDBSaveInterval: 1, // 1秒保存一次，方便测试
	}

	// 创建模拟数据库访问器
	accessor := newMockDBAccessor()

	// 创建RDB管理器
	wg := &sync.WaitGroup{}
	rdbManager := NewRDBManager(config, accessor, wg)

	return rdbManager, accessor, tempDir
}

// 清理测试环境
func cleanupRDBTest(tempDir string) {
	os.RemoveAll(tempDir)
}

// 填充测试数据
func fillTestData(accessor *mockDBAccessor) {
	accessor.mutex.Lock()
	defer accessor.mutex.Unlock()

	// 添加字符串值
	accessor.data["string1"] = ds2.NewString("value1")
	accessor.data["string2"] = ds2.NewString("value2")

	// 添加列表值
	list := ds2.NewList()
	list.RPush("item1", "item2", "item3")
	accessor.data["list1"] = list

	// 添加哈希值
	hash := ds2.NewHash()
	hash.Set("field1", "value1")
	hash.Set("field2", "value2")
	accessor.data["hash1"] = hash

	// 添加集合值
	set := ds2.NewSet()
	set.Add("member1", "member2", "member3")
	accessor.data["set1"] = set

	// 添加有序集合值
	zset := ds2.NewZSet()
	zset.Add(1.0, "member1")
	zset.Add(2.0, "member2")
	zset.Add(3.0, "member3")
	accessor.data["zset1"] = zset

	// 设置过期时间
	now := time.Now()
	accessor.expires["string1"] = now.Add(10 * time.Second)
	accessor.expires["list1"] = now.Add(20 * time.Second)
	accessor.expires["hash1"] = now.Add(30 * time.Second)
	accessor.expires["set1"] = now.Add(40 * time.Second)
	accessor.expires["zset1"] = now.Add(50 * time.Second)
}

// 测试RDB文件的生成
func TestSaveToRDB(t *testing.T) {
	rdbManager, accessor, tempDir := setupRDBTest(t)
	defer cleanupRDBTest(tempDir)

	// 填充测试数据
	fillTestData(accessor)

	// 保存RDB文件
	err := rdbManager.SaveToRDB()
	require.NoError(t, err)

	// 验证文件是否存在
	_, err = os.Stat(rdbManager.config.RDBFilePath)
	assert.NoError(t, err)

	// 验证文件大小是否大于0
	fileInfo, _ := os.Stat(rdbManager.config.RDBFilePath)
	assert.Greater(t, fileInfo.Size(), int64(0))
}

// 测试从RDB文件加载数据
func TestLoadFromRDB(t *testing.T) {
	rdbManager, accessor, tempDir := setupRDBTest(t)
	defer cleanupRDBTest(tempDir)

	// 填充测试数据
	fillTestData(accessor)

	// 保存RDB文件
	err := rdbManager.SaveToRDB()
	require.NoError(t, err)

	// 调试打印文件内容
	t.Log("RDB file content after saving:")
	printRDBFileContent(t, rdbManager.config.RDBFilePath)

	// 清空数据
	accessor.mutex.Lock()
	accessor.data = make(map[string]cache2.Value)
	accessor.expires = make(map[string]time.Time)
	accessor.mutex.Unlock()

	// 从RDB文件加载数据
	err = rdbManager.LoadFromRDB()
	if err != nil {
		t.Logf("Error loading from RDB: %v", err)
		printRDBFileContent(t, rdbManager.config.RDBFilePath)
		t.FailNow()
	}

	// 验证数据是否正确加载
	data, expires := accessor.GetAllData()

	// 验证键数量
	assert.Equal(t, 6, len(data))
	assert.Equal(t, 5, len(expires))

	// 验证字符串值
	strVal, ok := data["string1"]
	assert.True(t, ok)
	assert.Equal(t, cache2.TypeString, strVal.Type())
	assert.Equal(t, "value1", strVal.(cache2.StringValue).String())

	// 验证列表值
	listVal, ok := data["list1"]
	assert.True(t, ok)
	assert.Equal(t, cache2.TypeList, listVal.Type())
	assert.Equal(t, int64(3), listVal.(cache2.ListValue).Len())

	// 验证哈希值
	hashVal, ok := data["hash1"]
	assert.True(t, ok)
	assert.Equal(t, cache2.TypeHash, hashVal.Type())
	assert.Equal(t, int64(2), hashVal.(cache2.HashValue).Len())

	// 验证集合值
	setVal, ok := data["set1"]
	assert.True(t, ok)
	assert.Equal(t, cache2.TypeSet, setVal.Type())
	assert.Equal(t, int64(3), setVal.(cache2.SetValue).Len())

	// 验证有序集合值
	zsetVal, ok := data["zset1"]
	assert.True(t, ok)
	assert.Equal(t, cache2.TypeZSet, zsetVal.Type())
	assert.Equal(t, int64(3), zsetVal.(cache2.ZSetValue).Len())

	// 验证过期时间
	_, ok = expires["string1"]
	assert.True(t, ok)
	_, ok = expires["list1"]
	assert.True(t, ok)
}

// 测试不同保存间隔的影响
func TestPeriodicSave(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "rdb_test")
	require.NoError(t, err)
	defer cleanupRDBTest(tempDir)

	// 创建配置，设置较短的保存间隔
	config := &cache2.Config{
		EnableRDB:       true,
		RDBFilePath:     filepath.Join(tempDir, "dump.rdb"),
		RDBSaveInterval: 1, // 1秒保存一次
	}

	// 创建模拟数据库访问器
	accessor := newMockDBAccessor()

	// 创建RDB管理器
	wg := &sync.WaitGroup{}
	rdbManager := NewRDBManager(config, accessor, wg)

	// 填充测试数据
	fillTestData(accessor)

	// 启动定期保存
	rdbManager.StartPeriodicSave()

	// 等待足够的时间让定期保存执行
	time.Sleep(2 * time.Second)

	// 验证文件是否存在
	_, err = os.Stat(rdbManager.config.RDBFilePath)
	assert.NoError(t, err)

	// 关闭RDB管理器
	rdbManager.Shutdown()
	wg.Wait()

	// 修改配置，设置较长的保存间隔
	config.RDBSaveInterval = 60 // 60秒保存一次

	// 删除现有RDB文件
	os.Remove(config.RDBFilePath)

	// 创建新的RDB管理器
	rdbManager = NewRDBManager(config, accessor, wg)

	// 启动定期保存
	rdbManager.StartPeriodicSave()

	// 等待短时间，此时不应该生成RDB文件
	time.Sleep(2 * time.Second)

	// 验证文件是否不存在
	_, err = os.Stat(rdbManager.config.RDBFilePath)
	assert.True(t, os.IsNotExist(err))

	// 关闭RDB管理器
	rdbManager.Shutdown()
	wg.Wait()
}

// 测试文件损坏时的恢复机制
func TestCorruptedRDBFile(t *testing.T) {
	rdbManager, accessor, tempDir := setupRDBTest(t)
	defer cleanupRDBTest(tempDir)

	// 填充测试数据
	fillTestData(accessor)

	// 保存RDB文件
	err := rdbManager.SaveToRDB()
	require.NoError(t, err)

	// 清空数据
	accessor.mutex.Lock()
	accessor.data = make(map[string]cache2.Value)
	accessor.expires = make(map[string]time.Time)
	accessor.mutex.Unlock()

	// 损坏RDB文件（完全重写文件而不是追加）
	err = os.WriteFile(rdbManager.config.RDBFilePath, []byte("CORRUPTED_DATA"), 0644)
	require.NoError(t, err)

	// 尝试从损坏的RDB文件加载数据
	err = rdbManager.LoadFromRDB()
	assert.Error(t, err) // 应该返回错误

	// 验证数据是否为空（加载失败不应影响现有数据）
	data, _ := accessor.GetAllData()
	assert.Equal(t, 0, len(data))

	// 创建新的有效RDB文件
	fillTestData(accessor)
	err = rdbManager.SaveToRDB()
	require.NoError(t, err)

	// 清空数据
	accessor.mutex.Lock()
	accessor.data = make(map[string]cache2.Value)
	accessor.expires = make(map[string]time.Time)
	accessor.mutex.Unlock()

	// 从新的有效RDB文件加载数据
	err = rdbManager.LoadFromRDB()
	require.NoError(t, err)

	// 验证数据是否正确加载
	data, _ = accessor.GetAllData()
	assert.Equal(t, 6, len(data))
}

// 测试RDB文件不存在的情况
func TestRDBFileNotExists(t *testing.T) {
	rdbManager, accessor, tempDir := setupRDBTest(t)
	defer cleanupRDBTest(tempDir)

	// 确保RDB文件不存在
	os.Remove(rdbManager.config.RDBFilePath)

	// 尝试从不存在的RDB文件加载数据
	err := rdbManager.LoadFromRDB()
	assert.NoError(t, err) // 不应该返回错误，而是静默处理

	// 验证数据是否为空
	data, _ := accessor.GetAllData()
	assert.Equal(t, 0, len(data))
}

// 测试禁用RDB的情况
func TestDisabledRDB(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "rdb_test")
	require.NoError(t, err)
	defer cleanupRDBTest(tempDir)

	// 创建配置，禁用RDB
	config := &cache2.Config{
		EnableRDB:       false,
		RDBFilePath:     filepath.Join(tempDir, "dump.rdb"),
		RDBSaveInterval: 1,
	}

	// 创建模拟数据库访问器
	accessor := newMockDBAccessor()

	// 创建RDB管理器
	wg := &sync.WaitGroup{}
	rdbManager := NewRDBManager(config, accessor, wg)

	// 填充测试数据
	fillTestData(accessor)

	// 尝试保存RDB文件
	err = rdbManager.SaveToRDB()
	assert.NoError(t, err) // 不应该返回错误，而是静默处理

	// 验证文件是否不存在
	_, err = os.Stat(rdbManager.config.RDBFilePath)
	assert.True(t, os.IsNotExist(err))
}

// 打印RDB文件内容用于调试
func printRDBFileContent(t *testing.T, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Logf("Error reading RDB file: %v", err)
		return
	}

	t.Logf("RDB file size: %d bytes", len(data))

	if len(data) >= 8 {
		t.Logf("Header bytes: % x", data[:8])
		t.Logf("Header as string: %q", string(data[:8]))
	} else {
		t.Logf("File too short, content: % x", data)
	}

	// 打印完整的文件内容（十六进制）
	if len(data) > 0 {
		t.Logf("Full content: % x", data)
	}
}
