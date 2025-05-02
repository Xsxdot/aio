package config

import (
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/utils"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// NetworkConfig 表示全局网络配置
type NetworkConfig struct {
	BindIP            string `yaml:"bind_ip"`
	LocalIP           string `yaml:"local_ip"`
	HttpAllowExternal bool   `yaml:"http_allow_external"`
	PublicIp          string `yaml:"public_ip"`
	HttpPort          int    `yaml:"http_port"`
}

// SystemConfig 表示系统配置
type SystemConfig struct {
	Mode       string `yaml:"mode"`
	NodeId     string `yaml:"node_id"`
	DataDir    string `yaml:"data_dir"`
	ConfigSalt string `yaml:"config_salt"`
}

type Node struct {
	NodeId string `json:"node_id" yaml:"node_id"`
	Addr   string `json:"addr" yaml:"addr"`
	Port   int    `json:"port" yaml:"port"`
	Master bool   `json:"master" yaml:"master"`
}

// ErrorsConfig 表示错误处理配置
type ErrorsConfig struct {
	DebugMode bool `yaml:"debug_mode"`
}

type EtcdConfig struct {
	ClientPort          int       `json:"client_port" yaml:"client_port"`
	PeerPort            int       `json:"peer_port" yaml:"peer_port"`
	InitialClusterToken string    `json:"initial_cluster_token" yaml:"initial_cluster_token"`
	AuthToken           string    `json:"auth_token" yaml:"auth_token"`
	ClientTLSConfig     TLSConfig `json:"client_tls_config" yaml:"client_tls_config"`
	ServerTLSConfig     TLSConfig `json:"server_tls_config" yaml:"server_tls_config"`
	Jwt                 bool      `json:"jwt" yaml:"jwt"`
	Username            string    `json:"username" yaml:"username"`
	Password            string    `json:"password" yaml:"password"`
}

// TLSConfig 表示TLS配置
type TLSConfig struct {
	TLSEnabled bool   `yaml:"tls_enabled" json:"tls_enabled,omitempty"`
	AutoTls    bool   `yaml:"auto_tls" json:"auto_tls,omitempty"`
	Cert       string `yaml:"cert_file" json:"cert,omitempty"`
	Key        string `yaml:"key_file" json:"key,omitempty"`
	TrustedCA  string `yaml:"trusted_ca_file" json:"trusted_ca,omitempty"`
}

// BaseConfig 表示应用程序配置
type BaseConfig struct {
	System   *SystemConfig     `yaml:"system"`
	Network  *NetworkConfig    `yaml:"network"`
	Logger   *common.LogConfig `yaml:"logger"`
	Errors   *ErrorsConfig     `yaml:"errors"`
	Protocol *ProtocolConfig   `yaml:"protocol"`
	Nodes    []*Node           `yaml:"nodes"`
	Etcd     *EtcdConfig       `yaml:"etcd"`
	Monitor  *MonitorConfig    `yaml:"monitor"`
}

type SSLConfig struct {
	Email string `yaml:"email" json:"email"` // Let's Encrypt账户邮箱
}

type MonitorConfig struct {
	// CollectInterval 指定服务器指标采集的间隔时间（秒）
	CollectInterval int `json:"collect_interval" yaml:"collect_interval"`

	// RetentionDays 指定数据保留的天数，默认为15天
	RetentionDays int `json:"retention_days" yaml:"retention_days"`
}

type CacheConfig struct {
	// 基本配置
	Port       int    `yaml:"port" json:"port"`               // 服务器端口
	DBCount    int    `yaml:"db_count" json:"db_count"`       // 数据库数量
	MaxMemory  int64  `yaml:"max_memory" json:"max_memory"`   // 最大内存使用量(MB)，0表示不限制
	MaxClients int    `yaml:"max_clients" json:"max_clients"` // 最大客户端连接数
	Password   string `yaml:"password" json:"password"`       // 访问密码，空表示不需要密码

	// 持久化配置
	EnableRDB       bool   `yaml:"enable_rdb" json:"enable_rdb"`               // 是否启用RDB持久化
	RDBFilePath     string `yaml:"rdb_file_path" json:"rdb_file_path"`         // RDB文件路径
	RDBSaveInterval int    `yaml:"rdb_save_interval" json:"rdb_save_interval"` // RDB保存间隔(秒)
	EnableAOF       bool   `yaml:"enable_aof" json:"enable_aof"`               // 是否启用AOF持久化
	AOFFilePath     string `yaml:"aof_file_path" json:"aof_file_path"`         // AOF文件路径
	AOFSyncStrategy int    `yaml:"aof_sync_strategy" json:"aof_sync_strategy"` // AOF同步策略：0=always, 1=everysec, 2=no
}

type NatsConfig struct {
	Port    int    `yaml:"port" json:"port"`
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// 监听集群端口
	ClusterPort int `json:"cluster_port" yaml:"cluster_port"`
	// 集群名称
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
	// 集群地址，用于集群节点间通信
	Routes []string `json:"routes" yaml:"routes"`
	// 最大连接数
	MaxConnections int `json:"max_connections" yaml:"max_connections"`
	// 最大控制线路大小
	MaxControlLine int `json:"max_control_line" yaml:"max_control_line"`
	// 最大有效负载大小
	MaxPayload int `json:"max_payload" yaml:"max_payload"`
	// 写入超时
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	// JetStream配置
	JetStreamEnabled bool  `json:"jet_stream_enabled" yaml:"jet_stream_enabled"`
	JetStreamMaxMem  int64 `json:"jet_stream_max_mem" yaml:"jet_stream_max_mem"`
	JetStreamMaxFile int64 `json:"jet_stream_max_file" yaml:"jet_stream_max_file"`
	// 授权配置
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	// 是否输出debug日志
	Debug bool `json:"debug" yaml:"debug"`
	// 是否输出跟踪日志
	Trace           bool      `json:"trace" yaml:"trace"`
	ClientTLSConfig TLSConfig `json:"client_tls_config" yaml:"client_tls_config"`
	ServerTLSConfig TLSConfig `json:"server_tls_config" yaml:"server_tls_config"`
}

type ProtocolConfig struct {
	Port             int    `yaml:"port"`
	MaxConnections   int    `yaml:"max_connections"`
	BufferSize       int    `yaml:"buffer_size"`
	EnableKeepAlive  bool   `yaml:"enable_keepalive"`
	ReadTimeout      string `yaml:"read_timeout"`      // 新增读超时
	WriteTimeout     string `yaml:"write_timeout"`     // 新增写超时
	IdleTimeout      string `yaml:"idle_timeout"`      // 新增空闲超时
	HeartbeatTimeout string `yaml:"heartbeat_timeout"` // 新增心跳超时
	EnableAuth       bool   `yaml:"enable_auth"`       // 是否启用认证
}

type ReadType string

const (
	ReadTypeFile   = ReadType("file")
	ReadTypeCenter = ReadType("center")
	ReadTypeNil    = ReadType("nil")
)

type ComponentConfig struct {
	Body     []byte
	ReadType ReadType
	Name     string
}

func NewComponentType(name string, t ReadType) *ComponentConfig {
	return &ComponentConfig{
		Name:     name,
		ReadType: t,
	}
}

// LoadConfig 从文件中加载配置
func LoadConfig(configPath string) (*BaseConfig, error) {
	data, err := os.ReadFile(filepath.Join(configPath, "aio.yaml"))
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg BaseConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	//验证数据目录
	dir := cfg.System.DataDir
	if dir == "" {
		dir = "./data"
	}
	logDir := filepath.Dir(dir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	network := cfg.Network
	//先赋默认值
	if network.PublicIp == "" || network.PublicIp == "auto" {
		network.PublicIp = utils.GetPublicIP()
	}
	if network.LocalIP == "" || network.LocalIP == "auto" {
		network.LocalIP = utils.GetLocalIP()
	}

	if network.BindIP == "local" {
		network.BindIP = network.LocalIP
	} else {
		network.BindIP = "0.0.0.0"
	}

	for _, node := range cfg.Nodes {
		if node.NodeId == cfg.System.NodeId {
			node.Addr = network.LocalIP
		}
	}

	return &cfg, nil
}
