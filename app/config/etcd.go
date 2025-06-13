package config

// EtcdConfig 代表etcd客户端配置
type EtcdConfig struct {
	// 端点列表，如 ["localhost:2379", "localhost:22379", "localhost:32379"]
	Endpoints string `yaml:"endpoints" json:"endpoints,omitempty"`
	// 拨号超时时间
	DialTimeout int `yaml:"dial_timeout" json:"dial_timeout,omitempty"`
	// 用户名
	Username string `yaml:"username" json:"username,omitempty"`
	// 密码
	Password string `yaml:"password" json:"password,omitempty"`
	// 自动同步端点
	AutoSyncEndpoints bool `yaml:"auto_sync_endpoints" json:"auto_sync_endpoints,omitempty"`
	// 安全连接配置
	TLS *TLSConfig `yaml:"tls" json:"tls,omitempty"`
}

// TLSConfig 表示TLS配置
type TLSConfig struct {
	TLSEnabled bool   `yaml:"tls_enabled" json:"tls_enabled,omitempty"`
	AutoTls    bool   `yaml:"auto_tls" json:"auto_tls,omitempty"`
	Cert       string `yaml:"cert_file" json:"cert,omitempty"`
	Key        string `yaml:"key_file" json:"key,omitempty"`
	TrustedCA  string `yaml:"trusted_ca_file" json:"trusted_ca,omitempty"`
}
