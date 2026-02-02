# 短链接系统缓存实现说明

## 概述

为短链接系统（shorturl）的 App 层增加了 Redis 缓存功能，以提升短链接解析和域名查询的性能。

## 缓存策略

### 1. 短链接缓存

**缓存键格式**: `shorturl:domain:{domainID}:code:{code}`

**缓存内容**: 完整的 `ShortLink` 对象

**缓存时长**: 5 分钟

**应用场景**: 
- 在 `ResolveShortLink` 方法中，根据域名 ID 和短码查询短链接时使用缓存
- 高频访问的短链接将从缓存中读取，减少数据库查询

**失效策略**:
- 创建短链接后：清除对应缓存（虽然是新建，但清除缓存为了保证一致性）
- 更新短链接后：清除对应缓存
- 更新短链接状态后：清除对应缓存
- 删除短链接后：清除对应缓存

### 2. 域名缓存

**缓存键格式**: `shorturl:domain:host:{host}`

**缓存内容**: 完整的 `ShortDomain` 对象

**缓存时长**: 10 分钟

**应用场景**:
- 在 `resolveDomainWithCache` 方法中，根据 host 查询域名时使用缓存
- 域名配置相对稳定，设置较长的缓存时间

**失效策略**:
- 提供了 `invalidateDomainCache` 方法，在域名更新时可调用

## 实现位置

### App 层 (`system/shorturl/internal/app/link_manage.go`)

**新增方法**:

1. **ResolveShortLink** - 解析短链接（已优化为带缓存）
   - 先通过 `resolveDomainWithCache` 解析域名（带缓存）
   - 再通过缓存查询短链接
   - 最后验证短链接（不缓存验证结果，因为涉及密码等动态验证）

2. **resolveDomainWithCache** - 根据 host 解析域名（带缓存）
   - 使用 `base.Cache.Once` 实现缓存穿透保护
   - 如果找不到对应域名，会返回默认域名

3. **UpdateShortLink** - 更新短链接
   - 更新后自动清除对应缓存

4. **UpdateShortLinkStatus** - 更新短链接状态
   - 更新后自动清除对应缓存

5. **DeleteShortLink** - 删除短链接
   - 删除后自动清除对应缓存

6. **invalidateLinkCache** - 清除短链接缓存（私有方法）
   - 缓存清除失败时记录警告日志，不阻断业务流程

7. **invalidateDomainCache** - 清除域名缓存（私有方法）
   - 缓存清除失败时记录警告日志，不阻断业务流程

### Controller 层 (`system/shorturl/external/http/shorturl_admin_controller.go`)

**修改方法**:

1. **UpdateLink** - 更新短链接
   - 改为调用 App 层的 `UpdateShortLink` 方法
   - 自动处理缓存失效

2. **UpdateLinkStatus** - 更新短链接状态
   - 改为调用 App 层的 `UpdateShortLinkStatus` 方法
   - 自动处理缓存失效

3. **DeleteLink** - 删除短链接
   - 改为调用 App 层的 `DeleteShortLink` 方法
   - 自动处理缓存失效

## 技术细节

### 缓存实现方式

使用 Redis 官方推荐的 `github.com/go-redis/cache/v9` 包，通过 `base.Cache.Once` 方法实现：

```go
err := base.Cache.Once(&cache.Item{
    Key:   cacheKey,
    Value: &result,
    TTL:   5 * time.Minute,
    Do: func(*cache.Item) (interface{}, error) {
        // 缓存未命中时，执行数据库查询
        return dao.FindByXxx(ctx, ...)
    },
})
```

### 缓存优势

1. **缓存穿透保护**: `Cache.Once` 自动处理并发请求，避免缓存击穿
2. **自动序列化**: 自动将 Go 对象序列化为 JSON 存储到 Redis
3. **错误处理**: 缓存失败不影响业务逻辑，降级为直接查询数据库

### 缓存一致性

- **写操作后立即失效**: 所有更新、删除操作后立即清除相关缓存
- **容错设计**: 缓存清除失败时记录日志但不阻断业务
- **不缓存验证结果**: 密码验证、过期检查等动态验证不缓存，保证安全性

## 性能提升

### 预期收益

1. **短链接解析**: 从每次查询数据库（~10ms）优化为缓存读取（~1ms），性能提升约 10 倍
2. **域名解析**: 稳定配置缓存 10 分钟，减少大量重复查询
3. **高并发场景**: 缓存穿透保护机制，避免缓存击穿导致的雪崩

### 适用场景

- 高频访问的短链接（如营销活动、热门内容）
- 域名配置相对稳定的场景
- 读多写少的业务特点

## 监控建议

1. 监控缓存命中率
2. 监控缓存失效日志
3. 定期清理过期缓存
4. 根据实际访问模式调整 TTL

## 后续优化方向

1. **按访问频率动态调整 TTL**: 高频访问的短链接可以缓存更长时间
2. **增加本地缓存**: 在 Redis 缓存之上增加进程内缓存，进一步降低延迟
3. **缓存预热**: 在系统启动时预加载热门短链接
4. **统计缓存命中率**: 增加 Prometheus 指标监控

