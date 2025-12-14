package dao

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/application/internal/model"

	"gorm.io/gorm"
)

// ReleaseDao 版本数据访问层
type ReleaseDao struct {
	mvc.IBaseDao[model.Release]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewReleaseDao 创建版本 DAO 实例
func NewReleaseDao(db *gorm.DB, log *logger.Log) *ReleaseDao {
	return &ReleaseDao{
		IBaseDao: mvc.NewGormDao[model.Release](db),
		log:      log.WithEntryName("ReleaseDao"),
		err:      errorc.NewErrorBuilder("ReleaseDao"),
		db:       db,
	}
}

// ListByApplicationID 根据应用 ID 查询版本列表
func (d *ReleaseDao) ListByApplicationID(ctx context.Context, applicationID int64, limit int) ([]*model.Release, error) {
	var list []*model.Release
	q := d.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, d.err.New("查询版本列表失败", err).DB()
	}
	return list, nil
}

// FindActiveByApplicationID 查找当前活动版本
func (d *ReleaseDao) FindActiveByApplicationID(ctx context.Context, applicationID int64) (*model.Release, error) {
	var release model.Release
	err := d.db.WithContext(ctx).
		Where("application_id = ? AND status = ?", applicationID, model.ReleaseStatusActive).
		First(&release).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 没有活动版本不算错误
		}
		return nil, d.err.New("查询活动版本失败", err).DB()
	}
	return &release, nil
}

// UpdateStatus 更新版本状态
func (d *ReleaseDao) UpdateStatus(ctx context.Context, id int64, status model.ReleaseStatus) error {
	res := d.db.WithContext(ctx).Model(&model.Release{}).
		Where("id = ?", id).
		Update("status", status)
	if res.Error != nil {
		return d.err.New("更新版本状态失败", res.Error).DB()
	}
	return nil
}

// MarkSuperseded 将指定应用的所有 active 版本标记为 superseded
func (d *ReleaseDao) MarkSuperseded(ctx context.Context, applicationID int64, excludeID int64) error {
	res := d.db.WithContext(ctx).Model(&model.Release{}).
		Where("application_id = ? AND status = ? AND id != ?", applicationID, model.ReleaseStatusActive, excludeID).
		Update("status", model.ReleaseStatusSuperseded)
	if res.Error != nil {
		return d.err.New("标记历史版本失败", res.Error).DB()
	}
	return nil
}

// WithTx 使用事务
func (d *ReleaseDao) WithTx(tx *gorm.DB) *ReleaseDao {
	return &ReleaseDao{
		IBaseDao: mvc.NewGormDao[model.Release](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}

