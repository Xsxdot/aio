// Package credential 提供独立的密钥管理功能
package credential

import (
	"context"
	"time"
)

// CredentialType 密钥类型
type CredentialType string

const (
	CredentialTypeSSHKey   CredentialType = "ssh_key"  // SSH私钥
	CredentialTypePassword CredentialType = "password" // 用户名密码
	CredentialTypeToken    CredentialType = "token"    // Token类型
)

// Credential 密钥配置
type Credential struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        CredentialType `json:"type"`
	Content     string         `json:"content"` // 加密存储的内容
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by"`
}

// CredentialSafe 安全的密钥信息（不包含敏感内容）
type CredentialSafe struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        CredentialType `json:"type"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by"`
	HasContent  bool           `json:"has_content"` // 是否已设置内容
}

// CredentialCreateRequest 创建密钥请求
type CredentialCreateRequest struct {
	Name        string         `json:"name" validate:"required"`
	Description string         `json:"description"`
	Type        CredentialType `json:"type" validate:"required"`
	Content     string         `json:"content" validate:"required"`
}

// CredentialUpdateRequest 更新密钥请求
type CredentialUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"` // 如果为空则不更新内容
}

// CredentialListRequest 密钥列表查询请求
type CredentialListRequest struct {
	Type   CredentialType `json:"type"`   // 按类型筛选
	Limit  int            `json:"limit"`  // 分页大小
	Offset int            `json:"offset"` // 分页偏移
}

// SSHKeyInfo SSH密钥信息
type SSHKeyInfo struct {
	Type        string `json:"type"`        // 密钥类型 (rsa, ed25519, etc.)
	Fingerprint string `json:"fingerprint"` // 密钥指纹
	Comment     string `json:"comment"`     // 密钥注释
	KeySize     int    `json:"key_size"`    // 密钥长度
}

// PasswordInfo 密码信息
type PasswordInfo struct {
	Username string `json:"username"` // 用户名
	// 密码不在此结构中返回
}

// TokenInfo Token信息
type TokenInfo struct {
	TokenType string    `json:"token_type"` // Token类型
	ExpiresAt time.Time `json:"expires_at"` // 过期时间
	Scopes    []string  `json:"scopes"`     // 权限范围
}

// CredentialTestRequest 测试密钥请求
type CredentialTestRequest struct {
	Host     string `json:"host" validate:"required"`
	Port     int    `json:"port" validate:"required,min=1,max=65535"`
	Username string `json:"username" validate:"required"`
}

// CredentialTestResult 密钥测试结果
type CredentialTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"` // 连接延迟(毫秒)
}

// Storage 密钥存储接口
type Storage interface {
	// 密钥管理
	CreateCredential(ctx context.Context, credential *Credential) error
	GetCredential(ctx context.Context, id string) (*Credential, error)
	GetCredentialSafe(ctx context.Context, id string) (*CredentialSafe, error)
	UpdateCredential(ctx context.Context, credential *Credential) error
	DeleteCredential(ctx context.Context, id string) error
	ListCredentials(ctx context.Context, req *CredentialListRequest) ([]*CredentialSafe, int, error)

	// 密钥查询
	GetCredentialsByType(ctx context.Context, credType CredentialType) ([]*CredentialSafe, error)
	GetCredentialsByUser(ctx context.Context, userID string) ([]*CredentialSafe, error)

	// 安全相关
	EncryptContent(content string) (string, error)
	DecryptContent(encryptedContent string) (string, error)
}
