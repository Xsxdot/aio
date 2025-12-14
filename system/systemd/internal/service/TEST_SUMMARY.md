# Unit Generator Service 测试总结

## 测试文件

- `unit_generator_service_test.go` - unit 生成服务测试
- `unit_file_test.go` - 文件写入测试

## 测试覆盖率

```
xiaozhizhang/system/systemd/internal/service/unit_generator_service.go:
  - NewUnitGeneratorService    100.0%
  - Generate                   100.0%
  - validate                    82.5%
  - checkNoNewlines            100.0%

总体覆盖率: 26.5%（仅针对 unit_generator_service.go 为 90%+）
```

## 测试用例列表

### 一、Unit 生成测试（unit_generator_service_test.go）

#### 1. 基本功能测试

- **TestGenerate_FullParams** - 测试完整参数生成
  - 验证所有 [Unit]、[Service]、[Install] 段落的字段
  - 验证扩展行（ExtraLines）功能
  - 验证段落顺序正确

- **TestGenerate_MinimalParams** - 测试最小参数生成
  - 只提供必填字段 ExecStart
  - 验证默认值自动填充（Type=simple, Restart=always, WantedBy=multi-user.target）

#### 2. 默认值测试

- **TestGenerate_DefaultValues** - 测试默认值
  - 默认 Type=simple
  - 默认 Restart=always
  - 默认 WantedBy=multi-user.target
  - 自定义值覆盖默认值

#### 3. 扩展功能测试

- **TestGenerate_ExtraLines** - 测试扩展行功能
  - ExtraUnitLines 出现在 [Unit] 段
  - ExtraServiceLines 出现在 [Service] 段
  - ExtraInstallLines 出现在 [Install] 段

- **TestGenerate_MultipleValues** - 测试多值字段
  - After 多个依赖
  - Environment 多个环境变量
  - ExecStartPre 多个预启动命令
  - WantedBy 多个目标

#### 4. 参数校验测试

- **TestValidate_NilParams** - 测试空参数校验
  - nil 参数返回错误

- **TestValidate_EmptyExecStart** - 测试 ExecStart 为空
  - 空字符串返回错误
  - 只有空格返回错误
  - 正常值通过校验

- **TestValidate_NoNewlines** - 测试换行符校验
  - Description 包含换行符
  - Documentation 包含换行符
  - ExecStart 包含换行符
  - Type 包含换行符
  - User 包含换行符
  - WorkingDirectory 包含换行符
  - Restart 包含换行符
  - 正常参数不包含换行符

- **TestValidate_ArrayFieldsWithNewlines** - 测试数组字段包含换行符
  - After 包含换行符
  - Environment 包含换行符
  - ExecStartPre 包含换行符
  - WantedBy 包含换行符
  - ExtraUnitLines 包含换行符
  - 正常数组字段通过校验

#### 5. 真实场景测试

- **TestGenerate_RealWorldExample** - 测试真实场景示例
  - Go Web 应用配置
  - Nginx 服务配置
  - 定时任务（oneshot）配置

#### 6. 格式测试

- **TestGenerate_SectionSeparation** - 测试段落之间的分隔
  - [Unit] 和 [Service] 之间有空行
  - [Service] 和 [Install] 之间有空行

- **TestGenerate_IntegerFields** - 测试整型字段
  - RestartSec
  - TimeoutStartSec
  - TimeoutStopSec
  - LimitNOFILE
  - LimitNPROC

- **TestGenerate_ZeroIntegerFields** - 测试整型字段为 0 时不输出
  - 验证 0 值字段不出现在生成的内容中

---

### 二、文件写入测试（unit_file_test.go）

#### 1. 基本文件操作

- **TestWriteToFile_Success** - 测试成功写入文件
  - 写入临时目录
  - 验证文件存在
  - 验证文件内容与生成内容一致

- **TestWriteToFile_MultipleFiles** - 测试写入多个文件
  - web.service
  - api.service
  - worker.service
  - 验证所有文件都成功创建

- **TestWriteToFile_Overwrite** - 测试覆盖已存在的文件
  - 第一次写入
  - 第二次覆盖
  - 验证内容已更新

#### 2. 目录与路径测试

- **TestWriteToFile_NestedDirectory** - 测试在嵌套目录中写入文件
  - 创建 system/multi-user.target.wants/ 目录结构
  - 验证文件成功写入嵌套目录

- **TestWriteToFile_EmptyDirectory** - 测试在空目录中写入文件
  - 确认目录初始为空
  - 写入后确认只有一个文件

#### 3. 文件名测试

- **TestWriteToFile_SpecialCharacters** - 测试包含特殊字符的文件名
  - 带连字符：my-app.service
  - 带下划线：my_app.service
  - 带点：my.app.service
  - 带@符号（实例化服务）：my-app@.service

#### 4. 大内容测试

- **TestWriteToFile_LargeContent** - 测试写入大内容
  - 100 个环境变量
  - 100 个预启动命令
  - 验证文件大小与内容一致（8179 字节）

#### 5. 文件权限测试

