# AIO (All-In-One) 服务框架

[English](#english) | [中文](#chinese)

## English

### Introduction
**AIO is an all-in-one service framework designed to simplify the infrastructure dependencies for small to medium-sized microservices by providing essential distributed components in a single, cohesive package.**

Built around etcd as its core, AIO offers a comprehensive suite of components necessary for distributed systems, eliminating the need to integrate and manage multiple separate dependencies. It provides consistent distributed primitives including distributed locks, ID generation, leader election, service discovery, configuration center, and monitoring capabilities.

Beyond the etcd-based distributed services, AIO includes a Redis-protocol compatible caching service and an embedded NATS message queue, allowing developers to leverage familiar protocols while reducing infrastructure complexity. The framework's modular design enables developers to selectively activate or use only the components they need, making it highly adaptable to different application requirements.

AIO focuses on developer experience, offering a unified platform where teams can focus on business logic rather than infrastructure integration and maintenance. By bundling essential distributed system components in a cohesive package, AIO significantly reduces the development and operational overhead commonly associated with microservice architectures.

#### Core Philosophy
- **Simplicity**: Despite offering comprehensive functionality, AIO maintains a clean, intuitive API
- **Modularity**: Components can be used together or independently based on application needs
- **Reliability**: Built with distributed system challenges in mind, focusing on fault tolerance
- **Performance**: Optimized for high throughput and low latency applications
- **Extensibility**: Designed to be extended with custom implementations when needed

#### System Architecture
AIO follows a modular architecture with several key components that interact through well-defined interfaces:

![AIO Architecture](docs/images/architecture.png)

The architecture consists of:
1. **Core Layer**: Provides fundamental services like logging, authentication, and protocol handling
2. **Distributed Layer**: Handles coordination between nodes in a distributed environment
3. **Service Layer**: Implements specialized services like message queuing and caching
4. **API Layer**: Exposes functionality through HTTP and custom protocol handlers

### Features
- **Distributed Architecture**: Built-in support for service discovery and leader election
  - Automatic node registration and health checking
  - Distributed leader election for high availability
  - Cluster state synchronization
  
- **Configuration Management**: Centralized configuration using etcd
  - Dynamic configuration updates without restarts
  - Configuration versioning and rollback support
  - Configuration change notifications
  
- **Message Queue**: Integrated NATS server for reliable message delivery
  - High-performance publish/subscribe messaging
  - Persistent message storage with configurable retention
  - Message replay and durable subscriptions
  
- **Caching**: Built-in caching server for improved performance
  - Distributed cache with automatic invalidation
  - Multiple storage backends (memory, Redis)
  - Customizable caching policies and TTL
  
- **Monitoring**: Comprehensive system monitoring capabilities
  - Resource usage tracking (CPU, memory, disk)
  - Performance metrics for all components
  - Customizable alerting and notification system
  
- **Authentication**: Built-in authentication management
  - Support for multiple authentication methods
  - Fine-grained access control
  - JWT token management
  
- **API Gateway**: Fiber-based HTTP server for API exposure
  - High-performance HTTP routing
  - Middleware support for common tasks
  - API versioning and documentation
  
- **Protocol Support**: Custom protocol manager for service communication
  - Extensible protocol handlers
  - Support for custom binary protocols
  - Protocol conversion and interoperability

### Use Cases
AIO is particularly well-suited for:

- **Microservices Architecture**: Provides the infrastructure needed for reliable microservices
- **Distributed Systems**: Handles complex coordination between distributed nodes
- **API Gateways**: Acts as a central entry point for multiple backend services
- **IoT Platforms**: Manages device communication, data collection, and processing
- **Real-time Applications**: Supports low-latency, high-throughput messaging
- **Edge Computing**: Can run on resource-constrained environments with optimized configuration

### Prerequisites
- Go 1.24 or higher
- etcd 3.5.x
- Redis (optional)
- NATS (optional)

### Installation
```bash
# Clone the repository
git clone https://github.com/xsxdot/aio.git

# Navigate to the project directory
cd aio

# Install dependencies
go mod download
```

### Configuration
1. Copy the example `conf/aio.yaml` file and modify according to your environment
2. The basic configuration is sufficient for initial startup
3. Additional configuration can be completed through the web interface after initialization

The basic configuration structure is as follows:
```yaml
errors:
    debug_mode: true
logger:
    compress: true
    console: true
    file: ./logs/aio.log
    level: info
    max_age: 7
    max_backups: 10
    max_size: 100
network:
    allow_external: false
    bind_ip: localhost
    http_allow_external: true
    http_port: 8080
protocol:
    buffer_size: 4096
    enable_auth: true
    enable_keep_alive: true
    heartbeat_timeout: 30s
    idle_timeout: 60s
    max_connections: 1000
    port: 6666
    read_timeout: 30s
    write_timeout: 30s
system:
    config_salt: "123456789087654321"
    data_dir: ./data
    mode: standalone
    node_id: node1
```

### Usage
```bash
# Compile the application
go build -o aio ./cmd/aio

# Start the service with the basic configuration file
./aio -config ./conf

# Show version information
./aio -version
```

After starting the service, access the web interface at `http://localhost:8080` (or the configured HTTP port) to complete the initialization and additional configuration.

#### SDK Usage Example
The AIO SDK provides a client library for interacting with the AIO service. Here's how to use it in your application:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	
	"github.com/xsxdot/aio/sdk"
)

