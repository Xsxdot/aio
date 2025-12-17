package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"xiaozhizhang/app"
	"xiaozhizhang/base"
	"xiaozhizhang/pkg/core/security"
	"xiaozhizhang/pkg/core/start"
	"xiaozhizhang/pkg/db"
	"xiaozhizhang/pkg/grpc"
	"xiaozhizhang/pkg/oss"
	"xiaozhizhang/pkg/scheduler"
	"xiaozhizhang/router"
	agentclient "xiaozhizhang/system/agent/api/client"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	env, filename := getBaseInfo()

	file, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("读取配置文件失败,因为：%v", err))
	}

	configures := start.NewConfigures(file, env)
	base.Configures = configures
	base.Logger = configures.Logger
	base.ENV = env
	base.AdminAuth = base.Configures.AdminAuth
	base.UserAuth = base.Configures.UserAuth
	base.ClientAuth = security.NewClientAuth([]byte(base.Configures.Config.Jwt.Secret))

	base.OSS, err = oss.InitAliyunOSS(context.Background(), &configures.Config.Oss)
	if err != nil {
		configures.Logger.Panic(err)
	}

	base.DB = configures.EnableMysql()

	// 执行数据库迁移
	if err := db.AutoMigrate(base.DB); err != nil {
		configures.Logger.Panic(fmt.Sprintf("数据库迁移失败: %v", err))
	}

	base.RDB = configures.EnableRedis()
	base.Cache = configures.EnableCache(base.RDB)
	base.Scheduler = scheduler.NewScheduler(scheduler.DefaultSchedulerConfig())
	err = base.Scheduler.Start()
	if err != nil {
		configures.Logger.Panic(fmt.Sprintf("启动调度器失败: %v", err))
	}

	// 初始化 Agent 客户端（用于调用远程 agent 守护进程）
	base.AgentClient = initAgentClient()

	if env == "dev" {
		// 开发环境下添加数据库保活任务，防止代理超时导致连接断开
		keepAliveTask := scheduler.NewIntervalTask(
			"数据库连接保活",
			time.Now(),
			10*time.Second,
			scheduler.TaskExecuteModeLocal,
			5*time.Second,
			func(ctx context.Context) error {
				sqlDB, err := base.DB.DB()
				if err != nil {
					base.Logger.WithErr(err).Error("获取数据库连接失败")
					return err
				}
				if err := sqlDB.Ping(); err != nil {
					base.Logger.WithErr(err).Error("数据库Ping失败")
					return err
				}
				return nil
			},
		)
		if err := base.Scheduler.AddTask(keepAliveTask); err != nil {
			configures.Logger.Panic(fmt.Sprintf("添加数据库保活任务失败: %v", err))
		}
		base.Logger.Info("已启动数据库保活任务，每10秒执行一次")
	}

	// 创建应用组合根
	appRoot := app.NewApp()

	// 初始化默认超级管理员（当 user_admin 表为空时自动创建 admin/admin）
	if err := appRoot.UserModule.EnsureBootstrapSuperAdmin(context.Background()); err != nil {
		configures.Logger.Panic(fmt.Sprintf("初始化默认超级管理员失败: %v", err))
	}

	// 初始化 bootstrap 服务器（从配置文件加载）
	if err := appRoot.ServerModule.EnsureBootstrapServers(context.Background()); err != nil {
		configures.Logger.Panic(fmt.Sprintf("初始化 bootstrap 服务器失败: %v", err))
	}

	// 初始化 bootstrap 服务器的 SSH 凭证（从配置文件加载）
	if err := appRoot.ServerModule.EnsureBootstrapServerSSHCredentials(context.Background()); err != nil {
		configures.Logger.Panic(fmt.Sprintf("初始化 bootstrap 服务器 SSH 凭证失败: %v", err))
	}

	// 注册 SSL 证书自动续期任务（每天凌晨 2:30 执行）
	sslRenewTask, err := scheduler.NewCronTask(
		"SSL证书自动续期",
		"0 30 2 * * *", // 每天凌晨 2:30
		scheduler.TaskExecuteModeDistributed,
		10*time.Minute, // 超时时间 10 分钟
		func(ctx context.Context) error {
			base.Logger.Info("开始执行 SSL 证书自动续期任务")
			if err := appRoot.SslModule.RenewDueCertificates(ctx); err != nil {
				base.Logger.WithErr(err).Error("SSL 证书自动续期任务执行失败")
				return err
			}
			base.Logger.Info("SSL 证书自动续期任务执行完成")
			return nil
		},
	)
	if err != nil {
		configures.Logger.Panic(fmt.Sprintf("创建 SSL 证书自动续期任务失败: %v", err))
	}
	if err := base.Scheduler.AddTask(sslRenewTask); err != nil {
		configures.Logger.Panic(fmt.Sprintf("添加 SSL 证书自动续期任务失败: %v", err))
	}
	base.Logger.Info("已注册 SSL 证书自动续期任务，每天凌晨 2:30 执行")

	// 创建 Fiber 应用
	fiberApp := app.GetApp()

	// 注册路由
	router.Register(appRoot, fiberApp)

	// 初始化和启动 gRPC 服务器
	if base.Configures.Config.GRPC.Address != "" {
		grpcConfig := base.Configures.Config.GRPC

		// 创建 gRPC Server 配置
		grpcServerConfig := &grpc.Config{
			Address:           grpcConfig.Address,
			EnableReflection:  grpcConfig.EnableReflection,
			EnableRecovery:    grpcConfig.EnableRecovery,
			EnableValidation:  grpcConfig.EnableValidation,
			EnableAuth:        grpcConfig.EnableAuth,
			EnablePermission:  grpcConfig.EnablePermission,
			LogLevel:          grpcConfig.LogLevel,
			MaxRecvMsgSize:    grpcConfig.MaxRecvMsgSize,
			MaxSendMsgSize:    grpcConfig.MaxSendMsgSize,
			ConnectionTimeout: grpcConfig.ConnectionTimeout,
		}

		// 创建 zap logger 用于 gRPC
		zapLogger, err := createZapLogger(env)
		if err != nil {
			configures.Logger.Panic(fmt.Sprintf("创建 zap logger 失败: %v", err))
		}

		// 创建 gRPC Server
		grpcServer := grpc.NewServer(grpcServerConfig, zapLogger)

		// 设置客户端凭证鉴权提供者
		tokenParser := appRoot.UserModule.NewTokenParser()
		authProvider := grpc.NewClientAuthProvider(tokenParser, zapLogger)
		grpcServer.SetAuthProvider(authProvider)

		base.GRPCServer = grpcServer

		// 注册客户端认证服务
		if err := grpcServer.RegisterService(appRoot.UserModule.GRPCService); err != nil {
			configures.Logger.Panic(fmt.Sprintf("注册客户端认证服务失败: %v", err))
		}

		// 注册 config 组件的 gRPC 服务
		if err := grpcServer.RegisterService(appRoot.ConfigModule.GRPCService); err != nil {
			configures.Logger.Panic(fmt.Sprintf("注册配置服务失败: %v", err))
		}

		// 注册 registry 组件的 gRPC 服务
		if err := grpcServer.RegisterService(appRoot.RegistryModule.GRPCService); err != nil {
			configures.Logger.Panic(fmt.Sprintf("注册注册中心服务失败: %v", err))
		}

		// 注册 application 组件的 gRPC 服务
		if err := grpcServer.RegisterService(appRoot.ApplicationModule.GRPCService); err != nil {
			configures.Logger.Panic(fmt.Sprintf("注册应用部署服务失败: %v", err))
		}

		// 注册 server 组件的 gRPC 服务
		if err := grpcServer.RegisterService(appRoot.ServerModule.GRPCService); err != nil {
			configures.Logger.Panic(fmt.Sprintf("注册服务器管理服务失败: %v", err))
		}

		// 注册短网址组件的 gRPC 服务
		if err := grpcServer.RegisterService(appRoot.ShortURLModule.GRPCService); err != nil {
			configures.Logger.Panic(fmt.Sprintf("注册短网址服务失败: %v", err))
		}

		// 为短网址的 ReportSuccess 方法添加到跳过鉴权列表
		grpcServer.EnableAuth([]string{"/shorturl.v1.ShortURLService/ReportSuccess"})

		// 启动 gRPC 服务器
		if err := grpcServer.Start(); err != nil {
			configures.Logger.Panic(fmt.Sprintf("启动 gRPC 服务器失败: %v", err))
		}

		configures.Logger.Info(fmt.Sprintf("gRPC 服务器已启动，监听地址: %s", grpcConfig.Address))
	}

	log.Fatal(fiberApp.Listen(fmt.Sprintf(":%d", base.Configures.Config.Port)))
}

