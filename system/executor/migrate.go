package executor

import (
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/executor/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 执行任务执行器的数据库迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.Info("开始迁移任务执行器表...")

	// 迁移任务主表
	if err := db.AutoMigrate(&model.ExecutorJobModel{}); err != nil {
		log.WithErr(err).Error("迁移 executor_jobs 表失败")
		return err
	}
	log.Info("迁移 executor_jobs 表成功")

	// 迁移任务尝试记录表
	if err := db.AutoMigrate(&model.ExecutorJobAttemptModel{}); err != nil {
		log.WithErr(err).Error("迁移 executor_job_attempts 表失败")
		return err
	}
	log.Info("迁移 executor_job_attempts 表成功")

	return nil
}
