// Package monitoring 提供一个集成式的服务器和应用指标监控系统
package monitoring

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"path/filepath"
	"sync"

	"github.com/xsxdot/aio/app/config"
	"github.com/xsxdot/aio/internal/monitoring/alerting"
	"github.com/xsxdot/aio/internal/monitoring/collector"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"github.com/xsxdot/aio/internal/monitoring/notifier"
	"github.com/xsxdot/aio/internal/monitoring/storage"

	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/etcd"
)

// Config 定义监控系统的配置选项
type Config struct {
	// DataDir 指定数据存储的目录
	DataDir string `json:"data_dir" yaml:"data_dir"`

	config.MonitorConfig

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
	etcdClient  *etcd.EtcdClient
	collector   *collector.Collector
	storage     *storage.Storage
	notifierMgr *notifier.Manager
	alertMgr    *alerting.Manager
}

// New 创建一个新的监控系统实例
func New(cfg *config.BaseConfig, client *etcd.EtcdClient) *Monitor {
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
	notifierConfig := notifier.Config{
		EtcdClient:     m.etcdClient,
		NotifierPrefix: m.config.EtcdNotifierPrefix,
		Logger:         m.config.Logger,
	}
	m.notifierMgr = notifier.New(notifierConfig)
	if err := m.notifierMgr.Start(); err != nil {
		m.storage.Stop()
		return fmt.Errorf("启动通知管理器失败: %w", err)
	}

	// 3. 初始化告警管理器
	alertConfig := alerting.Config{
		EtcdClient:  m.etcdClient,
		Storage:     m.storage,
		AlertPrefix: m.config.EtcdAlertPrefix,
		Logger:      m.config.Logger,
	}
	m.alertMgr = alerting.New(alertConfig, m.notifierMgr)
	if err := m.alertMgr.Start(); err != nil {
		m.notifierMgr.Stop()
		m.storage.Stop()
		return fmt.Errorf("启动告警管理器失败: %w", err)
	}

	// 4. 初始化数据采集器
	collectorConfig := collector.Config{
		CollectInterval: m.config.CollectInterval,
		Logger:          m.config.Logger,
	}
	m.collector = collector.New(collectorConfig)

	// 将存储引擎注册为数据处理器
	m.collector.RegisterHandler(m.createStorageHandler())

	if err := m.collector.Start(); err != nil {
		m.alertMgr.Stop()
		m.notifierMgr.Stop()
		m.storage.Stop()
		return fmt.Errorf("启动数据采集器失败: %w", err)
	}

	m.config.Logger.Info("监控系统已启动")
	return nil
}

// Stop 停止监控系统的所有组件
func (m *Monitor) Stop(ctx context.Context) error {
	// 按照与启动相反的顺序停止组件
	if m.collector != nil {
		m.collector.Stop()
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

// GetStorage 返回存储引擎实例
func (m *Monitor) GetStorage() *storage.Storage {
	return m.storage
}

// GetAlertManager 返回告警管理器实例
func (m *Monitor) GetAlertManager() *alerting.Manager {
	return m.alertMgr
}

// GetNotifierManager 返回通知管理器实例
func (m *Monitor) GetNotifierManager() *notifier.Manager {
	return m.notifierMgr
}

// GetCollector 返回数据采集器实例
func (m *Monitor) GetCollector() *collector.Collector {
	return m.collector
}

// createStorageHandler 创建一个存储处理器
func (m *Monitor) createStorageHandler() collector.MetricHandler {
	return &storageHandler{
		storage: m.storage,
	}
}

// storageHandler 实现了MetricHandler接口
type storageHandler struct {
	storage *storage.Storage
}

// HandleServerMetrics 处理服务器指标
func (h *storageHandler) HandleServerMetrics(metrics *models2.ServerMetrics) error {
	return h.storage.StoreServerMetrics(metrics)
}

// HandleAPICalls 处理API调用信息
func (h *storageHandler) HandleAPICalls(calls *models2.APICalls) error {
	return h.storage.StoreAPICalls(calls)
}

// HandleAppMetrics 处理应用状态指标
func (h *storageHandler) HandleAppMetrics(metrics *models2.AppMetrics) error {
	return h.storage.StoreAppMetrics(metrics)
}

// HandleServiceData 处理应用服务数据
func (h *storageHandler) HandleServiceData(data *models2.ServiceData) error {
	return h.storage.StoreServiceData(data)
}

// HandleServiceAPIData 处理应用服务API调用数据
func (h *storageHandler) HandleServiceAPIData(data *models2.ServiceAPIData) error {
	return h.storage.StoreServiceAPIData(data)
}
