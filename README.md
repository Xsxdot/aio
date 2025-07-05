# AIO All-In-One 分布式服务

本项目旨在通过单一软件包中提供必要的分布式服务组件，以ETCD为基础核心，可单点运行，也可以集群运行，提供一个完整的分布式系统解决方案。

## 🚀 项目特性

- ✅ **一体化解决方案**: 提供服务注册与发现、配置中心、分布式锁、定时任务、监控告警、证书管理等功能。
- ✅ **高可用架构**: 基于ETCD构建，支持集群部署，保证核心服务高可用。
- ✅ **插件化设计**: 客户端支持插件，方便与GORM等第三方库集成。
- ✅ **Web管理界面**: 提供美观易用的Web UI，方便管理和监控。
- ✅ **gRPC支持**: 内置gRPC服务框架，遵循规范，方便开发微服务。
- ✅ **自动化部署**: 提供一键部署脚本，支持滚动更新、健康检查和回滚。

## 📁 项目结构

```
aio/
├── api/                   # API协议定义 (Protobuf)
├── app/                   # 应用核心与启动器
├── client/                # AIO客户端库
├── cmd/                   # 应用程序入口
│   └── server/            # 主服务器
├── conf/                  # 生产环境配置文件示例
├── internal/              # 内部业务逻辑模块
│   ├── authmanager/       # 认证管理
│   ├── certmanager/       # 证书管理
│   ├── etcd/              # ETCD客户端封装
│   ├── fiber/             # Web框架集成
│   └── grpc/              # gRPC服务框架
├── pkg/                   # 公共功能包
│   ├── config/            # 配置中心
│   ├── lock/              # 分布式锁
│   ├── monitoring/        # 监控
│   ├── notifier/          # 通知服务
│   ├── registry/          # 服务注册与发现
│   ├── scheduler/         # 定时任务
│   └── ...
├── web/                   # 前端静态资源
├── deploy.sh              # 部署脚本
├── go.mod                 # Go模块文件
└── README.md              # 项目说明
```

## ⚡ 部署与运行

### 环境要求

- Go 1.24+ (如果需要从源码部署)
- ETCD 3.5+
- 可以访问目标服务器的SSH免密登录 (如果使用部署脚本)

### 方式一：使用预编译包（推荐）

1.  **下载安装包**
    访问项目的 GitHub Releases 页面，下载最新的 `aio-linux-amd64.tar.gz` 安装包。

2.  **上传并解压**
    将安装包上传到服务器，并解压到指定目录，例如 `/opt/aio`。
    ```bash
    mkdir -p /opt/aio
    tar -zxvf aio-linux-amd64.tar.gz -C /opt/aio
    ```

3.  **修改配置**
    进入 `conf` 目录，将示例配置文件 `aio-example.yaml` 复制为 `aio.yaml`，并根据实际需求（如ETCD地址、数据库连接等）进行修改。
    ```bash
    cd /opt/aio/conf
    cp aio-example.yaml aio.yaml
    vim aio.yaml
    ```

4.  **启动服务**
    回到 `/opt/aio` 目录，直接运行主程序即可。
    ```bash
    cd /opt/aio
    ./aio
    ```
    建议使用 `systemd` 或 `supervisor` 等工具管理服务进程，以实现开机自启和守护进程。部署脚本中已包含 `systemd` 服务文件示例。

### 方式二：源码编译部署

此方式适用于开发环境或需要自定义构建的场景。

1.  **克隆项目**
    ```bash
    git clone https://github.com/xsxdot/aio.git
    cd aio/go
    ```

2.  **配置部署脚本**
    打开 `deploy.sh` 文件，修改 `SERVERS` 和 `REMOTE_USER` 等变量，配置目标服务器信息。
    ```bash
    vim deploy.sh
    ```

3.  **执行部署**
    运行部署脚本。脚本会自动完成编译、打包、上传、安装、启动服务、健康检查等一系列操作。
    ```bash
    ./deploy.sh deploy
    ```

### 访问服务

- **Web界面**: `http://<server-ip>:9999` (默认端口，见配置文件)
- **gRPC服务**: `<server-ip>:6666` (默认端口，见配置文件)
- **健康检查**: `http://<server-ip>:9999/health`

## 🔧 配置说明

配置文件支持YAML格式，也可以通过环境变量进行配置。环境变量使用 `AIO_` 前缀。

### 主要配置项

- **server**: HTTP服务器配置
- **logger**: 日志配置
- **etcd**: ETCD连接配置
- **registry**: 服务注册配置
- **lock**: 分布式锁配置
- **scheduler**: 定时任务配置
- **cert**: SSL证书配置

### 环境变量示例

```bash
export AIO_SERVER_PORT=9090
export AIO_LOGGER_LEVEL=debug
export AIO_ETCD_ENDPOINTS=localhost:2379,localhost:2380
```

## 💻 客户端使用

AIO提供了Go客户端库，方便业务应用集成AIO的各项能力。

### 1. 引入客户端

```bash
go get github.com/xsxdot/aio/client
```

### 2. 使用示例

以下是一个简单的使用示例，演示了如何初始化客户端并使用配置中心、服务发现和分布式锁功能。

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/xsxdot/aio/client"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/scheduler"
)

