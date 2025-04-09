package config

import (
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/utils"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// NetworkConfig 表示全局网络配置
type NetworkConfig struct {
	BindIP            string `yaml:"bind_ip"`
	LocalIp           string `yaml:"local_ip"`
	AllowExternal     bool   `yaml:"allow_external"`
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

// ErrorsConfig 表示错误处理配置
type ErrorsConfig struct {
	DebugMode bool `yaml:"debug_mode"`
}

// BaseConfig 表示应用程序配置
type BaseConfig struct {
	System   *SystemConfig     `yaml:"system"`
	Network  *NetworkConfig    `yaml:"network"`
	Logger   *common.LogConfig `yaml:"logger"`
	Errors   *ErrorsConfig     `yaml:"errors"`
	Protocol *ProtocolConfig   `yaml:"protocol"`
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
func LoadConfig(configPath string) (*BaseConfig, bool, error) {
	data, err := os.ReadFile(filepath.Join(configPath, "aio.yaml"))
	if err != nil {
		return nil, false, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg BaseConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("解析配置文件失败: %w", err)
	}

	//验证数据目录
	dir := cfg.System.DataDir
	if dir == "" {
		dir = "./data"
	}
	logDir := filepath.Dir(dir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, false, fmt.Errorf("创建数据目录失败: %w", err)
	}

	localIP := utils.GetLocalIP()
	publicIP := utils.GetPublicIP()

	network := cfg.Network
	//先赋默认值
	if network.PublicIp == "" || network.PublicIp == "auto" {
		network.PublicIp = publicIP
	}
	if network.LocalIp == "" || network.PublicIp == "auto" {
		network.LocalIp = localIP
	}

	// 处理GlobalIP自动检测
	if network.AllowExternal || network.BindIP == "0.0.0.0" {
		//如果允许外部访问，则必须使用0.0.0.0
		network.BindIP = "0.0.0.0"
	} else if network.BindIP == "auto" {
		//到这里就说明只绑定内网
		network.PublicIp = network.LocalIp
		network.BindIP = network.LocalIp
	} else {
		network.PublicIp = network.BindIP
		network.LocalIp = network.BindIP
	}

	hasEtcd := true
	etcdFile, err := ioutil.ReadFile(filepath.Join(configPath, "etcd.conf"))
	if err != nil {
		if os.IsNotExist(err) {
			hasEtcd = false
			return &cfg, hasEtcd, nil
		} else {
			return nil, false, fmt.Errorf("读取etcd配置文件失败: %w", err)
		}
	}

	if err := yaml.Unmarshal(etcdFile, &cfg); err != nil {
		return nil, false, fmt.Errorf("解析ETCD配置文件失败: %w", err)
	}

	return &cfg, true, nil
}
