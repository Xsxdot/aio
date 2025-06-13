# AIO All-In-One 分布式服务

本项目旨在通过单一软件包中提供必要的分布式服务组件，以ETCD为基础核心，可单点运行，也可以集群运行，提供一个完整的分布式系统解决方案。

## 🚀 项目特性

- ✅ **基础架构**: 完整的项目结构和配置管理
- ✅ **日志系统**: 基于logrus的结构化日志
- ✅ **自定义错误**: 统一的错误处理和HTTP状态码映射
- 🚧 **服务注册与发现**: 基于ETCD的服务注册发现机制
- 🚧 **分布式锁**: 高可用的分布式锁服务
- 🚧 **配置中心**: 集中化配置管理和热更新
- 🚧 **定时任务**: 分布式定时任务调度
- 🚧 **SSL证书**: 自动证书管理和部署

## 📁 项目结构

```
aio/
├── cmd/                     # 应用程序入口
│   ├── server/             # 主服务器
│   │   ├── main.go         # 主入口文件
│   │   └── app.go          # 应用程序结构
│   └── client/             # 客户端工具 (待开发)
├── internal/               # 内部包
│   ├── config/            # 配置管理
│   │   └── config.go      # 配置结构和加载逻辑
│   ├── registry/          # 服务注册与发现 (待开发)
│   ├── lock/              # 分布式锁 (待开发)
│   ├── scheduler/         # 分布式定时任务 (待开发)
│   ├── cert/              # SSL证书管理 (待开发)
│   ├── web/               # Web服务(Fiber) (待开发)
│   ├── tcp/               # TCP通信 (待开发)
│   └── etcd/              # ETCD客户端封装 (待开发)
├── pkg/                    # 公共包
│   ├── logger/            # 日志模块
│   │   └── logger.go      # 日志接口和实现
│   ├── errors/            # 自定义错误
│   │   ├── error.go       # 错误定义和处理
│   │   └── error_test.go  # 错误处理测试
│   ├── utils/             # 工具函数
│   │   ├── string.go      # 字符串工具
│   │   └── time.go        # 时间工具
│   └── proto/             # 协议定义 (待开发)
├── configs/               # 配置文件
│   └── config.yaml        # 示例配置文件
├── docs/                  # 文档 (待完善)
├── scripts/               # 脚本 (待开发)
├── deployments/           # 部署文件 (待开发)
├── go.mod                 # Go模块文件
└── README.md              # 项目说明
```

## ⚡ 快速开始

### 1. 环境要求

- Go 1.24+
- ETCD 3.5+ (可选，用于分布式功能)

### 2. 克隆项目

```bash
git clone <repository-url>
cd aio/go
```

### 3. 安装依赖

```bash
go mod tidy
```

### 4. 运行服务

```bash
# 使用默认配置
go run cmd/server/*.go

# 使用指定配置文件
go run cmd/server/*.go -config ./configs/config.yaml

# 查看帮助
go run cmd/server/*.go -help

# 查看版本
go run cmd/server/*.go -version
```

### 5. 测试服务

```bash
# 健康检查
curl http://localhost:8080/health

# 服务信息
curl http://localhost:8080/info

# 访问Web界面
open http://localhost:8080
```

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

## 📋 已实现功能

### ✅ 基础架构

- [x] 项目目录结构
- [x] Go模块初始化
- [x] 主程序入口
- [x] 应用程序生命周期管理

### ✅ 配置管理

- [x] 基于Viper的配置加载
- [x] YAML配置文件支持
- [x] 环境变量支持
- [x] 配置验证
- [x] 默认配置

### ✅ 日志系统

- [x] 基于Logrus的结构化日志
- [x] 多种日志级别 (debug, info, warn, error, fatal, panic)
- [x] 多种输出格式 (text, json)
- [x] 多种输出目标 (stdout, stderr, file)
- [x] 调用者信息显示
- [x] 自定义格式化器

### ✅ 自定义错误

- [x] 结构化错误定义
- [x] 错误码和HTTP状态码映射
- [x] 错误包装和链式调用
- [x] 便捷错误创建函数
- [x] 错误处理测试

### ✅ 工具函数

- [x] 字符串处理工具
- [x] 时间处理工具
- [x] 命名转换 (camelCase, snake_case, kebab-case)

### ✅ HTTP服务

- [x] 基础HTTP服务器
- [x] 健康检查端点
- [x] 服务信息端点
- [x] 美观的Web界面
- [x] 优雅关闭

## 🚧 待开发功能

### ETCD客户端

- [ ] ETCD连接管理
- [ ] 键值存储操作
- [ ] 监听和事件处理
- [ ] 集群管理

### 服务注册与发现

- [ ] 服务注册
- [ ] 服务发现
- [ ] 健康检查
- [ ] 负载均衡策略

### 分布式锁

- [ ] 互斥锁
- [ ] 读写锁
- [ ] 可重入锁
- [ ] 锁超时和续期

### 定时任务调度

- [ ] Cron表达式支持
- [ ] 任务分片
- [ ] 故障转移
- [ ] 任务监控

### SSL证书管理

- [ ] Let's Encrypt集成
- [ ] 自签名证书生成
- [ ] 证书自动续期
- [ ] 多域名支持

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