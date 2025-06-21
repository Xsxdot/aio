// Package monitoring 提供一个集成式的服务器监控系统
package monitoring

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/monitoring/alerting"
	collector2 "github.com/xsxdot/aio/pkg/monitoring/collector"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"github.com/xsxdot/aio/pkg/scheduler"

	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/notifier"
	notifierstorage "github.com/xsxdot/aio/pkg/notifier/storage"

	"github.com/xsxdot/aio/app/config"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/etcd"
)

// Config 定义监控系统的配置选项
type Config struct {
	// DataDir 指定数据存储的目录
	DataDir string `json:"data_dir" yaml:"data_dir"`

	config.MonitorConfig

	// ServiceName 服务名称
	ServiceName string `json:"service_name" yaml:"service_name"`

	// InstanceID 实例ID
	InstanceID string `json:"instance_id" yaml:"instance_id"`

	// EtcdAlertPrefix 指定存储告警规则的etcd前缀
	EtcdAlertPrefix string `json:"etcd_alert_prefix" yaml:"etcd_alert_prefix"`

	// EtcdNotifierPrefix 指定存储通知器配置的etcd前缀
	EtcdNotifierPrefix string `json:"etcd_notifier_prefix" yaml:"etcd_notifier_prefix"`

	// Logger 日志记录器
	Logger *zap.Logger `json:"logger" yaml:"logger"`
}

// Monitor 代表一个监控系统实例
type Monitor struct {
	config Config
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 内部组件
	etcdClient          *etcd.EtcdClient
	storage             *storage.Storage
	notifierMgr         *notifier.Manager
	notifierIntegration *alerting.NotifierIntegration
	alertMgr            *alerting.Manager
	scheduler           *scheduler.Scheduler
	grpcStorage         *storage.GrpcStorage

	// 各类收集器
	serverCollector *collector2.ServerCollector
	appCollector    *collector2.AppCollector
	apiCollector    *collector2.APICollector
}

// New 创建一个新的监控系统实例
func New(cfg *config.BaseConfig, client *etcd.EtcdClient, scheduler *scheduler.Scheduler) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Monitor{
		ctx:        ctx,
		cancel:     cancel,
		etcdClient: client,
		config: Config{
			DataDir:            filepath.Join(cfg.System.DataDir, "monitoring"),
			MonitorConfig:      *cfg.Monitor,
			EtcdAlertPrefix:    "/monitoring/alerts/",
			EtcdNotifierPrefix: "/monitoring/notifiers/",
			Logger:             common.GetLogger().GetZapLogger("aio-monitoring"),
		},
		scheduler: scheduler,
	}
}

// Start 启动监控系统的所有组件
func (m *Monitor) Start(ctx context.Context) error {
	var err error

	// 1. 初始化存储引擎
	storageConfig := storage.Config{
		DataDir:       m.config.DataDir,
		RetentionDays: m.config.RetentionDays,
		Logger:        m.config.Logger,
	}
	m.storage, err = storage.New(storageConfig)
	if err != nil {
		return fmt.Errorf("初始化存储引擎失败: %w", err)
	}
	if err := m.storage.Start(); err != nil {
		return fmt.Errorf("启动存储引擎失败: %w", err)
	}

	// 2. 初始化通知管理器
	etcdStorage, err := notifierstorage.NewEtcdStorage(notifierstorage.EtcdStorageConfig{
		Client: m.etcdClient,
		Prefix: m.config.EtcdNotifierPrefix,
		Logger: m.config.Logger,
	})
	if err != nil {
		m.storage.Stop()
		return fmt.Errorf("创建通知器存储失败: %w", err)
	}

	m.notifierMgr, err = notifier.NewManager(notifier.ManagerConfig{
		Storage:       etcdStorage,
		Factory:       notifier.NewDefaultFactory(),
		Logger:        m.config.Logger,
		EnableWatcher: true,
		SendTimeout:   30 * time.Second,
	})
	if err != nil {
		m.storage.Stop()
		return fmt.Errorf("创建通知管理器失败: %w", err)
	}

	if err := m.notifierMgr.Start(); err != nil {
		m.storage.Stop()
		return fmt.Errorf("启动通知管理器失败: %w", err)
	}

	// 3. 创建通知器集成适配器
	m.notifierIntegration = alerting.NewNotifierIntegration(m.notifierMgr, m.config.Logger)

	// 4. 初始化告警管理器
	alertConfig := alerting.Config{
		EtcdClient:  m.etcdClient,
		Storage:     m.storage,
		AlertPrefix: m.config.EtcdAlertPrefix,
		Logger:      m.config.Logger,
	}
	m.alertMgr = alerting.New(alertConfig, m.notifierIntegration)
	if err := m.alertMgr.Start(); err != nil {
		m.notifierMgr.Stop()
		m.storage.Stop()
		return fmt.Errorf("启动告警管理器失败: %w", err)
	}

	// 4. 初始化各类数据采集器

	// 服务器指标收集器
	serverCollectorConfig := collector2.ServerCollectorConfig{
		CollectInterval: m.config.CollectInterval,
		Logger:          m.config.Logger,
		Storage:         m.storage,
		Scheduler:       m.scheduler,
	}
	m.serverCollector = collector2.NewServerCollector(serverCollectorConfig)
	if err := m.serverCollector.Start(); err != nil {
		m.alertMgr.Stop()
		m.notifierMgr.Stop()
		m.storage.Stop()
		return fmt.Errorf("启动服务器指标采集器失败: %w", err)
	}

	// API指标收集器
	apiCollectorConfig := collector2.APICollectorConfig{
		ServiceName: m.config.ServiceName,
		InstanceID:  m.config.InstanceID,
		Logger:      m.config.Logger,
		Storage:     m.storage,
	}
	m.apiCollector = collector2.NewAPICollector(apiCollectorConfig)
	if err := m.apiCollector.Start(); err != nil {
		if m.appCollector != nil {
			m.appCollector.Stop()
		}
		m.serverCollector.Stop()
		m.alertMgr.Stop()
		m.notifierMgr.Stop()
		m.storage.Stop()
		return fmt.Errorf("启动API指标采集器失败: %w", err)
	}

	m.config.Logger.Info("监控系统已启动")
	return nil
}

