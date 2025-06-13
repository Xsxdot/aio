// Package collector 实现数据采集功能
package collector

import (
	"context"
	"os"
	"sync"
	"time"

	models2 "github.com/xsxdot/aio/internal/monitoring/models"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"go.uber.org/zap"
)

// Config 定义数据采集器的配置
type Config struct {
	// CollectInterval 指定服务器指标采集的间隔时间（秒）
	CollectInterval int

	// Logger 日志记录器
	Logger *zap.Logger
}

// MetricHandler 是处理采集到的指标的接口
type MetricHandler interface {
	// HandleServerMetrics 处理服务器指标
	HandleServerMetrics(metrics *models2.ServerMetrics) error

	// HandleAPICalls 处理API调用信息
	HandleAPICalls(calls *models2.APICalls) error

	// HandleAppMetrics 处理应用状态指标
	HandleAppMetrics(metrics *models2.AppMetrics) error

	// HandleServiceData 处理应用服务数据
	HandleServiceData(data *models2.ServiceData) error

	// HandleServiceAPIData 处理应用服务API调用数据
	HandleServiceAPIData(data *models2.ServiceAPIData) error
}

// Collector 负责收集和处理指标数据
type Collector struct {
	config     Config
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	handlers   []MetricHandler
	logger     *zap.Logger
}

// New 创建一个新的数据采集器
func New(config Config) *Collector {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认logger，如果没有提供
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
		defer logger.Sync()
	}

	c := &Collector{
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
		handlers:   make([]MetricHandler, 0),
		logger:     logger,
	}

	return c
}

// Start 启动数据采集
func (c *Collector) Start() error {
	c.logger.Info("启动数据采集器")

	// 启动服务器指标采集
	c.wg.Add(1)
	go c.collectServerMetrics()

	c.logger.Info("数据采集器启动成功")
	return nil
}

// Stop 停止数据采集
func (c *Collector) Stop() error {
	c.logger.Info("停止数据采集器")
	c.cancelFunc()
	c.wg.Wait()

	c.logger.Info("数据采集器已停止")
	return nil
}

// RegisterHandler 注册一个指标处理器
func (c *Collector) RegisterHandler(handler MetricHandler) {
	c.handlers = append(c.handlers, handler)
	c.logger.Debug("注册指标处理器",
		zap.Int("handlersCount", len(c.handlers)))
}

// collectServerMetrics 定期采集服务器指标
func (c *Collector) collectServerMetrics() {
	defer c.wg.Done()
	c.logger.Info("启动服务器指标采集",
		zap.Int("间隔(秒)", c.config.CollectInterval))

	ticker := time.NewTicker(time.Duration(c.config.CollectInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("服务器指标采集停止")
			return
		case <-ticker.C:
			metrics, err := c.gatherServerMetrics()
			if err != nil {
				c.logger.Error("采集服务器指标失败", zap.Error(err))
				continue
			}

			c.logger.Debug("采集到服务器指标",
				zap.String("hostname", metrics.Hostname),
				zap.Int("metricsCount", len(metrics.Metrics)))

			// 将采集到的指标发送给所有处理器
			for _, handler := range c.handlers {
				if err := handler.HandleServerMetrics(metrics); err != nil {
					c.logger.Error("处理服务器指标失败", zap.Error(err))
				}
			}
		}
	}
}

// gatherServerMetrics 采集服务器指标
func (c *Collector) gatherServerMetrics() (*models2.ServerMetrics, error) {
	hostname, _ := os.Hostname()

	metrics := &models2.ServerMetrics{
		Timestamp: time.Now(),
		Hostname:  hostname,
		Metrics:   make(map[models2.ServerMetricName]float64),
	}

	// 采集CPU指标
	if err := c.collectCPUMetrics(metrics); err != nil {
		c.logger.Warn("采集CPU指标部分失败", zap.Error(err))
		// 即使部分采集失败也继续进行
	}

	// 采集内存指标
	if err := c.collectMemoryMetrics(metrics); err != nil {
		c.logger.Warn("采集内存指标部分失败", zap.Error(err))
		// 即使部分采集失败也继续进行
	}

	// 采集磁盘指标
	if err := c.collectDiskMetrics(metrics); err != nil {
		c.logger.Warn("采集磁盘指标部分失败", zap.Error(err))
		// 即使部分采集失败也继续进行
	}

	// 采集网络指标
	if err := c.collectNetworkMetrics(metrics); err != nil {
		c.logger.Warn("采集网络指标部分失败", zap.Error(err))
		// 即使部分采集失败也继续进行
	}

	return metrics, nil
}

