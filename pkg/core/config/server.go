package config

// ServerConfig 服务器组件配置
type ServerConfig struct {
	Bootstrap []BootstrapServer `yaml:"bootstrap"`
}

// BootstrapServer bootstrap 服务器配置项
type BootstrapServer struct {
	Name             string                  `yaml:"name"`
	Host             string                  `yaml:"host"`
	IntranetHost     string                  `yaml:"intranet_host"`
	ExtranetHost     string                  `yaml:"extranet_host"`
	AgentGrpcAddress string                  `yaml:"agent_grpc_address"`
	Enabled          bool                    `yaml:"enabled"`
	Tags             map[string]string       `yaml:"tags"`
	Comment          string                  `yaml:"comment"`
	SSH              *BootstrapSSHCredential `yaml:"ssh"`
}

// BootstrapSSHCredential bootstrap 服务器 SSH 凭证配置
type BootstrapSSHCredential struct {
	Port           int    `yaml:"port"`             // SSH 端口，默认 22
	Username       string `yaml:"username"`         // SSH 用户名
	AuthMethod     string `yaml:"auth_method"`      // 认证方式(password/privatekey)
	Password       string `yaml:"password"`         // 密码（可选，明文，写入时会加密）
	PrivateKey     string `yaml:"private_key"`      // SSH 私钥内容（可选，YAML 内联）
	PrivateKeyFile string `yaml:"private_key_file"` // SSH 私钥文件路径（可选，优先使用）
	Comment        string `yaml:"comment"`          // 备注
}
