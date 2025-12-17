package service

import (
	"context"
	"time"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/shorturl/internal/dao"
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
