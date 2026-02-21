# AI 服务集成指南

本指南说明如何在应用中集成和使用 AI 服务统一抽象层。

## 配置

### 1. 配置文件

在 `resources/config.yaml` 中添加 AI 配置：

```yaml
ai:
  providers:
    - name: dashscope_compat
      type: openai_compat
      base_url: https://dashscope.aliyuncs.com/compatible-mode/v1
      api_key: ${DASHSCOPE_API_KEY}
      timeout: 30

    - name: dashscope
      type: dashscope
      base_url: https://dashscope.aliyuncs.com
      api_key: ${DASHSCOPE_API_KEY}
      timeout: 30

  router:
    prefix_mappings:
      qwen: dashscope_compat
    default_provider: dashscope_compat
    enable_auto_retry: true
    max_retry_attempts: 3
```

### 2. 环境变量

设置必要的环境变量：

```bash
export DASHSCOPE_API_KEY=your_dashscope_api_key
export OPENAI_API_KEY=your_openai_api_key  # 如果使用 OpenAI
```

### 3. 在 base 中初始化

在应用启动时初始化 AI 客户端：

```go
import (
    "github.com/xsxdot/aio/pkg/ai"
    "github.com/xsxdot/aio/pkg/core/config"
)

// 在应用初始化中
func InitializeAIClient(cfg *config.AIConfig) error {
    factory := ai.NewFactory()
    client, err := factory.CreateClient(cfg)
    if err != nil {
        return fmt.Errorf("create AI client failed: %w", err)
    }

    // 将客户端保存到 base
    base.AIClient = client
    return nil
}
```

## 使用示例

### 1. 在 Service 中使用

```go
package myservice

import (
    "context"
    "github.com/xsxdot/aio/base"
    "github.com/xsxdot/aio/pkg/ai"
    "github.com/xsxdot/aio/pkg/ai/core"
)

type MyService struct {
    aiClient *ai.Client
}

func NewMyService() *MyService {
    return &MyService{
        aiClient: base.AIClient.(*ai.Client),
    }
}

// 简单对话
func (s *MyService) Chat(ctx context.Context, userMessage string) (string, error) {
    req := &core.ChatRequest{
        Model: "qwen3-vl-plus",
        Messages: []*core.ChatMessage{
            core.NewTextMessage("user", userMessage),
        },
    }

    resp, err := s.aiClient.Chat(ctx, req)
    if err != nil {
        return "", err
    }

    return resp.Content, nil
}

// 多模态对话
func (s *MyService) AnalyzeImage(ctx context.Context, imageURL, question string) (string, error) {
    req := &core.ChatRequest{
        Model: "qwen3-vl-plus",
        Messages: []*core.ChatMessage{
            core.NewMultimodalMessage("user",
                core.NewImageURL(imageURL, nil),
                core.NewTextMedia(question),
            ),
        },
    }

    resp, err := s.aiClient.Chat(ctx, req,
        core.WithTemperature(0.7),
        core.WithMaxTokens(1000),
    )
    if err != nil {
        return "", err
    }

    return resp.Content, nil
}
```

### 2. 流式对话

```go
func (s *MyService) StreamChat(ctx context.Context, userMessage string, onChunk func(string)) error {
    req := &core.ChatRequest{
        Model: "qwen3-vl-plus",
        Messages: []*core.ChatMessage{
            core.NewTextMessage("user", userMessage),
        },
    }

    stream, err := s.aiClient.ChatStream(ctx, req)
    if err != nil {
        return err
    }
    defer stream.Close()

    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        if chunk.Delta != "" {
            onChunk(chunk.Delta)
        }
    }

    return nil
}
```

### 3. 异步任务（视频合成）

```go
func (s *MyService) SynthesizeVideo(ctx context.Context, imageURL, videoURL string) (string, error) {
    // 提交任务
    jobReq := &core.AsyncJobRequest{
        Operation: core.OpImage2Video,
        Model:     "wan2.2-animate-mix",
        Input: map[string]interface{}{
            "image_url": imageURL,
            "video_url": videoURL,
        },
        Parameters: map[string]interface{}{
            "mode": "wan-std",
        },
    }

    handle, err := s.aiClient.SubmitAsyncJob(ctx, jobReq)
    if err != nil {
        return "", err
    }

    // 等待任务完成
    result, err := s.aiClient.WaitAsyncJob(ctx, handle, nil)
    if err != nil {
        return "", err
    }

    if !result.IsSuccess() {
        return "", result.Error
    }

    videoURL, ok := result.Result["video_url"].(string)
    if !ok {
        return "", fmt.Errorf("video_url not found in result")
    }

    return videoURL, nil
}
```

