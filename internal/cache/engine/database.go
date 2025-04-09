// Package engine 提供缓存引擎的核心实现
package engine

import (
	"context"
	"fmt"
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"github.com/xsxdot/aio/internal/cache/protocol"
	"github.com/xsxdot/aio/pkg/common"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stats 表示数据库统计信息
type Stats struct {
	KeyCount       int64
	ExpiredKeys    int64
	CmdProcessed   int64
	KeyspaceHits   int64
	KeyspaceMisses int64
}

// MemoryDatabase 表示内存数据库
type MemoryDatabase struct {
	index           int
	data            map[string]cache2.Value
	expiryPolicy    ExpiryPolicy
	mutex           sync.RWMutex
	cmdChan         chan cache2.Command
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	lastSaveTime    time.Time
	config          *cache2.Config
	stats           Stats
	persistence     *PersistenceManager
	commandHandlers map[string]func([]string) cache2.Reply
}

// NewMemoryDatabase 创建一个新的内存数据库实例
func NewMemoryDatabase(index int, config *cache2.Config) *MemoryDatabase {
	ctx, cancel := context.WithCancel(context.Background())

	db := &MemoryDatabase{
		index:           index,
		data:            make(map[string]cache2.Value),
		expiryPolicy:    NewDefaultExpiryPolicy(),
		cmdChan:         make(chan cache2.Command, 1000),
		ctx:             ctx,
		cancel:          cancel,
		lastSaveTime:    time.Now(),
		config:          config,
		commandHandlers: make(map[string]func([]string) cache2.Reply),
	}

	// 启动命令处理协程
	db.wg.Add(1)
	go db.processCommands()

	// 启动过期键清理协程
	db.wg.Add(1)
	go db.cleanExpiredKeys()

	// 只为0号数据库初始化持久化管理器
	if index == 0 {
		persistenceManager, err := NewPersistenceManager(db, &db.wg)
		if err != nil {
			common.GetLogger().Infof("初始化持久化管理器失败: %v", err)
		} else {
			db.persistence = persistenceManager
			db.persistence.Start()
		}
	}

	return db
}

// Get 获取键对应的值
func (db *MemoryDatabase) Get(key string) (cache2.Value, bool) {
	// 检查键是否过期
	if db.expiryPolicy.IsExpired(key) {
		// 异步删除过期键
		go func() {
			db.mutex.Lock()
			defer db.mutex.Unlock()

			// 再次检查，避免竞态条件
			if db.expiryPolicy.IsExpired(key) {
				delete(db.data, key)
				db.expiryPolicy.RemoveExpiry(key)
				db.stats.ExpiredKeys++
			}
		}()
		return nil, false
	}

	db.mutex.RLock()
	defer db.mutex.RUnlock()

	value, exists := db.data[key]
	return value, exists
}

// Set 设置键值对
func (db *MemoryDatabase) Set(key string, value cache2.Value, expiration time.Duration) bool {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.data[key] = value

	// 设置过期时间
	if expiration > 0 {
		db.expiryPolicy.SetExpiry(key, expiration)
	} else if expiration == 0 {
		// 如果过期时间为0，则移除过期时间
		db.expiryPolicy.RemoveExpiry(key)
	}

	return true
}

// Delete 删除键
func (db *MemoryDatabase) Delete(keys ...string) int64 {
	if len(keys) == 0 {
		return 0
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	var count int64
	for _, key := range keys {
		_, exists := db.data[key]
		if exists {
			delete(db.data, key)
			db.expiryPolicy.RemoveExpiry(key)
			count++
		}
	}
	return count
}

// Exists 判断键是否存在
func (db *MemoryDatabase) Exists(key string) bool {
	return !db.expiryPolicy.IsExpired(key) && db.hasKey(key)
}

// hasKey 检查键是否存在（不考虑过期）
func (db *MemoryDatabase) hasKey(key string) bool {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	_, exists := db.data[key]
	return exists
}

// Expire 设置键的过期时间
func (db *MemoryDatabase) Expire(key string, expiration time.Duration) bool {
	// 检查键是否存在
	if !db.hasKey(key) {
		return false
	}

	// 设置过期时间
	return db.expiryPolicy.SetExpiry(key, expiration)
}

// TTL 获取键的剩余生存时间
func (db *MemoryDatabase) TTL(key string) time.Duration {
	// 检查键是否存在
	if !db.hasKey(key) {
		return -2 * time.Second // 键不存在
	}

	// 获取过期时间
	expireTime, hasExpiry := db.expiryPolicy.GetExpiry(key)
	if !hasExpiry {
		return -1 * time.Second // 键没有设置过期时间
	}

	// 计算剩余时间
	ttl := expireTime.Sub(time.Now())
	if ttl < 0 {
		// 键已过期，异步删除
		go db.Delete(key)
		return -2 * time.Second
	}

	return ttl
}

// Keys 按模式获取匹配的键
func (db *MemoryDatabase) Keys(pattern string) []string {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	// 将Redis风格的通配符转换为正则表达式
	regexPattern := pattern
	if pattern == "*" {
		regexPattern = ".*"
	} else {
		// 转义正则表达式特殊字符
		specialChars := []string{".", "+", "^", "$", "(", ")", "[", "]", "{", "}", "|"}
		for _, char := range specialChars {
			regexPattern = strings.ReplaceAll(regexPattern, char, "\\"+char)
		}
		// 转换Redis的通配符为正则表达式
		regexPattern = strings.ReplaceAll(regexPattern, "\\*", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "\\?", ".")
	}

	// 添加开头和结尾的边界
	regexPattern = "^" + regexPattern + "$"

	// 编译正则表达式
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil
	}

	keys := make([]string, 0, len(db.data))
	for key := range db.data {
		// 跳过空字符串键
		if key == "" {
			continue
		}

		// 检查键是否过期
		if db.expiryPolicy.IsExpired(key) {
			continue
		}

		// 匹配模式
		if regex.MatchString(key) {
			keys = append(keys, key)
		}
	}

	return keys
}

// Flush 清空数据库中的所有数据
func (db *MemoryDatabase) Flush() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	// 清空数据
	db.data = make(map[string]cache2.Value)

	// 清空过期时间
	db.expiryPolicy = NewDefaultExpiryPolicy()

	// 重置统计信息
	db.stats.KeyCount = 0
	db.stats.ExpiredKeys = 0
}

// Size 返回数据库中键的数量
func (db *MemoryDatabase) Size() int64 {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	return int64(len(db.data))
}

// Close 关闭数据库
func (db *MemoryDatabase) Close() error {
	// 取消上下文，通知所有goroutine退出
	db.cancel()

	// 关闭持久化管理器
	if db.persistence != nil {
		if err := db.persistence.Close(); err != nil {
			return err
		}
	}

	// 等待所有goroutine退出
	db.wg.Wait()

	return nil
}