func main() {
	// 1. 初始化AIO客户端配置
	// 客户端ID和密钥需在AIO服务端预先创建
	aioClient := client.New(&client.AioConfig{
		Endpoints: []string{"127.0.0.1:6666"}, // AIO的gRPC服务地址
		ClientId:  "your-app-id",
		Secret:    "your-app-secret",
	}, &registry.ServiceInstance{
		Name:     "my-awesome-app",
		Address:  "127.0.0.1:8080",
		Protocol: "http",
		Env:      registry.Development,
	})

	// 2. 启动客户端并连接到AIO服务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := aioClient.Start(ctx); err != nil {
		log.Fatalf("无法启动AIO客户端: %v", err)
	}
	defer aioClient.Close()
	log.Println("AIO客户端启动成功")

	// 3. 使用配置中心
	// 假设AIO配置中心有名为 "my-awesome-app.database" 的配置
	type dbConfig struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	var myDbConfig dbConfig
	
	err := aioClient.ConfigClient.GetEnvConfigJSONWithParse(context.Background(), "my-awesome-app.database", "dev", []string{"default"}, &myDbConfig)
	if err != nil {
		log.Printf("获取配置失败: %v\n", err)
	} else {
		log.Printf("成功加载配置: %+v\n", myDbConfig)
	}

	// 4. 使用服务发现
	// 假设需要发现名为 "user-service" 的服务
	service, err := aioClient.RegistryClient.GetService(context.Background(), "user-service")
	if err != nil {
		log.Printf("服务发现失败: %v\n", err)
	} else {
		log.Printf("发现服务实例: %+v\n", service.Instances)
	}

	// 5. 使用分布式锁
	lock, err := aioClient.LockManager.NewLock("my-critical-resource", 15*time.Second)
	if err != nil {
		log.Fatalf("创建锁失败: %v", err)
	}
	
	log.Println("尝试获取锁...")
	if err := lock.Lock(context.Background()); err != nil {
		log.Println("获取锁失败")
	} else {
		log.Println("成功获取锁！执行关键业务...")
		time.Sleep(5 * time.Second) // 模拟业务操作
		_ = lock.Unlock(context.Background())
		log.Println("锁已释放")
	}

	// 6. 使用分布式定时任务
	// 创建一个5秒后执行的本地一次性任务
	oneTimeTask := scheduler.NewOnceTask(
		"my-one-time-task",
		time.Now().Add(5*time.Second),
		scheduler.TaskExecuteModeLocal,
		10*time.Second, // 超时时间
		func(ctx context.Context) error {
			log.Println("分布式一次性任务被执行！")
			return nil
		},
	)

	if err := aioClient.Scheduler.AddTask(oneTimeTask); err != nil {
		log.Printf("添加一次性任务失败: %v", err)
	} else {
		log.Println("成功添加一次性任务，ID:", oneTimeTask.GetID())
	}

	// 创建一个每10秒执行一次的分布式周期性任务
	// TaskExecuteModeDistributed 表示任务将在集群中选举一个节点执行
	intervalTask := scheduler.NewIntervalTask(
		"my-interval-task",
		time.Now().Add(10*time.Second),
		10*time.Second,
		scheduler.TaskExecuteModeDistributed,
		5*time.Second,
		func(ctx context.Context) error {
			log.Println("分布式周期性任务被执行！")
			return nil
		},
	)

	if err := aioClient.Scheduler.AddTask(intervalTask); err != nil {
		log.Printf("添加周期性任务失败: %v", err)
	} else {
		log.Println("成功添加周期性任务，ID:", intervalTask.GetID())
	}

	// 主程序需要持续运行才能看到定时任务执行
	// 这里等待足够长的时间以便观察任务执行
	log.Println("等待定时任务执行...")
	time.Sleep(30 * time.Second)
}
```

## 📋 功能清单

- [x] **核心架构**: 基于ETCD，提供稳定的分布式协调能力。
- [x] **配置中心**: 支持动态配置下发、版本管理、环境隔离。
- [x] **服务注册与发现**: 支持服务实例注册、健康检查和客户端服务发现。
- [x] **分布式锁**: 提供基于ETCD的分布式互斥锁，支持超时和自动续期。
- [x] **定时任务**: 支持Cron表达式，实现分布式任务调度、分片和故障转移。
- [x] **证书管理**: 集成Let's Encrypt，实现SSL证书自动申请和续期。
- [x] **监控告警**: 提供应用和系统指标采集、存储，并支持多种通知渠道（钉钉、企业微信、邮件、Webhook）发送告警。
- [x] **Web管理界面**: 可视化管理所有功能模块。
- [x] **Go客户端**: 提供简单易用的Go Client SDK，方便应用快速接入。
- [x] **gRPC框架**: 标准化的gRPC服务开发、注册和管理。
- [x] **日志系统**: 基于Logrus的结构化日志。
- [x] **部署工具**: 提供`deploy.sh`脚本，简化部署流程。

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./pkg/errors/

# 运行测试并显示覆盖率
go test -cover ./...
```

## 📚 开发规范

### 代码规范

- 遵循Go官方代码规范
- 使用`gofmt`和`golangci-lint`进行代码检查
- 函数和方法必须有注释
- 接口设计优先，面向接口编程

### 提交规范

- `feat`: 新功能
- `fix`: 修复问题
- `refactor`: 重构代码
- `docs`: 文档更新
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

## 🤝 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🆕 更新日志

### v1.0.0 (当前)

- ✅ 完成基础项目架构
- ✅ 实现配置管理模块
- ✅ 实现日志系统
- ✅ 实现自定义错误处理
- ✅ 实现基础HTTP服务
- ✅ 添加工具函数库 