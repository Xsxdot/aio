package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/xsxdot/aio/internal/fiber"
	grpcserver "github.com/xsxdot/aio/internal/grpc"
	"github.com/xsxdot/aio/pkg/lock"
	"github.com/xsxdot/aio/pkg/registry"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/pkg/config"
	"github.com/xsxdot/aio/pkg/scheduler"

	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/certmanager"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/internal/monitoring"
	common2 "github.com/xsxdot/aio/pkg/common"
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
	a.Logger = logger

	etcdClient, err := etcd.NewClient(a.BaseConfig.Etcd)
	if err != nil {
		return err
	}
	a.Etcd = etcdClient

	lockManager, err := lock.NewEtcdLockManager(etcdClient, "aio-lock-manager")
	if err != nil {
		return err
	}
	a.LockManager = lockManager

	authManager, err := authmanager.NewAuthManager(authmanager.NewEtcdStorage(etcdClient), lockManager)
	if err != nil {
		return err
	}
	a.AuthManager = authManager
	err = authManager.Start(context.Background())
	if err != nil {
		return err
	}

	schedulerWithEtcd := scheduler.NewScheduler(lockManager, scheduler.DefaultSchedulerConfig())
	a.Scheduler = schedulerWithEtcd
	err = schedulerWithEtcd.Start()
	if err != nil {
		return err
	}

	service, err := config.NewService(a.BaseConfig, etcdClient)
	if err != nil {
		return err
	}
	a.ConfigService = service
	err = service.Start(context.Background())
	if err != nil {
		return err
	}

	discoveryComponent, err := registry.NewRegistry(etcdClient)
	if err != nil {
		return err
	}
	a.Registry = discoveryComponent

	monitor := monitoring.New(a.BaseConfig, etcdClient)
	a.Monitor = monitor
	err = monitor.Start(context.Background())
	if err != nil {
		return err
	}

	certManager, err := certmanager.NewCertManager(a.BaseConfig, etcdClient, schedulerWithEtcd)
	if err != nil {
		return err
	}
	a.SSL = certManager
	err = certManager.Start(context.Background())
	if err != nil {
		return err
	}

	err = a.initGrpc()
	if err != nil {
		return err
	}

	err = a.initHTTPServer()
	if err != nil {
		return err
	}

	// 启动完成后注册AIO服务到服务注册中心
	err = a.registerAIOService()
	if err != nil {
		log.Printf("注册AIO服务失败: %v", err)
		// 注册失败不影响应用启动，只记录警告
	}

	log.Println("AIO服务启动完成")
	return nil
}

func (a *App) initGrpc() error {
	defaultConfig := grpcserver.DefaultConfig()
	defaultConfig.Address = fmt.Sprintf(":%d", a.BaseConfig.System.GrpcPort)
	defaultConfig.EnableAuth = true
	defaultConfig.Auth = grpcserver.DefaultAuthConfig()
	defaultConfig.Auth.AuthProvider = authmanager.NewAuthManagerAdapter(a.AuthManager)

	server := grpcserver.NewServer(defaultConfig, a.Logger.GetZapLogger("Grpc"))

	authService := authmanager.NewGRPCService(a.AuthManager)
	if err := server.RegisterService(authService); err != nil {
		log.Fatalf("注册鉴权服务失败: %v", err)
	}

	configGRPCService := config.NewGRPCService(a.ConfigService, a.Logger.GetZapLogger("Config"))
	if err := server.RegisterService(configGRPCService); err != nil {
		log.Fatalf("注册配置服务失败: %v", err)
	}

	// 创建并注册 Registry gRPC 服务
	registryGRPCService := registry.NewGRPCService(a.Registry)
	if err := server.RegisterService(registryGRPCService); err != nil {
		log.Fatal("注册 Registry gRPC 服务失败", zap.Error(err))
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		log.Fatal("启动 gRPC 服务器失败", zap.Error(err))
		return err
	}

	a.GrpcServer = server

	log.Println("启动 gRPC 服务器成功")

	return nil
}

