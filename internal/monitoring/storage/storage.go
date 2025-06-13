// Package storage 实现指标数据的本地存储
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/internal/monitoring/models"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v3"
	"go.uber.org/zap"
)

// Config 定义存储引擎的配置选项
type Config struct {
	// DataDir 指定数据存储的目录
	DataDir string

	// RetentionDays 指定数据保留的天数
	RetentionDays int

	// Logger 日志记录器
	Logger *zap.Logger
}

// Storage 实现指标数据的本地持久化存储
type Storage struct {
	config     Config
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	logger     *zap.Logger

	// BadgerDB实例
	db *badger.DB
}

// 键前缀定义
const (
	serverMetricsPrefix = "server:"
	apiCallsPrefix      = "api:"
	appMetricsPrefix    = "app:"
)

// New 创建一个新的存储引擎实例
func New(config Config) (*Storage, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 创建BadgerDB实例
	dbPath := filepath.Join(config.DataDir, "badger")
	opts := badger.DefaultOptions(dbPath)
	// 根据需要调整BadgerDB选项
	opts.Logger = nil // 禁用默认日志器，在生产环境中应该设置适当的日志器

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("打开BadgerDB失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认logger，如果没有提供
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
		defer logger.Sync()
	}

	s := &Storage{
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
		db:         db,
		logger:     logger,
	}

	return s, nil
}

// Start 启动存储引擎
func (s *Storage) Start() error {
	s.logger.Info("存储引擎启动中")

	// 启动数据清理任务
	s.wg.Add(1)
	go s.runRetentionCleaner()

	s.logger.Info("存储引擎已启动成功")
	return nil
}

// Stop 停止存储引擎
func (s *Storage) Stop() error {
	s.logger.Info("停止存储引擎")
	s.cancelFunc()
	s.wg.Wait()

	// 关闭数据库
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error("关闭BadgerDB失败", zap.Error(err))
			return fmt.Errorf("关闭BadgerDB失败: %w", err)
		}
	}

	s.logger.Info("存储引擎已停止")
	return nil
}

// StoreServerMetrics 存储服务器指标
func (s *Storage) StoreServerMetrics(metrics *models.ServerMetrics) error {
	// 生成键：前缀 + 主机名 + 时间戳
	key := fmt.Sprintf("%s%s:%d", serverMetricsPrefix, metrics.Hostname, metrics.Timestamp.UnixNano())

	// 序列化指标数据
	data, err := json.Marshal(metrics)
	if err != nil {
		s.logger.Error("序列化服务器指标失败", zap.Error(err))
		return fmt.Errorf("序列化服务器指标失败: %w", err)
	}

	// 写入数据库
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err != nil {
		s.logger.Error("存储服务器指标失败", zap.Error(err))
		return fmt.Errorf("存储服务器指标失败: %w", err)
	}

	s.logger.Debug("存储服务器指标成功",
		zap.String("hostname", metrics.Hostname),
		zap.Time("timestamp", metrics.Timestamp),
		zap.Int("metricsCount", len(metrics.Metrics)))

	return nil
}

// StoreAPICalls 存储API调用信息
func (s *Storage) StoreAPICalls(calls *models.APICalls) error {
	// 生成键：前缀 + 来源 + 实例 + 时间戳
	key := fmt.Sprintf("%s%s:%s:%d", apiCallsPrefix, calls.Source, calls.Instance, calls.Timestamp.UnixNano())

	// 为每个API调用设置时间戳
	for i := range calls.Calls {
		calls.Calls[i].Timestamp = calls.Timestamp
	}

	// 序列化API调用数据
	data, err := json.Marshal(calls)
	if err != nil {
		s.logger.Error("序列化API调用信息失败", zap.Error(err))
		return fmt.Errorf("序列化API调用信息失败: %w", err)
	}

	// 写入数据库
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err != nil {
		s.logger.Error("存储API调用信息失败", zap.Error(err))
		return fmt.Errorf("存储API调用信息失败: %w", err)
	}

	s.logger.Debug("存储API调用信息成功",
		zap.String("source", calls.Source),
		zap.String("instance", calls.Instance),
		zap.Time("timestamp", calls.Timestamp),
		zap.Int("callsCount", len(calls.Calls)))

	return nil
}

