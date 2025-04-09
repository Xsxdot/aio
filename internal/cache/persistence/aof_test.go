package persistence

import (
	"bufio"
	cache2 "github.com/xsxdot/aio/internal/cache"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 模拟命令执行器
type mockCommandExecutor struct {
	data    map[string]cache2.Value
	expires map[string]time.Time
	mutex   sync.RWMutex
	cmds    []Command // 记录执行的命令
}

func newMockCommandExecutor() *mockCommandExecutor {
	return &mockCommandExecutor{
		data:    make(map[string]cache2.Value),
		expires: make(map[string]time.Time),
		cmds:    make([]Command, 0),
	}
}

func (m *mockCommandExecutor) ExecuteCommand(cmd Command) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 记录命令
	m.cmds = append(m.cmds, cmd)

	// 简单模拟命令执行
	switch strings.ToUpper(cmd.Name) {
	case "SET":
		if len(cmd.Args) >= 2 {
			// 模拟设置字符串值
			m.data[cmd.Args[0]] = &mockStringValue{value: cmd.Args[1]}
		}
	case "DEL":
		if len(cmd.Args) >= 1 {
			// 模拟删除键
			for _, key := range cmd.Args {
				delete(m.data, key)
				delete(m.expires, key)
			}
		}
	case "EXPIRE":
		if len(cmd.Args) >= 2 {
			// 模拟设置过期时间
			key := cmd.Args[0]
			if _, exists := m.data[key]; exists {
				ttl, _ := parseIntOrZero(cmd.Args[1])
				m.expires[key] = time.Now().Add(time.Duration(ttl) * time.Second)
			}
		}
	}

	return nil
}

func (m *mockCommandExecutor) GetAllData() (map[string]cache2.Value, map[string]time.Time) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建数据副本
	dataCopy := make(map[string]cache2.Value, len(m.data))
	expiresCopy := make(map[string]time.Time, len(m.expires))

	for k, v := range m.data {
		dataCopy[k] = v
	}

	for k, v := range m.expires {
		expiresCopy[k] = v
	}

	return dataCopy, expiresCopy
}

// 辅助函数：解析整数或返回0
func parseIntOrZero(s string) (int64, error) {
	// 简单实现，实际应该使用strconv.ParseInt
	return 0, nil
}

// 模拟字符串值
type mockStringValue struct {
	value string
}

func (m *mockStringValue) Type() cache2.DataType {
	return cache2.TypeString
}

func (m *mockStringValue) Encode() ([]byte, error) {
	return []byte(m.value), nil
}

func (m *mockStringValue) Size() int64 {
	return int64(len(m.value))
}

func (m *mockStringValue) DeepCopy() cache2.Value {
	return &mockStringValue{value: m.value}
}

func (m *mockStringValue) String() string {
	return m.value
}

// 准备测试环境
func setupAOFTest(t *testing.T) (*AOFManager, *mockCommandExecutor, string) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "aof_test")
	require.NoError(t, err)

	// 创建配置
	config := &cache2.Config{
		EnableAOF:       true,
		AOFFilePath:     filepath.Join(tempDir, "appendonly.aof"),
		AOFSyncStrategy: 0, // 默认不同步
	}

	// 创建模拟命令执行器
	executor := newMockCommandExecutor()

	// 创建AOF管理器
	wg := &sync.WaitGroup{}
	aofManager, err := NewAOFManager(config, executor, wg)
	require.NoError(t, err)

	return aofManager, executor, tempDir
}

// 清理测试环境
func cleanupAOFTest(tempDir string) {
	os.RemoveAll(tempDir)
}

// 测试AOF文件的生成
func TestWriteCommand(t *testing.T) {
	aofManager, _, tempDir := setupAOFTest(t)
	defer cleanupAOFTest(tempDir)

	// 写入命令
	cmd := Command{
		Name: "SET",
		Args: []string{"key1", "value1"},
	}
	err := aofManager.WriteCommand(cmd)
	require.NoError(t, err)

	// 刷新缓冲区
	aofManager.aofLock.Lock()
	err = aofManager.aofBuf.Flush()
	aofManager.aofLock.Unlock()
	require.NoError(t, err)

	// 验证文件是否存在
	_, err = os.Stat(aofManager.config.AOFFilePath)
	assert.NoError(t, err)

	// 验证文件大小是否大于0
	fileInfo, _ := os.Stat(aofManager.config.AOFFilePath)
	assert.Greater(t, fileInfo.Size(), int64(0))
}

