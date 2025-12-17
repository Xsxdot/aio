package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"

	"xiaozhizhang/pkg/core/config"
	"xiaozhizhang/pkg/core/logger"
	grpcpkg "xiaozhizhang/pkg/grpc"
	agentgrpc "xiaozhizhang/system/agent/external/grpc"
	"xiaozhizhang/system/agent/internal/service"
	usersvc "xiaozhizhang/system/user/internal/service"
	"xiaozhizhang/system/user"
)

// AgentConfig Agent 配置
type AgentConfig struct {
	Agent config.AgentConfig `yaml:"agent"`
	JWT   config.JWTConfig   `yaml:"jwt"`
}

func main() {
	// 解析命令行参数
	configFile := flag.String("config", "/etc/xiaozhizhang/agent.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := loadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 创建 zap logger
	zapLogger, err := createZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 logger 失败: %v\n", err)
		os.Exit(1)
	}
	defer zapLogger.Sync()

	// 创建 logrus logger（用于 agent 服务内部）
	logrusLogger := logger.GetLogger().WithEntryName("Agent")

	// 创建 JWT 服务（用于验证 token）
	jwtService := usersvc.NewJwtService(cfg.JWT.Secret, time.Duration(cfg.JWT.Expires)*time.Second, logrusLogger)

	// 创建 Token 解析器
	tokenParser := user.NewTokenParserAdapter(jwtService)

	// 创建鉴权提供者
	authProvider := grpcpkg.NewClientAuthProvider(tokenParser, zapLogger)

	// 创建 nginx 服务
	nginxSvc := service.NewNginxService(
		cfg.Agent.Nginx.RootDir,
		cfg.Agent.Nginx.FileMode,
		cfg.Agent.Nginx.ValidateCommand,
		cfg.Agent.Nginx.ReloadCommand,
		cfg.Agent.Timeout,
		logrusLogger,
	)

	// 创建 systemd 服务
	systemdSvc := service.NewSystemdService(
		cfg.Agent.Systemd.UnitDir,
		cfg.Agent.Timeout,
		logrusLogger,
	)

	// 创建 SSL 服务
	sslSvc := service.NewSSLService(nginxSvc, systemdSvc, logrusLogger)

	// 创建 Agent gRPC 服务
	agentGRPCService := agentgrpc.NewAgentGRPCService(nginxSvc, systemdSvc, sslSvc, logrusLogger)

	// 创建 gRPC 服务器
	grpcConfig := &grpcpkg.Config{
		Address:          cfg.Agent.Address,
		EnableReflection: true,
		EnableRecovery:   true,
		EnableValidation: false,
		EnableAuth:       true,
		EnablePermission: false,
		LogLevel:         "info",
	}

	grpcServer := grpcpkg.NewServer(grpcConfig, zapLogger)

	// 设置鉴权提供者
	grpcServer.SetAuthProvider(authProvider)

	// 注册 Agent 服务
	if err := grpcServer.RegisterService(agentGRPCService); err != nil {
		zapLogger.Fatal("注册 Agent 服务失败", zap.Error(err))
	}

	// 启动 gRPC 服务器
	if err := grpcServer.Start(); err != nil {
		zapLogger.Fatal("启动 gRPC 服务器失败", zap.Error(err))
	}

	zapLogger.Info("Agent 已启动", zap.String("address", cfg.Agent.Address))

	// 等待信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("Agent 正在关闭...")

	// 优雅关闭
	grpcServer.Stop()

	zapLogger.Info("Agent 已关闭")
}

func loadConfig(filename string) (*AgentConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.Agent.Address == "" {
		cfg.Agent.Address = ":50052"
	}
	if cfg.Agent.Timeout == 0 {
		cfg.Agent.Timeout = 30 * time.Second
	}
	if cfg.Agent.Nginx.RootDir == "" {
		cfg.Agent.Nginx.RootDir = "/etc/nginx/conf.d"
	}
	if cfg.Agent.Nginx.FileMode == "" {
		cfg.Agent.Nginx.FileMode = "0644"
	}
	if cfg.Agent.Nginx.ValidateCommand == "" {
		cfg.Agent.Nginx.ValidateCommand = "nginx -t"
	}
	if cfg.Agent.Nginx.ReloadCommand == "" {
		cfg.Agent.Nginx.ReloadCommand = "nginx -s reload"
	}
	if cfg.Agent.Systemd.UnitDir == "" {
		cfg.Agent.Systemd.UnitDir = "/etc/systemd/system"
	}

	return &cfg, nil
}

func createZapLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return config.Build()
}

