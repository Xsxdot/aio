package nginx

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xsxdot/aio/pkg/server"
)

func TestService_generateSiteConfig(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name        string
		site        *server.NginxSite
		contains    []string
		notContains []string
	}{
		{
			name: "简单静态站点配置",
			site: &server.NginxSite{
				Name:       "static-site",
				Type:       server.NginxSiteTypeStatic,
				ServerName: "example.com",
				Listen:     []string{"80"},
				Root:       "/var/www/html",
				Index:      []string{"index.html", "index.htm"},
				SSL:        false,
			},
			contains: []string{
				"server {",
				"listen 80;",
				"server_name example.com;",
				"root /var/www/html;",
				"index index.html index.htm;",
				"location / {",
				"try_files $uri $uri/ =404;",
				"}",
			},
			notContains: []string{
				"ssl_certificate",
				"upstream",
				"proxy_pass",
			},
		},
		{
			name: "带SSL的静态站点配置",
			site: &server.NginxSite{
				Name:       "ssl-static",
				Type:       server.NginxSiteTypeStatic,
				ServerName: "secure.example.com",
				Listen:     []string{"443 ssl"},
				Root:       "/var/www/secure",
				Index:      []string{"index.html"},
				SSL:        true,
				SSLCert:    "/etc/ssl/certs/example.com.crt",
				SSLKey:     "/etc/ssl/private/example.com.key",
				AccessLog:  "/var/log/nginx/secure_access.log",
				ErrorLog:   "/var/log/nginx/secure_error.log",
			},
			contains: []string{
				"listen 443 ssl;",
				"server_name secure.example.com;",
				"ssl_certificate /etc/ssl/certs/example.com.crt;",
				"ssl_certificate_key /etc/ssl/private/example.com.key;",
				"access_log /var/log/nginx/secure_access.log;",
				"error_log /var/log/nginx/secure_error.log;",
				"root /var/www/secure;",
			},
		},
		{
			name: "反向代理站点配置",
			site: &server.NginxSite{
				Name:       "proxy-site",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "api.example.com",
				Listen:     []string{"80"},
				GlobalProxy: &server.NginxProxyConfig{
					ProxyPass: "http://backend:8080",
				},
			},
			contains: []string{
				"server {",
				"listen 80;",
				"server_name api.example.com;",
				"location / {",
				"proxy_pass http://backend:8080;",
				"proxy_set_header Host $host;",
				"proxy_set_header X-Real-IP $remote_addr;",
				"proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
				"proxy_set_header X-Forwarded-Proto $scheme;",
			},
		},
		{
			name: "带upstream的反向代理站点",
			site: &server.NginxSite{
				Name:       "upstream-proxy",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "app.example.com",
				Listen:     []string{"80"},
				Upstream: &server.NginxUpstream{
					Name:        "backend_servers",
					LoadBalance: server.NginxLoadBalanceRoundRobin,
					Servers: []server.NginxUpstreamServer{
						{
							Address:     "192.168.1.10:8080",
							Weight:      2,
							MaxFails:    3,
							FailTimeout: "30s",
						},
						{
							Address: "192.168.1.11:8080",
							Weight:  1,
							Backup:  true,
						},
					},
					KeepAlive: 32,
				},
			},
			contains: []string{
				"upstream backend_servers {",
				"server 192.168.1.10:8080 weight=2 max_fails=3 fail_timeout=30s;",
				"server 192.168.1.11:8080 backup;",
				"keepalive 32;",
				"proxy_pass http://backend_servers;",
			},
		},
		{
			name: "最少连接负载均衡upstream",
			site: &server.NginxSite{
				Name:       "leastconn-proxy",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "lb.example.com",
				Listen:     []string{"80"},
				Upstream: &server.NginxUpstream{
					Name:        "leastconn_backend",
					LoadBalance: server.NginxLoadBalanceLeastConn,
					Servers: []server.NginxUpstreamServer{
						{Address: "10.0.1.1:8080"},
						{Address: "10.0.1.2:8080"},
					},
				},
			},
			contains: []string{
				"upstream leastconn_backend {",
				"least_conn;",
				"server 10.0.1.1:8080;",
				"server 10.0.1.2:8080;",
			},
		},
		{
			name: "IP哈希负载均衡upstream",
			site: &server.NginxSite{
				Name:       "iphash-proxy",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "iphash.example.com",
				Listen:     []string{"80"},
				Upstream: &server.NginxUpstream{
					Name:        "iphash_backend",
					LoadBalance: server.NginxLoadBalanceIPHash,
					Servers: []server.NginxUpstreamServer{
						{Address: "10.0.2.1:8080"},
						{Address: "10.0.2.2:8080"},
					},
				},
			},
			contains: []string{
				"upstream iphash_backend {",
				"ip_hash;",
			},
		},
		{
			name: "自定义哈希负载均衡upstream",
			site: &server.NginxSite{
				Name:       "hash-proxy",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "hash.example.com",
				Listen:     []string{"80"},
				Upstream: &server.NginxUpstream{
					Name:        "hash_backend",
					LoadBalance: server.NginxLoadBalanceHash,
					HashKey:     "$request_uri",
					Servers: []server.NginxUpstreamServer{
						{Address: "10.0.3.1:8080"},
						{Address: "10.0.3.2:8080"},
					},
				},
			},
			contains: []string{
				"upstream hash_backend {",
				"hash $request_uri;",
			},
		},
		{
			name: "随机负载均衡upstream",
			site: &server.NginxSite{
				Name:       "random-proxy",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "random.example.com",
				Listen:     []string{"80"},
				Upstream: &server.NginxUpstream{
					Name:        "random_backend",
					LoadBalance: server.NginxLoadBalanceRandom,
					Servers: []server.NginxUpstreamServer{
						{Address: "10.0.4.1:8080"},
						{Address: "10.0.4.2:8080"},
					},
				},
			},
			contains: []string{
				"upstream random_backend {",
				"random;",
			},
		},
		{
			name: "带自定义location的站点",
			site: &server.NginxSite{
				Name:       "custom-locations",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "api.example.com",
				Listen:     []string{"80"},
				Locations: []server.NginxLocation{
					{
						Path: "/api/v1/",
						ProxyConfig: &server.NginxProxyConfig{
							ProxyPass: "http://api_v1_servers",
							ProxySetHeader: map[string]string{
								"Host":              "$host",
								"X-API-Version":     "v1",
								"X-Forwarded-Proto": "$scheme",
							},
							ProxyConnectTimeout: "5s",
							ProxyReadTimeout:    "60s",
						},
						Headers: map[string]string{
							"X-Service": "api-v1",
						},
					},
					{
						Path:     "/static/",
						TryFiles: []string{"$uri", "$uri/", "=404"},
						Headers: map[string]string{
							"Cache-Control": "public, max-age=3600",
						},
						ExtraConfig: "root /var/www/static;",
					},
				},
			},
			contains: []string{
				"location /api/v1/ {",
				"proxy_pass http://api_v1_servers;",
				"proxy_set_header Host $host;",
				"proxy_set_header X-API-Version v1;",
				"proxy_connect_timeout 5s;",
				"proxy_read_timeout 60s;",
				"add_header X-Service api-v1;",
				"location /static/ {",
				"try_files $uri $uri/ =404;",
				"add_header Cache-Control public, max-age=3600;",
				"root /var/www/static;",
			},
		},
		{
			name: "带速率限制的location",
			site: &server.NginxSite{
				Name:       "rate-limited",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "limited.example.com",
				Listen:     []string{"80"},
				Locations: []server.NginxLocation{
					{
						Path: "/api/",
						ProxyConfig: &server.NginxProxyConfig{
							ProxyPass: "http://backend",
						},
						RateLimit: &server.NginxRateLimit{
							Zone:    "api_limit",
							Burst:   10,
							NoDelay: true,
						},
					},
				},
			},
			contains: []string{
				"location /api/ {",
				"limit_req zone=api_limit burst=10 nodelay;",
			},
		},
		{
			name: "带全局代理配置的站点",
			site: &server.NginxSite{
				Name:       "global-proxy",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "proxy.example.com",
				Listen:     []string{"80"},
				GlobalProxy: &server.NginxProxyConfig{
					ProxyTimeout:        "30s",
					ProxyConnectTimeout: "5s",
					ProxyReadTimeout:    "60s",
					ProxyBuffering:      &[]bool{false}[0],
					ProxyBufferSize:     "8k",
					ProxyBuffers:        "8 8k",
					ProxyRedirect:       "off",
				},
			},
			contains: []string{
				"proxy_timeout 30s;",
				"proxy_connect_timeout 5s;",
				"proxy_read_timeout 60s;",
				"proxy_buffering off;",
				"proxy_buffer_size 8k;",
				"proxy_buffers 8 8k;",
				"proxy_redirect off;",
			},
		},
		{
			name: "带额外配置的站点",
			site: &server.NginxSite{
				Name:        "extra-config",
				Type:        server.NginxSiteTypeStatic,
				ServerName:  "extra.example.com",
				Listen:      []string{"80"},
				Root:        "/var/www/html",
				ExtraConfig: "client_max_body_size 10M;\ngzip on;",
			},
			contains: []string{
				"# 额外配置",
				"client_max_body_size 10M;",
				"gzip on;",
			},
		},
		{
			name: "完整的upstream配置",
			site: &server.NginxSite{
				Name:       "full-upstream",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "full.example.com",
				Listen:     []string{"80"},
				Upstream: &server.NginxUpstream{
					Name:             "full_backend",
					LoadBalance:      server.NginxLoadBalanceLeastConn,
					KeepAlive:        64,
					KeepaliveTime:    "1h",
					KeepaliveTimeout: "60s",
					Servers: []server.NginxUpstreamServer{
						{
							Address:     "192.168.1.10:8080",
							Weight:      3,
							MaxFails:    5,
							FailTimeout: "30s",
							SlowStart:   "30s",
						},
						{
							Address: "192.168.1.11:8080",
							Down:    true,
						},
					},
				},
			},
			contains: []string{
				"upstream full_backend {",
				"least_conn;",
				"server 192.168.1.10:8080 weight=3 max_fails=5 fail_timeout=30s slow_start=30s;",
				"server 192.168.1.11:8080 down;",
				"keepalive 64;",
				"keepalive_time 1h;",
				"keepalive_timeout 60s;",
			},
		},
		{
			name: "多监听端口站点",
			site: &server.NginxSite{
				Name:       "multi-listen",
				Type:       server.NginxSiteTypeStatic,
				ServerName: "multi.example.com",
				Listen:     []string{"80", "8080", "443 ssl"},
				Root:       "/var/www/html",
				SSL:        true,
				SSLCert:    "/etc/ssl/certs/multi.crt",
				SSLKey:     "/etc/ssl/private/multi.key",
			},
			contains: []string{
				"listen 80;",
				"listen 8080;",
				"listen 443 ssl;",
				"ssl_certificate /etc/ssl/certs/multi.crt;",
				"ssl_certificate_key /etc/ssl/private/multi.key;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置创建时间
			tt.site.CreatedAt = time.Now()
			tt.site.UpdatedAt = time.Now()

			config := service.generateSiteConfig(tt.site)

			// 检查必须包含的内容
			for _, expected := range tt.contains {
				assert.Contains(t, config, expected, "配置应该包含: %s", expected)
			}

			// 检查不应该包含的内容
			for _, notExpected := range tt.notContains {
				assert.NotContains(t, config, notExpected, "配置不应该包含: %s", notExpected)
			}

			// 检查基本结构
			assert.Contains(t, config, "server {", "配置应该包含server块")
			// 检查配置基本结构：应该包含server块和对应的结束符
			serverBlockCount := strings.Count(config, "server {")
			assert.Equal(t, 1, serverBlockCount, "应该只有一个server块")
			// 确保配置以}结尾（nginx配置的结构完整性）
			assert.True(t, strings.HasSuffix(strings.TrimSpace(config), "}"), "配置应该以}结尾")

			// 输出配置以便调试
			t.Logf("Generated config for %s:\n%s", tt.name, config)
		})
	}
}