### 4. WebSocket 双工（TTS）

```go
func (s *MyService) TextToSpeech(ctx context.Context, texts []string, outputPath string) error {
    // 创建双工会话
    req := &core.DuplexRequest{
        Operation: core.OpTTS,
        Model:     "cosyvoice-v3-flash",
        Parameters: map[string]interface{}{
            "text_type":   "PlainText",
            "voice":       "longanyang",
            "format":      "mp3",
            "sample_rate": 22050,
            "volume":      50,
        },
    }

    session, err := s.aiClient.RunDuplex(ctx, req)
    if err != nil {
        return err
    }
    defer session.Close()

    // 创建输出文件
    outputFile, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer outputFile.Close()

    // 等待会话启动
    started := false
    go func() {
        for {
            msg, err := session.Recv()
            if err != nil {
                return
            }

            if msg.IsEvent() {
                if msg.Event.Type == core.DuplexEventTaskStarted {
                    started = true
                    
                    // 发送文本
                    for _, text := range texts {
                        session.Send(map[string]interface{}{"text": text})
                    }
                }
            } else if msg.IsBinary() {
                // 写入音频数据
                outputFile.Write(msg.Binary)
            }
        }
    }()

    // 等待启动
    for !started {
        time.Sleep(100 * time.Millisecond)
    }

    return nil
}
```

### 5. 错误处理

```go
func (s *MyService) ChatWithErrorHandling(ctx context.Context, userMessage string) (string, error) {
    req := &core.ChatRequest{
        Model: "qwen3-vl-plus",
        Messages: []*core.ChatMessage{
            core.NewTextMessage("user", userMessage),
        },
    }

    resp, err := s.aiClient.Chat(ctx, req)
    if err != nil {
        // 检查是否是 AI 错误
        if aiErr, ok := err.(*core.AIError); ok {
            // 根据错误码处理
            switch aiErr.Code {
            case core.ErrCodeInvalidAPIKey:
                return "", fmt.Errorf("API Key 无效，请检查配置")
            case core.ErrCodeRateLimitExceeded:
                // 可以等待后重试
                return "", fmt.Errorf("请求频率超限，请稍后重试")
            case core.ErrCodeQuotaExceeded:
                return "", fmt.Errorf("配额已用完")
            default:
                // 检查是否可重试
                if aiErr.IsRetryable() {
                    // 可以实现重试逻辑
                    return "", fmt.Errorf("请求失败但可重试: %v", aiErr)
                }
                return "", fmt.Errorf("请求失败: %v", aiErr)
            }
        }
        return "", err
    }

    return resp.Content, nil
}
```

## 最佳实践

### 1. 配置管理

- 敏感信息（API Key）使用环境变量
- 模型映射和路由策略放在配置文件
- 不同环境使用不同配置

### 2. 错误处理

- 始终检查 `AIError` 类型
- 根据 `RetryClass` 决定是否重试
- 记录 `RequestID` 用于追踪

### 3. 超时控制

- 使用 `context.WithTimeout` 控制请求超时
- 异步任务设置合理的轮询间隔
- WebSocket 会话设置心跳检测

### 4. 资源清理

- 流式响应使用 `defer stream.Close()`
- WebSocket 会话使用 `defer session.Close()`
- 大文件处理注意内存管理

### 5. 监控与日志

- 记录所有 AI 请求的模型、耗时、Token 使用
- 监控错误率和重试率
- 追踪费用消耗

## 常见问题

### Q: 如何添加新的供应商？

A: 在配置文件中添加新的 provider 配置，目前支持 `openai_compat` 和 `dashscope` 两种类型。

### Q: 如何实现自动降级？

A: 在路由器配置中设置 `fallback_providers`，当主供应商失败时会自动尝试备用供应商。

### Q: 如何控制重试策略？

A: 通过 `enable_auto_retry` 和 `max_retry_attempts` 配置，也可以使用 `router.Retryer` 自定义重试逻辑。

### Q: 如何获取原始响应？

A: 所有响应对象都包含 `Raw json.RawMessage` 字段，保留完整的原始 JSON 响应。

## 参考

- [pkg/ai/README.md](../pkg/ai/README.md) - 架构详细说明
- [ali.md](../ali.md) - 通义千问 API 示例
