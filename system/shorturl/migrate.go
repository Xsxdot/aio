package shorturl

import (
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/shorturl/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动执行短网址组件的数据库迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始迁移短网址组件表...")

	// 迁移所有表
	if err := db.AutoMigrate(
		&model.ShortDomain{},
		&model.ShortLink{},
		&model.ShortVisit{},
		&model.ShortSuccessEvent{},
	); err != nil {
		log.WithErr(err).Error("短网址组件表迁移失败")
		return err
	}

	log.Info("短网址组件表迁移完成")
	return nil
}
