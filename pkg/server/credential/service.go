// Package credential 密钥管理服务
package credential

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// Service 密钥管理服务接口
type Service interface {
	// 密钥管理
	CreateCredential(ctx context.Context, req *CredentialCreateRequest) (*CredentialSafe, error)
	GetCredential(ctx context.Context, id string) (*CredentialSafe, error)
	UpdateCredential(ctx context.Context, id string, req *CredentialUpdateRequest) (*CredentialSafe, error)
	DeleteCredential(ctx context.Context, id string) error
	ListCredentials(ctx context.Context, req *CredentialListRequest) ([]*CredentialSafe, int, error)

	// 密钥测试
	TestCredential(ctx context.Context, id string, req *CredentialTestRequest) (*CredentialTestResult, error)

	// 获取密钥内容（供其他组件使用）
	GetCredentialContent(ctx context.Context, id string) (string, CredentialType, error)

	// 分析密钥信息
	AnalyzeSSHKey(ctx context.Context, id string) (*SSHKeyInfo, error)
}

// ServiceImpl 密钥管理服务实现
type ServiceImpl struct {
	storage Storage
	logger  *zap.Logger
}

// Config 服务配置
type Config struct {
	EtcdClient *etcd.EtcdClient
	Logger     *zap.Logger
}

// NewService 创建密钥管理服务
func NewService(config Config) (Service, error) {
	if config.Logger == nil {
		config.Logger, _ = zap.NewProduction()
	}
	storage, err := NewETCDStorage(ETCDStorageConfig{
		Client: config.EtcdClient,
		Logger: config.Logger,
	})
	if err != nil {
		return nil, err
	}
	return &ServiceImpl{
		storage: storage,
		logger:  config.Logger,
	}, nil
}

// CreateCredential 创建密钥
func (s *ServiceImpl) CreateCredential(ctx context.Context, req *CredentialCreateRequest) (*CredentialSafe, error) {
	// 验证必填字段
	if req.Name == "" {
		return nil, fmt.Errorf("密钥名称不能为空")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("密钥类型不能为空")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("密钥内容不能为空")
	}

	// 验证密钥内容格式
	if err := s.validateCredentialContent(req.Type, req.Content); err != nil {
		return nil, fmt.Errorf("密钥内容格式验证失败: %w", err)
	}

	// 检查密钥名称是否重复
	listReq := &CredentialListRequest{Limit: 100, Offset: 0}
	existingCredentials, _, err := s.storage.ListCredentials(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("检查密钥名称失败: %w", err)
	}

	for _, credential := range existingCredentials {
		if credential.Name == req.Name {
			return nil, fmt.Errorf("密钥名称 '%s' 已存在", req.Name)
		}
	}

	// 加密密钥内容
	encryptedContent, err := s.storage.EncryptContent(req.Content)
	if err != nil {
		return nil, fmt.Errorf("加密密钥内容失败: %w", err)
	}

	// 创建密钥对象
	now := time.Now()
	credential := &Credential{
		ID:          s.generateCredentialID(req.Name),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Content:     encryptedContent,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "system", // TODO: 从上下文获取用户信息
	}

	// 保存到存储
	if err := s.storage.CreateCredential(ctx, credential); err != nil {
		s.logger.Error("创建密钥失败",
			zap.String("name", req.Name),
			zap.Error(err))
		return nil, fmt.Errorf("创建密钥失败: %w", err)
	}

	s.logger.Info("创建密钥成功",
		zap.String("id", credential.ID),
		zap.String("name", credential.Name))

	// 返回安全的密钥信息
	return &CredentialSafe{
		ID:          credential.ID,
		Name:        credential.Name,
		Description: credential.Description,
		Type:        credential.Type,
		CreatedAt:   credential.CreatedAt,
		UpdatedAt:   credential.UpdatedAt,
		CreatedBy:   credential.CreatedBy,
		HasContent:  true,
	}, nil
}

// GetCredential 获取密钥
func (s *ServiceImpl) GetCredential(ctx context.Context, id string) (*CredentialSafe, error) {
	if id == "" {
		return nil, fmt.Errorf("密钥ID不能为空")
	}

	credential, err := s.storage.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取密钥失败: %w", err)
	}

	// 返回安全的密钥信息
	return &CredentialSafe{
		ID:          credential.ID,
		Name:        credential.Name,
		Description: credential.Description,
		Type:        credential.Type,
		CreatedAt:   credential.CreatedAt,
		UpdatedAt:   credential.UpdatedAt,
		CreatedBy:   credential.CreatedBy,
		HasContent:  credential.Content != "",
	}, nil
}