// collectCPUMetrics 采集CPU相关指标
func (c *Collector) collectCPUMetrics(metrics *models2.ServerMetrics) error {
	// 获取CPU使用率
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		metrics.Metrics[models2.MetricCPUUsage] = cpuPercents[0]
	} else if err != nil {
		c.logger.Error("获取CPU使用率失败", zap.Error(err))
		return err
	}

	// 获取更详细的CPU使用率
	cpuTimes, err := cpu.Times(false)
	if err == nil && len(cpuTimes) > 0 {
		total := cpuTimes[0].User + cpuTimes[0].System + cpuTimes[0].Idle + cpuTimes[0].Nice +
			cpuTimes[0].Iowait + cpuTimes[0].Irq + cpuTimes[0].Softirq + cpuTimes[0].Steal + cpuTimes[0].Guest + cpuTimes[0].GuestNice

		if total > 0 {
			metrics.Metrics[models2.MetricCPUUsageUser] = (cpuTimes[0].User / total) * 100
			metrics.Metrics[models2.MetricCPUUsageSystem] = (cpuTimes[0].System / total) * 100
			metrics.Metrics[models2.MetricCPUUsageIdle] = (cpuTimes[0].Idle / total) * 100
			metrics.Metrics[models2.MetricCPUUsageIOWait] = (cpuTimes[0].Iowait / total) * 100
		}
	} else if err != nil {
		c.logger.Error("获取CPU时间详情失败", zap.Error(err))
		return err
	}

	// 获取系统负载
	loadInfo, err := load.Avg()
	if err == nil {
		metrics.Metrics[models2.MetricCPULoad1] = loadInfo.Load1
		metrics.Metrics[models2.MetricCPULoad5] = loadInfo.Load5
		metrics.Metrics[models2.MetricCPULoad15] = loadInfo.Load15
	} else {
		c.logger.Error("获取系统负载失败", zap.Error(err))
		return err
	}

	c.logger.Debug("采集CPU指标成功",
		zap.Float64("cpuUsage", metrics.Metrics[models2.MetricCPUUsage]),
		zap.Float64("load1", metrics.Metrics[models2.MetricCPULoad1]))
	return nil
}

// collectMemoryMetrics 采集内存相关指标
func (c *Collector) collectMemoryMetrics(metrics *models2.ServerMetrics) error {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		c.logger.Error("获取内存信息失败", zap.Error(err))
		return err
	}

	metrics.Metrics[models2.MetricMemoryTotal] = float64(memInfo.Total) / 1024 / 1024     // MB
	metrics.Metrics[models2.MetricMemoryUsed] = float64(memInfo.Used) / 1024 / 1024       // MB
	metrics.Metrics[models2.MetricMemoryFree] = float64(memInfo.Free) / 1024 / 1024       // MB
	metrics.Metrics[models2.MetricMemoryBuffers] = float64(memInfo.Buffers) / 1024 / 1024 // MB
	metrics.Metrics[models2.MetricMemoryCache] = float64(memInfo.Cached) / 1024 / 1024    // MB
	metrics.Metrics[models2.MetricMemoryUsedPercent] = memInfo.UsedPercent

	c.logger.Debug("采集内存指标成功",
		zap.Float64("memoryTotal(MB)", metrics.Metrics[models2.MetricMemoryTotal]),
		zap.Float64("memoryUsed(MB)", metrics.Metrics[models2.MetricMemoryUsed]),
		zap.Float64("memoryUsedPercent", metrics.Metrics[models2.MetricMemoryUsedPercent]))
	return nil
}

// collectDiskMetrics 采集磁盘相关指标
func (c *Collector) collectDiskMetrics(metrics *models2.ServerMetrics) error {
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
	metrics.Metrics[models2.MetricDiskTotal] = float64(totalSize) / 1024 / 1024 / 1024 // GB
	metrics.Metrics[models2.MetricDiskUsed] = float64(totalUsed) / 1024 / 1024 / 1024  // GB
	metrics.Metrics[models2.MetricDiskFree] = float64(totalFree) / 1024 / 1024 / 1024  // GB
	metrics.Metrics[models2.MetricDiskUsedPercent] = maxUsedPercent

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

		metrics.Metrics[models2.MetricDiskIORead] = float64(readCount)
		metrics.Metrics[models2.MetricDiskIOWrite] = float64(writeCount)
		metrics.Metrics[models2.MetricDiskIOReadBytes] = float64(readBytes) / 1024 / 1024   // MB
		metrics.Metrics[models2.MetricDiskIOWriteBytes] = float64(writeBytes) / 1024 / 1024 // MB
	}

	c.logger.Debug("采集磁盘指标成功",
		zap.Float64("diskTotal(GB)", metrics.Metrics[models2.MetricDiskTotal]),
		zap.Float64("diskUsed(GB)", metrics.Metrics[models2.MetricDiskUsed]),
		zap.Float64("diskUsedPercent", metrics.Metrics[models2.MetricDiskUsedPercent]))
	return nil
}

// collectNetworkMetrics 采集网络相关指标
func (c *Collector) collectNetworkMetrics(metrics *models2.ServerMetrics) error {
	// 初始化网络指标，确保即使获取失败也有默认值
	metrics.Metrics[models2.MetricNetworkIn] = 0.0
	metrics.Metrics[models2.MetricNetworkOut] = 0.0
	metrics.Metrics[models2.MetricNetworkInPackets] = 0.0
	metrics.Metrics[models2.MetricNetworkOutPackets] = 0.0

	// 获取网络IO统计
	ioStats, err := net.IOCounters(false)
	if err != nil {
		c.logger.Error("获取网络IO统计失败", zap.Error(err))
		// 返回错误但已设置默认值，调用方可以继续处理其他指标
		return err
	}

	// 安全检查：确保ioStats数组不为空
	if len(ioStats) == 0 {
		c.logger.Warn("未检测到网络接口")
		return nil
	}

	// 记录总体网络流量
	metrics.Metrics[models2.MetricNetworkIn] = float64(ioStats[0].BytesRecv) / 1024 / 1024  // MB
	metrics.Metrics[models2.MetricNetworkOut] = float64(ioStats[0].BytesSent) / 1024 / 1024 // MB
	metrics.Metrics[models2.MetricNetworkInPackets] = float64(ioStats[0].PacketsRecv)
	metrics.Metrics[models2.MetricNetworkOutPackets] = float64(ioStats[0].PacketsSent)

	c.logger.Debug("采集网络指标成功",
		zap.Float64("networkIn(MB)", metrics.Metrics[models2.MetricNetworkIn]),
		zap.Float64("networkOut(MB)", metrics.Metrics[models2.MetricNetworkOut]))
	return nil
}