func getBaseInfo() (string, string) {
	// 定义命令行参数
	env := flag.String("env", "dev", "环境配置 (dev, prod, test等)")
	configFile := flag.String("config", "", "配置文件路径，默认为 ./resources/{env}.yaml")

	// 解析命令行参数
	flag.Parse()

	// 如果没有指定配置文件路径，则使用默认路径
	var filename string
	if *configFile == "" {
		getwd, err := os.Getwd()
		if err != nil {
			panic(fmt.Sprintf("获取当前文件位置失败,因为：%v", err))
		}
		filename = getwd + "/resources/" + *env + ".yaml"
	} else {
		filename = *configFile
	}
	return *env, filename
}

// createZapLogger 创建 zap logger 用于 gRPC
func createZapLogger(env string) (*zap.Logger, error) {
	var config zap.Config

	if env == "prod" {
		// 生产环境配置
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	} else {
		// 开发/测试环境配置
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	// 设置时间格式
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return config.Build()
}

// initAgentClient 初始化 Agent 客户端
func initAgentClient() *agentclient.AgentClient {
	// Token 提供者：从 ClientAuth 获取当前有效的 JWT token
	tokenProvider := func() string {
		// 这里返回一个系统级 token（由配置或环境变量提供）
		// 在实际调用时，可以从 context 或请求中获取用户的 token
		// 目前先返回空，agent client 在使用时会检查 context 中的 token
		return ""
	}

	// 默认 30 秒超时
	return agentclient.NewAgentClient(tokenProvider, 30*time.Second)
}
