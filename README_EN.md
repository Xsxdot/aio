# AIO Service

![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)

AIO is a multifunctional service framework based on the Go language, providing core functionalities including authentication management, distributed coordination, configuration management, and message queuing, helping you build reliable, high-performance distributed applications quickly. By integrating common infrastructure services into a single application, AIO aims to simplify environment dependencies for small and medium-sized projects, providing a rich toolkit to reduce development and operational costs.

## Core Features

- 🔐 **Authentication & Certificate Management**
  - Automatic SSL certificate issuance and renewal
  - Self-generated CA certificates for encrypted communications between etcd, NATS, and other components
  - Built-in JWT authentication support

- 🌐 **Distributed Capabilities**
  - Embedded etcd, no external deployment required
  - Service registration and discovery
  - Leader election mechanism
  - Distributed locks
  - Distributed ID generator
  - Support for cluster mode deployment

- ⚙️ **Configuration Center**
  - etcd-based configuration management
  - Support for multi-environment configurations (dev, test, prod)
  - Hot configuration updates

- ⏰ **Scheduled Tasks**
  - Support for regular scheduled tasks
  - Support for distributed tasks (implemented based on etcd distributed locks)
  - Cron expression support

- 💾 **Cache Service**
  - Embedded cache server
  - Compatible with Redis protocol
  - Can be used as a standalone cache

- 📨 **Message Queue**
  - Embedded NATS message queue
  - Support for publish/subscribe model
  - Support for request/response model
  - Support for queue groups and message persistence

- 📊 **Monitoring Capabilities**
  - System resource monitoring (CPU, memory, disk)
  - Application metrics collection
  - Data visualization

- 🔧 **Extensibility**
  - Modular design, easy to extend and customize
  - Plugin mechanism support

## System Requirements

- Go 1.24 or higher
- Supported operating systems: Linux, macOS, Windows

## Quick Start

### Installation

**Linux/macOS**:
```bash
./install.sh
```

**Windows**:
```cmd
install.bat
```

### Configuration

Configuration files are located in the `./conf` directory by default. Basic configuration example:

```yaml
system:
  mode: standalone  # Options: standalone, cluster
  nodeId: node1
  logLevel: info
```

### Running

```bash
./cmd/aio/aio --config ./conf
```

To view version information:
```bash
./cmd/aio/aio --version
```

## Usage Methods

AIO provides multiple usage methods, including Go SDK and Web management interface.

### Go Client SDK

AIO provides a complete Go client SDK. After generating a key and secret in the admin dashboard, you can use it in your project as follows:

#### 1. Installing the SDK

```bash
go get github.com/xsxdot/aio/client
```

#### 2. Basic Usage

Here's an example of client usage based on a real project:

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
	// Load application configuration
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

	// Connect to AIO service
	hosts := strings.Split(cfg.Aio.Hosts, ",")
	clientOptions := client.NewBuilder("", cfg.AppName, cfg.Port).
		WithDefaultProtocolOptions(hosts, cfg.Aio.ClientId, cfg.Aio.ClientSecret).
		WithEtcdOptions(client.DefaultEtcdOptions).Build()

	aioClient := clientOptions.NewClient()
	err := aioClient.Start(context.Background())
	if err != nil {
		log.Fatalf("Failed to start AIO client: %v", err)
	}
	defer aioClient.Close()

	// Get configuration from the configuration center
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
		log.Fatalf("Failed to get log configuration: %v", err)
	}
	log.Printf("Log level: %s, SLS enabled: %v", logConfig.Level, logConfig.Sls)

	// Service discovery
	services, err := aioClient.Discovery.ListServices(context.Background(), "web")
	if err != nil {
		log.Printf("Failed to get service list: %v", err)
	} else {
		for _, service := range services {
			log.Printf("Service: %s, Address: %s", service.Name, service.Address)
		}
	}
}
```

#### 3. Functional Module Usage Examples

##### Configuration Service

```go
// Configuration environment fallback order
fallbacks := []string{"default"}

