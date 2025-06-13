package config

import (
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

// BaseConfig 表示应用程序配置
type BaseConfig struct {
	System  *SystemConfig     `yaml:"system"`
	Logger  *common.LogConfig `yaml:"logger"`
	Etcd    *EtcdConfig       `yaml:"etcd"`
	Monitor *MonitorConfig    `yaml:"monitor"`
	SSL     *SSLConfig        `yaml:"ssl"`
}

// SystemConfig 表示系统配置
type SystemConfig struct {
	DataDir    string `yaml:"data_dir"`
	ConfigSalt string `yaml:"config_salt"`
	HttpPort   int    `yaml:"http_port"`
	GrpcPort   int    `yaml:"grpc_port"`
}

type MonitorConfig struct {
	// CollectInterval 指定服务器指标采集的间隔时间（秒）
	CollectInterval int `json:"collect_interval" yaml:"collect_interval"`

	// RetentionDays 指定数据保留的天数，默认为15天
	RetentionDays int `json:"retention_days" yaml:"retention_days"`
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
	dir := "./data"
	cfg.System.DataDir = dir
	logDir := filepath.Dir(dir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	return &cfg, nil
}
