package workflow

import (
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 数据库自动迁移
func AutoMigrate(db *gorm.DB, log *logger.Log) error {
	log.GetLogger().Info("Running workflow database migrations...")

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
