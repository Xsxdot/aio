package service

import (
	"strings"
	"testing"

	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/systemd/internal/model/dto"
)

// setupTestService 创建测试用的 UnitGeneratorService
func setupTestService(t *testing.T) *UnitGeneratorService {
	log := logger.GetLogger()
	return NewUnitGeneratorService(log)
}

// TestGenerate_FullParams 测试完整参数生成
func TestGenerate_FullParams(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		// [Unit] 段
		Description:   "Test Service for Full Parameters",
		Documentation: "https://example.com/docs",
		After:         []string{"network.target", "remote-fs.target"},
		Wants:         []string{"postgresql.service"},
		Requires:      []string{"network-online.target"},

		// [Service] 段
		Type:             "simple",
		ExecStart:        "/usr/bin/test-app --config /etc/test/config.yaml",
		ExecStartPre:     []string{"/bin/mkdir -p /var/run/test", "/bin/chown test:test /var/run/test"},
		ExecStartPost:    []string{"/bin/echo Started"},
		ExecStop:         "/bin/kill -SIGTERM $MAINPID",
		ExecReload:       "/bin/kill -SIGHUP $MAINPID",
		WorkingDirectory: "/opt/test",
		User:             "test",
		Group:            "test",
		Environment:      []string{"ENV=production", "LOG_LEVEL=info"},
		EnvironmentFile:  "/etc/test/environment",
		Restart:          "on-failure",
		RestartSec:       5,
		TimeoutStartSec:  60,
		TimeoutStopSec:   30,
		LimitNOFILE:      65536,
		LimitNPROC:       4096,

		// [Install] 段
		WantedBy:   []string{"multi-user.target"},
		RequiredBy: []string{"custom.target"},
		Alias:      []string{"test-app.service"},

		// 扩展行
		ExtraUnitLines:    []string{"ConditionPathExists=/opt/test"},
		ExtraServiceLines: []string{"StandardOutput=journal", "StandardError=journal"},
		ExtraInstallLines: []string{"Also=test-helper.service"},
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 验证生成的内容包含预期的关键信息
	expectedStrings := []string{
		"[Unit]",
		"Description=Test Service for Full Parameters",
		"Documentation=https://example.com/docs",
		"After=network.target",
		"After=remote-fs.target",
		"Wants=postgresql.service",
		"Requires=network-online.target",
		"ConditionPathExists=/opt/test",

		"[Service]",
		"Type=simple",
		"ExecStartPre=/bin/mkdir -p /var/run/test",
		"ExecStartPre=/bin/chown test:test /var/run/test",
		"ExecStart=/usr/bin/test-app --config /etc/test/config.yaml",
		"ExecStartPost=/bin/echo Started",
		"ExecStop=/bin/kill -SIGTERM $MAINPID",
		"ExecReload=/bin/kill -SIGHUP $MAINPID",
		"WorkingDirectory=/opt/test",
		"User=test",
		"Group=test",
		"Environment=ENV=production",
		"Environment=LOG_LEVEL=info",
		"EnvironmentFile=/etc/test/environment",
		"Restart=on-failure",
		"RestartSec=5",
		"TimeoutStartSec=60",
		"TimeoutStopSec=30",
		"LimitNOFILE=65536",
		"LimitNPROC=4096",
		"StandardOutput=journal",
		"StandardError=journal",

		"[Install]",
		"WantedBy=multi-user.target",
		"RequiredBy=custom.target",
		"Alias=test-app.service",
		"Also=test-helper.service",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("Generate() content missing expected string: %s\nGenerated content:\n%s", expected, content)
		}
	}

	// 验证段落顺序
	unitIdx := strings.Index(content, "[Unit]")
	serviceIdx := strings.Index(content, "[Service]")
	installIdx := strings.Index(content, "[Install]")

	if unitIdx == -1 || serviceIdx == -1 || installIdx == -1 {
		t.Errorf("Generate() missing required sections")
	}
	if unitIdx >= serviceIdx || serviceIdx >= installIdx {
		t.Errorf("Generate() sections in wrong order: Unit=%d, Service=%d, Install=%d", unitIdx, serviceIdx, installIdx)
	}

	t.Logf("Generated content:\n%s", content)
}

