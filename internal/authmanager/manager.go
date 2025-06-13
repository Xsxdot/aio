package authmanager

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	mathrand "math/rand"
	"time"

	"github.com/google/uuid"
	auth2 "github.com/xsxdot/aio/pkg/auth"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/lock"
	"go.uber.org/zap"
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
	log        *zap.Logger
	// lockManager 锁管理器
	lockManager lock.LockManager
}

func (m *AuthManager) genConfig() *AuthManagerConfig {
	a := &AuthManagerConfig{
		JWTConfig: &auth2.AuthJWTConfig{
			AccessTokenExpiry: 48 * time.Hour,
			Issuer:            "aio-system",
			Audience:          "aio-api",
		},
		PasswordHashCost: 10,
		InitialAdmin: &User{
			Username:    "admin",
			DisplayName: "超级管理员",
			Phone:       "admin",
		},
	}
	return a
}

func (m *AuthManager) Start(ctx context.Context) error {
	// 初始化JWT配置
	err := m.initializeJWTConfig(ctx)
	if err != nil {
		return fmt.Errorf("initialize JWT config failed: %w", err)
	}

	// 初始化默认管理员账户（如果配置了）
	if m.config.InitialAdmin != nil {
		err := m.initializeAdmin(m.config.InitialAdmin)
		if err != nil {
			return fmt.Errorf("initialize admin failed: %w", err)
		}
	}

	return nil
}

// NewAuthManager 创建认证管理器
func NewAuthManager(storage StorageProvider, lockManager lock.LockManager) (*AuthManager, error) {
	manager := &AuthManager{
		storage:     storage,
		jwtService:  nil,
		log:         common.GetLogger().GetZapLogger("AuthManager"),
		lockManager: lockManager,
	}

	manager.config = manager.genConfig()

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

	if req.ClientSecret != "" {
		// 基于密钥认证（适用于服务类型）
		// 1. 查找客户端凭证
		cred, err := m.storage.GetClientCredential(req.ClientID)
		if err != nil {
			return nil, ErrInvalidCredentials
		}

		// 2. 查找客户端主体信息
		subject, err = m.storage.GetSubject(req.ClientID)
		if err != nil {
			return nil, ErrInvalidCredentials
		}

		// 3. 验证客户端密钥
		err = bcrypt.CompareHashAndPassword([]byte(cred.Secret), []byte(req.ClientSecret))
		if err != nil {
			return nil, ErrInvalidCredentials
		}
	} else {
		return nil, ErrInvalidCredentials
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

// initializeJWTConfig 初始化JWT配置，支持集群环境下的证书同步
func (m *AuthManager) initializeJWTConfig(ctx context.Context) error {
	const (
		jwtPrivateKeyConfigKey = "jwt.private_key_pem"
		jwtPublicKeyConfigKey  = "jwt.public_key_pem"
		jwtLockKey             = "jwt_init_lock"
	)

	// 创建分布式锁，防止多个节点同时生成证书
	lockOpts := lock.DefaultLockOptions()
	lockOpts.TTL = 30 * time.Second // 锁超时时间
	distributedLock := m.lockManager.NewLock(jwtLockKey, lockOpts)

	// 获取锁
	err := distributedLock.LockWithTimeout(ctx, 10*time.Second)
	if err != nil {
		return fmt.Errorf("acquire JWT init lock failed: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		distributedLock.Unlock(unlockCtx)
	}()

	m.log.Info("开始初始化JWT配置")

	// 尝试从存储中获取现有的JWT密钥
	privateKeyPEM, err1 := m.storage.GetConfig(jwtPrivateKeyConfigKey)
	publicKeyPEM, err2 := m.storage.GetConfig(jwtPublicKeyConfigKey)

	var keyPair *auth2.RSAKeyPair

	if err1 != nil || err2 != nil {
		// 密钥不存在，生成新的密钥对
		m.log.Info("未找到现有JWT密钥，正在生成新的密钥对")

		keyPair, err = auth2.GenerateRSAKeyPair(2048)
		if err != nil {
			return fmt.Errorf("generate RSA key pair failed: %w", err)
		}

		// 将密钥保存到存储中
		err = m.storage.SetConfig(jwtPrivateKeyConfigKey, keyPair.PrivateKeyPEM)
		if err != nil {
			return fmt.Errorf("save private key failed: %w", err)
		}

		err = m.storage.SetConfig(jwtPublicKeyConfigKey, keyPair.PublicKeyPEM)
		if err != nil {
			return fmt.Errorf("save public key failed: %w", err)
		}

		m.log.Info("JWT密钥对已生成并保存到存储")
	} else {
		// 从存储中加载现有密钥
		m.log.Info("从存储中加载现有JWT密钥")

		keyPair, err = auth2.LoadRSAKeyPair(privateKeyPEM, publicKeyPEM)
		if err != nil {
			return fmt.Errorf("load RSA key pair from storage failed: %w", err)
		}

		m.log.Info("JWT密钥对已从存储中加载")
	}

	// 更新配置中的密钥对
	m.config.JWTConfig.KeyPair = *keyPair

	// 创建JWT服务
	jwtConfig := auth2.JWTConfig{
		AccessTokenExpiry: m.config.JWTConfig.AccessTokenExpiry,
		Issuer:            m.config.JWTConfig.Issuer,
		Audience:          m.config.JWTConfig.Audience,
	}

	// 从密钥对创建JWT服务
	jwtService := auth2.NewJWTServiceFromKeys(
		keyPair.PrivateKey,
		keyPair.PublicKey,
		jwtConfig,
	)
	m.jwtService = jwtService

	m.log.Info("JWT服务已初始化完成")
	return nil
}
