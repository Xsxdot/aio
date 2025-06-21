# 分布式锁实现

本模块提供了基于 ETCD 的高性能、高可靠性分布式锁实现。

## 核心特性

### ✨ 全新重构的特性

- **🚀 高性能共享会话模型**: 单个管理器使用一个共享的 ETCD 会话，大幅提升性能，减少 ETCD 负载
- **🔔 锁丢失事件通知**: 通过 `Done()` channel 主动通知锁丢失事件，确保关键任务的安全性
- **🛡️ 防僵尸锁机制**: 自动监控会话健康状况，彻底解决"僵尸锁"问题
- **🏭 工厂模式**: 确保同一锁资源返回单例实例，保证可重入逻辑的正确性
- **🔧 正确的强制解锁**: 通过租约吊销实现可靠的管理员强制解锁功能

### 🎯 继承的核心功能

- **可重入锁**: 同一实例可多次获取同一锁
- **超时控制**: 支持带超时的锁获取
- **非阻塞获取**: `TryLock` 方法立即返回结果
- **锁信息查询**: 查看锁的详细信息（持有者、创建时间等）
- **锁列表**: 列出所有当前持有的锁

## 快速开始

### 1. 创建锁管理器

```go
import (
    "github.com/xsxdot/aio/pkg/lock"
    "github.com/xsxdot/aio/internal/etcd"
)

// 创建 ETCD 客户端
client, err := etcd.NewClient(etcdConfig)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 创建锁管理器
opts := &lock.LockManagerOptions{
    TTL: 30 * time.Second, // 锁的生存时间
}
manager, err := lock.NewEtcdLockManager(client, "/myapp/locks", opts)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()
```

### 2. 使用分布式锁

```go
// 创建锁实例
lock := manager.NewLock("my-resource", nil)

ctx := context.Background()

// 获取锁
err := lock.Lock(ctx)
if err != nil {
    log.Fatal(err)
}
defer lock.Unlock(context.Background())

// 执行关键代码
fmt.Println("执行受保护的代码...")
```

### 3. 锁丢失事件通知 (新特性)

```go
lock := manager.NewLock("critical-resource", nil)

// 获取锁
err := lock.Lock(ctx)
if err != nil {
    log.Fatal(err)
}

// 启动关键任务
go func() {
    for {
        select {
        case <-lock.Done():
            // 锁丢失！立即停止关键任务
            log.Warn("锁已丢失，停止关键任务")
            return
        case <-time.After(1 * time.Second):
            // 执行关键任务的一个步骤
            fmt.Println("执行关键任务...")
        }
    }
}()

// ... 其他代码
```

## API 参考

### LockManager 接口

```go
type LockManager interface {
    // 创建新的分布式锁
    NewLock(key string, opts *LockOptions) DistributedLock
    
    // 获取锁信息
    GetLockInfo(ctx context.Context, key string) (*LockInfo, error)
    
    // 列出所有锁
    ListLocks(ctx context.Context, prefix string) ([]*LockInfo, error)
    
    // 强制释放锁（管理员操作）
    ForceUnlock(ctx context.Context, key string) error
    
    // 关闭锁管理器
    Close() error
}
```

### DistributedLock 接口

```go
type DistributedLock interface {
    // 获取锁
    Lock(ctx context.Context) error
    
    // 尝试获取锁，不阻塞
    TryLock(ctx context.Context) (bool, error)
    
    // 带超时的获取锁
    LockWithTimeout(ctx context.Context, timeout time.Duration) error
    
    // 释放锁
    Unlock(ctx context.Context) error
    
    // 检查锁是否被当前实例持有
    IsLocked() bool
    
    // 获取锁的键
    GetLockKey() string
    
    // 🆕 返回锁丢失事件通知 channel
    Done() <-chan struct{}
}
```

## 配置选项

### LockManagerOptions

```go
type LockManagerOptions struct {
    TTL time.Duration // 锁的生存时间，默认 30 秒
}
```

### LockOptions

```go
type LockOptions struct {
    RetryInterval time.Duration // 重试间隔，默认 100ms
    MaxRetries    int           // 最大重试次数，0 表示无限重试
}
```

## 使用示例

### 基本用法

```go
func basicExample() {
    lock := manager.NewLock("resource-1", nil)
    
    ctx := context.Background()
    
    // 获取锁
    if err := lock.Lock(ctx); err != nil {
        log.Fatal(err)
    }
    defer lock.Unlock(context.Background())
    
    // 执行关键代码
    fmt.Println("执行受保护的操作")
}
```

### 可重入锁

```go
func reentrantExample() {
    lock := manager.NewLock("resource-1", nil)
    
    ctx := context.Background()
    
    // 第一次获取锁
    if err := lock.Lock(ctx); err != nil {
        log.Fatal(err)
    }
    defer lock.Unlock(context.Background())
    
    // 可重入：同一实例可再次获取
    if err := lock.Lock(ctx); err != nil {
        log.Fatal(err)
    }
    defer lock.Unlock(context.Background())
    
    fmt.Println("可重入锁获取成功")
}
```

### 超时控制

