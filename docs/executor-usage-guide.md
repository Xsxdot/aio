# 后台任务执行器使用指南

## 概述

后台任务执行器（`pkg/executor`）用于将耗时任务从 HTTP 请求链路中剥离，确保任务按提交顺序依次执行。

## 核心特性

- ✅ **顺序执行**：同一时刻最多只有 1 个任务在运行
- ✅ **状态查询**：Submit 返回 jobId，可查询任务状态
- ✅ **TraceID 传播**：自动传播 traceId，便于日志追踪
- ✅ **超时控制**：每个任务可指定超时时间
- ✅ **优雅关闭**：停止时自动取消未执行的任务

## 快速开始

### 1. 提交任务

```go
import (
    "context"
    "time"
    "github.com/xsxdot/aio/base"
)

// 在业务代码中提交耗时任务
jobID, err := base.Executor.Submit(
    ctx,                    // 原始请求 context（会自动提取 traceId）
    "任务名称",              // 任务名称（用于日志和监控）
    10*time.Minute,         // 超时时间
    func(execCtx context.Context) error {
        // 耗时任务逻辑
        // execCtx 是后台 context，不会因为原始请求结束而取消
        // execCtx 中已包含 traceId，可用于日志追踪
        
        // 执行业务逻辑...
        return nil // 返回 nil 表示成功，返回 error 表示失败
    },
)

if err != nil {
    // 队列已满或 executor 未启动
    return err
}

// 返回 jobID 给前端
return result.OK(ctx, fiber.Map{
    "message": "任务已提交",
    "job_id": jobID,
})
```

### 2. 查询任务状态

#### 方式 A：在代码中查询

```go
job, err := base.Executor.GetJob(jobID)
if err != nil {
    return err
}

// job 包含以下信息：
// - ID: 任务 ID
// - Name: 任务名称
// - Status: 状态（pending/running/succeeded/failed/canceled）
// - CreatedAt: 创建时间
// - StartedAt: 开始执行时间
// - FinishedAt: 结束时间
// - Error: 错误摘要
// - TraceID: 追踪 ID
```

#### 方式 B：通过管理 API 查询

```bash
# 查询单个任务状态
curl -H "Authorization: Bearer <admin_token>" \
  http://localhost:9000/admin/executor/jobs/{jobId}

# 列出所有任务（可按状态过滤）
curl -H "Authorization: Bearer <admin_token>" \
  "http://localhost:9000/admin/executor/jobs?status=running&page=1&page_size=20"

# 获取统计信息
curl -H "Authorization: Bearer <admin_token>" \
  http://localhost:9000/admin/executor/stats
```

## 实际案例

### 案例 1：SSL 证书自动部署

**场景**：证书签发成功后，自动部署到匹配的服务器

**实现**：在 `system/ssl/internal/app/certificate_manage.go` 中

```go
// 7. 触发自动部署（根据证书域名自动匹配部署目标）
if req.AutoDeploy {
    certID := certificate.ID
    domain := certificate.Domain
    _, err := base.Executor.Submit(ctx, fmt.Sprintf("自动部署证书[%s]", certificate.Name), 10*time.Minute, func(execCtx context.Context) error {
        // 按证书域名自动匹配部署目标
        targetIDs, err := a.DeployTargetSvc.MatchTargetsByCertificateDomain(execCtx, domain)
        if err != nil {
            return err
        }

        if len(targetIDs) > 0 {
            return a.DeployCertificateToTargets(execCtx, uint(certID), targetIDs, "auto_issue")
        }
        return nil
    })
    if err != nil {
        a.log.WithErr(err).Warn("提交自动部署任务失败")
    }
}
```

**效果**：
- 证书签发接口立即返回，不会因为部署耗时而阻塞
- 部署任务在后台顺序执行，避免并发冲突
- 可以通过 jobId 查询部署进度

### 案例 2：手动部署证书并返回 jobId

**场景**：管理员手动触发证书部署，前端需要知道部署进度

**实现**：在 `system/ssl/external/http/ssl_controller.go` 中

```go
func (c *SslController) DeployCertificate(ctx *fiber.Ctx) error {
    // ... 参数解析与校验 ...

    // 获取证书名称用于任务命名
    cert, err := c.app.GetCertificate(ctx.UserContext(), uint(id))
    if err != nil {
        return err
    }

    // 提交到后台任务执行器
    certID := uint(id)
    targetIDs := req.TargetIDs
    jobID, err := base.Executor.Submit(ctx.UserContext(), fmt.Sprintf("手动部署证书[%s]", cert.Name), 10*time.Minute, func(execCtx context.Context) error {
        return c.app.DeployCertificateToTargets(execCtx, certID, targetIDs, "manual")
    })

    if err != nil {
        return c.err.New("提交部署任务失败", err)
    }

    return result.OK(ctx, fiber.Map{
        "message": "证书部署已提交",
        "job_id":  jobID,
    })
}
```