func main() {
	// Define server endpoints
	servers := []sdk.ServerEndpoint{
		{
			Address: "localhost:6666",
			Weight:  100,
		},
	}
	
	// Configure client options
	options := &sdk.ClientOptions{
		ClientID:            "client_example",
		ClientSecret:        "client_secret",
		ConnectionTimeout:   30 * time.Second,
		ServiceWatchInterval: 60 * time.Second,
	}
	
	// Create a new client
	client := sdk.NewClient(servers, options)
	defer client.Close()
	
	// Connect to the server
	err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Use configuration service
	configValue, err := client.Config.GetValue("my.config.key")
	if err != nil {
		log.Printf("Config error: %v", err)
	} else {
		fmt.Printf("Config value: %s\n", configValue)
	}
	
	// Use service discovery
	services, err := client.Discovery.GetServices("my-service")
	if err != nil {
		log.Printf("Discovery error: %v", err)
	} else {
		for _, service := range services {
			fmt.Printf("Found service: %s at %s\n", service.Name, service.Address)
		}
	}
	
	// Send custom command (example)
	ctx := context.Background()
	response, err := client.SendRequest(ctx, "system.status", []byte("query data"))
	if err != nil {
		log.Printf("Request error: %v", err)
	} else {
		fmt.Printf("Response: %s\n", string(response))
	}
}

### Project Structure
```
.
├── app/            # Application core
│   ├── config/     # Configuration structures and loading
│   ├── fiber/      # HTTP server implementation
│   └── const/      # Constants and default values
├── cmd/            # Command line entry points
│   └── aio/        # Main application executable
├── conf/           # Configuration files
├── docs/           # Documentation
├── internal/       # Internal packages
│   ├── authmanager/  # Authentication management
│   ├── cache/      # Caching implementation
│   ├── etcd/       # etcd integration
│   ├── monitoring/ # System monitoring
│   └── mq/         # Message queue implementation
├── pkg/            # Public packages
│   ├── common/     # Common utilities
│   ├── distributed/ # Distributed system utilities
│   └── protocol/   # Protocol handlers
└── sdk/            # SDK for client applications
```

### Roadmap
Future development plans for AIO include:

- **Cluster Logic Optimization**: Enhancing the coordination and communication between cluster nodes for improved stability and performance
- **Command Execution Engine**: Adding functionality to execute commands remotely across the cluster with proper access control and audit logging
- **Error Log Management**: Implementing comprehensive collection, aggregation, and analysis of error logs from applications using AIO components
- **Distributed Tracing**: Integrating distributed tracing capabilities for end-to-end monitoring of requests across services, helping identify bottlenecks and diagnose issues

We welcome contributions to any of these upcoming features. Please check the issue tracker for current development priorities.

### Contributing
Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### License
This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

---

## Chinese

### 简介
**AIO 是一个一体化服务框架，旨在通过在单一、统一的软件包中提供必要的分布式组件，简化中小型微服务的基础设施依赖。**

