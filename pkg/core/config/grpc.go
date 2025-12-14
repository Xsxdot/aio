package config

import "time"

// GRPCConfig gRPC服务配置
type GRPCConfig struct {
	// Address 监听地址
	Address string `yaml:"address"`
	// EnableReflection 是否启用反射服务
	EnableReflection bool `yaml:"enable_reflection"`
	// EnableRecovery 是否启用恢复中间件
	EnableRecovery bool `yaml:"enable_recovery"`
	// EnableValidation 是否启用参数验证中间件
	EnableValidation bool `yaml:"enable_validation"`
	// EnableAuth 是否启用鉴权中间件
	EnableAuth bool `yaml:"enable_auth"`
	// EnablePermission 是否启用权限验证中间件
	EnablePermission bool `yaml:"enable_permission"`
	// LogLevel 日志级别
	LogLevel string `yaml:"log_level"`
	// MaxRecvMsgSize 最大接收消息大小（字节）
	MaxRecvMsgSize int `yaml:"max_recv_msg_size"`
	// MaxSendMsgSize 最大发送消息大小（字节）
	MaxSendMsgSize int `yaml:"max_send_msg_size"`
	// ConnectionTimeout 连接超时时间
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
}



