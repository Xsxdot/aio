// Package dao 中本文件负责工作流回调幂等标记的持久化。
//
// 职责：登记「某个 executor 任务的完成回调已被应用」，并提供过期标记清理。
//
// 边界：不解释回调内容，不参与 DAG 推进；登记失败必须让调用方感知，
// 不做静默降级——静默降级会让重复回调穿透到状态推进逻辑。
package dao

import (
	"context"
	"time"

	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	errorc "github.com/xsxdot/gokit/err"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkflowAppliedCallbackDao 工作流回调幂等标记 DAO。
type WorkflowAppliedCallbackDao struct {
	mvc.IBaseDao[model.WorkflowAppliedCallbackModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewWorkflowAppliedCallbackDao 创建回调幂等标记 DAO。
func NewWorkflowAppliedCallbackDao(db *gorm.DB, log *logger.Log) *WorkflowAppliedCallbackDao {
	return &WorkflowAppliedCallbackDao{
		IBaseDao: mvc.NewGormDao[model.WorkflowAppliedCallbackModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("WorkflowAppliedCallbackDao"),
		db:       db,
	}
}

// MarkApplied 登记一次回调的应用记录。
//
// 参数：
//   - ctx: 上下文
//   - tx: 调用方所在事务；必须与后续状态推进使用同一事务，否则幂等无意义
//   - jobID: 来源 executor 任务 ID，幂等键
//   - instanceID / nodeID: 仅用于排查，不参与判重
//
// 返回：
//   - applied=true 表示本次登记成功（首次处理，调用方应继续推进）
//   - applied=false 表示该 jobID 已处理过（调用方必须跳过，直接视为成功）
//   - 数据库错误
//
// 注意：用 ON CONFLICT DO NOTHING 而非「先查后插」，避免并发下的 TOCTOU 窗口；
// 该写法由 GORM 翻译到 MySQL 与 PostgreSQL 的原生冲突语义。
func (d *WorkflowAppliedCallbackDao) MarkApplied(ctx context.Context, tx *gorm.DB, jobID, instanceID int64, nodeID string) (bool, error) {
	rec := &model.WorkflowAppliedCallbackModel{
		JobID:      jobID,
		InstanceID: instanceID,
		NodeID:     nodeID,
	}
	res := mvc.ExtractDB(ctx, tx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "job_id"}},
			DoNothing: true,
		}).Create(rec)
	if res.Error != nil {
		d.log.WithErr(res.Error).
			WithField("job_id", jobID).
			WithField("instance_id", instanceID).
			WithField("node_id", nodeID).
			Error("登记回调幂等标记失败")
		return false, d.err.New("登记回调幂等标记失败", res.Error).DB().WithTraceID(ctx)
	}
	return res.RowsAffected > 0, nil
}

// CleanupBefore 硬删除 before 之前登记的幂等标记，返回删除行数。
//
// 注意：必须 Unscoped 硬删。common.Model 内嵌 gorm.DeletedAt，软删除的行
// 仍然占用 job_id 唯一索引，会导致该任务 ID 永远无法再次登记。
func (d *WorkflowAppliedCallbackDao) CleanupBefore(ctx context.Context, before time.Time) (int64, error) {
	res := mvc.ExtractDB(ctx, d.db).
		Unscoped().
		Where("created_at < ?", before).
		Delete(&model.WorkflowAppliedCallbackModel{})
	if res.Error != nil {
		d.log.WithErr(res.Error).WithField("before", before).Error("清理回调幂等标记失败")
		return 0, d.err.New("清理回调幂等标记失败", res.Error).DB().WithTraceID(ctx)
	}
	d.log.WithField("before", before).WithField("deleted", res.RowsAffected).Info("回调幂等标记清理完成")
	return res.RowsAffected, nil
}
