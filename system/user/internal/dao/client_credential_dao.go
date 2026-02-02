package dao

import (
	"context"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/user/internal/model"

	"gorm.io/gorm"
)

// ClientCredentialDao 客户端凭证数据访问层
type ClientCredentialDao struct {
	mvc.IBaseDao[model.ClientCredential]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewClientCredentialDao 创建客户端凭证DAO实例
func NewClientCredentialDao(db *gorm.DB, log *logger.Log) *ClientCredentialDao {
	return &ClientCredentialDao{
		IBaseDao: mvc.NewGormDao[model.ClientCredential](db),
		log:      log,
		err:      errorc.NewErrorBuilder("ClientCredentialDao"),
		db:       db,
	}
}

// FindByClientKey 根据客户端 key 查询
func (d *ClientCredentialDao) FindByClientKey(ctx context.Context, clientKey string) (*model.ClientCredential, error) {
	var client model.ClientCredential
	err := d.db.WithContext(ctx).Where("client_key = ?", clientKey).First(&client).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("客户端不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询客户端失败", err).DB()
	}
	return &client, nil
}

// ExistsByClientKey 检查客户端 key 是否存在
func (d *ClientCredentialDao) ExistsByClientKey(ctx context.Context, clientKey string) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.ClientCredential{}).
		Where("client_key = ?", clientKey).Count(&count).Error
	if err != nil {
		return false, d.err.New("检查客户端 key 是否存在失败", err).DB()
	}
	return count > 0, nil
}

// UpdateSecret 更新客户端 secret
func (d *ClientCredentialDao) UpdateSecret(ctx context.Context, id int64, secretHash string) error {
	err := d.db.WithContext(ctx).Model(&model.ClientCredential{}).
		Where("id = ?", id).
		Update("client_secret", secretHash).Error
	if err != nil {
		return d.err.New("更新客户端 secret 失败", err).DB()
	}
	return nil
}

// UpdateStatus 更新客户端状态
func (d *ClientCredentialDao) UpdateStatus(ctx context.Context, id int64, status int8) error {
	err := d.db.WithContext(ctx).Model(&model.ClientCredential{}).
		Where("id = ?", id).
		Update("status", status).Error
	if err != nil {
		return d.err.New("更新状态失败", err).DB()
	}
	return nil
}

// FindAllActive 查询所有启用且未过期的客户端
func (d *ClientCredentialDao) FindAllActive(ctx context.Context) ([]*model.ClientCredential, error) {
	var clients []*model.ClientCredential
	err := d.db.WithContext(ctx).
		Where("status = ?", model.ClientCredentialStatusEnabled).
		Where("(expires_at IS NULL OR expires_at > NOW())").
		Find(&clients).Error
	if err != nil {
		return nil, d.err.New("查询启用客户端失败", err).DB()
	}
	return clients, nil
}

// WithTx 使用事务
func (d *ClientCredentialDao) WithTx(tx *gorm.DB) *ClientCredentialDao {
	return &ClientCredentialDao{
		IBaseDao: mvc.NewGormDao[model.ClientCredential](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}



