# 数据库指标收集器使用说明

数据库收集器(`DatabaseCollector`)用于收集和监控数据库操作的性能指标，包括SQL执行时间、错误统计、慢查询检测等。

## 核心功能

- **数据库操作监控**: 记录每次SQL执行的详细信息
- **性能分析**: 统计查询耗时、影响行数等指标
- **错误追踪**: 捕获和分类数据库操作错误
- **慢查询检测**: 标识和统计慢查询
- **多数据库支持**: 支持MySQL、PostgreSQL等多种数据库

## 指标类型

### 核心指标

- `db.operation.count`: 数据库操作总数（计数器）
- `db.operation.duration`: 数据库操作耗时（仪表，单位：毫秒）
- `db.error.count`: 数据库错误总数（计数器）
- `db.slow.query.count`: 慢查询总数（计数器）
- `db.rows.affected`: 影响行数（仪表）

### 标签维度

- `service_name`: 服务名称
- `instance_id`: 实例标识
- `database_name`: 数据库名称
- `table_name`: 数据表名称
- `operation`: 操作类型（SELECT/INSERT/UPDATE/DELETE等）
- `method`: 执行方法名（可选）
- `driver`: 数据库驱动（可选）
- `error_code`: 错误码（仅错误指标）

## 基本使用方法

### 1. 创建数据库收集器

```go
package main

import (
    "github.com/xsxdot/aio/pkg/monitoring/collector"
    "github.com/xsxdot/aio/pkg/monitoring/storage"
    "go.uber.org/zap"
)

func main() {
    // 创建存储引擎
    storageConfig := storage.Config{
        DataDir:       "/data/metrics",
        RetentionDays: 7,
    }
    localStorage, _ := storage.New(storageConfig)

    // 创建数据库收集器
    dbCollectorConfig := collector.DatabaseCollectorConfig{
        ServiceName: "user-service",
        InstanceID:  "user-service-001",
        Logger:      zap.NewExample(),
        Storage:     localStorage,
    }
    
    dbCollector := collector.NewDatabaseCollector(dbCollectorConfig)
    
    // 启动收集器
    dbCollector.Start()
}
```

### 2. 记录数据库操作

#### 单个操作记录

```go
import (
    "time"
    "github.com/xsxdot/aio/pkg/monitoring/collector"
)

// 在数据库操作前后记录指标
func GetUserByID(userID int) (*User, error) {
    startTime := time.Now()
    
    // 执行数据库查询
    sql := "SELECT * FROM users WHERE id = ?"
    rows, err := db.Query(sql, userID)
    
    // 记录操作指标
    operationMetrics := &collector.DatabaseOperationMetrics{
        DatabaseName: "userdb",
        TableName:    "users",
        Operation:    collector.DatabaseOperationSELECT,
        Method:       "GetUserByID",
        SQL:          sql,
        Duration:     float64(time.Since(startTime).Nanoseconds()) / 1e6, // 转换为毫秒
        RowsReturned: 1,
    }
    
    if err != nil {
        operationMetrics.ErrorMessage = err.Error()
        operationMetrics.ErrorCode = "1064" // MySQL错误码示例
    }
    
    // 慢查询检测（超过100ms认为是慢查询）
    if operationMetrics.Duration > 100 {
        operationMetrics.IsSlowQuery = true
    }
    
    // 记录到收集器
    dbCollector.RecordDatabaseOperation(operationMetrics)
    
    return user, err
}
```

#### 批量操作记录

```go
func ProcessBatchOperations() {
    var operations []*collector.DatabaseOperationMetrics
    
    // 批量操作示例
    for i := 0; i < 100; i++ {
        startTime := time.Now()
        
        // 执行插入操作
        sql := "INSERT INTO orders (user_id, amount) VALUES (?, ?)"
        result, err := db.Exec(sql, userID, amount)
        
        operation := &collector.DatabaseOperationMetrics{
            DatabaseName: "orderdb",
            TableName:    "orders",
            Operation:    collector.DatabaseOperationINSERT,
            Method:       "CreateOrder",
            SQL:          sql,
            Duration:     float64(time.Since(startTime).Nanoseconds()) / 1e6,
        }
        
        if err != nil {
            operation.ErrorMessage = err.Error()
        } else {
            affected, _ := result.RowsAffected()
            operation.RowsAffected = affected
        }
        
        operations = append(operations, operation)
    }
    
    // 批量记录
    dbCollector.RecordBatch(operations)
}
```

### 3. 中间件集成示例

#### GORM中间件