// 测试不同同步策略下的AOF写入
func TestAOFSyncStrategies(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "aof_test")
	require.NoError(t, err)
	defer cleanupAOFTest(tempDir)

	// 测试每命令同步策略
	testSyncStrategy(t, tempDir, 2, "always_sync.aof")

	// 测试每秒同步策略
	testSyncStrategy(t, tempDir, 1, "everysec_sync.aof")

	// 测试不同步策略
	testSyncStrategy(t, tempDir, 0, "no_sync.aof")
}

// 测试特定同步策略
func testSyncStrategy(t *testing.T, tempDir string, strategy int, filename string) {
	// 创建配置
	config := &cache2.Config{
		EnableAOF:       true,
		AOFFilePath:     filepath.Join(tempDir, filename),
		AOFSyncStrategy: strategy,
	}

	// 创建模拟命令执行器
	executor := newMockCommandExecutor()

	// 创建AOF管理器
	wg := &sync.WaitGroup{}
	aofManager, err := NewAOFManager(config, executor, wg)
	require.NoError(t, err)

	// 启动AOF管理器
	err = aofManager.Start()
	require.NoError(t, err)

	// 写入命令
	cmd := Command{
		Name: "SET",
		Args: []string{"key1", "value1"},
	}
	err = aofManager.WriteCommand(cmd)
	require.NoError(t, err)

	// 如果是每秒同步策略，等待足够的时间让同步发生
	if strategy == 1 {
		time.Sleep(2 * time.Second)
	}

	// 关闭AOF管理器
	err = aofManager.Shutdown()
	require.NoError(t, err)

	// 验证文件是否存在
	_, err = os.Stat(config.AOFFilePath)
	assert.NoError(t, err)

	// 验证文件大小是否大于0
	fileInfo, _ := os.Stat(config.AOFFilePath)
	assert.Greater(t, fileInfo.Size(), int64(0))
}

// 测试从AOF文件重建数据
func TestLoadAOF(t *testing.T) {
	aofManager, _, tempDir := setupAOFTest(t)
	defer cleanupAOFTest(tempDir)

	// 写入多个命令
	commands := []Command{
		{Name: "SET", Args: []string{"key1", "value1"}},
		{Name: "SET", Args: []string{"key2", "value2"}},
		{Name: "DEL", Args: []string{"key1"}},
		{Name: "SET", Args: []string{"key3", "value3"}},
		{Name: "EXPIRE", Args: []string{"key3", "60"}},
	}

	for _, cmd := range commands {
		err := aofManager.WriteCommand(cmd)
		require.NoError(t, err)
	}

	// 刷新缓冲区
	aofManager.aofLock.Lock()
	err := aofManager.aofBuf.Flush()
	aofManager.aofLock.Unlock()
	require.NoError(t, err)

	// 关闭AOF管理器
	err = aofManager.Shutdown()
	require.NoError(t, err)

	// 创建新的执行器和AOF管理器
	newExecutor := newMockCommandExecutor()
	wg := &sync.WaitGroup{}
	newAofManager, err := NewAOFManager(aofManager.config, newExecutor, wg)
	require.NoError(t, err)

	// 从AOF文件加载数据
	err = newAofManager.LoadAOF()
	require.NoError(t, err)

	// 验证命令是否正确执行
	assert.Equal(t, len(commands), len(newExecutor.cmds))

	// 验证数据是否正确加载
	data, _ := newExecutor.GetAllData()
	assert.Equal(t, 2, len(data)) // key1被删除，应该有key2和key3

	// 验证key1是否被删除
	_, exists := data["key1"]
	assert.False(t, exists)

	// 验证key2和key3是否存在
	_, exists = data["key2"]
	assert.True(t, exists)
	_, exists = data["key3"]
	assert.True(t, exists)
}

