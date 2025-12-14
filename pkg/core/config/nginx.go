package config

import "time"

// NginxConfig Nginx 管理组件配置
type NginxConfig struct {
	// RootDir 配置文件根目录（默认 /etc/nginx/conf.d）
	RootDir string `yaml:"root_dir"`
	// ValidateCommand 配置校验命令（默认 nginx -t）
	ValidateCommand string `yaml:"validate_command"`
	// ReloadCommand 配置重载命令（默认 nginx -s reload）
	ReloadCommand string `yaml:"reload_command"`
	// CommandTimeout 命令执行超时时间（默认 30s）
	CommandTimeout time.Duration `yaml:"command_timeout"`
	// FileMode 配置文件权限（默认 0644）
	FileMode string `yaml:"file_mode"`
}

// DefaultNginxConfig 返回默认配置
func DefaultNginxConfig() NginxConfig {
	return NginxConfig{
		RootDir:         "/etc/nginx/conf.d",
		ValidateCommand: "nginx -t",
		ReloadCommand:   "nginx -s reload",
		CommandTimeout:  30 * time.Second,
		FileMode:        "0644",
	}
}




