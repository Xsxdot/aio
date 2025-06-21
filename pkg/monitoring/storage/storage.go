// Package storage 实现指标数据的本地存储
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
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

// 确保Storage实现了UnifiedMetricStorage接口
var _ UnifiedMetricStorage = (*Storage)(nil)

// 键前缀定义
const (
	metricPointPrefix = "metric:"
)

// UnifiedMetricStorage 定义统一指标存储接口
type UnifiedMetricStorage interface {
	// StoreMetricPoints 存储指标数据点
	StoreMetricPoints(points []models.MetricPoint) error
	// StoreMetricProvider 存储实现了MetricProvider接口的数据
	StoreMetricProvider(provider models.MetricProvider) error
	// QueryMetricPoints 查询指标数据点
	QueryMetricPoints(query MetricQuery) ([]models.MetricPoint, error)
	// QueryTimeSeries 查询时间序列数据
	QueryTimeSeries(query MetricQuery) (*models.QueryResult, error)
}

// MetricQuery 定义统一的查询接口
type MetricQuery struct {
	StartTime     time.Time
	EndTime       time.Time
	MetricNames   []string
	Categories    []models.MetricCategory
	Sources       []string
	Instances     []string
	LabelMatchers map[string]string
	Aggregation   string // sum, avg, count, max, min
	Interval      string // 聚合间隔，如 "1m", "5m", "1h"
	Limit         int
}

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