// StoreAppMetrics 存储应用状态指标
func (s *Storage) StoreAppMetrics(metrics *models.AppMetrics) error {
	// 生成键：前缀 + 来源 + 实例 + 时间戳
	key := fmt.Sprintf("%s%s:%s:%d", appMetricsPrefix, metrics.Source, metrics.Instance, metrics.Timestamp.UnixNano())

	// 序列化应用指标数据
	data, err := json.Marshal(metrics)
	if err != nil {
		s.logger.Error("序列化应用状态指标失败", zap.Error(err))
		return fmt.Errorf("序列化应用状态指标失败: %w", err)
	}

	// 写入数据库
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err != nil {
		s.logger.Error("存储应用状态指标失败", zap.Error(err))
		return fmt.Errorf("存储应用状态指标失败: %w", err)
	}

	s.logger.Debug("存储应用状态指标成功",
		zap.String("source", metrics.Source),
		zap.String("instance", metrics.Instance),
		zap.Time("timestamp", metrics.Timestamp))

	return nil
}

// QueryTimeSeries 查询时间序列数据
func (s *Storage) QueryTimeSeries(options models.QueryOptions) (*models.QueryResult, error) {
	result := &models.QueryResult{
		Series: make([]models.TimeSeries, 0),
	}

	// 根据请求的指标名称创建不同的时间序列
	seriesMap := make(map[string]*models.TimeSeries)

	// 格式化时间区间
	startTime := options.StartTime
	endTime := options.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-1 * time.Hour) // 默认查询最近1小时
	}

	s.logger.Debug("查询时间序列",
		zap.Time("startTime", startTime),
		zap.Time("endTime", endTime),
		zap.Strings("metricNames", options.MetricNames),
		zap.Any("labelMatchers", options.LabelMatchers))

	// 根据指标类型分别查询
	err := s.db.View(func(txn *badger.Txn) error {
		// 创建迭代器选项
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历所有服务器指标
		for _, metricName := range options.MetricNames {
			// 确定要查询的前缀
			var prefix string
			if isServerMetric(metricName) {
				prefix = serverMetricsPrefix
			} else if isAPIMetric(metricName) {
				prefix = apiCallsPrefix
			} else if isAppMetric(metricName) {
				prefix = appMetricsPrefix
			} else {
				s.logger.Warn("未知的指标类型", zap.String("metricName", metricName))
				continue
			}

			s.logger.Debug("查询指标", zap.String("metricName", metricName), zap.String("prefix", prefix))

			// 使用前缀查询
			for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
				item := it.Item()

				// 提取时间戳，检查是否在时间范围内
				timestamp := extractTimestampFromKey(item.Key())
				ts := time.Unix(0, timestamp)

				if ts.Before(startTime) || ts.After(endTime) {
					continue
				}

				// 处理不同类型的指标
				err := item.Value(func(val []byte) error {
					if isServerMetric(metricName) {
						var serverMetrics models.ServerMetrics
						if err := json.Unmarshal(val, &serverMetrics); err != nil {
							return err
						}

						// 检查是否包含请求的指标
						metricValue, exists := serverMetrics.Metrics[models.ServerMetricName(metricName)]
						if !exists {
							return nil
						}

						// 创建标签
						labels := map[string]string{
							"hostname": serverMetrics.Hostname,
						}

						// 检查标签匹配
						if !labelsMatch(options.LabelMatchers, labels) {
							return nil
						}

						// 获取或创建对应的时间序列
						seriesKey := fmt.Sprintf("%s:%s", metricName, serverMetrics.Hostname)
						series, exists := seriesMap[seriesKey]
						if !exists {
							series = &models.TimeSeries{
								Name:   metricName,
								Labels: labels,
								Points: make([]models.TimeSeriesPoint, 0),
							}
							seriesMap[seriesKey] = series
						}

						// 添加数据点
						series.Points = append(series.Points, models.TimeSeriesPoint{
							Timestamp: serverMetrics.Timestamp,
							Value:     metricValue,
						})
					} else if isAPIMetric(metricName) {
						var apiCalls models.APICalls
						if err := json.Unmarshal(val, &apiCalls); err != nil {
							return err
						}

						// 为每个API调用计算指标
						for _, call := range apiCalls.Calls {
							// 创建标签
							labels := map[string]string{
								"source":   apiCalls.Source,
								"instance": apiCalls.Instance,
								"endpoint": call.Endpoint,
								"method":   call.Method,
							}

							// 添加自定义标签
							for k, v := range call.Tags {
								labels[k] = v
							}

							// 检查标签匹配
							if !labelsMatch(options.LabelMatchers, labels) {
								continue
							}

							// 根据指标名称获取值
							var metricValue float64
							switch models.ApplicationMetricName(metricName) {
							case models.MetricAPIRequestCount:
								metricValue = 1 // 每个调用计为1次
							case models.MetricAPIRequestDuration:
								metricValue = call.DurationMs
							case models.MetricAPIRequestError:
								metricValue = 0
								if call.HasError {
									metricValue = 1
								}
							case models.MetricAPIRequestSize:
								metricValue = float64(call.RequestSize)
							case models.MetricAPIResponseSize:
								metricValue = float64(call.ResponseSize)
							default:
								continue // 这里的 continue 是在 for 循环内，所以没问题
							}

							// 获取或创建对应的时间序列
							seriesKey := fmt.Sprintf("%s:%s:%s:%s:%s",
								metricName, apiCalls.Source, apiCalls.Instance, call.Endpoint, call.Method)
							series, exists := seriesMap[seriesKey]
							if !exists {
								series = &models.TimeSeries{
									Name:   metricName,
									Labels: labels,
									Points: make([]models.TimeSeriesPoint, 0),
								}
								seriesMap[seriesKey] = series
							}

							// 添加数据点
							series.Points = append(series.Points, models.TimeSeriesPoint{
								Timestamp: apiCalls.Timestamp,
								Value:     metricValue,
							})
						}
					} else if isAppMetric(metricName) {
						var appMetrics models.AppMetrics
						if err := json.Unmarshal(val, &appMetrics); err != nil {
							return err
						}

						// 创建标签
						labels := map[string]string{
							"source":   appMetrics.Source,
							"instance": appMetrics.Instance,
						}

						// 检查标签匹配
						if !labelsMatch(options.LabelMatchers, labels) {
							return nil
						}

						// 根据指标名称获取值
						var metricValue float64
						switch models.ApplicationMetricName(metricName) {
						case models.MetricAppMemoryUsed:
							metricValue = appMetrics.Metrics.Memory.UsedMB
						case models.MetricAppMemoryTotal:
							metricValue = appMetrics.Metrics.Memory.TotalMB
						case models.MetricAppMemoryHeap:
							metricValue = appMetrics.Metrics.Memory.HeapMB
						case models.MetricAppMemoryNonHeap:
							metricValue = appMetrics.Metrics.Memory.NonHeapMB
						case models.MetricAppGCCount:
							metricValue = float64(appMetrics.Metrics.Memory.GCCount)
						case models.MetricAppGCTime:
							metricValue = float64(appMetrics.Metrics.Memory.GCTimeMs)
						case models.MetricAppThreadTotal:
							metricValue = float64(appMetrics.Metrics.Threads.Total)
						case models.MetricAppThreadActive:
							metricValue = float64(appMetrics.Metrics.Threads.Active)
						case models.MetricAppThreadBlocked:
							metricValue = float64(appMetrics.Metrics.Threads.Blocked)
						case models.MetricAppThreadWaiting:
							metricValue = float64(appMetrics.Metrics.Threads.Waiting)
						case models.MetricAppCPUUsage:
							metricValue = appMetrics.Metrics.CPUUsagePercent
						case models.MetricAppClassLoaded:
							metricValue = float64(appMetrics.Metrics.ClassLoaded)
						default:
							return nil // 改用 return nil 而不是 continue
						}

						// 获取或创建对应的时间序列
						seriesKey := fmt.Sprintf("%s:%s:%s", metricName, appMetrics.Source, appMetrics.Instance)
						series, exists := seriesMap[seriesKey]
						if !exists {
							series = &models.TimeSeries{
								Name:   metricName,
								Labels: labels,
								Points: make([]models.TimeSeriesPoint, 0),
							}
							seriesMap[seriesKey] = series
						}

						// 添加数据点
						series.Points = append(series.Points, models.TimeSeriesPoint{
							Timestamp: appMetrics.Timestamp,
							Value:     metricValue,
						})
					}
					return nil
				})

				if err != nil {
					s.logger.Error("处理指标数据失败", zap.Error(err))
					continue
				}
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("查询时间序列失败", zap.Error(err))
		return nil, fmt.Errorf("查询时间序列失败: %w", err)
	}

	// 将结果从map转为slice
	for _, series := range seriesMap {
		// 如果设置了聚合和间隔，需要对数据进行处理
		if options.Aggregation != "" && options.Interval != "" {
			// 解析时间间隔
			interval, err := time.ParseDuration(options.Interval)
			if err != nil {
				s.logger.Warn("解析时间间隔失败，使用原始数据", zap.Error(err))
			} else {
				series.Points = s.aggregatePoints(series.Points, interval, options.Aggregation)
			}
		}

		// 如果设置了返回限制，对结果进行裁剪
		if options.Limit > 0 && len(series.Points) > options.Limit {
			series.Points = series.Points[len(series.Points)-options.Limit:]
		}

		result.Series = append(result.Series, *series)
	}

	s.logger.Debug("查询时间序列完成",
		zap.Int("seriesCount", len(result.Series)))
	return result, nil
}

// aggregatePoints 根据时间间隔和聚合方法对数据点进行聚合
func (s *Storage) aggregatePoints(points []models.TimeSeriesPoint, interval time.Duration, aggregation string) []models.TimeSeriesPoint {
	if len(points) == 0 {
		return points
	}

	// 按时间排序
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	// 确定起止时间
	start := points[0].Timestamp.Truncate(interval)
	end := points[len(points)-1].Timestamp.Add(interval).Truncate(interval)

	// 初始化结果
	result := make([]models.TimeSeriesPoint, 0)

	// 对每个时间桶进行聚合
	for t := start; t.Before(end); t = t.Add(interval) {
		nextT := t.Add(interval)

		// 收集当前时间桶内的所有点
		bucket := make([]float64, 0)
		for _, p := range points {
			if !p.Timestamp.Before(t) && p.Timestamp.Before(nextT) {
				bucket = append(bucket, p.Value)
			}
		}

		// 如果桶中没有点，则跳过
		if len(bucket) == 0 {
			continue
		}

		// 根据聚合方法计算值
		var value float64
		switch strings.ToLower(aggregation) {
		case "avg", "average":
			sum := 0.0
			for _, v := range bucket {
				sum += v
			}
			value = sum / float64(len(bucket))
		case "max":
			value = bucket[0]
			for _, v := range bucket {
				if v > value {
					value = v
				}
			}
		case "min":
			value = bucket[0]
			for _, v := range bucket {
				if v < value {
					value = v
				}
			}
		case "sum":
			value = 0
			for _, v := range bucket {
				value += v
			}
		case "count":
			value = float64(len(bucket))
		default: // 默认为平均值
			sum := 0.0
			for _, v := range bucket {
				sum += v
			}
			value = sum / float64(len(bucket))
		}

		// 添加聚合后的点
		result = append(result, models.TimeSeriesPoint{
			Timestamp: t,
			Value:     value,
		})
	}

	return result
}

// QueryAPICalls 查询API调用信息
func (s *Storage) QueryAPICalls(options models.QueryOptions) ([]*models.APICalls, error) {
	result := make([]*models.APICalls, 0)

	// 格式化时间区间
	startTime := options.StartTime
	endTime := options.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-1 * time.Hour) // 默认查询最近1小时
	}

	s.logger.Debug("查询API调用信息",
		zap.Time("startTime", startTime),
		zap.Time("endTime", endTime),
		zap.Any("labelMatchers", options.LabelMatchers))

	// 查询API调用数据
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历API调用数据
		for it.Seek([]byte(apiCallsPrefix)); it.ValidForPrefix([]byte(apiCallsPrefix)); it.Next() {
			item := it.Item()

			// 提取时间戳，检查是否在时间范围内
			timestamp := extractTimestampFromKey(item.Key())
			ts := time.Unix(0, timestamp)

			if ts.Before(startTime) || ts.After(endTime) {
				continue
			}

			// 读取值
			err := item.Value(func(val []byte) error {
				var apiCalls models.APICalls
				if err := json.Unmarshal(val, &apiCalls); err != nil {
					return err
				}

				// 应用标签过滤
				if len(options.LabelMatchers) > 0 {
					labels := map[string]string{
						"source":   apiCalls.Source,
						"instance": apiCalls.Instance,
					}

					if !labelsMatch(options.LabelMatchers, labels) {
						return nil // 不匹配，跳过
					}
				}

				// 添加到结果
				result = append(result, &apiCalls)
				return nil
			})

			if err != nil {
				s.logger.Error("处理API调用数据失败", zap.Error(err))
				continue
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("查询API调用信息失败", zap.Error(err))
		return nil, fmt.Errorf("查询API调用信息失败: %w", err)
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	// 如果设置了返回限制，对结果进行裁剪
	if options.Limit > 0 && len(result) > options.Limit {
		result = result[len(result)-options.Limit:]
	}

	s.logger.Debug("查询API调用信息完成",
		zap.Int("count", len(result)))
	return result, nil
}

// QueryAppMetrics 查询应用状态指标
func (s *Storage) QueryAppMetrics(options models.QueryOptions) ([]*models.AppMetrics, error) {
	result := make([]*models.AppMetrics, 0)

	// 格式化时间区间
	startTime := options.StartTime
	endTime := options.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-1 * time.Hour) // 默认查询最近1小时
	}

	s.logger.Debug("查询应用状态指标",
		zap.Time("startTime", startTime),
		zap.Time("endTime", endTime),
		zap.Any("labelMatchers", options.LabelMatchers))

	// 查询应用指标数据
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历应用指标数据
		for it.Seek([]byte(appMetricsPrefix)); it.ValidForPrefix([]byte(appMetricsPrefix)); it.Next() {
			item := it.Item()

			// 提取时间戳，检查是否在时间范围内
			timestamp := extractTimestampFromKey(item.Key())
			ts := time.Unix(0, timestamp)

			if ts.Before(startTime) || ts.After(endTime) {
				continue
			}

			// 读取值
			err := item.Value(func(val []byte) error {
				var appMetrics models.AppMetrics
				if err := json.Unmarshal(val, &appMetrics); err != nil {
					return err
				}

				// 应用标签过滤
				if len(options.LabelMatchers) > 0 {
					labels := map[string]string{
						"source":   appMetrics.Source,
						"instance": appMetrics.Instance,
					}

					if !labelsMatch(options.LabelMatchers, labels) {
						return nil // 不匹配，跳过
					}
				}

				// 添加到结果
				result = append(result, &appMetrics)
				return nil
			})

			if err != nil {
				s.logger.Error("处理应用指标数据失败", zap.Error(err))
				continue
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("查询应用状态指标失败", zap.Error(err))
		return nil, fmt.Errorf("查询应用状态指标失败: %w", err)
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	// 如果设置了返回限制，对结果进行裁剪
	if options.Limit > 0 && len(result) > options.Limit {
		result = result[len(result)-options.Limit:]
	}

	s.logger.Debug("查询应用状态指标完成",
		zap.Int("count", len(result)))
	return result, nil
}

// QueryAPICallsDetails 查询API调用详情
func (s *Storage) QueryAPICallsDetails(options models.QueryOptions) ([]*models.APICall, error) {
	result := make([]*models.APICall, 0)

	// 格式化时间区间
	startTime := options.StartTime
	endTime := options.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-1 * time.Hour) // 默认查询最近1小时
	}

	s.logger.Debug("查询API调用详情",
		zap.Time("startTime", startTime),
		zap.Time("endTime", endTime),
		zap.Any("labelMatchers", options.LabelMatchers))

	// 查询API调用数据
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历API调用数据
		for it.Seek([]byte(apiCallsPrefix)); it.ValidForPrefix([]byte(apiCallsPrefix)); it.Next() {
			item := it.Item()

			// 提取时间戳，检查是否在时间范围内
			timestamp := extractTimestampFromKey(item.Key())
			ts := time.Unix(0, timestamp)

			if ts.Before(startTime) || ts.After(endTime) {
				continue
			}

			// 读取值
			err := item.Value(func(val []byte) error {
				var apiCalls models.APICalls
				if err := json.Unmarshal(val, &apiCalls); err != nil {
					return err
				}

				// 过滤来源和实例
				sourceMatch := true
				instanceMatch := true

				if source, ok := options.LabelMatchers["source"]; ok {
					sourceMatch = apiCalls.Source == source
				}
				if instance, ok := options.LabelMatchers["instance"]; ok {
					instanceMatch = apiCalls.Instance == instance
				}

				if !sourceMatch || !instanceMatch {
					return nil // 不匹配，跳过
				}

				// 遍历所有调用，应用过滤条件
				for _, call := range apiCalls.Calls {
					// 创建调用的标签映射
					callLabels := map[string]string{
						"endpoint": call.Endpoint,
						"method":   call.Method,
					}

					// 添加自定义标签
					for k, v := range call.Tags {
						callLabels[k] = v
					}

					// 检查标签匹配（排除已检查的source和instance）
					match := true
					for k, v := range options.LabelMatchers {
						if k == "source" || k == "instance" {
							continue
						}

						if labelValue, exists := callLabels[k]; !exists || labelValue != v {
							match = false
							break
						}
					}

					if !match {
						continue
					}

					// 复制API调用，并添加来源和实例信息
					callCopy := models.APICall{
						Endpoint:     call.Endpoint,
						Method:       call.Method,
						DurationMs:   call.DurationMs,
						StatusCode:   call.StatusCode,
						HasError:     call.HasError,
						ErrorMessage: call.ErrorMessage,
						RequestSize:  call.RequestSize,
						ResponseSize: call.ResponseSize,
						ClientIP:     call.ClientIP,
						Tags:         make(map[string]string),
					}

					// 复制标签
					if call.Tags != nil {
						for k, v := range call.Tags {
							callCopy.Tags[k] = v
						}
					}

					// 添加来源和实例信息
					callCopy.Tags["_source"] = apiCalls.Source
					callCopy.Tags["_instance"] = apiCalls.Instance
					callCopy.Tags["_timestamp"] = apiCalls.Timestamp.Format(time.RFC3339)

					// 添加到结果
					result = append(result, &callCopy)
				}

				return nil
			})

			if err != nil {
				s.logger.Error("处理API调用数据失败", zap.Error(err))
				continue
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("查询API调用详情失败", zap.Error(err))
		return nil, fmt.Errorf("查询API调用详情失败: %w", err)
	}

	// 如果设置了返回限制，对结果进行裁剪
	if options.Limit > 0 && len(result) > options.Limit {
		result = result[len(result)-options.Limit:]
	}

	s.logger.Debug("查询API调用详情完成",
		zap.Int("count", len(result)))
	return result, nil
}

