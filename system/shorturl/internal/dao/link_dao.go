package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/shorturl/internal/model"

	"gorm.io/gorm"
)

// LinkDao 短链接数据访问层
type LinkDao struct {
	mvc.IBaseDao[model.ShortLink]
	log *logger.Log
	err *errorc.ErrorBuilder
	DB  *gorm.DB
}

// NewLinkDao 创建短链接 DAO 实例
func NewLinkDao(db *gorm.DB, log *logger.Log) *LinkDao {
	return &LinkDao{
		IBaseDao: mvc.NewGormDao[model.ShortLink](db),
		log:      log.WithEntryName("LinkDao"),
		err:      errorc.NewErrorBuilder("LinkDao"),
		DB:       db,
	}
}

// FindByDomainAndCode 根据域名ID和短码查找
func (d *LinkDao) FindByDomainAndCode(ctx context.Context, domainID int64, code string) (*model.ShortLink, error) {
	var result model.ShortLink
	err := d.DB.WithContext(ctx).Where("domain_id = ? AND code = ?", domainID, code).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("短链接不存在", err).NotFound()
		}
		return nil, d.err.New("查询短链接失败", err).DB()
	}
	return &result, nil
}

// FindByCode 根据短码查找（任意域名）
func (d *LinkDao) FindByCode(ctx context.Context, code string) (*model.ShortLink, error) {
	var result model.ShortLink
	err := d.DB.WithContext(ctx).Where("code = ?", code).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("短链接不存在", err).NotFound()
		}
		return nil, d.err.New("查询短链接失败", err).DB()
	}
	return &result, nil
}

// ExistsByDomainAndCode 检查短码是否已存在
func (d *LinkDao) ExistsByDomainAndCode(ctx context.Context, domainID int64, code string) (bool, error) {
	var count int64
	err := d.DB.WithContext(ctx).Model(&model.ShortLink{}).
		Where("domain_id = ? AND code = ?", domainID, code).Count(&count).Error
	if err != nil {
		return false, d.err.New("检查短码是否存在失败", err).DB()
	}
	return count > 0, nil
}

// IncrementVisitCount 原子递增访问次数
func (d *LinkDao) IncrementVisitCount(ctx context.Context, id int64) error {
	err := d.DB.WithContext(ctx).Model(&model.ShortLink{}).
		Where("id = ?", id).
		UpdateColumn("visit_count", gorm.Expr("visit_count + ?", 1)).Error
	if err != nil {
		return d.err.New("更新访问次数失败", err).DB()
	}
	return nil
}

// IncrementSuccessCount 原子递增成功次数
func (d *LinkDao) IncrementSuccessCount(ctx context.Context, id int64) error {
	err := d.DB.WithContext(ctx).Model(&model.ShortLink{}).
		Where("id = ?", id).
		UpdateColumn("success_count", gorm.Expr("success_count + ?", 1)).Error
	if err != nil {
		return d.err.New("更新成功次数失败", err).DB()
	}
	return nil
}

// ListByDomainWithPage 分页查询域名下的短链接
func (d *LinkDao) ListByDomainWithPage(ctx context.Context, domainID int64, pageNum, pageSize int) ([]*model.ShortLink, int64, error) {
	var results []*model.ShortLink
	var total int64

	query := d.DB.WithContext(ctx).Model(&model.ShortLink{}).Where("domain_id = ?", domainID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, d.err.New("统计短链接数量失败", err).DB()
	}

	offset := (pageNum - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&results).Error
	if err != nil {
		return nil, 0, d.err.New("分页查询短链接失败", err).DB()
	}

	return results, total, nil
}

