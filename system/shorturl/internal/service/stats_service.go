package service

import (
	"context"
	"time"

	"github.com/xsxdot/aio/system/shorturl/internal/dao"
	"github.com/xsxdot/aio/system/shorturl/internal/model"
	errorc "github.com/xsxdot/gokit/err"
	"github.com/xsxdot/gokit/logger"
)

// StatsService 统计服务
type StatsService struct {
	VisitDao        *dao.VisitDao
	SuccessEventDao *dao.SuccessEventDao
	log             *logger.Log
	err             *errorc.ErrorBuilder
}

// NewStatsService 创建统计服务实例
func NewStatsService(visitDaoInstance *dao.VisitDao, successEventDaoInstance *dao.SuccessEventDao, log *logger.Log) *StatsService {
	return &StatsService{
		VisitDao:        visitDaoInstance,
		SuccessEventDao: successEventDaoInstance,
		log:             log.WithEntryName("StatsService"),
		err:             errorc.NewErrorBuilder("StatsService"),
	}
}

// DailyStat 每日统计数据
type DailyStat struct {
	Date         string
	VisitCount   int64
	SuccessCount int64
}

// GetDailyStats 获取指定时间范围内的每日统计
func (s *StatsService) GetDailyStats(ctx context.Context, linkID int64, days int) ([]DailyStat, error) {
	if days <= 0 {
		days = 30 // 默认30天
	}

	endDate := time.Now().AddDate(0, 0, 1) // 明天0点
	startDate := endDate.AddDate(0, 0, -days)

	visitCounts, err := s.VisitDao.CountByLinkIDAndDateRange(ctx, linkID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	successCounts, err := s.SuccessEventDao.CountByLinkIDAndDateRange(ctx, linkID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 合并数据
	stats := make([]DailyStat, 0, days)
	current := startDate
	for current.Before(endDate) {
		dateStr := current.Format("2006-01-02")
		stats = append(stats, DailyStat{
			Date:         dateStr,
			VisitCount:   visitCounts[dateStr],
			SuccessCount: successCounts[dateStr],
		})
		current = current.AddDate(0, 0, 1)
	}

	return stats, nil
}

// CreateVisit 记录一条访问
func (s *StatsService) CreateVisit(ctx context.Context, visit *model.ShortVisit) error {
	if err := s.VisitDao.Create(ctx, visit); err != nil {
		s.log.WithErr(err).Error("创建访问记录失败")
		// 不阻断流程，记录失败仅打日志
		return err
	}
	return nil
}

// CreateSuccessEvent 记录一次成功事件（带幂等判断）
func (s *StatsService) CreateSuccessEvent(ctx context.Context, event *model.ShortSuccessEvent) error {
	if event.EventID != "" {
		exists, err := s.SuccessEventDao.ExistsByEventID(ctx, event.EventID)
		if err != nil {
			return err
		}
		if exists {
			s.log.WithField("event_id", event.EventID).Info("事件ID已存在，跳过重复上报")
			return nil // 幂等，不报错
		}
	}
	if err := s.SuccessEventDao.Create(ctx, event); err != nil {
		return err
	}
	return nil
}

// ListRecentVisits 获取最近的访问记录
func (s *StatsService) ListRecentVisits(ctx context.Context, linkID int64, limit int) ([]*model.ShortVisit, error) {
	return s.VisitDao.ListByLinkID(ctx, linkID, limit)
}

// ListRecentSuccess 获取最近的成功事件
func (s *StatsService) ListRecentSuccess(ctx context.Context, linkID int64, limit int) ([]*model.ShortSuccessEvent, error) {
	return s.SuccessEventDao.ListByLinkID(ctx, linkID, limit)
}