func TestService_generateUpstreamConfig(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name     string
		upstream *server.NginxUpstream
		contains []string
	}{
		{
			name: "基本upstream配置",
			upstream: &server.NginxUpstream{
				Name: "basic_backend",
				Servers: []server.NginxUpstreamServer{
					{Address: "127.0.0.1:8080"},
				},
			},
			contains: []string{
				"upstream basic_backend {",
				"server 127.0.0.1:8080;",
				"}",
			},
		},
		{
			name: "带权重的upstream配置",
			upstream: &server.NginxUpstream{
				Name: "weighted_backend",
				Servers: []server.NginxUpstreamServer{
					{
						Address:  "10.0.1.1:8080",
						Weight:   3,
						MaxFails: 3,
					},
					{
						Address: "10.0.1.2:8080",
						Weight:  1,
					},
				},
			},
			contains: []string{
				"upstream weighted_backend {",
				"server 10.0.1.1:8080 weight=3 max_fails=3;",
				"server 10.0.1.2:8080;",
			},
		},
		{
			name: "复杂upstream配置",
			upstream: &server.NginxUpstream{
				Name:        "complex_backend",
				LoadBalance: server.NginxLoadBalanceLeastConn,
				KeepAlive:   32,
				Servers: []server.NginxUpstreamServer{
					{
						Address:     "10.0.1.1:8080",
						Weight:      2,
						MaxFails:    3,
						FailTimeout: "30s",
						SlowStart:   "30s",
					},
					{
						Address: "10.0.1.2:8080",
						Backup:  true,
					},
					{
						Address: "10.0.1.3:8080",
						Down:    true,
					},
				},
			},
			contains: []string{
				"upstream complex_backend {",
				"least_conn;",
				"server 10.0.1.1:8080 weight=2 max_fails=3 fail_timeout=30s slow_start=30s;",
				"server 10.0.1.2:8080 backup;",
				"server 10.0.1.3:8080 down;",
				"keepalive 32;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := service.generateUpstreamConfig(tt.upstream)

			for _, expected := range tt.contains {
				assert.Contains(t, config, expected, "upstream配置应该包含: %s", expected)
			}

			// 输出配置以便调试
			t.Logf("Generated upstream config for %s:\n%s", tt.name, config)
		})
	}
}

