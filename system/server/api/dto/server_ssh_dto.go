package dto

// ServerSSHConfig 服务器 SSH 配置（用于跨组件调用）
type ServerSSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	AuthMethod string `json:"authMethod"` // password/privatekey
	Password   string `json:"password"`   // 已解密
	PrivateKey string `json:"privateKey"` // 已解密
}

// ServerAgentInfo 服务器 Agent 信息（最小 DTO）
type ServerAgentInfo struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Host             string `json:"host"`
	AgentGrpcAddress string `json:"agentGrpcAddress"`
}
