package dao

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/registry/internal/model"

	"gorm.io/gorm"
)

// RegistryServiceDao 服务定义数据访问层
type RegistryServiceDao struct {
	mvc.IBaseDao[model.RegistryService]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

func NewRegistryServiceDao(db *gorm.DB, log *logger.Log) *RegistryServiceDao {
	return &RegistryServiceDao{
		IBaseDao: mvc.NewGormDao[model.RegistryService](db),
		log:      log,
		err:      errorc.NewErrorBuilder("RegistryServiceDao"),
		db:       db,
	}
}

func (d *RegistryServiceDao) FindByKey(ctx context.Context, project, name string) (*model.RegistryService, error) {
	var svc model.RegistryService
	err := d.db.WithContext(ctx).
		Where("project = ? AND name = ?", project, name).
		First(&svc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("服务不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询服务失败", err).DB()
	}
	return &svc, nil
}

func (d *RegistryServiceDao) ListByFilter(ctx context.Context, project string) ([]*model.RegistryService, error) {
	var list []*model.RegistryService
	q := d.db.WithContext(ctx).Model(&model.RegistryService{})
	if project != "" {
		q = q.Where("project = ?", project)
	}
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		return nil, d.err.New("查询服务列表失败", err).DB()
	}
	return list, nil
}

func (d *RegistryServiceDao) WithTx(tx *gorm.DB) *RegistryServiceDao {
	return &RegistryServiceDao{
		IBaseDao: mvc.NewGormDao[model.RegistryService](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}
