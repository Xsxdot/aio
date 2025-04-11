package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"

	"github.com/xsxdot/aio/internal/monitoring/models"
)

// MonitoringClient 监控客户端，用于发送应用和服务监控指标
type MonitoringClient struct {
	client             *Client
	natsConn           *nats.Conn
	log                *zap.Logger
	serviceInfo        *ServiceInfo
	metricsSubject     string
	serviceSubject     string
	processID          int32
	hostname           string
	startTime          time.Time
	memStatsTicker     *time.Ticker
	memStatsCollection bool
	memStatsMutex      sync.RWMutex
	lastMemStats       runtime.MemStats
	apiCallBuffer      []models.APICall
	apiCallMutex       sync.Mutex
	apiFlushTicker     *time.Ticker
	ctx                context.Context
	cancel             context.CancelFunc
}

// MonitoringOptions 监控客户端配置选项
type MonitoringOptions struct {
	// MetricsSubject 指定用于发送应用指标的NATS主题前缀，默认为"metrics"
	MetricsSubject string

	// ServiceSubject 指定用于发送服务监控数据的NATS主题前缀，默认为"service.metrics"
	ServiceSubject string

	// MemStatsInterval 指定内存统计信息收集间隔，默认为30秒
	MemStatsInterval time.Duration

	// APIFlushInterval 指定API调用数据发送间隔，默认为10秒
	APIFlushInterval time.Duration

	// APIBufferSize 指定API调用缓冲区大小，达到此大小时也会触发发送，默认为100
	APIBufferSize int
}

// DefaultMonitoringOptions 返回默认的监控配置选项
func DefaultMonitoringOptions() *MonitoringOptions {
	return &MonitoringOptions{
		MetricsSubject:   "metrics",
		ServiceSubject:   "service.metrics",
		MemStatsInterval: 30 * time.Second,
		APIFlushInterval: 10 * time.Second,
		APIBufferSize:    100,
	}
}

// NewMonitoringClient 创建一个新的监控客户端
func NewMonitoringClient(client *Client, options *MonitoringOptions) (*MonitoringClient, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	if client.Nats == nil {
		return nil, fmt.Errorf("nats client must be initialized first")
	}

	if client.GetServiceInfo() == nil {
		return nil, fmt.Errorf("service info not available")
	}

	if options == nil {
		options = DefaultMonitoringOptions()
	}

	// 获取主机名
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &MonitoringClient{
		client:         client,
		natsConn:       client.Nats.natsClient.GetConn(),
		log:            client.log.Named("monitoring"),
		serviceInfo:    client.GetServiceInfo(),
		metricsSubject: options.MetricsSubject,
		serviceSubject: options.ServiceSubject,
		processID:      int32(os.Getpid()),
		hostname:       hostname,
		startTime:      time.Now(),
		apiCallBuffer:  make([]models.APICall, 0, options.APIBufferSize),
		ctx:            ctx,
		cancel:         cancel,
	}

	// 初始化后台任务
	m.memStatsTicker = time.NewTicker(options.MemStatsInterval)
	m.apiFlushTicker = time.NewTicker(options.APIFlushInterval)

	// 启动后台协程收集和发送指标
	go m.collectAndSendMetrics()
	go m.flushAPICallsRoutine()

	m.log.Info("监控客户端已初始化",
		zap.String("service", m.serviceInfo.Name),
		zap.String("instance", m.serviceInfo.ID),
		zap.String("metricsSubject", m.metricsSubject),
		zap.String("serviceSubject", m.serviceSubject))

	return m, nil
}

// Stop 停止监控客户端
func (m *MonitoringClient) Stop() {
	m.cancel()
	m.memStatsTicker.Stop()
	m.apiFlushTicker.Stop()

	// 确保所有API调用都被发送
	m.flushAPICalls()

	m.log.Info("监控客户端已停止")
}