func TestService_generateLocationConfig(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name     string
		location *server.NginxLocation
		expected []string
	}{
		{
			name: "基本代理location",
			location: &server.NginxLocation{
				Path: "/api/",
				ProxyConfig: &server.NginxProxyConfig{
					ProxyPass: "http://backend",
				},
			},
			expected: []string{
				"location /api/ {",
				"proxy_pass http://backend;",
				"}",
			},
		},
		{
			name: "带头部的location",
			location: &server.NginxLocation{
				Path: "/assets/",
				Headers: map[string]string{
					"Cache-Control": "public, max-age=3600",
					"X-Static":      "true",
				},
			},
			expected: []string{
				"location /assets/ {",
				"add_header Cache-Control public, max-age=3600;",
				"add_header X-Static true;",
			},
		},
		{
			name: "带try_files的location",
			location: &server.NginxLocation{
				Path:     "/",
				TryFiles: []string{"$uri", "$uri/", "=404"},
			},
			expected: []string{
				"location / {",
				"try_files $uri $uri/ =404;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config strings.Builder
			service.generateLocationConfig(&config, tt.location)
			result := config.String()

			for _, expected := range tt.expected {
				assert.Contains(t, result, expected, "location配置应该包含: %s", expected)
			}

			// 输出配置以便调试
			t.Logf("Generated location config for %s:\n%s", tt.name, result)
		})
	}
}

