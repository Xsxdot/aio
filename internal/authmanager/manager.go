package authmanager

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/xsxdot/aio/pkg/common"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	auth2 "github.com/xsxdot/aio/pkg/auth"
	"go.uber.org/zap"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// 初始化
func init() {
	// 初始化随机数种子
	mathrand.Seed(time.Now().UnixNano())
}

var (
	// ErrUserNotFound 用户未找到错误
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidCredentials 凭证无效错误
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserDisabled 用户已禁用错误
	ErrUserDisabled = errors.New("user is disabled")
	// ErrUserLocked 用户已锁定错误
	ErrUserLocked = errors.New("user is locked")
	// ErrSessionExpired 会话已过期错误
	ErrSessionExpired = errors.New("session has expired")
	// ErrInvalidSession 无效会话错误
	ErrInvalidSession = errors.New("invalid session")
	// ErrOperationNotPermitted 操作不允许错误
	ErrOperationNotPermitted = errors.New("operation not permitted")
)

// AuthManager 认证管理器
type AuthManager struct {
	// storage 存储提供者
	storage StorageProvider
	// config 配置
	config *AuthManagerConfig
	// jwtService JWT服务
	jwtService *auth2.JWTService
	// certManager 证书管理器
	certManager *auth2.CertificateManager
	log         *zap.Logger
	status      consts.ComponentStatus
}

func (m *AuthManager) RegisterMetadata() (bool, int, map[string]string) {
	return false, 0, nil
}

func (m *AuthManager) Name() string {
	return consts.ComponentAuthManager
}

func (m *AuthManager) Status() consts.ComponentStatus {
	return m.status
}

func (m *AuthManager) Init(config *config.BaseConfig, body []byte) error {
	a := m.genConfig(config)

	if len(body) > 6 {
		if err := json.Unmarshal(body, a); err != nil {
			return fmt.Errorf("unmarshal config failed: %w", err)
		}
	}

	var keyPair *auth2.RSAKeyPair
	if _, err := os.Stat(a.JWTConfig.PrivateKeyPath); err != nil {
		keyPair, _ = auth2.GenerateAndSaveRSAKeyPair(2048, a.JWTConfig.PrivateKeyPath, a.JWTConfig.PublicKeyPath)
	} else {
		keyPair, _ = auth2.LoadRSAKeyPairFromFiles(a.JWTConfig.PrivateKeyPath, a.JWTConfig.PublicKeyPath)
	}
	a.JWTConfig.KeyPair = *keyPair

	m.config = a
	m.status = consts.StatusInitialized
	return nil
}

func (m *AuthManager) genConfig(config *config.BaseConfig) *AuthManagerConfig {
	certPath := filepath.Join(config.System.DataDir, "cert")
	a := &AuthManagerConfig{
		EnableCert:   true,
		CertPath:     certPath,
		NodeId:       config.System.NodeId,
		Ip:           config.Network.LocalIp,
		ValidityDays: 365,
		JWTConfig: auth2.AuthJWTConfig{
			PrivateKeyPath:    filepath.Join(certPath, "jwt.key"),
			PublicKeyPath:     filepath.Join(certPath, "jwt.key.pub"),
			AccessTokenExpiry: 48 * time.Hour,
			Issuer:            "aio-system",
			Audience:          "aio-api",
		},
		PasswordHashCost: 10,
		InitialAdmin: &User{
			Username:    "admin",
			DisplayName: "超级管理员",
			Phone:       "123456789",
		},
	}
	return a
}

func (m *AuthManager) Start(ctx context.Context) error {
	// 从etcd加载或创建CA证书
	if err := m.initializeCertificates(ctx); err != nil {
		return fmt.Errorf("initialize certificates failed: %w", err)
	}

	// 启动CA证书监听
	go m.watchCACertificate(ctx)

	// 启动客户端证书监听
	// go m.watchClientCertificate(ctx)

	// 初始化默认管理员账户（如果配置了）
	if m.config.InitialAdmin != nil {
		err := m.initializeAdmin(m.config.InitialAdmin)
		if err != nil {
			return fmt.Errorf("initialize admin failed: %w", err)
		}
	}

	// 创建JWT服务
	jwtConfig := auth2.JWTConfig{
		AccessTokenExpiry: m.config.JWTConfig.AccessTokenExpiry,
		Issuer:            m.config.JWTConfig.Issuer,
		Audience:          m.config.JWTConfig.Audience,
	}

	// 从配置中获取密钥对
	jwtService := auth2.NewJWTServiceFromKeys(
		m.config.JWTConfig.KeyPair.PrivateKey,
		m.config.JWTConfig.KeyPair.PublicKey,
		jwtConfig,
	)
	m.jwtService = jwtService

	m.status = consts.StatusRunning

	return nil
}

