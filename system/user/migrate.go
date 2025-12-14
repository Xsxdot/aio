package user

import (
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/user/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始迁移用户组件数据库表...")

	// 迁移管理员表
	if err := db.AutoMigrate(&model.Admin{}); err != nil {
		log.WithErr(err).Error("迁移管理员表失败")
		return err
	}

	// 迁移客户端凭证表
	if err := db.AutoMigrate(&model.ClientCredential{}); err != nil {
		log.WithErr(err).Error("迁移客户端凭证表失败")
		return err
	}

	log.Info("用户组件数据库表迁移完成")
	return nil
}



