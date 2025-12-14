package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiaozhizhang/system/systemd/internal/model/dto"
)

// TestWriteToFile_Success 测试成功写入文件
func TestWriteToFile_Success(t *testing.T) {
	svc := setupTestService(t)

	// 创建临时目录
	tempDir := t.TempDir()
	t.Logf("Using temp directory: %s", tempDir)

	params := &dto.ServiceUnitParams{
		Description: "Test Service for File Write",
		ExecStart:   "/usr/bin/test-app",
		User:        "test",
		Group:       "test",
	}

	// 生成内容
	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 写入文件
	filename := filepath.Join(tempDir, "test.service")
	err = os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("File %s does not exist after writing", filename)
	}

	// 读取文件内容并验证
	readContent, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(readContent) != content {
		t.Errorf("File content mismatch.\nExpected:\n%s\n\nGot:\n%s", content, string(readContent))
	}

	t.Logf("Successfully wrote and verified file: %s", filename)
	t.Logf("File content:\n%s", string(readContent))
}

// TestWriteToFile_MultipleFiles 测试写入多个文件
func TestWriteToFile_MultipleFiles(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()
	t.Logf("Using temp directory: %s", tempDir)

	testCases := []struct {
		name   string
		params *dto.ServiceUnitParams
	}{
		{
			name: "web.service",
			params: &dto.ServiceUnitParams{
				Description: "Web Application",
				ExecStart:   "/usr/bin/web-app",
				User:        "webapp",
			},
		},
		{
			name: "api.service",
			params: &dto.ServiceUnitParams{
				Description: "API Server",
				ExecStart:   "/usr/bin/api-server",
				User:        "api",
			},
		},
		{
			name: "worker.service",
			params: &dto.ServiceUnitParams{
				Description: "Background Worker",
				ExecStart:   "/usr/bin/worker",
				User:        "worker",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := svc.Generate(tc.params)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			filename := filepath.Join(tempDir, tc.name)
			err = os.WriteFile(filename, []byte(content), 0644)
			if err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			// 验证文件存在
			info, err := os.Stat(filename)
			if err != nil {
				t.Fatalf("Stat() error = %v", err)
			}

			// 验证文件权限
			if info.Mode().Perm() != 0644 {
				t.Errorf("File permission mismatch. Expected: 0644, Got: %v", info.Mode().Perm())
			}

			// 验证内容
			readContent, err := os.ReadFile(filename)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			if !strings.Contains(string(readContent), tc.params.Description) {
				t.Errorf("File content missing expected Description: %s", tc.params.Description)
			}

			t.Logf("Successfully wrote file: %s", filename)
		})
	}

	// 列出临时目录中的所有文件
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	t.Logf("Files in temp directory:")
	for _, entry := range entries {
		t.Logf("  - %s", entry.Name())
	}

	if len(entries) != len(testCases) {
		t.Errorf("Expected %d files, got %d", len(testCases), len(entries))
	}
}

// TestWriteToFile_Overwrite 测试覆盖已存在的文件
func TestWriteToFile_Overwrite(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "overwrite.service")

	// 第一次写入
	params1 := &dto.ServiceUnitParams{
		Description: "Original Service",
		ExecStart:   "/usr/bin/original",
	}
	content1, err := svc.Generate(params1)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	err = os.WriteFile(filename, []byte(content1), 0644)
	if err != nil {
		t.Fatalf("WriteFile() first write error = %v", err)
	}

	// 读取第一次写入的内容
	readContent1, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile() first read error = %v", err)
	}
	if !strings.Contains(string(readContent1), "Original Service") {
		t.Error("First write content missing expected description")
	}

	t.Logf("First write content:\n%s", string(readContent1))

	// 第二次写入（覆盖）
	params2 := &dto.ServiceUnitParams{
		Description: "Updated Service",
		ExecStart:   "/usr/bin/updated",
	}
	content2, err := svc.Generate(params2)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	err = os.WriteFile(filename, []byte(content2), 0644)
	if err != nil {
		t.Fatalf("WriteFile() second write error = %v", err)
	}

	// 读取第二次写入的内容
	readContent2, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile() second read error = %v", err)
	}

	// 验证内容已更新
	if !strings.Contains(string(readContent2), "Updated Service") {
		t.Error("Second write content missing expected description")
	}
	if strings.Contains(string(readContent2), "Original Service") {
		t.Error("Second write content still contains old description")
	}

	t.Logf("Second write content:\n%s", string(readContent2))
}

// TestWriteToFile_NestedDirectory 测试在嵌套目录中写入文件
func TestWriteToFile_NestedDirectory(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()

	// 创建嵌套目录结构
	nestedDir := filepath.Join(tempDir, "system", "multi-user.target.wants")
	err := os.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	params := &dto.ServiceUnitParams{
		Description: "Nested Service",
		ExecStart:   "/usr/bin/nested-app",
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	filename := filepath.Join(nestedDir, "nested.service")
	err = os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("File %s does not exist after writing", filename)
	}

	t.Logf("Successfully wrote file in nested directory: %s", filename)
}