// TestGenerate_MinimalParams 测试最小参数生成（只有必填字段）
func TestGenerate_MinimalParams(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		ExecStart: "/usr/bin/test-app",
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 验证包含必需的三个段落
	if !strings.Contains(content, "[Unit]") {
		t.Error("Generate() missing [Unit] section")
	}
	if !strings.Contains(content, "[Service]") {
		t.Error("Generate() missing [Service] section")
	}
	if !strings.Contains(content, "[Install]") {
		t.Error("Generate() missing [Install] section")
	}

	// 验证必填字段
	if !strings.Contains(content, "ExecStart=/usr/bin/test-app") {
		t.Error("Generate() missing ExecStart")
	}

	// 验证默认值
	if !strings.Contains(content, "Type=simple") {
		t.Error("Generate() missing default Type=simple")
	}
	if !strings.Contains(content, "Restart=always") {
		t.Error("Generate() missing default Restart=always")
	}
	if !strings.Contains(content, "WantedBy=multi-user.target") {
		t.Error("Generate() missing default WantedBy=multi-user.target")
	}

	t.Logf("Generated minimal content:\n%s", content)
}

// TestGenerate_DefaultValues 测试默认值
func TestGenerate_DefaultValues(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		name     string
		params   *dto.ServiceUnitParams
		expected map[string]string
	}{
		{
			name: "默认 Type=simple",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
			},
			expected: map[string]string{
				"Type": "simple",
			},
		},
		{
			name: "默认 Restart=always",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
			},
			expected: map[string]string{
				"Restart": "always",
			},
		},
		{
			name: "默认 WantedBy=multi-user.target",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
			},
			expected: map[string]string{
				"WantedBy": "multi-user.target",
			},
		},
		{
			name: "自定义 Type 覆盖默认值",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
				Type:      "forking",
			},
			expected: map[string]string{
				"Type": "forking",
			},
		},
		{
			name: "自定义 Restart 覆盖默认值",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
				Restart:   "on-failure",
			},
			expected: map[string]string{
				"Restart": "on-failure",
			},
		},
		{
			name: "自定义 WantedBy 覆盖默认值",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
				WantedBy:  []string{"graphical.target"},
			},
			expected: map[string]string{
				"WantedBy": "graphical.target",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := svc.Generate(tt.params)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			for key, value := range tt.expected {
				expected := key + "=" + value
				if !strings.Contains(content, expected) {
					t.Errorf("Generate() missing expected %s, content:\n%s", expected, content)
				}
			}
		})
	}
}

// TestGenerate_ExtraLines 测试扩展行功能
func TestGenerate_ExtraLines(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		ExecStart: "/usr/bin/app",
		ExtraUnitLines: []string{
			"ConditionPathExists=/opt/app",
			"ConditionFileNotEmpty=/etc/app/config.yaml",
		},
		ExtraServiceLines: []string{
			"StandardOutput=journal",
			"StandardError=journal",
			"SyslogIdentifier=myapp",
		},
		ExtraInstallLines: []string{
			"Also=app-helper.service",
		},
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 验证扩展行出现在正确的段落中
	unitSectionStart := strings.Index(content, "[Unit]")
	serviceSectionStart := strings.Index(content, "[Service]")
	installSectionStart := strings.Index(content, "[Install]")

	// 检查 Unit 段的扩展行
	unitSection := content[unitSectionStart:serviceSectionStart]
	if !strings.Contains(unitSection, "ConditionPathExists=/opt/app") {
		t.Error("Unit section missing ExtraUnitLines")
	}
	if !strings.Contains(unitSection, "ConditionFileNotEmpty=/etc/app/config.yaml") {
		t.Error("Unit section missing ExtraUnitLines")
	}

	// 检查 Service 段的扩展行
	serviceSection := content[serviceSectionStart:installSectionStart]
	if !strings.Contains(serviceSection, "StandardOutput=journal") {
		t.Error("Service section missing ExtraServiceLines")
	}
	if !strings.Contains(serviceSection, "SyslogIdentifier=myapp") {
		t.Error("Service section missing ExtraServiceLines")
	}

	// 检查 Install 段的扩展行
	installSection := content[installSectionStart:]
	if !strings.Contains(installSection, "Also=app-helper.service") {
		t.Error("Install section missing ExtraInstallLines")
	}

	t.Logf("Generated content with extra lines:\n%s", content)
}