// Get configuration from the configuration center for a specific environment and parse it into a struct
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

// Watch for configuration changes
ch := make(chan client.ConfigChange)
err := aioClient.Config.Watch(ctx, fmt.Sprintf("%s.%s", appName, "database"), ch)
go func() {
	for change := range ch {
		log.Printf("Configuration change: %s = %s", change.Key, change.Value)
	}
}()
```

##### Service Discovery

```go
// Register a service
service := &client.Service{
	Name:    "api",
	Version: "1.0.0",
	Address: host,
	Port:    port,
	Tags:    []string{"http", "json"},
}
err := aioClient.Discovery.Register(ctx, service)

// Discover services
services, err := aioClient.Discovery.Discover(ctx, "api", "1.0.0")
```

##### Distributed Locks

```go
// Acquire a lock
lock, err := aioClient.Etcd.Lock(ctx, "my-resource", 30)
if err != nil {
	log.Printf("Failed to acquire lock: %v", err)
} else {
	defer lock.Unlock()
	// Execute operations that require locking
}
```

##### Leader Election

```go
// Participate in election
election, err := aioClient.Etcd.Election("cluster-leader")
if err != nil {
	log.Printf("Failed to create election: %v", err)
}

// Callback for becoming leader
electionCh := make(chan bool)
err = election.Campaign(ctx, "node-1", electionCh)
go func() {
	for isLeader := range electionCh {
		if isLeader {
			log.Println("Became leader")
			// Leader logic
		} else {
			log.Println("Lost leadership")
			// Follower logic
		}
	}
}()
```

##### Message Queue

```go
// Publish a message
err := aioClient.NATS.Publish(ctx, "events.user", []byte(`{"id":1,"name":"User1"}`))

// Subscribe to messages
subscription, err := aioClient.NATS.Subscribe(ctx, "events.user", func(msg []byte) {
	log.Printf("Received message: %s", string(msg))
})
defer subscription.Unsubscribe()
```

##### Cache Service

```go
// Set cache
err := aioClient.Cache.Set(ctx, "user:1", `{"id":1,"name":"User1"}`, 3600)

// Get cache
value, err := aioClient.Cache.Get(ctx, "user:1")
```

##### Scheduled Tasks

```go
// Get scheduler instance
scheduler := aioClient.Scheduler

// Create a regular task that doesn't need a distributed lock
taskID1, err := scheduler.AddTask("email-notification", func(ctx context.Context) error {
    // Execute task logic here
    log.Println("Sending email notification")
    return nil
}, false)
if err != nil {
    log.Printf("Failed to add task: %v", err)
}

// Create a delayed task that needs a distributed lock to ensure only one instance in the cluster executes it
taskID2, err := scheduler.AddDelayTask("cleanup-files", 1*time.Hour, func(ctx context.Context) error {
    // Execute task logic here
    log.Println("Cleaning up temporary files")
    return nil
}, true)
if err != nil {
    log.Printf("Failed to add delayed task: %v", err)
}

// Create a periodic task, executing every 10 minutes, with immediate first execution
taskID3, err := scheduler.AddIntervalTask("health-check", 10*time.Minute, true, func(ctx context.Context) error {
    // Execute task logic here
    log.Println("Performing health check")
    return nil
}, true)
if err != nil {
    log.Printf("Failed to add interval task: %v", err)
}

// Create a Cron-based scheduled task, executing at 3 AM every day
taskID4, err := scheduler.AddCronTask("backup-database", "0 3 * * *", func(ctx context.Context) error {
    // Execute task logic here
    log.Println("Backing up database")
    return nil
}, true)
if err != nil {
    log.Printf("Failed to add Cron task: %v", err)
}

// Cancel a task
err = scheduler.CancelTask(taskID1)
if err != nil {
    log.Printf("Failed to cancel task: %v", err)
}

