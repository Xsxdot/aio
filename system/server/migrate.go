package server

import (
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/server/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移 server 组件的数据库表
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始迁移 server 组件表...")

	// 迁移服务器表
	if err := db.AutoMigrate(&model.ServerModel{}); err != nil {
		log.WithErr(err).Error("迁移 server_servers 表失败")
		return err
	}
	log.Info("server_servers 表迁移成功")

	// 迁移服务器状态表
	if err := db.AutoMigrate(&model.ServerStatusModel{}); err != nil {
		log.WithErr(err).Error("迁移 server_status 表失败")
		return err
	}
	log.Info("server_status 表迁移成功")

	log.Info("server 组件表迁移完成")
	return nil
}


