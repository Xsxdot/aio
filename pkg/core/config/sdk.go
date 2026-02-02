package config

import "time"

// SdkConfig SDK 客户端配置
type SdkConfig struct {
	// RegistryAddr 注册中心地址（必填）
	RegistryAddr string `yaml:"registry_addr"`
	// ClientKey 客户端认证密钥（必填）
	ClientKey string `yaml:"client_key"`
	// ClientSecret 客户端认证密文（必填）
	ClientSecret string `yaml:"client_secret"`
	// DefaultTimeout 默认超时时间
	DefaultTimeout time.Duration `yaml:"default_timeout"`
	// DisableAuth 禁用自动鉴权（用于特殊场景）
	DisableAuth bool `yaml:"disable_auth"`
	// BootstrapConfigPrefix 启动配置前缀（用于从配置中心拉取配置）
	BootstrapConfigPrefix string `yaml:"bootstrap_config_prefix"`
	// Register 自动注册配置（可选）
	Register SdkRegisterConfig `yaml:"register"`
}

// SdkRegisterConfig SDK 自动注册配置
type SdkRegisterConfig struct {
	// ========== 服务配置（EnsureService 使用） ==========
	// Project 项目名称（必填）
	Project string `yaml:"project"`
	// Name 服务名称（必填）
	Name string `yaml:"name"`
	// Owner 服务负责人（必填）
	Owner string `yaml:"owner"`
	// Description 服务描述（可选）
	Description string `yaml:"description"`
	// SpecJSON 服务规格 JSON（可选）
	SpecJSON string `yaml:"spec_json"`

	// ========== 实例配置（RegisterInstance 使用） ==========
	// InstanceKey 实例唯一标识（可选，为空则自动生成）
	InstanceKey string `yaml:"instance_key"`
	// Env 环境（可选，为空则使用全局 env）
	Env string `yaml:"env"`
	// Host 主机地址（可选，为空则使用全局 host）
	Host string `yaml:"host"`
	// Endpoint 服务端点（可选，为空则使用 http://{host}:{port}）
	Endpoint string `yaml:"endpoint"`
	// MetaJSON 元数据 JSON（可选）
	MetaJSON string `yaml:"meta_json"`
	// TTLSeconds TTL 秒数（可选，默认 60）
	TTLSeconds int64 `yaml:"ttl_seconds"`
}
