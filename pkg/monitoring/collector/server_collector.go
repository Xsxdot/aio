// Package collector 实现服务器指标采集功能
package collector

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"github.com/xsxdot/aio/pkg/scheduler"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"go.uber.org/zap"
)

// ServerCollectorConfig 服务器收集器配置
type ServerCollectorConfig struct {
	CollectInterval int                          // 采集间隔（秒）
	Logger          *zap.Logger                  // 日志记录器
	Storage         storage.UnifiedMetricStorage // 存储层
	Scheduler       *scheduler.Scheduler         // 调度器
}

// ServerCollector 服务器指标收集器
type ServerCollector struct {
	config  ServerCollectorConfig
	logger  *zap.Logger
	storage storage.UnifiedMetricStorage
	task    scheduler.Task

	// 缓存相关字段
	lastCPUTime    time.Time
	lastCPUPercent float64
}

// NewServerCollector 创建新的服务器收集器
func NewServerCollector(config ServerCollectorConfig) *ServerCollector {
	// 设置默认logger，如果没有提供
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	collector := &ServerCollector{
		config:  config,
		logger:  logger,
		storage: config.Storage,
	}

	// 创建固定间隔任务
	hostname, _ := os.Hostname()
	taskName := fmt.Sprintf("server-collector-%s", hostname)
	interval := time.Duration(config.CollectInterval) * time.Second

	collector.task = scheduler.NewIntervalTask(
		taskName,
		time.Now(),
		interval,
		scheduler.TaskExecuteModeLocal, // 服务器指标采集是本地任务
		30*time.Second,                 // 任务超时时间
		collector.collectAndStore,
	)

	return collector
}

// Start 启动服务器指标采集
func (c *ServerCollector) Start() error {
	c.logger.Info("启动服务器指标采集器",
		zap.Int("interval_seconds", c.config.CollectInterval))

	// 将任务添加到调度器
	if c.config.Scheduler != nil {
		return c.config.Scheduler.AddTask(c.task)
	}

	return fmt.Errorf("scheduler not provided")
}

// Stop 停止服务器指标采集
func (c *ServerCollector) Stop() error {
	c.logger.Info("停止服务器指标采集器")

	// 从调度器移除任务
	if c.config.Scheduler != nil {
		c.config.Scheduler.RemoveTask(c.task.GetID())
	}

	c.logger.Info("服务器指标采集器已停止")
	return nil
}

// collectAndStore 采集并存储指标（作为任务函数）
func (c *ServerCollector) collectAndStore(ctx context.Context) error {
	metrics, err := c.collectServerMetrics()
	if err != nil {
		c.logger.Error("采集服务器指标失败", zap.Error(err))
		return err
	}

	// 使用统一存储方法
	if err := c.storage.StoreMetricProvider(metrics); err != nil {
		c.logger.Error("存储服务器指标失败", zap.Error(err))
		return err
	}

	c.logger.Debug("服务器指标采集存储成功",
		zap.String("hostname", metrics.Hostname),
		zap.Int("metrics_count", len(metrics.Metrics)))

	return nil
}

// collectServerMetrics 采集服务器指标
func (c *ServerCollector) collectServerMetrics() (*ServerMetrics, error) {
	hostname, _ := os.Hostname()

	metrics := &ServerMetrics{
		Timestamp: time.Now(),
		Hostname:  hostname,
		Metrics:   make(map[ServerMetricName]float64),
	}

	// 采集CPU指标
	if err := c.collectCPUMetrics(metrics); err != nil {
		c.logger.Warn("采集CPU指标部分失败", zap.Error(err))
	}

	// 采集内存指标
	if err := c.collectMemoryMetrics(metrics); err != nil {
		c.logger.Warn("采集内存指标部分失败", zap.Error(err))
	}

	// 采集磁盘指标
	if err := c.collectDiskMetrics(metrics); err != nil {
		c.logger.Warn("采集磁盘指标部分失败", zap.Error(err))
	}

	// 采集网络指标
	if err := c.collectNetworkMetrics(metrics); err != nil {
		c.logger.Warn("采集网络指标部分失败", zap.Error(err))
	}

	return metrics, nil
}