// UpdateCredential 更新密钥
func (s *ServiceImpl) UpdateCredential(ctx context.Context, id string, req *CredentialUpdateRequest) (*CredentialSafe, error) {
	if id == "" {
		return nil, fmt.Errorf("密钥ID不能为空")
	}

	// 获取现有密钥
	credential, err := s.storage.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("密钥不存在: %w", err)
	}

	// 更新字段
	if req.Name != "" {
		// 检查名称是否重复
		listReq := &CredentialListRequest{Limit: 100, Offset: 0}
		existingCredentials, _, err := s.storage.ListCredentials(ctx, listReq)
		if err != nil {
			return nil, fmt.Errorf("检查密钥名称失败: %w", err)
		}

		for _, existingCredential := range existingCredentials {
			if existingCredential.ID != id && existingCredential.Name == req.Name {
				return nil, fmt.Errorf("密钥名称 '%s' 已存在", req.Name)
			}
		}
		credential.Name = req.Name
	}

	if req.Description != "" {
		credential.Description = req.Description
	}

	if req.Content != "" {
		// 验证新的密钥内容格式
		if err := s.validateCredentialContent(credential.Type, req.Content); err != nil {
			return nil, fmt.Errorf("密钥内容格式验证失败: %w", err)
		}

		// 加密新的密钥内容
		encryptedContent, err := s.storage.EncryptContent(req.Content)
		if err != nil {
			return nil, fmt.Errorf("加密密钥内容失败: %w", err)
		}
		credential.Content = encryptedContent
	}

	credential.UpdatedAt = time.Now()

	// 保存更新
	if err := s.storage.UpdateCredential(ctx, credential); err != nil {
		s.logger.Error("更新密钥失败",
			zap.String("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("更新密钥失败: %w", err)
	}

	s.logger.Info("更新密钥成功",
		zap.String("id", id),
		zap.String("name", credential.Name))

	// 返回安全的密钥信息
	return &CredentialSafe{
		ID:          credential.ID,
		Name:        credential.Name,
		Description: credential.Description,
		Type:        credential.Type,
		CreatedAt:   credential.CreatedAt,
		UpdatedAt:   credential.UpdatedAt,
		CreatedBy:   credential.CreatedBy,
		HasContent:  credential.Content != "",
	}, nil
}

// DeleteCredential 删除密钥
func (s *ServiceImpl) DeleteCredential(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("密钥ID不能为空")
	}

	// 检查密钥是否存在
	credential, err := s.storage.GetCredential(ctx, id)
	if err != nil {
		return fmt.Errorf("密钥不存在: %w", err)
	}

	// TODO: 检查密钥是否被其他组件使用
	// 如果被使用，应该阻止删除或提供级联删除选项

	// 删除密钥
	if err := s.storage.DeleteCredential(ctx, id); err != nil {
		s.logger.Error("删除密钥失败",
			zap.String("id", id),
			zap.Error(err))
		return fmt.Errorf("删除密钥失败: %w", err)
	}

	s.logger.Info("删除密钥成功",
		zap.String("id", id),
		zap.String("name", credential.Name))

	return nil
}

// ListCredentials 获取密钥列表
func (s *ServiceImpl) ListCredentials(ctx context.Context, req *CredentialListRequest) ([]*CredentialSafe, int, error) {
	// 设置默认分页
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	return s.storage.ListCredentials(ctx, req)
}

// TestCredential 测试密钥
func (s *ServiceImpl) TestCredential(ctx context.Context, id string, req *CredentialTestRequest) (*CredentialTestResult, error) {
	if id == "" {
		return nil, fmt.Errorf("密钥ID不能为空")
	}

	// 获取密钥
	credential, err := s.storage.GetCredential(ctx, id)
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "获取密钥失败",
		}, nil
	}

	// 解密密钥内容
	content, err := s.storage.DecryptContent(credential.Content)
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "解密密钥内容失败",
		}, nil
	}

	// 设置默认端口
	port := req.Port
	if port <= 0 {
		port = 22
	}

	// 根据密钥类型进行测试
	switch credential.Type {
	case CredentialTypeSSHKey:
		return s.testSSHKey(req.Host, port, req.Username, content)
	case CredentialTypePassword:
		return s.testPassword(req.Host, port, req.Username, content)
	default:
		return &CredentialTestResult{
			Success: false,
			Message: "不支持的密钥类型",
		}, nil
	}
}

// testSSHKey 测试SSH密钥
func (s *ServiceImpl) testSSHKey(host string, port int, username, privateKey string) (*CredentialTestResult, error) {
	start := time.Now()

	// 解析私钥
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "解析SSH私钥失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	// 配置SSH客户端
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 连接SSH服务器
	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "SSH连接失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer client.Close()

	// 执行简单命令测试
	session, err := client.NewSession()
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "创建SSH会话失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer session.Close()

	err = session.Run("echo 'test'")
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "执行测试命令失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	return &CredentialTestResult{
		Success: true,
		Message: "SSH密钥测试成功",
		Latency: time.Since(start).Milliseconds(),
	}, nil
}

