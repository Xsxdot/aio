# SDK 自动注册流程更新说明

## 更新日期
2026-02-01

## 更新原因
原有的 `EnableSDKAndRegisterSelf` 方法错误地要求预先配置 `ServiceID`，这要求服务在注册前必须在注册中心手动创建。这不符合实际使用场景，正确的做法应该使用 `RegisterSelfWithEnsureService` 方法，该方法会自动创建或获取服务，无需预先配置 `ServiceID`。

## 主要变更

### 1. 配置结构变更 (`pkg/core/config/sdk.go`)

**变更前：**
```go
type SdkRegisterConfig struct {
    ServiceID int64 `yaml:"service_id"`  // ❌ 需要预先创建服务
    InstanceKey string `yaml:"instance_key"`
    Env string `yaml:"env"`
    Host string `yaml:"host"`
    Endpoint string `yaml:"endpoint"`
    MetaJSON string `yaml:"meta_json"`
    TTLSeconds int64 `yaml:"ttl_seconds"`
}
```

**变更后：**
```go
type SdkRegisterConfig struct {
    // ========== 服务配置（EnsureService 使用） ==========
    Project string `yaml:"project"`           // ✅ 新增：项目名称
    Name string `yaml:"name"`                 // ✅ 新增：服务名称
    Owner string `yaml:"owner"`               // ✅ 新增：服务负责人
    Description string `yaml:"description"`   // ✅ 新增：服务描述
    SpecJSON string `yaml:"spec_json"`        // ✅ 新增：服务规格

    // ========== 实例配置（RegisterInstance 使用） ==========
    InstanceKey string `yaml:"instance_key"`
    Env string `yaml:"env"`
    Host string `yaml:"host"`
    Endpoint string `yaml:"endpoint"`
    MetaJSON string `yaml:"meta_json"`
    TTLSeconds int64 `yaml:"ttl_seconds"`
}
```

**主要区别：**
- ❌ 移除了 `ServiceID` 字段（不再需要预先创建服务）
- ✅ 新增服务配置字段：`Project`、`Name`、`Owner`、`Description`、`SpecJSON`
- ✅ 实例配置字段保持不变

### 2. 注册方法变更 (`pkg/core/start/config.go`)

**变更前：**
```go
func (c *Configures) EnableSDKAndRegisterSelf() (*sdk.Client, *sdk.RegistrationHandle) {
    // 检查 ServiceID（必须预先配置）
    if c.Config.Sdk.Register.ServiceID == 0 {
        c.Logger.Panic("sdk.register.service_id is required")
    }

    // 直接注册实例
    req := &sdk.RegisterInstanceRequest{
        ServiceID: c.Config.Sdk.Register.ServiceID,
        // ... 其他字段
    }

    handle, err := client.Registry.RegisterSelf(ctx, req)
    // ...
}
```

**变更后：**
```go
func (c *Configures) EnableSDKAndRegisterSelf() (*sdk.Client, *sdk.RegistrationHandle) {
    // 检查服务必填字段
    if c.Config.Sdk.Register.Project == "" {
        c.Logger.Panic("sdk.register.project is required")
    }
    if c.Config.Sdk.Register.Name == "" {
        c.Logger.Panic("sdk.register.name is required")
    }
    if c.Config.Sdk.Register.Owner == "" {
        c.Logger.Panic("sdk.register.owner is required")
    }

    // 准备服务确保请求
    svcReq := &sdk.EnsureServiceRequest{
        Project:     c.Config.Sdk.Register.Project,
        Name:        c.Config.Sdk.Register.Name,
        Owner:       c.Config.Sdk.Register.Owner,
        Description: c.Config.Sdk.Register.Description,
        SpecJSON:    c.Config.Sdk.Register.SpecJSON,
    }

    // 准备实例注册请求
    instReq := &sdk.RegisterInstanceRequest{
        // ServiceID 由 RegisterSelfWithEnsureService 自动填充
        // ... 其他字段
    }

    // 使用完整注册流程
    handle, err := client.Registry.RegisterSelfWithEnsureService(ctx, svcReq, instReq)
    // ...
}
```

**主要区别：**
- ❌ 不再依赖预先配置的 `ServiceID`
- ✅ 使用 `EnsureServiceRequest` 自动创建或获取服务
- ✅ 使用 `RegisterSelfWithEnsureService` 完成完整注册流程
- ✅ 超时时间从 10s 增加到 15s（因为包含了 EnsureService 步骤）

### 3. 配置文件变更

**变更前：**
```yaml
sdk:
  registry_addr: "localhost:50051"
  client_key: "your-client-key"
  client_secret: "your-client-secret"
  register:
    service_id: 1  # ❌ 必须预先在注册中心创建服务
    instance_key: ""
    env: ""
    host: ""
    endpoint: ""
    meta_json: "{}"
    ttl_seconds: 60
```

