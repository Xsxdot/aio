package dao

import (
	"context"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/registry/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RegistryInstanceDao 实例登记数据访问层
type RegistryInstanceDao struct {
	mvc.IBaseDao[model.RegistryInstance]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

func NewRegistryInstanceDao(db *gorm.DB, log *logger.Log) *RegistryInstanceDao {
	return &RegistryInstanceDao{
		IBaseDao: mvc.NewGormDao[model.RegistryInstance](db),
		log:      log,
		err:      errorc.NewErrorBuilder("RegistryInstanceDao"),
		db:       db,
	}
}

func (d *RegistryInstanceDao) FindByServiceAndKey(ctx context.Context, serviceID int64, instanceKey string) (*model.RegistryInstance, error) {
	var inst model.RegistryInstance
	err := d.db.WithContext(ctx).
		Where("service_id = ? AND instance_key = ?", serviceID, instanceKey).
		First(&inst).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("实例不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, d.err.New("查询实例失败", err).DB()
	}
	return &inst, nil
}

func (d *RegistryInstanceDao) ListByServiceID(ctx context.Context, serviceID int64) ([]*model.RegistryInstance, error) {
	var list []*model.RegistryInstance
	if err := d.db.WithContext(ctx).
		Where("service_id = ?", serviceID).
		Order("id DESC").
		Find(&list).Error; err != nil {
		return nil, d.err.New("查询实例列表失败", err).DB()
	}
	return list, nil
}

// ListByServiceIDAndEnv 查询指定服务和环境的实例列表
func (d *RegistryInstanceDao) ListByServiceIDAndEnv(ctx context.Context, serviceID int64, env string) ([]*model.RegistryInstance, error) {
	var list []*model.RegistryInstance
	q := d.db.WithContext(ctx).Where("service_id = ?", serviceID)
	if env != "" {
		q = q.Where("env = ?", env)
	}
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		return nil, d.err.New("查询实例列表失败", err).DB()
	}
	return list, nil
}

// Upsert 实例存在则更新，不存在则创建
func (d *RegistryInstanceDao) Upsert(ctx context.Context, inst *model.RegistryInstance) error {
	if inst.LastHeartbeatAt.IsZero() {
		inst.LastHeartbeatAt = time.Now()
	}
	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "service_id"}, {Name: "instance_key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"env",
				"host",
				"endpoint",
				"meta",
				"ttl_seconds",
				"last_heartbeat_at",
				"updated_at",
			}),
		}).
		Create(inst).Error
}

func (d *RegistryInstanceDao) UpdateHeartbeat(ctx context.Context, serviceID int64, instanceKey string, at time.Time) error {
	res := d.db.WithContext(ctx).Model(&model.RegistryInstance{}).
		Where("service_id = ? AND instance_key = ?", serviceID, instanceKey).
		Update("last_heartbeat_at", at)
	if res.Error != nil {
		return d.err.New("更新心跳失败", res.Error).DB()
	}
	if res.RowsAffected == 0 {
		return d.err.New("实例不存在", gorm.ErrRecordNotFound).WithCode(errorc.ErrorCodeNotFound)
	}
	return nil
}

func (d *RegistryInstanceDao) DeleteByServiceAndKey(ctx context.Context, serviceID int64, instanceKey string) error {
	res := d.db.WithContext(ctx).Where("service_id = ? AND instance_key = ?", serviceID, instanceKey).Delete(&model.RegistryInstance{})
	if res.Error != nil {
		return d.err.New("删除实例失败", res.Error).DB()
	}
	return nil
}

func (d *RegistryInstanceDao) WithTx(tx *gorm.DB) *RegistryInstanceDao {
	return &RegistryInstanceDao{
		IBaseDao: mvc.NewGormDao[model.RegistryInstance](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}