func (m *AuthManager) Restart(ctx context.Context) error {
	return m.Start(ctx)
}

func (m *AuthManager) Stop(ctx context.Context) error {
	m.status = consts.StatusStopped
	return nil
}

// DefaultConfig 返回组件的默认配置
func (m *AuthManager) DefaultConfig(config *config.BaseConfig) interface{} {
	return m.genConfig(config)
}

// NewAuthManager 创建认证管理器
func NewAuthManager(storage StorageProvider) (*AuthManager, error) {
	manager := &AuthManager{
		storage: storage,
		log:     common.GetLogger().GetZapLogger(consts.ComponentAuthManager),
	}

	return manager, nil
}

// initializeAdmin 初始化管理员账户
func (m *AuthManager) initializeAdmin(admin *User) error {
	// 检查管理员是否已存在
	existing, err := m.storage.GetUserByUsername(admin.Username)
	if err == nil && existing != nil {
		// 管理员已存在，不需要创建
		return nil
	}

	// 确保管理员有ID
	if admin.ID == "" {
		admin.ID = uuid.New().String()
	}

	// 设置创建和更新时间
	now := time.Now()
	admin.CreatedAt = now
	admin.UpdatedAt = now
	admin.Status = "active"

	// 创建管理员账户
	credential := &UserCredential{
		Username:  admin.Username,
		Password:  "admin", // 初始密码，应该在创建后立即更改
		UpdatedAt: now,
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(credential.Password), m.config.PasswordHashCost)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}
	credential.Password = string(hashedPassword)

	// 保存管理员账户
	err = m.storage.CreateUser(admin, credential)
	if err != nil {
		return fmt.Errorf("create admin account failed: %w", err)
	}

	// 创建管理员角色（如果尚不存在）
	adminRole, err := m.storage.GetRole("admin")
	if err != nil || adminRole == nil {
		adminRole = &auth2.Role{
			ID:          "admin",
			Name:        "Administrator",
			Description: "System administrator with full permissions",
			Permissions: []auth2.Permission{
				{Resource: "*", Action: "*"}, // 所有权限
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		err = m.storage.SaveRole(adminRole)
		if err != nil {
			return fmt.Errorf("create admin role failed: %w", err)
		}
	}

	// 创建管理员主体并关联角色
	adminSubject := &auth2.Subject{
		ID:        admin.ID,
		Type:      auth2.SubjectTypeUser,
		Name:      admin.Username,
		Roles:     []string{"admin"},
		Disabled:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = m.storage.SaveSubject(adminSubject)
	if err != nil {
		return fmt.Errorf("create admin subject failed: %w", err)
	}

	return nil
}

// Login 用户登录
func (m *AuthManager) Login(req LoginRequest, ip, userAgent string) (*LoginResponse, error) {
	// 获取用户
	user, err := m.storage.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	// 检查用户状态
	if user.Status == "disabled" {
		return nil, ErrUserDisabled
	}
	if user.Status == "locked" {
		return nil, ErrUserLocked
	}

	// 获取用户凭证
	cred, err := m.storage.GetUserCredential(req.Username)
	if err != nil {
		return nil, fmt.Errorf("get user credential failed: %w", err)
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(cred.Password), []byte(req.Password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// 获取用户权限主体
	subject, err := m.storage.GetSubject(user.ID)
	if err != nil {
		return nil, fmt.Errorf("get user subject failed: %w", err)
	}

	// 获取用户权限
	roles, permissions, err := m.getUserPermissions(subject)
	if err != nil {
		return nil, fmt.Errorf("get user permissions failed: %w", err)
	}

	// 生成JWT令牌
	authInfo := &auth2.AuthInfo{
		SubjectID:   user.ID,
		SubjectType: auth2.SubjectTypeUser,
		Name:        user.Username,
		Roles:       roles,
		Permissions: permissions,
		Extra: map[string]interface{}{
			"phone":        user.Phone,
			"display_name": user.DisplayName,
		},
	}

	token, err := m.jwtService.GenerateToken(*authInfo)
	if err != nil {
		return nil, fmt.Errorf("generate token failed: %w", err)
	}

	// 更新最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	err = m.storage.UpdateUser(user)
	if err != nil {
		// 不要因为更新最后登录时间失败而阻止登录
		// 只记录错误
		fmt.Printf("update last login time failed: %v\n", err)
	}

	// 返回登录响应
	return &LoginResponse{
		User:  *user,
		Token: *token,
	}, nil
}

// AuthenticateClient 客户端认证
func (m *AuthManager) AuthenticateClient(req ClientAuthRequest) (*auth2.Token, error) {
	var subject *auth2.Subject
	var err error

	// 根据认证方式获取主体
	if req.ClientSecret != "" {
		// 基于客户端密钥认证（适用于服务类型）
		// 1. 获取客户端主体
		subject, err = m.storage.GetSubject(req.ClientID)
		if err != nil {
			return nil, ErrInvalidCredentials
		}

		// 2. 获取客户端凭证
		cred, err := m.storage.GetClientCredential(req.ClientID)
		if err != nil {
			return nil, ErrInvalidCredentials
		}

		// 3. 验证客户端密钥
		err = bcrypt.CompareHashAndPassword([]byte(cred.Secret), []byte(req.ClientSecret))
		if err != nil {
			return nil, ErrInvalidCredentials
		}
	} else if req.Certificate != "" {
		// 基于证书认证（适用于节点、组件类型）
		// 这里只是为了未来扩展，暂时返回错误
		// TODO: 实现基于证书的认证
		// 这里应该验证证书的有效性，以及证书中的主题与请求的ClientID是否匹配
		return nil, fmt.Errorf("certificate authentication not implemented yet")
	}

	if subject == nil {
		return nil, ErrInvalidCredentials
	}

	if subject.Disabled {
		return nil, ErrUserDisabled
	}

	// 获取主体权限
	roles, permissions, err := m.getUserPermissions(subject)
	//todo 现在还不需要验证这个
	//if err != nil {
	//	return nil, fmt.Errorf("get client permissions failed: %w", err)
	//}

	// 生成JWT令牌
	authInfo := &auth2.AuthInfo{
		SubjectID:   subject.ID,
		SubjectType: subject.Type,
		Name:        subject.Name,
		Roles:       roles,
		Permissions: permissions,
	}

	token, err := m.jwtService.GenerateToken(*authInfo)
	if err != nil {
		return nil, fmt.Errorf("generate token failed: %w", err)
	}

	return token, nil
}

// VerifyPermission 验证权限
func (m *AuthManager) VerifyPermission(token string, resource, action string) (*auth2.VerifyResponse, error) {
	// 解析令牌
	authInfo, err := m.jwtService.ValidateToken(token)
	if err != nil {
		return &auth2.VerifyResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("invalid token: %v", err),
		}, nil
	}

	// 获取主体
	subject, err := m.storage.GetSubject(authInfo.SubjectID)
	if err != nil {
		return &auth2.VerifyResponse{
			Allowed: false,
			Reason:  "subject not found",
		}, nil
	}

	if subject.Disabled {
		return &auth2.VerifyResponse{
			Allowed: false,
			Subject: subject,
			Reason:  "subject is disabled",
		}, nil
	}

	// 检查权限
	for _, perm := range authInfo.Permissions {
		if m.matchPermission(perm, resource, action) {
			return &auth2.VerifyResponse{
				Allowed: true,
				Subject: subject,
			}, nil
		}
	}

	return &auth2.VerifyResponse{
		Allowed: false,
		Subject: subject,
		Reason:  "permission denied",
	}, nil
}

// matchPermission 匹配权限
func (m *AuthManager) matchPermission(perm auth2.Permission, resource, action string) bool {
	// 通配符匹配所有
	if perm.Resource == "*" && perm.Action == "*" {
		return true
	}

	// 资源通配符
	if perm.Resource == "*" && perm.Action == action {
		return true
	}

	// 操作通配符
	if perm.Resource == resource && perm.Action == "*" {
		return true
	}

	// 精确匹配
	return perm.Resource == resource && perm.Action == action
}

// getUserPermissions 获取用户权限
func (m *AuthManager) getUserPermissions(subject *auth2.Subject) ([]string, []auth2.Permission, error) {
	roleIDs := subject.Roles
	var allPermissions []auth2.Permission

	for _, roleID := range roleIDs {
		role, err := m.storage.GetRole(roleID)
		if err != nil {
			return nil, nil, fmt.Errorf("get role %s failed: %w", roleID, err)
		}

		if role != nil {
			allPermissions = append(allPermissions, role.Permissions...)
		}
	}

	return roleIDs, allPermissions, nil
}

// CreateUser 创建用户
func (m *AuthManager) CreateUser(user *User, password string, roles []string) error {
	// 确保用户有ID
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	// 设置创建和更新时间
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	if user.Status == "" {
		user.Status = "active"
	}

	// 创建用户凭证
	credential := &UserCredential{
		Username:  user.Username,
		UpdatedAt: now,
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), m.config.PasswordHashCost)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}
	credential.Password = string(hashedPassword)

	// 保存用户
	err = m.storage.CreateUser(user, credential)
	if err != nil {
		return fmt.Errorf("create user failed: %w", err)
	}

	// 创建用户主体并关联角色
	userSubject := &auth2.Subject{
		ID:        user.ID,
		Type:      auth2.SubjectTypeUser,
		Name:      user.Username,
		Roles:     roles,
		Disabled:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = m.storage.SaveSubject(userSubject)
	if err != nil {
		// 如果创建主体失败，尝试回滚用户创建
		_ = m.storage.DeleteUser(user.ID)
		return fmt.Errorf("create user subject failed: %w", err)
	}

	return nil
}

// UpdateUserPassword 更新用户密码
func (m *AuthManager) UpdateUserPassword(userID, oldPassword, newPassword string) error {
	// 获取用户
	user, err := m.storage.GetUser(userID)
	if err != nil {
		return ErrUserNotFound
	}

	// 获取用户凭证
	cred, err := m.storage.GetUserCredential(user.Username)
	if err != nil {
		return fmt.Errorf("get user credential failed: %w", err)
	}

	// 验证旧密码
	err = bcrypt.CompareHashAndPassword([]byte(cred.Password), []byte(oldPassword))
	if err != nil {
		return ErrInvalidCredentials
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), m.config.PasswordHashCost)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}

	// 更新凭证
	cred.Password = string(hashedPassword)
	cred.UpdatedAt = time.Now()
	return m.storage.UpdateUserCredential(user.Username, cred)
}