// StoreMetricPoints 实现统一的指标点存储方法
func (s *Storage) StoreMetricPoints(points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	// 批量写入事务
	err := s.db.Update(func(txn *badger.Txn) error {
		for _, point := range points {
			// 生成键：前缀 + 分类 + 来源 + 实例 + 指标名 + 时间戳
			key := fmt.Sprintf("%s%s:%s:%s:%s:%d",
				metricPointPrefix,
				point.Category,
				point.Source,
				point.Instance,
				point.MetricName,
				point.Timestamp.UnixNano())

			// 序列化数据
			data, err := json.Marshal(point)
			if err != nil {
				s.logger.Error("序列化指标点失败",
					zap.String("metric", point.MetricName),
					zap.Error(err))
				return fmt.Errorf("序列化指标点失败: %w", err)
			}

			// 写入数据
			if err := txn.Set([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("批量存储指标点失败", zap.Error(err))
		return fmt.Errorf("批量存储指标点失败: %w", err)
	}

	s.logger.Debug("批量存储指标点成功", zap.Int("count", len(points)))
	return nil
}

// StoreMetricProvider 存储实现了MetricProvider接口的数据
func (s *Storage) StoreMetricProvider(provider models.MetricProvider) error {
	points := provider.ToMetricPoints()
	return s.StoreMetricPoints(points)
}

// QueryMetricPoints 查询指标数据点
func (s *Storage) QueryMetricPoints(query MetricQuery) ([]models.MetricPoint, error) {
	var results []models.MetricPoint

	// 设置默认时间范围
	startTime := query.StartTime
	endTime := query.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-1 * time.Hour)
	}

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(metricPointPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			// 解析时间戳进行过滤
			timestamp := extractTimestampFromMetricKey(item.Key())
			if timestamp.Before(startTime) || timestamp.After(endTime) {
				continue
			}

			err := item.Value(func(val []byte) error {
				var point models.MetricPoint
				if err := json.Unmarshal(val, &point); err != nil {
					return err
				}

				// 应用过滤条件
				if s.matchesQuery(&point, query) {
					results = append(results, point)
				}
				return nil
			})
			if err != nil {
				return err
			}

			// 限制结果数量
			if query.Limit > 0 && len(results) >= query.Limit {
				break
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("查询指标点失败", zap.Error(err))
		return nil, fmt.Errorf("查询指标点失败: %w", err)
	}

	s.logger.Debug("查询指标点成功", zap.Int("count", len(results)))
	return results, nil
}

// matchesQuery 检查指标点是否匹配查询条件
func (s *Storage) matchesQuery(point *models.MetricPoint, query MetricQuery) bool {
	// 检查指标名称
	if len(query.MetricNames) > 0 {
		found := false
		for _, name := range query.MetricNames {
			if point.MetricName == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查分类
	if len(query.Categories) > 0 {
		found := false
		for _, category := range query.Categories {
			if point.Category == category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查来源
	if len(query.Sources) > 0 {
		found := false
		for _, source := range query.Sources {
			if point.Source == source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查实例
	if len(query.Instances) > 0 {
		found := false
		for _, instance := range query.Instances {
			if point.Instance == instance {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查标签匹配
	for key, value := range query.LabelMatchers {
		if pointValue, exists := point.Labels[key]; !exists || pointValue != value {
			return false
		}
	}

	return true
}

// extractTimestampFromMetricKey 从统一指标键中提取时间戳
func extractTimestampFromMetricKey(key []byte) time.Time {
	keyStr := string(key)
	parts := strings.Split(keyStr, ":")
	if len(parts) < 5 {
		return time.Time{}
	}

	timestampStr := parts[len(parts)-1]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return time.Time{}
	}

	return time.Unix(0, timestamp)
}

// QueryTimeSeries 实现统一查询接口
func (s *Storage) QueryTimeSeries(query MetricQuery) (*models.QueryResult, error) {
	// 先获取原始指标点
	points, err := s.QueryMetricPoints(query)
	if err != nil {
		return nil, err
	}

	// 将指标点按指标名称和标签分组
	seriesMap := make(map[string]*models.TimeSeries)

	for _, point := range points {
		// 生成系列键
		seriesKey := s.generateSeriesKey(point.MetricName, point.Labels)

		// 获取或创建时间序列
		series, exists := seriesMap[seriesKey]
		if !exists {
			series = &models.TimeSeries{
				Name:   point.MetricName,
				Labels: point.Labels,
				Points: make([]models.TimeSeriesPoint, 0),
			}
			seriesMap[seriesKey] = series
		}

		// 添加数据点
		series.Points = append(series.Points, models.TimeSeriesPoint{
			Timestamp: point.Timestamp,
			Value:     point.Value,
		})
	}

	// 转换为结果格式
	result := &models.QueryResult{
		Series: make([]models.TimeSeries, 0, len(seriesMap)),
	}

	for _, series := range seriesMap {
		// 对时间点排序
		sort.Slice(series.Points, func(i, j int) bool {
			return series.Points[i].Timestamp.Before(series.Points[j].Timestamp)
		})

		// 如果指定了聚合间隔，则进行聚合
		if query.Interval != "" {
			interval, err := time.ParseDuration(query.Interval)
			if err == nil {
				series.Points = s.aggregatePointsByInterval(series.Points, interval, query.Aggregation)
			}
		}

		result.Series = append(result.Series, *series)
	}

	return result, nil
}

// generateSeriesKey 生成时间序列的唯一键
func (s *Storage) generateSeriesKey(metricName string, labels map[string]string) string {
	if len(labels) == 0 {
		return metricName
	}

	// 对标签进行排序以确保一致性
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var keyParts []string
	keyParts = append(keyParts, metricName)
	for _, k := range keys {
		keyParts = append(keyParts, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	return strings.Join(keyParts, ",")
}

// aggregatePointsByInterval 按时间间隔聚合数据点
func (s *Storage) aggregatePointsByInterval(points []models.TimeSeriesPoint, interval time.Duration, aggregation string) []models.TimeSeriesPoint {
	if len(points) == 0 {
		return points
	}

	var result []models.TimeSeriesPoint
	buckets := make(map[int64][]float64)

	// 将数据点分组到时间桶中
	for _, point := range points {
		bucketTime := point.Timestamp.Truncate(interval).Unix()
		buckets[bucketTime] = append(buckets[bucketTime], point.Value)
	}

	// 对每个桶进行聚合
	for bucketTime, values := range buckets {
		timestamp := time.Unix(bucketTime, 0)
		aggregatedValue := s.aggregateValues(values, aggregation)

		result = append(result, models.TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     aggregatedValue,
		})
	}

	// 对结果按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// aggregateValues 聚合值数组
func (s *Storage) aggregateValues(values []float64, aggregation string) float64 {
	if len(values) == 0 {
		return 0
	}

	switch aggregation {
	case "sum":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case "avg":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case "max":
		max := values[0]
		for _, v := range values {
			if v > max {
				max = v
			}
		}
		return max
	case "min":
		min := values[0]
		for _, v := range values {
			if v < min {
				min = v
			}
		}
		return min
	case "count":
		return float64(len(values))
	default:
		// 默认返回最后一个值
		return values[len(values)-1]
	}
}

// runRetentionCleaner 运行数据保留清理任务
func (s *Storage) runRetentionCleaner() {
	defer s.wg.Done()

	ticker := time.NewTicker(24 * time.Hour) // 每24小时运行一次
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanStaleData()
		}
	}
}

// cleanStaleData 清理过期数据
func (s *Storage) cleanStaleData() {
	if s.config.RetentionDays <= 0 {
		return // 如果保留天数为0或负数，不进行清理
	}

	cutoffTime := time.Now().AddDate(0, 0, -s.config.RetentionDays)
	s.logger.Info("开始清理过期数据", zap.Time("cutoffTime", cutoffTime))

	var keysToDelete [][]byte

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // 我们只需要键，不需要值
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// 提取时间戳
			timestamp := extractTimestampFromKey(key)
			ts := time.Unix(0, timestamp)

			// 如果数据超过保留期限，标记删除
			if ts.Before(cutoffTime) {
				keysToDelete = append(keysToDelete, append([]byte(nil), key...))
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("遍历数据库失败", zap.Error(err))
		return
	}

	if len(keysToDelete) == 0 {
		s.logger.Info("没有过期数据需要清理")
		return
	}

	// 批量删除过期数据
	err = s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("删除过期数据失败", zap.Error(err))
	} else {
		s.logger.Info("成功清理过期数据", zap.Int("deletedCount", len(keysToDelete)))
	}

	// 运行垃圾回收
	err = s.db.RunValueLogGC(0.7)
	if err != nil && err != badger.ErrNoRewrite {
		s.logger.Error("运行数据库垃圾回收失败", zap.Error(err))
	}
}

// extractTimestampFromKey 从键中提取时间戳
func extractTimestampFromKey(key []byte) int64 {
	keyStr := string(key)
	parts := strings.Split(keyStr, ":")
	if len(parts) < 2 {
		return 0
	}

	timestamp, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0
	}

	return timestamp
}
