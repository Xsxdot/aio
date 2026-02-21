package service

import (
	"context"

	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
)

// ExecutorJobAttemptService 任务尝试记录服务层
type ExecutorJobAttemptService struct {
	dao *dao.ExecutorJobAttemptDAO
}

// NewExecutorJobAttemptService 创建任务尝试记录服务实例
func NewExecutorJobAttemptService() *ExecutorJobAttemptService {
	return &ExecutorJobAttemptService{
		dao: dao.NewExecutorJobAttemptDAO(),
	}
}

// ListByJobID 根据任务ID列出所有尝试记录
func (s *ExecutorJobAttemptService) ListByJobID(ctx context.Context, jobID uint64) ([]*model.ExecutorJobAttemptModel, error) {
	return s.dao.ListByJobID(ctx, jobID)
}
