# AIO 服务

![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)

AIO 是一个基于 Go 语言的多功能服务框架，提供了认证管理、分布式协调、配置管理、消息队列等核心功能，帮助您快速构建可靠、高性能的分布式应用。通过将常用基础服务内置到单一应用中，AIO旨在简化中小项目的环境依赖，提供丰富的工具集，降低开发和运维成本。

## 核心特性

- 🔐 **认证与证书管理**
  - SSL证书自动签发和续期
  - 自生成CA证书用于etcd、NATS等组件通信加密
  - 内置JWT认证支持

- 🌐 **分布式功能**
  - 内嵌etcd，无需额外部署
  - 服务注册与发现
  - 领导选举机制
  - 分布式锁
  - 分布式ID生成器
  - 支持集群模式部署

- ⚙️ **配置中心**
  - 基于etcd的配置管理
  - 支持多环境配置（开发、测试、生产）
  - 配置热更新

- ⏰ **定时任务**
  - 支持普通定时任务
  - 支持分布式任务（基于etcd分布式锁实现）
  - Cron表达式支持

- 💾 **缓存服务**
  - 内嵌缓存服务器
  - 兼容Redis协议
  - 可作为独立缓存使用

- 📨 **消息队列**
  - 内嵌NATS消息队列
  - 支持发布/订阅模型
  - 支持请求/响应模型
  - 支持队列组和消息持久化

- 📊 **监控能力**
  - 系统资源监控（CPU、内存、磁盘）
  - 应用指标收集
  - 可视化数据展示

- 🔧 **可扩展**
  - 模块化设计，易于扩展和定制
  - 插件机制支持

## 系统要求

- Go 1.24或更高版本
- 支持的操作系统: Linux, macOS, Windows

## 快速开始

### 安装

**Linux/macOS**:
```bash
./install.sh
```

**Windows**:
```cmd
install.bat
```

### 配置

配置文件默认位于 `./conf` 目录。基本配置示例:

```yaml
system:
  mode: standalone  # 可选: standalone, cluster
  nodeId: node1
  logLevel: info
```

### 运行

```bash
./cmd/aio/aio --config ./conf
```

查看版本信息:
```bash
./cmd/aio/aio --version
```

## 使用方式

AIO 提供了多种使用方式，包括 Go 语言 SDK 和 Web 管理后台。

### Go 语言客户端SDK

AIO 提供了完整的 Go 语言客户端 SDK，先在管理后台生成一对key和密钥之后您可以通过以下方式在您的项目中使用：

#### 1. 安装SDK

```bash
go get github.com/xsxdot/aio/client
```

#### 2. 基本使用

下面是一个基于实际项目的客户端使用示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"github.com/xsxdot/aio/client"
)

type AppConfig struct {
	AppName string           `yaml:"app-name"`
	Env     string           `yaml:"env"`
	Host    string           `yaml:"host"`
	Port    int              `yaml:"port"`
	Domain  string           `json:"domain"`
	Aio     struct {
		Hosts        string `yaml:"hosts"`
		ClientId     string `yaml:"client-id"`
		ClientSecret string `yaml:"client-secret"`
	} `yaml:"aio"`
}

func main() {
	// 加载应用配置
	cfg := AppConfig{
		AppName: "my-app",
		Env:     "dev",
		Port:    8080,
		Aio: struct {
			Hosts        string `yaml:"hosts"`
			ClientId     string `yaml:"client-id"`
			ClientSecret string `yaml:"client-secret"`
		}{
			Hosts:        "localhost:9100",
			ClientId:     "client-id",
			ClientSecret: "client-secret",
		},
	}

	// 连接AIO服务
	hosts := strings.Split(cfg.Aio.Hosts, ",")
	clientOptions := client.NewBuilder("", cfg.AppName, cfg.Port).
		WithDefaultProtocolOptions(hosts, cfg.Aio.ClientId, cfg.Aio.ClientSecret).
		WithEtcdOptions(client.DefaultEtcdOptions).Build()

	aioClient := clientOptions.NewClient()
	err := aioClient.Start(context.Background())
	if err != nil {
		log.Fatalf("启动AIO客户端失败: %v", err)
	}
	defer aioClient.Close()

	// 从配置中心获取配置
	var logConfig struct {
		Level string `json:"level"`
		Sls   bool   `json:"sls"`
	}
	err = aioClient.Config.GetEnvConfigJSONParse(
		context.Background(),
		fmt.Sprintf("%s.%s", cfg.AppName, "log"),
		cfg.Env,
		[]string{"default"},
		&logConfig,
	)
	if err != nil {
		log.Fatalf("获取日志配置失败: %v", err)
	}
	log.Printf("日志级别: %s, SLS启用: %v", logConfig.Level, logConfig.Sls)

	// 服务发现
	services, err := aioClient.Discovery.ListServices(context.Background(), "web")
	if err != nil {
		log.Printf("获取服务列表失败: %v", err)
	} else {
		for _, service := range services {
			log.Printf("服务: %s, 地址: %s", service.Name, service.Address)
		}
	}
}
```

#### 3. 功能模块使用示例

##### 配置服务

```go
// 配置环境回退顺序
fallbacks := []string{"default"}

