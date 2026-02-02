package base

import (
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/security"
	"github.com/xsxdot/aio/pkg/core/start"
	"github.com/xsxdot/aio/pkg/grpc"
	"github.com/xsxdot/aio/pkg/oss"
	"github.com/xsxdot/aio/pkg/scheduler"

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
	GRPCServer *grpc.Server
)