// QueryAppMetricsDetails 查询应用指标详情
func (s *Storage) QueryAppMetricsDetails(options models.QueryOptions) ([]*models.AppMetrics, error) {
	// 此函数与QueryAppMetrics相似，但提供更详细的过滤和处理
	result := make([]*models.AppMetrics, 0)

	// 格式化时间区间
	startTime := options.StartTime
	endTime := options.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-1 * time.Hour) // 默认查询最近1小时
	}

	s.logger.Debug("查询应用指标详情",
		zap.Time("startTime", startTime),
		zap.Time("endTime", endTime),
		zap.Any("labelMatchers", options.LabelMatchers),
		zap.Strings("metricNames", options.MetricNames))

	// 查询应用指标数据
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历应用指标数据
		for it.Seek([]byte(appMetricsPrefix)); it.ValidForPrefix([]byte(appMetricsPrefix)); it.Next() {
			item := it.Item()

			// 提取时间戳，检查是否在时间范围内
			timestamp := extractTimestampFromKey(item.Key())
			ts := time.Unix(0, timestamp)

			if ts.Before(startTime) || ts.After(endTime) {
				continue
			}

			// 读取值
			err := item.Value(func(val []byte) error {
				var appMetrics models.AppMetrics
				if err := json.Unmarshal(val, &appMetrics); err != nil {
					return err
				}

				// 应用标签过滤
				if len(options.LabelMatchers) > 0 {
					labels := map[string]string{
						"source":   appMetrics.Source,
						"instance": appMetrics.Instance,
					}

					if !labelsMatch(options.LabelMatchers, labels) {
						return nil // 不匹配，跳过
					}
				}

				// 如果指定了指标名称，检查是否包含相关指标
				if len(options.MetricNames) > 0 {
					hasRequiredMetric := false
					for _, metricName := range options.MetricNames {
						if isAppMetric(metricName) {
							// 实际检查需要根据指标名称查找对应的应用指标字段
							// 这里简化处理，认为有任何应用指标就匹配
							hasRequiredMetric = true
							break
						}
					}

					if !hasRequiredMetric {
						return nil // 没有请求的指标，跳过
					}
				}

				// 添加到结果
				result = append(result, &appMetrics)
				return nil
			})

			if err != nil {
				s.logger.Error("处理应用指标数据失败", zap.Error(err))
				continue
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("查询应用指标详情失败", zap.Error(err))
		return nil, fmt.Errorf("查询应用指标详情失败: %w", err)
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	// 如果设置了返回限制，对结果进行裁剪
	if options.Limit > 0 && len(result) > options.Limit {
		result = result[len(result)-options.Limit:]
	}

	s.logger.Debug("查询应用指标详情完成",
		zap.Int("count", len(result)))
	return result, nil
}

// labelsMatch 检查标签是否匹配
func labelsMatch(matchers, labels map[string]string) bool {
	if len(matchers) == 0 {
		return true
	}

	for k, v := range matchers {
		if labelValue, exists := labels[k]; !exists || labelValue != v {
			return false
		}
	}
	return true
}

// 辅助函数：判断是否是服务器指标
func isServerMetric(metricName string) bool {
	// 先根据长度进行初步筛选，避免后续越界
	if len(metricName) < 4 {
		return false
	}

	// 检查是否是CPU指标
	if metricName[:4] == "cpu." {
		return true
	}

	// 检查是否是内存指标
	if len(metricName) >= 7 && metricName[:7] == "memory." {
		return true
	}

	// 检查是否是磁盘指标
	if len(metricName) >= 5 && metricName[:5] == "disk." {
		return true
	}

	// 检查是否是网络指标
	if len(metricName) >= 8 && metricName[:8] == "network." {
		return true
	}

	return false
}

// 辅助函数：判断是否是API指标
func isAPIMetric(metricName string) bool {
	return len(metricName) >= 4 && metricName[:4] == "api."
}

// 辅助函数：判断是否是应用指标
func isAppMetric(metricName string) bool {
	return len(metricName) >= 4 && metricName[:4] == "app."
}

// 辅助函数：从键中提取时间戳
func extractTimestamp(key string) time.Time {
	// 键格式: prefix:suffix:timestamp 或 prefix:suffix:suffix:timestamp
	parts := strings.Split(key, ":")
	if len(parts) < 2 {
		return time.Time{}
	}

	// 获取最后一个部分作为时间戳
	tsStr := parts[len(parts)-1]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return time.Time{}
	}

	return time.Unix(0, ts)
}

