package authmanager

import (
	"context"
	"time"

	"github.com/xsxdot/aio/pkg/auth"
)

// UserCredential 用户凭证
type UserCredential struct {
	// Username 用户名
	Username string `json:"username"`
	// Password 密码（存储时应为加密后的哈希值）
	Password string `json:"password,omitempty"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// ClientCredential 客户端凭证
type ClientCredential struct {
	// ClientID 客户端ID
	ClientID string `json:"client_id"`
	// Secret 密钥（存储时应为加密后的哈希值）
	Secret string `json:"secret,omitempty"`
	// Type 客户端类型
	Type auth.SubjectType `json:"type"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// User 用户信息
type User struct {
	// ID 用户ID
	ID string `json:"id"`
	// Username 用户名
	Username string `json:"username"`
	// DisplayName 显示名称
	DisplayName string `json:"display_name"`
	// Email 电子邮箱
	Email string `json:"email"`
	// Phone 电话号码
	Phone string `json:"phone,omitempty"`
	// Status 状态：active, locked, disabled
	Status string `json:"status"`
	// LastLoginAt 最后登录时间
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	// Username 用户名
	Username string `json:"username"`
	// Password 密码
	Password string `json:"password"`
	// CaptchaID 验证码ID（如适用）
	CaptchaID string `json:"captcha_id,omitempty"`
	// CaptchaValue 验证码值（如适用）
	CaptchaValue string `json:"captcha_value,omitempty"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	// User 用户信息
	User User `json:"user"`
	// Token 认证令牌
	Token auth.Token `json:"token"`
}

// ClientAuthRequest 客户端认证请求
type ClientAuthRequest struct {
	// ClientID 客户端ID
	ClientID string `json:"client_id"`
	// ClientSecret 客户端密钥（适用于服务类型）
	ClientSecret string `json:"client_secret,omitempty"`
	// Certificate 证书（适用于节点、组件类型）
	Certificate string `json:"certificate,omitempty"`
}

// AuthManagerConfig 认证管理器配置
type AuthManagerConfig struct {
	EnableCert   bool   `json:"enable_cert" yaml:"enable_cert"`
	CertPath     string `json:"certPath" yaml:"cert_path"`
	NodeId       string `json:"node_id" yaml:"node_id"`
	Ip           string `json:"ip" yaml:"ip"`
	ValidityDays int    `json:"validityDays" yaml:"validity_days"`
	// JWTConfig JWT配置
	JWTConfig *auth.AuthJWTConfig `json:"jwt_config" yaml:"jwt_config"`
	// PasswordHashCost 密码哈希成本
	PasswordHashCost int `json:"password_hash_cost" yaml:"password_hash_cost"`
	// InitialAdmin 初始管理员用户
	InitialAdmin *User `json:"initial_admin,omitempty" yaml:"initial_admin"`
}

// StorageProvider 存储提供者接口
type StorageProvider interface {
	// GetUser 获取用户
	GetUser(id string) (*User, error)
	// GetUserByUsername 根据用户名获取用户
	GetUserByUsername(username string) (*User, error)
	// CreateUser 创建用户
	CreateUser(user *User, credential *UserCredential) error
	// UpdateUser 更新用户
	UpdateUser(user *User) error
	// DeleteUser 删除用户
	DeleteUser(id string) error
	// ListUsers 列出用户
	ListUsers() ([]*User, error)

	// GetUserCredential 获取用户凭证
	GetUserCredential(username string) (*UserCredential, error)
	// UpdateUserCredential 更新用户凭证
	UpdateUserCredential(username string, credential *UserCredential) error

	// GetClientCredential 获取客户端凭证
	GetClientCredential(clientID string) (*ClientCredential, error)
	// SaveClientCredential 保存客户端凭证
	SaveClientCredential(credential *ClientCredential) error
	// DeleteClientCredential 删除客户端凭证
	DeleteClientCredential(clientID string) error

	// SaveRole 保存角色
	SaveRole(role *auth.Role) error
	// GetRole 获取角色
	GetRole(id string) (*auth.Role, error)
	// DeleteRole 删除角色
	DeleteRole(id string) error
	// ListRoles 列出角色
	ListRoles() ([]*auth.Role, error)

	// SaveSubject 保存主体
	SaveSubject(subject *auth.Subject) error
	// GetSubject 获取主体
	GetSubject(id string) (*auth.Subject, error)
	// DeleteSubject 删除主体
	DeleteSubject(id string) error
	// ListSubjects 列出主体
	ListSubjects(subjectType auth.SubjectType) ([]*auth.Subject, error)

	// 证书相关
	GetCACertificate() ([]byte, error)
	GetCAPrivateKey() ([]byte, error)
	SaveCACertificate(cert []byte) error
	SaveCAPrivateKey(key []byte) error
	SaveNodeCertificate(nodeID string, cert *auth.NodeCertificate) error
	GetNodeCertificate(nodeID string) (*auth.NodeCertificate, error)

	// WatchCACertificate 监听CA证书变更
	WatchCACertificate(ctx context.Context, handler func(cert []byte, key []byte)) error
}
