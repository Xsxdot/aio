package config

// ServerConfig 服务器组件配置
type ServerConfig struct {
	Bootstrap []BootstrapServer `yaml:"bootstrap"`
}

// BootstrapServer bootstrap 服务器配置项
type BootstrapServer struct {
	Name             string            `yaml:"name"`
	Host             string            `yaml:"host"`
	AgentGrpcAddress string            `yaml:"agent_grpc_address"`
	Enabled          bool              `yaml:"enabled"`
	Tags             map[string]string `yaml:"tags"`
	Comment          string            `yaml:"comment"`
}