// GetUser 获取用户
func (m *AuthManager) GetUser(id string) (*User, error) {
	return m.storage.GetUser(id)
}

// UpdateUser 更新用户
func (m *AuthManager) UpdateUser(user *User) error {
	user.UpdatedAt = time.Now()
	return m.storage.UpdateUser(user)
}

// ListUsers 列出用户
func (m *AuthManager) ListUsers() ([]*User, error) {
	return m.storage.ListUsers()
}

// SaveRole 保存角色
func (m *AuthManager) SaveRole(role *auth2.Role) error {
	// 设置创建和更新时间
	now := time.Now()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = now
	}
	role.UpdatedAt = now

	return m.storage.SaveRole(role)
}

// GetRole 获取角色
func (m *AuthManager) GetRole(id string) (*auth2.Role, error) {
	return m.storage.GetRole(id)
}

// DeleteRole 删除角色
func (m *AuthManager) DeleteRole(id string) error {
	return m.storage.DeleteRole(id)
}

// ListRoles 列出角色
func (m *AuthManager) ListRoles() ([]*auth2.Role, error) {
	return m.storage.ListRoles()
}

// CreateClient 创建客户端凭证
// 如果clientSecret为空，则自动生成随机密钥
// 返回值: (实际使用的密钥, error)
func (m *AuthManager) CreateClient(clientID string, clientSecret string, clientType auth2.SubjectType, roles []string, clientName string) (string, error) {
	// 1. 创建客户端凭证
	now := time.Now()

	// 检查客户端ID是否已存在
	_, err := m.storage.GetSubject(clientID)
	if err == nil {
		return "", fmt.Errorf("client with ID %s already exists", clientID)
	}

	// 如果密钥为空，则生成随机密钥
	if clientSecret == "" {
		clientSecret = generateRandomSecret(32)
	}

	// 哈希客户端密钥
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), m.config.PasswordHashCost)
	if err != nil {
		return "", fmt.Errorf("hash client secret failed: %w", err)
	}

	// 创建客户端凭证
	clientCred := &ClientCredential{
		ClientID:  clientID,
		Secret:    string(hashedSecret),
		Type:      clientType,
		UpdatedAt: now,
	}

	// 保存客户端凭证
	err = m.storage.SaveClientCredential(clientCred)
	if err != nil {
		return "", fmt.Errorf("save client credential failed: %w", err)
	}

	// 2. 创建权限主体
	subject := &auth2.Subject{
		ID:        clientID,
		Type:      clientType,
		Name:      clientName,
		Roles:     roles,
		Disabled:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 保存主体
	err = m.storage.SaveSubject(subject)
	if err != nil {
		// 如果创建主体失败，回滚客户端凭证创建
		_ = m.storage.DeleteClientCredential(clientID)
		return "", fmt.Errorf("create client subject failed: %w", err)
	}

	return clientSecret, nil
}

