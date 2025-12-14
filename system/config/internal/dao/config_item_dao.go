package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/config/internal/model"

	"gorm.io/gorm"
)

// ConfigItemDao 配置项数据访问层
type ConfigItemDao struct {
	mvc.IBaseDao[model.ConfigItemModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewConfigItemDao 创建配置项DAO实例
func NewConfigItemDao(db *gorm.DB, log *logger.Log) *ConfigItemDao {
	return &ConfigItemDao{
		IBaseDao: mvc.NewGormDao[model.ConfigItemModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("ConfigItemDao"),
		db:       db,
	}
}

// FindByKey 根据配置键查询
func (d *ConfigItemDao) FindByKey(ctx context.Context, key string) (*model.ConfigItemModel, error) {
	var item model.ConfigItemModel
	err := d.db.WithContext(ctx).Where("key = ?", key).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("配置项不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询配置项失败", err).DB()
	}
	return &item, nil
}

// FindByKeyLike 根据配置键模糊查询
func (d *ConfigItemDao) FindByKeyLike(ctx context.Context, keyPattern string) ([]*model.ConfigItemModel, error) {
	var items []*model.ConfigItemModel
	err := d.db.WithContext(ctx).Where("key LIKE ?", "%"+keyPattern+"%").Find(&items).Error
	if err != nil {
		return nil, d.err.New("模糊查询配置项失败", err).DB()
	}
	return items, nil
}

// FindAll 查询所有配置项
func (d *ConfigItemDao) FindAll(ctx context.Context) ([]*model.ConfigItemModel, error) {
	var items []*model.ConfigItemModel
	err := d.db.WithContext(ctx).Find(&items).Error
	if err != nil {
		return nil, d.err.New("查询所有配置项失败", err).DB()
	}
	return items, nil
}

// FindByKeys 根据多个配置键查询
func (d *ConfigItemDao) FindByKeys(ctx context.Context, keys []string) ([]*model.ConfigItemModel, error) {
	var items []*model.ConfigItemModel
	err := d.db.WithContext(ctx).Where("key IN ?", keys).Find(&items).Error
	if err != nil {
		return nil, d.err.New("批量查询配置项失败", err).DB()
	}
	return items, nil
}

// ExistsByKey 检查配置键是否存在
func (d *ConfigItemDao) ExistsByKey(ctx context.Context, key string) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.ConfigItemModel{}).Where("key = ?", key).Count(&count).Error
	if err != nil {
		return false, d.err.New("检查配置键是否存在失败", err).DB()
	}
	return count > 0, nil
}

// IncrementVersion 增加版本号
func (d *ConfigItemDao) IncrementVersion(ctx context.Context, id int64) error {
	err := d.db.WithContext(ctx).Model(&model.ConfigItemModel{}).
		Where("id = ?", id).
		UpdateColumn("version", gorm.Expr("version + 1")).Error
	if err != nil {
		return d.err.New("更新版本号失败", err).DB()
	}
	return nil
}

// WithTx 使用事务
func (d *ConfigItemDao) WithTx(tx *gorm.DB) *ConfigItemDao {
	return &ConfigItemDao{
		IBaseDao: mvc.NewGormDao[model.ConfigItemModel](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}
