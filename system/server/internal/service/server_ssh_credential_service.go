package service

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/server/internal/dao"
	"xiaozhizhang/system/server/internal/model"
)

// ServerSSHCredentialService SSH 凭证服务层
type ServerSSHCredentialService struct {
	dao       *dao.ServerSSHCredentialDao
	cryptoSvc *CryptoService
	log       *logger.Log
	err       *errorc.ErrorBuilder
}

// NewServerSSHCredentialService 创建 SSH 凭证服务实例
func NewServerSSHCredentialService(dao *dao.ServerSSHCredentialDao, cryptoSvc *CryptoService, log *logger.Log) *ServerSSHCredentialService {
	return &ServerSSHCredentialService{
		dao:       dao,
		cryptoSvc: cryptoSvc,
		log:       log.WithEntryName("ServerSSHCredentialService"),
		err:       errorc.NewErrorBuilder("ServerSSHCredentialService"),
	}
}

// Upsert 更新或插入 SSH 凭证（加密敏感字段）
func (s *ServerSSHCredentialService) Upsert(ctx context.Context, credential *model.ServerSSHCredential) error {
	// 加密敏感字段
	if credential.Password != "" && !s.cryptoSvc.IsEncrypted(credential.Password) {
		encrypted, err := s.cryptoSvc.Encrypt(credential.Password)
		if err != nil {
			return s.err.New("加密密码失败", err)
		}
		credential.Password = encrypted
	}

	if credential.PrivateKey != "" && !s.cryptoSvc.IsEncrypted(credential.PrivateKey) {
		encrypted, err := s.cryptoSvc.Encrypt(credential.PrivateKey)
		if err != nil {
			return s.err.New("加密私钥失败", err)
		}
		credential.PrivateKey = encrypted
	}

	return s.dao.UpsertByServerID(ctx, credential)
}

// GetDecrypted 获取 SSH 凭证并解密敏感字段
func (s *ServerSSHCredentialService) GetDecrypted(ctx context.Context, serverID int64) (*model.ServerSSHCredential, error) {
	credential, err := s.dao.FindByServerID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// 解密敏感字段
	if s.cryptoSvc.IsEncrypted(credential.Password) {
		decrypted, err := s.cryptoSvc.Decrypt(credential.Password)
		if err != nil {
			return nil, s.err.New("解密密码失败", err)
		}
		credential.Password = decrypted
	}

	if s.cryptoSvc.IsEncrypted(credential.PrivateKey) {
		decrypted, err := s.cryptoSvc.Decrypt(credential.PrivateKey)
		if err != nil {
			return nil, s.err.New("解密私钥失败", err)
		}
		credential.PrivateKey = decrypted
	}

	return credential, nil
}

// Delete 删除 SSH 凭证
func (s *ServerSSHCredentialService) Delete(ctx context.Context, serverID int64) error {
	return s.dao.DeleteByServerID(ctx, serverID)
}

// Exists 检查 SSH 凭证是否存在（不触发解密）
func (s *ServerSSHCredentialService) Exists(ctx context.Context, serverID int64) (bool, error) {
	_, err := s.dao.FindByServerID(ctx, serverID)
	if err != nil {
		if errorc.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
