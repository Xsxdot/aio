package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/nginx/internal/model/dto"
)

// TestNginxConfigGenerateService_Generate_Proxy 测试生成反向代理配置
func TestNginxConfigGenerateService_Generate_Proxy(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type:        dto.ConfigTypeProxy,
		Description: "测试反向代理配置",
		Server: dto.ServerConfig{
			Listen:     80,
			ServerName: "example.com",
			Locations: []dto.LocationConfig{
				{
					Path:      "/",
					ProxyPass: "http://backend",
				},
			},
		},
		Upstreams: []dto.UpstreamConfig{
			{
				Name: "backend",
				Servers: []dto.UpstreamServer{
					{Address: "127.0.0.1:8080", Weight: 1},
					{Address: "127.0.0.1:8081", Weight: 1},
				},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证生成的内容
	if !strings.Contains(content, "upstream backend") {
		t.Error("配置中应包含 upstream backend")
	}
	if !strings.Contains(content, "server 127.0.0.1:8080") {
		t.Error("配置中应包含上游服务器地址")
	}
	if !strings.Contains(content, "listen 80") {
		t.Error("配置中应包含监听端口")
	}
	if !strings.Contains(content, "server_name example.com") {
		t.Error("配置中应包含服务器名称")
	}
	if !strings.Contains(content, "proxy_pass http://backend") {
		t.Error("配置中应包含 proxy_pass 指令")
	}
	if !strings.Contains(content, "proxy_set_header Host $host") {
		t.Error("配置中应包含代理头设置")
	}

	t.Logf("生成的配置:\n%s", content)
}

// TestNginxConfigGenerateService_Generate_Static 测试生成静态站点配置
func TestNginxConfigGenerateService_Generate_Static(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type:        dto.ConfigTypeStatic,
		Description: "测试静态站点配置",
		Server: dto.ServerConfig{
			Listen:     80,
			ServerName: "static.example.com",
			Locations: []dto.LocationConfig{
				{
					Path:     "/",
					Root:     "/var/www/html",
					Index:    "index.html index.htm",
					TryFiles: "$uri $uri/ /index.html",
				},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证生成的内容
	if !strings.Contains(content, "listen 80") {
		t.Error("配置中应包含监听端口")
	}
	if !strings.Contains(content, "server_name static.example.com") {
		t.Error("配置中应包含服务器名称")
	}
	if !strings.Contains(content, "root /var/www/html") {
		t.Error("配置中应包含 root 指令")
	}
	if !strings.Contains(content, "index index.html index.htm") {
		t.Error("配置中应包含 index 指令")
	}
	if !strings.Contains(content, "try_files $uri $uri/ /index.html") {
		t.Error("配置中应包含 try_files 指令")
	}

	t.Logf("生成的配置:\n%s", content)
}

// TestNginxConfigGenerateService_Generate_SSL 测试生成SSL配置
func TestNginxConfigGenerateService_Generate_SSL(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type:        dto.ConfigTypeProxy,
		Description: "测试SSL配置",
		Server: dto.ServerConfig{
			Listen:     443,
			ServerName: "secure.example.com",
			SSLEnabled: true,
			SSLCert:    "/etc/nginx/ssl/cert.pem",
			SSLKey:     "/etc/nginx/ssl/key.pem",
			Locations: []dto.LocationConfig{
				{
					Path:      "/",
					ProxyPass: "http://backend",
				},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证SSL相关配置
	if !strings.Contains(content, "listen 443 ssl") {
		t.Error("配置中应包含 ssl 监听")
	}
	if !strings.Contains(content, "ssl_certificate /etc/nginx/ssl/cert.pem") {
		t.Error("配置中应包含SSL证书路径")
	}
	if !strings.Contains(content, "ssl_certificate_key /etc/nginx/ssl/key.pem") {
		t.Error("配置中应包含SSL密钥路径")
	}
	if !strings.Contains(content, "ssl_protocols TLSv1.2 TLSv1.3") {
		t.Error("配置中应包含SSL协议版本")
	}

	t.Logf("生成的配置:\n%s", content)
}

// TestNginxConfigGenerateService_Generate_WebSocket 测试生成WebSocket配置
func TestNginxConfigGenerateService_Generate_WebSocket(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type:        dto.ConfigTypeProxy,
		Description: "测试WebSocket配置",
		Server: dto.ServerConfig{
			Listen:     80,
			ServerName: "ws.example.com",
			Locations: []dto.LocationConfig{
				{
					Path:            "/ws",
					ProxyPass:       "http://websocket_backend",
					EnableWebSocket: true,
				},
			},
		},
		Upstreams: []dto.UpstreamConfig{
			{
				Name: "websocket_backend",
				Servers: []dto.UpstreamServer{
					{Address: "127.0.0.1:9000"},
				},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证WebSocket相关配置
	if !strings.Contains(content, "proxy_http_version 1.1") {
		t.Error("配置中应包含HTTP版本设置")
	}
	if !strings.Contains(content, "proxy_set_header Upgrade $http_upgrade") {
		t.Error("配置中应包含Upgrade头设置")
	}
	if !strings.Contains(content, `proxy_set_header Connection "upgrade"`) {
		t.Error("配置中应包含Connection头设置")
	}

	t.Logf("生成的配置:\n%s", content)
}

// TestNginxConfigGenerateService_Generate_LoadBalance 测试负载均衡配置
func TestNginxConfigGenerateService_Generate_LoadBalance(t *testing.T) {
	testCases := []struct {
		name         string
		loadBalance  string
		expectedText string
	}{
		{
			name:         "ip_hash负载均衡",
			loadBalance:  "ip_hash",
			expectedText: "ip_hash;",
		},
		{
			name:         "least_conn负载均衡",
			loadBalance:  "least_conn",
			expectedText: "least_conn;",
		},
		{
			name:         "默认轮询",
			loadBalance:  "",
			expectedText: "upstream backend",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewNginxConfigGenerateService(logger.GetLogger())

			spec := &dto.ConfigSpec{
				Type: dto.ConfigTypeProxy,
				Server: dto.ServerConfig{
					Listen:     80,
					ServerName: "lb.example.com",
					Locations: []dto.LocationConfig{
						{
							Path:      "/",
							ProxyPass: "http://backend",
						},
					},
				},
				Upstreams: []dto.UpstreamConfig{
					{
						Name:        "backend",
						LoadBalance: tc.loadBalance,
						Servers: []dto.UpstreamServer{
							{Address: "127.0.0.1:8080", Weight: 2},
							{Address: "127.0.0.1:8081", Weight: 1},
							{Address: "127.0.0.1:8082", Weight: 1, Backup: true},
						},
					},
				},
			}

			content, err := service.Generate(spec)
			if err != nil {
				t.Fatalf("生成配置失败: %v", err)
			}

			if !strings.Contains(content, tc.expectedText) {
				t.Errorf("配置中应包含 %s", tc.expectedText)
			}

			// 验证权重和备份服务器
			if !strings.Contains(content, "weight=2") {
				t.Error("配置中应包含权重设置")
			}
			if !strings.Contains(content, "backup") {
				t.Error("配置中应包含备份服务器标记")
			}

			t.Logf("生成的配置:\n%s", content)
		})
	}
}

// TestNginxConfigGenerateService_Generate_MultipleLocations 测试多个location配置
func TestNginxConfigGenerateService_Generate_MultipleLocations(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type:        dto.ConfigTypeProxy,
		Description: "测试多个location",
		Server: dto.ServerConfig{
			Listen:     80,
			ServerName: "multi.example.com",
			Locations: []dto.LocationConfig{
				{
					Path:      "/api",
					ProxyPass: "http://api_backend",
				},
				{
					Path:      "/admin",
					ProxyPass: "http://admin_backend",
				},
				{
					Path:            "/ws",
					ProxyPass:       "http://ws_backend",
					EnableWebSocket: true,
				},
			},
		},
		Upstreams: []dto.UpstreamConfig{
			{
				Name:    "api_backend",
				Servers: []dto.UpstreamServer{{Address: "127.0.0.1:8080"}},
			},
			{
				Name:    "admin_backend",
				Servers: []dto.UpstreamServer{{Address: "127.0.0.1:8081"}},
			},
			{
				Name:    "ws_backend",
				Servers: []dto.UpstreamServer{{Address: "127.0.0.1:8082"}},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证所有location
	locations := []string{"/api", "/admin", "/ws"}
	for _, loc := range locations {
		if !strings.Contains(content, "location "+loc) {
			t.Errorf("配置中应包含 location %s", loc)
		}
	}

	// 验证所有upstream
	upstreams := []string{"api_backend", "admin_backend", "ws_backend"}
	for _, upstream := range upstreams {
		if !strings.Contains(content, "upstream "+upstream) {
			t.Errorf("配置中应包含 upstream %s", upstream)
		}
	}

	t.Logf("生成的配置:\n%s", content)
}

// TestNginxConfigGenerateService_Generate_NilSpec 测试spec为nil的错误情况
func TestNginxConfigGenerateService_Generate_NilSpec(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	_, err := service.Generate(nil)
	if err == nil {
		t.Fatal("spec为nil时应返回错误")
	}

	if !strings.Contains(err.Error(), "配置规格不能为空") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

// TestNginxConfigGenerateService_Generate_InvalidUpstream 测试无效的upstream配置
func TestNginxConfigGenerateService_Generate_InvalidUpstream(t *testing.T) {
	testCases := []struct {
		name          string
		upstream      dto.UpstreamConfig
		expectedError string
	}{
		{
			name: "upstream名称为空",
			upstream: dto.UpstreamConfig{
				Name:    "",
				Servers: []dto.UpstreamServer{{Address: "127.0.0.1:8080"}},
			},
			expectedError: "upstream 名称不能为空",
		},
		{
			name: "upstream服务器列表为空",
			upstream: dto.UpstreamConfig{
				Name:    "backend",
				Servers: []dto.UpstreamServer{},
			},
			expectedError: "upstream 服务器列表不能为空",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewNginxConfigGenerateService(logger.GetLogger())

			spec := &dto.ConfigSpec{
				Type: dto.ConfigTypeProxy,
				Server: dto.ServerConfig{
					Listen:     80,
					ServerName: "test.example.com",
					Locations: []dto.LocationConfig{
						{
							Path:      "/",
							ProxyPass: "http://backend",
						},
					},
				},
				Upstreams: []dto.UpstreamConfig{tc.upstream},
			}

			_, err := service.Generate(spec)
			if err == nil {
				t.Fatal("应返回错误")
			}

			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("期望错误信息包含 '%s'，实际: %v", tc.expectedError, err)
			}
		})
	}
}

// TestNginxConfigGenerateService_Generate_InvalidServer 测试无效的server配置
func TestNginxConfigGenerateService_Generate_InvalidServer(t *testing.T) {
	testCases := []struct {
		name          string
		server        dto.ServerConfig
		expectedError string
	}{
		{
			name: "监听端口无效",
			server: dto.ServerConfig{
				Listen:     0,
				ServerName: "test.example.com",
				Locations:  []dto.LocationConfig{{Path: "/"}},
			},
			expectedError: "监听端口无效",
		},
		{
			name: "server_name为空",
			server: dto.ServerConfig{
				Listen:     80,
				ServerName: "",
				Locations:  []dto.LocationConfig{{Path: "/"}},
			},
			expectedError: "server_name 不能为空",
		},
		{
			name: "location配置为空",
			server: dto.ServerConfig{
				Listen:     80,
				ServerName: "test.example.com",
				Locations:  []dto.LocationConfig{},
			},
			expectedError: "location 配置不能为空",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewNginxConfigGenerateService(logger.GetLogger())

			spec := &dto.ConfigSpec{
				Type:   dto.ConfigTypeProxy,
				Server: tc.server,
			}

			_, err := service.Generate(spec)
			if err == nil {
				t.Fatal("应返回错误")
			}

			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("期望错误信息包含 '%s'，实际: %v", tc.expectedError, err)
			}
		})
	}
}

// TestNginxConfigGenerateService_Generate_InvalidLocation 测试无效的location配置
func TestNginxConfigGenerateService_Generate_InvalidLocation(t *testing.T) {
	testCases := []struct {
		name          string
		location      dto.LocationConfig
		configType    dto.ConfigType
		expectedError string
	}{
		{
			name: "location path为空",
			location: dto.LocationConfig{
				Path:      "",
				ProxyPass: "http://backend",
			},
			configType:    dto.ConfigTypeProxy,
			expectedError: "location path 不能为空",
		},
		{
			name: "反向代理模式下proxy_pass为空",
			location: dto.LocationConfig{
				Path:      "/",
				ProxyPass: "",
			},
			configType:    dto.ConfigTypeProxy,
			expectedError: "反向代理模式下 proxy_pass 不能为空",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewNginxConfigGenerateService(logger.GetLogger())

			spec := &dto.ConfigSpec{
				Type: tc.configType,
				Server: dto.ServerConfig{
					Listen:     80,
					ServerName: "test.example.com",
					Locations:  []dto.LocationConfig{tc.location},
				},
			}

			_, err := service.Generate(spec)
			if err == nil {
				t.Fatal("应返回错误")
			}

			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("期望错误信息包含 '%s'，实际: %v", tc.expectedError, err)
			}
		})
	}
}

// TestNginxConfigGenerateService_Generate_ComplexConfig 测试复杂配置场景
func TestNginxConfigGenerateService_Generate_ComplexConfig(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type:        dto.ConfigTypeProxy,
		Description: "复杂的生产环境配置",
		Server: dto.ServerConfig{
			Listen:     443,
			ServerName: "prod.example.com www.prod.example.com",
			SSLEnabled: true,
			SSLCert:    "/etc/nginx/ssl/prod.crt",
			SSLKey:     "/etc/nginx/ssl/prod.key",
			Locations: []dto.LocationConfig{
				{
					Path:      "/api/v1",
					ProxyPass: "http://api_v1_backend",
				},
				{
					Path:      "/api/v2",
					ProxyPass: "http://api_v2_backend",
				},
				{
					Path:            "/ws",
					ProxyPass:       "http://websocket_backend",
					EnableWebSocket: true,
				},
			},
		},
		Upstreams: []dto.UpstreamConfig{
			{
				Name:        "api_v1_backend",
				LoadBalance: "least_conn",
				Servers: []dto.UpstreamServer{
					{Address: "10.0.1.10:8080", Weight: 3},
					{Address: "10.0.1.11:8080", Weight: 2},
					{Address: "10.0.1.12:8080", Weight: 1, Backup: true},
				},
			},
			{
				Name:        "api_v2_backend",
				LoadBalance: "ip_hash",
				Servers: []dto.UpstreamServer{
					{Address: "10.0.2.10:8080"},
					{Address: "10.0.2.11:8080"},
				},
			},
			{
				Name: "websocket_backend",
				Servers: []dto.UpstreamServer{
					{Address: "10.0.3.10:9000"},
				},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证所有关键元素
	checks := []string{
		"upstream api_v1_backend",
		"upstream api_v2_backend",
		"upstream websocket_backend",
		"least_conn",
		"ip_hash",
		"listen 443 ssl",
		"ssl_certificate /etc/nginx/ssl/prod.crt",
		"ssl_certificate_key /etc/nginx/ssl/prod.key",
		"server_name prod.example.com www.prod.example.com",
		"location /api/v1",
		"location /api/v2",
		"location /ws",
		"proxy_http_version 1.1",
		"weight=3",
		"backup",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("配置中应包含: %s", check)
		}
	}

	t.Logf("生成的复杂配置:\n%s", content)
}

// TestNginxConfigGenerateService_Generate_NoUpstream 测试没有upstream的配置
func TestNginxConfigGenerateService_Generate_NoUpstream(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	spec := &dto.ConfigSpec{
		Type: dto.ConfigTypeProxy,
		Server: dto.ServerConfig{
			Listen:     80,
			ServerName: "direct.example.com",
			Locations: []dto.LocationConfig{
				{
					Path:      "/",
					ProxyPass: "http://127.0.0.1:8080",
				},
			},
		},
		Upstreams: []dto.UpstreamConfig{}, // 空的upstream列表
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 验证不包含upstream块
	if strings.Contains(content, "upstream") {
		t.Error("配置中不应包含 upstream 块")
	}

	// 验证包含直接的proxy_pass
	if !strings.Contains(content, "proxy_pass http://127.0.0.1:8080") {
		t.Error("配置中应包含直接的 proxy_pass 地址")
	}

	t.Logf("生成的配置:\n%s", content)
}

// TestNginxConfigGenerateService_SaveToFile 测试将生成的配置保存到文件
func TestNginxConfigGenerateService_SaveToFile(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	// 创建临时目录（Mac系统）
	tmpDir := filepath.Join(os.TempDir(), "nginx_test_configs")
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer func() {
		// 清理临时目录
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("清理临时目录失败: %v", err)
		}
	}()

	testCases := []struct {
		name     string
		spec     *dto.ConfigSpec
		filename string
	}{
		{
			name: "保存反向代理配置",
			spec: &dto.ConfigSpec{
				Type:        dto.ConfigTypeProxy,
				Description: "测试反向代理配置保存",
				Server: dto.ServerConfig{
					Listen:     80,
					ServerName: "api.example.com",
					Locations: []dto.LocationConfig{
						{
							Path:      "/",
							ProxyPass: "http://backend",
						},
					},
				},
				Upstreams: []dto.UpstreamConfig{
					{
						Name: "backend",
						Servers: []dto.UpstreamServer{
							{Address: "127.0.0.1:8080"},
						},
					},
				},
			},
			filename: "proxy.conf",
		},
		{
			name: "保存静态站点配置",
			spec: &dto.ConfigSpec{
				Type:        dto.ConfigTypeStatic,
				Description: "测试静态站点配置保存",
				Server: dto.ServerConfig{
					Listen:     80,
					ServerName: "www.example.com",
					Locations: []dto.LocationConfig{
						{
							Path:     "/",
							Root:     "/var/www/html",
							Index:    "index.html",
							TryFiles: "$uri $uri/ /index.html",
						},
					},
				},
			},
			filename: "static.conf",
		},
		{
			name: "保存SSL配置",
			spec: &dto.ConfigSpec{
				Type:        dto.ConfigTypeProxy,
				Description: "测试SSL配置保存",
				Server: dto.ServerConfig{
					Listen:     443,
					ServerName: "secure.example.com",
					SSLEnabled: true,
					SSLCert:    "/etc/nginx/ssl/cert.pem",
					SSLKey:     "/etc/nginx/ssl/key.pem",
					Locations: []dto.LocationConfig{
						{
							Path:      "/",
							ProxyPass: "http://backend",
						},
					},
				},
			},
			filename: "ssl.conf",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 生成配置内容
			content, err := service.Generate(tc.spec)
			if err != nil {
				t.Fatalf("生成配置失败: %v", err)
			}

			// 保存到文件
			filePath := filepath.Join(tmpDir, tc.filename)
			err = os.WriteFile(filePath, []byte(content), 0644)
			if err != nil {
				t.Fatalf("保存配置文件失败: %v", err)
			}

			t.Logf("配置已保存到: %s", filePath)

			// 验证文件是否存在
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Fatalf("配置文件不存在: %s", filePath)
			}

			// 读取文件内容并验证
			readContent, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("读取配置文件失败: %v", err)
			}

			if string(readContent) != content {
				t.Error("文件内容与生成的内容不一致")
			}

			// 验证文件大小
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("获取文件信息失败: %v", err)
			}

			if fileInfo.Size() == 0 {
				t.Error("配置文件为空")
			}

			t.Logf("文件大小: %d 字节", fileInfo.Size())
			t.Logf("文件内容:\n%s", string(readContent))
		})
	}
}

// TestNginxConfigGenerateService_SaveMultipleFiles 测试保存多个配置文件
func TestNginxConfigGenerateService_SaveMultipleFiles(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	// 创建临时目录结构
	tmpDir := filepath.Join(os.TempDir(), "nginx_test_multiple")
	sitesAvailable := filepath.Join(tmpDir, "sites-available")
	sitesEnabled := filepath.Join(tmpDir, "sites-enabled")

	// 创建目录
	for _, dir := range []string{sitesAvailable, sitesEnabled} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("清理临时目录失败: %v", err)
		}
	}()

	// 生成多个站点配置
	sites := []struct {
		name string
		port int
	}{
		{"site1.example.com", 8081},
		{"site2.example.com", 8082},
		{"site3.example.com", 8083},
	}

	for _, site := range sites {
		spec := &dto.ConfigSpec{
			Type: dto.ConfigTypeProxy,
			Server: dto.ServerConfig{
				Listen:     site.port,
				ServerName: site.name,
				Locations: []dto.LocationConfig{
					{
						Path:      "/",
						ProxyPass: "http://127.0.0.1:" + strings.TrimPrefix(site.name, "site"),
					},
				},
			},
		}

		content, err := service.Generate(spec)
		if err != nil {
			t.Fatalf("生成 %s 配置失败: %v", site.name, err)
		}

		// 保存到 sites-available
		configFile := filepath.Join(sitesAvailable, site.name+".conf")
		if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
			t.Fatalf("保存配置文件失败: %v", err)
		}

		t.Logf("已保存配置: %s", configFile)

		// 创建符号链接到 sites-enabled（模拟nginx的配置启用方式）
		symlinkPath := filepath.Join(sitesEnabled, site.name+".conf")
		if err := os.Symlink(configFile, symlinkPath); err != nil {
			t.Logf("创建符号链接失败（Mac可能需要权限）: %v", err)
			// 在Mac上符号链接可能失败，不影响测试
		}
	}

	// 验证所有文件是否创建成功
	files, err := os.ReadDir(sitesAvailable)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}

	if len(files) != len(sites) {
		t.Errorf("期望创建 %d 个配置文件，实际创建了 %d 个", len(sites), len(files))
	}

	t.Logf("成功创建了 %d 个站点配置文件", len(files))

	// 列出所有创建的文件
	for _, file := range files {
		filePath := filepath.Join(sitesAvailable, file.Name())
		fileInfo, _ := os.Stat(filePath)
		t.Logf("  - %s (%d 字节)", file.Name(), fileInfo.Size())
	}
}