// DeleteClient 删除客户端
func (m *AuthManager) DeleteClient(clientID string) error {
	// 1. 删除客户端凭证
	err := m.storage.DeleteClientCredential(clientID)
	if err != nil {
		return fmt.Errorf("delete client credential failed: %w", err)
	}

	// 2. 删除权限主体
	err = m.storage.DeleteSubject(clientID)
	if err != nil {
		return fmt.Errorf("delete client subject failed: %w", err)
	}

	return nil
}

// CreateClientWithRandomSecret 使用随机生成的密钥创建客户端
// 简化CreateClient的调用，自动生成随机密钥
func (m *AuthManager) CreateClientWithRandomSecret(clientID string, clientType auth2.SubjectType, roles []string, clientName string) (string, error) {
	return m.CreateClient(clientID, "", clientType, roles, clientName)
}

// CreateServiceClient 创建服务类型客户端
// serviceName: 服务名称
// roles: 分配的角色
// serviceDesc: 服务描述（可选）
func (m *AuthManager) CreateServiceClient(serviceName string, roles []string, serviceDesc string) (string, string, error) {
	// 1. 生成唯一的客户端ID
	clientID := uuid.New().String()

	// 2. 设置客户端名称
	clientName := serviceName
	if serviceDesc != "" {
		clientName = fmt.Sprintf("%s (%s)", serviceName, serviceDesc)
	}

	// 3. 创建客户端（使用随机密钥）
	clientSecret, err := m.CreateClientWithRandomSecret(clientID, auth2.SubjectTypeService, roles, clientName)
	if err != nil {
		return "", "", err
	}

	return clientID, clientSecret, nil
}

