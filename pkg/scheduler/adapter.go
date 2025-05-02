package scheduler

import (
	"context"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/lock"
)

// EtcdLockProvider 基于etcd的锁提供者
type EtcdLockProvider struct {
	lockService lock.LockService
}

// CreateLock 创建锁实例
func (e *EtcdLockProvider) CreateLock(ctx context.Context, key string, options ...lock.LockOption) (lock.Lock, error) {
	return e.lockService.Create(key, options...)
}

// NewEtcdLockProvider 创建基于etcd的锁提供者
func NewEtcdLockProvider(lockService lock.LockService) *EtcdLockProvider {
	return &EtcdLockProvider{
		lockService: lockService,
	}
}

// NewSchedulerWithEtcd 使用etcd客户端创建调度器
func NewSchedulerWithEtcd(etcdClient *etcd.EtcdClient, options *SchedulerOptions) (*Scheduler, error) {
	logger := common.GetLogger().GetZapLogger("scheduler")

	// 创建锁服务
	lockService, err := lock.NewLockService(etcdClient, logger)
	if err != nil {
		return nil, err
	}

	// 启动锁服务
	err = lockService.Start(context.Background())
	if err != nil {
		return nil, err
	}

	// 创建锁提供者
	lockProvider := NewEtcdLockProvider(lockService)

	return NewScheduler(lockProvider, options)
}
