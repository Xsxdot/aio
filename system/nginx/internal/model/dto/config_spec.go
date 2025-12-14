package dto

// ConfigType 配置类型
type ConfigType string

const (
	// ConfigTypeProxy 反向代理类型
	ConfigTypeProxy ConfigType = "proxy"
	// ConfigTypeStatic 静态站点类型
	ConfigTypeStatic ConfigType = "static"
)

// UpstreamServer upstream 后端服务器
type UpstreamServer struct {
	Address string `json:"address" validate:"required" comment:"后端地址（host:port）"`
	Weight  int    `json:"weight,omitempty" comment:"权重（可选）"`
	Backup  bool   `json:"backup,omitempty" comment:"是否为备份节点"`
}

// UpstreamConfig upstream 配置
type UpstreamConfig struct {
	Name        string           `json:"name" validate:"required" comment:"upstream 名称"`
	Servers     []UpstreamServer `json:"servers" validate:"required,min=1" comment:"后端服务器列表"`
	LoadBalance string           `json:"loadBalance,omitempty" comment:"负载均衡算法（可选：ip_hash, least_conn）"`
}

// LocationConfig location 配置
type LocationConfig struct {
	Path            string `json:"path" validate:"required" comment:"匹配路径（如 / 或 /api）"`
	ProxyPass       string `json:"proxyPass,omitempty" comment:"代理目标（如 http://127.0.0.1:8080 或 http://upstream_name）"`
	EnableWebSocket bool   `json:"enableWebSocket,omitempty" comment:"是否启用 WebSocket 支持"`
	// 静态站点专用
	Root     string `json:"root,omitempty" comment:"静态文件根目录"`
	Index    string `json:"index,omitempty" comment:"索引文件（如 index.html）"`
	TryFiles string `json:"tryFiles,omitempty" comment:"try_files 规则（如 $uri $uri/ /index.html）"`
}

// ServerConfig server 配置
type ServerConfig struct {
	Listen     int              `json:"listen" validate:"required,min=1,max=65535" comment:"监听端口"`
	ServerName string           `json:"serverName" validate:"required" comment:"域名（多个用空格分隔）"`
	Locations  []LocationConfig `json:"locations" validate:"required,min=1" comment:"location 配置列表"`
	SSLEnabled bool             `json:"sslEnabled,omitempty" comment:"是否启用 SSL"`
	SSLCert    string           `json:"sslCert,omitempty" comment:"SSL 证书路径"`
	SSLKey     string           `json:"sslKey,omitempty" comment:"SSL 私钥路径"`
}

// ConfigSpec 配置规格（用于生成配置文件）
type ConfigSpec struct {
	Type        ConfigType       `json:"type" validate:"required,oneof=proxy static" comment:"配置类型（proxy/static）"`
	Description string           `json:"description,omitempty" comment:"配置描述"`
	Upstreams   []UpstreamConfig `json:"upstreams,omitempty" comment:"upstream 配置列表（可选）"`
	Server      ServerConfig     `json:"server" validate:"required" comment:"server 配置"`
}

