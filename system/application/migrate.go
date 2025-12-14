package application

import (
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 执行 Application 组件的数据库迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始执行 Application 组件数据库迁移...")

	err := db.AutoMigrate(
		&model.Application{},
		&model.Artifact{},
		&model.Release{},
		&model.Deployment{},
	)
	if err != nil {
		log.WithErr(err).Error("Application 组件数据库迁移失败")
		return err
	}

	log.Info("Application 组件数据库迁移完成")
	return nil
}

