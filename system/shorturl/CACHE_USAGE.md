# 短链接缓存使用指南

## 快速开始

缓存功能已自动集成到短链接系统中，无需额外配置。所有短链接访问和管理操作都会自动使用缓存。

## 缓存使用示例

### 1. 访问短链接（自动使用缓存）

```go
// Controller 层调用
link, domain, err := c.app.ResolveShortLink(ctx, host, code, password)
```

**缓存行为**:
- 第一次访问：从数据库查询，并写入缓存（5分钟有效期）
- 后续访问：直接从缓存读取，性能提升 10 倍
- 缓存过期：自动重新从数据库加载

### 2. 更新短链接（自动清除缓存）

```go
// App 层方法
req := &UpdateShortLinkRequest{
    TargetConfig: newConfig,
    Comment:      "更新说明",
}
err := app.UpdateShortLink(ctx, linkID, req)
```

**缓存行为**:
- 更新成功后，自动清除对应的短链接缓存
- 下次访问时会重新从数据库加载最新数据

### 3. 删除短链接（自动清除缓存）

```go
err := app.DeleteShortLink(ctx, linkID)
```

**缓存行为**:
- 删除前先查询短链接信息
- 删除成功后，自动清除对应的缓存

## 缓存键说明

### 短链接缓存键

```
shorturl:domain:{domainID}:code:{code}
```

示例：`shorturl:domain:1:code:abc123`

### 域名缓存键

```
shorturl:domain:host:{host}
```

示例：`shorturl:domain:host:s.example.com`

## 手动清除缓存（高级用法）

虽然系统会自动处理缓存失效，但在某些特殊情况下（如数据迁移、手动修复数据等），可能需要手动清除缓存。

### 清除特定短链接缓存

```bash
redis-cli DEL "shorturl:domain:{domainID}:code:{code}"
```

示例：
```bash
redis-cli DEL "shorturl:domain:1:code:abc123"
```

### 清除特定域名缓存

```bash
redis-cli DEL "shorturl:domain:host:{host}"
```

示例：
```bash
redis-cli DEL "shorturl:domain:host:s.example.com"
```

### 清除所有短链接缓存

```bash
redis-cli --scan --pattern "shorturl:*" | xargs redis-cli DEL
```

⚠️ **警告**: 此操作会清除所有短链接相关缓存，请谨慎使用。

## 缓存监控

### 查看缓存内容

```bash
# 查看特定缓存
redis-cli GET "shorturl:domain:1:code:abc123"

# 查看所有短链接缓存键
redis-cli KEYS "shorturl:*"
```

### 查看缓存 TTL

```bash
redis-cli TTL "shorturl:domain:1:code:abc123"
```

返回值说明：
- 正整数：剩余有效时间（秒）
- -1：永不过期
- -2：键不存在

## 性能对比

### 无缓存场景

```
短链接访问 → 查询数据库 → 返回结果
耗时：~10ms
```

### 有缓存场景（命中）

```
短链接访问 → 读取缓存 → 返回结果
耗时：~1ms
```

### 有缓存场景（未命中）

```
短链接访问 → 查询数据库 → 写入缓存 → 返回结果
耗时：~12ms（首次稍慢，后续快速）
```

## 常见问题

### Q1: 缓存不生效怎么办？

**检查步骤**:
1. 确认 Redis 服务正常运行
2. 检查配置文件中 Redis 连接配置
3. 查看应用日志中是否有缓存相关错误

### Q2: 更新短链接后还是返回旧数据？

**可能原因**:
- 缓存清除失败（检查日志）
- 使用了错误的域名/短码组合

**解决方案**:
手动清除对应缓存（参见上文"手动清除缓存"）

### Q3: 如何调整缓存过期时间？

修改 `system/shorturl/internal/app/link_manage.go` 中的 TTL 参数：

```go
// 短链接缓存（当前 5 分钟）
TTL: 5 * time.Minute,

// 域名缓存（当前 10 分钟）
TTL: 10 * time.Minute,
```

根据实际业务需求调整，建议值：
- 高频访问场景：10-30 分钟
- 中频访问场景：5-10 分钟（默认）
- 低频但配置稳定：30-60 分钟

### Q4: 缓存占用内存过大怎么办？

**优化建议**:
1. 降低缓存 TTL
2. 只缓存热点短链接（需要修改代码实现）
3. 配置 Redis 内存淘汰策略（如 `allkeys-lru`）

## 性能调优建议

1. **根据业务特点调整 TTL**
   - 营销活动期间可适当延长缓存时间
   - 测试环境可缩短缓存时间便于验证

2. **监控缓存命中率**
   - 添加 Prometheus 指标
   - 定期分析访问模式

3. **缓存预热**
   - 在系统启动或营销活动前预加载热点短链接

4. **分级缓存**
   - 考虑增加本地内存缓存（如 go-cache）
   - Redis 作为二级缓存

