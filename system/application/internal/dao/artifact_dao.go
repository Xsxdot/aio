package dao

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/application/internal/model"

	"gorm.io/gorm"
)

// ArtifactDao 产物数据访问层
type ArtifactDao struct {
	mvc.IBaseDao[model.Artifact]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewArtifactDao 创建产物 DAO 实例
func NewArtifactDao(db *gorm.DB, log *logger.Log) *ArtifactDao {
	return &ArtifactDao{
		IBaseDao: mvc.NewGormDao[model.Artifact](db),
		log:      log.WithEntryName("ArtifactDao"),
		err:      errorc.NewErrorBuilder("ArtifactDao"),
		db:       db,
	}
}

// ListByApplicationID 根据应用 ID 查询产物列表
func (d *ArtifactDao) ListByApplicationID(ctx context.Context, applicationID int64) ([]*model.Artifact, error) {
	var list []*model.Artifact
	err := d.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, d.err.New("查询产物列表失败", err).DB()
	}
	return list, nil
}

// WithTx 使用事务
func (d *ArtifactDao) WithTx(tx *gorm.DB) *ArtifactDao {
	return &ArtifactDao{
		IBaseDao: mvc.NewGormDao[model.Artifact](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}

