// Package credential 密钥管理存储实现
package credential

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/xsxdot/aio/internal/etcd"
	"go.uber.org/zap"
)

const (
	// ETCD存储路径前缀
	credentialPrefix = "/aio/credentials/"
	// 加密前缀标识
	encryptedPrefix = "ENC_AES:"
)

// ETCDStorage ETCD存储实现
type ETCDStorage struct {
	client     *etcd.EtcdClient
	logger     *zap.Logger
	encryptKey []byte // 32字节的AES-256密钥
}

// ETCDStorageConfig ETCD存储配置
type ETCDStorageConfig struct {
	Client     *etcd.EtcdClient
	Logger     *zap.Logger
	EncryptKey string // 用于加密的密钥，应至少32字符
}

// NewETCDStorage 创建ETCD存储实例
func NewETCDStorage(config ETCDStorageConfig) (Storage, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("ETCD客户端不能为空")
	}

	if config.Logger == nil {
		config.Logger, _ = zap.NewProduction()
	}

	// 生成AES密钥
	encryptKey := generateAESKey(config.EncryptKey)

	return &ETCDStorage{
		client:     config.Client,
		logger:     config.Logger,
		encryptKey: encryptKey,
	}, nil
}

// generateAESKey 生成32字节的AES-256密钥
func generateAESKey(keyString string) []byte {
	if keyString == "" {
		keyString = "default-aio-credential-key-12345"
	}

	key := []byte(keyString)
	// 确保密钥长度为32字节(AES-256)
	if len(key) > 32 {
		return key[:32]
	}

	// 如果密钥太短，重复填充
	result := make([]byte, 32)
	for i := 0; i < 32; i++ {
		result[i] = key[i%len(key)]
	}
	return result
}

// CreateCredential 创建密钥
func (s *ETCDStorage) CreateCredential(ctx context.Context, credential *Credential) error {
	key := s.buildKey(credential.ID)

	// 序列化密钥对象
	data, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("序列化密钥失败: %w", err)
	}

	// 保存到ETCD
	if err := s.client.Put(ctx, key, string(data)); err != nil {
		s.logger.Error("保存密钥到ETCD失败",
			zap.String("id", credential.ID),
			zap.Error(err))
		return fmt.Errorf("保存密钥失败: %w", err)
	}

	s.logger.Info("密钥创建成功",
		zap.String("id", credential.ID),
		zap.String("name", credential.Name))

	return nil
}

// GetCredential 获取密钥
func (s *ETCDStorage) GetCredential(ctx context.Context, id string) (*Credential, error) {
	key := s.buildKey(id)

	value, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("从ETCD获取密钥失败: %w", err)
	}

	if value == "" {
		return nil, fmt.Errorf("密钥不存在: %s", id)
	}

	var credential Credential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return nil, fmt.Errorf("解析密钥数据失败: %w", err)
	}

	return &credential, nil
}

// GetCredentialSafe 获取安全的密钥信息（不包含敏感内容）
func (s *ETCDStorage) GetCredentialSafe(ctx context.Context, id string) (*CredentialSafe, error) {
	credential, err := s.GetCredential(ctx, id)
	if err != nil {
		return nil, err
	}

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
func (s *ETCDStorage) UpdateCredential(ctx context.Context, credential *Credential) error {
	key := s.buildKey(credential.ID)

	// 序列化密钥对象
	data, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("序列化密钥失败: %w", err)
	}

	// 更新ETCD中的数据
	if err := s.client.Put(ctx, key, string(data)); err != nil {
		s.logger.Error("更新ETCD中的密钥失败",
			zap.String("id", credential.ID),
			zap.Error(err))
		return fmt.Errorf("更新密钥失败: %w", err)
	}

	s.logger.Info("密钥更新成功",
		zap.String("id", credential.ID),
		zap.String("name", credential.Name))

	return nil
}

// DeleteCredential 删除密钥
func (s *ETCDStorage) DeleteCredential(ctx context.Context, id string) error {
	key := s.buildKey(id)

	if err := s.client.Delete(ctx, key); err != nil {
		s.logger.Error("从ETCD删除密钥失败",
			zap.String("id", id),
			zap.Error(err))
		return fmt.Errorf("删除密钥失败: %w", err)
	}

	s.logger.Info("密钥删除成功", zap.String("id", id))
	return nil
}