// testPassword 测试密码
func (s *ServiceImpl) testPassword(host string, port int, username, password string) (*CredentialTestResult, error) {
	start := time.Now()

	// 配置SSH客户端
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 连接SSH服务器
	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "SSH连接失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer client.Close()

	// 执行简单命令测试
	session, err := client.NewSession()
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "创建SSH会话失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}
	defer session.Close()

	err = session.Run("echo 'test'")
	if err != nil {
		return &CredentialTestResult{
			Success: false,
			Message: "执行测试命令失败",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	return &CredentialTestResult{
		Success: true,
		Message: "密码测试成功",
		Latency: time.Since(start).Milliseconds(),
	}, nil
}

// GetCredentialContent 获取密钥内容（供其他组件使用）
func (s *ServiceImpl) GetCredentialContent(ctx context.Context, id string) (string, CredentialType, error) {
	if id == "" {
		return "", "", fmt.Errorf("密钥ID不能为空")
	}

	credential, err := s.storage.GetCredential(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("获取密钥失败: %w", err)
	}

	// 解密密钥内容
	content, err := s.storage.DecryptContent(credential.Content)
	if err != nil {
		return "", "", fmt.Errorf("解密密钥内容失败: %w", err)
	}

	return content, credential.Type, nil
}

// AnalyzeSSHKey 分析SSH密钥信息
func (s *ServiceImpl) AnalyzeSSHKey(ctx context.Context, id string) (*SSHKeyInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("密钥ID不能为空")
	}

	credential, err := s.storage.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取密钥失败: %w", err)
	}

	if credential.Type != CredentialTypeSSHKey {
		return nil, fmt.Errorf("密钥类型不是SSH密钥")
	}

	// 解密密钥内容
	content, err := s.storage.DecryptContent(credential.Content)
	if err != nil {
		return nil, fmt.Errorf("解密密钥内容失败: %w", err)
	}

	// 解析SSH私钥
	signer, err := ssh.ParsePrivateKey([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("解析SSH私钥失败: %w", err)
	}

	publicKey := signer.PublicKey()

	return &SSHKeyInfo{
		Type:        publicKey.Type(),
		Fingerprint: ssh.FingerprintSHA256(publicKey),
		Comment:     "", // 私钥通常不包含注释
		KeySize:     getBitSize(publicKey),
	}, nil
}

// 辅助函数

// validateCredentialContent 验证密钥内容格式
func (s *ServiceImpl) validateCredentialContent(credType CredentialType, content string) error {
	switch credType {
	case CredentialTypeSSHKey:
		return s.validateSSHKey(content)
	case CredentialTypePassword:
		return s.validatePassword(content)
	case CredentialTypeToken:
		return s.validateToken(content)
	default:
		return fmt.Errorf("不支持的密钥类型: %s", credType)
	}
}

// validateSSHKey 验证SSH密钥格式
func (s *ServiceImpl) validateSSHKey(privateKey string) error {
	_, err := ssh.ParsePrivateKey([]byte(privateKey))
	return err
}

// validatePassword 验证密码格式
func (s *ServiceImpl) validatePassword(password string) error {
	if len(password) < 1 {
		return fmt.Errorf("密码不能为空")
	}
	// 这里可以添加密码强度验证逻辑
	return nil
}

// validateToken 验证Token格式
func (s *ServiceImpl) validateToken(token string) error {
	if len(token) < 1 {
		return fmt.Errorf("Token不能为空")
	}
	// 这里可以添加Token格式验证逻辑
	return nil
}

// generateCredentialID 生成密钥ID
func (s *ServiceImpl) generateCredentialID(name string) string {
	// 使用时间戳和名称生成唯一ID
	timestamp := time.Now().Unix()
	cleanName := strings.ReplaceAll(strings.ToLower(name), " ", "-")
	return fmt.Sprintf("cred-%s-%d", cleanName, timestamp)
}

// getBitSize 获取密钥位数
func getBitSize(key ssh.PublicKey) int {
	switch key.Type() {
	case "ssh-rsa":
		return 2048 // 默认RSA密钥长度
	case "ssh-ed25519":
		return 256 // Ed25519密钥长度
	case "ecdsa-sha2-nistp256":
		return 256
	case "ecdsa-sha2-nistp384":
		return 384
	case "ecdsa-sha2-nistp521":
		return 521
	default:
		return 0
	}
}
