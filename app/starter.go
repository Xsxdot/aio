package app

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/pkg/config"
	"log"

	cfg "github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/app/fiber"
	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/cache/server"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/internal/monitoring"
	"github.com/xsxdot/aio/internal/mq"
	common2 "github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/protocol"
)

// startApp 实现应用启动流程
func (a *App) startApp() error {
	log.Println("开始启动AIO服务...")

	//初始化日志
	logger, err := common2.NewLogger(*a.BaseConfig.Logger)
	if err != nil {
		panic(err)
	}
	common2.SetLogger(logger)
	common2.ErrorDebugMode = a.BaseConfig.Errors.DebugMode
	a.Logger = logger

	if !a.initialized {
		a.BaseConfig.System.DataDir = "./init"
	}

	a.Manager.Register(func() (*ComponentEntity, error) {
		etcdComponent := etcd.NewEtcdComponent()
		a.Etcd = etcdComponent
		return NewBaseComponentEntity(etcdComponent, "etcd.conf"), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		service := config.NewService(a.Etcd.GetClient())
		a.ConfigService = service
		return NewBaseComponentEntityWithNilConfig(service), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		discoveryComponent, err := discovery.NewDiscoveryService(a.Etcd.GetClient().GetClient(), common2.GetLogger().GetZapLogger(consts.ComponentDiscovery))
		if err != nil {
			return nil, err
		}
		a.Discovery = discoveryComponent
		return NewMustComponentEntity(discoveryComponent, cfg.ReadTypeNil), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		storage := authmanager.NewEtcdStorage(a.Etcd.GetClient())
		manager, err := authmanager.NewAuthManager(storage)
		if err != nil {
			return nil, err
		}
		a.AuthManager = manager
		return NewMustComponentEntity(manager, cfg.ReadTypeCenter), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		protocolManager := protocol.NewServer(a.AuthManager)
		a.Protocol = protocolManager
		return NewMustComponentEntity(protocolManager, cfg.ReadTypeNil), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		electionService, err2 := election.NewElectionService(a.Etcd.GetClient().GetClient(), common2.GetLogger().GetZapLogger(consts.ComponentElection))
		if err2 != nil {
			return nil, err2
		}
		a.Election = electionService
		return NewNormalComponentEntity(electionService, cfg.ReadTypeCenter), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		mqServer, err := mq.NewNatsServer()
		if err != nil {
			return nil, err
		}
		a.MQServer = mqServer
		return NewNormalComponentEntity(mqServer, cfg.ReadTypeCenter), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		var ops []server.ServerOption
		if a.BaseConfig.System.Mode == consts.SystemModeCluster {
			ops = append(ops, server.WithReplication(a.Election.GetDefaultElection(), a.Discovery, a.Protocol))
		}
		cacheServer, err := server.NewServer(ops...)
		if err != nil {
			return nil, err
		}
		a.CacheServer = cacheServer
		return NewNormalComponentEntity(cacheServer, cfg.ReadTypeCenter), nil
	})

	a.Manager.Register(func() (*ComponentEntity, error) {
		var client *mq.NatsClient
		if a.MQServer != nil {
			client, err = a.MQServer.GetClient()
			if err != nil {
				logger.Errorf("创建消息队列客户端失败: %v", err)
			}
		}
		monitor := monitoring.New(a.Etcd.GetClient(), client)
		a.Monitor = monitor
		return NewNormalComponentEntity(monitor, cfg.ReadTypeCenter), nil
	})

	a.Manager.Register(a.initHTTPServer)

	err = a.Manager.StartAll(context.Background())
	if err != nil {
		return err
	}

	if err := a.initTcpApi(); err != nil {
		return fmt.Errorf("初始化TCP接口失败: %w", err)
	}

	a.Manager.RegisterAllService()
	a.Manager.RegisterClientConfig()

	log.Println("AIO服务启动完成")
	return nil
}

// initHTTPServer 初始化HTTP服务
func (a *App) initHTTPServer() (*ComponentEntity, error) {
	fiberServer := fiber.NewFiberServer()
	// 如果认证管理器已配置，注册认证API
	if a.AuthManager != nil {
		fiberServer.RegisterAuthManagerAPI(a.AuthManager)
	}

	// 注册ETCD API
	fiberServer.RegisterEtcdAPI(a.Etcd.Server, a.Etcd.GetClient())

	// 注册分布式组件API
	if a.Discovery != nil {
		fiberServer.RegisterDistributedAPI(a.Discovery, a.Election)
	} else {
		a.Logger.Warn("分布式组件管理器未初始化，跳过API注册")
	}

	// 注册配置中心API
	if a.ConfigService != nil {
		fiberServer.RegisterConfigAPI(a.ConfigService)
	} else {
		a.Logger.Warn("配置中心服务未初始化，跳过API注册")
	}

	// 注册缓存服务器API
	if a.CacheServer != nil {
		fiberServer.RegisterCacheServerAPI(a.CacheServer)
	} else {
		a.Logger.Warn("缓存服务器未初始化，跳过API注册")
	}

	if a.Monitor != nil {
		fiberServer.RegisterMonitorAPI(a.Monitor)
	} else {
		a.Logger.Warn("监控服务未初始化，跳过API注册")
	}

	apiHandler := NewAPIHandler(a)
	apiHandler.SetupRoutes(fiberServer.GetApp())

	a.FiberServer = fiberServer
	return NewMustComponentEntity(fiberServer, cfg.ReadTypeNil), nil
}