// TestWriteToFile_SpecialCharacters 测试包含特殊字符的文件名
func TestWriteToFile_SpecialCharacters(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()

	testCases := []struct {
		name     string
		filename string
	}{
		{
			name:     "带连字符",
			filename: "my-app.service",
		},
		{
			name:     "带下划线",
			filename: "my_app.service",
		},
		{
			name:     "带点",
			filename: "my.app.service",
		},
		{
			name:     "带@符号（实例化服务）",
			filename: "my-app@.service",
		},
	}

	params := &dto.ServiceUnitParams{
		Description: "Test Service",
		ExecStart:   "/usr/bin/test",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := svc.Generate(params)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			filename := filepath.Join(tempDir, tc.filename)
			err = os.WriteFile(filename, []byte(content), 0644)
			if err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			if _, err := os.Stat(filename); os.IsNotExist(err) {
				t.Fatalf("File %s does not exist after writing", filename)
			}

			t.Logf("Successfully wrote file: %s", filename)
		})
	}
}

// TestWriteToFile_LargeContent 测试写入大内容
func TestWriteToFile_LargeContent(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()

	// 创建一个包含大量环境变量和命令的参数
	var envVars []string
	var execStartPre []string
	for i := 0; i < 100; i++ {
		envVars = append(envVars, "VAR_"+strings.Repeat("A", i%10)+"="+strings.Repeat("value", i%5))
		execStartPre = append(execStartPre, "/bin/echo Preparing step "+strings.Repeat("X", i%10))
	}

	params := &dto.ServiceUnitParams{
		Description:  "Large Content Service",
		ExecStart:    "/usr/bin/app " + strings.Repeat("--flag ", 50),
		ExecStartPre: execStartPre,
		Environment:  envVars,
		After:        []string{"network.target", "syslog.target", "remote-fs.target", "nss-lookup.target"},
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	t.Logf("Generated content size: %d bytes", len(content))

	filename := filepath.Join(tempDir, "large.service")
	err = os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// 验证文件大小
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if info.Size() != int64(len(content)) {
		t.Errorf("File size mismatch. Expected: %d, Got: %d", len(content), info.Size())
	}

	// 读取并验证内容
	readContent, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(readContent) != content {
		t.Error("File content mismatch for large content")
	}

	t.Logf("Successfully wrote and verified large file (%d bytes): %s", info.Size(), filename)
}

// TestWriteToFile_EmptyDirectory 测试在空目录中写入文件
func TestWriteToFile_EmptyDirectory(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()

	// 确认目录为空
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Expected empty directory, got %d entries", len(entries))
	}

	params := &dto.ServiceUnitParams{
		Description: "First Service",
		ExecStart:   "/usr/bin/first",
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	filename := filepath.Join(tempDir, "first.service")
	err = os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// 确认目录现在有一个文件
	entries, err = os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Name() != "first.service" {
		t.Errorf("Expected filename 'first.service', got '%s'", entries[0].Name())
	}

	t.Logf("Successfully wrote first file to empty directory: %s", filename)
}

// TestWriteToFile_FilePermissions 测试不同的文件权限
func TestWriteToFile_FilePermissions(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()

	params := &dto.ServiceUnitParams{
		Description: "Permission Test Service",
		ExecStart:   "/usr/bin/test",
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	testCases := []struct {
		name string
		perm os.FileMode
	}{
		{"只读 (0444)", 0444},
		{"标准 (0644)", 0644},
		{"可执行 (0755)", 0755},
		{"严格权限 (0600)", 0600},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filename := filepath.Join(tempDir, "perm_"+tc.name+".service")
			err = os.WriteFile(filename, []byte(content), tc.perm)
			if err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			info, err := os.Stat(filename)
			if err != nil {
				t.Fatalf("Stat() error = %v", err)
			}

			if info.Mode().Perm() != tc.perm {
				t.Errorf("File permission mismatch. Expected: %v, Got: %v", tc.perm, info.Mode().Perm())
			}

			t.Logf("Successfully wrote file with permission %v: %s", tc.perm, filename)
		})
	}
}

// TestWriteToFile_ConcurrentWrites 测试并发写入不同文件
func TestWriteToFile_ConcurrentWrites(t *testing.T) {
	svc := setupTestService(t)
	tempDir := t.TempDir()

	concurrency := 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			params := &dto.ServiceUnitParams{
				Description: "Concurrent Service " + strings.Repeat("*", index),
				ExecStart:   "/usr/bin/app-" + strings.Repeat("x", index),
			}

			content, err := svc.Generate(params)
			if err != nil {
				errors <- err
				done <- false
				return
			}

			filename := filepath.Join(tempDir, "concurrent_"+strings.Repeat("x", index)+".service")
			err = os.WriteFile(filename, []byte(content), 0644)
			if err != nil {
				errors <- err
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	successCount := 0
	for i := 0; i < concurrency; i++ {
		if <-done {
			successCount++
		}
	}
	close(errors)

	// 检查是否有错误
	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
	}

	if successCount != concurrency {
		t.Errorf("Expected %d successful writes, got %d", concurrency, successCount)
	}

	// 验证所有文件都存在
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(entries) != concurrency {
		t.Errorf("Expected %d files, got %d", concurrency, len(entries))
	}

	t.Logf("Successfully completed %d concurrent writes", successCount)
}




