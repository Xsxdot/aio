package dao

import (
	"context"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/server/internal/model"

	"gorm.io/gorm"
)

// ServerSSHCredentialDao SSH 凭证数据访问层
type ServerSSHCredentialDao struct {
	mvc.IBaseDao[model.ServerSSHCredential]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewServerSSHCredentialDao 创建 SSH 凭证 DAO 实例
func NewServerSSHCredentialDao(db *gorm.DB, log *logger.Log) *ServerSSHCredentialDao {
	return &ServerSSHCredentialDao{
		IBaseDao: mvc.NewGormDao[model.ServerSSHCredential](db),
		log:      log.WithEntryName("ServerSSHCredentialDao"),
		err:      errorc.NewErrorBuilder("ServerSSHCredentialDao"),
		db:       db,
	}
}

// FindByServerID 根据服务器 ID 查询 SSH 凭证
func (d *ServerSSHCredentialDao) FindByServerID(ctx context.Context, serverID int64) (*model.ServerSSHCredential, error) {
	var credential model.ServerSSHCredential
	err := d.db.WithContext(ctx).Where("server_id = ?", serverID).First(&credential).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("SSH凭证不存在", err).NotFound()
		}
		return nil, d.err.New("查询SSH凭证失败", err).DB()
	}
	return &credential, nil
}

// UpsertByServerID 根据服务器 ID 更新或插入 SSH 凭证
func (d *ServerSSHCredentialDao) UpsertByServerID(ctx context.Context, credential *model.ServerSSHCredential) error {
	// 先查询是否存在
	var existing model.ServerSSHCredential
	err := d.db.WithContext(ctx).Where("server_id = ?", credential.ServerID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// 不存在，插入
		if err := d.db.WithContext(ctx).Create(credential).Error; err != nil {
			return d.err.New("创建SSH凭证失败", err).DB()
		}
		return nil
	}

	if err != nil {
		return d.err.New("查询SSH凭证失败", err).DB()
	}

	// 存在，更新
	credential.ID = existing.ID
	if err := d.db.WithContext(ctx).Model(&existing).Updates(credential).Error; err != nil {
		return d.err.New("更新SSH凭证失败", err).DB()
	}

	return nil
}

// DeleteByServerID 根据服务器 ID 删除 SSH 凭证
func (d *ServerSSHCredentialDao) DeleteByServerID(ctx context.Context, serverID int64) error {
	err := d.db.WithContext(ctx).Where("server_id = ?", serverID).Delete(&model.ServerSSHCredential{}).Error
	if err != nil {
		return d.err.New("删除SSH凭证失败", err).DB()
	}
	return nil
}
