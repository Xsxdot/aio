package config

import "time"

// AgentConfig Agent 守护进程配置
type AgentConfig struct {
	Address string        `yaml:"address"` // gRPC 监听地址
	JWT     JWTConfig     `yaml:"jwt"`     // JWT 配置（用于验证客户端token）
	Nginx   AgentNginxConfig   `yaml:"nginx"`   // Nginx 配置
	Systemd AgentSystemdConfig `yaml:"systemd"` // Systemd 配置
	Timeout time.Duration `yaml:"timeout"` // 命令执行超时
}

// AgentNginxConfig Agent Nginx 配置
type AgentNginxConfig struct {
	RootDir        string `yaml:"root_dir"`         // 配置文件根目录
	FileMode       string `yaml:"file_mode"`        // 文件权限（如 "0644"）
	ValidateCommand string `yaml:"validate_command"` // 校验命令
	ReloadCommand   string `yaml:"reload_command"`   // 重载命令
}

// AgentSystemdConfig Agent Systemd 配置
type AgentSystemdConfig struct {
	UnitDir string `yaml:"unit_dir"` // unit 文件目录
}