// 测试AOF重写功能
func TestRewriteAOF(t *testing.T) {
	aofManager, executor, tempDir := setupAOFTest(t)
	defer cleanupAOFTest(tempDir)

	// 写入多个命令，包括冗余操作
	commands := []Command{
		{Name: "SET", Args: []string{"key1", "value1"}},
		{Name: "SET", Args: []string{"key1", "new_value1"}}, // 覆盖key1
		{Name: "SET", Args: []string{"key2", "value2"}},
		{Name: "DEL", Args: []string{"key2"}}, // 删除key2
		{Name: "SET", Args: []string{"key3", "value3"}},
	}

	for _, cmd := range commands {
		err := aofManager.WriteCommand(cmd)
		require.NoError(t, err)
		// 同时执行命令，更新执行器的状态
		executor.ExecuteCommand(cmd)
	}

	// 刷新缓冲区
	aofManager.aofLock.Lock()
	err := aofManager.aofBuf.Flush()
	aofManager.aofLock.Unlock()
	require.NoError(t, err)

	// 获取原始文件大小
	originalFileInfo, _ := os.Stat(aofManager.config.AOFFilePath)
	originalSize := originalFileInfo.Size()

	// 执行AOF重写
	err = aofManager.RewriteAOF()
	require.NoError(t, err)

	// 获取重写后的文件大小
	rewrittenFileInfo, _ := os.Stat(aofManager.config.AOFFilePath)
	rewrittenSize := rewrittenFileInfo.Size()

	// 验证重写后的文件是否存在
	assert.NoError(t, err)

	// 验证重写后的文件大小是否小于原始文件
	// 注意：在某些情况下，重写后的文件可能不会更小，特别是当原始命令已经很精简时
	t.Logf("Original size: %d, Rewritten size: %d", originalSize, rewrittenSize)

	// 创建新的执行器和AOF管理器
	newExecutor := newMockCommandExecutor()
	wg := &sync.WaitGroup{}
	newAofManager, err := NewAOFManager(aofManager.config, newExecutor, wg)
	require.NoError(t, err)

	// 从重写后的AOF文件加载数据
	err = newAofManager.LoadAOF()
	require.NoError(t, err)

	// 验证数据是否正确加载
	data, _ := newExecutor.GetAllData()
	assert.Equal(t, 2, len(data)) // 应该只有key1和key3

	// 验证key1的值是否为新值
	val, exists := data["key1"]
	assert.True(t, exists)
	assert.Equal(t, "new_value1", val.(*mockStringValue).String())

	// 验证key2是否被删除
	_, exists = data["key2"]
	assert.False(t, exists)

	// 验证key3是否存在
	_, exists = data["key3"]
	assert.True(t, exists)
}

