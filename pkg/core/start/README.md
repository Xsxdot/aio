# pkg/core/start - 通用启动配置框架

`pkg/core/start` 提供了一套可复用的应用启动配置框架，支持本地 YAML 文件和配置中心（config-center）两种配置来源，并通过 **泛型配置加载器** 让每个项目自定义扩展配置，同时复用通用基础设施初始化能力。

---

## 核心概念

### 1. 配置结构 `start.Config`

`start.Config` 包含了所有通用的基础设施配置段：

```go
type Config struct {
    ConfigSource string                    `yaml:"config-source"` // "file" 或 "config-center"
    AppName      string                    `yaml:"app-name"`
    Env          string                    `yaml:"env"`
    Host         string                    `yaml:"host"`
    Port         int                       `yaml:"port"`
    Domain       string                    `yaml:"domain"`
    Jwt          config.JwtConfig          `yaml:"jwt"`
    Redis        config.RedisConfig        `yaml:"redis"`
    Database     config.Database           `yaml:"db"`
    Oss          config.OssConfig          `yaml:"oss"`
    ConfigCenter config.ConfigCenterConfig `yaml:"config"`
    Wechat       config.WechatConfig       `yaml:"wechat"`
    Proxy        config.ProxyConfig        `yaml:"proxy"`
    GRPC         config.GRPCConfig         `yaml:"grpc"`
    Server       config.ServerConfig       `yaml:"server"`
    Sdk          config.SdkConfig          `yaml:"sdk"`
}
```

### 2. 基础实例容器 `start.Configures`

`start.Configures` 负责创建和管理通用基础设施实例：

```go
type Configures struct {
    Config    Config
    Logger    *logger.Log
    AdminAuth *security.AdminAuth
    UserAuth  *security.UserAuth
}

// 提供的通用能力（Enable 系列方法）
- EnableAdminAuth() *security.AdminAuth
- EnableUserAuth() *security.UserAuth
- EnableRedis() *redis.Client
- EnableCache(*redis.Client) *cache.Cache
- EnableLocker(*redis.Client) *redislock.Client
- EnablePg() *gorm.DB
- EnableMysql() *gorm.DB
- EnableSDK() *sdk.Client
- EnableSDKAndRegisterSelf() (*sdk.Client, *sdk.RegistrationHandle)
```

---

## 使用方式

### 方式一：标准用法（当前仓库）

如果你的项目不需要额外的配置段，直接使用 `NewConfigures`：

```go
// main.go
file, _ := os.ReadFile("resources/dev.yaml")
configures := start.NewConfigures(file, "dev")

// 使用基础实例
base.Logger = configures.Logger
base.DB = configures.EnableMysql()
base.RDB = configures.EnableRedis()
// ...
```

### 方式二：扩展配置（推荐用于其他项目）

如果你的项目需要额外的配置段（如 `rocketmq`、`kafka`、业务开关等），使用 **泛型配置加载器**：

#### 步骤 1：定义扩展配置结构体

```go
// 在你的项目中定义（例如 internal/config/app_config.go）
package config

import (
    "github.com/xsxdot/aio/pkg/core/start"
    "github.com/xsxdot/aio/pkg/core/config"
)

type AppConfig struct {
    start.Config `yaml:",inline"` // ✅ 必须内嵌 start.Config（使用 inline 标签）

    // ✅ 你的自定义配置段
    RocketMQ config.RocketmqConfig `yaml:"rocketmq"`
    Kafka    KafkaConfig            `yaml:"kafka"`
    Feature  FeatureConfig          `yaml:"feature"`
}

type KafkaConfig struct {
    Brokers []string `yaml:"brokers"`
    Topic   string   `yaml:"topic"`
}

type FeatureConfig struct {
    EnableNewUI bool `yaml:"enable_new_ui"`
    MaxWorkers  int  `yaml:"max_workers"`
}
```

#### 步骤 2：加载配置并初始化

```go
// main.go
file, _ := os.ReadFile("resources/dev.yaml")

// ✅ 使用泛型加载器加载扩展配置
cfg := start.MustLoadConfig[config.AppConfig](file, "dev")

// ✅ 创建通用基础实例容器
configures := start.NewConfiguresFromConfig(cfg.Config)

// ✅ 访问通用配置
base.Logger = configures.Logger
base.DB = configures.EnableMysql()
base.RDB = configures.EnableRedis()

// ✅ 访问扩展配置
rocketMQProducer := initRocketMQ(cfg.RocketMQ)
kafkaConsumer := initKafka(cfg.Kafka)

if cfg.Feature.EnableNewUI {
    // 启用新 UI
}
```