// TestValidate_NilParams 测试空参数校验
func TestValidate_NilParams(t *testing.T) {
	svc := setupTestService(t)

	content, err := svc.Generate(nil)
	if err == nil {
		t.Error("Generate() with nil params should return error")
	}
	if content != "" {
		t.Errorf("Generate() with nil params should return empty string, got: %s", content)
	}
	if !strings.Contains(err.Error(), "参数不能为空") {
		t.Errorf("Generate() error message should contain '参数不能为空', got: %v", err)
	}
}

// TestValidate_EmptyExecStart 测试 ExecStart 为空
func TestValidate_EmptyExecStart(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		name      string
		params    *dto.ServiceUnitParams
		wantError bool
	}{
		{
			name: "ExecStart 为空字符串",
			params: &dto.ServiceUnitParams{
				ExecStart: "",
			},
			wantError: true,
		},
		{
			name: "ExecStart 只有空格",
			params: &dto.ServiceUnitParams{
				ExecStart: "   ",
			},
			wantError: true,
		},
		{
			name: "ExecStart 正常",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Generate(tt.params)
			if (err != nil) != tt.wantError {
				t.Errorf("Generate() error = %v, wantError = %v", err, tt.wantError)
			}
			if err != nil && !strings.Contains(err.Error(), "ExecStart 不能为空") {
				t.Errorf("Generate() error message should contain 'ExecStart 不能为空', got: %v", err)
			}
		})
	}
}