**前端轮询示例**：

```javascript
// 提交部署任务
const response = await fetch('/admin/ssl/certificates/123/deploy', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ target_ids: [1, 2, 3] })
});

const { job_id } = await response.json();

// 轮询查询任务状态
const pollInterval = setInterval(async () => {
  const jobResponse = await fetch(`/admin/executor/jobs/${job_id}`, {
    headers: { 'Authorization': 'Bearer ' + token }
  });
  
  const job = await jobResponse.json();
  
  if (job.status === 'succeeded') {
    clearInterval(pollInterval);
    console.log('部署成功！');
  } else if (job.status === 'failed') {
    clearInterval(pollInterval);
    console.error('部署失败：', job.error);
  } else if (job.status === 'running') {
    console.log('部署中...');
  }
}, 2000); // 每 2 秒查询一次
```

## 最佳实践

### 1. 任务命名

使用清晰的任务名称，便于日志查询和问题排查：

```go
// ✅ 好的命名
base.Executor.Submit(ctx, fmt.Sprintf("部署证书[%s]到服务器[%s]", certName, serverName), ...)

// ❌ 不好的命名
base.Executor.Submit(ctx, "deploy", ...)
```

### 2. 超时设置

根据任务实际耗时合理设置超时：

```go
// 快速任务（如发送通知）
base.Executor.Submit(ctx, "发送通知", 30*time.Second, ...)

// 中等任务（如部署证书）
base.Executor.Submit(ctx, "部署证书", 10*time.Minute, ...)

// 长时间任务（如数据导出）
base.Executor.Submit(ctx, "导出数据", 30*time.Minute, ...)
```

### 3. 错误处理

任务函数应该返回清晰的错误信息：

```go
base.Executor.Submit(ctx, "部署证书", 10*time.Minute, func(execCtx context.Context) error {
    // ✅ 返回清晰的错误信息
    if err := validateConfig(); err != nil {
        return fmt.Errorf("配置验证失败: %w", err)
    }
    
    if err := deployToServer(); err != nil {
        return fmt.Errorf("部署到服务器失败: %w", err)
    }
    
    return nil
})
```

### 4. 日志追踪

利用 traceId 串联日志：

```go
base.Executor.Submit(ctx, "部署证书", 10*time.Minute, func(execCtx context.Context) error {
    // execCtx 中已包含 traceId，直接使用即可
    log.WithTrace(execCtx).Info("开始部署证书")
    
    // 业务逻辑...
    
    log.WithTrace(execCtx).Info("部署完成")
    return nil
})
```

## 注意事项

### 1. 顺序执行限制

所有任务严格按提交顺序依次执行，同一时刻最多只有 1 个任务运行。如果需要并发执行，请使用其他方案。

### 2. 队列容量

队列有容量限制（默认 100），超过后 Submit 会返回错误。如果频繁遇到队列满：
- 考虑增大 `QueueSize` 配置
- 优化任务执行时间
- 使用持久化队列（如 RocketMQ）

### 3. 历史记录

历史记录有上限（默认 1000），超过后会自动淘汰最旧的已完成/失败任务。如果需要长期保存任务记录，建议在任务完成后将结果持久化到数据库。

### 4. Context 生命周期

- Submit 时传入的 `ctx` 只用于提取 traceId，不会影响任务执行
- 任务函数接收的 `execCtx` 是独立的后台 context，不会因为原始请求结束而取消
- `execCtx` 会在任务超时时自动取消

## 常见问题

### Q1：任务执行失败后会自动重试吗？

A：不会。当前版本不支持自动重试。如果需要重试，可以在任务函数内部实现重试逻辑，或者在失败后重新 Submit。

### Q2：可以取消正在执行的任务吗？

A：不支持手动取消。但任务会在超时时自动取消（通过 context cancel）。

### Q3：如何查看所有正在执行的任务？

A：通过管理 API 查询：
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:9000/admin/executor/jobs?status=running"
```

### Q4：任务执行顺序可以调整吗？

A：不可以。任务严格按提交顺序执行。如果需要优先级队列，需要扩展实现。

### Q5：Executor 与 Scheduler 有什么区别？

| 特性 | Executor | Scheduler |
|------|----------|-----------|
| 执行模式 | 顺序执行（单 worker） | 并发执行（worker pool） |
| 任务类型 | 一次性任务 | 定时/周期/cron 任务 |
| 适用场景 | 耗时的一次性任务 | 定时任务 |

## 相关文档

- [pkg/executor/README.md](../pkg/executor/README.md) - 技术文档
- [pkg/scheduler/README.md](../pkg/scheduler/README.md) - 定时任务调度器文档

---

**最后更新**：2026-02-06

