package service

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/ssl/internal/dao"
	"xiaozhizhang/system/ssl/internal/model"
)

// DnsCredentialService DNS 凭证服务
type DnsCredentialService struct {
	dao       *dao.DnsCredentialDao
	cryptoSvc *CryptoService
	log       *logger.Log
	err       *errorc.ErrorBuilder
}

// NewDnsCredentialService 创建 DNS 凭证服务实例
func NewDnsCredentialService(dao *dao.DnsCredentialDao, cryptoSvc *CryptoService, log *logger.Log) *DnsCredentialService {
	return &DnsCredentialService{
		dao:       dao,
		cryptoSvc: cryptoSvc,
		log:       log.WithEntryName("DnsCredentialService"),
		err:       errorc.NewErrorBuilder("DnsCredentialService"),
	}
}

// Create 创建 DNS 凭证（加密敏感字段）
func (s *DnsCredentialService) Create(ctx context.Context, credential *model.DnsCredential) error {
	// 加密敏感字段
	if !s.cryptoSvc.IsEncrypted(credential.AccessKey) {
		encrypted, err := s.cryptoSvc.Encrypt(credential.AccessKey)
		if err != nil {
			return s.err.New("加密 AccessKey 失败", err)
		}
		credential.AccessKey = encrypted
	}

	if !s.cryptoSvc.IsEncrypted(credential.SecretKey) {
		encrypted, err := s.cryptoSvc.Encrypt(credential.SecretKey)
		if err != nil {
			return s.err.New("加密 SecretKey 失败", err)
		}
		credential.SecretKey = encrypted
	}

	return s.dao.Create(ctx, credential)
}

// GetDecrypted 获取凭证并解密敏感字段
func (s *DnsCredentialService) GetDecrypted(ctx context.Context, id uint) (*model.DnsCredential, error) {
	credential, err := s.dao.FindById(ctx, id)
	if err != nil {
		return nil, err
	}

	// 解密敏感字段
	if s.cryptoSvc.IsEncrypted(credential.AccessKey) {
		decrypted, err := s.cryptoSvc.Decrypt(credential.AccessKey)
		if err != nil {
			return nil, s.err.New("解密 AccessKey 失败", err)
		}
		credential.AccessKey = decrypted
	}

	if s.cryptoSvc.IsEncrypted(credential.SecretKey) {
		decrypted, err := s.cryptoSvc.Decrypt(credential.SecretKey)
		if err != nil {
			return nil, s.err.New("解密 SecretKey 失败", err)
		}
		credential.SecretKey = decrypted
	}

	return credential, nil
}