// 测试边界情况和错误场景
func TestService_generateSiteConfigEdgeCases(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name string
		site *server.NginxSite
		desc string
	}{
		{
			name: "空listen数组",
			site: &server.NginxSite{
				Name:       "empty-listen",
				Type:       server.NginxSiteTypeStatic,
				ServerName: "example.com",
				Listen:     []string{},
				Root:       "/var/www/html",
			},
			desc: "应该处理空的listen数组",
		},
		{
			name: "无SSL配置但SSL为true",
			site: &server.NginxSite{
				Name:       "ssl-no-certs",
				Type:       server.NginxSiteTypeStatic,
				ServerName: "example.com",
				Listen:     []string{"443 ssl"},
				Root:       "/var/www/html",
				SSL:        true,
				// 没有设置SSLCert和SSLKey
			},
			desc: "应该处理SSL为true但没有证书配置的情况",
		},
		{
			name: "代理站点无任何代理配置",
			site: &server.NginxSite{
				Name:       "proxy-no-config",
				Type:       server.NginxSiteTypeProxy,
				ServerName: "proxy.example.com",
				Listen:     []string{"80"},
				// 没有Upstream、GlobalProxy或Locations配置
			},
			desc: "应该处理代理站点但没有任何代理配置的情况",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 这些测试主要确保不会panic，并生成有效的配置
			config := service.generateSiteConfig(tt.site)

			// 基本检查：应该包含server块
			assert.Contains(t, config, "server {", "配置应该包含server块")
			assert.Contains(t, config, "}", "配置应该有闭合的大括号")
			assert.Contains(t, config, fmt.Sprintf("server_name %s;", tt.site.ServerName),
				"配置应该包含server_name")

			t.Logf("Generated config for edge case %s:\n%s", tt.name, config)
		})
	}
}
