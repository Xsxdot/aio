package service

import (
	"context"
	"encoding/json"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/config/internal/dao"
	"xiaozhizhang/system/config/internal/model"

	"gorm.io/gorm"
)

// ConfigItemService 配置项服务
type ConfigItemService struct {
	mvc.IBaseService[model.ConfigItemModel]
	dao               *dao.ConfigItemDao
	encryptionService *EncryptionService
	log               *logger.Log
	err               *errorc.ErrorBuilder
}

// NewConfigItemService 创建配置项服务实例
func NewConfigItemService(dao *dao.ConfigItemDao, encryptionService *EncryptionService, log *logger.Log) *ConfigItemService {
	return &ConfigItemService{
		IBaseService:      mvc.NewBaseService[model.ConfigItemModel](dao.IBaseDao),
		dao:               dao,
		encryptionService: encryptionService,
		log:               log,
		err:               errorc.NewErrorBuilder("ConfigItemService"),
	}
}

// FindByKey 根据配置键查询
func (s *ConfigItemService) FindByKey(ctx context.Context, key string) (*model.ConfigItemModel, error) {
	return s.dao.FindByKey(ctx, key)
}

// FindByKeyWithDecrypt 根据配置键查询并解密
func (s *ConfigItemService) FindByKeyWithDecrypt(ctx context.Context, key string) (*model.ConfigItem, error) {
	item, err := s.dao.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return s.ConvertAndDecrypt(item)
}

// ConvertAndDecrypt 转换数据库模型为业务模型并解密
func (s *ConfigItemService) ConvertAndDecrypt(dbModel *model.ConfigItemModel) (*model.ConfigItem, error) {
	// 解析 Value JSON
	var values map[string]*model.ConfigValue
	if err := json.Unmarshal([]byte(dbModel.Value), &values); err != nil {
		return nil, s.err.New("解析配置值失败", err)
	}

	// 解密敏感字段
	if err := s.encryptionService.DecryptConfigValues(values); err != nil {
		return nil, err
	}

	// 解析 Metadata JSON
	var metadata map[string]string
	if dbModel.Metadata != "" {
		if err := json.Unmarshal([]byte(dbModel.Metadata), &metadata); err != nil {
			return nil, s.err.New("解析元数据失败", err)
		}
	}

	return &model.ConfigItem{
		Key:      dbModel.Key,
		Value:    values,
		Version:  dbModel.Version,
		Metadata: metadata,
	}, nil
}

// GetConfigValueByEnv 根据环境获取配置值
func (s *ConfigItemService) GetConfigValueByEnv(ctx context.Context, key string, env string) (*model.ConfigValue, error) {
	configItem, err := s.FindByKeyWithDecrypt(ctx, key)
	if err != nil {
		return nil, err
	}

	// 查找指定环境的配置
	if value, ok := configItem.Value[env]; ok {
		return value, nil
	}

	// 如果没有找到，尝试查找全局配置
	if value, ok := configItem.Value["global"]; ok {
		return value, nil
	}

	return nil, s.err.New("指定环境的配置不存在", nil).WithCode(errorc.ErrorCodeNotFound)
}

// CreateWithEncrypt 创建配置（自动加密）
func (s *ConfigItemService) CreateWithEncrypt(ctx context.Context, configItem *model.ConfigItem) error {
	// 验证配置键
	if err := ValidateConfigKey(configItem.Key); err != nil {
		return err
	}

	// 检查是否已存在
	exists, err := s.dao.ExistsByKey(ctx, configItem.Key)
	if err != nil {
		return err
	}
	if exists {
		return s.err.New("配置键已存在", nil).ValidWithCtx()
	}

	// 加密敏感字段
	if err := s.encryptionService.EncryptConfigValues(configItem.Value); err != nil {
		return err
	}

	// 序列化为 JSON
	valueJSON, err := json.Marshal(configItem.Value)
	if err != nil {
		return s.err.New("序列化配置值失败", err)
	}

	var metadataJSON []byte
	if configItem.Metadata != nil {
		metadataJSON, err = json.Marshal(configItem.Metadata)
		if err != nil {
			return s.err.New("序列化元数据失败", err)
		}
	}

	// 创建数据库模型
	dbModel := &model.ConfigItemModel{
		Key:      configItem.Key,
		Value:    string(valueJSON),
		Version:  1,
		Metadata: string(metadataJSON),
	}

	return s.dao.Create(ctx, dbModel)
}

// UpdateWithEncrypt 更新配置（自动加密）
func (s *ConfigItemService) UpdateWithEncrypt(ctx context.Context, id int64, configItem *model.ConfigItem) error {
	// 加密敏感字段
	if err := s.encryptionService.EncryptConfigValues(configItem.Value); err != nil {
		return err
	}

	// 序列化为 JSON
	valueJSON, err := json.Marshal(configItem.Value)
	if err != nil {
		return s.err.New("序列化配置值失败", err)
	}

	var metadataJSON []byte
	if configItem.Metadata != nil {
		metadataJSON, err = json.Marshal(configItem.Metadata)
		if err != nil {
			return s.err.New("序列化元数据失败", err)
		}
	}

	// 更新数据库模型
	dbModel := &model.ConfigItemModel{
		Value:    string(valueJSON),
		Metadata: string(metadataJSON),
	}

	_, err = s.dao.UpdateById(ctx, id, dbModel)
	return err
}

// FindByKeyLike 根据配置键模糊查询
func (s *ConfigItemService) FindByKeyLike(ctx context.Context, keyPattern string) ([]*model.ConfigItemModel, error) {
	return s.dao.FindByKeyLike(ctx, keyPattern)
}

// FindAll 查询所有配置项
func (s *ConfigItemService) FindAll(ctx context.Context) ([]*model.ConfigItemModel, error) {
	return s.dao.FindAll(ctx)
}

// FindByKeys 根据多个配置键查询
func (s *ConfigItemService) FindByKeys(ctx context.Context, keys []string) ([]*model.ConfigItemModel, error) {
	return s.dao.FindByKeys(ctx, keys)
}

// IncrementVersion 增加版本号
func (s *ConfigItemService) IncrementVersion(ctx context.Context, id int64) error {
	return s.dao.IncrementVersion(ctx, id)
}

// WithTx 使用事务
func (s *ConfigItemService) WithTx(tx *gorm.DB) *ConfigItemService {
	return &ConfigItemService{
		IBaseService:      s.IBaseService.WithTx(tx),
		dao:               s.dao.WithTx(tx),
		encryptionService: s.encryptionService,
		log:               s.log,
		err:               s.err,
	}
}