// generateRandomSecret 生成随机密钥
func generateRandomSecret(length int) string {
	// 生成随机字节
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// 如果出错则回退到不太安全的方法
		return generateFallbackSecret(length)
	}

	// 编码为base64
	return base64.URLEncoding.EncodeToString(randomBytes)[:length]
}

// generateFallbackSecret 备用的随机密钥生成方法（不太安全）
func generateFallbackSecret(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[mathrand.Intn(len(chars))]
	}
	return string(result)
}

// initializeCertificates 初始化证书
// 检查etcd中是否已有CA证书和私钥
// etcd中已有CA证书和私钥，保存到本地
func (m *AuthManager) initializeCertificates(ctx context.Context) error {
	caCertFile := filepath.Join(m.config.CertPath, "ca.crt")
	caKeyFile := filepath.Join(m.config.CertPath, "ca.key")

	caCert, err := m.storage.GetCACertificate()
	caKey, err2 := m.storage.GetCAPrivateKey()

	if err == nil && err2 == nil {
		if err := os.WriteFile(caCertFile, caCert, 0644); err != nil {
			return fmt.Errorf("write CA certificate to file failed: %w", err)
		}
		if err := os.WriteFile(caKeyFile, caKey, 0600); err != nil {
			return fmt.Errorf("write CA private key to file failed: %w", err)
		}
		m.log.Info("saved CA certificate and private key from etcd to local files")
		manager, err := auth2.NewCertificateManager(caCertFile, caKeyFile, m.config.CertPath)
		if err != nil {
			return fmt.Errorf("create certificate manager failed: %w", err)
		}
		m.certManager = manager

		certificate, err := m.storage.GetNodeCertificate("client")
		if err != nil {
			return err
		}

		clientCrt := filepath.Join(m.config.CertPath, "client.crt")
		err = os.WriteFile(clientCrt, []byte(certificate.CertificatePEM), 0644)
		if err != nil {
			return fmt.Errorf("write client certificate to file failed: %w", err)
		}
		clientKey := filepath.Join(m.config.CertPath, "client.key")
		err = os.WriteFile(clientKey, []byte(certificate.PrivateKeyPEM), 0600)
		if err != nil {
			return fmt.Errorf("write client private key to file failed: %w", err)
		}

	} else {
		manager, err := auth2.NewCertificateManager("", "", m.config.CertPath)
		if err != nil {
			return fmt.Errorf("create certificate manager failed: %w", err)
		}
		m.certManager = manager
	}

	// 检查是否已有节点证书
	nodeID := m.config.NodeId
	err = m.certManager.LoadAllCertificates()
	if err != nil {
		return err
	}

	_, err = m.certManager.GetNodeCertificate(nodeID)
	if err != nil {
		// 如果没有节点证书，创建新的
		m.log.Info("creating new node certificate", zap.String("nodeID", nodeID))
		_, err = m.certManager.GenerateNodeCertificate(nodeID, []string{m.config.Ip}, m.config.ValidityDays)
		if err != nil {
			return fmt.Errorf("generate node certificate failed: %w", err)
		}

	}

	client := "client"
	clientCert, err := m.certManager.GetNodeCertificate(client)
	if err != nil {
		// 如果没有节点证书，创建新的
		m.log.Info("creating new node certificate", zap.String("nodeID", client))
		clientCert, err = m.certManager.GenerateNodeCertificate(client, []string{}, m.config.ValidityDays)
		if err != nil {
			return fmt.Errorf("generate node certificate failed: %w", err)
		}

		// 保存节点证书到etcd
		if err := m.storage.SaveNodeCertificate(client, clientCert); err != nil {
			return fmt.Errorf("save node certificate to etcd failed: %w", err)
		}
		m.log.Info("saved client certificate to etcd", zap.String("nodeID", client))
	}

	return nil
}

