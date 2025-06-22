# GORM 监控插件

本插件为 GORM ORM 提供自动的数据库操作监控功能，能够记录数据库操作的各种指标，包括执行时间、SQL 语句、调用堆栈信息、错误信息等。

## 功能特性

- ✅ **自动监控**：无需手动记录，插件自动捕获所有 GORM 数据库操作
- ✅ **调用堆栈跟踪**：自动获取调用方的文件名、方法名、行号
- ✅ **链路追踪支持**：从 context 中提取 TraceID，支持任意类型
- ✅ **慢查询检测**：可配置慢查询阈值，自动标记慢查询
- ✅ **全面的指标**：记录执行时间、错误信息、影响行数等
- ✅ **原生 GORM 集成**：直接使用 GORM 插件接口，性能更好，更可靠
- ✅ **灵活配置**：支持用户包名过滤、自定义日志等
- ✅ **Debug 模式**：详细的调试信息输出，便于开发和排查问题
- ✅ **类型安全**：完全的类型安全，无反射开销

## 支持的操作类型

- SELECT 查询操作
- INSERT 插入操作  
- UPDATE 更新操作
- DELETE 删除操作
- CREATE 创建操作
- DROP 删除操作
- ALTER 修改操作
- RAW 原生 SQL 操作

## 记录的指标

### 指标类型
- `db.operation.count`: 数据库操作总数
- `db.operation.duration`: 数据库操作耗时（毫秒）
- `db.error.count`: 数据库错误数
- `db.slow.query.count`: 慢查询数
- `db.rows.affected`: 影响行数

### 指标标签
- `service_name`: 服务名称
- `instance_id`: 实例ID
- `env`: 环境标识
- `database_name`: 数据库名称
- `table_name`: 表名
- `operation`: 操作类型
- `method`: 调用方法名
- `driver`: 数据库驱动名称
- `error_code`: 错误码（如果有错误）

## 使用方法

### 1. 基本配置

```go
package main

import (
    "context"
    "time"
    
    "github.com/xsxdot/aio/client"
    "github.com/xsxdot/aio/client/plugin"
    "go.uber.org/zap"
    "gorm.io/gorm"
    "gorm.io/driver/mysql"
)

func main() {
    // 创建监控客户端
    monitorClient := client.NewMonitorClient(serviceInfo, manager, scheduler)
    
    // 配置插件
    pluginConfig := plugin.GORMMonitorConfig{
        MonitorClient: monitorClient,           // 必需：监控客户端
        UserPackage:   "your.project.package", // 可选：用户包名过滤
        TraceKey:      "trace_id",             // 可选：TraceID 键名
        Logger:        logger,                 // 可选：日志器
        SlowThreshold: 200 * time.Millisecond, // 可选：慢查询阈值
        Debug:         false,                  // 可选：是否开启 debug 模式
    }
    
    // 创建插件
    gormPlugin := plugin.NewGORMMonitorPlugin(pluginConfig)
    
    // 初始化数据库连接
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        panic(err)
    }
    
    // 安装插件 - 现在非常简单！
    if err := db.Use(gormPlugin); err != nil {
        panic(err)
    }
    
    // 启动监控
    monitorClient.Start()
}
```

### 2. 带 TraceID 的使用

```go
// 支持任意类型的 TraceID
ctx := context.WithValue(context.Background(), "trace_id", "trace-123")

// 所有操作都会自动记录指标和 TraceID
db.WithContext(ctx).Find(&users)
db.WithContext(ctx).Create(&user)
```

### 3. 自定义 TraceID 类型

```go
type CustomTraceID struct {
    Value string
    SpanID string
}

func (t CustomTraceID) String() string {
    return fmt.Sprintf("%s:%s", t.Value, t.SpanID)
}

// 插件会自动调用 String() 方法或使用反射转换
customTrace := CustomTraceID{Value: "trace", SpanID: "span"}
ctx := context.WithValue(context.Background(), "trace_id", customTrace)
db.WithContext(ctx).Where("id = ?", 1).First(&user)
```

## 配置选项详解

### MonitorClient
- **类型**: `*client.MonitorClient`
- **必需**: 是
- **说明**: 用于发送监控指标的客户端

### UserPackage
- **类型**: `string`
- **必需**: 否
- **说明**: 用户包名前缀，用于从调用堆栈中过滤出用户代码
- **示例**: `"github.com/yourcompany/yourproject"`
- **默认**: 如果不设置，会排除常见的框架代码