以 etcd 为核心构建，AIO 提供了分布式系统所需的全面组件套件，消除了集成和管理多个独立依赖项的需求。它提供了一致的分布式原语，包括分布式锁、ID 生成器、领导者选举、服务注册与发现、配置中心和监控功能。

除了基于 etcd 的分布式服务外，AIO 还包括兼容 Redis 协议的缓存服务和内嵌的 NATS 消息队列，使开发者能够利用熟悉的协议同时减少基础设施复杂性。框架的模块化设计使开发者能够选择性地激活或仅使用他们需要的组件，使其高度适应不同的应用需求。

AIO 注重开发者体验，提供了一个统一的平台，使团队能够专注于业务逻辑而非基础设施的集成和维护。通过在一个统一的软件包中捆绑必要的分布式系统组件，AIO 显著降低了微服务架构通常关联的开发和运维负担。

#### 核心理念
- **简洁性**：尽管提供了全面的功能，AIO 仍然保持简洁直观的 API
- **模块化**：组件可以根据应用需求一起使用或独立使用
- **可靠性**：考虑到分布式系统的挑战，专注于容错能力
- **性能**：为高吞吐量和低延迟应用程序优化
- **可扩展性**：设计为可以在需要时通过自定义实现进行扩展

#### 系统架构
AIO 遵循模块化架构，包含几个通过良好定义的接口进行交互的关键组件：

![AIO 架构](docs/images/architecture.png)

该架构包括：
1. **核心层**：提供日志记录、身份验证和协议处理等基础服务
2. **分布式层**：处理分布式环境中节点之间的协调
3. **服务层**：实现消息队列和缓存等专业服务
4. **API 层**：通过 HTTP 和自定义协议处理程序公开功能

### 特性
- **分布式架构**：内置服务发现和领导者选举支持
  - 自动节点注册和健康检查
  - 分布式领导者选举以实现高可用性
  - 集群状态同步
  
- **配置管理**：使用 etcd 实现集中式配置管理
  - 动态配置更新，无需重启
  - 配置版本控制和回滚支持
  - 配置变更通知
  
- **消息队列**：集成 NATS 服务器实现可靠的消息传递
  - 高性能发布/订阅消息系统
  - 持久化消息存储，支持配置保留策略
  - 消息重放和持久订阅
  
- **缓存服务**：内置缓存服务器提升性能
  - 分布式缓存，支持自动失效
  - 多种存储后端（内存、Redis）
  - 可自定义的缓存策略和 TTL
  
- **系统监控**：全面的系统监控能力
  - 资源使用跟踪（CPU、内存、磁盘）
  - 所有组件的性能指标
  - 可自定义的警报和通知系统
  
- **身份认证**：内置认证管理系统
  - 支持多种认证方法
  - 细粒度访问控制
  - JWT 令牌管理
  
- **API 网关**：基于 Fiber 的 HTTP 服务器
  - 高性能 HTTP 路由
  - 中间件支持常见任务
  - API 版本控制和文档
  
- **协议支持**：自定义协议管理器用于服务间通信
  - 可扩展的协议处理程序
  - 支持自定义二进制协议
  - 协议转换和互操作性

### 应用场景
AIO 特别适合以下场景：

- **微服务架构**：提供可靠微服务所需的基础设施
- **分布式系统**：处理分布式节点之间的复杂协调
- **API 网关**：作为多个后端服务的中央入口点
- **物联网平台**：管理设备通信、数据收集和处理
- **实时应用**：支持低延迟、高吞吐量消息传递
- **边缘计算**：通过优化配置可在资源受限环境中运行

### 环境要求
- Go 1.24 或更高版本
- etcd 3.5.x
- Redis（可选）
- NATS（可选）

### 安装
```bash
# 克隆仓库
git clone https://github.com/xsxdot/aio.git

# 进入项目目录
cd aio

# 安装依赖
go mod download
```

### 配置
1. 复制示例配置文件 `conf/aio.yaml` 并根据您的环境进行修改
2. 基本配置足以进行初始启动
3. 其他配置可以在初始化后通过Web界面完成

