package service

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/internal/dao"
	"xiaozhizhang/system/application/internal/model"
)

// ArtifactService 产物服务
type ArtifactService struct {
	dao *dao.ArtifactDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewArtifactService 创建产物服务实例
func NewArtifactService(dao *dao.ArtifactDao, log *logger.Log) *ArtifactService {
	return &ArtifactService{
		dao: dao,
		log: log.WithEntryName("ArtifactService"),
		err: errorc.NewErrorBuilder("ArtifactService"),
	}
}

// Create 创建产物记录
func (s *ArtifactService) Create(ctx context.Context, artifact *model.Artifact) error {
	return s.dao.Create(ctx, artifact)
}

// FindByID 根据 ID 查找产物
func (s *ArtifactService) FindByID(ctx context.Context, id int64) (*model.Artifact, error) {
	return s.dao.FindById(ctx, id)
}

// ListByApplicationID 根据应用 ID 查询产物列表
func (s *ArtifactService) ListByApplicationID(ctx context.Context, applicationID int64) ([]*model.Artifact, error) {
	return s.dao.ListByApplicationID(ctx, applicationID)
}

// Delete 删除产物记录
func (s *ArtifactService) Delete(ctx context.Context, id int64) error {
	return s.dao.DeleteById(ctx, id)
}

