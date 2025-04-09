package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"time"
)

// Role 角色定义
type Role struct {
	// ID 角色唯一标识
	ID string `json:"id" yaml:"id"`
	// Name 角色名称
	Name string `json:"name" yaml:"name"`
	// Description 角色描述
	Description string `json:"description" yaml:"description"`
	// Permissions 权限列表
	Permissions []Permission `json:"permissions" yaml:"permissions"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// Permission 权限定义
type Permission struct {
	// Resource 资源
	Resource string `json:"resource" yaml:"resource"`
	// Action 操作
	Action string `json:"action" yaml:"action"`
	// Conditions 条件
	Conditions map[string]interface{} `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// SubjectType 主体类型（内部使用）
type SubjectType string

const (
	// SubjectTypeService 服务主体
	SubjectTypeService SubjectType = "service"
	// SubjectTypeNode 节点主体
	SubjectTypeNode SubjectType = "node"
	// SubjectTypeComponent 组件主体
	SubjectTypeComponent SubjectType = "component"
	// SubjectTypeComponent 组件主体
	SubjectTypeUser SubjectType = "user"
)

// Subject 权限主体
type Subject struct {
	// ID 主体唯一标识
	ID string `json:"id" yaml:"id"`
	// Type 主体类型
	Type SubjectType `json:"type" yaml:"type"`
	// Name 主体名称
	Name string `json:"name" yaml:"name"`
	// Roles 角色列表
	Roles []string `json:"roles" yaml:"roles"`
	// Disabled 是否禁用
	Disabled bool `json:"disabled" yaml:"disabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// AuthInfo 认证信息
type AuthInfo struct {
	// SubjectID 主体ID
	SubjectID string `json:"subject_id"`
	// SubjectType 主体类型
	SubjectType SubjectType `json:"subject_type"`
	// Name 名称
	Name string `json:"name"`
	// Roles 角色列表
	Roles []string `json:"roles"`
	// Permissions 权限列表
	Permissions []Permission `json:"permissions"`
	// Extra 扩展信息
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// Token 认证令牌
type Token struct {
	// AccessToken 访问令牌
	AccessToken string `json:"access_token"`
	// ExpiresIn 过期时间（秒）
	ExpiresIn int64 `json:"expires_in"`
	// TokenType 令牌类型
	TokenType string `json:"token_type"`
}

// NodeCertificate 节点证书
type NodeCertificate struct {
	// NodeID 节点ID
	NodeID string `json:"node_id" yaml:"node_id"`
	// Certificate 证书
	Certificate *x509.Certificate `json:"-" yaml:"-"`
	// CertificatePEM 证书PEM格式
	CertificatePEM string `json:"certificate_pem" yaml:"certificate_pem"`
	// PrivateKey 私钥
	PrivateKey *rsa.PrivateKey `json:"-" yaml:"-"`
	// PrivateKeyPEM 私钥PEM格式
	PrivateKeyPEM string `json:"private_key_pem" yaml:"private_key_pem"`
	// ExpiresAt 过期时间
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	CertPath  string    // 证书文件路径
	KeyPath   string    // 私钥文件路径
}

// RSAKeyPair RSA密钥对
type RSAKeyPair struct {
	// PrivateKey 私钥
	PrivateKey *rsa.PrivateKey `json:"-" yaml:"-"`
	// PrivateKeyPEM 私钥PEM格式
	PrivateKeyPEM string `json:"private_key_pem" yaml:"private_key_pem"`
	// PublicKey 公钥
	PublicKey *rsa.PublicKey `json:"-" yaml:"-"`
	// PublicKeyPEM 公钥PEM格式
	PublicKeyPEM string `json:"public_key_pem" yaml:"public_key_pem"`
}

// AuthJWTConfig 权限系统JWT配置
type AuthJWTConfig struct {
	PrivateKeyPath string `json:"private_key_path" yaml:"private_key_path"`
	PublicKeyPath  string `json:"public_key_path" yaml:"public_key_path"`
	// KeyPair RSA密钥对
	KeyPair RSAKeyPair `json:"key_pair" yaml:"key_pair"`
	// AccessTokenExpiry 访问令牌过期时间
	AccessTokenExpiry time.Duration `json:"access_token_expiry" yaml:"access_token_expiry"`
	// Issuer 发行者
	Issuer string `json:"issuer" yaml:"issuer"`
	// Audience 受众
	Audience string `json:"audience" yaml:"audience"`
}

// RBACConfig RBAC配置
type RBACConfig struct {
	// DefaultRoles 默认角色
	DefaultRoles []Role `json:"default_roles" yaml:"default_roles"`
	// DefaultSubjects 默认主体
	DefaultSubjects []Subject `json:"default_subjects" yaml:"default_subjects"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	// JWT JWT配置
	JWT AuthJWTConfig `json:"jwt" yaml:"jwt"`
	// RBAC RBAC配置
	RBAC RBACConfig `json:"rbac" yaml:"rbac"`
	// CACertPath CA证书路径
	CACertPath string `json:"ca_cert_path" yaml:"ca_cert_path"`
	// CAKeyPath CA私钥路径
	CAKeyPath string `json:"ca_key_path" yaml:"ca_key_path"`
}

// VerifyRequest 验证请求
type VerifyRequest struct {
	// Token 令牌
	Token string `json:"token" yaml:"token"`
	// Resource 资源
	Resource string `json:"resource" yaml:"resource"`
	// Action 操作
	Action string `json:"action" yaml:"action"`
}

// VerifyResponse 验证响应
type VerifyResponse struct {
	// Allowed 是否允许
	Allowed bool `json:"allowed" yaml:"allowed"`
	// Subject 主体信息
	Subject *Subject `json:"subject,omitempty" yaml:"subject,omitempty"`
	// Reason 原因
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}
