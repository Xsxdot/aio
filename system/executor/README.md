# Executor - 远程任务执行中心

## 概述

Executor 是一个基于 MySQL 的分布式任务调度和执行系统，支持任务持久化、延时执行、失败重试、租约管理等功能。

### 核心特性

- **任务持久化**：所有任务存储在 MySQL 中，确保可靠性
- **租约机制**：使用数据库行锁 + 租约确保任务不会被重复执行
- **失败重试**：支持自动重试，带指数退避和抖动
- **延时执行**：支持指定任务的执行时间
- **幂等保证**：通过 `dedup_key` 实现任务去重
- **审计追踪**：记录每次任务执行尝试的详细信息
- **分布式支持**：多实例安全竞争领取任务

## 架构设计

### 任务状态流转

```
[创建] -> pending -> running -> succeeded
                  -> running -> pending (租约过期/失败重试)
                  -> running -> dead (超过最大重试次数)
pending/running -> canceled (手动取消)
dead/failed/canceled -> pending (手动重新入队)
```

### 数据模型

#### 任务主表 (`executor_jobs`)

- **路由信息**：`target_service`、`method`、`args_json`
- **调度信息**：`status`、`priority`、`next_run_at`
- **重试信息**：`max_attempts`、`attempts`
- **租约信息**：`lease_owner`、`lease_until`
- **幂等信息**：`dedup_key`
- **结果信息**：`last_error`、`result_json`

#### 任务尝试记录表 (`executor_job_attempts`)

记录每次任务执行的详细信息，用于审计和排障。

## 使用指南

### 1. 提交任务（客户端）

```go
import "github.com/xsxdot/aio/app"

func submitTask(appRoot *app.App) {
    ctx := context.Background()
    
    // 提交任务
    jobID, err := appRoot.ExecutorModule.Client.SubmitJob(
        ctx,
        base.ENV,                 // env
        "user-service",           // 目标服务名
        "SendEmailNotification",  // 方法名
        `{"user_id": 123}`,       // 参数 JSON
        0,                        // 立即执行（或使用 Unix 时间戳延时执行）
        3,                        // 最大重试次数
        0,                        // 优先级（0为默认）
        "email:123:signup",       // 幂等键（必填）
        "",                       // 重试策略：exponential（默认）| fixed
        0,                        // 固定间隔秒数（仅 fixed 时有效）
        "",                       // 顺序键（同 key 任务串行，空表示不限制）
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("任务已提交，ID: %d", jobID)
}
```

### 2. Worker 侧拉取并执行任务

#### 2.1 使用 gRPC 客户端

```go
import (
    pb "github.com/xsxdot/aio/system/executor/api/proto"
    "google.golang.org/grpc"
)

func workerLoop() {
    // 连接到 gRPC 服务器
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    client := pb.NewExecutorServiceClient(conn)
    ctx := context.Background()
    
    // Worker 主循环
    for {
        // 1. 领取任务（可选按 method 过滤）
        resp, err := client.AcquireJob(ctx, &pb.AcquireJobRequest{
            TargetService:  "user-service",
            Method:         "",  // 可选：指定方法名过滤，空表示领取所有方法的任务
            ConsumerId:     "worker-1",
            LeaseDuration:  30, // 30秒租约
        })
        
        if err != nil {
            log.Printf("领取任务失败: %v", err)
            time.Sleep(5 * time.Second)
            continue
        }
        
        // 没有任务，等待后重试
        if resp.JobId == 0 {
            time.Sleep(5 * time.Second)
            continue
        }
        
        // 2. 执行任务
        log.Printf("开始执行任务: %d, 方法: %s", resp.JobId, resp.Method)
        
        // 根据 method 路由到对应的处理函数
        err = executeMethod(resp.Method, resp.ArgsJson)
        
        // 3. 确认任务结果
        var status pb.JobStatus
        var errorMsg string
        
        if err != nil {
            status = pb.JobStatus_JOB_STATUS_FAILED
            errorMsg = err.Error()
        } else {
            status = pb.JobStatus_JOB_STATUS_SUCCEEDED
        }
        
        _, err = client.AckJob(ctx, &pb.AckJobRequest{
            JobId:      resp.JobId,
            AttemptNo:  resp.AttemptNo,
            ConsumerId: "worker-1",
            Status:     status,
            Error:      errorMsg,
        })
        
        if err != nil {
            log.Printf("确认任务失败: %v", err)
        }
    }
}

// 方法路由表
func executeMethod(method, argsJSON string) error {
    switch method {
    case "SendEmailNotification":
        return sendEmailNotification(argsJSON)
    case "ProcessPayment":
        return processPayment(argsJSON)
    default:
        return fmt.Errorf("未知方法: %s", method)
    }
}
```