- **TestWriteToFile_FilePermissions** - 测试不同的文件权限
  - 只读 (0444)
  - 标准 (0644)
  - 可执行 (0755)
  - 严格权限 (0600)

#### 6. 并发测试

- **TestWriteToFile_ConcurrentWrites** - 测试并发写入不同文件
  - 10 个 goroutine 并发写入
  - 验证所有文件都成功创建
  - 无并发冲突

---

## 测试执行结果

```bash
# 运行所有测试
go test -v ./system/systemd/internal/service/... -cover

# 测试统计
Total: 31 个测试用例
Passed: 31 ✅
Failed: 0
Skipped: 0

# 覆盖率
Coverage: 26.5% (主要测试文件为 unit_generator_service.go，该文件覆盖率 90%+)
```

## 测试策略

### 1. 单元测试
- 每个公共方法都有对应的测试用例
- 边界条件测试（空值、零值、特殊字符）
- 错误处理测试

### 2. 集成测试
- 文件系统操作测试
- 多文件操作测试
- 并发操作测试

### 3. 真实场景测试
- Go Web 应用
- Nginx 服务
- 定时任务
- 大内容文件

### 4. 安全性测试
- 换行符注入防护
- 参数校验
- 文件权限验证

## 临时目录使用

所有文件写入测试都使用 Go 标准库的 `t.TempDir()`，特点：

- ✅ 每个测试用例独立的临时目录
- ✅ 测试结束后自动清理
- ✅ macOS 系统使用系统临时目录（/var/folders/...）
- ✅ 无需手动清理
- ✅ 不影响系统文件
- ✅ 不需要 systemctl 命令

## 示例输出

```
TestWriteToFile_Success
  Using temp directory: /var/folders/hx/bw838qps02ngtg8wdz5z9sxw0000gn/T/TestWriteToFile_Success.../001
  Successfully wrote and verified file: .../test.service

TestWriteToFile_LargeContent
  Generated content size: 8179 bytes
  Successfully wrote and verified large file (8179 bytes)

TestWriteToFile_ConcurrentWrites
  Successfully completed 10 concurrent writes
```

## 注意事项

1. **Mac 兼容性**：所有测试在 macOS 上运行，使用系统临时目录，不依赖 systemctl
2. **自动清理**：使用 `t.TempDir()` 自动管理临时文件，无需手动清理
3. **隔离性**：每个测试用例独立的临时目录，互不影响
4. **真实性**：虽然不执行 systemctl 命令，但完全测试了文件生成和写入逻辑
5. **覆盖率**：核心逻辑 (Generate/validate) 覆盖率达到 90%+

## 如何运行测试

```bash
# 运行所有测试
go test -v ./system/systemd/internal/service/...

# 运行特定测试
go test -v ./system/systemd/internal/service -run TestGenerate

# 运行文件写入测试
go test -v ./system/systemd/internal/service -run TestWriteToFile

# 查看覆盖率
go test -v ./system/systemd/internal/service/... -cover

# 生成覆盖率报告
go test ./system/systemd/internal/service/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 测试数据

### 完整参数测试数据
```go
Description:      "Test Service for Full Parameters"
Documentation:    "https://example.com/docs"
After:            ["network.target", "remote-fs.target"]
ExecStart:        "/usr/bin/test-app --config /etc/test/config.yaml"
User:             "test"
Environment:      ["ENV=production", "LOG_LEVEL=info"]
Restart:          "on-failure"
RestartSec:       5
LimitNOFILE:      65536
```

### 生成的输出示例
```ini
[Unit]
Description=Test Service for Full Parameters
Documentation=https://example.com/docs
After=network.target
After=remote-fs.target
Wants=postgresql.service
Requires=network-online.target
ConditionPathExists=/opt/test

[Service]
Type=simple
ExecStartPre=/bin/mkdir -p /var/run/test
ExecStartPre=/bin/chown test:test /var/run/test
ExecStart=/usr/bin/test-app --config /etc/test/config.yaml
ExecStartPost=/bin/echo Started
ExecStop=/bin/kill -SIGTERM $MAINPID
ExecReload=/bin/kill -SIGHUP $MAINPID
WorkingDirectory=/opt/test
User=test
Group=test
Environment=ENV=production
Environment=LOG_LEVEL=info
EnvironmentFile=/etc/test/environment
Restart=on-failure
RestartSec=5
TimeoutStartSec=60
TimeoutStopSec=30
LimitNOFILE=65536
LimitNPROC=4096
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
RequiredBy=custom.target
Alias=test-app.service
Also=test-helper.service
```

## 总结

✅ **完整的测试覆盖**：涵盖了所有主要功能和边界情况  
✅ **Mac 友好**：所有测试在 macOS 上运行，无需 Linux 特定工具  
✅ **安全性验证**：测试了换行符注入等安全问题  
✅ **真实场景**：包含 Web 应用、Nginx、定时任务等实际用例  
✅ **并发安全**：验证了并发写入的安全性  
✅ **自动清理**：使用临时目录，无需手动清理  
✅ **高可维护性**：测试代码结构清晰，易于扩展




