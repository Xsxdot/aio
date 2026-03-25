package workflow

import (
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 数据库自动迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.GetLogger().Info("Running workflow database migrations...")

	// 删除旧索引（Env 字段新增后索引名变化）
	if db.Migrator().HasIndex(&model.WorkflowDefModel{}, "idx_code_version") {
		if err := db.Migrator().DropIndex(&model.WorkflowDefModel{}, "idx_code_version"); err != nil {
			log.GetLogger().WithError(err).Warn("Failed to drop old index idx_code_version")
		}
	}

	err := db.AutoMigrate(
		&model.WorkflowDefModel{},
		&model.WorkflowInstanceModel{},
		&model.WorkflowCheckpointModel{},
	)

	if err != nil {
		log.GetLogger().WithError(err).Error("Failed to run workflow migrations")
		return err
	}

	log.GetLogger().Info("Workflow migrations completed successfully")
	return nil
}
