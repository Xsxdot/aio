package dao

import (
	"context"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/application/internal/model"

	"gorm.io/gorm"
)

// DeploymentDao 部署记录数据访问层
type DeploymentDao struct {
	mvc.IBaseDao[model.Deployment]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewDeploymentDao 创建部署记录 DAO 实例
func NewDeploymentDao(db *gorm.DB, log *logger.Log) *DeploymentDao {
	return &DeploymentDao{
		IBaseDao: mvc.NewGormDao[model.Deployment](db),
		log:      log.WithEntryName("DeploymentDao"),
		err:      errorc.NewErrorBuilder("DeploymentDao"),
		db:       db,
	}
}

// ListByApplicationID 根据应用 ID 查询部署记录
func (d *DeploymentDao) ListByApplicationID(ctx context.Context, applicationID int64, limit int) ([]*model.Deployment, error) {
	var list []*model.Deployment
	q := d.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, d.err.New("查询部署记录失败", err).DB()
	}
	return list, nil
}

// ListByReleaseID 根据版本 ID 查询部署记录
func (d *DeploymentDao) ListByReleaseID(ctx context.Context, releaseID int64) ([]*model.Deployment, error) {
	var list []*model.Deployment
	err := d.db.WithContext(ctx).
		Where("release_id = ?", releaseID).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, d.err.New("查询部署记录失败", err).DB()
	}
	return list, nil
}

// UpdateStatus 更新部署状态
func (d *DeploymentDao) UpdateStatus(ctx context.Context, id int64, status model.DeploymentStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if status == model.DeploymentStatusRunning {
		now := time.Now()
		updates["started_at"] = &now
	}
	if status == model.DeploymentStatusSuccess || status == model.DeploymentStatusFailed {
		now := time.Now()
		updates["finished_at"] = &now
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}

	res := d.db.WithContext(ctx).Model(&model.Deployment{}).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		return d.err.New("更新部署状态失败", res.Error).DB()
	}
	return nil
}

// AppendLog 追加部署日志
func (d *DeploymentDao) AppendLog(ctx context.Context, id int64, logEntry string) error {
	// 获取当前 logs
	var deployment model.Deployment
	if err := d.db.WithContext(ctx).Select("logs").First(&deployment, id).Error; err != nil {
		return d.err.New("获取部署记录失败", err).DB()
	}

	// 追加日志
	var logs []string
	if deployment.Logs != nil {
		logs = make([]string, 0)
		for _, v := range deployment.Logs {
			if s, ok := v.(string); ok {
				logs = append(logs, s)
			}
		}
	}
	logs = append(logs, logEntry)

	// 更新
	res := d.db.WithContext(ctx).Model(&model.Deployment{}).
		Where("id = ?", id).
		Update("logs", logs)
	if res.Error != nil {
		return d.err.New("追加部署日志失败", res.Error).DB()
	}
	return nil
}

// WithTx 使用事务
func (d *DeploymentDao) WithTx(tx *gorm.DB) *DeploymentDao {
	return &DeploymentDao{
		IBaseDao: mvc.NewGormDao[model.Deployment](tx),
		log:      d.log,
		err:      d.err,
		db:       tx,
	}
}

