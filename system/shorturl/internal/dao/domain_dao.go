package dao

import (
	"context"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/shorturl/internal/model"

	"gorm.io/gorm"
)

// DomainDao 短域名数据访问层
type DomainDao struct {
	mvc.IBaseDao[model.ShortDomain]
	log *logger.Log
	err *errorc.ErrorBuilder
	DB  *gorm.DB
}

// NewDomainDao 创建短域名 DAO 实例
func NewDomainDao(db *gorm.DB, log *logger.Log) *DomainDao {
	return &DomainDao{
		IBaseDao: mvc.NewGormDao[model.ShortDomain](db),
		log:      log.WithEntryName("DomainDao"),
		err:      errorc.NewErrorBuilder("DomainDao"),
		DB:       db,
	}
}

// FindByDomain 根据域名查找
func (d *DomainDao) FindByDomain(ctx context.Context, domain string) (*model.ShortDomain, error) {
	var result model.ShortDomain
	err := d.DB.WithContext(ctx).Where("domain = ?", domain).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("域名不存在", err).NotFound()
		}
		return nil, d.err.New("查询域名失败", err).DB()
	}
	return &result, nil
}

// FindDefault 查找默认域名
func (d *DomainDao) FindDefault(ctx context.Context) (*model.ShortDomain, error) {
	var result model.ShortDomain
	err := d.DB.WithContext(ctx).Where("is_default = ? AND enabled = ?", true, true).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("未配置默认域名", err).NotFound()
		}
		return nil, d.err.New("查询默认域名失败", err).DB()
	}
	return &result, nil
}

// ListEnabled 查询所有启用的域名
func (d *DomainDao) ListEnabled(ctx context.Context) ([]*model.ShortDomain, error) {
	var results []*model.ShortDomain
	err := d.DB.WithContext(ctx).Where("enabled = ?", true).Find(&results).Error
	if err != nil {
		return nil, d.err.New("查询域名列表失败", err).DB()
	}
	return results, nil
}


