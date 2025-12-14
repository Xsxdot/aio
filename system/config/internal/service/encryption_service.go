package service

import (
	"strings"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/util"
	"xiaozhizhang/system/config/internal/model"
)

// EncryptionService 加密服务
type EncryptionService struct {
	log  *logger.Log
	err  *errorc.ErrorBuilder
	salt string // 加密盐值
}

// NewEncryptionService 创建加密服务实例
func NewEncryptionService(salt string, log *logger.Log) *EncryptionService {
	return &EncryptionService{
		log:  log,
		err:  errorc.NewErrorBuilder("EncryptionService"),
		salt: salt,
	}
}

// EncryptConfigValues 加密配置值中的敏感字段
func (s *EncryptionService) EncryptConfigValues(values map[string]*model.ConfigValue) error {
	for env, configValue := range values {
		if configValue.Type == model.ValueTypeEncrypted {
			// 检查是否已加密
			if !util.IsEncrypted(configValue.Value) {
				encrypted, err := util.EncryptAES(configValue.Value, s.salt)
				if err != nil {
					s.log.WithField("env", env).WithErr(err).Error("加密配置值失败")
					return s.err.New("加密配置值失败", err)
				}
				configValue.Value = encrypted
			}
		}
	}
	return nil
}

// DecryptConfigValues 解密配置值中的敏感字段
func (s *EncryptionService) DecryptConfigValues(values map[string]*model.ConfigValue) error {
	for env, configValue := range values {
		if configValue.Type == model.ValueTypeEncrypted {
			// 检查是否已加密
			if util.IsEncrypted(configValue.Value) {
				decrypted, err := util.DecryptAES(configValue.Value, s.salt)
				if err != nil {
					s.log.WithField("env", env).WithErr(err).Error("解密配置值失败")
					return s.err.New("解密配置值失败", err)
				}
				configValue.Value = decrypted
			}
		}
	}
	return nil
}

// ReEncrypt 使用新的盐值重新加密
func (s *EncryptionService) ReEncrypt(value string, sourceSalt string) (string, error) {
	// 1. 使用源盐值解密
	decrypted, err := util.DecryptAES(value, sourceSalt)
	if err != nil {
		return "", s.err.New("使用源盐值解密失败", err)
	}

	// 2. 使用当前盐值加密
	encrypted, err := util.EncryptAES(decrypted, s.salt)
	if err != nil {
		return "", s.err.New("使用新盐值加密失败", err)
	}

	return encrypted, nil
}

// ParseConfigKey 解析配置键，提取基础路径和环境
func ParseConfigKey(key string) (basePath string, env string, isEnvSpecific bool) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return key, "", false
	}

	lastPart := parts[len(parts)-1]
	validEnvs := []string{"dev", "test", "prod", "staging"}

	for _, e := range validEnvs {
		if lastPart == e {
			return strings.Join(parts[:len(parts)-1], "."), lastPart, true
		}
	}

	return key, "", false // 全局配置
}

// ValidateConfigKey 验证配置键格式
func ValidateConfigKey(key string) error {
	if key == "" {
		return errorc.New("配置键不能为空", nil).ValidWithCtx()
	}

	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return errorc.New("配置键格式错误，至少需要2个部分（如 app.cert）", nil).ValidWithCtx()
	}

	if len(parts) > 4 {
		return errorc.New("配置键格式错误，最多4个部分（如 module.submodule.name.env）", nil).ValidWithCtx()
	}

	// 检查每个部分是否为空
	for _, part := range parts {
		if part == "" {
			return errorc.New("配置键的各部分不能为空", nil).ValidWithCtx()
		}
	}

	return nil
}
