# Unit Generator Service 测试指南

## 快速开始

### 运行所有测试

```bash
cd /Users/xushixin/workspace/go/xiaozhizhang
go test -v ./system/systemd/internal/service/...
```

### 运行特定测试类别

```bash
# 只运行 Generate 相关测试（生成功能）
go test -v ./system/systemd/internal/service -run TestGenerate

# 只运行 Validate 相关测试（参数校验）
go test -v ./system/systemd/internal/service -run TestValidate

# 只运行文件写入测试
go test -v ./system/systemd/internal/service -run TestWriteToFile
```

### 查看测试覆盖率

```bash
# 简单覆盖率
go test ./system/systemd/internal/service/... -cover

# 生成详细覆盖率报告
go test ./system/systemd/internal/service/... -coverprofile=coverage.out
go tool cover -func=coverage.out

# 生成 HTML 覆盖率报告（浏览器中查看）
go tool cover -html=coverage.out
```

## 测试文件说明

| 文件 | 说明 | 测试用例数 |
|------|------|-----------|
| `unit_generator_service_test.go` | unit 文件生成测试 | 17 个 |
| `unit_file_test.go` | 文件写入与保存测试 | 14 个 |

## 测试内容

### 1. 生成功能测试

- ✅ 完整参数生成
- ✅ 最小参数生成
- ✅ 默认值处理
- ✅ 扩展行功能
- ✅ 多值字段
- ✅ 整型字段
- ✅ 段落分隔
- ✅ 真实场景（Go Web、Nginx、定时任务）

### 2. 参数校验测试

- ✅ 空参数校验
- ✅ ExecStart 必填校验
- ✅ 换行符注入防护
- ✅ 字符串字段校验
- ✅ 数组字段校验

### 3. 文件操作测试

- ✅ 基本文件写入
- ✅ 多文件写入
- ✅ 文件覆盖
- ✅ 嵌套目录
- ✅ 特殊字符文件名
- ✅ 大内容文件
- ✅ 文件权限
- ✅ 并发写入

## 临时目录说明

所有文件写入测试都使用 Go 的 `testing.T.TempDir()`：

```go
tempDir := t.TempDir()  // 自动创建和清理
```

**特点：**
- 每个测试独立的临时目录
- 测试结束后自动清理
- macOS 示例路径：`/var/folders/hx/.../T/TestName.../001`
- 无需 root 权限
- 不影响系统文件

## 测试示例

### 示例 1：生成最小配置

```go
func TestGenerate_MinimalParams(t *testing.T) {
    svc := setupTestService(t)
    params := &dto.ServiceUnitParams{
        ExecStart: "/usr/bin/test-app",
    }
    content, err := svc.Generate(params)
    // 验证生成的 unit 文件内容
}
```

### 示例 2：写入文件

```go
func TestWriteToFile_Success(t *testing.T) {
    svc := setupTestService(t)
    tempDir := t.TempDir()  // 自动管理的临时目录
    
    params := &dto.ServiceUnitParams{
        Description: "Test Service",
        ExecStart:   "/usr/bin/app",
    }
    
    content, _ := svc.Generate(params)
    filename := filepath.Join(tempDir, "test.service")
    os.WriteFile(filename, []byte(content), 0644)
    
    // 验证文件内容
}
```

## 常见问题

### Q: 测试会影响系统文件吗？
**A:** 不会。所有测试都在临时目录中进行，测试结束后自动清理。

### Q: 需要 systemctl 命令吗？
**A:** 不需要。测试只验证文件生成和写入逻辑，不执行实际的 systemctl 命令。

### Q: Mac 上可以运行吗？
**A:** 可以。所有测试都兼容 macOS，不依赖 Linux 特定功能。

### Q: 如何调试单个测试用例？
**A:** 使用 `-run` 参数：
```bash
go test -v ./system/systemd/internal/service -run TestGenerate_FullParams
```

### Q: 如何查看生成的文件内容？
**A:** 运行测试时加上 `-v` 参数，测试会输出生成的内容：
```bash
go test -v ./system/systemd/internal/service -run TestGenerate_FullParams
```

## 测试覆盖率目标

| 模块 | 当前覆盖率 | 目标 |
|------|-----------|------|
| Generate() | 100% | ✅ |
| validate() | 82.5% | ✅ |
| checkNoNewlines() | 100% | ✅ |
| NewUnitGeneratorService() | 100% | ✅ |

## CI/CD 集成

在 CI/CD 管道中运行测试：

```bash
# 基础测试
go test ./system/systemd/internal/service/... -v

# 带覆盖率
go test ./system/systemd/internal/service/... -cover -coverprofile=coverage.out

# 检查覆盖率阈值（例如要求 80%）
go test ./system/systemd/internal/service/... -cover | grep -E "coverage: [0-9]+\.[0-9]+%" | awk '{if ($2+0 < 80) exit 1}'
```

## 添加新测试

1. 在 `unit_generator_service_test.go` 或 `unit_file_test.go` 中添加新函数
2. 函数名以 `Test` 开头
3. 使用 `setupTestService(t)` 创建测试服务
4. 使用 `t.TempDir()` 创建临时目录（如需文件操作）
5. 运行 `go test` 验证

示例：

```go
func TestYourNewFeature(t *testing.T) {
    svc := setupTestService(t)
    
    // 准备测试数据
    params := &dto.ServiceUnitParams{
        // ...
    }
    
    // 执行测试
    content, err := svc.Generate(params)
    
    // 验证结果
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }
    if !strings.Contains(content, "expected") {
        t.Errorf("missing expected content")
    }
}
```

## 性能测试

运行基准测试（如果需要）：

```bash
go test -bench=. ./system/systemd/internal/service/...
```

## 清理

测试会自动清理临时文件，但如果需要手动清理覆盖率文件：

```bash
rm -f coverage.out
```

## 参考文档

- [Go Testing 包文档](https://pkg.go.dev/testing)
- [systemd.service 文档](https://www.freedesktop.org/software/systemd/man/systemd.service.html)
- [项目测试总结](./TEST_SUMMARY.md)

---

**最后更新**: 2025-12-13  
**测试环境**: macOS, Go 1.21+




