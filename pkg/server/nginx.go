package server

import (
	"context"
	"time"
)

// NginxServiceManager nginx服务管理接口
type NginxServiceManager interface {
	// 服务器管理
	AddNginxServer(ctx context.Context, req *NginxServerCreateRequest) (*NginxServer, error)
	GetNginxServer(ctx context.Context, serverID string) (*NginxServer, error)
	UpdateNginxServer(ctx context.Context, serverID string, req *NginxServerUpdateRequest) (*NginxServer, error)
	DeleteNginxServer(ctx context.Context, serverID string) error
	ListNginxServers(ctx context.Context, req *NginxServerListRequest) ([]*NginxServer, int, error)

	// 配置文件管理
	ListConfigs(ctx context.Context, serverID string, req *NginxConfigListRequest) ([]*NginxConfig, int, error)
	GetConfig(ctx context.Context, serverID, configPath string) (*NginxConfig, error)
	CreateConfig(ctx context.Context, serverID string, req *NginxConfigCreateRequest) (*NginxConfig, error)
	UpdateConfig(ctx context.Context, serverID, configPath string, req *NginxConfigUpdateRequest) (*NginxConfig, error)
	DeleteConfig(ctx context.Context, serverID, configPath string) (*NginxOperationResult, error)

	// nginx操作
	TestConfig(ctx context.Context, serverID string) (*NginxOperationResult, error)
	ReloadConfig(ctx context.Context, serverID string) (*NginxOperationResult, error)
	RestartNginx(ctx context.Context, serverID string) (*NginxOperationResult, error)
	GetNginxStatus(ctx context.Context, serverID string) (*NginxStatusResult, error)

	// 站点管理
	ListSites(ctx context.Context, serverID string, req *NginxSiteListRequest) ([]*NginxSite, int, error)
	GetSite(ctx context.Context, serverID, siteName string) (*NginxSite, error)
	CreateSite(ctx context.Context, serverID string, req *NginxSiteCreateRequest) (*NginxSite, error)
	UpdateSite(ctx context.Context, serverID, siteName string, req *NginxSiteUpdateRequest) (*NginxSite, error)
	DeleteSite(ctx context.Context, serverID, siteName string) (*NginxOperationResult, error)
	EnableSite(ctx context.Context, serverID, siteName string) (*NginxOperationResult, error)
	DisableSite(ctx context.Context, serverID, siteName string) (*NginxOperationResult, error)
}

// NginxConfigType nginx配置文件类型
type NginxConfigType string

const (
	NginxConfigTypeMain   NginxConfigType = "main"   // 主配置文件
	NginxConfigTypeSite   NginxConfigType = "site"   // 站点配置
	NginxConfigTypeModule NginxConfigType = "module" // 模块配置
	NginxConfigTypeCustom NginxConfigType = "custom" // 自定义配置
)

// NginxStatus nginx状态
type NginxStatus string

const (
	NginxStatusRunning NginxStatus = "running" // 运行中
	NginxStatusStopped NginxStatus = "stopped" // 已停止
	NginxStatusError   NginxStatus = "error"   // 错误状态
	NginxStatusUnknown NginxStatus = "unknown" // 未知状态
)

// NginxServer nginx服务器配置
type NginxServer struct {
	ServerID       string    `json:"serverId"`       // 关联的服务器ID
	NginxPath      string    `json:"nginxPath"`      // nginx安装路径
	ConfigPath     string    `json:"configPath"`     // 配置文件保存路径
	SitesEnabled   string    `json:"sitesEnabled"`   // 启用站点目录
	SitesAvailable string    `json:"sitesAvailable"` // 可用站点目录
	LogPath        string    `json:"logPath"`        // 日志路径
	Version        string    `json:"version"`        // nginx版本
	Status         string    `json:"status"`         // nginx状态
	CreatedAt      time.Time `json:"createdAt"`      // 创建时间
	UpdatedAt      time.Time `json:"updatedAt"`      // 更新时间
}

// NginxServerCreateRequest 创建nginx服务器请求
type NginxServerCreateRequest struct {
	ServerID       string `json:"serverId" validate:"required"`   // 关联的服务器ID
	NginxPath      string `json:"nginxPath" validate:"required"`  // nginx安装路径
	ConfigPath     string `json:"configPath" validate:"required"` // 配置文件保存路径
	SitesEnabled   string `json:"sitesEnabled"`                   // 启用站点目录
	SitesAvailable string `json:"sitesAvailable"`                 // 可用站点目录
	LogPath        string `json:"logPath"`                        // 日志路径
}

