# AIO 分布式锁包

这个包提供了基于ETCD的分布式锁实现，支持可重入锁、锁超时机制和锁续期功能。

## 特性

- ✅ **可重入锁**: 同一个拥有者可以多次获取同一把锁
- ✅ **锁超时机制**: 支持设置锁的TTL，防止死锁
- ✅ **自动续期**: 支持自动续期功能，防止锁意外过期
- ✅ **非阻塞获取**: 支持TryLock非阻塞获取锁
- ✅ **锁管理**: 支持查看锁信息、列出所有锁、强制释放锁
- ✅ **高可用**: 基于ETCD集群，保证高可用性

## 快速开始

### 1. 创建ETCD客户端和锁管理器

```go
package main

import (
    "context"
    "time"
    
    "github.com/xsxdot/aio/app/config"
    "github.com/xsxdot/aio/internal/etcd"
    "github.com/xsxdot/aio/pkg/lock"
)

func main() {
    // 创建ETCD配置
    etcdConfig := &config.EtcdConfig{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    }

    // 创建ETCD客户端
    client, err := etcd.NewClient(etcdConfig)
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 创建锁管理器
    lockManager, err := lock.NewEtcdLockManager(client, "/aio")
    if err != nil {
        panic(err)
    }
    defer lockManager.Close()
}
```

### 2. 基本锁操作

```go
// 创建锁
distributedLock := lockManager.NewLock("my-resource", lock.DefaultLockOptions())

ctx := context.Background()

// 获取锁
err := distributedLock.Lock(ctx)
if err != nil {
    panic(err)
}

// 执行临界区代码
// ... your business logic here ...

// 释放锁
err = distributedLock.Unlock(ctx)
if err != nil {
    panic(err)
}
```

### 3. 非阻塞获取锁

```go
// 尝试获取锁，不阻塞
acquired, err := distributedLock.TryLock(ctx)
if err != nil {
    panic(err)
}

if acquired {
    // 成功获取锁
    defer distributedLock.Unlock(ctx)
    // ... your business logic here ...
} else {
    // 锁被其他实例占用
    fmt.Println("资源被占用，稍后重试")
}
```

### 4. 带超时的锁获取

```go
// 最多等待10秒获取锁
err := distributedLock.LockWithTimeout(ctx, 10*time.Second)
if err != nil {
    if lockErr, ok := err.(*lock.LockError); ok && lockErr.Code == lock.ErrCodeLockTimeout {
        fmt.Println("获取锁超时")
    } else {
        panic(err)
    }
}
defer distributedLock.Unlock(ctx)
```

## 配置选项

### LockOptions

```go
type LockOptions struct {
    // TTL 锁的生存时间
    TTL time.Duration
    
    // AutoRenew 是否自动续期
    AutoRenew bool
    
    // RenewInterval 续期间隔
    RenewInterval time.Duration
    
    // RetryInterval 重试间隔
    RetryInterval time.Duration
    
    // MaxRetries 最大重试次数，0表示无限重试
    MaxRetries int
}
```

### 默认配置

```go
opts := &lock.LockOptions{
    TTL:           30 * time.Second,  // 锁有效期30秒
    AutoRenew:     true,              // 启用自动续期
    RenewInterval: 10 * time.Second,  // 每10秒续期一次
    RetryInterval: 100 * time.Millisecond, // 重试间隔100毫秒
    MaxRetries:    0,                 // 无限重试
}
```

## 可重入锁

同一个拥有者可以多次获取同一把锁：

```go
lock := lockManager.NewLock("reentrant-resource", lock.DefaultLockOptions())

// 第一次获取锁
err := lock.Lock(ctx)
if err != nil {
    panic(err)
}

// 可重入获取锁
err = lock.Lock(ctx)
if err != nil {
    panic(err)
}

// 需要对应数量的释放调用
lock.Unlock(ctx) // 第一次释放
lock.Unlock(ctx) // 第二次释放，真正释放锁
```

## 锁管理功能

### 查看锁信息

```go
info, err := lockManager.GetLockInfo(ctx, "my-resource")
if err != nil {
    panic(err)
}

fmt.Printf("锁键: %s\n", info.Key)
fmt.Printf("拥有者: %s\n", info.Owner)
fmt.Printf("创建时间: %v\n", info.CreateTime)
fmt.Printf("过期时间: %v\n", info.ExpireTime)
```

### 列出所有锁

```go
// 列出所有锁
locks, err := lockManager.ListLocks(ctx, "")
if err != nil {
    panic(err)
}

// 列出特定前缀的锁
locks, err = lockManager.ListLocks(ctx, "user-")
if err != nil {
    panic(err)
}

for _, lockInfo := range locks {
    fmt.Printf("锁: %s, 拥有者: %s\n", lockInfo.Key, lockInfo.Owner)
}
```

### 强制释放锁（管理员操作）

```go
// 强制释放锁，慎用！
err := lockManager.ForceUnlock(ctx, "my-resource")
if err != nil {
    panic(err)
}
```

## 错误处理

lock包定义了具体的错误类型和错误代码：

```go
if err != nil {
    if lockErr, ok := err.(*lock.LockError); ok {
        switch lockErr.Code {
        case lock.ErrCodeLockTimeout:
            fmt.Println("获取锁超时")
        case lock.ErrCodeLockNotHeld:
            fmt.Println("锁未被持有")
        case lock.ErrCodeLockAlreadyHeld:
            fmt.Println("锁已被持有")
        case lock.ErrCodeLockExpired:
            fmt.Println("锁已过期")
        case lock.ErrCodeInvalidKey:
            fmt.Println("无效的锁键")
        default:
            fmt.Printf("未知锁错误: %v\n", lockErr)
        }
    }
}
```

## 最佳实践

1. **总是使用defer释放锁**：
   ```go
   err := lock.Lock(ctx)
   if err != nil {
       return err
   }
   defer lock.Unlock(ctx)
   ```

2. **设置合理的TTL**：
   - TTL应该大于业务逻辑执行时间
   - 启用自动续期防止意外过期

3. **处理错误**：
   - 检查锁超时错误
   - 处理网络异常

4. **避免死锁**：
   - 设置合理的获取超时
   - 使用TryLock进行非阻塞获取

5. **资源命名**：
   - 使用有意义的锁键名
   - 避免键名冲突

## 性能考虑

- 单个ETCD集群支持数千个并发锁
- 续期操作开销较小
- 网络延迟影响锁性能
- 建议在同一数据中心部署ETCD集群

## 故障恢复

- 锁会在TTL到期后自动释放
- ETCD集群故障时锁操作会失败
- 应用重启后锁会重新初始化
- 网络分区可能导致锁状态不一致 