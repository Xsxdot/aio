package app

import (
	"github.com/xsxdot/aio/system/executor/internal/service"
)

// App 是 executor 组件的内部应用实例，封装了所有服务层对象
type App struct {
	JobService        *service.ExecutorJobService
	JobAttemptService *service.ExecutorJobAttemptService
}

// NewApp 创建内部应用实例
func NewApp() *App {
	return &App{
		JobService:        service.NewExecutorJobService(),
		JobAttemptService: service.NewExecutorJobAttemptService(),
	}
}
