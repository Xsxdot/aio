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

// VisitDao 访问记录数据访问层
type VisitDao struct {
	mvc.IBaseDao[model.ShortVisit]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewVisitDao 创建访问记录 DAO 实例
func NewVisitDao(db *gorm.DB, log *logger.Log) *VisitDao {
	return &VisitDao{
		IBaseDao: mvc.NewGormDao[model.ShortVisit](db),
		log:      log.WithEntryName("VisitDao"),
		err:      errorc.NewErrorBuilder("VisitDao"),
		db:       db,
	}
}

// ListByLinkID 查询指定链接的访问记录
func (d *VisitDao) ListByLinkID(ctx context.Context, linkID int64, limit int) ([]*model.ShortVisit, error) {
	var results []*model.ShortVisit
	err := d.db.WithContext(ctx).Where("link_id = ?", linkID).
		Order("visited_at DESC").Limit(limit).Find(&results).Error
	if err != nil {
		return nil, d.err.New("查询访问记录失败", err).DB()
	}
	return results, nil
}

// CountByLinkID 统计指定链接的访问次数
func (d *VisitDao) CountByLinkID(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.ShortVisit{}).
		Where("link_id = ?", linkID).Count(&count).Error
	if err != nil {
		return 0, d.err.New("统计访问次数失败", err).DB()
	}
	return count, nil
}

// CountByLinkIDAndDateRange 统计指定链接在日期范围内的访问次数（按天聚合）
func (d *VisitDao) CountByLinkIDAndDateRange(ctx context.Context, linkID int64, startDate, endDate time.Time) (map[string]int64, error) {
	type DailyCount struct {
		Date  string
		Count int64
	}
	var results []DailyCount

	err := d.db.WithContext(ctx).Model(&model.ShortVisit{}).
		Select("DATE(visited_at) as date, COUNT(*) as count").
		Where("link_id = ? AND visited_at >= ? AND visited_at < ?", linkID, startDate, endDate).
		Group("DATE(visited_at)").
		Order("date ASC").
		Scan(&results).Error

	if err != nil {
		return nil, d.err.New("按日期统计访问次数失败", err).DB()
	}

	countMap := make(map[string]int64)
	for _, r := range results {
		countMap[r.Date] = r.Count
	}
	return countMap, nil
}