// 从配置中心获取指定环境的配置并解析到结构体
var dbConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}
err := aioClient.Config.GetEnvConfigJSONParse(
	context.Background(),
	fmt.Sprintf("%s.%s", appName, "database"), 
	env, 
	fallbacks, 
	&dbConfig,
)

// 监听配置变更
ch := make(chan client.ConfigChange)
err := aioClient.Config.Watch(ctx, fmt.Sprintf("%s.%s", appName, "database"), ch)
go func() {
	for change := range ch {
		log.Printf("配置变更: %s = %s", change.Key, change.Value)
	}
}()
```

##### 服务发现

```go
// 注册服务
service := &client.Service{
	Name:    "api",
	Version: "1.0.0",
	Address: host,
	Port:    port,
	Tags:    []string{"http", "json"},
}
err := aioClient.Discovery.Register(ctx, service)

// 发现服务
services, err := aioClient.Discovery.Discover(ctx, "api", "1.0.0")
```

##### 分布式锁

```go
// 获取锁
lock, err := aioClient.Etcd.Lock(ctx, "my-resource", 30)
if err != nil {
	log.Printf("获取锁失败: %v", err)
} else {
	defer lock.Unlock()
	// 执行需要加锁的操作
}
```

##### 领导选举

```go
// 参与选举
election, err := aioClient.Etcd.Election("cluster-leader")
if err != nil {
	log.Printf("创建选举失败: %v", err)
}

// 成为领导者的回调
electionCh := make(chan bool)
err = election.Campaign(ctx, "node-1", electionCh)
go func() {
	for isLeader := range electionCh {
		if isLeader {
			log.Println("成为领导者")
			// 领导者逻辑
		} else {
			log.Println("失去领导者地位")
			// 跟随者逻辑
		}
	}
}()
```

##### 消息队列

```go
// 发布消息
err := aioClient.NATS.Publish(ctx, "events.user", []byte(`{"id":1,"name":"用户1"}`))

// 订阅消息
subscription, err := aioClient.NATS.Subscribe(ctx, "events.user", func(msg []byte) {
	log.Printf("收到消息: %s", string(msg))
})
defer subscription.Unsubscribe()
```

##### 缓存服务

```go
// 设置缓存
err := aioClient.Cache.Set(ctx, "user:1", `{"id":1,"name":"用户1"}`, 3600)

// 获取缓存
value, err := aioClient.Cache.Get(ctx, "user:1")
```

##### 定时任务

```go
// 获取scheduler实例
scheduler := aioClient.Scheduler

// 创建一个普通任务，不需要分布式锁
taskID1, err := scheduler.AddTask("email-notification", func(ctx context.Context) error {
    // 在这里执行任务逻辑
    log.Println("发送邮件通知")
    return nil
}, false)
if err != nil {
    log.Printf("添加任务失败: %v", err)
}

// 创建一个延时任务，需要分布式锁保证集群中只有一个实例执行
taskID2, err := scheduler.AddDelayTask("cleanup-files", 1*time.Hour, func(ctx context.Context) error {
    // 在这里执行任务逻辑
    log.Println("清理临时文件")
    return nil
}, true)
if err != nil {
    log.Printf("添加延时任务失败: %v", err)
}

// 创建一个周期性任务，每10分钟执行一次，立即执行第一次
taskID3, err := scheduler.AddIntervalTask("health-check", 10*time.Minute, true, func(ctx context.Context) error {
    // 在这里执行任务逻辑
    log.Println("执行健康检查")
    return nil
}, true)
if err != nil {
    log.Printf("添加周期任务失败: %v", err)
}