// initHTTPServer 初始化HTTP服务
func (a *App) initHTTPServer() error {
	fiberServer := fiber.NewFiberServer()
	fiberServer.RegisterStaticFiles("./web", "/")
	// 如果认证管理器已配置，注册认证API
	if a.AuthManager != nil {
		fiberServer.RegisterAuthManagerAPI(a.AuthManager)
	}

	if a.Registry != nil {
		fiberServer.RegisterDistributedAPI(a.Registry)
	} else {
		a.Logger.Warn("分布式组件管理器未初始化，跳过API注册")
	}

	// 注册配置中心API
	if a.ConfigService != nil {
		fiberServer.RegisterConfigAPI(a.ConfigService)
	} else {
		a.Logger.Warn("配置中心服务未初始化，跳过API注册")
	}

	if a.Monitor != nil {
		fiberServer.RegisterMonitorAPI(a.Monitor)
	} else {
		a.Logger.Warn("监控服务未初始化，跳过API注册")
	}

	// 注册证书管理器API
	if a.SSL != nil {
		fiberServer.RegisterCertManagerAPI(a.SSL)
	} else {
		a.Logger.Warn("证书管理器未初始化，跳过API注册")
	}

	a.FiberServer = fiberServer

	return fiberServer.Start(a.BaseConfig.System.HttpPort)
}

// registerAIOService 将AIO服务注册到服务注册中心
func (a *App) registerAIOService() error {
	ctx := context.Background()

	// 获取本机IP地址
	localIP, err := a.getLocalIP()
	if err != nil {
		return fmt.Errorf("获取本机IP失败: %w", err)
	}

	// 构建服务实例
	instance := &registry.ServiceInstance{
		Name:     "aio-service",
		Address:  fmt.Sprintf("%s:%d", localIP, a.BaseConfig.System.GrpcPort),
		Protocol: "grpc",
		Env:      registry.EnvAll, // AIO服务适用于所有环境
		Metadata: map[string]string{
			"version":   "1.0.0",
			"node_type": "aio-node",
			"http_port": strconv.Itoa(a.BaseConfig.System.HttpPort),
			"grpc_port": strconv.Itoa(a.BaseConfig.System.GrpcPort),
			"services":  "auth,config,registry,monitor,ssl", // 支持的服务列表
		},
		Weight: 100,
		Status: "active",
	}

	// 注册服务实例
	err = a.Registry.Register(ctx, instance)
	if err != nil {
		return fmt.Errorf("注册AIO服务实例失败: %w", err)
	}

	// 保存实例ID，用于后续注销
	// 注意：Register方法会自动生成并设置ID到传入的instance中
	a.serviceInstanceID = instance.ID

	if a.serviceInstanceID == "" {
		return fmt.Errorf("注册成功但未获取到服务实例ID")
	}

	log.Printf("AIO服务已成功注册到服务中心: %s (ID: %s, Env: %s)", instance.Address, instance.ID, instance.Env)

	// 创建自动续约定时任务
	if err := a.setupAutoRenewal(ctx); err != nil {
		log.Printf("设置自动续约任务失败: %v", err)
		// 不返回错误，让服务继续运行
	}

	return nil
}

// setupAutoRenewal 设置自动续约定时任务
func (a *App) setupAutoRenewal(ctx context.Context) error {
	if a.serviceInstanceID == "" {
		return fmt.Errorf("服务实例ID为空")
	}

	// 创建自动续约任务（每10秒续约一次）
	renewalTask := scheduler.NewIntervalTask(
		fmt.Sprintf("aio-service-renewal-%s", a.serviceInstanceID),
		time.Now().Add(10*time.Second), // 10秒后开始执行
		10*time.Second,                 // 每10秒执行一次
		scheduler.TaskExecuteModeLocal, // 本地任务
		5*time.Second,                  // 5秒超时
		func(ctx context.Context) error {
			// 执行服务续约
			err := a.Registry.Renew(ctx, a.serviceInstanceID)
			if err != nil {
				log.Printf("服务续约失败 (ID: %s): %v", a.serviceInstanceID, err)
				return err
			}
			log.Printf("服务续约成功 (ID: %s)", a.serviceInstanceID)
			return nil
		},
	)

	// 添加任务到调度器
	if err := a.Scheduler.AddTask(renewalTask); err != nil {
		return fmt.Errorf("添加自动续约任务失败: %v", err)
	}

	// 保存任务ID，用于后续取消
	a.renewalTaskID = renewalTask.GetID()

	log.Printf("自动续约任务已设置 (TaskID: %s, ServiceID: %s)", renewalTask.GetID(), a.serviceInstanceID)
	return nil
}

// getLocalIP 获取本机IP地址（优先获取内网IP）
func (a *App) getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				// 优先返回内网IP
				if ipnet.IP.IsPrivate() {
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	// 如果没找到内网IP，返回第一个非回环地址
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "127.0.0.1", nil
}
