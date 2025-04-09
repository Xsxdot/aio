package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// MetricsCollectorOptions 定义指标收集器的配置选项
type MetricsCollectorOptions struct {
	// 服务名称，将作为指标的来源标识
	ServiceName string
	// 应用实例ID，如果为空则自动生成
	InstanceID string
	// 应用版本
	Version string
	// 标签，用于分类和筛选
	Tags map[string]string
	// 状态指标收集间隔，默认30秒
	StatusCollectInterval time.Duration
	// API调用指标缓冲区大小，达到此数量后批量发送
	APIBufferSize int
	// API调用指标发送的最大间隔时间，即使未达到缓冲区大小也发送
	APIFlushInterval time.Duration
	// 是否自动采集应用状态指标
	AutoCollectStatus bool
	// 是否禁用指标收集
	DisableMetrics bool
}

// DefaultMetricsCollectorOptions 返回默认的指标收集器选项
func DefaultMetricsCollectorOptions() *MetricsCollectorOptions {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &MetricsCollectorOptions{
		ServiceName:           "app",
		InstanceID:            fmt.Sprintf("%s-%d", hostname, os.Getpid()),
		StatusCollectInterval: 30 * time.Second,
		APIBufferSize:         100,
		APIFlushInterval:      10 * time.Second,
		AutoCollectStatus:     true,
		DisableMetrics:        false,
	}
}

// APICall 表示单个API调用信息
type APICall struct {
	Endpoint     string            `json:"endpoint"`
	Method       string            `json:"method"`
	Timestamp    time.Time         `json:"timestamp"`
	DurationMs   float64           `json:"duration_ms"`
	StatusCode   int               `json:"status_code"`
	HasError     bool              `json:"has_error"`
	ErrorMessage string            `json:"error_message,omitempty"`
	RequestSize  int64             `json:"request_size_bytes"`
	ResponseSize int64             `json:"response_size_bytes"`
	ClientIP     string            `json:"client_ip"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// APICalls 表示一组API调用信息
type APICalls struct {
	Source    string    `json:"source"`
	Instance  string    `json:"instance"`
	IP        string    `json:"ip,omitempty"`
	Port      int       `json:"port,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Calls     []APICall `json:"api_calls"`
}

