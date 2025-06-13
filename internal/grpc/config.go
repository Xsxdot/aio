package grpc

import (
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Config gRPC 服务器配置
type Config struct {
	// Address 监听地址
	Address string `json:"address" yaml:"address"`
	// EnableReflection 是否启用反射服务
	EnableReflection bool `json:"enable_reflection" yaml:"enable_reflection"`
	// EnableRecovery 是否启用恢复中间件
	EnableRecovery bool `json:"enable_recovery" yaml:"enable_recovery"`
	// EnableValidation 是否启用参数验证中间件
	EnableValidation bool `json:"enable_validation" yaml:"enable_validation"`
	// EnableAuth 是否启用鉴权中间件
	EnableAuth bool `json:"enable_auth" yaml:"enable_auth"`
	// EnablePermission 是否启用权限验证中间件
	EnablePermission bool `json:"enable_permission" yaml:"enable_permission"`
	// LogLevel 日志级别
	LogLevel string `json:"log_level" yaml:"log_level"`
	// MaxRecvMsgSize 最大接收消息大小（字节）
	MaxRecvMsgSize int `json:"max_recv_msg_size" yaml:"max_recv_msg_size"`
	// MaxSendMsgSize 最大发送消息大小（字节）
	MaxSendMsgSize int `json:"max_send_msg_size" yaml:"max_send_msg_size"`
	// ConnectionTimeout 连接超时时间
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`
	// KeepAlive 保活配置
	KeepAlive *KeepAliveConfig `json:"keep_alive" yaml:"keep_alive"`
	// TLS TLS 配置
	TLS *TLSConfig `json:"tls" yaml:"tls"`
	// Auth 鉴权配置
	Auth *AuthConfig `json:"auth" yaml:"auth"`
}

// KeepAliveConfig 保活配置
type KeepAliveConfig struct {
	// Time 保活时间
	Time time.Duration `json:"time" yaml:"time"`
	// Timeout 保活超时
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// PermitWithoutStream 是否允许没有活跃流时发送保活
	PermitWithoutStream bool `json:"permit_without_stream" yaml:"permit_without_stream"`
}

// TLSConfig TLS 配置
type TLSConfig struct {
	// Enable 是否启用 TLS
	Enable bool `json:"enable" yaml:"enable"`
	// CertFile 证书文件路径
	CertFile string `json:"cert_file" yaml:"cert_file"`
	// KeyFile 私钥文件路径
	KeyFile string `json:"key_file" yaml:"key_file"`
	// CAFile CA 证书文件路径
	CAFile string `json:"ca_file" yaml:"ca_file"`
	// ClientAuthRequired 是否要求客户端认证
	ClientAuthRequired bool `json:"client_auth_required" yaml:"client_auth_required"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Address:           ":50051",
		EnableReflection:  true,
		EnableRecovery:    true,
		EnableValidation:  true,
		EnableAuth:        false, // 默认不启用鉴权
		EnablePermission:  false, // 默认不启用权限验证
		LogLevel:          "info",
		MaxRecvMsgSize:    4 * 1024 * 1024, // 4MB
		MaxSendMsgSize:    4 * 1024 * 1024, // 4MB
		ConnectionTimeout: 30 * time.Second,
		KeepAlive: &KeepAliveConfig{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		},
	}
}

// DefaultAuthConfig 返回默认鉴权配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		SkipMethods: []string{
			"/auth.v1.AuthService/ClientAuth",                                // 客户端认证不需要鉴权
			"/grpc.health.v1.Health/Check",                                   // 健康检查不需要鉴权
			"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo", // 反射服务不需要鉴权
		},
		RequireAuth:  true,
		AuthProvider: nil, // 需要外部设置
	}
}

// BuildServerOptions 根据配置构建 gRPC 服务器选项
func (c *Config) BuildServerOptions(logger *zap.Logger) []grpc.ServerOption {
	var opts []grpc.ServerOption

	// 设置消息大小限制
	if c.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(c.MaxRecvMsgSize))
	}
	if c.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(c.MaxSendMsgSize))
	}

	// 构建中间件链
	var interceptors []grpc.UnaryServerInterceptor

	// 恢复中间件（应该最先执行）
	if c.EnableRecovery {
		interceptors = append(interceptors, RecoveryInterceptor(logger))
	}

	// 鉴权中间件
	if c.EnableAuth && c.Auth != nil {
		interceptors = append(interceptors, AuthInterceptor(c.Auth, logger))
	}

	// 权限验证中间件（在鉴权之后）
	if c.EnablePermission && c.Auth != nil {
		interceptors = append(interceptors, PermissionInterceptor(c.Auth, logger))
	}

	// 日志中间件
	interceptors = append(interceptors, unaryLoggingInterceptor(logger))

	// 验证中间件
	if c.EnableValidation {
		interceptors = append(interceptors, ValidationInterceptor())
	}

	// 添加中间件链
	if len(interceptors) > 0 {
		opts = append(opts, grpc.UnaryInterceptor(ChainUnaryInterceptors(interceptors...)))
	}

	// TODO: 添加 TLS 支持
	// if c.TLS != nil && c.TLS.Enable {
	//     // 添加 TLS 证书
	// }

	// TODO: 添加 KeepAlive 支持
	// if c.KeepAlive != nil {
	//     // 添加保活设置
	// }

	return opts
}
