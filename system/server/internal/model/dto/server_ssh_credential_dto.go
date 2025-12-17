package dto

// UpsertServerSSHCredentialRequest 更新或插入 SSH 凭证请求
type UpsertServerSSHCredentialRequest struct {
	Port       int    `json:"port" validate:"required,min=1,max=65535"`
	Username   string `json:"username" validate:"required,max=100"`
	AuthMethod string `json:"authMethod" validate:"required,oneof=password privatekey"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"`
	Comment    string `json:"comment" validate:"max=500"`
}

// ServerSSHCredentialResponse SSH 凭证响应（脱敏）
type ServerSSHCredentialResponse struct {
	ServerID      int64  `json:"serverId"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthMethod    string `json:"authMethod"`
	HasPassword   bool   `json:"hasPassword"`
	HasPrivateKey bool   `json:"hasPrivateKey"`
	Comment       string `json:"comment"`
}