```go
import (
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

func setupGORMWithMetrics(db *gorm.DB, dbCollector *collector.DatabaseCollector) {
    db.Callback().Query().Before("gorm:query").Register("metrics:before", func(db *gorm.DB) {
        db.Set("start_time", time.Now())
    })
    
    db.Callback().Query().After("gorm:query").Register("metrics:after", func(db *gorm.DB) {
        startTime, exists := db.Get("start_time")
        if !exists {
            return
        }
        
        duration := float64(time.Since(startTime.(time.Time)).Nanoseconds()) / 1e6
        
        operation := &collector.DatabaseOperationMetrics{
            DatabaseName: "mydb",
            TableName:    db.Statement.Table,
            Operation:    collector.DatabaseOperationSELECT,
            Method:       "GORM Query",
            SQL:          db.Statement.SQL.String(),
            Duration:     duration,
            RowsReturned: db.RowsAffected,
        }
        
        if db.Error != nil {
            operation.ErrorMessage = db.Error.Error()
        }
        
        dbCollector.RecordDatabaseOperation(operation)
    })
}
```

#### database/sql中间件

```go
import (
    "database/sql/driver"
    "context"
)

type MetricsConnector struct {
    driver.Connector
    collector *collector.DatabaseCollector
}

func (c *MetricsConnector) Connect(ctx context.Context) (driver.Conn, error) {
    conn, err := c.Connector.Connect(ctx)
    if err != nil {
        return nil, err
    }
    return &MetricsConn{Conn: conn, collector: c.collector}, nil
}

type MetricsConn struct {
    driver.Conn
    collector *collector.DatabaseCollector
}

func (c *MetricsConn) Query(query string, args []driver.Value) (driver.Rows, error) {
    startTime := time.Now()
    rows, err := c.Conn.Query(query, args)
    
    operation := &collector.DatabaseOperationMetrics{
        DatabaseName: "mydb",
        Operation:    collector.DatabaseOperationSELECT,
        SQL:          query,
        Duration:     float64(time.Since(startTime).Nanoseconds()) / 1e6,
    }
    
    if err != nil {
        operation.ErrorMessage = err.Error()
    }
    
    c.collector.RecordDatabaseOperation(operation)
    return rows, err
}
```

## 操作类型

支持的数据库操作类型：

- `DatabaseOperationSELECT`: 查询操作
- `DatabaseOperationINSERT`: 插入操作
- `DatabaseOperationUPDATE`: 更新操作
- `DatabaseOperationDELETE`: 删除操作
- `DatabaseOperationCREATE`: 创建操作（建表等）
- `DatabaseOperationDROP`: 删除操作（删表等）
- `DatabaseOperationALTER`: 修改操作（修改表结构等）
- `DatabaseOperationOTHER`: 其他操作

## 最佳实践

### 1. SQL安全性

为了保护敏感数据，建议：

```go
// 使用SQL Hash而不是完整SQL语句
operationMetrics.SQLHash = fmt.Sprintf("%x", md5.Sum([]byte(sql)))
operationMetrics.SQL = "" // 不记录完整SQL
```

### 2. 慢查询阈值

根据业务需求设定慢查询阈值：

```go
const SlowQueryThreshold = 100.0 // 100毫秒

if operationMetrics.Duration > SlowQueryThreshold {
    operationMetrics.IsSlowQuery = true
}
```

### 3. 批量处理

对于高频操作，使用批量记录以提升性能：

```go
// 定期批量提交，而不是每次操作都记录
func periodicFlush() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        if len(pendingOperations) > 0 {
            dbCollector.RecordBatch(pendingOperations)
            pendingOperations = pendingOperations[:0]
        }
    }
}
```

### 4. 错误分类

根据数据库类型对错误进行分类：

```go
func categorizeError(err error, dbType string) (code string, category string) {
    switch dbType {
    case "mysql":
        if mysqlErr, ok := err.(*mysql.MySQLError); ok {
            return fmt.Sprintf("%d", mysqlErr.Number), "mysql_error"
        }
    case "postgres":
        if pgErr, ok := err.(*pq.Error); ok {
            return string(pgErr.Code), "postgres_error"
        }
    }
    return "unknown", "generic_error"
}
```

## 监控和告警

基于收集的指标可以设置以下监控规则：

1. **错误率监控**: `db.error.count / db.operation.count > 0.01`
2. **慢查询监控**: `db.slow.query.count > 10 (per minute)`
3. **平均响应时间**: `avg(db.operation.duration) > 200ms`
4. **操作频率异常**: `rate(db.operation.count) > normal_threshold`

## 注意事项

1. **性能影响**: 指标收集会带来少量性能开销，建议在生产环境中适当采样
2. **存储空间**: 高频操作会产生大量指标数据，注意存储空间管理
3. **隐私保护**: 避免在指标中记录敏感的SQL内容
4. **错误处理**: 确保指标收集失败不会影响业务逻辑 