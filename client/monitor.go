package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/monitoring/collector"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/scheduler"
	"go.uber.org/zap"
)

type MonitorClient struct {
	instance      *registry.ServiceInstance
	manager       *GRPCClientManager
	scheduler     *scheduler.Scheduler
	storageClient *MonitoringStorageClient

	// 收集器实例
	appCollector *collector.AppCollector
	apiCollector *collector.APICollector

	// API指标缓存队列
	apiMetricsQueue []*collector.APICallMetrics
	queueMutex      sync.RWMutex
	logger          *zap.Logger
	flushTask       scheduler.Task // 定时刷新任务

	// 批量发送配置
	maxBatchSize  int           // 最大批量大小，默认100
	flushInterval time.Duration // 刷新间隔，默认15秒
}

func NewMonitorClient(serviceInfo *registry.ServiceInstance, manager *GRPCClientManager, scheduler *scheduler.Scheduler) *MonitorClient {
	logger := common.GetLogger().GetZapLogger("aio-client-monitoring")
	storageClient := NewMonitoringStorageClientFromManager(manager, serviceInfo.Name, logger)

	client := &MonitorClient{
		instance:        serviceInfo,
		manager:         manager,
		scheduler:       scheduler,
		storageClient:   storageClient,
		apiMetricsQueue: make([]*collector.APICallMetrics, 0, 100),
		logger:          logger,
		maxBatchSize:    100,
		flushInterval:   15 * time.Second,
	}

	// 创建应用收集器
	appConfig := collector.AppCollectorConfig{
		ServiceName:     serviceInfo.Name,
		InstanceID:      serviceInfo.ID,
		CollectInterval: 30, // 30秒采集一次应用指标
		Logger:          logger,
		Storage:         storageClient,
		Scheduler:       scheduler,
	}
	client.appCollector = collector.NewAppCollector(appConfig)

	// 创建API收集器
	apiConfig := collector.APICollectorConfig{
		ServiceName: serviceInfo.Name,
		InstanceID:  serviceInfo.ID,
		Logger:      logger,
		Storage:     storageClient,
	}
	client.apiCollector = collector.NewAPICollector(apiConfig)

	// 创建定时刷新任务
	client.createFlushTask()

	return client
}

// createFlushTask 创建定时刷新任务
func (m *MonitorClient) createFlushTask() {
	taskName := fmt.Sprintf("api-metrics-flush-%s-%s", m.instance.Name, m.instance.ID)

	m.flushTask = scheduler.NewIntervalTask(
		taskName,
		time.Now(),
		m.flushInterval,
		scheduler.TaskExecuteModeLocal, // 本地任务
		30*time.Second,                 // 任务超时时间
		m.flushAPIMetricsTask,
	)
}

// flushAPIMetricsTask 定时刷新任务函数
func (m *MonitorClient) flushAPIMetricsTask(ctx context.Context) error {
	m.flushAPIMetrics()
	return nil
}

// Start 启动监控客户端
func (m *MonitorClient) Start() error {
	m.logger.Info("启动监控客户端",
		zap.String("service_name", m.instance.Name),
		zap.String("instance_id", m.instance.ID))

	// 启动应用收集器
	if err := m.appCollector.Start(); err != nil {
		m.logger.Error("启动应用收集器失败", zap.Error(err))
		return err
	}

	// 启动API收集器
	if err := m.apiCollector.Start(); err != nil {
		m.logger.Error("启动API收集器失败", zap.Error(err))
		return err
	}

	// 启动定时刷新任务
	if m.scheduler != nil && m.flushTask != nil {
		if err := m.scheduler.AddTask(m.flushTask); err != nil {
			m.logger.Error("启动定时刷新任务失败", zap.Error(err))
			return err
		}
	}

	return nil
}

// Stop 停止监控客户端
func (m *MonitorClient) Stop() error {
	m.logger.Info("停止监控客户端")

	// 停止定时刷新任务
	if m.scheduler != nil && m.flushTask != nil {
		m.scheduler.RemoveTask(m.flushTask.GetID())
	}

	// 发送剩余的指标数据
	m.flushAPIMetrics()

	// 停止收集器
	if m.appCollector != nil {
		m.appCollector.Stop()
	}
	if m.apiCollector != nil {
		m.apiCollector.Stop()
	}

	return nil
}

