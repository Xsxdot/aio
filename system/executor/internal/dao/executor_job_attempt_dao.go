package dao

import (
	"context"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/executor/internal/model"

	"gorm.io/gorm"
)

// ExecutorJobAttemptDAO 任务尝试记录数据访问层
type ExecutorJobAttemptDAO struct {
	db *gorm.DB
}

// NewExecutorJobAttemptDAO 创建任务尝试记录DAO实例
func NewExecutorJobAttemptDAO() *ExecutorJobAttemptDAO {
	return &ExecutorJobAttemptDAO{
		db: base.DB,
	}
}

// Create 创建尝试记录
func (d *ExecutorJobAttemptDAO) Create(ctx context.Context, attempt *model.ExecutorJobAttemptModel) error {
	return mvc.ExtractDB(ctx, d.db).Create(attempt).Error
}

// ListByJobID 根据任务ID列出所有尝试记录
func (d *ExecutorJobAttemptDAO) ListByJobID(ctx context.Context, jobID uint64) ([]*model.ExecutorJobAttemptModel, error) {
	var attempts []*model.ExecutorJobAttemptModel
	err := mvc.ExtractDB(ctx, d.db).
		Where("job_id = ?", jobID).
		Order("attempt_no DESC").
		Find(&attempts).Error
	return attempts, err
}
