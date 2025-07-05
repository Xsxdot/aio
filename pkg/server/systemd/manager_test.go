package systemd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xsxdot/aio/pkg/server"
)

func TestGenerateServiceFileContent(t *testing.T) {
	tests := []struct {
		name     string
		req      *server.ServiceCreateRequest
		expected string
	}{
		{
			name: "最小配置 - 只有必填字段",
			req: &server.ServiceCreateRequest{
				Name:      "test-service",
				ExecStart: "/usr/bin/test-app",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/test-app
Restart=on-failure

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "带描述的基本配置",
			req: &server.ServiceCreateRequest{
				Name:        "web-service",
				Description: "Web应用服务",
				ExecStart:   "/opt/app/bin/web-server",
			},
			expected: `[Unit]
Description=Web应用服务
After=network.target

[Service]
Type=simple
ExecStart=/opt/app/bin/web-server
Restart=on-failure

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "完整配置除环境变量 - 大部分字段",
			req: &server.ServiceCreateRequest{
				Name:        "full-service",
				Description: "完整的服务配置示例",
				Type:        server.ServiceTypeForking,
				ExecStart:   "/usr/local/bin/app start",
				ExecReload:  "/usr/local/bin/app reload",
				ExecStop:    "/usr/local/bin/app stop",
				WorkingDir:  "/var/lib/app",
				User:        "appuser",
				Group:       "appgroup",
				PIDFile:     "/var/run/app.pid",
				Restart:     "always",
			},
			expected: `[Unit]
Description=完整的服务配置示例
After=network.target

[Service]
Type=forking
ExecStart=/usr/local/bin/app start
ExecReload=/usr/local/bin/app reload
ExecStop=/usr/local/bin/app stop
WorkingDirectory=/var/lib/app
User=appuser
Group=appgroup
PIDFile=/var/run/app.pid
Restart=always

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "使用notify类型",
			req: &server.ServiceCreateRequest{
				Name:      "notify-service",
				Type:      server.ServiceTypeNotify,
				ExecStart: "/usr/bin/notify-daemon",
				User:      "daemon",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=notify
ExecStart=/usr/bin/notify-daemon
User=daemon
Restart=on-failure

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "oneshot类型服务",
			req: &server.ServiceCreateRequest{
				Name:      "setup-service",
				Type:      server.ServiceTypeOneshot,
				ExecStart: "/usr/local/bin/setup.sh",
				Restart:   "no",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/setup.sh
Restart=no

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "带工作目录和用户组",
			req: &server.ServiceCreateRequest{
				Name:       "worker-service",
				ExecStart:  "/opt/worker/bin/worker",
				WorkingDir: "/opt/worker",
				User:       "worker",
				Group:      "workers",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=simple
ExecStart=/opt/worker/bin/worker
WorkingDirectory=/opt/worker
User=worker
Group=workers
Restart=on-failure

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "单个环境变量",
			req: &server.ServiceCreateRequest{
				Name:      "env-service",
				ExecStart: "/usr/bin/env-app",
				Environment: map[string]string{
					"CONFIG_PATH": "/etc/app/config.json",
				},
			},
			expected: `[Unit]
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/env-app
Restart=on-failure
Environment=CONFIG_PATH=/etc/app/config.json

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "带PID文件",
			req: &server.ServiceCreateRequest{
				Name:      "pid-service",
				Type:      server.ServiceTypeForking,
				ExecStart: "/usr/sbin/daemon",
				PIDFile:   "/var/run/daemon.pid",
				User:      "root",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=forking
ExecStart=/usr/sbin/daemon
User=root
PIDFile=/var/run/daemon.pid
Restart=on-failure

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "自定义重启策略",
			req: &server.ServiceCreateRequest{
				Name:      "critical-service",
				ExecStart: "/usr/bin/critical-app",
				Restart:   "on-abnormal",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/critical-app
Restart=on-abnormal

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "带reload和stop命令",
			req: &server.ServiceCreateRequest{
				Name:       "managed-service",
				ExecStart:  "/usr/bin/managed-app",
				ExecReload: "/bin/kill -HUP $MAINPID",
				ExecStop:   "/usr/bin/managed-app stop",
			},
			expected: `[Unit]
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/managed-app
ExecReload=/bin/kill -HUP $MAINPID
ExecStop=/usr/bin/managed-app stop
Restart=on-failure

[Install]
WantedBy=multi-user.target
`,
		},
		{
			name: "多个环境变量测试",
			req: &server.ServiceCreateRequest{
				Name:      "multi-env-service",
				ExecStart: "/usr/bin/app",
				Environment: map[string]string{
					"APP_ENV":     "production",
					"CONFIG_FILE": "/etc/app/config.yaml",
				},
			},
			// 不检查具体的环境变量顺序，在专门的测试函数中处理
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateServiceFileContent(tt.req)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, result, "生成的service文件内容不匹配")
			} else {
				// 对于环境变量等无法预测顺序的测试用例，只检查基本结构
				assert.Contains(t, result, "[Unit]", "应该包含Unit段")
				assert.Contains(t, result, "[Service]", "应该包含Service段")
				assert.Contains(t, result, "[Install]", "应该包含Install段")
				assert.Contains(t, result, "ExecStart=", "应该包含ExecStart")
			}
		})
	}
}

func TestGenerateServiceFileContent_EnvironmentVariables(t *testing.T) {
	// 测试环境变量的顺序（由于map的无序性，我们需要单独测试）
	req := &server.ServiceCreateRequest{
		Name:      "env-test",
		ExecStart: "/usr/bin/app",
		Environment: map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
		},
	}

	result := generateServiceFileContent(req)

	// 检查是否包含所有环境变量
	assert.Contains(t, result, "Environment=", "应该包含Environment配置")
	assert.Contains(t, result, "VAR1=value1", "应该包含VAR1环境变量")
	assert.Contains(t, result, "VAR2=value2", "应该包含VAR2环境变量")

	// 检查基本结构
	assert.Contains(t, result, "[Unit]", "应该包含Unit段")
	assert.Contains(t, result, "[Service]", "应该包含Service段")
	assert.Contains(t, result, "[Install]", "应该包含Install段")
}

func TestGenerateServiceFileContent_DefaultValues(t *testing.T) {
	// 测试默认值的应用
	req := &server.ServiceCreateRequest{
		Name:      "default-test",
		ExecStart: "/usr/bin/app",
		// Type 和 Restart 为空，应该使用默认值
	}

	result := generateServiceFileContent(req)

	assert.Contains(t, result, "Type=simple", "应该使用默认的服务类型simple")
	assert.Contains(t, result, "Restart=on-failure", "应该使用默认的重启策略on-failure")
}

func TestGenerateServiceFileContent_EmptyFields(t *testing.T) {
	// 测试空字段不会被添加到配置中
	req := &server.ServiceCreateRequest{
		Name:        "empty-test",
		Description: "", // 空描述
		ExecStart:   "/usr/bin/app",
		ExecReload:  "",  // 空reload命令
		ExecStop:    "",  // 空stop命令
		WorkingDir:  "",  // 空工作目录
		User:        "",  // 空用户
		Group:       "",  // 空组
		PIDFile:     "",  // 空PID文件
		Environment: nil, // 空环境变量
	}

	result := generateServiceFileContent(req)

	// 检查空字段不会出现在结果中
	assert.NotContains(t, result, "Description=", "空描述不应该出现")
	assert.NotContains(t, result, "ExecReload=", "空reload命令不应该出现")
	assert.NotContains(t, result, "ExecStop=", "空stop命令不应该出现")
	assert.NotContains(t, result, "WorkingDirectory=", "空工作目录不应该出现")
	assert.NotContains(t, result, "User=", "空用户不应该出现")
	assert.NotContains(t, result, "Group=", "空组不应该出现")
	assert.NotContains(t, result, "PIDFile=", "空PID文件不应该出现")
	assert.NotContains(t, result, "Environment=", "空环境变量不应该出现")

	// 检查必要字段仍然存在
	assert.Contains(t, result, "ExecStart=/usr/bin/app", "ExecStart应该存在")
	assert.Contains(t, result, "Type=simple", "Type应该存在（默认值）")
	assert.Contains(t, result, "Restart=on-failure", "Restart应该存在（默认值）")
}

func TestGenerateServiceFileContent_SpecialCharacters(t *testing.T) {
	// 测试特殊字符处理
	req := &server.ServiceCreateRequest{
		Name:        "special-service",
		Description: "服务 with 特殊字符 & symbols",
		ExecStart:   "/usr/bin/app --config=/path/with/spaces and symbols",
		Environment: map[string]string{
			"PATH_WITH_SPACES": "/path with spaces",
			"SPECIAL_CHARS":    "value=with&special!chars",
		},
	}

	result := generateServiceFileContent(req)

	// 检查特殊字符被正确处理
	assert.Contains(t, result, "Description=服务 with 特殊字符 & symbols", "应该正确处理描述中的特殊字符")
	assert.Contains(t, result, "ExecStart=/usr/bin/app --config=/path/with/spaces and symbols", "应该正确处理命令中的特殊字符")
	assert.Contains(t, result, "PATH_WITH_SPACES=/path with spaces", "应该正确处理环境变量中的空格")
	assert.Contains(t, result, "SPECIAL_CHARS=value=with&special!chars", "应该正确处理环境变量中的特殊字符")
}