```go
func timeoutExample() {
    lock := manager.NewLock("resource-1", nil)
    
    ctx := context.Background()
    
    // 尝试在 5 秒内获取锁
    err := lock.LockWithTimeout(ctx, 5*time.Second)
    if err != nil {
        if e, ok := err.(*lock.LockError); ok && e.Code == lock.ErrCodeLockTimeout {
            fmt.Println("获取锁超时")
            return
        }
        log.Fatal(err)
    }
    defer lock.Unlock(context.Background())
    
    fmt.Println("成功获取锁")
}
```

### 非阻塞获取

```go
func tryLockExample() {
    lock := manager.NewLock("resource-1", nil)
    
    ctx := context.Background()
    
    // 尝试获取锁，立即返回
    acquired, err := lock.TryLock(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    if acquired {
        defer lock.Unlock(context.Background())
        fmt.Println("成功获取锁")
    } else {
        fmt.Println("锁已被其他实例持有")
    }
}
```

### 锁丢失监控

```go
func monitorLockLoss() {
    lock := manager.NewLock("critical-resource", nil)
    
    ctx := context.Background()
    
    // 获取锁
    if err := lock.Lock(ctx); err != nil {
        log.Fatal(err)
    }
    
    // 启动监控 goroutine
    go func() {
        <-lock.Done()
        log.Error("锁已丢失！停止所有关键任务")
        // 执行清理或补偿逻辑
    }()
    
    // 执行长时间运行的关键任务
    for i := 0; i < 100; i++ {
        select {
        case <-lock.Done():
            log.Error("锁丢失，任务中断")
            return
        default:
            // 执行任务步骤
            time.Sleep(100 * time.Millisecond)
        }
    }
    
    lock.Unlock(context.Background())
}
```

### 管理员操作

```go
func adminOperations() {
    ctx := context.Background()
    
    // 查看锁信息
    info, err := manager.GetLockInfo(ctx, "resource-1")
    if err != nil {
        log.Printf("获取锁信息失败: %v", err)
    } else {
        fmt.Printf("锁持有者: %s, 创建时间: %v\n", info.Owner, info.CreateTime)
    }
    
    // 列出所有锁
    locks, err := manager.ListLocks(ctx, "")
    if err != nil {
        log.Printf("列出锁失败: %v", err)
    } else {
        fmt.Printf("当前共有 %d 个锁\n", len(locks))
    }
    
    // 强制释放锁
    err = manager.ForceUnlock(ctx, "resource-1")
    if err != nil {
        log.Printf("强制释放锁失败: %v", err)
    } else {
        fmt.Println("强制释放锁成功")
    }
}
```

## 错误处理

分布式锁定义了以下错误类型：

```go
const (
    ErrCodeLockTimeout     = "LOCK_TIMEOUT"     // 锁获取超时
    ErrCodeLockNotHeld     = "LOCK_NOT_HELD"    // 锁未被持有
    ErrCodeLockAlreadyHeld = "LOCK_ALREADY_HELD" // 锁已被持有
    ErrCodeLockExpired     = "LOCK_EXPIRED"     // 锁已过期
    ErrCodeInvalidKey      = "INVALID_KEY"      // 无效的键
)
```

## 性能优化

### 共享会话模型的优势

1. **减少 ETCD 负载**: 所有锁共享一个会话，大幅减少与 ETCD 的连接数
2. **提升获取性能**: 避免每次加锁都创建新会话的开销
3. **自动续期**: 由 ETCD 客户端库自动处理租约续期，无需手动管理

### 最佳实践

1. **合理设置 TTL**: 根据业务需求设置合适的锁生存时间
2. **监控锁丢失**: 在关键任务中使用 `Done()` channel 监控锁状态
3. **及时释放锁**: 使用 `defer` 确保锁被正确释放
4. **错误处理**: 妥善处理各种锁相关错误

## 注意事项

1. **网络分区**: 在网络分区情况下，锁可能会意外释放，请使用 `Done()` channel 监控
2. **时钟同步**: 确保各节点时钟同步，避免 TTL 计算错误
3. **资源清理**: 应用退出时务必调用 `manager.Close()` 清理资源
4. **并发安全**: 所有接口都是并发安全的，可以在多个 goroutine 中使用

## 迁移指南

如果您正在从旧版本迁移，请注意以下变化：

1. **构造函数变化**: `NewEtcdLockManager` 现在需要 `LockManagerOptions` 参数
2. **移除的方法**: `Renew()` 方法已移除，续期现在自动处理
3. **新增的方法**: `Done()` 方法用于锁丢失事件通知
4. **配置变化**: TTL 配置从锁级别移动到管理器级别

## 故障排除

### 常见问题

1. **锁获取失败**: 检查 ETCD 连接和网络状况
2. **锁意外释放**: 检查网络稳定性和 TTL 设置
3. **内存泄漏**: 确保调用 `manager.Close()` 清理资源

### 日志监控

分布式锁会输出详细的日志信息，建议监控以下日志：

- `共享etcd会话已失效`: 表示会话连接问题
- `成功获取锁` / `成功释放锁`: 正常的锁操作
- `强制释放锁成功`: 管理员操作日志 