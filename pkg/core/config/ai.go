package config

// AIConfig AI 服务配置
type AIConfig struct {
	// Providers 供应商配置列表
	Providers []AIProviderConfig `yaml:"providers" mapstructure:"providers"`

	// Router 路由器配置
	Router AIRouterConfig `yaml:"router" mapstructure:"router"`
}

// AIProviderConfig 供应商配置
type AIProviderConfig struct {
	// Name 供应商名称（如 "openai", "dashscope_compat", "dashscope"）
	Name string `yaml:"name" mapstructure:"name"`

	// Type 供应商类型（"openai_compat" 或 "dashscope"）
	Type string `yaml:"type" mapstructure:"type"`

	// BaseURL API 基础 URL
	BaseURL string `yaml:"base_url" mapstructure:"base_url"`

	// APIKey API 密钥
	APIKey string `yaml:"api_key" mapstructure:"api_key"`

	// Organization 组织 ID（可选，OpenAI 专用）
	Organization string `yaml:"organization" mapstructure:"organization"`

	// Timeout 请求超时时间（秒，0 表示使用默认值）
	Timeout int `yaml:"timeout" mapstructure:"timeout"`

	// DefaultModel 默认模型（可选）
	DefaultModel string `yaml:"default_model" mapstructure:"default_model"`
}

// AIRouterConfig 路由器配置
type AIRouterConfig struct {
	// ModelMappings 模型名称到供应商的精确映射
	// 示例：{"qwen3-vl-plus": "dashscope_compat", "gpt-4": "openai"}
	ModelMappings map[string]string `yaml:"model_mappings" mapstructure:"model_mappings"`

	// PrefixMappings 模型前缀到供应商的映射
	// 示例：{"qwen": "dashscope_compat", "gpt": "openai"}
	PrefixMappings map[string]string `yaml:"prefix_mappings" mapstructure:"prefix_mappings"`

	// DefaultProvider 默认供应商
	DefaultProvider string `yaml:"default_provider" mapstructure:"default_provider"`

	// FallbackProviders 降级供应商列表（按优先级排序）
	FallbackProviders []string `yaml:"fallback_providers" mapstructure:"fallback_providers"`

	// EnableAutoRetry 是否启用自动重试
	EnableAutoRetry bool `yaml:"enable_auto_retry" mapstructure:"enable_auto_retry"`

	// MaxRetryAttempts 最大重试次数
	MaxRetryAttempts int `yaml:"max_retry_attempts" mapstructure:"max_retry_attempts"`
}