// RecordAPICall 记录API调用指标（对外接口）
func (m *MonitorClient) RecordAPICall(apiMetrics *collector.APICallMetrics) {
	if apiMetrics == nil {
		return
	}

	// 补充服务信息
	if apiMetrics.ServiceName == "" {
		apiMetrics.ServiceName = m.instance.Name
	}
	if apiMetrics.InstanceID == "" {
		apiMetrics.InstanceID = m.instance.ID
	}
	if apiMetrics.Timestamp.IsZero() {
		apiMetrics.Timestamp = time.Now()
	}

	m.queueMutex.Lock()
	defer m.queueMutex.Unlock()

	// 添加到队列
	m.apiMetricsQueue = append(m.apiMetricsQueue, apiMetrics)

	// 检查是否达到批量大小
	if len(m.apiMetricsQueue) >= m.maxBatchSize {
		go m.flushAPIMetrics()
	}
}

// RecordAPICallBatch 批量记录API调用指标（对外接口）
func (m *MonitorClient) RecordAPICallBatch(apiMetricsList []*collector.APICallMetrics) {
	if len(apiMetricsList) == 0 {
		return
	}

	now := time.Now()

	m.queueMutex.Lock()
	defer m.queueMutex.Unlock()

	// 补充服务信息并添加到队列
	for _, apiMetrics := range apiMetricsList {
		if apiMetrics == nil {
			continue
		}

		if apiMetrics.ServiceName == "" {
			apiMetrics.ServiceName = m.instance.Name
		}
		if apiMetrics.InstanceID == "" {
			apiMetrics.InstanceID = m.instance.ID
		}
		if apiMetrics.Timestamp.IsZero() {
			apiMetrics.Timestamp = now
		}

		m.apiMetricsQueue = append(m.apiMetricsQueue, apiMetrics)
	}

	// 检查是否达到批量大小
	if len(m.apiMetricsQueue) >= m.maxBatchSize {
		go m.flushAPIMetrics()
	}
}

// flushAPIMetrics 刷新API指标数据
func (m *MonitorClient) flushAPIMetrics() {
	m.queueMutex.Lock()
	if len(m.apiMetricsQueue) == 0 {
		m.queueMutex.Unlock()
		return
	}

	// 复制队列数据并清空队列
	metricsToSend := make([]*collector.APICallMetrics, len(m.apiMetricsQueue))
	copy(metricsToSend, m.apiMetricsQueue)
	m.apiMetricsQueue = m.apiMetricsQueue[:0] // 清空队列但保持容量
	m.queueMutex.Unlock()

	// 批量发送
	if err := m.apiCollector.RecordBatch(metricsToSend); err != nil {
		m.logger.Error("批量发送API指标失败",
			zap.Int("count", len(metricsToSend)),
			zap.Error(err))

		// 发送失败时，可以考虑重新加入队列或记录到文件
		// 这里简单记录错误日志
	} else {
		m.logger.Debug("批量发送API指标成功",
			zap.Int("count", len(metricsToSend)))
	}
}

// SetBatchConfig 设置批量发送配置
func (m *MonitorClient) SetBatchConfig(maxBatchSize int, flushInterval time.Duration) {
	m.queueMutex.Lock()
	defer m.queueMutex.Unlock()

	needRecreateTask := false

	if maxBatchSize > 0 {
		m.maxBatchSize = maxBatchSize
	}
	if flushInterval > 0 && flushInterval != m.flushInterval {
		m.flushInterval = flushInterval
		needRecreateTask = true
	}

	// 如果刷新间隔改变，需要重新创建任务
	if needRecreateTask && m.scheduler != nil {
		// 移除旧任务
		if m.flushTask != nil {
			m.scheduler.RemoveTask(m.flushTask.GetID())
		}

		// 创建新任务
		m.createFlushTask()

		// 添加新任务
		if err := m.scheduler.AddTask(m.flushTask); err != nil {
			m.logger.Error("重新创建定时刷新任务失败", zap.Error(err))
		}
	}

	m.logger.Info("更新批量发送配置",
		zap.Int("max_batch_size", m.maxBatchSize),
		zap.Duration("flush_interval", m.flushInterval))
}

// GetQueueStatus 获取队列状态（用于监控）
func (m *MonitorClient) GetQueueStatus() map[string]interface{} {
	m.queueMutex.RLock()
	defer m.queueMutex.RUnlock()

	return map[string]interface{}{
		"queue_length":   len(m.apiMetricsQueue),
		"max_batch_size": m.maxBatchSize,
		"flush_interval": m.flushInterval.String(),
	}
}

// UpdateServiceInstance 更新服务实例信息
func (m *MonitorClient) UpdateServiceInstance(serviceInfo *registry.ServiceInstance) {
	m.instance = serviceInfo

	// 更新收集器的实例ID
	if m.appCollector != nil {
		m.appCollector.SetServiceInstanceID(serviceInfo.ID)
	}

	m.logger.Info("更新服务实例信息",
		zap.String("service_name", serviceInfo.Name),
		zap.String("instance_id", serviceInfo.ID))
}
