package config

import "time"

// SystemdConfig Systemd 管理组件配置
type SystemdConfig struct {
	// RootDir unit 文件根目录（默认 /etc/systemd/system）
	RootDir string `yaml:"root_dir"`
	// CommandTimeout 命令执行超时时间（默认 30s）
	CommandTimeout time.Duration `yaml:"command_timeout"`
}

// DefaultSystemdConfig 返回默认配置
func DefaultSystemdConfig() SystemdConfig {
	return SystemdConfig{
		RootDir:        "/etc/systemd/system",
		CommandTimeout: 30 * time.Second,
	}
}