### TraceKey
- **类型**: `string`
- **必需**: 否
- **说明**: 从 context 中提取 TraceID 的键名
- **示例**: `"trace_id"`, `"request_id"`, `"correlation_id"`

### Logger
- **类型**: `*zap.Logger`
- **必需**: 否
- **说明**: 插件内部使用的日志器
- **默认**: 如果不提供，会创建一个默认的 production logger

### SlowThreshold
- **类型**: `time.Duration`
- **必需**: 否
- **说明**: 慢查询判定阈值
- **默认**: `200 * time.Millisecond`

### Debug
- **类型**: `bool`
- **必需**: 否
- **说明**: 是否开启 debug 模式，打印详细的数据库操作信息和指标内容
- **默认**: `false`

## 调用堆栈过滤

插件会自动从调用堆栈中提取用户代码的调用信息，排除以下类型的代码：

### 自动排除的代码
- GORM 内部代码
- 插件自身代码
- Go 运行时代码
- 反射相关代码
- 常见框架代码（Gin、Fiber 等）

### 用户代码识别
如果设置了 `UserPackage` 参数，插件会优先匹配包含该前缀的代码。例如：

```go
UserPackage: "github.com/yourcompany/yourproject"
```

这样只有来自你项目的代码才会被记录为调用方。

## 监控指标示例

执行以下代码：
```go
ctx := context.WithValue(context.Background(), "trace_id", "abc-123")
db.WithContext(ctx).Where("age > ?", 18).Find(&users)
```

会产生类似以下的监控指标：

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "metric_name": "db.operation.duration",
  "value": 25.6,
  "labels": {
    "service_name": "user-service",
    "instance_id": "instance-1",
    "env": "prod",
    "table_name": "users",
    "operation": "SELECT",
    "method": "GetUsers",
    "driver": "mysql",
    "trace_id": "abc-123"
  }
}
```

## 错误处理

插件会自动捕获和分类数据库错误：

- `DUPLICATE`: 重复键错误
- `NOT_FOUND`: 记录不存在
- `TIMEOUT`: 超时错误
- `CONNECTION`: 连接错误
- `UNKNOWN`: 其他未知错误

## 性能考虑

- 插件使用反射进行 GORM 集成，会有轻微的性能开销
- 调用堆栈分析会增加少量 CPU 开销
- 建议在生产环境中合理设置 `UserPackage` 以减少堆栈遍历成本
- 监控数据是异步发送的，不会阻塞数据库操作

## Debug 模式

### 开启 Debug 模式

```go
pluginConfig := plugin.GORMMonitorConfig{
    MonitorClient: monitorClient,
    UserPackage:   "your.project.package",
    TraceKey:      "trace_id",
    Logger:        logger,
    SlowThreshold: 100 * time.Millisecond,
    Debug:         true, // 开启 debug 模式
}
```

### Debug 输出内容

开启 debug 模式后，每次数据库操作都会输出详细信息：

#### 1. 基本操作信息
```
🐛 [GORM DEBUG] 数据库操作详情
  service_name: user-service
  instance_id: instance-1
  env: prod
  trace_id: abc-123
  database_name: testdb
  table_name: users
  operation: SELECT
  driver: mysql
  duration_ms: 25.6
  rows_affected: 0
  rows_returned: 5
  is_slow_query: false
  method: GetUsers
  file_name: user_service.go
  line: 45
```

#### 2. SQL 语句
```
🐛 [GORM DEBUG] SQL 语句
  sql: SELECT * FROM `users` WHERE age > ? ORDER BY `users`.`id` LIMIT 1000
```

#### 3. 生成的指标点
```
🐛 [GORM DEBUG] 生成的指标点数量
  metric_points_count: 3

🐛 [GORM DEBUG] 指标点详情
  index: 1
  metric_name: db.operation.count
  metric_type: counter
  value: 1
  source: user-service
  instance: instance-1
  category: custom
  labels: {service_name: user-service, table_name: users, operation: SELECT}
```

#### 4. 慢查询警告
```
🐛 [GORM DEBUG] 🐌 检测到慢查询
  duration_ms: 350.2
  threshold: 200ms
  suggestion: 考虑优化SQL语句或添加索引