// 创建一个基于Cron表达式的定时任务，每天凌晨3点执行
taskID4, err := scheduler.AddCronTask("backup-database", "0 3 * * *", func(ctx context.Context) error {
    // 在这里执行任务逻辑
    log.Println("备份数据库")
    return nil
}, true)
if err != nil {
    log.Printf("添加Cron任务失败: %v", err)
}

// 取消任务
err = scheduler.CancelTask(taskID1)
if err != nil {
    log.Printf("取消任务失败: %v", err)
}

// 获取任务信息
task, err := scheduler.GetTask(taskID2)
if err != nil {
    log.Printf("获取任务信息失败: %v", err)
} else {
    log.Printf("任务ID: %s, 名称: %s", task.ID, task.Name)
}

// 获取所有任务
allTasks := scheduler.GetAllTasks()
for _, t := range allTasks {
    log.Printf("任务: %s, 下次执行时间: %v", t.Name, t.NextRunTime)
}
```

##### 监控指标

```go
// 创建监控客户端
monitoringClient, err := client.NewMonitoringClient(
    aioClient, 
    aioClient.NATS.GetConnection(), 
    client.DefaultMonitoringOptions(),
)
if err != nil {
    log.Fatalf("创建监控客户端失败: %v", err)
}
defer monitoringClient.Stop()

// 跟踪API调用
startTime := time.Now()
// ... 处理API请求 ...
monitoringClient.TrackAPICall(
    "/api/users", 
    "GET", 
    startTime, 
    200, 
    false, 
    "", 
    128, 
    1024, 
    "192.168.1.100", 
    map[string]string{"department": "sales"},
)

// 发送自定义应用指标
err = monitoringClient.SendCustomAppMetrics(map[string]interface{}{
    "active_users": 1250,
    "pending_tasks": 45,
    "queue_depth": 12,
})
if err != nil {
    log.Printf("发送自定义指标失败: %v", err)
}

// 发送服务监控数据
err = monitoringClient.SendServiceData(map[string]interface{}{
    "database_connections": 25,
    "cache_hit_ratio": 0.85,
    "average_response_time": 42.5,
})
if err != nil {
    log.Printf("发送服务监控数据失败: %v", err)
}

// 发送服务API调用数据
apiCalls := []client.APICall{
    {
        Endpoint:     "/api/users",
        Method:       "GET",
        Timestamp:    time.Now().Add(-1 * time.Minute),
        DurationMs:   45.2,
        StatusCode:   200,
        HasError:     false,
        ErrorMessage: "",
        RequestSize:  128,
        ResponseSize: 1024,
        ClientIP:     "192.168.1.100",
        Tags:         map[string]string{"user_type": "admin"},
    },
    // ... 更多API调用记录 ...
}
err = monitoringClient.SendServiceAPIData(apiCalls)
if err != nil {
    log.Printf("发送API调用数据失败: %v", err)
}
```

### Web 管理后台

AIO 提供了一个功能强大的 Web 管理后台，可以通过浏览器访问并管理所有功能。

#### 访问方式

- 启动 AIO 服务后，直接通过浏览器访问：`http://<服务器IP>:<端口号>`
- 默认端口为 8080
- 默认账户和密码均为 `admin`

#### 主要功能

Web 管理后台提供以下功能：

1. **仪表盘**
   - 系统概览
   - 资源使用情况
   - 组件状态

2. **配置中心**
   - 查看和编辑配置
   - 多环境配置管理
   - 配置版本历史

3. **服务管理**
   - 服务注册列表
   - 服务健康状态
   - 服务详情查看

4. **任务调度**
   - 任务列表
   - 创建和编辑任务
   - 任务执行历史
   - 手动触发任务

5. **监控中心**
   - 系统资源监控
   - 自定义指标查看
   - 告警规则配置
   - 告警历史

6. **组件配置**
   - 消息队列管理和配置
   - 缓存服务管理和配置
   - SSL证书管理和配置

#### 组件启用说明

AIO 默认启用了大部分功能，但以下组件需要通过 Web 管理后台配置后启用：

- **消息队列 (MQ)** - 需要在管理后台中配置和启用
- **缓存服务 (Cache)** - 需要在管理后台中配置和启用
- **SSL证书管理** - 需要在管理后台中配置和启用

其他组件（如 etcd、服务发现、配置中心、定时任务、监控）默认已启用，可以直接使用。

## 详细功能说明

### 证书管理

AIO提供两种证书管理功能：

1. **SSL证书管理**
   - 支持通过ACME协议自动向Let's Encrypt申请SSL证书
   - 自动处理证书续期
   - 支持证书状态监控

