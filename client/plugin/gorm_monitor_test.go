package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGORMMonitorPlugin_Initialize(t *testing.T) {
	// 创建内存数据库进行测试
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 创建测试用的 logger
	logger, _ := zap.NewDevelopment()

	// 创建插件配置
	config := GORMMonitorConfig{
		MonitorClient: nil, // 测试中不需要真实的客户端
		UserPackage:   "test.package",
		TraceKey:      "trace_id",
		Logger:        logger,
		SlowThreshold: 100 * time.Millisecond,
		Debug:         true,
	}

	// 创建插件
	plugin := NewGORMMonitorPlugin(config)
	assert.NotNil(t, plugin)

	// 测试初始化
	err = plugin.Initialize(db)
	assert.NoError(t, err)
}

func TestGORMMonitorPlugin_ValidateGORMDB(t *testing.T) {
	// 测试 nil 对象
	err := ValidateGORMDB(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为 nil")

	// 测试有效对象
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = ValidateGORMDB(db)
	assert.NoError(t, err)
}

func TestGORMMonitorPlugin_PluginInterface(t *testing.T) {
	// 创建测试用的 logger
	logger, _ := zap.NewDevelopment()

	// 创建插件
	plugin := NewGORMMonitorPlugin(GORMMonitorConfig{
		Logger: logger,
		Debug:  true,
	})

	// 验证插件名称
	assert.Equal(t, "gorm:monitor", plugin.Name())

	// 验证插件实现了 gorm.Plugin 接口
	var _ gorm.Plugin = plugin
}

func TestGORMMonitorPlugin_Callbacks(t *testing.T) {
	// 创建内存数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 创建插件
	logger, _ := zap.NewDevelopment()
	plugin := NewGORMMonitorPlugin(GORMMonitorConfig{
		Logger:        logger,
		SlowThreshold: 50 * time.Millisecond,
		Debug:         true,
	})

	// 安装插件
	err = db.Use(plugin)
	assert.NoError(t, err)

	// 创建测试表
	type User struct {
		ID   uint `gorm:"primarykey"`
		Name string
	}

	err = db.AutoMigrate(&User{})
	assert.NoError(t, err)

	// 执行一些数据库操作来触发回调
	user := User{Name: "Test User"}
	result := db.Create(&user)
	assert.NoError(t, result.Error)

	var users []User
	result = db.Find(&users)
	assert.NoError(t, result.Error)
	assert.Len(t, users, 1)
}

func TestDebugGORMDB(t *testing.T) {
	// 测试 nil 对象
	DebugGORMDB(nil)

	// 测试有效对象
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	DebugGORMDB(db)
}

func TestGORMMonitorPlugin_ErrorHandling(t *testing.T) {
	// 创建插件
	logger, _ := zap.NewDevelopment()
	plugin := NewGORMMonitorPlugin(GORMMonitorConfig{
		Logger: logger,
		Debug:  true,
	})

	// 测试错误码提取
	assert.Equal(t, "DUPLICATE", plugin.extractErrorCode("Error 1062: Duplicate entry"))
	assert.Equal(t, "NOT_FOUND", plugin.extractErrorCode("record not found"))
	assert.Equal(t, "TIMEOUT", plugin.extractErrorCode("context deadline exceeded timeout"))
	assert.Equal(t, "CONNECTION", plugin.extractErrorCode("connection refused"))
	assert.Equal(t, "UNKNOWN", plugin.extractErrorCode("unknown error"))
}

func BenchmarkGORMMonitorPlugin_Initialize(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

	logger, _ := zap.NewProduction()
	config := GORMMonitorConfig{
		Logger: logger,
		Debug:  false,
	}

	plugin := NewGORMMonitorPlugin(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Initialize(db)
	}
}

func BenchmarkGORMMonitorPlugin_DatabaseOperations(b *testing.B) {
	// 创建数据库和插件
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

	logger, _ := zap.NewProduction()
	plugin := NewGORMMonitorPlugin(GORMMonitorConfig{
		Logger: logger,
		Debug:  false,
	})

	db.Use(plugin)

	// 创建测试表
	type User struct {
		ID   uint `gorm:"primarykey"`
		Name string
	}
	db.AutoMigrate(&User{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := User{Name: "Benchmark User"}
		db.Create(&user)

		var users []User
		db.Find(&users)
	}
}
