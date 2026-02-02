package registry

import (
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/registry/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 执行注册中心组件的数据库迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始执行注册中心组件数据库迁移...")

	// 执行标准 AutoMigrate（此时 model 已是新结构）
	if err := db.AutoMigrate(
		&model.RegistryService{},
		&model.RegistryInstance{},
	); err != nil {
		log.WithErr(err).Error("注册中心组件数据库迁移失败")
		return err
	}

	log.Info("注册中心组件数据库迁移完成")
	return nil
}