**变更后：**
```yaml
sdk:
  registry_addr: "localhost:50051"
  client_key: "your-client-key"
  client_secret: "your-client-secret"
  register:
    # ========== 服务配置 ==========
    project: "aio"              # ✅ 项目名称（必填）
    name: "xiaozhizhang-web"    # ✅ 服务名称（必填）
    owner: "devops"             # ✅ 服务负责人（必填）
    description: "小知章 Web 服务"  # ✅ 服务描述（可选）
    spec_json: '{"type":"web","framework":"fiber"}'  # ✅ 服务规格（可选）
    
    # ========== 实例配置 ==========
    instance_key: ""
    env: ""
    host: ""
    endpoint: ""
    meta_json: '{"version":"1.0.0"}'
    ttl_seconds: 60
```

## 迁移指南

### 如果你之前使用了 SDK 自动注册

**步骤 1：** 更新配置文件，移除 `service_id`，添加服务配置字段：

```yaml
sdk:
  register:
    # 移除这个
    # service_id: 1  ❌
    
    # 添加这些
    project: "your-project"           # ✅
    name: "your-service-name"         # ✅
    owner: "team-name"                # ✅
    description: "服务描述"            # ✅（可选）
    spec_json: '{"type":"api"}'       # ✅（可选）
```

**步骤 2：** 重启服务，注册流程会自动工作，无需在注册中心预先创建服务

### 如果你只使用 SDK 客户端（不使用自动注册）

无需任何修改，SDK 客户端创建 (`EnableSDK`) 不受影响。

## 优势

### 变更前的问题
1. ❌ 必须在注册中心手动创建服务后才能注册实例
2. ❌ ServiceID 硬编码在配置中，不同环境需要不同的 ID
3. ❌ 服务创建和实例注册是两个独立的步骤，容易出错
4. ❌ 无法自动处理服务不存在的情况

### 变更后的优势
1. ✅ 自动创建或获取服务，无需手动操作
2. ✅ 使用语义化的服务标识（project + name），更易理解
3. ✅ 完整的注册流程一步完成，更可靠
4. ✅ 支持服务的完整描述（owner、description、spec）

## 测试验证

参考测试文件：`pkg/sdk/example/sdk_full_integration_test.go` 中的 `Registry_RegisterSelfWithEnsureService_Heartbeat_Stop` 测试用例

```go
// 准备服务请求（无需预先创建服务）
svcReq := &sdk.EnsureServiceRequest{
    Project:     "aio",
    Name:        "test-service",
    Owner:       "devops",
    Description: "测试服务",
    SpecJSON:    `{"type":"test"}`,
}

// 准备实例请求
instReq := &sdk.RegisterInstanceRequest{
    InstanceKey: "test-instance",
    Env:         "dev",
    Host:        "localhost",
    Endpoint:    "http://localhost:8080",
    MetaJSON:    `{"version":"1.0.0"}`,
    TTLSeconds:  60,
}

// 完整注册流程
handle, err := client.Registry.RegisterSelfWithEnsureService(ctx, svcReq, instReq)
if err != nil {
    panic(err)
}
defer handle.Stop()  // 退出时自动注销
```

## 相关文件

### 修改的文件
- `pkg/core/config/sdk.go` - 配置结构定义
- `pkg/core/start/config.go` - 注册方法实现
- `resources/config.yaml.example` - 配置模板
- `resources/dev.yaml` - 开发环境配置
- `resources/test.yaml` - 测试环境配置
- `resources/prod.yaml` - 生产环境配置

### 参考文件
- `pkg/sdk/example/sdk_full_integration_test.go` - 集成测试（正确的注册流程示例）
- `pkg/sdk/registry.go` - SDK 注册相关方法
- `system/registry/` - 注册中心服务实现

## 兼容性说明

### ⚠️ 破坏性变更
本次更新是**破坏性变更**，需要修改配置文件：

- 移除 `sdk.register.service_id` 配置项
- 添加 `sdk.register.project`、`sdk.register.name`、`sdk.register.owner` 配置项

### 影响范围
仅影响使用 `EnableSDKAndRegisterSelf` 方法的服务，不影响：
- 只使用 `EnableSDK` 的服务（纯客户端）
- 不使用 SDK 的服务

## 总结

这次更新将 SDK 自动注册从"需要预先创建服务"的模式改为"自动创建或获取服务"的模式，大大简化了服务注册流程，使其更符合实际使用场景。

**核心变化：**
- ServiceID（数字ID） → Project + Name（语义化标识）
- RegisterSelf（仅注册实例） → RegisterSelfWithEnsureService（完整注册流程）
- 手动创建服务 → 自动创建/获取服务

