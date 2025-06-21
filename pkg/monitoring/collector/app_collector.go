// Package collector 实现应用指标采集功能
package collector

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"github.com/xsxdot/aio/pkg/scheduler"

	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"
)

// AppCollectorConfig 应用收集器配置
type AppCollectorConfig struct {
	ServiceName     string                       // 服务名称
	InstanceID      string                       // 实例ID
	CollectInterval int                          // 采集间隔（秒）
	Logger          *zap.Logger                  // 日志记录器
	Storage         storage.UnifiedMetricStorage // 存储层
	Scheduler       *scheduler.Scheduler         // 调度器
}

// AppCollector 应用指标收集器
type AppCollector struct {
	config  AppCollectorConfig
	logger  *zap.Logger
	storage storage.UnifiedMetricStorage
	task    scheduler.Task

	// 缓存的进程信息
	processInfo *process.Process
	lastCPUTime time.Time
}

// NewAppCollector 创建新的应用收集器
func NewAppCollector(config AppCollectorConfig) *AppCollector {
	// 设置默认logger，如果没有提供
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	collector := &AppCollector{
		config:  config,
		logger:  logger,
		storage: config.Storage,
	}

	// 创建固定间隔任务
	taskName := fmt.Sprintf("app-collector-%s-%s", config.ServiceName, config.InstanceID)
	interval := time.Duration(config.CollectInterval) * time.Second

	collector.task = scheduler.NewIntervalTask(
		taskName,
		time.Now(),
		interval,
		scheduler.TaskExecuteModeLocal, // 应用指标采集是本地任务
		30*time.Second,                 // 任务超时时间
		collector.collectAndStore,
	)

	return collector
}

// SetServiceInstanceID 设置服务实例ID
func (c *AppCollector) SetServiceInstanceID(instanceID string) {
	c.config.InstanceID = instanceID
	c.logger.Info("应用指标收集器已更新实例ID",
		zap.String("instance_id", instanceID))
}

// Start 启动应用指标采集
func (c *AppCollector) Start() error {
	c.logger.Info("启动应用指标采集器",
		zap.String("service_name", c.config.ServiceName),
		zap.String("instance_id", c.config.InstanceID),
		zap.Int("interval_seconds", c.config.CollectInterval))

	// 将任务添加到调度器
	if c.config.Scheduler != nil {
		return c.config.Scheduler.AddTask(c.task)
	}

	return fmt.Errorf("scheduler not provided")
}

// Stop 停止应用指标采集
func (c *AppCollector) Stop() error {
	c.logger.Info("停止应用指标采集器")

	// 从调度器移除任务
	if c.config.Scheduler != nil {
		c.config.Scheduler.RemoveTask(c.task.GetID())
	}

	c.logger.Info("应用指标采集器已停止")
	return nil
}

// collectAndStore 采集并存储指标（作为任务函数）
func (c *AppCollector) collectAndStore(ctx context.Context) error {
	metrics, err := c.collectAppMetrics()
	if err != nil {
		c.logger.Error("采集应用指标失败", zap.Error(err))
		return err
	}

	// 使用统一存储方法
	if err := c.storage.StoreMetricProvider(metrics); err != nil {
		c.logger.Error("存储应用指标失败", zap.Error(err))
		return err
	}

	c.logger.Debug("应用指标采集存储成功",
		zap.String("service_name", c.config.ServiceName),
		zap.String("instance_id", c.config.InstanceID))

	return nil
}

// collectAppMetrics 采集应用指标
func (c *AppCollector) collectAppMetrics() (*AppMetrics, error) {
	now := time.Now()

	appMetrics := &AppMetrics{
		Source:    c.config.ServiceName,
		Instance:  c.config.InstanceID,
		Timestamp: now,
	}

	// 采集运行时内存指标
	c.collectRuntimeMemoryMetrics(appMetrics)

	// 采集运行时线程指标
	c.collectRuntimeThreadMetrics(appMetrics)

	// 采集CPU使用率
	cpuUsage, err := c.collectCPUUsage()
	if err == nil {
		appMetrics.Metrics.CPUUsagePercent = cpuUsage
	}

	// 采集类加载数量（包数量）
	appMetrics.Metrics.ClassLoaded = c.getLoadedPackagesCount()

	return appMetrics, nil
}

// collectRuntimeMemoryMetrics 采集运行时内存指标
func (c *AppCollector) collectRuntimeMemoryMetrics(appMetrics *AppMetrics) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// 转换为MB
	appMetrics.Metrics.Memory.TotalMB = float64(mem.Sys) / 1024 / 1024
	appMetrics.Metrics.Memory.UsedMB = float64(mem.Alloc) / 1024 / 1024
	appMetrics.Metrics.Memory.HeapMB = float64(mem.HeapAlloc) / 1024 / 1024
	appMetrics.Metrics.Memory.NonHeapMB = float64(mem.StackSys+mem.MSpanSys+mem.MCacheSys+mem.OtherSys) / 1024 / 1024

	// GC统计
	appMetrics.Metrics.Memory.GCCount = int(mem.NumGC)
	appMetrics.Metrics.Memory.GCTimeMs = int(mem.PauseTotalNs / 1000000)
}

// collectRuntimeThreadMetrics 采集运行时线程指标
func (c *AppCollector) collectRuntimeThreadMetrics(appMetrics *AppMetrics) {
	// Go运行时的goroutine数量
	numGoroutines := runtime.NumGoroutine()
	appMetrics.Metrics.Threads.Total = numGoroutines
	appMetrics.Metrics.Threads.Active = numGoroutines // Go中所有goroutine都可以认为是active的
	appMetrics.Metrics.Threads.Blocked = 0            // Go中没有明确的blocked状态
	appMetrics.Metrics.Threads.Waiting = 0            // Go中没有明确的waiting状态
}

