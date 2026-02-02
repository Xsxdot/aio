package dao

import (
	"context"
	"time"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/shorturl/internal/model"

	"gorm.io/gorm"
)

// SuccessEventDao 成功上报事件数据访问层
type SuccessEventDao struct {
	mvc.IBaseDao[model.ShortSuccessEvent]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewSuccessEventDao 创建成功上报事件 DAO 实例
func NewSuccessEventDao(db *gorm.DB, log *logger.Log) *SuccessEventDao {
	return &SuccessEventDao{
		IBaseDao: mvc.NewGormDao[model.ShortSuccessEvent](db),
		log:      log.WithEntryName("SuccessEventDao"),
		err:      errorc.NewErrorBuilder("SuccessEventDao"),
		db:       db,
	}
}

// ListByLinkID 查询指定链接的成功事件记录
func (d *SuccessEventDao) ListByLinkID(ctx context.Context, linkID int64, limit int) ([]*model.ShortSuccessEvent, error) {
	var results []*model.ShortSuccessEvent
	err := d.db.WithContext(ctx).Where("link_id = ?", linkID).
		Order("created_at DESC").Limit(limit).Find(&results).Error
	if err != nil {
		return nil, d.err.New("查询成功事件记录失败", err).DB()
	}
	return results, nil
}

// CountByLinkID 统计指定链接的成功次数
func (d *SuccessEventDao) CountByLinkID(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.ShortSuccessEvent{}).
		Where("link_id = ?", linkID).Count(&count).Error
	if err != nil {
		return 0, d.err.New("统计成功次数失败", err).DB()
	}
	return count, nil
}

// CountByLinkIDAndDateRange 统计指定链接在日期范围内的成功次数（按天聚合）
func (d *SuccessEventDao) CountByLinkIDAndDateRange(ctx context.Context, linkID int64, startDate, endDate time.Time) (map[string]int64, error) {
	type DailyCount struct {
		Date  string
		Count int64
	}
	var results []DailyCount

	err := d.db.WithContext(ctx).Model(&model.ShortSuccessEvent{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("link_id = ? AND created_at >= ? AND created_at < ?", linkID, startDate, endDate).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error

	if err != nil {
		return nil, d.err.New("按日期统计成功次数失败", err).DB()
	}

	countMap := make(map[string]int64)
	for _, r := range results {
		countMap[r.Date] = r.Count
	}
	return countMap, nil
}

// ExistsByEventID 检查事件ID是否已存在（用于幂等）
func (d *SuccessEventDao) ExistsByEventID(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}
	var count int64
	err := d.db.WithContext(ctx).Model(&model.ShortSuccessEvent{}).
		Where("event_id = ?", eventID).Count(&count).Error
	if err != nil {
		return false, d.err.New("检查事件ID是否存在失败", err).DB()
	}
	return count > 0, nil
}