// Stop 停止监控系统的所有组件
func (m *Monitor) Stop(ctx context.Context) error {
	// 按照与启动相反的顺序停止组件

	// 停止各类收集器
	if m.apiCollector != nil {
		m.apiCollector.Stop()
	}

	if m.appCollector != nil {
		m.appCollector.Stop()
	}

	if m.serverCollector != nil {
		m.serverCollector.Stop()
	}

	if m.alertMgr != nil {
		m.alertMgr.Stop()
	}

	if m.notifierMgr != nil {
		m.notifierMgr.Stop()
	}

	if m.storage != nil {
		m.storage.Stop()
	}

	m.cancel()
	m.wg.Wait()
	m.config.Logger.Info("监控系统已停止")
	return nil
}

// SetGrpcStorage 设置grpc存储引擎实例
func (m *Monitor) SetGrpcStorage(grpcStorage *storage.GrpcStorage) {
	m.grpcStorage = grpcStorage
}

// GetStorage 返回存储引擎实例
func (m *Monitor) GetStorage() *storage.Storage {
	return m.storage
}

// GetGrpcStorage 返回grpc存储引擎实例
func (m *Monitor) GetGrpcStorage() *storage.GrpcStorage {
	return m.grpcStorage
}

// GetAlertManager 返回告警管理器实例
func (m *Monitor) GetAlertManager() *alerting.Manager {
	return m.alertMgr
}

// GetNotifierManager 返回通知管理器实例
func (m *Monitor) GetNotifierManager() *notifier.Manager {
	return m.notifierMgr
}

// GetNotifierIntegration 返回通知器集成适配器
func (m *Monitor) GetNotifierIntegration() *alerting.NotifierIntegration {
	return m.notifierIntegration
}

// GetServerCollector 返回服务器数据采集器实例
func (m *Monitor) GetServerCollector() *collector2.ServerCollector {
	return m.serverCollector
}

// GetAppCollector 返回应用数据采集器实例
func (m *Monitor) GetAppCollector() *collector2.AppCollector {
	return m.appCollector
}

// GetAPICollector 返回API数据采集器实例
func (m *Monitor) GetAPICollector() *collector2.APICollector {
	return m.apiCollector
}

// StartAppCollector 启动应用指标收集器
func (m *Monitor) StartAppCollector(serviceName, instanceID string) error {
	if m.appCollector != nil {
		m.config.Logger.Warn("应用指标收集器已经启动，跳过重复启动")
		return nil
	}

	// 应用指标收集器配置
	appCollectorConfig := collector2.AppCollectorConfig{
		ServiceName:     serviceName,
		InstanceID:      instanceID,
		CollectInterval: m.config.CollectInterval,
		Logger:          m.config.Logger,
		Storage:         m.storage,
		Scheduler:       m.scheduler,
	}

	m.appCollector = collector2.NewAppCollector(appCollectorConfig)
	if err := m.appCollector.Start(); err != nil {
		return fmt.Errorf("启动应用指标采集器失败: %w", err)
	}

	m.config.Logger.Info("应用指标采集器已启动",
		zap.String("service_name", serviceName),
		zap.String("instance_id", instanceID))

	return nil
}
