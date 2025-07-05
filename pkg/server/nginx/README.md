# Nginx 服务管理

本模块提供了强大的 Nginx 配置管理功能，支持静态网站和反向代理两种模式，包括负载均衡、SSL 配置等高级功能。

## 功能特性

- ✅ 静态网站配置
- ✅ 反向代理配置
- ✅ 负载均衡支持（轮询、最少连接、IP哈希等）
- ✅ 多节点后端应用转发
- ✅ SSL/TLS 配置
- ✅ 自定义 location 配置
- ✅ 健康检查配置
- ✅ 速率限制
- ✅ WebSocket 支持

## 站点类型

### 1. 静态站点 (NginxSiteTypeStatic)

用于托管静态文件，如 HTML、CSS、JavaScript、图片等。

```go
site := &server.NginxSiteCreateRequest{
    Name:       "static-site",
    Type:       server.NginxSiteTypeStatic,
    ServerName: "example.com",
    Root:       "/var/www/html",
    Index:      []string{"index.html", "index.htm"},
    SSL:        true,
    SSLCert:    "/etc/ssl/certs/example.com.crt",
    SSLKey:     "/etc/ssl/private/example.com.key",
    Enabled:    true,
}
```

### 2. 反向代理站点 (NginxSiteTypeProxy)

用于代理后端应用服务器，支持负载均衡。

```go
// 创建 upstream 配置
upstream := &server.NginxUpstream{
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
            Address:     "192.168.1.11:8080",
            Weight:      1,
            MaxFails:    3,
            FailTimeout: "30s",
        },
        {
            Address:     "192.168.1.12:8080",
            Weight:      1,
            Backup:      true, // 备用服务器
        },
    },
    KeepAlive: 32,
}

site := &server.NginxSiteCreateRequest{
    Name:       "api-gateway",
    Type:       server.NginxSiteTypeProxy,
    ServerName: "api.example.com",
    SSL:        true,
    SSLCert:    "/etc/ssl/certs/api.example.com.crt",
    SSLKey:     "/etc/ssl/private/api.example.com.key",
    Upstream:   upstream,
    Enabled:    true,
}
```

## 负载均衡方法

支持多种负载均衡算法：

- `NginxLoadBalanceRoundRobin`: 轮询（默认）
- `NginxLoadBalanceLeastConn`: 最少连接
- `NginxLoadBalanceIPHash`: IP 哈希
- `NginxLoadBalanceHash`: 通用哈希
- `NginxLoadBalanceRandom`: 随机

## 高级配置示例

### API 网关配置

```go
// 创建多个 location 配置不同的后端服务
locations := []server.NginxLocation{
    // API v1
    {
        Path: "/api/v1/",
        ProxyConfig: &server.NginxProxyConfig{
            ProxyPass: "http://api_v1_servers",
            ProxySetHeader: map[string]string{
                "Host":              "$host",
                "X-Real-IP":         "$remote_addr",
                "X-Forwarded-For":   "$proxy_add_x_forwarded_for",
                "X-Forwarded-Proto": "$scheme",
            },
            ProxyConnectTimeout: "5s",
            ProxyReadTimeout:    "60s",
        },
        Headers: map[string]string{
            "X-API-Version": "v1",
        },
    },
    // API v2
    {
        Path: "/api/v2/",
        ProxyConfig: &server.NginxProxyConfig{
            ProxyPass: "http://api_v2_servers",
            ProxySetHeader: map[string]string{
                "Host":              "$host",
                "X-Real-IP":         "$remote_addr",
                "X-Forwarded-For":   "$proxy_add_x_forwarded_for",
                "X-Forwarded-Proto": "$scheme",
            },
        },
        Headers: map[string]string{
            "X-API-Version": "v2",
        },
        RateLimit: &server.NginxRateLimit{
            Zone:    "api_limit",
            Rate:    "10r/s",
            Burst:   20,
            NoDelay: true,
        },
    },
    // 静态资源
    {
        Path:     "/static/",
        TryFiles: []string{"$uri", "$uri/", "=404"},
        Headers: map[string]string{
            "Cache-Control": "public, max-age=31536000",
            "Expires":       "1y",
        },
        ExtraConfig: "root /var/www/static;",
    },
}

site := &server.NginxSiteCreateRequest{
    Name:       "complex-api",
    Type:       server.NginxSiteTypeProxy,
    ServerName: "api.example.com",
    Locations:  locations,
    Enabled:    true,
}
```

### WebSocket 代理配置

```go
// WebSocket location 配置
wsLocation := server.NginxLocation{
    Path: "/ws/",
    ProxyConfig: &server.NginxProxyConfig{
        ProxyPass: "http://websocket_servers",
        ProxySetHeader: map[string]string{
            "Host":               "$host",
            "X-Real-IP":          "$remote_addr",
            "X-Forwarded-For":    "$proxy_add_x_forwarded_for",
            "X-Forwarded-Proto":  "$scheme",
            "Upgrade":            "$http_upgrade",
            "Connection":         "$connection_upgrade",
        },
        ProxyReadTimeout: "3600s", // WebSocket 长连接
    },
}
```

## 辅助函数

模块提供了一些辅助函数来简化配置创建：

```go
// 创建默认代理头部
headers := CreateDefaultProxyHeaders()

// 创建 WebSocket 代理头部
wsHeaders := CreateWebSocketProxyHeaders()

// 创建默认代理配置
proxyConfig := CreateDefaultProxyConfig("http://backend")

// 创建负载均衡 upstream
upstream := CreateLoadBalancedUpstream(
    "my_servers", 
    []string{"192.168.1.10:8080", "192.168.1.11:8080"},
    server.NginxLoadBalanceRoundRobin,
)

// 创建 API 网关 location
apiLocation := CreateAPIGatewayLocation("/api/", "api_servers")

// 创建静态资源 location
staticLocation := CreateStaticAssetsLocation("/assets/", "/var/www/assets")
```

## 配置验证

系统会自动验证配置的正确性：

- 站点名称和域名不能为空
- 静态站点必须指定根目录
- 代理站点必须配置 upstream 或代理配置
- SSL 配置必须包含证书和私钥路径
- upstream 服务器地址格式验证

## 生成的 Nginx 配置示例

### 反向代理配置

```nginx
upstream backend_servers {
    least_conn;
    server 192.168.1.10:8080 weight=2 max_fails=3 fail_timeout=30s;
    server 192.168.1.11:8080 weight=1 max_fails=3 fail_timeout=30s;
    server 192.168.1.12:8080 backup;
    keepalive 32;
}

server {
    listen 80;
    listen 443 ssl;
    server_name api.example.com;

    ssl_certificate /etc/ssl/certs/api.example.com.crt;
    ssl_certificate_key /etc/ssl/private/api.example.com.key;

    access_log /var/log/nginx/api.example.com.access.log;
    error_log /var/log/nginx/api.example.com.error.log;

    location / {
        proxy_pass http://backend_servers;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 注意事项

1. 确保后端服务器地址格式正确（ip:port 或 domain:port）
2. SSL 证书文件路径必须在服务器上存在
3. upstream 名称在同一 nginx 实例中必须唯一
4. 负载均衡配置会影响请求分发策略
5. 健康检查需要 nginx_upstream_check_module 模块支持 