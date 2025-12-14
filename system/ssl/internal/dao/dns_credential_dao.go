package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/ssl/internal/model"

	"gorm.io/gorm"
)

// DnsCredentialDao DNS 凭证数据访问层
type DnsCredentialDao struct {
	mvc.IBaseDao[model.DnsCredential]
	db  *gorm.DB
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewDnsCredentialDao 创建 DNS 凭证 DAO 实例
func NewDnsCredentialDao(db *gorm.DB, log *logger.Log) *DnsCredentialDao {
	return &DnsCredentialDao{
		IBaseDao: mvc.NewGormDao[model.DnsCredential](db),
		db:       db,
		log:      log.WithEntryName("DnsCredentialDao"),
		err:      errorc.NewErrorBuilder("DnsCredentialDao"),
	}
}

// FindByProvider 根据服务商类型查询凭证列表
func (d *DnsCredentialDao) FindByProvider(ctx context.Context, provider model.DnsProvider) ([]model.DnsCredential, error) {
	var credentials []model.DnsCredential
	err := d.db.WithContext(ctx).Where("provider = ? AND status = ?", provider, 1).Find(&credentials).Error
	if err != nil {
		d.log.WithErr(err).WithField("provider", provider).Error("根据服务商查询凭证失败")
		return nil, d.err.New("根据服务商查询凭证失败", err)
	}
	return credentials, nil
}

// FindActiveCredentials 查询所有启用的凭证
func (d *DnsCredentialDao) FindActiveCredentials(ctx context.Context) ([]model.DnsCredential, error) {
	var credentials []model.DnsCredential
	err := d.db.WithContext(ctx).Where("status = ?", 1).Find(&credentials).Error
	if err != nil {
		d.log.WithErr(err).Error("查询启用凭证失败")
		return nil, d.err.New("查询启用凭证失败", err)
	}
	return credentials, nil
}