#### 2.2 按方法过滤领取任务

从 v1.1 开始，支持在领取任务时按 `method` 精确过滤：

```go
// 示例 1: 创建只处理特定方法的 Worker
resp, err := client.AcquireJob(ctx, &pb.AcquireJobRequest{
    TargetService:  "user-service",
    Method:         "SendEmailNotification",  // 只领取邮件通知任务
    ConsumerId:     "email-worker-1",
    LeaseDuration:  30,
})

// 示例 2: 创建处理所有方法的 Worker
resp, err := client.AcquireJob(ctx, &pb.AcquireJobRequest{
    TargetService:  "user-service",
    Method:         "",  // 空表示不过滤，领取所有方法
    ConsumerId:     "general-worker-1",
    LeaseDuration:  30,
})
```

**使用场景：**
- 不同 Worker 专门处理不同类型的任务（如邮件 Worker、支付 Worker）
- 根据任务复杂度和资源需求分配不同的 Worker 实例
- 提高任务处理效率，避免不必要的方法路由判断

#### 2.3 长任务续租

```go
// 对于执行时间较长的任务，需要定期续租
func executeWithHeartbeat(client pb.ExecutorServiceClient, jobID int64, attemptNo int32) {
    // 启动心跳协程
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go func() {
        ticker := time.NewTicker(15 * time.Second) // 每15秒续租一次
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                _, err := client.RenewLease(ctx, &pb.RenewLeaseRequest{
                    JobId:          jobID,
                    AttemptNo:      attemptNo,
                    ConsumerId:     "worker-1",
                    ExtendDuration: 30, // 延长30秒
                })
                if err != nil {
                    log.Printf("续租失败: %v", err)
                }
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // 执行实际任务...
    doLongRunningTask()
}
```

### 3. 管理接口（HTTP）

所有管理接口都需要管理员权限。

#### 3.1 提交任务

```bash
POST /admin/executor/jobs
Content-Type: application/json

{
  "target_service": "user-service",
  "method": "SendEmailNotification",
  "args_json": "{\"user_id\": 123}",
  "run_at": 0,
  "max_attempts": 3,
  "priority": 0,
  "dedup_key": "email:123:signup"
}
```

#### 3.2 查询任务列表

```bash
GET /admin/executor/jobs?target_service=user-service&status=pending&page_num=1&page_size=20
```

#### 3.3 查询任务详情

```bash
GET /admin/executor/jobs/:id
```

#### 3.4 查询任务执行历史

```bash
GET /admin/executor/jobs/:id/attempts
```

#### 3.5 取消任务

```bash
POST /admin/executor/jobs/:id/cancel
```

#### 3.6 重新入队任务

```bash
POST /admin/executor/jobs/:id/requeue
Content-Type: application/json

{
  "run_at": 0  # 0表示立即执行
}
```

#### 3.7 获取统计信息

```bash
GET /admin/executor/stats
```

响应示例：

```json
{
  "queue_length": 120,
  "pending_count": 100,
  "running_count": 20,
  "succeeded_count": 5000,
  "failed_count": 50,
  "canceled_count": 10,
  "dead_count": 5,
  "due_count": 80,
  "retry_distribution": {
    "1": 4500,
    "2": 400,
    "3": 100
  }
}
```

#### 3.8 清理旧任务

```bash
POST /admin/executor/cleanup
Content-Type: application/json

{
  "succeeded_days": 7,   # 清理7天前的已成功任务
  "canceled_days": 30,   # 清理30天前的已取消任务
  "dead_days": 90        # 清理90天前的死信任务
}
```

## 运维指南

### 1. 监控指标

- `queue_length`: 待处理任务数（pending 状态）
- `due_count`: 到期可执行任务数
- `running_count`: 执行中任务数
- `succeeded_count`: 成功任务总数
- `failed_count`: 失败任务总数
- `dead_count`: 死信任务数（超过最大重试次数）
- `retry_distribution`: 重试次数分布

### 2. 周期性清理

系统已自动注册每天凌晨 3:00 执行的清理任务（见 `main.go`）：

