package base

import (
	"github.com/xsxdot/aio/pkg/core/start"
	"github.com/xsxdot/gokit/executor"
	"github.com/xsxdot/gokit/grpc"
	"github.com/xsxdot/gokit/logger"
	"github.com/xsxdot/gokit/oss"
	"github.com/xsxdot/gokit/scheduler"
	"github.com/xsxdot/gokit/security"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	Configures *start.Configures
	Logger     *logger.Log
	ENV        string
	AdminAuth  *security.AdminAuth
	UserAuth   *security.UserAuth
	ClientAuth *security.ClientAuth
	DB         *gorm.DB
	RDB        *redis.Client
	Cache      *cache.Cache
	OSS        *oss.AliyunService
	Scheduler  *scheduler.Scheduler
	Executor   *executor.Executor
	GRPCServer *grpc.Server
	// AIClient AI 服务统一客户端（可选）
	AIClient interface{}
)
