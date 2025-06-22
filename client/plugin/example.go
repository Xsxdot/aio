package plugin

/*
GORM 监控插件使用示例：

package main

import (
    "context"
    "log"
    "time"

    "github.com/xsxdot/aio/client"
    "github.com/xsxdot/aio/client/plugin"
    "github.com/xsxdot/aio/pkg/registry"
    "github.com/xsxdot/aio/pkg/scheduler"
    "go.uber.org/zap"

    // GORM 相关包
    "gorm.io/gorm"
    "gorm.io/driver/mysql"
)

func main() {
    // 1. 创建基础组件
    logger, _ := zap.NewProduction()

    // 假设你已经有了这些组件
    serviceInfo := &registry.ServiceInstance{
        Name: "your-service",
        ID:   "instance-1",
        Env:  registry.EnvProd,
    }

    // manager 和 scheduler 的创建过程略（根据你的具体实现）
    var manager *client.GRPCClientManager
    var taskScheduler *scheduler.Scheduler

    // 2. 创建监控客户端
    monitorClient := client.NewMonitorClient(serviceInfo, manager, taskScheduler)

    // 3. 创建 GORM 监控插件配置
    pluginConfig := plugin.GORMMonitorConfig{
        MonitorClient: monitorClient,
        UserPackage:   "your.project.package", // 替换为你的项目包名前缀
        TraceKey:      "trace_id",             // context 中 TraceID 的键名
        Logger:        logger,                 // zap logger
        SlowThreshold: 200 * time.Millisecond, // 慢查询阈值
        Debug:         true,                   // 开启 debug 模式，打印详细指标信息
    }

    // 4. 创建插件实例
    gormMonitorPlugin := plugin.NewGORMMonitorPlugin(pluginConfig)

    // 5. 创建 GORM 数据库连接
    dsn := "user:password@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("failed to connect database", err)
    }

    // 6. 使用插件 - 现在非常简单！
    if err := db.Use(gormMonitorPlugin); err != nil {
        log.Fatal("failed to install gorm monitor plugin", err)
    }

    // 7. 启动监控客户端
    if err := monitorClient.Start(); err != nil {
        log.Fatal("failed to start monitor client", err)
    }

    // 现在所有的数据库操作都会被自动监控

    // 示例：带 TraceID 的数据库操作
    ctx := context.WithValue(context.Background(), "trace_id", "your-trace-id-123")

    // 这些操作会被自动记录指标，包括：
    // - 执行时间
    // - SQL 语句
    // - 表名
    // - 操作类型
    // - 错误信息
    // - 调用堆栈信息（文件名、方法名、行号）
    // - TraceID

    var users []User
    db.WithContext(ctx).Where("age > ?", 18).Find(&users)

    user := User{Name: "张三", Age: 25}
    db.WithContext(ctx).Create(&user)

    db.WithContext(ctx).Model(&user).Update("age", 26)

    db.WithContext(ctx).Delete(&user)

    // 原生 SQL 也会被监控
    var result []map[string]interface{}
    db.WithContext(ctx).Raw("SELECT * FROM users WHERE age > ?", 18).Scan(&result)

    // 清理资源
    defer func() {
        if err := monitorClient.Stop(); err != nil {
            log.Printf("failed to stop monitor client: %v", err)
        }
    }()
}

// User 示例模型
type User struct {
    ID   uint   `gorm:"primarykey"`
    Name string
    Age  int
}

// 自定义 TraceID 类型示例
type CustomTraceID struct {
    Value string
}

func (t CustomTraceID) String() string {
    return t.Value
}

// 使用自定义 TraceID 类型的示例
func exampleWithCustomTraceID() {
    // 插件会自动将任何类型转换为字符串
    customTraceID := CustomTraceID{Value: "custom-trace-123"}
    ctx := context.WithValue(context.Background(), "trace_id", customTraceID)

    // 使用带自定义 TraceID 的 context 进行数据库操作
    // var db *gorm.DB // 假设已初始化
    // db.WithContext(ctx).Find(&users)
}

// Debug 模式使用示例
func exampleWithDebugMode() {
    // 创建开发环境的 logger（更详细的日志）
    logger, _ := zap.NewDevelopment()

    // 配置 debug 模式
    pluginConfig := plugin.GORMMonitorConfig{
        MonitorClient: monitorClient,
        UserPackage:   "github.com/yourcompany/yourproject",
        TraceKey:      "trace_id",
        Logger:        logger,
        SlowThreshold: 100 * time.Millisecond, // 更严格的慢查询阈值
        Debug:         true,                   // 开启 debug 模式
    }

    gormPlugin := plugin.NewGORMMonitorPlugin(pluginConfig)

    // 假设已初始化数据库
    var db *gorm.DB
    db.Use(gormPlugin)

    // Debug 模式下的数据库操作会打印详细信息：
    // - 🐛 [GORM DEBUG] 数据库操作详情
    // - 🐛 [GORM DEBUG] SQL 语句
    // - 🐛 [GORM DEBUG] 🐌 检测到慢查询 (如果是慢查询)
    // - 🐛 [GORM DEBUG] ❌ 数据库操作失败 (如果有错误)

    ctx := context.WithValue(context.Background(), "trace_id", "debug-trace-123")

    // 这些操作会产生详细的 debug 日志
    var users []User
    db.WithContext(ctx).Where("age > ?", 18).Find(&users)
    db.WithContext(ctx).Create(&User{Name: "Test", Age: 25})
}

// 完整的故障排除示例
func troubleshootingExample() {
    logger, _ := zap.NewDevelopment()

    // 1. 创建数据库连接
    dsn := "user:password@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("数据库连接失败:", err)
    }

    // 2. 验证数据库对象
    if err := plugin.ValidateGORMDB(db); err != nil {
        log.Fatal("数据库对象验证失败:", err)
    }

    // 3. 使用调试工具查看详细信息
    plugin.DebugGORMDB(db)

    // 4. 创建插件配置（开启 debug 模式）
    config := plugin.GORMMonitorConfig{
        MonitorClient: monitorClient,
        UserPackage:   "your.project.package",
        TraceKey:      "trace_id",
        Logger:        logger,
        SlowThreshold: 100 * time.Millisecond,
        Debug:         true, // 开启 debug 模式
    }

    // 5. 创建并安装插件
    gormPlugin := plugin.NewGORMMonitorPlugin(config)
    if err := db.Use(gormPlugin); err != nil {
        log.Fatal("插件安装失败:", err)
    }

    log.Println("GORM 监控插件安装成功！")
}

// 性能优化建议示例
func performanceOptimizationExample() {
    logger, _ := zap.NewProduction() // 使用 production logger 提高性能

    config := plugin.GORMMonitorConfig{
        MonitorClient: monitorClient,
        UserPackage:   "your.project.package", // 指定用户包名可以减少堆栈遍历成本
        TraceKey:      "trace_id",
        Logger:        logger,
        SlowThreshold: 500 * time.Millisecond, // 适当的慢查询阈值
        Debug:         false,                  // 生产环境关闭 debug 模式
    }

    gormPlugin := plugin.NewGORMMonitorPlugin(config)

    // 假设已初始化数据库
    var db *gorm.DB
    db.Use(gormPlugin)

    // 性能提示：
    // 1. 生产环境建议关闭 debug 模式
    // 2. 合理设置慢查询阈值
    // 3. 指定 UserPackage 可以减少堆栈分析成本
    // 4. 监控数据是异步发送的，不会阻塞数据库操作
}

// 配置选项说明：
//
// MonitorClient: 必需，用于发送监控指标
// UserPackage: 可选，用于从调用堆栈中过滤出用户代码
//             例如："github.com/yourcompany/yourproject"
//             如果不设置，会排除常见的框架代码
// TraceKey: 可选，从 context 中提取 TraceID 的键名
//          支持任何类型的值，都会转换为字符串
// Logger: 可选，用于插件内部日志记录
// SlowThreshold: 可选，慢查询阈值，默认 200ms
// Debug: 可选，是否开启 debug 模式，默认 false
//        开启后会打印详细的数据库操作信息和指标内容
//
// 插件会自动记录以下指标：
// - db.operation.count: 数据库操作计数
// - db.operation.duration: 数据库操作耗时
// - db.error.count: 数据库错误计数
// - db.slow.query.count: 慢查询计数
// - db.rows.affected: 影响行数
//
// 每个指标都会包含以下标签：
// - service_name: 服务名称
// - instance_id: 实例ID
// - env: 环境标识
// - database_name: 数据库名称（如果可获取）
// - table_name: 表名
// - operation: 操作类型（SELECT、INSERT、UPDATE等）
// - method: 调用方法名
// - driver: 数据库驱动名称

// 最新更新说明：
//
// ✅ 移除了复杂的反射逻辑
// ✅ 直接使用 GORM 的标准插件接口
// ✅ 更好的性能和可靠性
// ✅ 简化的使用方式
// ✅ 完整的类型安全

*/