基本配置结构如下：
```yaml
errors:
    debug_mode: true
logger:
    compress: true
    console: true
    file: ./logs/aio.log
    level: info
    max_age: 7
    max_backups: 10
    max_size: 100
network:
    allow_external: false
    bind_ip: localhost
    http_allow_external: true
    http_port: 8080
protocol:
    buffer_size: 4096
    enable_auth: true
    enable_keep_alive: true
    heartbeat_timeout: 30s
    idle_timeout: 60s
    max_connections: 1000
    port: 6666
    read_timeout: 30s
    write_timeout: 30s
system:
    config_salt: "123456789087654321"
    data_dir: ./data
    mode: standalone
    node_id: node1
```

### 使用方法
```bash
# 编译应用程序
go build -o aio ./cmd/aio

# 使用基本配置文件启动服务
./aio -config ./conf

# 显示版本信息
./aio -version
```

启动服务后，访问 `http://localhost:8080`（或配置的HTTP端口）的Web界面完成初始化和其他配置。

#### SDK 使用示例
AIO SDK提供了与AIO服务交互的客户端库。以下是在应用程序中使用它的方法：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	
	"github.com/xsxdot/aio/sdk"
)

func main() {
	// 定义服务器端点
	servers := []sdk.ServerEndpoint{
		{
			Address: "localhost:6666",
			Weight:  100,
		},
	}
	
	// 配置客户端选项
	options := &sdk.ClientOptions{
		ClientID:            "client_example",
		ClientSecret:        "client_secret",
		ConnectionTimeout:   30 * time.Second,
		ServiceWatchInterval: 60 * time.Second,
	}
	
	// 创建新客户端
	client := sdk.NewClient(servers, options)
	defer client.Close()
	
	// 连接到服务器
	err := client.Connect()
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	
	// 使用配置服务
	configValue, err := client.Config.GetValue("my.config.key")
	if err != nil {
		log.Printf("配置错误: %v", err)
	} else {
		fmt.Printf("配置值: %s\n", configValue)
	}
	
	// 使用服务发现
	services, err := client.Discovery.GetServices("my-service")
	if err != nil {
		log.Printf("服务发现错误: %v", err)
	} else {
		for _, service := range services {
			fmt.Printf("发现服务: %s 位于 %s\n", service.Name, service.Address)
		}
	}
	
	// 发送自定义命令（示例）
	ctx := context.Background()
	response, err := client.SendRequest(ctx, "system.status", []byte("查询数据"))
	if err != nil {
		log.Printf("请求错误: %v", err)
	} else {
		fmt.Printf("响应: %s\n", string(response))
	}
}

### 项目结构
```
.
├── app/            # 应用程序核心
│   ├── config/     # 配置结构和加载
│   ├── fiber/      # HTTP服务器实现
│   └── const/      # 常量和默认值
├── cmd/            # 命令行入口
│   └── aio/        # 主应用可执行文件
├── conf/           # 配置文件
├── docs/           # 文档
├── internal/       # 内部包
│   ├── authmanager/  # 认证管理
│   ├── cache/      # 缓存实现
│   ├── etcd/       # etcd集成
│   ├── monitoring/ # 系统监控
│   └── mq/         # 消息队列实现
├── pkg/            # 公共包
│   ├── common/     # 通用工具
│   ├── distributed/ # 分布式系统工具
│   └── protocol/   # 协议处理程序
└── sdk/            # 客户端SDK
```

### 未来计划
AIO 框架的未来开发计划包括：

- **集群逻辑优化**：增强集群节点之间的协调和通信，提高稳定性和性能
- **命令执行引擎**：添加跨集群远程执行命令的功能，包括适当的访问控制和审计日志
- **错误日志管理**：实现对使用 AIO 组件的应用程序错误日志的全面收集、聚合和分析
- **分布式链路追踪**：集成分布式追踪功能，实现对跨服务请求的端到端监控，帮助识别瓶颈并诊断问题

我们欢迎对任何这些即将推出的功能进行贡献。请查看问题跟踪器了解当前的开发优先事项。

### 贡献
欢迎贡献代码！请随时提交 Pull Request。

1. Fork 仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交您的更改 (`git commit -m '添加一些令人惊叹的功能'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开 Pull Request

### 许可证
本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。 