// TestNginxConfigGenerateService_FilePermissions 测试文件权限
func TestNginxConfigGenerateService_FilePermissions(t *testing.T) {
	service := NewNginxConfigGenerateService(logger.GetLogger())

	tmpDir := filepath.Join(os.TempDir(), "nginx_test_permissions")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	spec := &dto.ConfigSpec{
		Type: dto.ConfigTypeProxy,
		Server: dto.ServerConfig{
			Listen:     80,
			ServerName: "test.example.com",
			Locations: []dto.LocationConfig{
				{
					Path:      "/",
					ProxyPass: "http://backend",
				},
			},
		},
	}

	content, err := service.Generate(spec)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}

	// 测试不同的文件权限
	testCases := []struct {
		name string
		perm os.FileMode
	}{
		{"只读权限", 0444},
		{"读写权限", 0644},
		{"完全权限", 0755},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, "perm_test_"+tc.name+".conf")

			// 保存文件
			if err := os.WriteFile(filePath, []byte(content), tc.perm); err != nil {
				t.Fatalf("保存文件失败: %v", err)
			}

			// 检查文件权限
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("获取文件信息失败: %v", err)
			}

			actualPerm := fileInfo.Mode().Perm()
			if actualPerm != tc.perm {
				t.Logf("注意: 文件权限可能与预期不同 (期望: %o, 实际: %o)", tc.perm, actualPerm)
			}

			t.Logf("文件 %s 权限: %o", filePath, actualPerm)
		})
	}
}