// Get task information
task, err := scheduler.GetTask(taskID2)
if err != nil {
    log.Printf("Failed to get task information: %v", err)
} else {
    log.Printf("Task ID: %s, Name: %s", task.ID, task.Name)
}

// Get all tasks
allTasks := scheduler.GetAllTasks()
for _, t := range allTasks {
    log.Printf("Task: %s, Next execution time: %v", t.Name, t.NextRunTime)
}
```

##### Monitoring Metrics

```go
// Create monitoring client
monitoringClient, err := client.NewMonitoringClient(
    aioClient, 
    aioClient.NATS.GetConnection(), 
    client.DefaultMonitoringOptions(),
)
if err != nil {
    log.Fatalf("Failed to create monitoring client: %v", err)
}
defer monitoringClient.Stop()

// Track API calls
startTime := time.Now()
// ... Process API request ...
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

// Send custom application metrics
err = monitoringClient.SendCustomAppMetrics(map[string]interface{}{
    "active_users": 1250,
    "pending_tasks": 45,
    "queue_depth": 12,
})
if err != nil {
    log.Printf("Failed to send custom metrics: %v", err)
}

// Send service monitoring data
err = monitoringClient.SendServiceData(map[string]interface{}{
    "database_connections": 25,
    "cache_hit_ratio": 0.85,
    "average_response_time": 42.5,
})
if err != nil {
    log.Printf("Failed to send service monitoring data: %v", err)
}

// Send service API call data
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
    // ... More API call records ...
}
err = monitoringClient.SendServiceAPIData(apiCalls)
if err != nil {
    log.Printf("Failed to send API call data: %v", err)
}
```

### Web Management Interface

AIO provides a powerful Web management interface that can be accessed via a browser to manage all functionalities.

#### Access Method

- After starting the AIO service, access directly through a browser: `http://<server-IP>:<port>`
- The default port is 8080
- The default username and password are both `admin`

#### Main Features

The Web management interface provides the following features:

1. **Dashboard**
   - System overview
   - Resource usage
   - Component status

2. **Configuration Center**
   - View and edit configurations
   - Multi-environment configuration management
   - Configuration version history

3. **Service Management**
   - Service registration list
   - Service health status
   - Service details

4. **Task Scheduling**
   - Task list
   - Create and edit tasks
   - Task execution history
   - Manual task triggering

5. **Monitoring Center**
   - System resource monitoring
   - Custom metrics view
   - Alert rule configuration
   - Alert history

6. **Component Configuration**
   - Message queue management and configuration
   - Cache service management and configuration
   - SSL certificate management and configuration

#### Component Enabling Instructions

AIO enables most functionalities by default, but the following components need to be configured and enabled through the Web management interface:

- **Message Queue (MQ)** - Needs to be configured and enabled in the management interface
- **Cache Service (Cache)** - Needs to be configured and enabled in the management interface
- **SSL Certificate Management** - Needs to be configured and enabled in the management interface

Other components (such as etcd, service discovery, configuration center, scheduled tasks, monitoring) are enabled by default and can be used directly.

## Detailed Feature Description

### Certificate Management

AIO provides two certificate management functions:

1. **SSL Certificate Management**
   - Support for automatic SSL certificate issuance from Let's Encrypt via the ACME protocol
   - Automatic certificate renewal
   - Certificate status monitoring

2. **Internal CA Certificates**
   - Automatic generation of CA root certificates
   - Certificate issuance for service components (such as etcd, NATS)
   - Simplified internal communication encryption configuration

### Distributed Features

AIO has built-in distributed coordination capabilities:

1. **Embedded etcd**
   - No external dependencies required for distributed functionalities
   - Support for single-node and cluster modes
   - Data persistence

2. **Service Discovery**
   - Automatic service registration
   - Service health checks
   - Dynamic service discovery

3. **Leader Election and Distributed Locks**
   - Support for primary/secondary architecture applications
   - Split-brain prevention
   - Distributed lock API simplifies concurrency control

4. **Distributed ID Generator**
   - Support for multiple ID generation strategies
   - Global uniqueness guarantee
   - High-performance design

