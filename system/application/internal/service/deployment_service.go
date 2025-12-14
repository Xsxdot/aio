package service

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/internal/dao"
	"xiaozhizhang/system/application/internal/model"
)

// DeploymentService 部署服务
type DeploymentService struct {
	dao *dao.DeploymentDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewDeploymentService 创建部署服务实例
func NewDeploymentService(dao *dao.DeploymentDao, log *logger.Log) *DeploymentService {
	return &DeploymentService{
		dao: dao,
		log: log.WithEntryName("DeploymentService"),
		err: errorc.NewErrorBuilder("DeploymentService"),
	}
}

// Create 创建部署记录
func (s *DeploymentService) Create(ctx context.Context, deployment *model.Deployment) error {
	if deployment.Status == "" {
		deployment.Status = model.DeploymentStatusPending
	}
	return s.dao.Create(ctx, deployment)
}

// FindByID 根据 ID 查找部署记录
func (s *DeploymentService) FindByID(ctx context.Context, id int64) (*model.Deployment, error) {
	return s.dao.FindById(ctx, id)
}

// ListByApplicationID 根据应用 ID 查询部署记录
func (s *DeploymentService) ListByApplicationID(ctx context.Context, applicationID int64, limit int) ([]*model.Deployment, error) {
	return s.dao.ListByApplicationID(ctx, applicationID, limit)
}

// UpdateStatus 更新部署状态
func (s *DeploymentService) UpdateStatus(ctx context.Context, id int64, status model.DeploymentStatus, errorMsg string) error {
	return s.dao.UpdateStatus(ctx, id, status, errorMsg)
}

// AppendLog 追加部署日志
func (s *DeploymentService) AppendLog(ctx context.Context, id int64, logEntry string) error {
	return s.dao.AppendLog(ctx, id, logEntry)
}

// MarkRunning 标记为运行中
func (s *DeploymentService) MarkRunning(ctx context.Context, id int64) error {
	return s.UpdateStatus(ctx, id, model.DeploymentStatusRunning, "")
}

// MarkSuccess 标记为成功
func (s *DeploymentService) MarkSuccess(ctx context.Context, id int64) error {
	return s.UpdateStatus(ctx, id, model.DeploymentStatusSuccess, "")
}

// MarkFailed 标记为失败
func (s *DeploymentService) MarkFailed(ctx context.Context, id int64, errorMsg string) error {
	return s.UpdateStatus(ctx, id, model.DeploymentStatusFailed, errorMsg)
}

