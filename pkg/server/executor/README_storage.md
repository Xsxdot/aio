# Executor SQLite 存储实现

这个文件实现了 `ExecutorStorage` 接口，使用 SQLite 作为持久化存储，用于保存命令执行历史记录。

## 特性

- ✅ 实现完整的 `ExecutorStorage` 接口
- ✅ 使用 SQLite 轻量级数据库
- ✅ 支持单个命令和批量命令执行结果存储
- ✅ 支持 JSON 序列化复杂数据结构
- ✅ 提供过期记录清理功能
- ✅ 包含完整的错误处理
- ✅ 支持分页查询执行历史
- ✅ 自动创建数据库表和索引
- ✅ 连接池管理

## 数据库结构

### execute_results 表

| 字段名 | 类型 | 说明 |
|--------|------|------|
| request_id | TEXT | 请求ID（主键） |
| type | TEXT | 命令类型（single/batch） |
| server_id | TEXT | 服务器ID |
| async | INTEGER | 是否异步执行（0/1） |
| start_time | INTEGER | 开始时间（Unix时间戳） |
| end_time | INTEGER | 结束时间（Unix时间戳，可空） |
| command_result | TEXT | 单个命令结果（JSON，可空） |
| batch_result | TEXT | 批量命令结果（JSON，可空） |
| created_at | INTEGER | 创建时间（Unix时间戳） |
| updated_at | INTEGER | 更新时间（Unix时间戳） |

### 索引

- `idx_execute_results_server_id`: 按服务器ID索引
- `idx_execute_results_start_time`: 按开始时间索引  
- `idx_execute_results_created_at`: 按创建时间索引
- `idx_execute_results_type`: 按命令类型索引

## 使用示例

### 基本使用

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/xsxdot/aio/pkg/server"
    "github.com/xsxdot/aio/pkg/server/executor"
    "go.uber.org/zap"
)

func main() {
    // 创建logger
    logger, _ := zap.NewProduction()
    
    // 创建存储实例
    storage, err := executor.NewSQLiteStorage(executor.SQLiteConfig{
        DatabasePath: "./data/executor.db",
        Logger:       logger,
    })
    if err != nil {
        log.Fatal("创建存储失败:", err)
    }
    defer storage.(*executor.SQLiteStorage).Close()
    
    ctx := context.Background()
    
    // 保存执行结果
    result := &server.ExecuteResult{
        RequestID: "req-001",
        Type:      server.CommandTypeSingle,
        ServerID:  "server-1",
        Async:     false,
        StartTime: time.Now(),
        EndTime:   time.Now().Add(5 * time.Second),
        CommandResult: &server.CommandResult{
            CommandID:   "cmd-1",
            CommandName: "测试命令",
            Command:     "echo 'hello'",
            Status:      server.CommandStatusSuccess,
            ExitCode:    0,
            Stdout:      "hello\n",
        },
    }
    
    err = storage.SaveExecuteResult(ctx, result)
    if err != nil {
        log.Fatal("保存执行结果失败:", err)
    }
    
    // 查询执行结果
    retrieved, err := storage.GetExecuteResult(ctx, "req-001")
    if err != nil {
        log.Fatal("查询执行结果失败:", err)
    }
    
    log.Printf("查询到执行结果: %+v", retrieved)
    
    // 查询服务器执行历史
    results, total, err := storage.GetServerExecuteHistory(ctx, "server-1", 10, 0)
    if err != nil {
        log.Fatal("查询执行历史失败:", err)
    }
    
    log.Printf("服务器 server-1 共有 %d 条执行记录，返回 %d 条", total, len(results))
}
```

### 集成到 Executor 中

```go
package main