---

## 配置来源：本地文件 vs 配置中心

### 本地文件模式（默认）

**配置文件示例**（`resources/dev.yaml`）：

```yaml
config-source: file  # 或不指定（默认为 file）
app-name: my-service
env: dev
port: 9000

jwt:
  secret: "your-secret"
  admin_secret: "your-admin-secret"
  expire_time: 24

db:
  host: localhost
  port: 5432
  user: postgres
  password: password
  dbname: mydb

# ✅ 你的扩展配置段
rocketmq:
  name_server: "localhost:9876"
  topic: "my-topic"

kafka:
  brokers: ["localhost:9092"]
  topic: "my-kafka-topic"
```

### 配置中心模式

**本地启动文件**（`resources/dev.yaml`）：

```yaml
config-source: config-center  # ✅ 指定从配置中心加载

sdk:
  registry_addr: "localhost:50051"
  client_key: "your-client-key"
  client_secret: "your-client-secret"
  bootstrap_config_prefix: "myapp.config"  # ✅ 配置前缀
```

**配置中心存储格式**：

配置中心会按以下规则加载和组装配置：

1. **完整配置（推荐）**：直接存储整个配置对象
   - Key: `myapp.config.dev`（prefix + env）
   - Value: 完整 JSON（包含所有 section）

2. **分段配置**：按 section 拆分存储
   - Key: `myapp.config.app.dev` → 顶层字段（`app-name`、`env`、`port` 等）
   - Key: `myapp.config.jwt.dev` → `jwt` section
   - Key: `myapp.config.db.dev` → `db` section
   - Key: `myapp.config.rocketmq.dev` → 你的 `rocketmq` section
   - Key: `myapp.config.kafka.dev` → 你的 `kafka` section

**重要说明**：
- 远端配置**不需要**包含 `sdk` 连接信息（会自动从本地启动文件保留）
- 远端配置**不需要**包含 `config-source`（会自动从本地启动文件保留）
- `env` 和 `host` 会在加载后自动补齐（基于运行时环境）

---

## 初始化职责划分

### ✅ 应该在 `pkg/core/start` 中完成的事（通用能力）

- 配置加载（本地文件 / 配置中心）
- Logger 初始化
- Auth 初始化（AdminAuth / UserAuth）
- 通用基础设施工厂（EnableRedis / EnableMysql / EnableSDK 等）

### ✅ 应该在项目层完成的事（业务差异大）

建议在项目的 `main.go` 或 `internal/bootstrap` 包中完成：

- **数据库迁移**（`db.AutoMigrate`）
- **Redis/Cache/Scheduler 启动**
- **OSS 初始化**（业务相关的 Bucket 配置）
- **gRPC Server 启动与服务注册**
- **定时任务注册**（SSL 续期、数据同步等业务任务）
- **业务 Bootstrap 数据初始化**（默认管理员、默认服务器等）
- **应用组合根创建**（`app.NewApp()`）
- **路由注册**（`router.Register`）
- **HTTP Server 启动**

**为什么这样划分？**
- 业务差异大的初始化（如具体表结构、具体任务逻辑、具体服务注册）放在项目层，避免通用库过度膨胀。
- 通用能力（如"创建 DB 连接"）放在 `start`，各项目复用避免重复代码。

---

## API 参考

### 泛型配置加载器

```go
// LoadConfig 加载配置，返回指针和错误
func LoadConfig[T any](file []byte, env string) (*T, error)

// MustLoadConfig 加载配置，失败则 panic（快速启动风格）
func MustLoadConfig[T any](file []byte, env string) *T
```

**要求**：
- `T` 必须**内嵌** `start.Config`（使用 `yaml:",inline"` 标签）
- 支持本地文件和配置中心两种模式

### 基础实例构造

```go
// NewConfigures 从文件加载配置并创建 Configures（兼容旧 API）
func NewConfigures(file []byte, env string) *Configures

// NewConfiguresFromConfig 从已加载的 Config 创建 Configures（推荐）
func NewConfiguresFromConfig(cfg Config) *Configures
```

---

## 完整示例：其他项目启动

