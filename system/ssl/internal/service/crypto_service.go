package service

import (
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/util"
)

// CryptoService 加密服务
// 复用 pkg/core/util 的 AES-GCM 加密能力
type CryptoService struct {
	log  *logger.Log
	err  *errorc.ErrorBuilder
	salt string
}

// NewCryptoService 创建加密服务实例
func NewCryptoService(log *logger.Log) *CryptoService {
	return &CryptoService{
		log:  log.WithEntryName("CryptoService"),
		err:  errorc.NewErrorBuilder("CryptoService"),
		salt: base.Configures.Config.ConfigCenter.EncryptionSalt,
	}
}

// Encrypt 加密字符串
func (s *CryptoService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	encrypted, err := util.EncryptAES(plaintext, s.salt)
	if err != nil {
		return "", s.err.New("加密失败", err)
	}
	return encrypted, nil
}

// Decrypt 解密字符串
func (s *CryptoService) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	decrypted, err := util.DecryptAES(ciphertext, s.salt)
	if err != nil {
		return "", s.err.New("解密失败", err)
	}
	return decrypted, nil
}

// IsEncrypted 检查字符串是否已加密
func (s *CryptoService) IsEncrypted(text string) bool {
	return util.IsEncrypted(text)
}
