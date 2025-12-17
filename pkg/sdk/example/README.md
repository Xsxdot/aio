# SDK Example

这是一个最小的 SDK 使用示例，演示了完整的工作流程。

## 运行要求

1. 注册中心服务运行在 `localhost:50051`（或通过 `REGISTRY_ADDR` 环境变量指定）
2. 有效的客户端凭证（通过环境变量 `CLIENT_KEY` 和 `CLIENT_SECRET` 提供）

## 运行方式

```bash
# 设置环境变量
export CLIENT_KEY="your-client-key"
export CLIENT_SECRET="your-client-secret"
export REGISTRY_ADDR="localhost:50051"  # 可选，默认 localhost:50051

# 运行示例
go run pkg/sdk/example/main.go
```

## 示例流程

1. **认证**：使用 ClientKey/ClientSecret 获取 JWT token
2. **拉取服务列表**：调用 `ListServices` 获取 aio 项目下的所有服务
3. **服务发现**：使用 round-robin 选择健康的实例
4. **注册自身**：注册到注册中心并保持心跳
5. **运行**：持续运行直到收到退出信号（Ctrl+C）
6. **清理**：主动注销实例并退出

## 预期输出

```
Creating SDK client (registry: localhost:50051)...
SDK client created successfully

=== Step 1: Authentication ===
Token obtained: eyJhbGciOiJIUzI1NiIs...

=== Step 2: List Services ===
Found 2 services:
  - aio/aio: 1 instances
    * http://localhost:9000 (dev)

=== Step 3: Service Discovery ===
  Round 1: picked http://localhost:9000
  Round 2: picked http://localhost:9000
  Round 3: picked http://localhost:9000

=== Step 4: Register Self ===
Successfully registered as: sdk-example-1234567890
Heartbeat is running in background...

=== Step 5: Running (Press Ctrl+C to stop) ===
[10:30:00] Still running, heartbeat active...
[10:30:10] Still running, heartbeat active...
^C
Received signal: interrupt
Deregistering...
Deregistered successfully

=== Cleanup ===
Example completed
```
