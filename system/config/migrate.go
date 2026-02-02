package config

import (
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/config/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 执行配置中心的数据库迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始迁移配置中心表...")

	if err := db.AutoMigrate(&model.ConfigItemModel{}); err != nil {
		log.WithErr(err).Error("迁移 config_items 表失败")
		return err
	}
	log.Info("迁移 config_items 表成功")

	if err := db.AutoMigrate(&model.ConfigHistoryModel{}); err != nil {
		log.WithErr(err).Error("迁移 config_history 表失败")
		return err
	}
	log.Info("迁移 config_history 表成功")

	return nil
}