// ListCredentials 获取密钥列表
func (s *ETCDStorage) ListCredentials(ctx context.Context, req *CredentialListRequest) ([]*CredentialSafe, int, error) {
	// 获取所有密钥
	values, err := s.client.GetWithPrefix(ctx, credentialPrefix)
	if err != nil {
		return nil, 0, fmt.Errorf("从ETCD获取密钥列表失败: %w", err)
	}

	var allCredentials []*CredentialSafe
	for _, value := range values {
		var credential Credential
		if err := json.Unmarshal([]byte(value), &credential); err != nil {
			s.logger.Warn("解析密钥数据失败", zap.Error(err))
			continue
		}

		// 类型过滤
		if req.Type != "" && credential.Type != req.Type {
			continue
		}

		safe := &CredentialSafe{
			ID:          credential.ID,
			Name:        credential.Name,
			Description: credential.Description,
			Type:        credential.Type,
			CreatedAt:   credential.CreatedAt,
			UpdatedAt:   credential.UpdatedAt,
			CreatedBy:   credential.CreatedBy,
			HasContent:  credential.Content != "",
		}
		allCredentials = append(allCredentials, safe)
	}

	total := len(allCredentials)

	// 分页处理
	start := req.Offset
	if start > total {
		start = total
	}

	end := start + req.Limit
	if end > total {
		end = total
	}

	result := allCredentials[start:end]
	return result, total, nil
}

// GetCredentialsByType 根据类型获取密钥
func (s *ETCDStorage) GetCredentialsByType(ctx context.Context, credType CredentialType) ([]*CredentialSafe, error) {
	req := &CredentialListRequest{
		Type:   credType,
		Limit:  1000, // 设置较大的限制
		Offset: 0,
	}
	credentials, _, err := s.ListCredentials(ctx, req)
	return credentials, err
}

// GetCredentialsByUser 根据用户获取密钥
func (s *ETCDStorage) GetCredentialsByUser(ctx context.Context, userID string) ([]*CredentialSafe, error) {
	// 获取所有密钥
	values, err := s.client.GetWithPrefix(ctx, credentialPrefix)
	if err != nil {
		return nil, fmt.Errorf("从ETCD获取密钥列表失败: %w", err)
	}

	var credentials []*CredentialSafe
	for _, value := range values {
		var credential Credential
		if err := json.Unmarshal([]byte(value), &credential); err != nil {
			s.logger.Warn("解析密钥数据失败", zap.Error(err))
			continue
		}

		// 用户过滤
		if credential.CreatedBy == userID {
			safe := &CredentialSafe{
				ID:          credential.ID,
				Name:        credential.Name,
				Description: credential.Description,
				Type:        credential.Type,
				CreatedAt:   credential.CreatedAt,
				UpdatedAt:   credential.UpdatedAt,
				CreatedBy:   credential.CreatedBy,
				HasContent:  credential.Content != "",
			}
			credentials = append(credentials, safe)
		}
	}

	return credentials, nil
}

// EncryptContent 加密内容
func (s *ETCDStorage) EncryptContent(content string) (string, error) {
	if content == "" {
		return "", nil
	}

	// 创建AES加密器
	block, err := aes.NewCipher(s.encryptKey)
	if err != nil {
		return "", fmt.Errorf("创建AES加密器失败: %w", err)
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(content), nil)

	// Base64编码
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encryptedPrefix + encoded, nil
}

// DecryptContent 解密内容
func (s *ETCDStorage) DecryptContent(encryptedContent string) (string, error) {
	if encryptedContent == "" {
		return "", nil
	}

	// 检查加密前缀
	if !strings.HasPrefix(encryptedContent, encryptedPrefix) {
		// 如果没有加密前缀，假设是明文（向后兼容）
		return encryptedContent, nil
	}

	// 移除前缀
	encoded := strings.TrimPrefix(encryptedContent, encryptedPrefix)

	// Base64解码
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("Base64解码失败: %w", err)
	}

	// 创建AES解密器
	block, err := aes.NewCipher(s.encryptKey)
	if err != nil {
		return "", fmt.Errorf("创建AES解密器失败: %w", err)
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}

	// 检查密文长度
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("密文长度不足")
	}

	// 提取nonce和密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// buildKey 构建存储键
func (s *ETCDStorage) buildKey(id string) string {
	return fmt.Sprintf("%s%s", credentialPrefix, id)
}
