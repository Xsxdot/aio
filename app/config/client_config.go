package config

// ClientConfigValue 表示加密类型的值
type ClientConfigValue struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

// ClientConfig 表示返回给客户端的配置
type ClientConfig struct {
	// Key 配置键名
	Key string `json:"key"`
	// Value 配置值内容
	Value map[string]interface{} `json:"value"`
	// Version 配置版本号
	Version int `json:"version"`
	// Metadata 元数据
	Metadata map[string]string `json:"metadata"`
}

// NewClientConfig 创建新的客户端配置
func NewClientConfig(key string, value map[string]interface{}) *ClientConfig {
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
