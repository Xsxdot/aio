// Package app 提供AIO服务的应用核心
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/xsxdot/aio/pkg/auth"
	config3 "github.com/xsxdot/aio/pkg/config"
	"github.com/xsxdot/aio/pkg/scheduler"
	"gopkg.in/yaml.v3"

	"github.com/xsxdot/aio/app/config"
	"github.com/xsxdot/aio/app/fiber"
	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/cache/server"
	"github.com/xsxdot/aio/internal/certmanager"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/internal/monitoring"
	"github.com/xsxdot/aio/internal/mq"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/distributed/election"
	"github.com/xsxdot/aio/pkg/protocol"
)

// App 表示应用程序核心
type App struct {
	// 配置
	BaseConfig    *config.BaseConfig
	configDirPath string // 配置文件目录路径
	mode          string
	nodeID        string
	mu            sync.Mutex // 保护并发访问

	Manager *ComponentManager

	// 核心组件
	Logger             *common.Logger
	AuthManager        *authmanager.AuthManager
	Protocol           *protocol.ProtocolManager
	CertificateManager *auth.CertificateManager
	Scheduler          *scheduler.Scheduler

	// 分布式基础组件
	Etcd          *etcd.EtcdComponent
	ConfigService *config3.Service
	Election      election.ElectionService
	Discovery     discovery.DiscoveryService

	// 可选服务组件
	MQServer    *mq.NatsServer
	CacheServer *server.Server
	Monitor     *monitoring.Monitor
	SSL         *certmanager.CertManager

	// Fiber
	FiberServer *fiber.Server
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
	a.mode = cfg.System.Mode
	a.nodeID = cfg.System.NodeId

	return nil
}

// Start 启动应用
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.BaseConfig == nil {
		return fmt.Errorf("未加载配置，无法启动应用")
	}
	a.Manager = NewComponentRegistry(a)

	// 应用启动逻辑，将在starter.go中实现
	return a.startApp()
}

// Stop 停止应用
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Println("正在停止应用...")

	err := a.Manager.StopAll(context.Background())
	if err != nil {
		return err
	}

	log.Println("应用已停止")
	return nil
}

// Restart 重启应用
func (a *App) Restart() error {
	a.Logger.Info("正在重启应用...")

	// 停止所有组件
	if err := a.Stop(); err != nil {
		return fmt.Errorf("停止应用失败: %w", err)
	}

	// 重新加载配置
	if err := a.LoadConfig(a.configDirPath); err != nil {
		return fmt.Errorf("重新加载配置失败: %w", err)
	}

	// 重新启动应用
	if err := a.Start(); err != nil {
		return fmt.Errorf("重新启动应用失败: %w", err)
	}

	a.Logger.Info("应用重启完成")
	return nil
}

func saveYAMLConfig(path string, config interface{}) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

func (a *App) initTcpApi() error {
	tcpapi := config3.NewTCPAPI(a.ConfigService, a.Logger.GetZapLogger("ConfigTcpApi"))
	tcpapi.RegisterToManager(a.Protocol)

	err := distributed.RegisterDiscoveryTCPHandlers(a.Discovery, a.Protocol)
	if err != nil {
		return err
	}

	return distributed.RegisterElectionTCPHandlers(a.Election, a.Protocol)
}