// runRetentionCleaner 运行数据保留清理任务
func (s *Storage) runRetentionCleaner() {
	defer s.wg.Done()
	s.logger.Info("启动数据保留清理任务",
		zap.Int("retentionDays", s.config.RetentionDays))

	// 每天执行一次清理
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动立即执行一次
	s.cleanStaleData()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("停止数据保留清理任务")
			return
		case <-ticker.C:
			s.cleanStaleData()
		}
	}
}

// cleanStaleData 清理过期数据
func (s *Storage) cleanStaleData() {
	if s.config.RetentionDays <= 0 {
		s.logger.Info("未设置数据保留期限，跳过清理")
		return
	}

	// 计算截止时间
	cutoffTime := time.Now().AddDate(0, 0, -s.config.RetentionDays)
	cutoffNano := cutoffTime.UnixNano()

	s.logger.Info("开始清理过期数据",
		zap.Time("cutoffTime", cutoffTime),
		zap.Int("retentionDays", s.config.RetentionDays))

	// 要删除的键列表
	keysToDelete := make([][]byte, 0)

	// 收集所有过期的键
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		prefixes := []string{serverMetricsPrefix, apiCallsPrefix, appMetricsPrefix}

		for _, prefix := range prefixes {
			for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
				item := it.Item()
				key := item.Key()

				// 提取时间戳
				timestamp := extractTimestampFromKey(key)

				// 如果时间戳早于截止时间，标记为删除
				if timestamp < cutoffNano {
					// 复制键，因为迭代器的键是临时的
					keyCopy := make([]byte, len(key))
					copy(keyCopy, key)
					keysToDelete = append(keysToDelete, keyCopy)
				}
			}
		}

		return nil
	})

	if err != nil {
		s.logger.Error("收集过期数据失败", zap.Error(err))
		return
	}

	// 如果没有过期数据，直接返回
	if len(keysToDelete) == 0 {
		s.logger.Info("没有发现过期数据")
		return
	}

	s.logger.Info("发现过期数据", zap.Int("count", len(keysToDelete)))

	// 批量删除过期数据
	deleteCount := 0
	batchSize := 1000 // 每批删除的键数量

	for i := 0; i < len(keysToDelete); i += batchSize {
		end := i + batchSize
		if end > len(keysToDelete) {
			end = len(keysToDelete)
		}

		batch := keysToDelete[i:end]

		// 使用事务批量删除
		err := s.db.Update(func(txn *badger.Txn) error {
			for _, key := range batch {
				if err := txn.Delete(key); err != nil {
					return err
				}
				deleteCount++
			}
			return nil
		})

		if err != nil {
			s.logger.Error("删除过期数据批次失败",
				zap.Int("batchStart", i),
				zap.Int("batchEnd", end),
				zap.Error(err))
		}
	}

	s.logger.Info("过期数据清理完成",
		zap.Int("deletedCount", deleteCount),
		zap.Int("totalFound", len(keysToDelete)))

	// 手动触发垃圾回收
	err = s.db.RunValueLogGC(0.5) // 尝试回收至少50%的空间
	if err != nil && err != badger.ErrNoRewrite {
		s.logger.Warn("运行值日志垃圾回收失败", zap.Error(err))
	}
}

// extractTimestampFromKey 从键中提取时间戳
func extractTimestampFromKey(key []byte) int64 {
	// 键格式: prefix:suffix:timestamp 或 prefix:suffix:suffix:timestamp
	parts := strings.Split(string(key), ":")
	if len(parts) < 2 {
		return 0
	}

	// 获取最后一个部分作为时间戳
	tsStr := parts[len(parts)-1]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0
	}

	return ts
}