2. **内部CA证书**
   - 自动生成CA根证书
   - 为服务组件（如etcd、NATS）签发证书
   - 简化内部通信加密配置

### 分布式功能

AIO内置分布式协调能力：

1. **内嵌etcd**
   - 无需外部依赖即可使用分布式功能
   - 支持单节点和集群模式
   - 数据持久化

2. **服务发现**
   - 自动注册服务
   - 服务健康检查
   - 动态服务发现

3. **领导选举与分布式锁**
   - 支持主从架构应用
   - 防止脑裂
   - 分布式锁API简化并发控制

4. **分布式ID生成器**
   - 支持多种ID生成策略
   - 保证全局唯一性
   - 高性能设计

### 配置中心

中央化配置管理：

1. **基于etcd的配置存储**
   - 配置版本控制
   - 配置变更历史

2. **多环境支持**
   - 开发、测试、生产环境隔离
   - 环境继承机制

3. **动态配置**
   - 配置热更新
   - 配置变更通知

### 定时任务

强大的任务调度功能：

1. **本地任务**
   - 基于Cron表达式
   - 任务依赖关系

2. **分布式任务**
   - 集群间任务协调
   - 单次执行保证
   - 失败重试机制

3. **任务管理**
   - 可视化任务状态
   - 手动触发
   - 执行日志

### 缓存服务

内置缓存服务：

1. **Redis协议兼容**
   - 无需修改客户端即可使用
   - 支持主要Redis命令

2. **存储选项**
   - 内存存储
   - 可持久化
   - 支持TTL

### 消息队列

集成NATS消息队列：

1. **消息模式**
   - 发布/订阅
   - 请求/响应
   - 队列组

2. **持久化**
   - 消息持久化选项
   - 至少一次送达保证
   - 消息重放

### 监控

全面的监控能力：

1. **系统监控**
   - CPU、内存、网络、磁盘使用率
   - 进程监控

2. **应用监控**
   - 自定义指标
   - 性能分析
   - 日志聚合

## 项目结构

```
.
├── app/          # 应用核心代码
├── cmd/          # 命令行入口
├── conf/         # 配置文件
├── docs/         # 文档
├── internal/     # 内部包
│   ├── authmanager/    # 认证管理
│   ├── cache/          # 缓存服务
│   ├── certmanager/    # 证书管理
│   ├── etcd/           # etcd组件
│   ├── monitoring/     # 监控组件
│   └── mq/             # 消息队列
├── pkg/          # 公共包
│   ├── auth/           # 认证相关
│   ├── common/         # 通用工具
│   ├── config/         # 配置相关
│   ├── distributed/    # 分布式功能
│   ├── protocol/       # 协议相关
│   └── scheduler/      # 任务调度
├── client/       # Go语言客户端SDK
└── web/          # Web管理界面
```

## 开发

### 构建

```bash
./build.sh
```

### 测试

```bash
go test ./...
```

## 使用场景

AIO 适用于以下场景：

1. **微服务基础设施**
   - 降低微服务基础设施的复杂性
   - 提供服务注册、发现、配置管理等核心能力

2. **中小型应用架构**
   - 简化应用依赖
   - 减少运维成本

3. **边缘计算**
   - 资源受限环境下的分布式系统
   - 降低外部依赖

4. **开发和测试环境**
   - 快速搭建完整功能的开发环境
   - 降低环境差异导致的问题

## 配置参考

详细配置请参阅 [docs/config.md](./docs/config.md)

## 贡献指南

我们欢迎任何形式的贡献，包括但不限于:

1. 提交问题和建议
2. 提交Pull Request
3. 改进文档

在提交PR前，请确保:
- 代码符合Go的标准格式 (使用 `go fmt`)
- 所有测试用例通过
- 新功能添加了相应的测试
- 更新了相关的文档

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。

## 联系与支持

- 问题反馈: 在GitHub上提交Issue
- 邮件联系: [维护者邮箱]
- 讨论组: [讨论组链接，如有]

## 致谢

感谢所有贡献者和以下开源项目的支持:

- [etcd](https://github.com/etcd-io/etcd) - 分布式键值存储
- [NATS](https://github.com/nats-io/nats-server) - 高性能消息系统
- [Fiber](https://github.com/gofiber/fiber) - Web框架
- [Badger](https://github.com/dgraph-io/badger) - 键值数据库
- [Go Redis](https://github.com/redis/go-redis) - Redis客户端
- [Zap](https://github.com/uber-go/zap) - 日志库 