package ssl

import (
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/ssl/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 执行 SSL 证书组件的数据库迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始执行 SSL 证书组件数据库迁移...")

	// 自动迁移所有模型
	if err := db.AutoMigrate(
		&model.DnsCredential{},
		&model.Certificate{},
		&model.DeployTarget{},
		&model.DeployHistory{},
	); err != nil {
		log.WithErr(err).Error("SSL 证书组件数据库迁移失败")
		return err
	}

	log.Info("SSL 证书组件数据库迁移完成")
	return nil
}