### Configuration Center

Centralized configuration management:

1. **etcd-based Configuration Storage**
   - Configuration version control
   - Configuration change history

2. **Multi-environment Support**
   - Isolation of development, testing, and production environments
   - Environment inheritance mechanism

3. **Dynamic Configuration**
   - Hot configuration updates
   - Configuration change notifications

### Scheduled Tasks

Powerful task scheduling capabilities:

1. **Local Tasks**
   - Based on Cron expressions
   - Task dependencies

2. **Distributed Tasks**
   - Cluster-wide task coordination
   - Single execution guarantee
   - Failure retry mechanism

3. **Task Management**
   - Visual task status
   - Manual triggering
   - Execution logs

### Cache Service

Built-in cache service:

1. **Redis Protocol Compatibility**
   - No client modifications required for use
   - Support for major Redis commands

2. **Storage Options**
   - Memory storage
   - Persistence option
   - TTL support

### Message Queue

Integrated NATS message queue:

1. **Message Patterns**
   - Publish/subscribe
   - Request/response
   - Queue groups

2. **Persistence**
   - Message persistence options
   - At-least-once delivery guarantee
   - Message replay

### Monitoring

Comprehensive monitoring capabilities:

1. **System Monitoring**
   - CPU, memory, network, and disk usage
   - Process monitoring

2. **Application Monitoring**
   - Custom metrics
   - Performance analysis
   - Log aggregation

## Project Structure

```
.
├── app/          # Application core code
├── cmd/          # Command-line entry
├── conf/         # Configuration files
├── docs/         # Documentation
├── internal/     # Internal packages
│   ├── authmanager/    # Authentication management
│   ├── cache/          # Cache service
│   ├── certmanager/    # Certificate management
│   ├── etcd/           # etcd component
│   ├── monitoring/     # Monitoring component
│   └── mq/             # Message queue
├── pkg/          # Public packages
│   ├── auth/           # Authentication related
│   ├── common/         # Common utilities
│   ├── config/         # Configuration related
│   ├── distributed/    # Distributed features
│   ├── protocol/       # Protocol related
│   └── scheduler/      # Task scheduling
├── client/       # Go language client SDK
└── web/          # Web management interface
```

## Development

### Building

```bash
./build.sh
```

### Testing

```bash
go test ./...
```

## Use Cases

AIO is suitable for the following scenarios:

1. **Microservice Infrastructure**
   - Reduce the complexity of microservice infrastructure
   - Provide core capabilities for service registration, discovery, and configuration management

2. **Small and Medium Application Architecture**
   - Simplify application dependencies
   - Reduce operational costs

3. **Edge Computing**
   - Distributed systems in resource-constrained environments
   - Reduce external dependencies

4. **Development and Testing Environments**
   - Quickly set up full-featured development environments
   - Reduce problems caused by environment differences

## Configuration Reference

For detailed configuration, please refer to [docs/config.md](./docs/config.md)

## Contribution Guidelines

We welcome all forms of contribution, including but not limited to:

1. Submitting issues and suggestions
2. Submitting Pull Requests
3. Improving documentation

Before submitting a PR, please ensure:
- Code follows Go's standard format (use `go fmt`)
- All test cases pass
- New features have corresponding tests
- Related documentation has been updated

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Contact and Support

- Issue feedback: Submit issues on GitHub
- Email contact: [Maintainer Email]
- Discussion group: [Discussion group link, if any]

## Acknowledgements

Thanks to all contributors and the following open-source projects for their support:

- [etcd](https://github.com/etcd-io/etcd) - Distributed key-value store
- [NATS](https://github.com/nats-io/nats-server) - High-performance messaging system
- [Fiber](https://github.com/gofiber/fiber) - Web framework
- [Badger](https://github.com/dgraph-io/badger) - Key-value database
- [Go Redis](https://github.com/redis/go-redis) - Redis client
- [Zap](https://github.com/uber-go/zap) - Logging library 