// TrackAPICall 追踪API调用指标
func (m *MonitoringClient) TrackAPICall(endpoint, method string, startTime time.Time, statusCode int,
	hasError bool, errorMsg string, requestSize, responseSize int64, clientIP string, tags map[string]string) {

	duration := time.Since(startTime).Milliseconds()

	apiCall := models.APICall{
		Endpoint:     endpoint,
		Method:       method,
		Timestamp:    startTime,
		DurationMs:   float64(duration),
		StatusCode:   statusCode,
		HasError:     hasError,
		ErrorMessage: errorMsg,
		RequestSize:  requestSize,
		ResponseSize: responseSize,
		ClientIP:     clientIP,
		Tags:         tags,
	}

	m.apiCallMutex.Lock()
	m.apiCallBuffer = append(m.apiCallBuffer, apiCall)
	bufferSize := len(m.apiCallBuffer)
	m.apiCallMutex.Unlock()

	// 如果缓冲区满了，立即发送
	if bufferSize >= cap(m.apiCallBuffer) {
		m.flushAPICalls()
	}
}

// SendCustomAppMetrics 发送自定义应用指标
func (m *MonitoringClient) SendCustomAppMetrics(metrics map[string]interface{}) error {
	appMetrics := models.AppMetrics{
		Source:    m.serviceInfo.Name,
		Instance:  m.serviceInfo.ID,
		Timestamp: time.Now(),
	}

	// 收集标准的运行时指标
	m.collectRuntimeMetrics(&appMetrics)

	// 注意：这里需要实现自定义指标到AppMetrics的映射逻辑
	// 由于AppMetrics结构体有固定字段，这里只作为示例
	// 实际使用时可能需要扩展AppMetrics或使用其他方式支持自定义指标

	// 发送指标
	return m.sendAppMetrics(&appMetrics)
}

// SendServiceData 发送服务监控数据
func (m *MonitoringClient) SendServiceData(customMetrics interface{}) error {
	serviceData := models.ServiceData{
		Source:     m.serviceInfo.Name,
		Instance:   m.serviceInfo.ID,
		IP:         m.serviceInfo.LocalIP,
		Port:       m.serviceInfo.Port,
		Version:    m.serviceInfo.Metadata["version"],
		Tags:       m.serviceInfo.Metadata,
		Timestamp:  time.Now(),
		AppMetrics: customMetrics,
	}

	return m.sendServiceData(&serviceData)
}

// collectAndSendMetrics 后台收集并发送应用指标
func (m *MonitoringClient) collectAndSendMetrics() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.memStatsTicker.C:
			appMetrics := models.AppMetrics{
				Source:    m.serviceInfo.Name,
				Instance:  m.serviceInfo.ID,
				Timestamp: time.Now(),
			}

			m.collectRuntimeMetrics(&appMetrics)

			if err := m.sendAppMetrics(&appMetrics); err != nil {
				m.log.Error("发送应用指标失败", zap.Error(err))
			}
		}
	}
}

// flushAPICallsRoutine 定期发送API调用数据
func (m *MonitoringClient) flushAPICallsRoutine() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.apiFlushTicker.C:
			m.flushAPICalls()
		}
	}
}

// flushAPICalls 发送缓冲的API调用数据
func (m *MonitoringClient) flushAPICalls() {
	m.apiCallMutex.Lock()
	if len(m.apiCallBuffer) == 0 {
		m.apiCallMutex.Unlock()
		return
	}

	// 创建要发送的副本并清空缓冲区
	apiCalls := make([]models.APICall, len(m.apiCallBuffer))
	copy(apiCalls, m.apiCallBuffer)
	m.apiCallBuffer = m.apiCallBuffer[:0]
	m.apiCallMutex.Unlock()

	// 准备要发送的数据
	calls := models.APICalls{
		Source:    m.serviceInfo.Name,
		Instance:  m.serviceInfo.ID,
		Timestamp: time.Now(),
		Calls:     apiCalls,
	}

	// 发送API调用数据到NATS
	if err := m.sendAPICalls(&calls); err != nil {
		m.log.Error("发送API调用数据失败",
			zap.Error(err),
			zap.Int("callCount", len(apiCalls)))
	} else {
		m.log.Debug("已发送API调用数据",
			zap.Int("callCount", len(apiCalls)))
	}
}

