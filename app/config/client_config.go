package config

// ClientConfigValue 表示加密类型的值
type ClientConfigValue struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

// ClientConfigFixedValue 表示客户端配置固定字段的值结构
type ClientConfigFixedValue struct {
	// 用户名
	Username string `json:"username,omitempty"`
	// 密码 (可能是加密的)
	Password string `json:"password,omitempty"`
	// 是否启用TLS
	EnableTls bool `json:"enable_tls,omitempty"`
	// 客户端证书路径
	Cert string `json:"cert,omitempty"`
	// 客户端密钥路径
	Key string `json:"key,omitempty"`
	// 可信CA证书路径
	TrustedCAFile string `json:"trusted_ca_file,omitempty"`
	// 客户端证书内容
	CertContent string `json:"cert_content,omitempty"`
	// 客户端密钥内容
	KeyContent string `json:"key_content,omitempty"`
	// 可信CA证书内容
	CATrustedContent string `json:"ca_trusted_content,omitempty"`
}

// ClientConfig 表示返回给客户端的配置
type ClientConfig struct {
	// Key 配置键名
	Key string `json:"key"`
	// Value 配置值内容
	Value ClientConfigFixedValue `json:"value"`
	// Version 配置版本号
	Version int `json:"version"`
	// Metadata 元数据
	Metadata map[string]string `json:"metadata"`
}

// NewClientConfig 创建新的客户端配置
func NewClientConfig(key string, value ClientConfigFixedValue) *ClientConfig {
	return &ClientConfig{
		Key:      key,
		Value:    value,
		Version:  1,
		Metadata: make(map[string]string),
	}
}

// WithVersion 设置版本号
func (c *ClientConfig) WithVersion(version int) *ClientConfig {
	c.Version = version
	return c
}

// WithMetadata 设置元数据
func (c *ClientConfig) WithMetadata(metadata map[string]string) *ClientConfig {
	c.Metadata = metadata
	return c
}

// NewEncryptedValue 创建一个加密类型的值
func NewEncryptedValue(value string) ClientConfigValue {
	return ClientConfigValue{
		Value: value,
		Type:  "encrypted",
	}
}
