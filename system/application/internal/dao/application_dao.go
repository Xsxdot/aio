package dao

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/application/internal/model"

	"gorm.io/gorm"
)

// ApplicationDao 应用数据访问层
type ApplicationDao struct {
	mvc.IBaseDao[model.Application]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewApplicationDao 创建应用 DAO 实例
func NewApplicationDao(db *gorm.DB, log *logger.Log) *ApplicationDao {
	return &ApplicationDao{
		IBaseDao: mvc.NewGormDao[model.Application](db),
		log:      log.WithEntryName("ApplicationDao"),
		err:      errorc.NewErrorBuilder("ApplicationDao"),
		db:       db,
	}
}

// FindByKey 根据唯一键查找应用
func (d *ApplicationDao) FindByKey(ctx context.Context, project, name, env string) (*model.Application, error) {
	var app model.Application
	err := d.db.WithContext(ctx).
		Where("project = ? AND name = ? AND env = ?", project, name, env).
		First(&app).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("应用不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询应用失败", err).DB()
	}
	return &app, nil
}

// ExistsByKey 检查应用是否存在
func (d *ApplicationDao) ExistsByKey(ctx context.Context, project, name, env string) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.Application{}).
		Where("project = ? AND name = ? AND env = ?", project, name, env).
		Count(&count).Error
	if err != nil {
		return false, d.err.New("检查应用是否存在失败", err).DB()
	}
	return count > 0, nil
}

// ListByFilter 根据条件查询应用列表
func (d *ApplicationDao) ListByFilter(ctx context.Context, project, env, appType, keyword string) ([]*model.Application, error) {
	var list []*model.Application
	q := d.db.WithContext(ctx).Model(&model.Application{})
	if project != "" {
		q = q.Where("project = ?", project)
	}
	if env != "" {
		q = q.Where("env = ?", env)
	}
	if appType != "" {
		q = q.Where("type = ?", appType)
	}
	if keyword != "" {
		q = q.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		return nil, d.err.New("查询应用列表失败", err).DB()
	}
	return list, nil
}

// WithTx 使用事务
func (d *ApplicationDao) WithTx(tx *gorm.DB) *ApplicationDao {
	return &ApplicationDao{
		IBaseDao: mvc.NewGormDao[model.Application](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}

