package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/config/internal/model"

	"gorm.io/gorm"
)

// ConfigHistoryDao 配置历史数据访问层
type ConfigHistoryDao struct {
	mvc.IBaseDao[model.ConfigHistoryModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewConfigHistoryDao 创建配置历史DAO实例
func NewConfigHistoryDao(db *gorm.DB, log *logger.Log) *ConfigHistoryDao {
	return &ConfigHistoryDao{
		IBaseDao: mvc.NewGormDao[model.ConfigHistoryModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("ConfigHistoryDao"),
		db:       db,
	}
}

// FindByConfigKey 根据配置键查询历史记录
func (d *ConfigHistoryDao) FindByConfigKey(ctx context.Context, configKey string) ([]*model.ConfigHistoryModel, error) {
	var histories []*model.ConfigHistoryModel
	err := d.db.WithContext(ctx).
		Where("config_key = ?", configKey).
		Order("version DESC").
		Find(&histories).Error
	if err != nil {
		return nil, d.err.New("查询配置历史失败", err).DB()
	}
	return histories, nil
}

// FindByConfigKeyAndVersion 根据配置键和版本号查询
func (d *ConfigHistoryDao) FindByConfigKeyAndVersion(ctx context.Context, configKey string, version int64) (*model.ConfigHistoryModel, error) {
	var history model.ConfigHistoryModel
	err := d.db.WithContext(ctx).
		Where("config_key = ? AND version = ?", configKey, version).
		First(&history).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("配置历史版本不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询配置历史版本失败", err).DB()
	}
	return &history, nil
}

// FindLatestByConfigKey 查询配置的最新历史记录
func (d *ConfigHistoryDao) FindLatestByConfigKey(ctx context.Context, configKey string) (*model.ConfigHistoryModel, error) {
	var history model.ConfigHistoryModel
	err := d.db.WithContext(ctx).
		Where("config_key = ?", configKey).
		Order("version DESC").
		First(&history).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("配置历史记录不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询最新配置历史失败", err).DB()
	}
	return &history, nil
}

// CountByConfigKey 统计配置的历史版本数
func (d *ConfigHistoryDao) CountByConfigKey(ctx context.Context, configKey string) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).
		Model(&model.ConfigHistoryModel{}).
		Where("config_key = ?", configKey).
		Count(&count).Error
	if err != nil {
		return 0, d.err.New("统计配置历史版本数失败", err).DB()
	}
	return count, nil
}

// DeleteByConfigKey 删除配置的所有历史记录
func (d *ConfigHistoryDao) DeleteByConfigKey(ctx context.Context, configKey string) error {
	err := d.db.WithContext(ctx).
		Where("config_key = ?", configKey).
		Delete(&model.ConfigHistoryModel{}).Error
	if err != nil {
		return d.err.New("删除配置历史记录失败", err).DB()
	}
	return nil
}

// WithTx 使用事务
func (d *ConfigHistoryDao) WithTx(tx *gorm.DB) *ConfigHistoryDao {
	return &ConfigHistoryDao{
		IBaseDao: mvc.NewGormDao[model.ConfigHistoryModel](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}
