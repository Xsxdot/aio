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
	for field, configValue := range values {
		if configValue.Type == model.ValueTypeEncrypted {
			// 检查是否已加密
			if !util.IsEncrypted(configValue.Value) {
				encrypted, err := util.EncryptAES(configValue.Value, s.salt)
				if err != nil {
					s.log.WithField("field", field).WithErr(err).Error("加密配置值失败")
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
	for field, configValue := range values {
		if configValue.Type == model.ValueTypeEncrypted {
			// 检查是否已加密
			if util.IsEncrypted(configValue.Value) {
				decrypted, err := util.DecryptAES(configValue.Value, s.salt)
				if err != nil {
					s.log.WithField("field", field).WithErr(err).Error("解密配置值失败")
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

// ComposeFullKey 合成完整配置键（兼容直接传入完整 key）
// 规则：
// 1. 若 key 已带合法环境后缀，检查与 env 参数是否一致
// 2. 若 key 不带环境后缀，拼接 key + "." + env
// 3. 若两者不一致，返回错误
func ComposeFullKey(key string, env string) (string, error) {
	if key == "" {
		return "", errorc.New("配置键不能为空", nil).ValidWithCtx()
	}

	basePath, keyEnv, isEnvSpecific := ParseConfigKey(key)

	if isEnvSpecific {
		// key 已带环境后缀
		if env != "" && keyEnv != env {
			// 环境参数与 key 中的环境不一致
			return "", errorc.New("配置键中的环境("+keyEnv+")与参数环境("+env+")不一致", nil).ValidWithCtx()
		}
		// 直接使用完整 key
		return key, nil
	}

	// key 不带环境后缀，需要拼接
	if env == "" {
		return "", errorc.New("配置键未指定环境且参数环境为空", nil).ValidWithCtx()
	}

	// 验证 env 是否合法
	validEnvs := []string{"dev", "test", "prod", "staging"}
	isValidEnv := false
	for _, e := range validEnvs {
		if env == e {
			isValidEnv = true
			break
		}
	}
	if !isValidEnv {
		return "", errorc.New("环境参数无效，必须是 dev/test/prod/staging 之一", nil).ValidWithCtx()
	}

	return basePath + "." + env, nil
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

	if len(parts) > 6 {
		return errorc.New("配置键格式错误，最多6个部分（如 team.module.submodule.category.name.env）", nil).ValidWithCtx()
	}

	// 检查每个部分是否为空
	for _, part := range parts {
		if part == "" {
			return errorc.New("配置键的各部分不能为空", nil).ValidWithCtx()
		}
	}

	return nil
}
