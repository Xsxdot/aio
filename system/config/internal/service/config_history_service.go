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

// ConfigHistoryService 配置历史服务
type ConfigHistoryService struct {
	mvc.IBaseService[model.ConfigHistoryModel]
	dao *dao.ConfigHistoryDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewConfigHistoryService 创建配置历史服务实例
func NewConfigHistoryService(dao *dao.ConfigHistoryDao, log *logger.Log) *ConfigHistoryService {
	return &ConfigHistoryService{
		IBaseService: mvc.NewBaseService[model.ConfigHistoryModel](dao.IBaseDao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("ConfigHistoryService"),
	}
}

// CreateHistory 创建历史记录
func (s *ConfigHistoryService) CreateHistory(ctx context.Context, configKey string, version int64, values map[string]*model.ConfigValue, metadata map[string]string, operator string, operatorID int64, changeNote string) error {
	// 序列化配置值
	valueJSON, err := json.Marshal(values)
	if err != nil {
		return s.err.New("序列化配置值失败", err)
	}

	// 序列化元数据
	var metadataJSON []byte
	if metadata != nil {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return s.err.New("序列化元数据失败", err)
		}
	}

	history := &model.ConfigHistoryModel{
		ConfigKey:  configKey,
		Version:    version,
		Value:      string(valueJSON),
		Metadata:   string(metadataJSON),
		Operator:   operator,
		OperatorID: operatorID,
		ChangeNote: changeNote,
	}

	return s.dao.Create(ctx, history)
}

// FindByConfigKey 根据配置键查询历史记录
func (s *ConfigHistoryService) FindByConfigKey(ctx context.Context, configKey string) ([]*model.ConfigHistoryModel, error) {
	return s.dao.FindByConfigKey(ctx, configKey)
}

// FindByConfigKeyAndVersion 根据配置键和版本号查询
func (s *ConfigHistoryService) FindByConfigKeyAndVersion(ctx context.Context, configKey string, version int64) (*model.ConfigHistoryModel, error) {
	return s.dao.FindByConfigKeyAndVersion(ctx, configKey, version)
}

// FindLatestByConfigKey 查询配置的最新历史记录
func (s *ConfigHistoryService) FindLatestByConfigKey(ctx context.Context, configKey string) (*model.ConfigHistoryModel, error) {
	return s.dao.FindLatestByConfigKey(ctx, configKey)
}

// CountByConfigKey 统计配置的历史版本数
func (s *ConfigHistoryService) CountByConfigKey(ctx context.Context, configKey string) (int64, error) {
	return s.dao.CountByConfigKey(ctx, configKey)
}

// DeleteByConfigKey 删除配置的所有历史记录
func (s *ConfigHistoryService) DeleteByConfigKey(ctx context.Context, configKey string) error {
	return s.dao.DeleteByConfigKey(ctx, configKey)
}

// WithTx 使用事务
func (s *ConfigHistoryService) WithTx(tx *gorm.DB) *ConfigHistoryService {
	return &ConfigHistoryService{
		IBaseService: s.IBaseService.WithTx(tx),
		dao:          s.dao.WithTx(tx),
		log:          s.log,
		err:          s.err,
	}
}
