package service

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/internal/dao"
	"xiaozhizhang/system/application/internal/model"
)

// ReleaseService 版本服务
type ReleaseService struct {
	dao *dao.ReleaseDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewReleaseService 创建版本服务实例
func NewReleaseService(dao *dao.ReleaseDao, log *logger.Log) *ReleaseService {
	return &ReleaseService{
		dao: dao,
		log: log.WithEntryName("ReleaseService"),
		err: errorc.NewErrorBuilder("ReleaseService"),
	}
}

// Create 创建版本
func (s *ReleaseService) Create(ctx context.Context, release *model.Release) error {
	if release.Status == "" {
		release.Status = model.ReleaseStatusPending
	}
	return s.dao.Create(ctx, release)
}

// FindByID 根据 ID 查找版本
func (s *ReleaseService) FindByID(ctx context.Context, id int64) (*model.Release, error) {
	return s.dao.FindById(ctx, id)
}

// ListByApplicationID 根据应用 ID 查询版本列表
func (s *ReleaseService) ListByApplicationID(ctx context.Context, applicationID int64, limit int) ([]*model.Release, error) {
	return s.dao.ListByApplicationID(ctx, applicationID, limit)
}

// FindActiveByApplicationID 查找当前活动版本
func (s *ReleaseService) FindActiveByApplicationID(ctx context.Context, applicationID int64) (*model.Release, error) {
	return s.dao.FindActiveByApplicationID(ctx, applicationID)
}

// UpdateStatus 更新版本状态
func (s *ReleaseService) UpdateStatus(ctx context.Context, id int64, status model.ReleaseStatus) error {
	return s.dao.UpdateStatus(ctx, id, status)
}

// MarkAsActive 标记版本为活动状态，并将其他版本标记为历史
func (s *ReleaseService) MarkAsActive(ctx context.Context, applicationID int64, releaseID int64) error {
	// 先将其他活动版本标记为历史
	if err := s.dao.MarkSuperseded(ctx, applicationID, releaseID); err != nil {
		return err
	}
	// 标记当前版本为活动
	return s.dao.UpdateStatus(ctx, releaseID, model.ReleaseStatusActive)
}

// UpdateReleasePath 更新版本的解压路径
func (s *ReleaseService) UpdateReleasePath(ctx context.Context, id int64, path string) error {
	_, err := s.dao.UpdateById(ctx, id, &model.Release{ReleasePath: path})
	return err
}

