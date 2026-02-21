# pkg/executor - 后台任务执行器

## 概述

`pkg/executor` 提供了一个**单 worker 顺序执行**的后台任务执行器，用于将耗时任务从 HTTP 请求链路中剥离，确保任务按提交顺序依次执行（同一时刻最多只有 1 个任务在运行）。

## 核心特性

- ✅ **顺序执行**：单 worker goroutine 确保任务严格按提交顺序依次执行，无并发
- ✅ **状态查询**：Submit 返回 `jobId`，可查询任务状态（pending/running/succeeded/failed/canceled）
- ✅ **超时控制**：每个任务可指定超时时间，超时后自动取消
- ✅ **TraceID 传播**：自动从提交时的 context 中提取 traceId，便于日志串联
- ✅ **优雅关闭**：Stop 时会取消 worker，并将队列中未执行的任务标记为 canceled
- ✅ **历史记录**：保留任务历史（可配置上限），支持按状态过滤查询

## 快速开始

### 1. 初始化与启动（已在 main.go 完成）

```go
// 在 main.go 中已初始化
base.Executor = executor.NewExecutor(executor.DefaultConfig())
err = base.Executor.Start()
if err != nil {
    log.Panic(fmt.Sprintf("启动任务执行器失败: %v", err))
}

// 注册优雅关闭
system.RegisterClose(func() {
    base.Executor.Stop()
})
```

### 2. 提交任务

在业务代码中提交耗时任务：

```go
import (
    "context"
    "time"
    "github.com/xsxdot/aio/base"
)

// 提交任务
jobID, err := base.Executor.Submit(
    ctx,                          // 原始请求 context（会自动提取 traceId）
    "任务名称",                    // 任务名称（用于日志和监控）
    10*time.Minute,               // 超时时间
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
return fiber.Map{
    "message": "任务已提交",
    "job_id": jobID,
}
```

### 3. 查询任务状态

```go
// 查询任务状态
job, err := base.Executor.GetJob(jobID)
if err != nil {
    return err
}

// job 包含以下信息：
// - ID: 任务 ID
// - Name: 任务名称
// - Status: 状态（pending/running/succeeded/failed/canceled）
// - CreatedAt: 创建时间
// - StartedAt: 开始执行时间（nil 表示尚未开始）
// - FinishedAt: 结束时间（nil 表示尚未结束）
// - Error: 错误摘要（只有 failed 状态才有值）
// - TraceID: 追踪 ID
// - Timeout: 超时时间
```

### 4. 管理 API（已注册）

后台管理接口已在 `/admin/executor` 下注册：

- `GET /admin/executor/jobs/:id` - 查询任务状态
- `GET /admin/executor/jobs?status=running&page=1&page_size=20` - 列出任务（可按状态过滤）
- `GET /admin/executor/stats` - 获取统计信息（队列大小、各状态任务数等）

## 配置说明

```go
type Config struct {
    // QueueSize 队列大小（待执行任务数上限）
    QueueSize int `json:"queue_size"`

    // MaxHistorySize 历史记录保留上限（超过后会丢弃最旧的已完成/失败任务）
    MaxHistorySize int `json:"max_history_size"`

    // DefaultTimeout 任务默认超时时间（如果 Submit 时未指定）
    DefaultTimeout time.Duration `json:"default_timeout"`
}
```

默认配置：
- `QueueSize`: 100（队列最多 100 个待执行任务）
- `MaxHistorySize`: 1000（最多保留 1000 个历史记录）
- `DefaultTimeout`: 10 分钟

## 使用示例

### 示例 1：SSL 证书自动部署

```go
// 在 system/ssl/internal/app/certificate_manage.go 中
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

### 示例 2：手动部署并返回 jobId

```go
// 在 system/ssl/external/http/ssl_controller.go 中
func (c *SslController) DeployCertificate(ctx *fiber.Ctx) error {
    // ... 参数解析与校验 ...

    // 提交到后台任务执行器
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

## 任务状态流转

```
pending → running → succeeded
                 ↘ failed
                 ↘ canceled (executor 停止时)
```

## 注意事项

1. **顺序执行**：所有任务严格按提交顺序依次执行，同一时刻最多只有 1 个任务运行。如果需要并发执行，请使用其他方案（如 scheduler 或 goroutine pool）。

2. **队列容量**：队列有容量限制（默认 100），超过后 Submit 会返回错误。如果频繁遇到队列满，可以考虑：
   - 增大 `QueueSize`
   - 优化任务执行时间
   - 使用持久化队列（如 RocketMQ）

3. **历史记录**：历史记录有上限（默认 1000），超过后会自动淘汰最旧的已完成/失败任务。如果需要长期保存任务记录，建议在任务完成后将结果持久化到数据库。

4. **Context 传播**：
   - Submit 时传入的 `ctx` 只用于提取 traceId，不会影响任务执行
   - 任务函数接收的 `execCtx` 是独立的后台 context，不会因为原始请求结束而取消
   - `execCtx` 会在任务超时时自动取消

5. **错误处理**：
   - 任务函数返回 `error` 会被记录到 `job.Error` 字段
   - 任务 panic 会被捕获并记录到日志，任务状态会被标记为 `canceled`

## 与 Scheduler 的区别

| 特性 | Executor | Scheduler |
|------|----------|-----------|
| 执行模式 | 顺序执行（单 worker） | 并发执行（worker pool） |
| 任务类型 | 一次性任务 | 定时/周期/cron 任务 |
| 状态查询 | 支持（jobId） | 支持（taskId） |
| 分布式 | 不支持 | 支持（Redis 锁） |
| 持久化 | 不支持 | 不支持 |
| 适用场景 | 耗时的一次性任务（如部署、导出） | 定时任务（如定时清理、定时同步） |

## 未来扩展

如果需要以下能力，可以考虑扩展：

- **持久化**：将任务状态保存到 Redis/MySQL，支持进程重启后恢复
- **优先级队列**：支持高优先级任务插队
- **并发执行**：支持多 worker 并发执行（需要修改为 worker pool）
- **任务取消**：支持手动取消正在执行或等待中的任务
- **任务重试**：支持失败任务自动重试
- **Webhook 回调**：任务完成后自动调用回调接口

---

**维护者**：基础设施团队  
**最后更新**：2026-02-06

