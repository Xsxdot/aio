package engine

import (
	"fmt"
	cache2 "github.com/xsxdot/aio/internal/cache"
	"sync"
	"time"
)

// MemoryEngine 内存缓存引擎实现
type MemoryEngine struct {
	// 数据库实例
	databases map[int]*MemoryDatabase
	// 用于保护数据库访问的互斥锁
	mutex sync.RWMutex
	// 配置
	config cache2.Config
	// 启动时间
	startTime time.Time
}

// NewMemoryEngine 创建一个新的内存缓存引擎
func NewMemoryEngine(config cache2.Config) *MemoryEngine {
	// 验证并修复配置
	config = config.ValidateAndFix()

	engine := &MemoryEngine{
		databases: make(map[int]*MemoryDatabase),
		config:    config,
		startTime: time.Now(),
	}

	// 初始化0号数据库
	engine.databases[0] = NewMemoryDatabase(0, &config)

	return engine
}

// Select 选择数据库
func (e *MemoryEngine) Select(dbIndex int) (cache2.Database, error) {
	// 检查数据库索引是否有效
	if dbIndex < 0 || dbIndex >= e.config.DBCount {
		return nil, fmt.Errorf("invalid database index: %d", dbIndex)
	}

	// 获取数据库实例
	e.mutex.RLock()
	db, exists := e.databases[dbIndex]
	e.mutex.RUnlock()

	if exists {
		return db, nil
	}

	// 如果数据库不存在，则创建它
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 再次检查，防止在获取锁的期间被其他协程创建
	db, exists = e.databases[dbIndex]
	if exists {
		return db, nil
	}

	// 创建新的数据库实例
	db = NewMemoryDatabase(dbIndex, &e.config)
	e.databases[dbIndex] = db

	return db, nil
}

// FlushAll 清空所有数据库
func (e *MemoryEngine) FlushAll() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 清空所有已初始化的数据库
	for _, db := range e.databases {
		db.Flush()
	}
}

// GetStats 获取引擎统计信息
func (e *MemoryEngine) GetStats() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	stats := make(map[string]interface{})

	// 基本信息
	stats["uptime_in_seconds"] = int64(time.Since(e.startTime).Seconds())
	stats["used_memory"] = e.getUsedMemory()
	stats["db_count"] = len(e.databases)
	stats["total_commands_processed"] = e.getTotalCommands()
	stats["total_connections_received"] = 0 // 由服务层实现
	stats["instantaneous_ops_per_sec"] = 0  // 由服务层实现

	// 数据库信息
	dbStats := make(map[string]interface{})
	for i, db := range e.databases {
		dbKey := fmt.Sprintf("db%d", i)
		dbInfo := make(map[string]interface{})
		dbInfo["keys"] = db.Size()
		// 这里可以添加更多数据库特定的统计信息
		dbStats[dbKey] = dbInfo
	}
	stats["keyspace"] = dbStats

	return stats
}

// Close 关闭引擎
func (e *MemoryEngine) Close() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 关闭所有数据库
	for _, db := range e.databases {
		db.Close()
	}

	// 清空数据库映射
	e.databases = make(map[int]*MemoryDatabase)

	return nil
}

// 内部方法

// getUsedMemory 获取已使用的内存
func (e *MemoryEngine) getUsedMemory() int64 {
	var total int64

	// 遍历所有数据库，累计内存使用
	for _, db := range e.databases {
		// 获取数据库大小作为估算值
		total += db.Size() * 1024 // 假设每个键平均占用1KB
	}

	return total
}

// getTotalCommands 获取总命令数
func (e *MemoryEngine) getTotalCommands() int64 {
	var total int64

	// 遍历所有数据库，累计命令数
	for _, db := range e.databases {
		// 这里需要从数据库中获取命令计数
		db.mutex.RLock()
		total += db.stats.CmdProcessed
		db.mutex.RUnlock()
	}

	return total
}
