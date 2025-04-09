package cache

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		DBCount:         16,    // 默认支持16个数据库
		MaxMemory:       0,     // 默认不限制内存使用
		MaxClients:      10000, // 默认最大连接数10000
		Password:        "",    // 默认无密码
		EnableRDB:       false, // 默认启用RDB持久化
		RDBFilePath:     "cache/rdb",
		RDBSaveInterval: 3600,  // 默认每小时保存一次
		EnableAOF:       false, // 默认不启用AOF持久化
		AOFFilePath:     "cache/aof",
		AOFSyncStrategy: 1, // 默认每秒同步一次

		// 服务器配置
		Host:             "127.0.0.1", // 默认监听所有接口
		Port:             6379,        // 默认端口
		ReadTimeout:      60,          // 默认读超时时间(秒)
		WriteTimeout:     60,          // 默认写超时时间(秒)
		HeartbeatTimeout: 30,          // 默认心跳超时时间(秒)

		NodeID: "", // 默认节点ID为空
	}
}

// SentinelMonitorInfo 哨兵监控信息
type SentinelMonitorInfo struct {
	// Name 主节点名称
	Name string
	// Host 主节点地址
	Host string
	// Port 主节点端口
	Port int
	// Quorum 判定客观下线的票数阈值
	Quorum int
}

// Config 表示缓存服务的配置
type Config struct {
	// 基本配置
	DBCount    int    `yaml:"db_count" json:"db_count"`       // 数据库数量
	MaxMemory  int64  `yaml:"max_memory" json:"max_memory"`   // 最大内存使用量(MB)，0表示不限制
	MaxClients int    `yaml:"max_clients" json:"max_clients"` // 最大客户端连接数
	Password   string `yaml:"password" json:"password"`       // 访问密码，空表示不需要密码
	NodeID     string `yaml:"node_id" json:"node_id"`         // 节点ID，用于服务注册和发现

	// 持久化配置
	EnableRDB       bool   `yaml:"enable_rdb" json:"enable_rdb"`               // 是否启用RDB持久化
	RDBFilePath     string `yaml:"rdb_file_path" json:"rdb_file_path"`         // RDB文件路径
	RDBSaveInterval int    `yaml:"rdb_save_interval" json:"rdb_save_interval"` // RDB保存间隔(秒)
	EnableAOF       bool   `yaml:"enable_aof" json:"enable_aof"`               // 是否启用AOF持久化
	AOFFilePath     string `yaml:"aof_file_path" json:"aof_file_path"`         // AOF文件路径
	AOFSyncStrategy int    `yaml:"aof_sync_strategy" json:"aof_sync_strategy"` // AOF同步策略：0=always, 1=everysec, 2=no

	// 服务器配置
	Host             string `yaml:"host" json:"host"`                           // 服务器地址
	Port             int    `yaml:"port" json:"port"`                           // 服务器端口
	ReadTimeout      int    `yaml:"read_timeout" json:"read_timeout"`           // 读超时(秒)
	WriteTimeout     int    `yaml:"write_timeout" json:"write_timeout"`         // 写超时(秒)
	HeartbeatTimeout int    `yaml:"heartbeat_timeout" json:"heartbeat_timeout"` // 心跳超时(秒)

}

// WithMaxMemory 设置最大内存限制
func (c Config) WithMaxMemory(maxMemoryMB int64) Config {
	c.MaxMemory = maxMemoryMB
	return c
}

// WithPassword 设置密码
func (c Config) WithPassword(password string) Config {
	c.Password = password
	return c
}

// WithDBCount 设置数据库数量
func (c Config) WithDBCount(count int) Config {
	if count <= 0 {
		count = 16
	}
	c.DBCount = count
	return c
}

// WithPersistence 配置持久化选项
func (c Config) WithPersistence(enableRDB bool, enableAOF bool) Config {
	c.EnableRDB = enableRDB
	c.EnableAOF = enableAOF
	return c
}

// WithRDBOptions 配置RDB选项
func (c Config) WithRDBOptions(filePath string, saveInterval int) Config {
	if filePath != "" {
		c.RDBFilePath = filePath
	}
	if saveInterval > 0 {
		c.RDBSaveInterval = saveInterval
	}
	return c
}

// WithAOFOptions 配置AOF选项
func (c Config) WithAOFOptions(filePath string, syncStrategy int) Config {
	if filePath != "" {
		c.AOFFilePath = filePath
	}
	if syncStrategy >= 0 && syncStrategy <= 2 {
		c.AOFSyncStrategy = syncStrategy
	}
	return c
}

// WithServerOptions 配置服务器选项
func (c Config) WithServerOptions(host string, port int) Config {
	if host != "" {
		c.Host = host
	}
	if port > 0 {
		c.Port = port
	}
	return c
}

// WithTimeouts 配置超时选项
func (c Config) WithTimeouts(readTimeout, writeTimeout, heartbeatTimeout int) Config {
	if readTimeout > 0 {
		c.ReadTimeout = readTimeout
	}
	if writeTimeout > 0 {
		c.WriteTimeout = writeTimeout
	}
	if heartbeatTimeout > 0 {
		c.HeartbeatTimeout = heartbeatTimeout
	}
	return c
}

// WithNodeID 设置节点ID
func (c Config) WithNodeID(nodeID string) Config {
	c.NodeID = nodeID
	return c
}

// ValidateAndFix 验证并修复配置
func (c Config) ValidateAndFix() Config {
	// 确保数据库数量有效
	if c.DBCount <= 0 {
		c.DBCount = 16
	}

	// 确保最大客户端连接数有效
	if c.MaxClients <= 0 {
		c.MaxClients = 10000
	}

	// 根据系统可用内存调整最大内存限制
	if c.MaxMemory <= 0 {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		// 默认使用系统内存的80%作为上限（如果未指定）
		c.MaxMemory = int64(float64(memStats.Sys) * 0.8 / 1024 / 1024)
	}

	// 确保持久化目录存在
	if c.EnableRDB || c.EnableAOF {
		dir := filepath.Dir(c.RDBFilePath)
		if c.EnableAOF {
			dir = filepath.Dir(c.AOFFilePath)
		}
		_ = os.MkdirAll(dir, 0755)
	}

	// 确保超时设置有效
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 60
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 60
	}
	if c.HeartbeatTimeout <= 0 {
		c.HeartbeatTimeout = 30
	}

	// 确保服务器配置有效
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = 6379
	}

	return c
}