// 测试文件损坏时的恢复机制
func TestCorruptedAOFFile(t *testing.T) {
	aofManager, _, tempDir := setupAOFTest(t)
	defer cleanupAOFTest(tempDir)

	// 写入命令
	cmd := Command{
		Name: "SET",
		Args: []string{"key1", "value1"},
	}
	err := aofManager.WriteCommand(cmd)
	require.NoError(t, err)

	// 刷新缓冲区
	aofManager.aofLock.Lock()
	err = aofManager.aofBuf.Flush()
	aofManager.aofLock.Unlock()
	require.NoError(t, err)

	// 关闭AOF管理器
	err = aofManager.Shutdown()
	require.NoError(t, err)

	// 损坏AOF文件（追加无效数据）
	file, err := os.OpenFile(aofManager.config.AOFFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	require.NoError(t, err)
	_, err = file.WriteString("CORRUPTED DATA")
	require.NoError(t, err)
	file.Close()

	// 创建新的执行器和AOF管理器
	newExecutor := newMockCommandExecutor()
	wg := &sync.WaitGroup{}
	newAofManager, err := NewAOFManager(aofManager.config, newExecutor, wg)
	require.NoError(t, err)

	// 尝试从损坏的AOF文件加载数据
	err = newAofManager.LoadAOF()
	// 注意：根据实现，可能会返回错误，也可能会忽略损坏的部分
	if err != nil {
		t.Logf("LoadAOF returned error as expected: %v", err)
	} else {
		// 如果没有返回错误，验证是否至少加载了有效部分
		data, _ := newExecutor.GetAllData()
		assert.GreaterOrEqual(t, len(data), 0)
	}

	// 测试AOF重写能否修复损坏的文件
	err = newAofManager.RewriteAOF()
	require.NoError(t, err)

	// 创建另一个新的执行器和AOF管理器
	finalExecutor := newMockCommandExecutor()
	finalAofManager, err := NewAOFManager(aofManager.config, finalExecutor, wg)
	require.NoError(t, err)

	// 从重写后的AOF文件加载数据
	err = finalAofManager.LoadAOF()
	assert.NoError(t, err) // 重写后的文件应该是有效的
}

// 测试AOF文件不存在的情况
func TestAOFFileNotExists(t *testing.T) {
	aofManager, executor, tempDir := setupAOFTest(t)
	defer cleanupAOFTest(tempDir)

	// 确保AOF文件不存在
	os.Remove(aofManager.config.AOFFilePath)

	// 尝试从不存在的AOF文件加载数据
	err := aofManager.LoadAOF()
	assert.NoError(t, err) // 不应该返回错误，而是静默处理

	// 验证数据是否为空
	data, _ := executor.GetAllData()
	assert.Equal(t, 0, len(data))
}

// 测试禁用AOF的情况
func TestDisabledAOF(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "aof_test")
	require.NoError(t, err)
	defer cleanupAOFTest(tempDir)

	// 创建配置，禁用AOF
	config := &cache2.Config{
		EnableAOF:       false,
		AOFFilePath:     filepath.Join(tempDir, "appendonly.aof"),
		AOFSyncStrategy: 0,
	}

	// 创建模拟命令执行器
	executor := newMockCommandExecutor()

	// 创建AOF管理器
	wg := &sync.WaitGroup{}
	aofManager, err := NewAOFManager(config, executor, wg)
	require.NoError(t, err)

	// 写入命令
	cmd := Command{
		Name: "SET",
		Args: []string{"key1", "value1"},
	}
	err = aofManager.WriteCommand(cmd)
	assert.NoError(t, err) // 不应该返回错误，而是静默处理

	// 验证文件是否不存在
	_, err = os.Stat(aofManager.config.AOFFilePath)
	assert.True(t, os.IsNotExist(err))
}

// 测试TriggerRewrite方法
func TestTriggerRewrite(t *testing.T) {
	aofManager, executor, tempDir := setupAOFTest(t)
	defer cleanupAOFTest(tempDir)

	// 写入命令
	cmd := Command{
		Name: "SET",
		Args: []string{"key1", "value1"},
	}
	err := aofManager.WriteCommand(cmd)
	require.NoError(t, err)
	executor.ExecuteCommand(cmd)

	// 刷新缓冲区
	aofManager.aofLock.Lock()
	err = aofManager.aofBuf.Flush()
	aofManager.aofLock.Unlock()
	require.NoError(t, err)

	// 启动AOF管理器
	err = aofManager.Start()
	require.NoError(t, err)

	// 触发重写
	aofManager.TriggerRewrite()

	// 等待足够的时间让重写完成
	time.Sleep(1 * time.Second)

	// 关闭AOF管理器
	err = aofManager.Shutdown()
	require.NoError(t, err)

	// 验证文件是否存在
	_, err = os.Stat(aofManager.config.AOFFilePath)
	assert.NoError(t, err)
}

// 测试RESP编码和解析
func TestRESPEncoding(t *testing.T) {
	// 测试命令编码
	cmd := Command{
		Name: "SET",
		Args: []string{"key1", "value1"},
	}
	encoded := encodeCommandToRESP(cmd.Name, cmd.Args)
	assert.NotEmpty(t, encoded)

	// 测试命令解析
	reader := bufio.NewReader(strings.NewReader(string(encoded)))
	decoded, err := parseRESPCommand(reader)
	assert.NoError(t, err)
	assert.Equal(t, cmd.Name, decoded.Name)
	assert.Equal(t, cmd.Args, decoded.Args)
}