// TestValidate_NoNewlines 测试换行符校验
func TestValidate_NoNewlines(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		name      string
		params    *dto.ServiceUnitParams
		wantError bool
		errorMsg  string
	}{
		{
			name: "Description 包含换行符",
			params: &dto.ServiceUnitParams{
				Description: "Test\nService",
				ExecStart:   "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "Description 不能包含换行符",
		},
		{
			name: "Documentation 包含换行符",
			params: &dto.ServiceUnitParams{
				Documentation: "https://example.com\nhttps://test.com",
				ExecStart:     "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "Documentation 不能包含换行符",
		},
		{
			name: "ExecStart 包含换行符",
			params: &dto.ServiceUnitParams{
				ExecStart: "/usr/bin/app\n--flag",
			},
			wantError: true,
			errorMsg:  "ExecStart 不能包含换行符",
		},
		{
			name: "Type 包含换行符",
			params: &dto.ServiceUnitParams{
				Type:      "simple\nforking",
				ExecStart: "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "Type 不能包含换行符",
		},
		{
			name: "User 包含换行符",
			params: &dto.ServiceUnitParams{
				User:      "test\nroot",
				ExecStart: "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "User 不能包含换行符",
		},
		{
			name: "WorkingDirectory 包含换行符",
			params: &dto.ServiceUnitParams{
				WorkingDirectory: "/opt/test\n/opt/app",
				ExecStart:        "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "WorkingDirectory 不能包含换行符",
		},
		{
			name: "Restart 包含换行符",
			params: &dto.ServiceUnitParams{
				Restart:   "always\non-failure",
				ExecStart: "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "Restart 不能包含换行符",
		},
		{
			name: "正常参数不包含换行符",
			params: &dto.ServiceUnitParams{
				Description: "Test Service",
				ExecStart:   "/usr/bin/app",
				User:        "test",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Generate(tt.params)
			if (err != nil) != tt.wantError {
				t.Errorf("Generate() error = %v, wantError = %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Generate() error message should contain '%s', got: %v", tt.errorMsg, err)
			}
		})
	}
}

// TestValidate_ArrayFieldsWithNewlines 测试数组字段包含换行符
func TestValidate_ArrayFieldsWithNewlines(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		name      string
		params    *dto.ServiceUnitParams
		wantError bool
		errorMsg  string
	}{
		{
			name: "After 包含换行符",
			params: &dto.ServiceUnitParams{
				After:     []string{"network.target\nsyslog.target"},
				ExecStart: "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "After[0] 不能包含换行符",
		},
		{
			name: "Environment 包含换行符",
			params: &dto.ServiceUnitParams{
				Environment: []string{"KEY=VALUE\nKEY2=VALUE2"},
				ExecStart:   "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "Environment[0] 不能包含换行符",
		},
		{
			name: "ExecStartPre 包含换行符",
			params: &dto.ServiceUnitParams{
				ExecStartPre: []string{"/bin/echo test\n/bin/echo test2"},
				ExecStart:    "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "ExecStartPre[0] 不能包含换行符",
		},
		{
			name: "WantedBy 包含换行符",
			params: &dto.ServiceUnitParams{
				WantedBy:  []string{"multi-user.target\ngraphical.target"},
				ExecStart: "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "WantedBy[0] 不能包含换行符",
		},
		{
			name: "ExtraUnitLines 包含换行符",
			params: &dto.ServiceUnitParams{
				ExtraUnitLines: []string{"Condition=true\nCondition2=false"},
				ExecStart:      "/usr/bin/app",
			},
			wantError: true,
			errorMsg:  "ExtraUnitLines[0] 不能包含换行符",
		},
		{
			name: "正常数组字段",
			params: &dto.ServiceUnitParams{
				After:       []string{"network.target", "syslog.target"},
				Environment: []string{"KEY=VALUE", "KEY2=VALUE2"},
				ExecStart:   "/usr/bin/app",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Generate(tt.params)
			if (err != nil) != tt.wantError {
				t.Errorf("Generate() error = %v, wantError = %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Generate() error message should contain '%s', got: %v", tt.errorMsg, err)
			}
		})
	}
}

// TestGenerate_MultipleValues 测试多值字段
func TestGenerate_MultipleValues(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		ExecStart: "/usr/bin/app",
		After:     []string{"network.target", "syslog.target", "remote-fs.target"},
		Environment: []string{
			"ENV=production",
			"LOG_LEVEL=debug",
			"DB_HOST=localhost",
		},
		ExecStartPre: []string{
			"/bin/mkdir -p /var/run/app",
			"/bin/chown app:app /var/run/app",
			"/bin/chmod 755 /var/run/app",
		},
		WantedBy: []string{"multi-user.target", "graphical.target"},
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 验证所有 After 值都存在
	for _, after := range params.After {
		expected := "After=" + after
		if !strings.Contains(content, expected) {
			t.Errorf("Generate() missing %s", expected)
		}
	}

	// 验证所有 Environment 值都存在
	for _, env := range params.Environment {
		expected := "Environment=" + env
		if !strings.Contains(content, expected) {
			t.Errorf("Generate() missing %s", expected)
		}
	}

	// 验证所有 ExecStartPre 值都存在
	for _, cmd := range params.ExecStartPre {
		expected := "ExecStartPre=" + cmd
		if !strings.Contains(content, expected) {
			t.Errorf("Generate() missing %s", expected)
		}
	}

	// 验证所有 WantedBy 值都存在
	for _, wantedBy := range params.WantedBy {
		expected := "WantedBy=" + wantedBy
		if !strings.Contains(content, expected) {
			t.Errorf("Generate() missing %s", expected)
		}
	}

	t.Logf("Generated content with multiple values:\n%s", content)
}

// TestGenerate_RealWorldExample 测试真实场景示例
func TestGenerate_RealWorldExample(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		name   string
		params *dto.ServiceUnitParams
	}{
		{
			name: "Go Web 应用",
			params: &dto.ServiceUnitParams{
				Description:      "Go Web Application",
				Documentation:    "https://example.com/docs",
				After:            []string{"network.target"},
				Type:             "simple",
				ExecStart:        "/opt/myapp/myapp -config /etc/myapp/config.yaml",
				WorkingDirectory: "/opt/myapp",
				User:             "myapp",
				Group:            "myapp",
				Environment: []string{
					"ENV=production",
					"PORT=8080",
				},
				Restart:         "on-failure",
				RestartSec:      5,
				TimeoutStartSec: 30,
				LimitNOFILE:     65536,
				WantedBy:        []string{"multi-user.target"},
				ExtraServiceLines: []string{
					"StandardOutput=journal",
					"StandardError=journal",
				},
			},
		},
		{
			name: "Nginx 服务",
			params: &dto.ServiceUnitParams{
				Description:   "The nginx HTTP and reverse proxy server",
				Documentation: "http://nginx.org/en/docs/",
				After:         []string{"network.target", "remote-fs.target", "nss-lookup.target"},
				Type:          "forking",
				ExecStartPre: []string{
					"/usr/sbin/nginx -t",
				},
				ExecStart:       "/usr/sbin/nginx",
				ExecReload:      "/bin/kill -s HUP $MAINPID",
				ExecStop:        "/bin/kill -s QUIT $MAINPID",
				User:            "nginx",
				Group:           "nginx",
				Restart:         "on-failure",
				RestartSec:      3,
				TimeoutStartSec: 90,
				TimeoutStopSec:  90,
				LimitNOFILE:     100000,
				WantedBy:        []string{"multi-user.target"},
				ExtraServiceLines: []string{
					"PrivateTmp=true",
					"PIDFile=/run/nginx.pid",
				},
			},
		},
		{
			name: "定时任务（oneshot）",
			params: &dto.ServiceUnitParams{
				Description:      "Daily backup task",
				Type:             "oneshot",
				ExecStart:        "/usr/local/bin/backup.sh",
				WorkingDirectory: "/var/backups",
				User:             "backup",
				Environment: []string{
					"BACKUP_DIR=/var/backups",
				},
				ExtraServiceLines: []string{
					"RemainAfterExit=no",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := svc.Generate(tt.params)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			// 基本验证
			if !strings.Contains(content, "[Unit]") {
				t.Error("Missing [Unit] section")
			}
			if !strings.Contains(content, "[Service]") {
				t.Error("Missing [Service] section")
			}
			if !strings.Contains(content, "[Install]") {
				t.Error("Missing [Install] section")
			}
			if !strings.Contains(content, tt.params.ExecStart) {
				t.Error("Missing ExecStart")
			}

			t.Logf("Generated %s:\n%s", tt.name, content)
		})
	}
}

// TestGenerate_SectionSeparation 测试段落之间的分隔
func TestGenerate_SectionSeparation(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		Description: "Test Service",
		ExecStart:   "/usr/bin/app",
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 验证段落之间有空行
	lines := strings.Split(content, "\n")

	var unitEndIdx, serviceEndIdx int
	for i, line := range lines {
		if line == "[Service]" && unitEndIdx == 0 {
			if i > 0 && lines[i-1] != "" {
				t.Error("Should have blank line before [Service]")
			}
			unitEndIdx = i
		}
		if line == "[Install]" && serviceEndIdx == 0 {
			if i > 0 && lines[i-1] != "" {
				t.Error("Should have blank line before [Install]")
			}
			serviceEndIdx = i
		}
	}

	if unitEndIdx == 0 || serviceEndIdx == 0 {
		t.Error("Missing sections")
	}
}

// TestGenerate_IntegerFields 测试整型字段
func TestGenerate_IntegerFields(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		ExecStart:       "/usr/bin/app",
		RestartSec:      10,
		TimeoutStartSec: 60,
		TimeoutStopSec:  30,
		LimitNOFILE:     65536,
		LimitNPROC:      4096,
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	expectedStrings := []string{
		"RestartSec=10",
		"TimeoutStartSec=60",
		"TimeoutStopSec=30",
		"LimitNOFILE=65536",
		"LimitNPROC=4096",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("Generate() missing expected string: %s", expected)
		}
	}
}

// TestGenerate_ZeroIntegerFields 测试整型字段为 0 时不输出
func TestGenerate_ZeroIntegerFields(t *testing.T) {
	svc := setupTestService(t)

	params := &dto.ServiceUnitParams{
		ExecStart:       "/usr/bin/app",
		RestartSec:      0,
		TimeoutStartSec: 0,
		TimeoutStopSec:  0,
		LimitNOFILE:     0,
		LimitNPROC:      0,
	}

	content, err := svc.Generate(params)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 验证值为 0 的字段不输出（因为 if > 0 条件）
	notExpectedStrings := []string{
		"RestartSec=0",
		"TimeoutStartSec=0",
		"TimeoutStopSec=0",
		"LimitNOFILE=0",
		"LimitNPROC=0",
	}

	for _, notExpected := range notExpectedStrings {
		if strings.Contains(content, notExpected) {
			t.Errorf("Generate() should not contain: %s", notExpected)
		}
	}
}