// AppMetrics 表示应用状态指标
type AppMetrics struct {
	Source    string            `json:"source"`
	Instance  string            `json:"instance"`
	IP        string            `json:"ip,omitempty"`
	Port      int               `json:"port,omitempty"`
	Version   string            `json:"version,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Metrics   struct {
		Memory struct {
			TotalMB   float64 `json:"total_mb"`
			UsedMB    float64 `json:"used_mb"`
			HeapMB    float64 `json:"heap_mb"`
			NonHeapMB float64 `json:"non_heap_mb"`
			GCCount   int     `json:"gc_count"`
			GCTimeMs  int     `json:"gc_time_ms"`
		} `json:"memory"`
		Threads struct {
			Total   int `json:"total"`
			Active  int `json:"active"`
			Blocked int `json:"blocked"`
			Waiting int `json:"waiting"`
		} `json:"threads"`
		CPUUsagePercent float64 `json:"cpu_usage_percent"`
		ClassLoaded     int     `json:"class_loaded"`
		ConnectionPools map[string]struct {
			Active int `json:"active"`
			Idle   int `json:"idle"`
			Max    int `json:"max"`
		} `json:"connection_pools,omitempty"`
	} `json:"app_metrics"`
}

// ConnectionPoolStats 连接池统计信息
type ConnectionPoolStats struct {
	Name   string
	Active int
	Idle   int
	Max    int
}

// MetricsCollector 负责收集和发送应用指标
type MetricsCollector struct {
	// 指标收集选项
	options *MetricsCollectorOptions
	// SDK客户端引用
	client *Client
	// NATS连接
	natsConn *nats.Conn
	// API调用缓冲区
	apiBuffer     []APICall
	apiBufferLock sync.Mutex
	// 上次状态指标收集时间
	lastStatusCollect time.Time
	// 上次API指标发送时间
	lastAPIFlush time.Time
	// 收集和发送控制
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	// 连接池统计信息
	connectionPools     []ConnectionPoolStats
	connectionPoolsLock sync.RWMutex
	// 进程ID和对应的process.Process对象，用于获取CPU使用率
	pid     int32
	process *process.Process
	// 内存统计数据
	memStats runtime.MemStats
	// GC统计
	lastGCCount  uint32
	lastGCTimeNs uint64
	prevGCStats  struct {
		Count  uint32
		TimeNs uint64
		TimeMs int
	}
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector(client *Client, options *MetricsCollectorOptions) *MetricsCollector {
	if options == nil {
		options = DefaultMetricsCollectorOptions()
	}

	// 设置默认值
	ctx, cancel := context.WithCancel(context.Background())

	// 确保APIFlushInterval不为0或负数
	if options.APIFlushInterval <= 0 {
		options.APIFlushInterval = 10 * time.Second
	}

	// 确保StatusCollectInterval不为0或负数
	if options.StatusCollectInterval <= 0 {
		options.StatusCollectInterval = 30 * time.Second
	}

	collector := &MetricsCollector{
		options:         options,
		client:          client,
		apiBuffer:       make([]APICall, 0, options.APIBufferSize),
		lastAPIFlush:    time.Now(),
		ctx:             ctx,
		cancelFunc:      cancel,
		connectionPools: make([]ConnectionPoolStats, 0),
		pid:             int32(os.Getpid()),
	}

	// 初始化进程对象，用于获取CPU使用率
	proc, err := process.NewProcess(collector.pid)
	if err == nil {
		collector.process = proc
	}

	return collector
}

// Start 启动指标收集
func (m *MetricsCollector) Start() error {
	if m.options.DisableMetrics {
		fmt.Println("指标收集已禁用")
		return nil
	}

	// 尝试连接NATS服务
	err := m.connectNATS()
	if err != nil {
		return fmt.Errorf("连接NATS服务失败: %w", err)
	}

	// 如果启用了自动收集应用状态，启动收集协程
	if m.options.AutoCollectStatus {
		m.wg.Add(1)
		go m.collectStatusMetricsLoop()
	}

	// 启动API指标自动发送协程
	m.wg.Add(1)
	go m.autoFlushAPIMetricsLoop()

	fmt.Printf("指标收集器已启动，服务名称: %s, 实例ID: %s\n",
		m.options.ServiceName, m.options.InstanceID)
	return nil
}

// Stop 停止指标收集
func (m *MetricsCollector) Stop() error {
	// 发送取消信号
	m.cancelFunc()

	// 等待所有协程结束
	m.wg.Wait()

	// 发送最后一批API调用指标
	m.flushAPIMetrics()

	// 关闭NATS连接
	if m.natsConn != nil {
		m.natsConn.Flush()
		m.natsConn.Close()
		m.natsConn = nil
	}

	fmt.Println("指标收集器已停止")
	return nil
}

// RegisterConnectionPool 注册连接池以收集其统计信息
func (m *MetricsCollector) RegisterConnectionPool(name string, maxSize int) {
	if m.options.DisableMetrics {
		return
	}

	m.connectionPoolsLock.Lock()
	defer m.connectionPoolsLock.Unlock()

	m.connectionPools = append(m.connectionPools, ConnectionPoolStats{
		Name: name,
		Max:  maxSize,
	})

	fmt.Printf("已注册连接池: %s, 最大连接数: %d\n", name, maxSize)
}

// UpdateConnectionPoolStats 更新连接池的统计信息
func (m *MetricsCollector) UpdateConnectionPoolStats(name string, active, idle int) {
	if m.options.DisableMetrics {
		return
	}

	m.connectionPoolsLock.Lock()
	defer m.connectionPoolsLock.Unlock()

	for i := range m.connectionPools {
		if m.connectionPools[i].Name == name {
			m.connectionPools[i].Active = active
			m.connectionPools[i].Idle = idle
			return
		}
	}

	// 如果连接池不存在，注册一个新的
	m.connectionPools = append(m.connectionPools, ConnectionPoolStats{
		Name:   name,
		Active: active,
		Idle:   idle,
		Max:    active + idle,
	})
}

// RecordAPICall 记录一次API调用
func (m *MetricsCollector) RecordAPICall(call APICall) {
	if m.options.DisableMetrics {
		return
	}

	m.apiBufferLock.Lock()
	defer m.apiBufferLock.Unlock()

	// 添加到缓冲区
	m.apiBuffer = append(m.apiBuffer, call)

	// 如果达到缓冲区大小，立即发送
	if len(m.apiBuffer) >= m.options.APIBufferSize {
		m.flushAPIMetricsLocked()
	}
}

// RecordAPICallSimple 简化版的API调用记录方法
func (m *MetricsCollector) RecordAPICallSimple(
	endpoint, method string,
	duration time.Duration,
	statusCode int,
	hasError bool,
	errorMsg string,
) {
	if m.options.DisableMetrics {
		return
	}

	call := APICall{
		Endpoint:     endpoint,
		Method:       method,
		Timestamp:    time.Now(),
		DurationMs:   float64(duration.Milliseconds()),
		StatusCode:   statusCode,
		HasError:     hasError,
		ErrorMessage: errorMsg,
	}

	m.RecordAPICall(call)
}

// flushAPIMetrics 发送缓冲区中的API调用指标
func (m *MetricsCollector) flushAPIMetrics() {
	m.apiBufferLock.Lock()
	defer m.apiBufferLock.Unlock()

	m.flushAPIMetricsLocked()
}

// flushAPIMetricsLocked 发送缓冲区中的API调用指标（已加锁版本）
func (m *MetricsCollector) flushAPIMetricsLocked() {
	if len(m.apiBuffer) == 0 || m.natsConn == nil {
		return
	}

	// 准备API调用数据包
	apiCalls := APICalls{
		Source:    m.options.ServiceName,
		Instance:  m.options.InstanceID,
		Timestamp: time.Now(),
		Calls:     make([]APICall, len(m.apiBuffer)),
	}

	// 复制缓冲区数据
	copy(apiCalls.Calls, m.apiBuffer)

	// 清空缓冲区
	m.apiBuffer = m.apiBuffer[:0]

	// 更新最后发送时间
	m.lastAPIFlush = time.Now()

	// 序列化数据
	data, err := json.Marshal(apiCalls)
	if err != nil {
		fmt.Printf("序列化API调用数据失败: %v\n", err)
		return
	}

	// 发送到NATS - 使用固定主题"metrics.api"
	subject := "metrics.api"
	err = m.natsConn.Publish(subject, data)
	if err != nil {
		fmt.Printf("发送API调用数据失败: %v\n", err)
		return
	}

	fmt.Printf("已发送 %d 条API调用数据到NATS主题 %s\n", len(apiCalls.Calls), subject)
}

// CollectStatusMetrics 收集和发送一次应用状态指标
func (m *MetricsCollector) CollectStatusMetrics() error {
	if m.options.DisableMetrics || m.natsConn == nil {
		return nil
	}

	// 记录本次收集时间
	m.lastStatusCollect = time.Now()

	// 准备应用状态指标数据包
	metrics := AppMetrics{
		Source:    m.options.ServiceName,
		Instance:  m.options.InstanceID,
		Version:   m.options.Version,
		Tags:      m.options.Tags,
		Timestamp: time.Now(),
	}

	// 收集内存指标
	m.collectMemoryMetrics(&metrics)

	// 收集线程指标
	m.collectThreadMetrics(&metrics)

	// 收集CPU使用率
	m.collectCPUMetrics(&metrics)

	// 收集连接池指标
	m.collectConnectionPoolMetrics(&metrics)

	// 序列化数据
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("序列化应用状态数据失败: %w", err)
	}

	// 发送到NATS - 使用固定主题"metrics.app"
	subject := "metrics.app"
	err = m.natsConn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("发送应用状态数据失败: %w", err)
	}

	fmt.Printf("已发送应用状态指标数据到NATS主题 %s\n", subject)
	return nil
}

// collectMemoryMetrics 收集内存相关指标
func (m *MetricsCollector) collectMemoryMetrics(metrics *AppMetrics) {
	// 获取系统内存信息
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		// 转换为MB
		metrics.Metrics.Memory.TotalMB = float64(memInfo.Total) / 1024 / 1024
	}

	// 获取Go运行时内存统计
	runtime.ReadMemStats(&m.memStats)

	// 计算已用内存（MB）
	metrics.Metrics.Memory.UsedMB = float64(m.memStats.Alloc) / 1024 / 1024

	// 计算堆内存（MB）
	metrics.Metrics.Memory.HeapMB = float64(m.memStats.HeapAlloc) / 1024 / 1024

	// 计算非堆内存（MB）
	metrics.Metrics.Memory.NonHeapMB = float64(m.memStats.Alloc-m.memStats.HeapAlloc) / 1024 / 1024

	// 获取GC次数
	gcCount := m.memStats.NumGC

	// 计算本次收集周期内的GC次数增量
	if m.lastGCCount > 0 {
		if gcCount >= m.lastGCCount {
			m.prevGCStats.Count = gcCount - m.lastGCCount
		} else {
			// 处理计数器溢出的情况
			m.prevGCStats.Count = gcCount
		}
	}
	m.lastGCCount = gcCount

	// 获取GC累计耗时
	gcTimeNs := m.memStats.PauseTotalNs

	// 计算本次收集周期内的GC耗时增量（毫秒）
	if m.lastGCTimeNs > 0 {
		if gcTimeNs >= m.lastGCTimeNs {
			m.prevGCStats.TimeNs = gcTimeNs - m.lastGCTimeNs
		} else {
			// 处理计数器溢出的情况
			m.prevGCStats.TimeNs = gcTimeNs
		}
		m.prevGCStats.TimeMs = int(m.prevGCStats.TimeNs / 1000000)
	}
	m.lastGCTimeNs = gcTimeNs

	// 填充GC指标
	metrics.Metrics.Memory.GCCount = int(m.prevGCStats.Count)
	metrics.Metrics.Memory.GCTimeMs = m.prevGCStats.TimeMs
}

// collectThreadMetrics 收集线程相关指标
func (m *MetricsCollector) collectThreadMetrics(metrics *AppMetrics) {
	// Go没有直接的线程API，这里模拟一些基本数据
	// 在实际应用中，可能需要根据应用特性自定义这部分
	metrics.Metrics.Threads.Total = runtime.NumGoroutine()
	metrics.Metrics.Threads.Active = runtime.NumGoroutine()

	// 仅作为示例，实际应用中可能需要更精确的方法
	metrics.Metrics.Threads.Blocked = 0
	metrics.Metrics.Threads.Waiting = 0
}

// collectCPUMetrics 收集CPU使用率指标
func (m *MetricsCollector) collectCPUMetrics(metrics *AppMetrics) {
	if m.process != nil {
		// 如果可以获取进程信息，尝试获取进程CPU使用率
		cpuPercent, err := m.process.CPUPercent()
		if err == nil {
			metrics.Metrics.CPUUsagePercent = cpuPercent
			return
		}
	}

	// 作为备选，获取系统CPU使用率
	cpuPercent, err := cpu.Percent(100*time.Millisecond, false)
	if err == nil && len(cpuPercent) > 0 {
		metrics.Metrics.CPUUsagePercent = cpuPercent[0]
		return
	}

	// 如果都获取失败，设为0
	metrics.Metrics.CPUUsagePercent = 0
}

// collectConnectionPoolMetrics 收集连接池指标
func (m *MetricsCollector) collectConnectionPoolMetrics(metrics *AppMetrics) {
	m.connectionPoolsLock.RLock()
	defer m.connectionPoolsLock.RUnlock()

	if len(m.connectionPools) == 0 {
		return
	}

	// 初始化连接池指标映射
	metrics.Metrics.ConnectionPools = make(map[string]struct {
		Active int `json:"active"`
		Idle   int `json:"idle"`
		Max    int `json:"max"`
	})

	// 填充连接池指标
	for _, pool := range m.connectionPools {
		metrics.Metrics.ConnectionPools[pool.Name] = struct {
			Active int `json:"active"`
			Idle   int `json:"idle"`
			Max    int `json:"max"`
		}{
			Active: pool.Active,
			Idle:   pool.Idle,
			Max:    pool.Max,
		}
	}
}

// connectNATS 连接到NATS服务器
func (m *MetricsCollector) connectNATS() error {
	// 获取NATS客户端
	var err error
	m.natsConn, err = m.client.GetNatsClient(m.ctx)
	if err != nil {
		return fmt.Errorf("获取NATS客户端失败: %w", err)
	}

	fmt.Println("已成功连接到NATS服务器用于指标收集")
	return nil
}

// collectStatusMetricsLoop 定期收集和发送应用状态指标
func (m *MetricsCollector) collectStatusMetricsLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.options.StatusCollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.CollectStatusMetrics(); err != nil {
				fmt.Printf("收集应用状态指标失败: %v\n", err)
			}
		}
	}
}

// autoFlushAPIMetricsLoop 定期发送API调用指标
func (m *MetricsCollector) autoFlushAPIMetricsLoop() {
	defer m.wg.Done()

	// 再次检查间隔是否有效
	interval := m.options.APIFlushInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.flushAPIMetrics()
		}
	}
}
