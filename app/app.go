package app

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/xsxdot/aio/pkg/monitoring"
	"github.com/xsxdot/aio/pkg/server"

	"github.com/xsxdot/aio/app/config"
	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/certmanager"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/internal/fiber"
	"github.com/xsxdot/aio/internal/grpc"
	"github.com/xsxdot/aio/pkg/common"
	config3 "github.com/xsxdot/aio/pkg/config"
	"github.com/xsxdot/aio/pkg/lock"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/scheduler"
)

// App 表示应用程序核心
type App struct {
	// 配置
	BaseConfig    *config.BaseConfig
	configDirPath string     // 配置文件目录路径
	mu            sync.Mutex // 保护并发访问

	// 核心组件
	Logger      *common.Logger
	AuthManager *authmanager.AuthManager
	Scheduler   *scheduler.Scheduler

	// 分布式基础组件
	Etcd          *etcd.EtcdClient
	ConfigService *config3.Service
	LockManager   lock.LockManager
	Registry      registry.Registry

	// 可选服务组件
	Monitor       *monitoring.Monitor
	SSL           *certmanager.CertManager
	ServerService server.Service

	// Fiber
	FiberServer *fiber.Server

	//GRPC
	GrpcServer *grpc.Server

	// 服务注册信息
	serviceInstanceID string // 注册到服务中心的实例ID
	renewalTaskID     string // 自动续约任务ID
}

// New 创建一个新的应用实例
func New() *App {
	a := &App{}
	return a
}

// LoadConfig 加载配置
func (a *App) LoadConfig(configDirPath string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := config.LoadConfig(configDirPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	a.BaseConfig = cfg
	a.configDirPath = configDirPath

	return nil
}

// Start 启动应用
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.BaseConfig == nil {
		return fmt.Errorf("未加载配置，无法启动应用")
	}

	// 应用启动逻辑，将在starter.go中实现
	err := a.startApp()
	if err != nil {
		return err
	}

	return err
}

// Stop 停止应用
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Println("正在停止应用...")

	// 取消自动续约任务
	if a.renewalTaskID != "" && a.Scheduler != nil {
		if removed := a.Scheduler.RemoveTask(a.renewalTaskID); removed {
			log.Printf("自动续约任务已取消: %s\n", a.renewalTaskID)
		} else {
			log.Printf("取消自动续约任务失败: %s\n", a.renewalTaskID)
		}
		a.renewalTaskID = ""
	}

	// 注销服务实例
	if a.serviceInstanceID != "" && a.Registry != nil {
		_, err := a.Registry.Offline(context.Background(), a.serviceInstanceID)
		if err != nil {
			log.Printf("注销AIO服务实例失败: %v\n", err)
		} else {
			log.Printf("AIO服务实例已成功注销: %s\n", a.serviceInstanceID)
		}
	}

	err := a.FiberServer.Stop(context.Background())
	if err != nil {
		log.Printf("停止Fiber服务器失败: %v\n", err)
	}

	if err := a.GrpcServer.Shutdown(context.Background()); err != nil {
		log.Printf("关闭GRPC服务器失败")
	}

	err = a.LockManager.Close()
	if err != nil {
		log.Printf("关闭锁管理器失败: %v\n", err)
	}
	err = a.Registry.Close()
	if err != nil {
		log.Printf("关闭注册中心失败: %v\n", err)
	}
	err = a.Scheduler.Stop()
	if err != nil {
		log.Printf("停止计划任务失败: %v\n", err)
	}

	err = a.Monitor.Stop(context.Background())
	if err != nil {
		log.Printf("停止监控系统失败: %v\n", err)
	}
	err = a.SSL.Stop(context.Background())
	if err != nil {
		log.Printf("停止SSL服务失败: %v\n", err)
	}
	a.Etcd.Close()

	log.Println("应用已停止")
	return nil
}