```go
package main

import (
    "fmt"
    "os"

    "github.com/xsxdot/aio/base"
    "github.com/xsxdot/aio/pkg/core/start"
    "github.com/xsxdot/aio/pkg/db"
    "github.com/xsxdot/aio/pkg/scheduler"
    "yourproject/internal/config"
    "yourproject/app"
    "yourproject/router"
)

func main() {
    // 1. 加载扩展配置
    file, err := os.ReadFile("resources/dev.yaml")
    if err != nil {
        panic(err)
    }

    cfg := start.MustLoadConfig[config.AppConfig](file, "dev")

    // 2. 创建通用基础实例
    configures := start.NewConfiguresFromConfig(cfg.Config)
    base.Configures = configures
    base.Logger = configures.Logger
    base.AdminAuth = configures.AdminAuth
    base.UserAuth = configures.UserAuth

    // 3. 初始化通用基础设施
    base.DB = configures.EnableMysql()
    base.RDB = configures.EnableRedis()
    base.Cache = configures.EnableCache(base.RDB)
    base.Scheduler = scheduler.NewScheduler(scheduler.DefaultSchedulerConfig())

    // 4. 数据库迁移
    if err := db.AutoMigrate(base.DB); err != nil {
        base.Logger.Panic(fmt.Sprintf("数据库迁移失败: %v", err))
    }

    // 5. 启动调度器
    if err := base.Scheduler.Start(); err != nil {
        base.Logger.Panic(fmt.Sprintf("启动调度器失败: %v", err))
    }

    // 6. 初始化项目特有的中间件（使用扩展配置）
    rocketMQProducer := initRocketMQ(cfg.RocketMQ)
    kafkaConsumer := initKafka(cfg.Kafka)

    // 7. 创建应用组合根
    appRoot := app.NewApp()

    // 8. 业务 Bootstrap 初始化
    if err := appRoot.Bootstrap(); err != nil {
        base.Logger.Panic(fmt.Sprintf("业务初始化失败: %v", err))
    }

    // 9. 创建 Fiber 应用并注册路由
    fiberApp := app.GetApp()
    router.Register(appRoot, fiberApp)

    // 10. 启动 HTTP Server
    base.Logger.Info(fmt.Sprintf("服务启动: http://%s:%d", cfg.Host, cfg.Port))
    if err := fiberApp.Listen(fmt.Sprintf(":%d", cfg.Port)); err != nil {
        base.Logger.Panic(err)
    }
}
```

---

## 常见问题

### Q1: 我的项目不需要配置中心，是否还需要配置 `sdk` 字段？

**A:** 不需要。如果 `config-source` 不是 `config-center`（或不指定），`sdk` 配置会被忽略。

### Q2: 配置中心的 `bootstrap_config_prefix` 如何设计？

**A:** 推荐格式：`{项目名}.config`（例如 `myapp.config`）
- 完整配置 Key：`{prefix}.{env}`（例如 `myapp.config.dev`）
- 分段配置 Key：`{prefix}.{section}.{env}`（例如 `myapp.config.jwt.dev`）

### Q3: 如何在配置中心存储我的扩展配置段？

**A:** 与通用配置段一样，按 section 拆分：
- Key: `myapp.config.rocketmq.dev`
- Value: `{"name_server": "localhost:9876", "topic": "my-topic"}`

### Q4: 我能修改 `start.Config` 中的字段吗？

**A:** 不推荐。如果需要额外字段，使用"内嵌 + 扩展"模式，避免修改公共库。

### Q5: 配置中心加载失败会怎样？

**A:** `MustLoadConfig` 会 panic 并终止启动。如果需要优雅降级，使用 `LoadConfig` 并处理 error。

---

## 迁移指南

### 从旧版 `NewConfigures` 迁移到泛型加载器

**旧代码**：
```go
configures := start.NewConfigures(file, env)
// 访问配置：configures.Config.Jwt.Secret
```

**新代码**（无需修改，保持兼容）：
```go
configures := start.NewConfigures(file, env)
// 访问配置：configures.Config.Jwt.Secret
```

**新代码**（推荐，支持扩展配置）：
```go
cfg := start.MustLoadConfig[config.AppConfig](file, env)
configures := start.NewConfiguresFromConfig(cfg.Config)
// 访问通用配置：cfg.Config.Jwt.Secret
// 访问扩展配置：cfg.RocketMQ.NameServer
```

---

## 总结

- ✅ **复用 `pkg/core/start` 作为公共库**，避免每个项目重复实现配置加载逻辑
- ✅ **使用泛型 `LoadConfig[T]`** 支持项目自定义配置段，同时保持强类型
- ✅ **支持本地文件和配置中心两种模式**，灵活适配不同环境
- ✅ **职责清晰**：通用能力在 `start`，业务差异在项目层

如有问题或需要新功能，请联系基础设施团队。