// collectCPUUsage 采集CPU使用率
func (c *AppCollector) collectCPUUsage() (float64, error) {
	// 确保进程信息可用
	if err := c.ensureProcessInfo(); err != nil {
		return 0, err
	}

	// 获取CPU百分比
	cpuPercent, err := c.processInfo.CPUPercent()
	if err != nil {
		return 0, err
	}

	return cpuPercent, nil
}

// ensureProcessInfo 确保进程信息可用
func (c *AppCollector) ensureProcessInfo() error {
	if c.processInfo == nil {
		proc, err := process.NewProcess(int32(os.Getpid()))
		if err != nil {
			return err
		}
		c.processInfo = proc
	}
	return nil
}

// getLoadedPackagesCount 获取加载的包数量
func (c *AppCollector) getLoadedPackagesCount() int {
	// 在Go中，我们可以统计运行时信息作为类似的指标
	// 这里返回一个基于内存分配器的估算值
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// 使用堆对象数量作为加载类的近似值
	return int(mem.Mallocs - mem.Frees)
}

// AppMetrics 表示应用状态指标
type AppMetrics struct {
	Source    string    `json:"source"`
	Instance  string    `json:"instance"`
	Timestamp time.Time `json:"timestamp"`
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

// GetMetricNames 实现 MetricProvider 接口
func (a *AppMetrics) GetMetricNames() []string {
	return []string{
		string(MetricAppMemoryUsed),
		string(MetricAppMemoryTotal),
		string(MetricAppMemoryHeap),
		string(MetricAppMemoryNonHeap),
		string(MetricAppGCCount),
		string(MetricAppGCTime),
		string(MetricAppThreadTotal),
		string(MetricAppThreadActive),
		string(MetricAppThreadBlocked),
		string(MetricAppThreadWaiting),
		string(MetricAppCPUUsage),
		string(MetricAppClassLoaded),
	}
}

// GetCategory 实现 MetricProvider 接口
func (a *AppMetrics) GetCategory() models.MetricCategory {
	return models.CategoryApp
}

// ToMetricPoints 实现 MetricProvider 接口
func (a *AppMetrics) ToMetricPoints() []models.MetricPoint {
	points := make([]models.MetricPoint, 0, 12)

	labels := map[string]string{
		"source":   a.Source,
		"instance": a.Instance,
	}

	points = append(points,
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppMemoryUsed),
			MetricType: models.MetricTypeGauge,
			Value:      a.Metrics.Memory.UsedMB,
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
			Unit:       "MB",
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppMemoryTotal),
			MetricType: models.MetricTypeGauge,
			Value:      a.Metrics.Memory.TotalMB,
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
			Unit:       "MB",
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppMemoryHeap),
			MetricType: models.MetricTypeGauge,
			Value:      a.Metrics.Memory.HeapMB,
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
			Unit:       "MB",
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppMemoryNonHeap),
			MetricType: models.MetricTypeGauge,
			Value:      a.Metrics.Memory.NonHeapMB,
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
			Unit:       "MB",
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppGCCount),
			MetricType: models.MetricTypeCounter,
			Value:      float64(a.Metrics.Memory.GCCount),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppGCTime),
			MetricType: models.MetricTypeCounter,
			Value:      float64(a.Metrics.Memory.GCTimeMs),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
			Unit:       "ms",
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppThreadTotal),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.Metrics.Threads.Total),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppThreadActive),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.Metrics.Threads.Active),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppThreadBlocked),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.Metrics.Threads.Blocked),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppThreadWaiting),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.Metrics.Threads.Waiting),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppCPUUsage),
			MetricType: models.MetricTypeGauge,
			Value:      a.Metrics.CPUUsagePercent,
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
			Unit:       "%",
		},
		models.MetricPoint{
			Timestamp:  a.Timestamp,
			MetricName: string(MetricAppClassLoaded),
			MetricType: models.MetricTypeGauge,
			Value:      float64(a.Metrics.ClassLoaded),
			Source:     a.Source,
			Instance:   a.Instance,
			Category:   models.CategoryApp,
			Labels:     labels,
		},
	)

	return points
}

// ApplicationMetricName 定义应用指标的名称常量
type ApplicationMetricName string

const (
	// 应用状态相关指标
	MetricAppMemoryUsed    ApplicationMetricName = "app.memory.used"
	MetricAppMemoryTotal   ApplicationMetricName = "app.memory.total"
	MetricAppMemoryHeap    ApplicationMetricName = "app.memory.heap"
	MetricAppMemoryNonHeap ApplicationMetricName = "app.memory.non_heap"
	MetricAppGCCount       ApplicationMetricName = "app.gc.count"
	MetricAppGCTime        ApplicationMetricName = "app.gc.time"
	MetricAppThreadTotal   ApplicationMetricName = "app.thread.total"
	MetricAppThreadActive  ApplicationMetricName = "app.thread.active"
	MetricAppThreadBlocked ApplicationMetricName = "app.thread.blocked"
	MetricAppThreadWaiting ApplicationMetricName = "app.thread.waiting"
	MetricAppCPUUsage      ApplicationMetricName = "app.cpu.usage"
	MetricAppClassLoaded   ApplicationMetricName = "app.class.loaded"
)