// collectRuntimeMetrics 收集运行时指标
func (m *MonitoringClient) collectRuntimeMetrics(appMetrics *models.AppMetrics) {
	// 收集Go运行时内存统计信息
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 保存最新的内存统计信息
	m.memStatsMutex.Lock()
	m.lastMemStats = memStats
	m.memStatsMutex.Unlock()

	// 设置内存指标
	appMetrics.Metrics.Memory.TotalMB = float64(memStats.Sys) / 1024 / 1024
	appMetrics.Metrics.Memory.UsedMB = float64(memStats.Alloc) / 1024 / 1024
	appMetrics.Metrics.Memory.HeapMB = float64(memStats.HeapAlloc) / 1024 / 1024
	appMetrics.Metrics.Memory.NonHeapMB = float64(memStats.HeapSys-memStats.HeapAlloc) / 1024 / 1024
	appMetrics.Metrics.Memory.GCCount = int(memStats.NumGC)

	// 收集进程信息
	p, err := process.NewProcess(m.processID)
	if err == nil {
		// CPU使用率
		cpuPercent, err := p.CPUPercent()
		if err == nil {
			appMetrics.Metrics.CPUUsagePercent = cpuPercent
		}

		// 线程信息
		threadCount, _ := runtime.ThreadCreateProfile(nil)
		appMetrics.Metrics.Threads.Total = threadCount
		appMetrics.Metrics.Threads.Active = runtime.NumGoroutine()
	}

	// 其他Go运行时指标
	appMetrics.Metrics.ClassLoaded = runtime.NumCPU() // 用CPU数量作为示例，实际Go没有类加载概念
}

// sendAppMetrics 发送应用指标
func (m *MonitoringClient) sendAppMetrics(metrics *models.AppMetrics) error {
	// 序列化指标数据
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("序列化应用指标失败: %w", err)
	}

	// 发送到NATS
	subject := fmt.Sprintf("%s.app", m.metricsSubject)
	err = m.natsConn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("发送应用指标失败: %w", err)
	}

	m.log.Debug("已发送应用指标",
		zap.String("subject", subject),
		zap.Float64("memoryUsedMB", metrics.Metrics.Memory.UsedMB),
		zap.Float64("cpuPercent", metrics.Metrics.CPUUsagePercent))

	return nil
}

// sendAPICalls 发送API调用数据
func (m *MonitoringClient) sendAPICalls(calls *models.APICalls) error {
	// 序列化API调用数据
	data, err := json.Marshal(calls)
	if err != nil {
		return fmt.Errorf("序列化API调用数据失败: %w", err)
	}

	// 发送到NATS
	subject := fmt.Sprintf("%s.api", m.metricsSubject)
	err = m.natsConn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("发送API调用数据失败: %w", err)
	}

	return nil
}

// sendServiceData 发送服务监控数据
func (m *MonitoringClient) sendServiceData(data *models.ServiceData) error {
	// 序列化服务数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化服务数据失败: %w", err)
	}

	// 发送到NATS
	subject := fmt.Sprintf("%s.data", m.serviceSubject)
	err = m.natsConn.Publish(subject, jsonData)
	if err != nil {
		return fmt.Errorf("发送服务数据失败: %w", err)
	}

	m.log.Debug("已发送服务数据",
		zap.String("subject", subject),
		zap.String("service", data.Source),
		zap.String("instance", data.Instance))

	return nil
}

// SendServiceAPIData 发送服务API调用数据
func (m *MonitoringClient) SendServiceAPIData(apiCalls []models.APICall) error {
	if len(apiCalls) == 0 {
		return nil
	}

	serviceAPIData := models.ServiceAPIData{
		Source:    m.serviceInfo.Name,
		Instance:  m.serviceInfo.ID,
		IP:        m.serviceInfo.LocalIP,
		Port:      m.serviceInfo.Port,
		Timestamp: time.Now(),
		APICalls:  apiCalls,
	}

	// 序列化服务API调用数据
	jsonData, err := json.Marshal(serviceAPIData)
	if err != nil {
		return fmt.Errorf("序列化服务API调用数据失败: %w", err)
	}

	// 发送到NATS
	subject := fmt.Sprintf("%s.api", m.serviceSubject)
	err = m.natsConn.Publish(subject, jsonData)
	if err != nil {
		return fmt.Errorf("发送服务API调用数据失败: %w", err)
	}

	m.log.Debug("已发送服务API调用数据",
		zap.String("subject", subject),
		zap.String("service", serviceAPIData.Source),
		zap.String("instance", serviceAPIData.Instance),
		zap.Int("callCount", len(apiCalls)))

	return nil
}