// watchCACertificate 监听CA证书变动
func (m *AuthManager) watchCACertificate(ctx context.Context) {
	// 使用存储提供者的监听方法
	err := m.storage.WatchCACertificate(ctx, func(caCert []byte, caKey []byte) {
		// 保存到本地
		caCertPath := filepath.Join(m.config.CertPath, "ca.crt")
		caKeyPath := filepath.Join(m.config.CertPath, "ca.key")

		if err := os.WriteFile(caCertPath, caCert, 0644); err != nil {
			m.log.Error("write CA certificate to file failed", zap.Error(err))
			return
		}
		if err := os.WriteFile(caKeyPath, caKey, 0600); err != nil {
			m.log.Error("write CA private key to file failed", zap.Error(err))
			return
		}

		m.log.Info("updated local CA certificate and private key from etcd")
	})

	if err != nil {
		m.log.Error("watch CA certificate failed", zap.Error(err))
	}
}

// // watchClientCertificate 监听Client证书变动
// func (m *AuthManager) watchClientCertificate(ctx context.Context) {
// 	clientID := "client"
// 	// 使用存储提供者的监听方法
// 	err := m.storage.WatchNodeCertificate(ctx, clientID, func(certificate *auth2.NodeCertificate) {
// 		// 保存到本地
// 		clientCrtPath := filepath.Join(m.config.CertPath, "client.crt")
// 		clientKeyPath := filepath.Join(m.config.CertPath, "client.key")

// 		if err := os.WriteFile(clientCrtPath, []byte(certificate.CertificatePEM), 0644); err != nil {
// 			m.log.Error("写入客户端证书到文件失败", zap.Error(err))
// 			return
// 		}
// 		if err := os.WriteFile(clientKeyPath, []byte(certificate.PrivateKeyPEM), 0600); err != nil {
// 			m.log.Error("写入客户端私钥到文件失败", zap.Error(err))
// 			return
// 		}

// 		m.certManager.LoadAllCertificates()

// 		m.log.Info("从etcd更新了本地客户端证书和私钥", zap.String("clientID", clientID))
// 	})

// 	if err != nil {
// 		m.log.Error("监听客户端证书失败", zap.Error(err))
// 	}
// }

// GetCACertPath 获取CA证书文件路径
func (m *AuthManager) GetCACertPath() string {
	return m.certManager.GetCAFilePath()
}

// GetNodeCertPath 获取节点证书文件路径
func (m *AuthManager) GetNodeCertPath(nodeID string) (string, error) {
	cert, err := m.certManager.GetNodeCertificate(nodeID)
	if err != nil {
		return "", err
	}
	return cert.CertPath, nil
}

// GetNodeKeyPath 获取节点私钥文件路径
func (m *AuthManager) GetNodeKeyPath(nodeID string) (string, error) {
	cert, err := m.certManager.GetNodeCertificate(nodeID)
	if err != nil {
		return "", err
	}
	return cert.KeyPath, nil
}

// GetClientConfig 实现Component接口，返回客户端配置
func (m *AuthManager) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}
