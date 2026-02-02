package dao

import (
	"context"
	"time"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/ssl/internal/model"

	"gorm.io/gorm"
)

// CertificateDao 证书数据访问层
type CertificateDao struct {
	mvc.IBaseDao[model.Certificate]
	db  *gorm.DB
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewCertificateDao 创建证书 DAO 实例
func NewCertificateDao(db *gorm.DB, log *logger.Log) *CertificateDao {
	return &CertificateDao{
		IBaseDao: mvc.NewGormDao[model.Certificate](db),
		db:       db,
		log:      log.WithEntryName("CertificateDao"),
		err:      errorc.NewErrorBuilder("CertificateDao"),
	}
}

// FindCertificatesToRenew 查询需要续期的证书
// 查询条件：auto_renew=1 且 expires_at - now <= renew_before_days
func (d *CertificateDao) FindCertificatesToRenew(ctx context.Context) ([]model.Certificate, error) {
	var certificates []model.Certificate
	now := time.Now()

	// 查询条件：
	// 1. 自动续期开启
	// 2. 状态为 active
	// 3. 过期时间 - 当前时间 <= 续期提前天数
	err := d.db.WithContext(ctx).
		Where("auto_renew = ?", 1).
		Where("status = ?", model.CertificateStatusActive).
		Where("expires_at IS NOT NULL").
		Where("TIMESTAMPDIFF(DAY, ?, expires_at) <= renew_before_days", now).
		Find(&certificates).Error

	if err != nil {
		d.log.WithErr(err).Error("查询需要续期的证书失败")
		return nil, d.err.New("查询需要续期的证书失败", err)
	}

	return certificates, nil
}

// FindByStatus 根据状态查询证书列表
func (d *CertificateDao) FindByStatus(ctx context.Context, status model.CertificateStatus) ([]model.Certificate, error) {
	var certificates []model.Certificate
	err := d.db.WithContext(ctx).Where("status = ?", status).Find(&certificates).Error
	if err != nil {
		d.log.WithErr(err).WithField("status", status).Error("根据状态查询证书失败")
		return nil, d.err.New("根据状态查询证书失败", err)
	}
	return certificates, nil
}

// UpdateStatus 更新证书状态
func (d *CertificateDao) UpdateStatus(ctx context.Context, id uint, status model.CertificateStatus) error {
	err := d.db.WithContext(ctx).Model(&model.Certificate{}).
		Where("id = ?", id).
		Update("status", status).Error
	if err != nil {
		d.log.WithErr(err).WithField("id", id).WithField("status", status).Error("更新证书状态失败")
		return d.err.New("更新证书状态失败", err)
	}
	return nil
}