// NginxServerUpdateRequest 更新nginx服务器请求
type NginxServerUpdateRequest struct {
	NginxPath      *string `json:"nginxPath,omitempty"`      // nginx安装路径
	ConfigPath     *string `json:"configPath,omitempty"`     // 配置文件保存路径
	SitesEnabled   *string `json:"sitesEnabled,omitempty"`   // 启用站点目录
	SitesAvailable *string `json:"sitesAvailable,omitempty"` // 可用站点目录
	LogPath        *string `json:"logPath,omitempty"`        // 日志路径
}

// NginxServerListRequest nginx服务器列表查询请求
type NginxServerListRequest struct {
	Limit  int    `json:"limit"`  // 分页大小
	Offset int    `json:"offset"` // 分页偏移
	Status string `json:"status"` // 状态过滤
}

// NginxConfig nginx配置文件
type NginxConfig struct {
	ServerID    string          `json:"serverId"`    // 服务器ID
	Path        string          `json:"path"`        // 配置文件路径
	Name        string          `json:"name"`        // 配置文件名称
	Type        NginxConfigType `json:"type"`        // 配置文件类型
	Content     string          `json:"content"`     // 配置内容
	Size        int64           `json:"size"`        // 文件大小
	IsDirectory bool            `json:"isDirectory"` // 是否为目录
	ModTime     time.Time       `json:"modTime"`     // 修改时间
	CreatedAt   time.Time       `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time       `json:"updatedAt"`   // 更新时间
}

// NginxConfigListRequest nginx配置文件列表查询请求
type NginxConfigListRequest struct {
	Path   string          `json:"path"`   // 目录路径，为空时查询根配置目录
	Type   NginxConfigType `json:"type"`   // 配置文件类型过滤
	Limit  int             `json:"limit"`  // 分页大小
	Offset int             `json:"offset"` // 分页偏移
}

// NginxConfigCreateRequest 创建nginx配置文件请求
type NginxConfigCreateRequest struct {
	Path    string          `json:"path" validate:"required"`    // 配置文件路径
	Name    string          `json:"name" validate:"required"`    // 配置文件名称
	Type    NginxConfigType `json:"type"`                        // 配置文件类型
	Content string          `json:"content" validate:"required"` // 配置内容
}

// NginxConfigUpdateRequest 更新nginx配置文件请求
type NginxConfigUpdateRequest struct {
	Content *string `json:"content,omitempty"` // 配置内容
}

// NginxSiteType 站点类型
type NginxSiteType string

const (
	NginxSiteTypeStatic NginxSiteType = "static" // 静态站点
	NginxSiteTypeProxy  NginxSiteType = "proxy"  // 反向代理
)

// NginxLoadBalanceMethod 负载均衡方法
type NginxLoadBalanceMethod string

const (
	NginxLoadBalanceRoundRobin NginxLoadBalanceMethod = "round_robin" // 轮询（默认）
	NginxLoadBalanceLeastConn  NginxLoadBalanceMethod = "least_conn"  // 最少连接
	NginxLoadBalanceIPHash     NginxLoadBalanceMethod = "ip_hash"     // IP哈希
	NginxLoadBalanceHash       NginxLoadBalanceMethod = "hash"        // 通用哈希
	NginxLoadBalanceRandom     NginxLoadBalanceMethod = "random"      // 随机
)

// NginxUpstreamServer upstream服务器配置
type NginxUpstreamServer struct {
	Address     string `json:"address" validate:"required"` // 服务器地址 (ip:port 或 domain:port)
	Weight      int    `json:"weight"`                      // 权重 (默认1)
	MaxFails    int    `json:"maxFails"`                    // 最大失败次数 (默认1)
	FailTimeout string `json:"failTimeout"`                 // 失败超时时间 (默认10s)
	Backup      bool   `json:"backup"`                      // 是否为备用服务器
	Down        bool   `json:"down"`                        // 是否下线
	SlowStart   string `json:"slowStart,omitempty"`         // 慢启动时间
}

// NginxUpstream upstream配置
type NginxUpstream struct {
	Name             string                 `json:"name" validate:"required"`    // upstream名称
	Servers          []NginxUpstreamServer  `json:"servers" validate:"required"` // 服务器列表
	LoadBalance      NginxLoadBalanceMethod `json:"loadBalance"`                 // 负载均衡方法
	HashKey          string                 `json:"hashKey,omitempty"`           // hash键（用于hash负载均衡）
	KeepAlive        int                    `json:"keepAlive,omitempty"`         // 保持连接数
	KeepaliveTime    string                 `json:"keepaliveTime,omitempty"`     // 保持连接时间
	KeepaliveTimeout string                 `json:"keepaliveTimeout,omitempty"`  // 保持连接超时
	HealthCheck      *NginxHealthCheck      `json:"healthCheck,omitempty"`       // 健康检查配置
}

// NginxHealthCheck 健康检查配置
type NginxHealthCheck struct {
	Enabled      bool   `json:"enabled"`      // 是否启用健康检查
	URI          string `json:"uri"`          // 健康检查URI
	Interval     string `json:"interval"`     // 检查间隔
	Timeout      string `json:"timeout"`      // 检查超时
	Rises        int    `json:"rises"`        // 成功次数阈值
	Falls        int    `json:"falls"`        // 失败次数阈值
	ExpectedCode int    `json:"expectedCode"` // 期望的HTTP状态码
}

// NginxProxyConfig 代理配置
type NginxProxyConfig struct {
	ProxyPass           string            `json:"proxyPass"`                     // 代理地址
	ProxySetHeader      map[string]string `json:"proxySetHeader,omitempty"`      // 设置头部
	ProxyTimeout        string            `json:"proxyTimeout,omitempty"`        // 代理超时
	ProxyConnectTimeout string            `json:"proxyConnectTimeout,omitempty"` // 连接超时
	ProxyReadTimeout    string            `json:"proxyReadTimeout,omitempty"`    // 读取超时
	ProxyBuffering      *bool             `json:"proxyBuffering,omitempty"`      // 是否缓冲
	ProxyBufferSize     string            `json:"proxyBufferSize,omitempty"`     // 缓冲区大小
	ProxyBuffers        string            `json:"proxyBuffers,omitempty"`        // 缓冲区数量和大小
	ProxyRedirect       string            `json:"proxyRedirect,omitempty"`       // 重定向处理
}

// NginxLocation location配置块
type NginxLocation struct {
	Path        string            `json:"path" validate:"required"` // location路径
	ProxyConfig *NginxProxyConfig `json:"proxyConfig,omitempty"`    // 代理配置
	TryFiles    []string          `json:"tryFiles,omitempty"`       // try_files配置
	ExtraConfig string            `json:"extraConfig,omitempty"`    // 额外配置
	Headers     map[string]string `json:"headers,omitempty"`        // 自定义头部
	RateLimit   *NginxRateLimit   `json:"rateLimit,omitempty"`      // 速率限制
}

// NginxRateLimit 速率限制配置
type NginxRateLimit struct {
	Zone    string `json:"zone"`    // 限制区域名称
	Rate    string `json:"rate"`    // 限制速率 (如: 10r/s)
	Burst   int    `json:"burst"`   // 突发请求数
	NoDelay bool   `json:"nodelay"` // 是否无延迟
}

// NginxSite nginx站点配置
type NginxSite struct {
	ServerID    string        `json:"serverId"`              // 服务器ID
	Name        string        `json:"name"`                  // 站点名称
	Type        NginxSiteType `json:"type"`                  // 站点类型
	ServerName  string        `json:"serverName"`            // 域名
	Listen      []string      `json:"listen"`                // 监听端口
	Root        string        `json:"root,omitempty"`        // 根目录（静态站点）
	Index       []string      `json:"index,omitempty"`       // 索引文件（静态站点）
	AccessLog   string        `json:"accessLog"`             // 访问日志
	ErrorLog    string        `json:"errorLog"`              // 错误日志
	SSL         bool          `json:"ssl"`                   // 是否启用SSL
	SSLCert     string        `json:"sslCert,omitempty"`     // SSL证书路径
	SSLKey      string        `json:"sslKey,omitempty"`      // SSL私钥路径
	Enabled     bool          `json:"enabled"`               // 是否启用
	ConfigPath  string        `json:"configPath"`            // 配置文件路径
	ConfigMode  string        `json:"configMode"`            // 配置模式（auto/manual）
	ExtraConfig string        `json:"extraConfig,omitempty"` // 额外配置

	// 反向代理相关配置
	Upstream    *NginxUpstream    `json:"upstream,omitempty"`    // upstream配置（反向代理）
	Locations   []NginxLocation   `json:"locations,omitempty"`   // location配置块
	GlobalProxy *NginxProxyConfig `json:"globalProxy,omitempty"` // 全局代理配置

	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// NginxSiteListRequest nginx站点列表查询请求
type NginxSiteListRequest struct {
	Enabled *bool  `json:"enabled"` // 启用状态过滤
	SSL     *bool  `json:"ssl"`     // SSL状态过滤
	Pattern string `json:"pattern"` // 站点名称模式匹配
	Limit   int    `json:"limit"`   // 分页大小
	Offset  int    `json:"offset"`  // 分页偏移
}

// NginxSiteCreateRequest 创建nginx站点请求
type NginxSiteCreateRequest struct {
	Name        string        `json:"name" validate:"required"`       // 站点名称
	Type        NginxSiteType `json:"type"`                           // 站点类型
	ServerName  string        `json:"serverName" validate:"required"` // 域名
	Listen      []string      `json:"listen"`                         // 监听端口
	Root        string        `json:"root,omitempty"`                 // 根目录（静态站点时必填）
	Index       []string      `json:"index"`                          // 索引文件
	AccessLog   string        `json:"accessLog"`                      // 访问日志
	ErrorLog    string        `json:"errorLog"`                       // 错误日志
	SSL         bool          `json:"ssl"`                            // 是否启用SSL
	SSLCert     string        `json:"sslCert"`                        // SSL证书路径
	SSLKey      string        `json:"sslKey"`                         // SSL私钥路径
	Enabled     bool          `json:"enabled"`                        // 是否启用
	ConfigMode  string        `json:"configMode"`                     // 配置模式
	ExtraConfig string        `json:"extraConfig"`                    // 额外配置

	// 反向代理相关配置
	Upstream    *NginxUpstream    `json:"upstream,omitempty"`    // upstream配置（反向代理）
	Locations   []NginxLocation   `json:"locations,omitempty"`   // location配置块
	GlobalProxy *NginxProxyConfig `json:"globalProxy,omitempty"` // 全局代理配置
}

// NginxSiteUpdateRequest 更新nginx站点请求
type NginxSiteUpdateRequest struct {
	Type        *NginxSiteType `json:"type,omitempty"`        // 站点类型
	ServerName  *string        `json:"serverName,omitempty"`  // 域名
	Listen      *[]string      `json:"listen,omitempty"`      // 监听端口
	Root        *string        `json:"root,omitempty"`        // 根目录
	Index       *[]string      `json:"index,omitempty"`       // 索引文件
	AccessLog   *string        `json:"accessLog,omitempty"`   // 访问日志
	ErrorLog    *string        `json:"errorLog,omitempty"`    // 错误日志
	SSL         *bool          `json:"ssl,omitempty"`         // 是否启用SSL
	SSLCert     *string        `json:"sslCert,omitempty"`     // SSL证书路径
	SSLKey      *string        `json:"sslKey,omitempty"`      // SSL私钥路径
	Enabled     *bool          `json:"enabled,omitempty"`     // 是否启用
	ConfigMode  *string        `json:"configMode,omitempty"`  // 配置模式
	ExtraConfig *string        `json:"extraConfig,omitempty"` // 额外配置

	// 反向代理相关配置
	Upstream    *NginxUpstream    `json:"upstream,omitempty"`    // upstream配置（反向代理）
	Locations   *[]NginxLocation  `json:"locations,omitempty"`   // location配置块
	GlobalProxy *NginxProxyConfig `json:"globalProxy,omitempty"` // 全局代理
}

// NginxOperationResult nginx操作结果
type NginxOperationResult struct {
	Success  bool   `json:"success"`          // 是否成功
	Message  string `json:"message"`          // 结果消息
	Error    string `json:"error,omitempty"`  // 错误信息
	Output   string `json:"output,omitempty"` // 命令输出
	ExitCode int    `json:"exitCode"`         // 退出码
}

// NginxStatusResult nginx状态结果
type NginxStatusResult struct {
	Status       NginxStatus `json:"status"`          // nginx状态
	Version      string      `json:"version"`         // 版本信息
	PID          int         `json:"pid"`             // 进程ID
	Uptime       string      `json:"uptime"`          // 运行时间
	ActiveConns  int         `json:"activeConns"`     // 活跃连接数
	TotalConns   int64       `json:"totalConns"`      // 总连接数
	RequestsRate float64     `json:"requestsRate"`    // 请求速率
	Error        string      `json:"error,omitempty"` // 错误信息
}
