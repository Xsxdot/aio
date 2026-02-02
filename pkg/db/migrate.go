package db

import (
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/registry"
	"xiaozhizhang/system/server"
	"xiaozhizhang/system/shorturl"
	"xiaozhizhang/system/ssl"
	"xiaozhizhang/system/user"

	"gorm.io/gorm"
)

// AutoMigrate 自动执行所有数据库迁移
func AutoMigrate(db *gorm.DB) error {
	log := logger.GetLogger().WithEntryName("DatabaseMigration")

	log.Info("开始执行数据库迁移...")

	// 用户组件表迁移（管理员、客户端凭证）
	if err := user.AutoMigrate(db, log); err != nil {
		return err
	}

	// 配置中心表迁移
	if err := config.AutoMigrate(db, log); err != nil {
		return err
	}

	// SSL 证书组件表迁移
	if err := ssl.AutoMigrate(db, log); err != nil {
		return err
	}

	// 注册中心组件表迁移
	if err := registry.AutoMigrate(db, log); err != nil {
		return err
	}

	// Server 管理组件表迁移
	if err := server.AutoMigrate(db, log); err != nil {
		return err
	}

	// 短网址组件表迁移
	if err := shorturl.AutoMigrate(db, log); err != nil {
		return err
	}

	log.Info("所有数据库迁移执行完成")
	return nil
}