import (
    "context"
    "log"
    
    "github.com/xsxdot/aio/pkg/server"
    "github.com/xsxdot/aio/pkg/server/credential"
    "github.com/xsxdot/aio/pkg/server/executor"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    
    // 创建存储
    storage, err := executor.NewSQLiteStorage(executor.SQLiteConfig{
        DatabasePath: "./data/executor.db",
        Logger:       logger,
    })
    if err != nil {
        log.Fatal("创建存储失败:", err)
    }
    
    // 创建其他依赖服务（示例）
    var serverService server.Service       // 实际项目中需要初始化
    var credentialService credential.Service // 实际项目中需要初始化
    
    // 创建执行器
    executor := executor.NewExecutor(executor.Config{
        ServerService:     serverService,
        CredentialService: credentialService,
        Storage:           storage,
        Logger:            logger,
    })
    
    // 现在可以使用executor执行命令，执行结果会自动保存到SQLite数据库
    ctx := context.Background()
    
    // 执行命令示例
    req := &server.ExecuteRequest{
        ServerID: "server-1",
        Type:     server.CommandTypeSingle,
        Command: &server.Command{
            ID:      "cmd-1",
            Name:    "测试命令",
            Command: "echo 'hello world'",
        },
        SaveLog: true, // 设置为true以保存执行日志
    }
    
    result, err := executor.Execute(ctx, req)
    if err != nil {
        log.Fatal("执行命令失败:", err)
    }
    
    log.Printf("命令执行完成: %+v", result)
}
```

### 配置选项

```go
// 默认配置
storage, err := executor.NewSQLiteStorage(executor.SQLiteConfig{})

// 自定义配置
storage, err := executor.NewSQLiteStorage(executor.SQLiteConfig{
    DatabasePath: "/var/lib/aio/executor.db", // 自定义数据库路径
    Logger:       customLogger,               // 自定义日志器
})
```

### 定期清理过期记录

```go
// 创建定期清理任务
go func() {
    ticker := time.NewTicker(24 * time.Hour) // 每天清理一次
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // 清理30天前的记录
            err := storage.CleanupExpiredResults(context.Background(), 30*24*time.Hour)
            if err != nil {
                logger.Error("清理过期记录失败", zap.Error(err))
            } else {
                logger.Info("过期记录清理完成")
            }
        }
    }
}()
```

## API 说明

### SaveExecuteResult

保存命令执行结果到数据库。

```go
func (s *SQLiteStorage) SaveExecuteResult(ctx context.Context, result *ExecuteResult) error
```

- 支持单个命令和批量命令结果
- 使用 `INSERT OR REPLACE` 语句，支持更新已存在的记录
- 自动序列化复杂的嵌套结构为 JSON

### GetExecuteResult

根据请求ID获取执行结果。

```go
func (s *SQLiteStorage) GetExecuteResult(ctx context.Context, requestID string) (*ExecuteResult, error)
```

- 自动反序列化 JSON 数据
- 如果记录不存在返回错误

### GetServerExecuteHistory

获取指定服务器的执行历史记录。

```go
func (s *SQLiteStorage) GetServerExecuteHistory(ctx context.Context, serverID string, limit int, offset int) ([]*ExecuteResult, int, error)
```

- 支持分页查询
- 按执行开始时间倒序排序
- 返回总记录数和当前页记录

### DeleteExecuteResult

删除指定的执行记录。

```go
func (s *SQLiteStorage) DeleteExecuteResult(ctx context.Context, requestID string) error
```

- 如果记录不存在返回错误

### CleanupExpiredResults

清理过期的执行记录。

```go
func (s *SQLiteStorage) CleanupExpiredResults(ctx context.Context, expiration time.Duration) error
```

- 根据创建时间判断是否过期
- 返回清理的记录数量

## 错误处理

所有方法都遵循 Go 的错误处理最佳实践：

- 输入参数验证
- 数据库操作错误处理
- JSON 序列化/反序列化错误处理
- 详细的错误信息

## 性能注意事项

1. **索引优化**: 已为常用查询字段创建索引
2. **连接池**: 配置了合理的连接池参数
3. **批量操作**: 大量数据操作时建议使用事务
4. **定期清理**: 建议定期清理过期记录以保持性能

## 测试

运行测试验证存储功能：

```bash
cd pkg/server/executor
go test -v
```

测试覆盖了：
- 基本的增删改查操作
- 单个命令和批量命令结果存储
- 分页查询
- 过期记录清理
- 错误场景处理
- 配置选项

## 依赖

- `github.com/mattn/go-sqlite3`: SQLite 驱动
- `go.uber.org/zap`: 日志库
- `github.com/stretchr/testify`: 测试框架（测试用）

## 注意事项

1. SQLite 数据库文件会在首次使用时自动创建
2. 确保应用有写入数据库文件目录的权限
3. 在高并发场景下，考虑使用其他数据库（如 PostgreSQL）
4. 定期备份数据库文件
5. 在容器化部署时，建议将数据库文件挂载到持久卷 