// collectCPUMetrics 采集CPU指标
func (c *ServerCollector) collectCPUMetrics(metrics *ServerMetrics) error {
	// CPU使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		metrics.Metrics[MetricCPUUsage] = cpuPercent[0]
	}

	// 获取更详细的CPU使用率
	cpuTimes, err := cpu.Times(false)
	if err == nil && len(cpuTimes) > 0 {
		total := cpuTimes[0].User + cpuTimes[0].System + cpuTimes[0].Idle + cpuTimes[0].Nice +
			cpuTimes[0].Iowait + cpuTimes[0].Irq + cpuTimes[0].Softirq + cpuTimes[0].Steal + cpuTimes[0].Guest + cpuTimes[0].GuestNice

		if total > 0 {
			metrics.Metrics[MetricCPUUsageUser] = (cpuTimes[0].User / total) * 100
			metrics.Metrics[MetricCPUUsageSystem] = (cpuTimes[0].System / total) * 100
			metrics.Metrics[MetricCPUUsageIdle] = (cpuTimes[0].Idle / total) * 100
			metrics.Metrics[MetricCPUUsageIOWait] = (cpuTimes[0].Iowait / total) * 100
		}
	} else if err != nil {
		c.logger.Error("获取CPU时间详情失败", zap.Error(err))
		return err
	}
	// 系统负载
	loadAvg, err := load.Avg()
	if err == nil {
		metrics.Metrics[MetricCPULoad1] = loadAvg.Load1
		metrics.Metrics[MetricCPULoad5] = loadAvg.Load5
		metrics.Metrics[MetricCPULoad15] = loadAvg.Load15
	}

	return nil
}

// collectMemoryMetrics 采集内存指标
func (c *ServerCollector) collectMemoryMetrics(metrics *ServerMetrics) error {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return err
	}

	metrics.Metrics[MetricMemoryTotal] = float64(memInfo.Total) / 1024 / 1024     // MB
	metrics.Metrics[MetricMemoryUsed] = float64(memInfo.Used) / 1024 / 1024       // MB
	metrics.Metrics[MetricMemoryFree] = float64(memInfo.Free) / 1024 / 1024       // MB
	metrics.Metrics[MetricMemoryBuffers] = float64(memInfo.Buffers) / 1024 / 1024 // MB
	metrics.Metrics[MetricMemoryCache] = float64(memInfo.Cached) / 1024 / 1024    // MB
	metrics.Metrics[MetricMemoryUsedPercent] = memInfo.UsedPercent

	return nil
}

// collectDiskMetrics 采集磁盘指标
func (c *ServerCollector) collectDiskMetrics(metrics *ServerMetrics) error {
	// 获取磁盘使用情况
	parts, err := disk.Partitions(false)
	if err != nil {
		c.logger.Error("获取磁盘分区信息失败", zap.Error(err))
		return err
	}

	var totalSize, totalUsed, totalFree uint64
	var maxUsedPercent float64

	for _, part := range parts {
		usage, err := disk.Usage(part.Mountpoint)
		if err != nil {
			c.logger.Warn("获取分区使用情况失败",
				zap.String("mountpoint", part.Mountpoint),
				zap.Error(err))
			continue
		}

		totalSize += usage.Total
		totalUsed += usage.Used
		totalFree += usage.Free
		if usage.UsedPercent > maxUsedPercent {
			maxUsedPercent = usage.UsedPercent
		}
	}

	// 记录磁盘总体使用情况
	metrics.Metrics[MetricDiskTotal] = float64(totalSize) / 1024 / 1024 / 1024 // GB
	metrics.Metrics[MetricDiskUsed] = float64(totalUsed) / 1024 / 1024 / 1024  // GB
	metrics.Metrics[MetricDiskFree] = float64(totalFree) / 1024 / 1024 / 1024  // GB
	metrics.Metrics[MetricDiskUsedPercent] = maxUsedPercent

	// 获取磁盘IO统计
	ioStats, err := disk.IOCounters()
	if err != nil {
		c.logger.Warn("获取磁盘IO统计失败", zap.Error(err))
		// 继续执行，不返回错误
	} else {
		var readCount, writeCount, readBytes, writeBytes uint64
		for _, stat := range ioStats {
			readCount += stat.ReadCount
			writeCount += stat.WriteCount
			readBytes += stat.ReadBytes
			writeBytes += stat.WriteBytes
		}

		metrics.Metrics[MetricDiskIORead] = float64(readCount)
		metrics.Metrics[MetricDiskIOWrite] = float64(writeCount)
		metrics.Metrics[MetricDiskIOReadBytes] = float64(readBytes) / 1024 / 1024   // MB
		metrics.Metrics[MetricDiskIOWriteBytes] = float64(writeBytes) / 1024 / 1024 // MB
	}

	return nil
}

// collectNetworkMetrics 采集网络指标
func (c *ServerCollector) collectNetworkMetrics(metrics *ServerMetrics) error {
	// 初始化网络指标，确保即使获取失败也有默认值
	metrics.Metrics[MetricNetworkIn] = 0.0
	metrics.Metrics[MetricNetworkOut] = 0.0
	metrics.Metrics[MetricNetworkInPackets] = 0.0
	metrics.Metrics[MetricNetworkOutPackets] = 0.0

	netIO, err := net.IOCounters(false)
	if err != nil || len(netIO) == 0 {
		return err
	}

	// 汇总所有网络接口的统计
	var totalBytesSent, totalBytesRecv uint64
	var totalPacketsSent, totalPacketsRecv uint64

	for _, stat := range netIO {
		totalBytesSent += stat.BytesSent
		totalBytesRecv += stat.BytesRecv
		totalPacketsSent += stat.PacketsSent
		totalPacketsRecv += stat.PacketsRecv
	}

	metrics.Metrics[MetricNetworkOut] = float64(totalBytesSent)
	metrics.Metrics[MetricNetworkIn] = float64(totalBytesRecv)
	metrics.Metrics[MetricNetworkOutPackets] = float64(totalPacketsSent)
	metrics.Metrics[MetricNetworkInPackets] = float64(totalPacketsRecv)

	return nil
}