```

#### 5. 错误信息
```
🐛 [GORM DEBUG] ❌ 数据库操作失败
  error_code: DUPLICATE
  error_message: Error 1062: Duplicate entry 'test' for key 'users.name'
  table: users
  operation: INSERT
```

### Debug 模式的用途

- **开发阶段**：了解数据库操作的详细信息
- **性能调优**：识别慢查询和性能瓶颈
- **问题排查**：调试数据库操作错误
- **监控验证**：确认指标生成是否正确

## 故障排除

### 常见问题

1. **插件安装失败**
   
   **原因分析：**
   - 数据库对象为 `nil`
   - GORM 版本不兼容
   - 插件配置错误

   **解决方案：**
   ```go
   // 1. 检查数据库对象是否为 nil
   if db == nil {
       log.Fatal("数据库对象为 nil")
   }
   
   // 2. 验证数据库对象
   if err := plugin.ValidateGORMDB(db); err != nil {
       log.Fatal("数据库对象验证失败:", err)
   }
   
   // 3. 测试数据库连接
   if err := db.Exec("SELECT 1").Error; err != nil {
       log.Fatal("数据库连接失败:", err)
   }
   
   // 4. 开启 debug 模式获取详细信息
   pluginConfig := plugin.GORMMonitorConfig{
       MonitorClient: monitorClient,
       Debug:         true, // 开启 debug 模式
       Logger:        logger,
   }
   ```

2. **使用了错误的方法安装插件**
   
   **错误示例：**
   ```go
   // ❌ 这是旧版本的使用方式
   gormPlugin.Initialize(db)
   ```
   
   **正确做法：**
   ```go
   // ✅ 使用 GORM 标准插件接口
   if err := db.Use(gormPlugin); err != nil {
       log.Fatal("插件安装失败:", err)
   }
   ```

3. **没有记录到监控数据**
   - 确保 MonitorClient 已正确启动
   - 检查网络连接和监控服务状态
   - 查看插件日志是否有错误

4. **调用堆栈信息不正确**
   - 检查 UserPackage 配置是否正确
   - 确认调用代码确实来自指定的包

5. **GORM 版本兼容性问题**
   
   **支持的 GORM 版本：**
   - GORM v1.x: ✅ 支持
   - GORM v2.x: ✅ 支持
   
   **如果遇到版本问题：**
   ```go
   // 检查 GORM 版本
   fmt.Printf("GORM 版本: %s\n", gorm.Version)
   
       // 开启 debug 模式查看详细信息
    pluginConfig.Debug = true
    ```

6. **性能优化建议**

   **提高性能的配置：**
   ```go
   pluginConfig := plugin.GORMMonitorConfig{
       MonitorClient: monitorClient,
       UserPackage:   "your.project.package", // 指定用户包名减少堆栈遍历
       Logger:        productionLogger,       // 使用 production logger
       SlowThreshold: 500 * time.Millisecond, // 适当的慢查询阈值
       Debug:         false,                  // 生产环境关闭 debug 模式
   }
   ```

   **性能提示：**
   - 生产环境建议关闭 debug 模式
   - 合理设置慢查询阈值避免过多告警
   - 指定 UserPackage 可以减少堆栈分析成本
   - 监控数据是异步发送的，不会阻塞数据库操作

### 调试建议

#### 1. 启用 Debug 模式
```go
pluginConfig := plugin.GORMMonitorConfig{
    MonitorClient: monitorClient,
    UserPackage:   "your.project.package",
    TraceKey:      "trace_id",
    Logger:        logger,
    Debug:         true, // 开启 debug 模式
}
```

#### 2. 使用开发环境日志器
```go
logger, _ := zap.NewDevelopment() // 更详细的日志输出
pluginConfig.Logger = logger
```

#### 3. 结合使用获得最佳调试效果
```go
logger, _ := zap.NewDevelopment()
pluginConfig := plugin.GORMMonitorConfig{
    MonitorClient: monitorClient,
    UserPackage:   "your.project.package", 
    TraceKey:      "trace_id",
    Logger:        logger,     // 开发环境日志器
    Debug:         true,       // debug 模式
    SlowThreshold: 50 * time.Millisecond, // 更严格的慢查询检测
}
```

这会输出插件的详细运行信息，包括每次数据库操作的完整调试信息，有助于排查问题。

## 示例项目

完整的示例代码请参考 `example.go` 文件。 