// ProcessCommand 处理命令
func (db *MemoryDatabase) ProcessCommand(cmd cache2.Command) cache2.Reply {
	// 检查命令是否为nil
	if cmd == nil {
		return protocol.NewErrorReply("ERR nil command")
	}

	// 获取命令名并转换为大写以进行不区分大小写的比较
	cmdName := strings.ToUpper(cmd.Name())

	// 更新命令计数
	db.mutex.Lock()
	db.stats.CmdProcessed++
	db.mutex.Unlock()

	// 检查是否是读命令
	isReadCmd := false
	switch cmdName {
	case "GET", "EXISTS", "TYPE", "TTL", "KEYS", "LLEN", "LRANGE", "LINDEX", "STRLEN",
		"HGET", "HEXISTS", "HLEN", "HGETALL", "HKEYS", "HVALS",
		"SISMEMBER", "SMEMBERS", "SCARD", "SINTER", "SUNION", "SDIFF",
		"ZSCORE", "ZCARD", "ZRANGE", "ZREVRANGE", "ZRANGEBYSCORE", "ZRANK", "ZREVRANK", "PING", "DBSIZE", "FLUSHDB", "FLUSHALL":
		isReadCmd = true
	}

	// 处理命令
	switch cmdName {
	case "DBSIZE":
		return protocol.NewIntegerReply(db.Size())
	case "PING":
		return db.execPing(cmd.Args())
	case "DEL":
		// 检查参数数量
		if len(cmd.Args()) < 1 {
			return protocol.NewErrorReply("ERR wrong number of arguments")
		}
		return db.execDel(cmd.Args())
	case "EXISTS":
		reply := db.execExists(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "TYPE":
		return db.execType(cmd.Args())
	case "EXPIRE":
		reply := db.execExpire(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "TTL":
		reply := db.execTTL(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "KEYS":
		reply := db.execKeys(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "FLUSHDB":
		reply := db.execFlushDB(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "FLUSHALL":
		reply := db.execFlushAll(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply

	// 字符串命令
	case "GET":
		reply := db.execGet(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SET":
		reply := db.execSet(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SETNX":
		reply := db.execSetNX(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "MSET":
		reply := db.execMSet(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "INCR":
		reply := db.execIncr(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "INCRBY":
		reply := db.execIncrBy(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "DECR":
		reply := db.execDecr(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "DECRBY":
		reply := db.execDecrBy(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "APPEND":
		reply := db.execAppend(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "STRLEN":
		reply := db.execStrLen(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply

	// 列表命令
	case "LPUSH":
		reply := db.execLPush(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "RPUSH":
		reply := db.execRPush(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "LPOP":
		reply := db.execLPop(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "RPOP":
		reply := db.execRPop(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "LLEN":
		reply := db.execLLen(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "LRANGE":
		reply := db.execLRange(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "LINDEX":
		reply := db.execLIndex(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "LSET":
		reply := db.execLSet(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "LREM":
		reply := db.execLRem(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply

	// 哈希表命令
	case "HSET":
		reply := db.execHSet(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HSETNX":
		reply := db.execHSetNX(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HGET":
		reply := db.execHGet(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HDEL":
		reply := db.execHDel(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HEXISTS":
		reply := db.execHExists(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HLEN":
		reply := db.execHLen(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HGETALL":
		reply := db.execHGetAll(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HKEYS":
		reply := db.execHKeys(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HVALS":
		reply := db.execHVals(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "HINCRBY":
		reply := db.execHIncrBy(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply

	// 集合命令
	case "SADD":
		reply := db.execSAdd(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SREM":
		reply := db.execSRem(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SISMEMBER":
		reply := db.execSIsMember(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SMEMBERS":
		reply := db.execSMembers(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SCARD":
		reply := db.execSCard(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SPOP":
		reply := db.execSPop(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SINTER":
		reply := db.execSInter(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SUNION":
		reply := db.execSUnion(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "SDIFF":
		reply := db.execSDiff(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply

	// 有序集合命令
	case "ZADD":
		reply := db.execZAdd(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZSCORE":
		reply := db.execZScore(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZINCRBY":
		reply := db.execZIncrBy(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZCARD":
		reply := db.execZCard(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZRANGE":
		reply := db.execZRange(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZREVRANGE":
		reply := db.execZRevRange(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZRANGEBYSCORE":
		reply := db.execZRangeByScore(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZRANK":
		reply := db.execZRank(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZREVRANK":
		reply := db.execZRevRank(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	case "ZREM":
		reply := db.execZRem(cmd.Args())
		if reply.Type() != cache2.ReplyError {
			// 如果不是读命令，写入AOF
			if !isReadCmd {
				// 尝试写入AOF，最多重试3次
				var err error
				for i := 0; i < 3; i++ {
					err = db.persistence.WriteAOF(cmdName, cmd.Args())
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					common.GetLogger().Infof("Error writing to AOF file: %v", err)
				}
			}
		}
		return reply
	default:
		// 未知命令
		reply := protocol.NewErrorReply(fmt.Sprintf("ERR unknown command '%s'", cmdName))
		return reply
	}
}

// 内部方法

// isExpired 检查键是否过期
func (db *MemoryDatabase) isExpired(key string) bool {
	// 注意：调用此方法前需要持有读锁
	expireTime, hasExpiry := db.expiryPolicy.GetExpiry(key)
	return hasExpiry && time.Now().After(expireTime)
}

// incrementHits 增加键命中计数
func (db *MemoryDatabase) incrementHits() {
	// 使用原子操作更新统计信息
	go func() {
		db.mutex.Lock()
		defer db.mutex.Unlock()
		db.stats.KeyspaceHits++
	}()
}

// incrementMisses 增加键未命中计数
func (db *MemoryDatabase) incrementMisses() {
	// 使用原子操作更新统计信息
	go func() {
		db.mutex.Lock()
		defer db.mutex.Unlock()
		db.stats.KeyspaceMisses++
	}()
}

// processCommands 处理命令的协程
func (db *MemoryDatabase) processCommands() {
	defer db.wg.Done()

	for {
		select {
		case <-db.ctx.Done():
			// 上下文已取消，退出协程
			return
		case cmd := <-db.cmdChan:
			db.stats.CmdProcessed++
			reply := db.executeCommand(cmd)
			cmd.SetReply(reply)
		}
	}
}

// executeCommand 执行命令
func (db *MemoryDatabase) executeCommand(cmd cache2.Command) cache2.Reply {
	command := strings.ToUpper(cmd.Name())
	args := cmd.Args()

	var reply cache2.Reply

	// 主要的命令处理逻辑
	switch command {
	// 服务器命令
	case "PING":
		reply = db.execPing(args)
	case "FLUSHDB":
		reply = db.execFlushDB(args)
	case "FLUSHALL":
		reply = db.execFlushAll(args)

	// 键操作命令
	case "DEL":
		reply = db.execDel(args)
	case "EXISTS":
		reply = db.execExists(args)
	case "EXPIRE":
		reply = db.execExpire(args)
	case "TTL":
		reply = db.execTTL(args)
	case "TYPE":
		reply = db.execType(args)
	case "KEYS":
		reply = db.execKeys(args)

	// 字符串操作命令
	case "GET":
		reply = db.execGet(args)
	case "SET":
		reply = db.execSet(args)
	case "SETNX":
		reply = db.execSetNX(args)
	case "MSET":
		reply = db.execMSet(args)
	case "INCR":
		reply = db.execIncr(args)
	case "INCRBY":
		reply = db.execIncrBy(args)
	case "DECR":
		reply = db.execDecr(args)
	case "DECRBY":
		reply = db.execDecrBy(args)
	case "APPEND":
		reply = db.execAppend(args)
	case "STRLEN":
		reply = db.execStrLen(args)

	// 列表操作命令
	case "LPUSH":
		reply = db.execLPush(args)
	case "RPUSH":
		reply = db.execRPush(args)
	case "LPOP":
		reply = db.execLPop(args)
	case "RPOP":
		reply = db.execRPop(args)
	case "LLEN":
		reply = db.execLLen(args)
	case "LRANGE":
		reply = db.execLRange(args)
	case "LINDEX":
		reply = db.execLIndex(args)
	case "LSET":
		reply = db.execLSet(args)
	case "LREM":
		reply = db.execLRem(args)

	// 哈希表命令
	case "HSET":
		reply = db.execHSet(args)
	case "HSETNX":
		reply = db.execHSetNX(args)
	case "HGET":
		reply = db.execHGet(args)
	case "HDEL":
		reply = db.execHDel(args)
	case "HEXISTS":
		reply = db.execHExists(args)
	case "HLEN":
		reply = db.execHLen(args)
	case "HGETALL":
		reply = db.execHGetAll(args)
	case "HKEYS":
		reply = db.execHKeys(args)
	case "HVALS":
		reply = db.execHVals(args)
	case "HINCRBY":
		reply = db.execHIncrBy(args)

	// 集合命令
	case "SADD":
		reply = db.execSAdd(args)
	case "SREM":
		reply = db.execSRem(args)
	case "SISMEMBER":
		reply = db.execSIsMember(args)
	case "SMEMBERS":
		reply = db.execSMembers(args)
	case "SCARD":
		reply = db.execSCard(args)
	case "SPOP":
		reply = db.execSPop(args)
	case "SINTER":
		reply = db.execSInter(args)
	case "SUNION":
		reply = db.execSUnion(args)
	case "SDIFF":
		reply = db.execSDiff(args)

	// 有序集合命令
	case "ZADD":
		reply = db.execZAdd(args)
	case "ZSCORE":
		reply = db.execZScore(args)
	case "ZINCRBY":
		reply = db.execZIncrBy(args)
	case "ZCARD":
		reply = db.execZCard(args)
	case "ZRANGE":
		reply = db.execZRange(args)
	case "ZREVRANGE":
		reply = db.execZRevRange(args)
	case "ZRANGEBYSCORE":
		reply = db.execZRangeByScore(args)
	case "ZRANK":
		reply = db.execZRank(args)
	case "ZREVRANK":
		reply = db.execZRevRank(args)
	case "ZREM":
		reply = db.execZRem(args)
	default:
		// 未知命令
		reply = protocol.NewErrorReply(fmt.Sprintf("ERR unknown command '%s'", command))
	}

	// 如果命令执行成功且是写命令，将其写入AOF文件
	if db.persistence != nil && reply.Type() != cache2.ReplyError {
		// 检查是否是读命令
		isReadCmd := false
		switch command {
		case "GET", "EXISTS", "TYPE", "TTL", "KEYS", "LLEN", "LRANGE", "LINDEX", "STRLEN",
			"HGET", "HEXISTS", "HLEN", "HGETALL", "HKEYS", "HVALS",
			"SISMEMBER", "SMEMBERS", "SCARD", "SINTER", "SUNION", "SDIFF",
			"ZSCORE", "ZCARD", "ZRANGE", "ZREVRANGE", "ZRANGEBYSCORE", "ZRANK", "ZREVRANK", "PING", "DBSIZE":
			isReadCmd = true
		}

		// 如果不是读命令，写入AOF
		if !isReadCmd {
			// 尝试写入AOF，最多重试3次
			var err error
			for i := 0; i < 3; i++ {
				err = db.persistence.WriteAOF(command, args)
				if err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err != nil {
				common.GetLogger().Infof("Error writing to AOF file: %v", err)
			}
		}
	}

	return reply
}

// 基本命令实现

// execPing 实现PING命令
func (db *MemoryDatabase) execPing(args []string) cache2.Reply {
	if len(args) == 0 {
		return protocol.NewStatusReply("PONG")
	}
	return protocol.NewBulkReply(args[0])
}

// execDel 实现DEL命令
func (db *MemoryDatabase) execDel(args []string) cache2.Reply {
	if len(args) < 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'del' command")
	}

	count := db.Delete(args...)
	return protocol.NewIntegerReply(count)
}

// execExists 实现EXISTS命令
func (db *MemoryDatabase) execExists(args []string) cache2.Reply {
	if len(args) < 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'exists' command")
	}

	var count int64
	for _, key := range args {
		if db.Exists(key) {
			count++
		}
	}

	return protocol.NewIntegerReply(count)
}

// execExpire 实现EXPIRE命令
func (db *MemoryDatabase) execExpire(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'expire' command")
	}

	key := args[0]
	seconds, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}
	if seconds < 0 {
		return protocol.NewErrorReply("ERR invalid expire time in 'expire' command")
	}

	ok := db.Expire(key, time.Duration(seconds)*time.Second)
	if ok {
		return protocol.NewIntegerReply(1)
	}
	return protocol.NewIntegerReply(0)
}

// execTTL 实现TTL命令
func (db *MemoryDatabase) execTTL(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'ttl' command")
	}

	key := args[0]
	ttl := db.TTL(key)

	// 特殊处理: -2表示键不存在，-1表示键没有过期时间
	if ttl == -2*time.Second {
		return protocol.NewIntegerReply(-2)
	}

	if ttl == -1*time.Second {
		return protocol.NewIntegerReply(-1)
	}

	// 将时间转换为秒
	return protocol.NewIntegerReply(int64(ttl.Seconds()))
}

// execType 实现TYPE命令
func (db *MemoryDatabase) execType(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'type' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewStatusReply("none")
	}

	// 根据值类型返回相应的类型名
	switch value.Type() {
	case cache2.TypeString:
		return protocol.NewStatusReply("string")
	case cache2.TypeList:
		return protocol.NewStatusReply("list")
	case cache2.TypeHash:
		return protocol.NewStatusReply("hash")
	case cache2.TypeSet:
		return protocol.NewStatusReply("set")
	case cache2.TypeZSet:
		return protocol.NewStatusReply("zset")
	default:
		return protocol.NewStatusReply("unknown")
	}
}

// execKeys 实现KEYS命令
func (db *MemoryDatabase) execKeys(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'keys' command")
	}

	pattern := args[0]
	keys := db.Keys(pattern)

	// 将结果转换为多批量回复
	return protocol.NewMultiBulkReply(keys)
}

// 字符串命令实现

// execGet 实现GET命令
func (db *MemoryDatabase) execGet(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'get' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	return protocol.NewBulkReply(strValue.String())
}

// execSet 实现SET命令
func (db *MemoryDatabase) execSet(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'set' command")
	}

	key := args[0]
	value := args[1]

	// 创建字符串值
	strValue := ds2.NewString(value)

	// 检查是否有过期时间选项
	var expiration time.Duration = 0

	// 解析其他选项 (如EX, PX, NX, XX等)
	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX": // 以秒为单位的过期时间
			if i+1 >= len(args) {
				return protocol.NewErrorReply("ERR syntax error")
			}
			seconds, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || seconds <= 0 {
				return protocol.NewErrorReply("ERR invalid expire time in 'set' command")
			}
			expiration = time.Duration(seconds) * time.Second
			i++
		case "PX": // 以毫秒为单位的过期时间
			if i+1 >= len(args) {
				return protocol.NewErrorReply("ERR syntax error")
			}
			milliseconds, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || milliseconds <= 0 {
				return protocol.NewErrorReply("ERR invalid expire time in 'set' command")
			}
			expiration = time.Duration(milliseconds) * time.Millisecond
			i++
			// 后续可以添加NX, XX等选项支持
		}
	}

	// 设置值
	db.Set(key, strValue, expiration)
	return protocol.NewStatusReply("OK")
}

// execSetNX 实现SETNX命令
func (db *MemoryDatabase) execSetNX(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'setnx' command")
	}

	key := args[0]
	value := args[1]

	// 检查键是否已经存在
	if db.Exists(key) {
		return protocol.NewIntegerReply(0)
	}

	// 创建字符串值
	strValue := ds2.NewString(value)

	// 设置值
	db.Set(key, strValue, 0)

	return protocol.NewIntegerReply(1)
}

// execMSet 实现MSET命令
func (db *MemoryDatabase) execMSet(args []string) cache2.Reply {
	if len(args) < 2 || len(args)%2 != 0 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'mset' command")
	}

	// 设置所有键值对
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]

		// 创建字符串值
		strValue := ds2.NewString(value)

		// 设置值
		db.Set(key, strValue, 0)
	}

	return protocol.NewStatusReply("OK")
}

// execIncr 实现INCR命令
func (db *MemoryDatabase) execIncr(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'incr' command")
	}

	key := args[0]

	// 获取键对应的值
	value, exists := db.Get(key)

	if !exists {
		// 如果键不存在，创建一个新的值并设为1
		strValue := ds2.NewString("1")
		db.Set(key, strValue, 0)
		return protocol.NewIntegerReply(1)
	}

	// 确认值为字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	// 执行递增操作
	newVal, err := strValue.IncrBy(1)
	if err != nil {
		return protocol.NewErrorReply(err.Error())
	}

	return protocol.NewIntegerReply(newVal)
}

// execIncrBy 实现INCRBY命令
func (db *MemoryDatabase) execIncrBy(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'incrby' command")
	}

	key := args[0]
	increment, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	// 获取键对应的值
	value, exists := db.Get(key)

	if !exists {
		// 如果键不存在，创建一个新的值并设为增量值
		strValue := ds2.NewString(args[1])
		db.Set(key, strValue, 0)
		return protocol.NewIntegerReply(increment)
	}

	// 确认值为字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	// 执行递增操作
	newVal, err := strValue.IncrBy(increment)
	if err != nil {
		return protocol.NewErrorReply(err.Error())
	}

	return protocol.NewIntegerReply(newVal)
}

// execDecr 实现DECR命令
func (db *MemoryDatabase) execDecr(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'decr' command")
	}

	key := args[0]

	// 获取键对应的值
	value, exists := db.Get(key)

	if !exists {
		// 如果键不存在，创建一个新的值并设为-1
		strValue := ds2.NewString("-1")
		db.Set(key, strValue, 0)
		return protocol.NewIntegerReply(-1)
	}

	// 确认值为字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	// 执行递减操作
	newVal, err := strValue.DecrBy(1)
	if err != nil {
		return protocol.NewErrorReply(err.Error())
	}

	return protocol.NewIntegerReply(newVal)
}

// execDecrBy 实现DECRBY命令
func (db *MemoryDatabase) execDecrBy(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'decrby' command")
	}

	key := args[0]
	decrement, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	// 获取键对应的值
	value, exists := db.Get(key)

	if !exists {
		// 如果键不存在，创建一个新的值并设为减量的负值
		negValue := strconv.FormatInt(-decrement, 10)
		strValue := ds2.NewString(negValue)
		db.Set(key, strValue, 0)
		return protocol.NewIntegerReply(-decrement)
	}

	// 确认值为字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	// 执行递减操作
	newVal, err := strValue.DecrBy(decrement)
	if err != nil {
		return protocol.NewErrorReply(err.Error())
	}

	return protocol.NewIntegerReply(newVal)
}

// execAppend 实现APPEND命令
func (db *MemoryDatabase) execAppend(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'append' command")
	}

	key := args[0]
	appendStr := args[1]

	// 获取键对应的值
	value, exists := db.Get(key)

	if !exists {
		// 如果键不存在，创建一个新的值
		strValue := ds2.NewString(appendStr)
		db.Set(key, strValue, 0)
		return protocol.NewIntegerReply(int64(len(appendStr)))
	}

	// 如果键存在，确保值是字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	// 执行追加操作
	newLen := strValue.Append(appendStr)

	return protocol.NewIntegerReply(int64(newLen))
}

// execStrLen 实现STRLEN命令
func (db *MemoryDatabase) execStrLen(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'strlen' command")
	}

	key := args[0]

	// 获取键对应的值
	value, exists := db.Get(key)

	if !exists {
		// 如果键不存在，返回0
		return protocol.NewIntegerReply(0)
	}

	// 确认值为字符串类型
	if value.Type() != cache2.TypeString {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	strValue, ok := value.(cache2.StringValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a string")
	}

	// 返回字符串长度
	length := len(strValue.String())
	return protocol.NewIntegerReply(int64(length))
}

// cleanExpiredKeys 清理过期的键
func (db *MemoryDatabase) cleanExpiredKeys() {
	defer db.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-db.ctx.Done():
			return
		case <-ticker.C:
			// 使用过期策略清理过期键
			db.expiryPolicy.CleanExpiredKeys(func(key string) {
				db.mutex.Lock()
				delete(db.data, key)
				db.stats.ExpiredKeys++
				db.mutex.Unlock()
			})
		}
	}
}

// 列表命令实现

// execLPush 实现LPUSH命令
func (db *MemoryDatabase) execLPush(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'lpush' command")
	}

	key := args[0]
	value, exists := db.Get(key)

	var listValue cache2.ListValue

	if !exists {
		// 如果键不存在，创建一个新的列表
		listValue = ds2.NewList()
		// 将所有值添加到列表
		for i := 1; i < len(args); i++ {
			listValue.LPush(args[i])
		}
		// 存储新列表
		db.Set(key, listValue, 0)
	} else {
		// 键存在，但不是列表类型
		if value.Type() != cache2.TypeList {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		// 将值转换为列表类型
		var ok bool
		listValue, ok = value.(cache2.ListValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a list")
		}

		// 将所有值添加到列表
		for i := 1; i < len(args); i++ {
			listValue.LPush(args[i])
		}
	}

	// 返回列表长度
	return protocol.NewIntegerReply(listValue.Len())
}

// execRPush 实现RPUSH命令
func (db *MemoryDatabase) execRPush(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'rpush' command")
	}

	key := args[0]
	value, exists := db.Get(key)

	var listValue cache2.ListValue

	if !exists {
		// 如果键不存在，创建一个新的列表
		listValue = ds2.NewList()
		// 将所有值添加到列表
		for i := 1; i < len(args); i++ {
			listValue.RPush(args[i])
		}
		// 存储新列表
		db.Set(key, listValue, 0)
	} else {
		// 键存在，但不是列表类型
		if value.Type() != cache2.TypeList {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		// 将值转换为列表类型
		var ok bool
		listValue, ok = value.(cache2.ListValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a list")
		}

		// 将所有值添加到列表
		for i := 1; i < len(args); i++ {
			listValue.RPush(args[i])
		}
	}

	// 返回列表长度
	return protocol.NewIntegerReply(listValue.Len())
}

// execLPop 实现LPOP命令
func (db *MemoryDatabase) execLPop(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'lpop' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为列表类型
	if value.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := value.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	// 弹出左侧元素
	if listValue.Len() == 0 {
		return protocol.NewNilReply()
	}

	item, ok := listValue.LPop()
	if !ok {
		return protocol.NewNilReply()
	}

	// 如果列表为空，删除键
	if listValue.Len() == 0 {
		db.Delete(key)
	}

	return protocol.NewBulkReply(item)
}

// execRPop 实现RPOP命令
func (db *MemoryDatabase) execRPop(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'rpop' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为列表类型
	if value.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := value.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	// 弹出右侧元素
	if listValue.Len() == 0 {
		return protocol.NewNilReply()
	}

	item, ok := listValue.RPop()
	if !ok {
		return protocol.NewNilReply()
	}

	// 如果列表为空，删除键
	if listValue.Len() == 0 {
		db.Delete(key)
	}

	return protocol.NewBulkReply(item)
}

// execLLen 实现LLEN命令
func (db *MemoryDatabase) execLLen(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'llen' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为列表类型
	if value.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := value.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	// 返回列表长度
	return protocol.NewIntegerReply(listValue.Len())
}

// execLRange 实现LRANGE命令
func (db *MemoryDatabase) execLRange(args []string) cache2.Reply {
	if len(args) != 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'lrange' command")
	}

	key := args[0]
	start, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	stop, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为列表类型
	if value.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := value.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	// 获取指定范围的元素
	items := listValue.Range(start, stop)
	return protocol.NewMultiBulkReply(items)
}

// execLIndex 实现LINDEX命令
func (db *MemoryDatabase) execLIndex(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'lindex' command")
	}

	key := args[0]
	index, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为列表类型
	if value.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := value.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	element, ok := listValue.Index(index)
	if !ok {
		return protocol.NewNilReply()
	}

	return protocol.NewBulkReply(element)
}

// execLSet 实现LSET命令
func (db *MemoryDatabase) execLSet(args []string) cache2.Reply {
	if len(args) != 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'lset' command")
	}

	key := args[0]
	index, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	value := args[2]
	listObj, exists := db.Get(key)
	if !exists {
		return protocol.NewErrorReply("ERR no such key")
	}

	// 确认值为列表类型
	if listObj.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := listObj.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	// 设置指定索引的元素
	ok = listValue.SetItem(index, value)
	if !ok {
		return protocol.NewErrorReply("ERR index out of range")
	}

	return protocol.NewStatusReply("OK")
}

// execLRem 实现LREM命令
func (db *MemoryDatabase) execLRem(args []string) cache2.Reply {
	if len(args) != 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'lrem' command")
	}

	key := args[0]
	count, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	element := args[2]
	listObj, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为列表类型
	if listObj.Type() != cache2.TypeList {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listValue, ok := listObj.(cache2.ListValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a list")
	}

	// 移除指定元素
	removed := listValue.RemoveItem(count, element)

	return protocol.NewIntegerReply(removed)
}

// 哈希表命令实现

// execHSet 实现HSET命令
func (db *MemoryDatabase) execHSet(args []string) cache2.Reply {
	if len(args) < 3 || len(args)%2 != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hset' command")
	}

	key := args[0]
	value, exists := db.Get(key)

	var hashValue cache2.HashValue
	var count int64 = 0

	if !exists {
		// 如果键不存在，创建一个新的哈希表
		hashValue = ds2.NewHash()
		// 设置所有字段
		for i := 1; i < len(args); i += 2 {
			field := args[i]
			val := args[i+1]
			if hashValue.Set(field, val) {
				count++
			}
		}
		// 存储新哈希表
		db.Set(key, hashValue, 0)
	} else {
		// 如果键存在，确保其类型是哈希表
		if value.Type() != cache2.TypeHash {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		hashValue, ok = value.(cache2.HashValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a hash")
		}

		// 设置所有字段
		for i := 1; i < len(args); i += 2 {
			field := args[i]
			val := args[i+1]
			if hashValue.Set(field, val) {
				count++
			}
		}
	}

	return protocol.NewIntegerReply(count)
}

// execHSetNX 实现HSETNX命令
func (db *MemoryDatabase) execHSetNX(args []string) cache2.Reply {
	if len(args) != 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hsetnx' command")
	}

	key := args[0]
	field := args[1]
	value := args[2]

	hashObj, exists := db.Get(key)

	var hashValue cache2.HashValue

	if !exists {
		// 如果键不存在，创建一个新的哈希表
		hashValue = ds2.NewHash()
		hashValue.Set(field, value)
		db.Set(key, hashValue, 0)
		return protocol.NewIntegerReply(1)
	} else {
		// 如果键存在，确保其类型是哈希表
		if hashObj.Type() != cache2.TypeHash {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		hashValue, ok = hashObj.(cache2.HashValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a hash")
		}
	}

	// 如果字段不存在，则设置值
	if hashValue.Exists(field) {
		return protocol.NewIntegerReply(0)
	}

	hashValue.Set(field, value)
	return protocol.NewIntegerReply(1)
}

// execHGet 实现HGET命令
func (db *MemoryDatabase) execHGet(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hget' command")
	}

	key := args[0]
	field := args[1]

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 获取字段值
	val, exists := hashValue.Get(field)
	if !exists {
		return protocol.NewNilReply()
	}

	return protocol.NewBulkReply(val)
}

// execHDel 实现HDEL命令
func (db *MemoryDatabase) execHDel(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hdel' command")
	}

	key := args[0]
	fields := args[1:]

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 删除字段
	count := hashValue.Del(fields...)

	return protocol.NewIntegerReply(count)
}

// execHExists 实现HEXISTS命令
func (db *MemoryDatabase) execHExists(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hexists' command")
	}

	key := args[0]
	field := args[1]

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 检查字段是否存在
	if hashValue.Exists(field) {
		return protocol.NewIntegerReply(1)
	}

	return protocol.NewIntegerReply(0)
}

// execHLen 实现HLEN命令
func (db *MemoryDatabase) execHLen(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hlen' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 返回哈希表长度
	return protocol.NewIntegerReply(hashValue.Len())
}

// execHGetAll 实现HGETALL命令
func (db *MemoryDatabase) execHGetAll(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hgetall' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 获取所有字段和值
	entries := hashValue.GetAll()

	// 将字段和值转换为字符串数组
	result := make([]string, 0, len(entries)*2)
	for field, val := range entries {
		result = append(result, field, val)
	}

	return protocol.NewMultiBulkReply(result)
}

// execHKeys 实现HKEYS命令
func (db *MemoryDatabase) execHKeys(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hkeys' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 获取所有字段
	entries := hashValue.GetAll()

	// 提取字段名
	fields := make([]string, 0, len(entries))
	for field := range entries {
		fields = append(fields, field)
	}

	return protocol.NewMultiBulkReply(fields)
}

// execHVals 实现HVALS命令
func (db *MemoryDatabase) execHVals(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hvals' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为哈希表类型
	if value.Type() != cache2.TypeHash {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	hashValue, ok := value.(cache2.HashValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a hash")
	}

	// 获取所有值
	entries := hashValue.GetAll()

	// 提取值
	values := make([]string, 0, len(entries))
	for _, val := range entries {
		values = append(values, val)
	}

	return protocol.NewMultiBulkReply(values)
}

// execHIncrBy 实现HINCRBY命令
func (db *MemoryDatabase) execHIncrBy(args []string) cache2.Reply {
	if len(args) != 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'hincrby' command")
	}

	key := args[0]
	field := args[1]
	increment, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	value, exists := db.Get(key)

	var hashValue cache2.HashValue

	if !exists {
		// 如果键不存在，创建一个新的哈希表
		hashValue = ds2.NewHash()
		// 设置字段初始值为0，然后增加
		hashValue.Set(field, "0")
		db.Set(key, hashValue, 0)
	} else {
		// 如果键存在，确保是哈希表类型
		if value.Type() != cache2.TypeHash {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		hashValue, ok = value.(cache2.HashValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a hash")
		}
	}

	// 增加字段值
	newVal, err := hashValue.IncrBy(field, increment)
	if err != nil {
		return protocol.NewErrorReply(err.Error())
	}

	return protocol.NewIntegerReply(newVal)
}

// 集合命令实现

// execSAdd 实现SADD命令
func (db *MemoryDatabase) execSAdd(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'sadd' command")
	}

	key := args[0]
	members := args[1:]

	value, exists := db.Get(key)

	var setVal cache2.SetValue

	if !exists {
		// 如果键不存在，创建一个新的集合
		setVal = ds2.NewSet()
		// 添加所有成员
		added := setVal.Add(members...)
		// 存储新集合
		db.Set(key, setVal, 0)
		return protocol.NewIntegerReply(added)
	} else {
		// 如果键存在，确保值是集合类型
		if value.Type() != cache2.TypeSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		setVal, ok = value.(cache2.SetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a set")
		}
	}

	// 添加成员到集合
	added := setVal.Add(members...)

	return protocol.NewIntegerReply(added)
}

// execSRem 实现SREM命令
func (db *MemoryDatabase) execSRem(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'srem' command")
	}

	key := args[0]
	members := args[1:]

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	setVal, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 从集合中移除成员
	removed := setVal.Remove(members...)

	return protocol.NewIntegerReply(removed)
}

// execSIsMember 实现SISMEMBER命令
func (db *MemoryDatabase) execSIsMember(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'sismember' command")
	}

	key := args[0]
	member := args[1]

	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	setVal, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 检查成员是否存在
	if setVal.IsMember(member) {
		return protocol.NewIntegerReply(1)
	}

	return protocol.NewIntegerReply(0)
}

// execSMembers 实现SMEMBERS命令
func (db *MemoryDatabase) execSMembers(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'smembers' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	setVal, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 获取所有成员
	members := setVal.Members()

	return protocol.NewMultiBulkReply(members)
}

// execSCard 实现SCARD命令
func (db *MemoryDatabase) execSCard(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'scard' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	setVal, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 返回集合大小
	return protocol.NewIntegerReply(setVal.Len())
}

// execSPop 实现SPOP命令
func (db *MemoryDatabase) execSPop(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'spop' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	setVal, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 弹出一个随机成员
	member, ok := setVal.Pop()
	if !ok {
		return protocol.NewNilReply()
	}

	// 如果集合为空，删除键
	if setVal.Len() == 0 {
		db.Delete(key)
	}

	return protocol.NewBulkReply(member)
}

// execSInter 实现SINTER命令
func (db *MemoryDatabase) execSInter(args []string) cache2.Reply {
	if len(args) < 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'sinter' command")
	}

	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	firstSet, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 如果只有一个集合，返回其所有成员
	if len(args) == 1 {
		members := firstSet.Members()
		return protocol.NewMultiBulkReply(members)
	}

	// 获取其他集合
	otherSets := make([]cache2.SetValue, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		otherKey := args[i]
		otherValue, exists := db.Get(otherKey)
		if !exists {
			// 如果有一个集合不存在，交集为空
			return protocol.NewMultiBulkReply([]string{})
		}

		// 确认值为集合类型
		if otherValue.Type() != cache2.TypeSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		otherSet, ok := otherValue.(cache2.SetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a set")
		}

		otherSets = append(otherSets, otherSet)
	}

	// 计算交集
	intersection := firstSet.Inter(otherSets...)

	return protocol.NewMultiBulkReply(intersection)
}

// execSUnion 实现SUNION命令
func (db *MemoryDatabase) execSUnion(args []string) cache2.Reply {
	if len(args) < 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'sunion' command")
	}

	// 获取第一个集合
	key := args[0]
	value, exists := db.Get(key)

	var firstSet cache2.SetValue

	if !exists {
		// 如果第一个集合不存在，创建一个空集合
		firstSet = ds2.NewSet()
	} else {
		// 如果键存在，确保值是集合类型
		if value.Type() != cache2.TypeSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		firstSet, ok = value.(cache2.SetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a set")
		}
	}

	// 如果只有一个集合，返回其所有成员
	if len(args) == 1 {
		members := firstSet.Members()
		return protocol.NewMultiBulkReply(members)
	}

	// 获取其他集合
	otherSets := make([]cache2.SetValue, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		otherKey := args[i]
		otherValue, exists := db.Get(otherKey)
		if !exists {
			continue // 如果集合不存在，跳过
		}

		// 确认值为集合类型
		if otherValue.Type() != cache2.TypeSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		otherSet, ok := otherValue.(cache2.SetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a set")
		}

		otherSets = append(otherSets, otherSet)
	}

	// 计算并集
	union := firstSet.Union(otherSets...)

	return protocol.NewMultiBulkReply(union)
}

// execSDiff 实现SDIFF命令
func (db *MemoryDatabase) execSDiff(args []string) cache2.Reply {
	if len(args) < 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'sdiff' command")
	}

	// 获取第一个集合
	key := args[0]
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为集合类型
	if value.Type() != cache2.TypeSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	firstSet, ok := value.(cache2.SetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a set")
	}

	// 如果只有一个集合，返回其所有成员
	if len(args) == 1 {
		members := firstSet.Members()
		return protocol.NewMultiBulkReply(members)
	}

	// 获取其他集合
	otherSets := make([]cache2.SetValue, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		otherKey := args[i]
		otherValue, exists := db.Get(otherKey)
		if !exists {
			// 如果集合不存在，使用空集合
			otherSets = append(otherSets, ds2.NewSet())
			continue
		}

		// 确认值为集合类型
		if otherValue.Type() != cache2.TypeSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		otherSet, ok := otherValue.(cache2.SetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a set")
		}

		otherSets = append(otherSets, otherSet)
	}

	// 计算差集
	diff := firstSet.Diff(otherSets...)

	return protocol.NewMultiBulkReply(diff)
}

// execZAdd 实现ZADD命令
func (db *MemoryDatabase) execZAdd(args []string) cache2.Reply {
	if len(args) < 3 || len(args)%2 != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zadd' command")
	}

	key := args[0]

	// 获取键对应的值
	value, exists := db.Get(key)

	var zsetVal cache2.ZSetValue
	if !exists {
		// 如果键不存在，创建一个新的有序集合
		zsetVal = ds2.NewZSet()
		db.Set(key, zsetVal, 0)
	} else {
		// 如果键存在，确保值是有序集合类型
		if value.Type() != cache2.TypeZSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		zsetVal, ok = value.(cache2.ZSetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a zset")
		}
	}

	// 添加成员
	count := int64(0)
	for i := 1; i < len(args); i += 2 {
		score, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return protocol.NewErrorReply("ERR value is not a valid float")
		}

		member := args[i+1]
		if zsetVal.Add(score, member) {
			count++
		}
	}

	return protocol.NewIntegerReply(count)
}

// execZScore 实现ZSCORE命令
func (db *MemoryDatabase) execZScore(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zscore' command")
	}

	key := args[0]
	member := args[1]

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	// 获取成员分数
	score, exists := zsetVal.Score(member)
	if !exists {
		return protocol.NewNilReply()
	}

	// 将分数转换为字符串
	scoreStr := strconv.FormatFloat(score, 'f', -1, 64)

	return protocol.NewBulkReply(scoreStr)
}

// execZIncrBy 实现ZINCRBY命令
func (db *MemoryDatabase) execZIncrBy(args []string) cache2.Reply {
	if len(args) != 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zincrby' command")
	}

	key := args[0]
	increment, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not a valid float")
	}

	member := args[2]

	// 获取键对应的值
	value, exists := db.Get(key)

	var zsetVal cache2.ZSetValue
	if !exists {
		// 如果键不存在，创建一个新的有序集合
		zsetVal = ds2.NewZSet()
		db.Set(key, zsetVal, 0)
	} else {
		// 如果键存在，确保值是有序集合类型
		if value.Type() != cache2.TypeZSet {
			return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		var ok bool
		zsetVal, ok = value.(cache2.ZSetValue)
		if !ok {
			return protocol.NewErrorReply("ERR value is not a zset")
		}
	}

	// 增加成员分数
	newScore, ok := zsetVal.IncrBy(member, increment)
	if !ok {
		// 如果成员不存在，添加它
		zsetVal.Add(increment, member)
		newScore = increment
	}

	// 将分数转换为字符串
	scoreStr := strconv.FormatFloat(newScore, 'f', -1, 64)

	return protocol.NewBulkReply(scoreStr)
}

// execZCard 实现ZCARD命令
func (db *MemoryDatabase) execZCard(args []string) cache2.Reply {
	if len(args) != 1 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zcard' command")
	}

	key := args[0]

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	// 返回有序集合大小
	return protocol.NewIntegerReply(zsetVal.Len())
}

// execZRange 实现ZRANGE命令
func (db *MemoryDatabase) execZRange(args []string) cache2.Reply {
	if len(args) < 3 || len(args) > 4 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zrange' command")
	}

	key := args[0]
	start, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	stop, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	withScores := false
	if len(args) == 4 && strings.ToUpper(args[3]) == "WITHSCORES" {
		withScores = true
	}

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	var members []string

	if withScores {
		// 获取指定分数范围的成员和分数
		membersWithScores := zsetVal.RangeWithScores(start, stop)

		for member, score := range membersWithScores {
			members = append(members, member)
			scoreStr := strconv.FormatFloat(score, 'f', -1, 64)
			members = append(members, scoreStr)
		}
	} else {
		// 获取指定分数范围的成员
		members = zsetVal.Range(start, stop)

	}

	return protocol.NewMultiBulkReply(members)
}

// execZRevRange 实现ZREVRANGE命令
func (db *MemoryDatabase) execZRevRange(args []string) cache2.Reply {
	if len(args) < 3 || len(args) > 4 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zrevrange' command")
	}

	key := args[0]
	start, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	stop, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR value is not an integer or out of range")
	}

	withScores := false
	if len(args) == 4 && strings.ToUpper(args[3]) == "WITHSCORES" {
		withScores = true
	}

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	// 获取有序集合的长度
	length := zsetVal.Len()
	if length == 0 {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 处理负索引
	startIdx := start
	if startIdx < 0 {
		startIdx = int64(length) + startIdx
	}
	if startIdx < 0 {
		startIdx = 0
	}

	stopIdx := stop
	if stopIdx < 0 {
		stopIdx = int64(length) + stopIdx
	}
	if stopIdx >= int64(length) {
		stopIdx = int64(length) - 1
	}

	// 确保范围有效
	if startIdx > stopIdx || startIdx >= int64(length) {
		return protocol.NewMultiBulkReply([]string{})
	}

	var members []string

	if withScores {
		// 获取所有成员和分数
		membersWithScores := zsetVal.RangeWithScores(0, -1)

		// 创建一个有序的成员列表
		orderedMembers := make([]string, 0, len(membersWithScores))
		for member := range membersWithScores {
			orderedMembers = append(orderedMembers, member)
		}

		// 按分数降序排序
		sort.Slice(orderedMembers, func(i, j int) bool {
			scoreI := membersWithScores[orderedMembers[i]]
			scoreJ := membersWithScores[orderedMembers[j]]
			return scoreI > scoreJ
		})

		// 应用范围限制
		resultSize := int(stopIdx - startIdx + 1)
		if resultSize > len(orderedMembers) {
			resultSize = len(orderedMembers)
		}

		// 添加到结果列表
		members = make([]string, 0, resultSize*2)
		for i := int(startIdx); i <= int(stopIdx) && i < len(orderedMembers); i++ {
			member := orderedMembers[i]
			members = append(members, member)
			scoreStr := strconv.FormatFloat(membersWithScores[member], 'f', -1, 64)
			members = append(members, scoreStr)
		}
	} else {
		// 获取所有成员
		allMembers := zsetVal.Range(0, -1)

		// 获取所有成员和分数
		membersWithScores := zsetVal.RangeWithScores(0, -1)

		// 按分数降序排序
		sort.Slice(allMembers, func(i, j int) bool {
			scoreI := membersWithScores[allMembers[i]]
			scoreJ := membersWithScores[allMembers[j]]
			return scoreI > scoreJ
		})

		// 应用范围限制
		resultSize := int(stopIdx - startIdx + 1)
		if resultSize > len(allMembers) {
			resultSize = len(allMembers)
		}

		// 添加到结果列表
		members = make([]string, 0, resultSize)
		for i := int(startIdx); i <= int(stopIdx) && i < len(allMembers); i++ {
			members = append(members, allMembers[i])
		}
	}

	return protocol.NewMultiBulkReply(members)
}

// execZRank 实现ZRANK命令
func (db *MemoryDatabase) execZRank(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zrank' command")
	}

	key := args[0]
	member := args[1]

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	// 获取成员排名
	rank, exists := zsetVal.Rank(member)
	if !exists {
		return protocol.NewNilReply()
	}

	return protocol.NewIntegerReply(rank)
}

// execZRevRank 实现ZREVRANK命令
func (db *MemoryDatabase) execZRevRank(args []string) cache2.Reply {
	if len(args) != 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zrevrank' command")
	}

	key := args[0]
	member := args[1]

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewNilReply()
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	// 获取成员排名
	rank, exists := zsetVal.Rank(member)
	if !exists {
		return protocol.NewNilReply()
	}

	// 转换为反向排名
	revRank := zsetVal.Len() - 1 - rank

	return protocol.NewIntegerReply(revRank)
}

// execZRem 实现ZREM命令
func (db *MemoryDatabase) execZRem(args []string) cache2.Reply {
	if len(args) < 2 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zrem' command")
	}

	key := args[0]
	members := args[1:]

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewIntegerReply(0)
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	// 移除成员
	removed := zsetVal.Remove(members...)

	// 如果有序集合为空，删除键
	if zsetVal.Len() == 0 {
		db.Delete(key)
	}

	return protocol.NewIntegerReply(removed)
}

// execZRangeByScore 实现ZRANGEBYSCORE命令
func (db *MemoryDatabase) execZRangeByScore(args []string) cache2.Reply {
	if len(args) < 3 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'zrangebyscore' command")
	}

	key := args[0]
	minStr := args[1]
	maxStr := args[2]

	// 解析最小值和最大值
	min, err := strconv.ParseFloat(minStr, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR min or max is not a float")
	}

	max, err := strconv.ParseFloat(maxStr, 64)
	if err != nil {
		return protocol.NewErrorReply("ERR min or max is not a float")
	}

	// 检查是否需要返回分数
	withScores := false
	for i := 3; i < len(args); i++ {
		if strings.ToUpper(args[i]) == "WITHSCORES" {
			withScores = true
			break
		}
	}

	// 获取键对应的值
	value, exists := db.Get(key)
	if !exists {
		return protocol.NewMultiBulkReply([]string{})
	}

	// 确认值为有序集合类型
	if value.Type() != cache2.TypeZSet {
		return protocol.NewErrorReply("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	zsetVal, ok := value.(cache2.ZSetValue)
	if !ok {
		return protocol.NewErrorReply("ERR value is not a zset")
	}

	var members []string

	if withScores {
		// 获取指定分数范围的成员和分数
		membersWithScores := zsetVal.RangeByScoreWithScores(min, max)

		for member, score := range membersWithScores {
			members = append(members, member)
			scoreStr := strconv.FormatFloat(score, 'f', -1, 64)
			members = append(members, scoreStr)
		}
	} else {
		// 获取指定分数范围的成员
		members = zsetVal.RangeByScore(min, max)
	}

	return protocol.NewMultiBulkReply(members)
}

// 注册所有命令处理函数

// execFlushDB 实现FLUSHDB命令，清空当前数据库
func (db *MemoryDatabase) execFlushDB(args []string) cache2.Reply {
	if len(args) > 0 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'flushdb' command")
	}

	// 清空当前数据库所有键
	db.Flush()
	return protocol.NewStatusReply("OK")
}

// execFlushAll 实现FLUSHALL命令，清空所有数据库
// 注意在数据库级别只能清空当前数据库，完整的FLUSHALL功能需要在引擎级别实现
func (db *MemoryDatabase) execFlushAll(args []string) cache2.Reply {
	if len(args) > 0 {
		return protocol.NewErrorReply("ERR wrong number of arguments for 'flushall' command")
	}

	// 在数据库级别，FLUSHALL实际上只能清空当前数据库
	// 真正的FLUSHALL功能在服务器层面通过引擎的FlushAll方法实现
	db.Flush()
	return protocol.NewStatusReply("OK")
}
