package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/user/internal/dao"
	"github.com/xsxdot/aio/system/user/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ClientCredentialService 客户端凭证服务
type ClientCredentialService struct {
	mvc.IBaseService[model.ClientCredential]
	dao *dao.ClientCredentialDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewClientCredentialService 创建客户端凭证服务实例
func NewClientCredentialService(dao *dao.ClientCredentialDao, log *logger.Log) *ClientCredentialService {
	return &ClientCredentialService{
		IBaseService: mvc.NewBaseService[model.ClientCredential](dao.IBaseDao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("ClientCredentialService"),
	}
}

// FindByClientKey 根据客户端 key 查询
func (s *ClientCredentialService) FindByClientKey(ctx context.Context, clientKey string) (*model.ClientCredential, error) {
	return s.dao.FindByClientKey(ctx, clientKey)
}

// CreateClient 创建客户端凭证（自动生成 key/secret）
func (s *ClientCredentialService) CreateClient(ctx context.Context, name, description string, ipWhitelist []string, expiresAt *time.Time) (*model.ClientCredential, string, error) {
	// 生成客户端 key
	clientKey, err := s.GenerateClientKey()
	if err != nil {
		return nil, "", err
	}

	// 检查 key 是否已存在（理论上概率极低）
	exists, err := s.dao.ExistsByClientKey(ctx, clientKey)
	if err != nil {
		return nil, "", err
	}
	if exists {
		return nil, "", s.err.New("客户端 key 冲突，请重试", nil).ValidWithCtx()
	}

	// 生成客户端 secret（明文，只返回一次）
	clientSecret, err := s.GenerateClientSecret()
	if err != nil {
		return nil, "", err
	}

	// 散列 secret 用于存储
	secretHash, err := s.HashSecret(clientSecret)
	if err != nil {
		return nil, "", err
	}

	// 序列化 IP 白名单（空数组序列化为 "[]" 而非空字符串）
	data, err := json.Marshal(ipWhitelist)
	if err != nil {
		return nil, "", s.err.New("序列化 IP 白名单失败", err)
	}
	ipWhitelistJSON := string(data)

	// 创建客户端
	client := &model.ClientCredential{
		Name:         name,
		ClientKey:    clientKey,
		ClientSecret: secretHash,
		Status:       model.ClientCredentialStatusEnabled,
		Description:  description,
		IPWhitelist:  ipWhitelistJSON,
		ExpiresAt:    expiresAt,
	}

	if err := s.dao.Create(ctx, client); err != nil {
		return nil, "", err
	}

	// 返回客户端信息和明文 secret（明文 secret 只在创建时返回）
	return client, clientSecret, nil
}

// UpdateClient 更新客户端信息（不更新 secret）
func (s *ClientCredentialService) UpdateClient(ctx context.Context, id int64, name, description string, ipWhitelist []string, expiresAt *time.Time) error {
	// 验证客户端是否存在
	_, err := s.dao.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 序列化 IP 白名单（空数组序列化为 "[]" 而非空字符串）
	data, err := json.Marshal(ipWhitelist)
	if err != nil {
		return s.err.New("序列化 IP 白名单失败", err)
	}
	ipWhitelistJSON := string(data)

	// 更新客户端
	updateData := &model.ClientCredential{
		Name:        name,
		Description: description,
		IPWhitelist: ipWhitelistJSON,
		ExpiresAt:   expiresAt,
	}

	_, err = s.dao.UpdateById(ctx, id, updateData)
	return err
}

// RotateSecret 重新生成客户端 secret
func (s *ClientCredentialService) RotateSecret(ctx context.Context, id int64) (string, error) {
	// 验证客户端是否存在
	_, err := s.dao.FindById(ctx, id)
	if err != nil {
		return "", err
	}

	// 生成新的 secret
	newSecret, err := s.GenerateClientSecret()
	if err != nil {
		return "", err
	}

	// 散列新 secret
	secretHash, err := s.HashSecret(newSecret)
	if err != nil {
		return "", err
	}

	// 更新 secret
	if err := s.dao.UpdateSecret(ctx, id, secretHash); err != nil {
		return "", err
	}

	// 返回明文 secret（只在生成时返回）
	return newSecret, nil
}

// UpdateStatus 更新客户端状态
func (s *ClientCredentialService) UpdateStatus(ctx context.Context, id int64, status int8) error {
	// 验证状态值
	if status != model.ClientCredentialStatusEnabled && status != model.ClientCredentialStatusDisabled {
		return s.err.New("无效的状态值", nil).ValidWithCtx()
	}

	return s.dao.UpdateStatus(ctx, id, status)
}

// ValidateClient 验证客户端凭证（用于鉴权）
func (s *ClientCredentialService) ValidateClient(ctx context.Context, clientKey, clientSecret string) (*model.ClientCredential, error) {
	// 查询客户端
	client, err := s.dao.FindByClientKey(ctx, clientKey)
	if err != nil {
		if errorc.IsNotFound(err) {
			return nil, s.err.New("客户端凭证无效", nil).ValidWithCtx()
		}
		return nil, err
	}

	// 检查状态
	if !client.IsActive() {
		if client.Status == model.ClientCredentialStatusDisabled {
			return nil, s.err.New("客户端已被禁用", nil).ValidWithCtx()
		}
		if client.IsExpired() {
			return nil, s.err.New("客户端凭证已过期", nil).ValidWithCtx()
		}
	}

	// 验证 secret
	if !s.VerifySecret(clientSecret, client.ClientSecret) {
		return nil, s.err.New("客户端凭证无效", nil).ValidWithCtx()
	}

	return client, nil
}

// FindAllActive 查询所有启用且未过期的客户端
func (s *ClientCredentialService) FindAllActive(ctx context.Context) ([]*model.ClientCredential, error) {
	return s.dao.FindAllActive(ctx)
}

// GenerateClientKey 生成客户端 key（24字节随机，Base64编码）
func (s *ClientCredentialService) GenerateClientKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", s.err.New("生成客户端 key 失败", err)
	}
	// 使用 URL 安全的 Base64 编码
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateClientSecret 生成客户端 secret（32字节随机，Base64编码）
func (s *ClientCredentialService) GenerateClientSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", s.err.New("生成客户端 secret 失败", err)
	}
	// 使用 URL 安全的 Base64 编码
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashSecret 散列 secret
func (s *ClientCredentialService) HashSecret(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", s.err.New("secret 散列失败", err)
	}
	return string(hash), nil
}

// VerifySecret 验证 secret
func (s *ClientCredentialService) VerifySecret(secret, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret))
	return err == nil
}

// WithTx 使用事务
func (s *ClientCredentialService) WithTx(tx *gorm.DB) *ClientCredentialService {
	return &ClientCredentialService{
		IBaseService: s.IBaseService.WithTx(tx),
		dao:          s.dao.WithTx(tx),
		log:          s.log,
		err:          s.err,
	}
}


