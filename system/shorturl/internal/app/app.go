package app

import (
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/shorturl/internal/dao"
	"xiaozhizhang/system/shorturl/internal/service"
)

// App 短网址组件应用层
type App struct {
	DomainService *service.DomainService
	LinkService   *service.LinkService
	StatsService  *service.StatsService
	log           *logger.Log
	err           *errorc.ErrorBuilder
}

// NewApp 创建短网址组件应用层实例
func NewApp() *App {
	log := logger.GetLogger().WithEntryName("ShortURLApp")

	// 初始化 DAO
	domainDao := dao.NewDomainDao(base.DB, log)
	linkDao := dao.NewLinkDao(base.DB, log)
	visitDao := dao.NewVisitDao(base.DB, log)
	successEventDao := dao.NewSuccessEventDao(base.DB, log)

	// 初始化 Service
	domainSvc := service.NewDomainService(domainDao, log)
	linkSvc := service.NewLinkService(linkDao, log)
	statsSvc := service.NewStatsService(visitDao, successEventDao, log)

	return &App{
		DomainService: domainSvc,
		LinkService:   linkSvc,
		StatsService:  statsSvc,
		log:           log,
		err:           errorc.NewErrorBuilder("ShortURLApp"),
	}
}