// ServerMetrics 表示服务器指标集合
type ServerMetrics struct {
	Timestamp time.Time                    `json:"timestamp"`
	Hostname  string                       `json:"hostname"`
	Metrics   map[ServerMetricName]float64 `json:"metrics"`
}

// GetMetricNames 实现 MetricProvider 接口
func (s *ServerMetrics) GetMetricNames() []string {
	return []string{
		string(MetricCPUUsage),
		string(MetricCPUUsageUser),
		string(MetricCPUUsageSystem),
		string(MetricCPUUsageIdle),
		string(MetricCPUUsageIOWait),
		string(MetricCPULoad1),
		string(MetricCPULoad5),
		string(MetricCPULoad15),
		string(MetricMemoryTotal),
		string(MetricMemoryUsed),
		string(MetricMemoryFree),
		string(MetricMemoryBuffers),
		string(MetricMemoryCache),
		string(MetricMemoryUsedPercent),
		string(MetricDiskTotal),
		string(MetricDiskUsed),
		string(MetricDiskFree),
		string(MetricDiskUsedPercent),
		string(MetricDiskIORead),
		string(MetricDiskIOWrite),
		string(MetricDiskIOReadBytes),
		string(MetricDiskIOWriteBytes),
		string(MetricNetworkIn),
		string(MetricNetworkOut),
		string(MetricNetworkInPackets),
		string(MetricNetworkOutPackets),
	}
}

// GetCategory 实现 MetricProvider 接口
func (s *ServerMetrics) GetCategory() models.MetricCategory {
	return models.CategoryServer
}

// ToMetricPoints 实现 MetricProvider 接口
func (s *ServerMetrics) ToMetricPoints() []models.MetricPoint {
	points := make([]models.MetricPoint, 0, len(s.Metrics))
	for name, value := range s.Metrics {
		points = append(points, models.MetricPoint{
			Timestamp:  s.Timestamp,
			MetricName: string(name),
			MetricType: models.MetricTypeGauge,
			Value:      value,
			Source:     s.Hostname,
			Instance:   "",
			Category:   models.CategoryServer,
			Labels:     map[string]string{"hostname": s.Hostname},
		})
	}
	return points
}

// ServerMetricName 定义服务器指标的名称常量
type ServerMetricName string

const (
	// CPU相关指标
	MetricCPUUsage       ServerMetricName = "cpu.usage"
	MetricCPUUsageUser   ServerMetricName = "cpu.usage.user"
	MetricCPUUsageSystem ServerMetricName = "cpu.usage.system"
	MetricCPUUsageIdle   ServerMetricName = "cpu.usage.idle"
	MetricCPUUsageIOWait ServerMetricName = "cpu.usage.iowait"
	MetricCPULoad1       ServerMetricName = "cpu.load1"
	MetricCPULoad5       ServerMetricName = "cpu.load5"
	MetricCPULoad15      ServerMetricName = "cpu.load15"

	// 内存相关指标
	MetricMemoryTotal       ServerMetricName = "memory.total"
	MetricMemoryUsed        ServerMetricName = "memory.used"
	MetricMemoryFree        ServerMetricName = "memory.free"
	MetricMemoryBuffers     ServerMetricName = "memory.buffers"
	MetricMemoryCache       ServerMetricName = "memory.cache"
	MetricMemoryUsedPercent ServerMetricName = "memory.used_percent"

	// 磁盘相关指标
	MetricDiskTotal        ServerMetricName = "disk.total"
	MetricDiskUsed         ServerMetricName = "disk.used"
	MetricDiskFree         ServerMetricName = "disk.free"
	MetricDiskUsedPercent  ServerMetricName = "disk.used_percent"
	MetricDiskIORead       ServerMetricName = "disk.io.read"
	MetricDiskIOWrite      ServerMetricName = "disk.io.write"
	MetricDiskIOReadBytes  ServerMetricName = "disk.io.read_bytes"
	MetricDiskIOWriteBytes ServerMetricName = "disk.io.write_bytes"

	// 网络相关指标
	MetricNetworkIn         ServerMetricName = "network.in"
	MetricNetworkOut        ServerMetricName = "network.out"
	MetricNetworkInPackets  ServerMetricName = "network.in_packets"
	MetricNetworkOutPackets ServerMetricName = "network.out_packets"
)
