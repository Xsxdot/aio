package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/ssl/internal/model"

	"gorm.io/gorm"
)

// DeployHistoryDao 部署历史数据访问层
type DeployHistoryDao struct {
	mvc.IBaseDao[model.DeployHistory]
	db  *gorm.DB
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewDeployHistoryDao 创建部署历史 DAO 实例
func NewDeployHistoryDao(db *gorm.DB, log *logger.Log) *DeployHistoryDao {
	return &DeployHistoryDao{
		IBaseDao: mvc.NewGormDao[model.DeployHistory](db),
		db:       db,
		log:      log.WithEntryName("DeployHistoryDao"),
		err:      errorc.NewErrorBuilder("DeployHistoryDao"),
	}
}

// FindByCertificateID 根据证书 ID 查询部署历史
func (d *DeployHistoryDao) FindByCertificateID(ctx context.Context, certificateID uint, limit int) ([]model.DeployHistory, error) {
	var histories []model.DeployHistory
	query := d.db.WithContext(ctx).Where("certificate_id = ?", certificateID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	if err != nil {
		d.log.WithErr(err).WithField("certificate_id", certificateID).Error("根据证书ID查询部署历史失败")
		return nil, d.err.New("根据证书ID查询部署历史失败", err)
	}
	return histories, nil
}

// FindByDeployTargetID 根据部署目标 ID 查询部署历史
func (d *DeployHistoryDao) FindByDeployTargetID(ctx context.Context, deployTargetID uint, limit int) ([]model.DeployHistory, error) {
	var histories []model.DeployHistory
	query := d.db.WithContext(ctx).Where("deploy_target_id = ?", deployTargetID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	if err != nil {
		d.log.WithErr(err).WithField("deploy_target_id", deployTargetID).Error("根据部署目标ID查询部署历史失败")
		return nil, d.err.New("根据部署目标ID查询部署历史失败", err)
	}
	return histories, nil
}

// FindByStatus 根据状态查询部署历史
func (d *DeployHistoryDao) FindByStatus(ctx context.Context, status model.DeployStatus, limit int) ([]model.DeployHistory, error) {
	var histories []model.DeployHistory
	query := d.db.WithContext(ctx).Where("status = ?", status).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	if err != nil {
		d.log.WithErr(err).WithField("status", status).Error("根据状态查询部署历史失败")
		return nil, d.err.New("根据状态查询部署历史失败", err)
	}
	return histories, nil
}
