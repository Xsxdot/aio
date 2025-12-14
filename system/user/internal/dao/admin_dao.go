package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/user/internal/model"

	"gorm.io/gorm"
)

// AdminDao 管理员数据访问层
type AdminDao struct {
	mvc.IBaseDao[model.Admin]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewAdminDao 创建管理员DAO实例
func NewAdminDao(db *gorm.DB, log *logger.Log) *AdminDao {
	return &AdminDao{
		IBaseDao: mvc.NewGormDao[model.Admin](db),
		log:      log,
		err:      errorc.NewErrorBuilder("AdminDao"),
		db:       db,
	}
}

// FindByAccount 根据账号查询管理员
func (d *AdminDao) FindByAccount(ctx context.Context, account string) (*model.Admin, error) {
	var admin model.Admin
	err := d.db.WithContext(ctx).Where("account = ?", account).First(&admin).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("管理员不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询管理员失败", err).DB()
	}
	return &admin, nil
}

// ExistsByAccount 检查账号是否存在
func (d *AdminDao) ExistsByAccount(ctx context.Context, account string) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.Admin{}).Where("account = ?", account).Count(&count).Error
	if err != nil {
		return false, d.err.New("检查账号是否存在失败", err).DB()
	}
	return count > 0, nil
}

// UpdatePassword 更新管理员密码
func (d *AdminDao) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	err := d.db.WithContext(ctx).Model(&model.Admin{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash).Error
	if err != nil {
		return d.err.New("更新密码失败", err).DB()
	}
	return nil
}

// UpdateStatus 更新管理员状态
func (d *AdminDao) UpdateStatus(ctx context.Context, id int64, status int8) error {
	err := d.db.WithContext(ctx).Model(&model.Admin{}).
		Where("id = ?", id).
		Update("status", status).Error
	if err != nil {
		return d.err.New("更新状态失败", err).DB()
	}
	return nil
}

// FindAllActive 查询所有启用的管理员
func (d *AdminDao) FindAllActive(ctx context.Context) ([]*model.Admin, error) {
	var admins []*model.Admin
	err := d.db.WithContext(ctx).Where("status = ?", model.AdminStatusEnabled).Find(&admins).Error
	if err != nil {
		return nil, d.err.New("查询启用管理员失败", err).DB()
	}
	return admins, nil
}

// Count 查询管理员总数
func (d *AdminDao) Count(ctx context.Context) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.Admin{}).Count(&count).Error
	if err != nil {
		return 0, d.err.New("查询管理员数量失败", err).DB()
	}
	return count, nil
}

// WithTx 使用事务
func (d *AdminDao) WithTx(tx *gorm.DB) *AdminDao {
	return &AdminDao{
		IBaseDao: mvc.NewGormDao[model.Admin](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}