- 清理 7 天前的已成功任务
- 清理 30 天前的已取消任务
- 清理 90 天前的死信任务

可根据实际需求调整清理策略。

### 3. 性能优化

#### 数据库索引

系统已自动创建以下索引：

- `idx_target_status_next`: `(target_service, status, next_run_at)` - 用于任务领取
- `idx_lease_owner`: `(lease_owner)` - 用于检查 consumer 是否有未完成任务
- `idx_lease_until`: `(lease_until)` - 用于租约过期检查
- `idx_dedup_key`: `(dedup_key)` - 唯一索引，用于去重
- `idx_status`: `(status)` - 用于按状态查询
- `idx_priority`: `(priority)` - 用于优先级排序
- `idx_next_run`: `(next_run_at)` - 用于延时任务查询

#### 调优建议

1. **租约时长**：根据任务平均执行时间设置，建议设置为任务执行时间的 2-3 倍
2. **心跳间隔**：建议设置为租约时长的 1/2
3. **Worker 数量**：根据任务量和处理速度调整，避免过多 worker 导致数据库竞争
4. **清理策略**：根据业务需求和存储容量调整清理周期

### 4. 故障排查

#### 任务一直处于 running 状态

原因：Worker 崩溃或网络故障导致租约未释放

解决：
1. 检查 `lease_until` 是否已过期
2. 如果过期，任务会自动被其他 worker 领取
3. 如果未过期，等待租约过期或手动取消后重新入队

#### 任务变成 dead 状态

原因：超过最大重试次数

解决：
1. 查看 `last_error` 了解失败原因
2. 修复问题后，使用"重新入队"功能重新执行

#### 任务执行重复

原因：幂等性未正确实现

解决：
1. 使用 `dedup_key` 在提交时去重
2. 在业务层实现幂等性（如：检查是否已处理）

## 最佳实践

### 1. 幂等性设计

```go
// 使用幂等键防止重复提交
dedupKey := fmt.Sprintf("order:payment:%s", orderID)

jobID, err := client.SubmitJob(
    ctx,
    "payment-service",
    "ProcessPayment",
    argsJSON,
    0, 3, 0,
    dedupKey,  // 相同订单的支付任务不会重复创建
)
```

### 2. 方法路由表

```go
// 在 Worker 侧维护方法路由表
var methodHandlers = map[string]func(string) error{
    "SendEmailNotification": sendEmailNotification,
    "ProcessPayment":        processPayment,
    "GenerateReport":        generateReport,
}

func executeMethod(method, argsJSON string) error {
    handler, ok := methodHandlers[method]
    if !ok {
        return fmt.Errorf("未知方法: %s", method)
    }
    return handler(argsJSON)
}
```

### 3. 错误处理与重试策略

```go
// 区分可重试和不可重试的错误
func processPayment(argsJSON string) error {
    err := doPayment(argsJSON)
    
    if err != nil {
        // 不可重试的错误（如：参数错误），直接返回成功避免重试
        if isValidationError(err) {
            log.Printf("参数错误，不重试: %v", err)
            return nil
        }
        
        // 可重试的错误（如：网络超时、第三方服务暂时不可用）
        return err
    }
    
    return nil
}
```

### 4. 监控告警

建议监控以下指标并设置告警：

- `due_count` 持续增长：Worker 处理能力不足
- `dead_count` 增长：任务失败率高，需要排查原因
- `running_count` 持续高位：可能有任务卡住或租约未释放
- `queue_length` 超过阈值：任务积压，需要扩容 Worker

## 注意事项

1. **数据库性能**：高并发场景下注意数据库连接池配置和索引优化
2. **租约管理**：确保租约时长大于任务平均执行时间
3. **心跳机制**：长任务务必实现心跳续租
4. **幂等性**：业务侧必须实现幂等性，系统无法完全保证 exactly-once
5. **错误处理**：区分可重试和不可重试的错误，避免无意义的重试
6. **资源清理**：定期清理旧任务，避免表过大影响性能

## 后续扩展

可根据业务需求扩展以下功能：

- 任务优先级队列优化
- 任务依赖（DAG 工作流）
- 任务分片（大任务拆分成多个子任务并行执行）
- 更丰富的重试策略（如：固定间隔、指数退避、自定义退避函数）
- 任务结果通知（Webhook、消息队列）
- 可视化监控面板
