package storage

import (
	"encoding/json"
	"fmt"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"math"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v3"
	"go.uber.org/zap"
)

// QueryAPIMetricsTimeSeries 按时间序列查询和聚合API指标
func (s *Storage) QueryAPIMetricsTimeSeries(options models2.APIMetricsQueryOptions) (*models2.APIMetricsResult, error) {
	s.logger.Debug("查询API指标时间序列",
		zap.Time("startTime", options.StartTime),
		zap.Time("endTime", options.EndTime),
		zap.String("source", options.Source),
		zap.String("endpoint", options.Endpoint),
		zap.Duration("interval", options.Interval))

	// 处理默认参数
	if options.EndTime.IsZero() {
		options.EndTime = time.Now()
	}
	if options.StartTime.IsZero() {
		options.StartTime = options.EndTime.Add(-1 * time.Hour) // 默认查询最近1小时
	}
	if options.Interval == 0 {
		options.Interval = 60 * time.Second // 默认1分钟间隔
	}

	// 根据请求的指标类型执行不同的查询
	result := &models2.APIMetricsResult{}

	switch options.Aggregation {
	case models2.AggregationAvg, models2.AggregationMax, models2.AggregationMin,
		models2.AggregationP95, models2.AggregationP99:
		// 查询响应时间
		responseTimeResult, err := s.QueryAPIResponseTimeWithInterval(options)
		if err != nil {
			return nil, err
		}
		result.ResponseTime = responseTimeResult

	case models2.AggregationCount:
		// QPS 查询
		qpsResult, err := s.QueryAPIQPSWithInterval(options)
		if err != nil {
			return nil, err
		}
		result.QPS = qpsResult

	default:
		// 默认查询响应时间
		responseTimeResult, err := s.QueryAPIResponseTimeWithInterval(options)
		if err != nil {
			return nil, err
		}
		result.ResponseTime = responseTimeResult
	}

	return result, nil
}

// QueryAPIResponseTimeWithInterval 按时间间隔查询API响应时间
func (s *Storage) QueryAPIResponseTimeWithInterval(options models2.APIMetricsQueryOptions) (*models2.APIResponseTimeResult, error) {
	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	if len(calls) == 0 {
		return &models2.APIResponseTimeResult{
			Period: models2.TimeRange{
				Start: options.StartTime,
				End:   options.EndTime,
			},
			GroupBy:    makeGroupByMap(options),
			SampleSize: 0,
		}, nil
	}

	// 按时间间隔聚合
	if options.Interval > 0 {
		// 时间序列预处理
		timePoints := generateTimePoints(options.StartTime, options.EndTime, options.Interval)
		responseTimesInIntervals := groupResponseTimesByInterval(calls, timePoints)

		// 提取所有响应时间进行整体统计
		var allResponseTimes []float64
		for _, rt := range responseTimesInIntervals {
			allResponseTimes = append(allResponseTimes, rt...)
		}

		// 整体统计
		sort.Float64s(allResponseTimes)
		count := len(allResponseTimes)

		// 计算统计值
		result := &models2.APIResponseTimeResult{
			Period: models2.TimeRange{
				Start: options.StartTime,
				End:   options.EndTime,
			},
			GroupBy:    makeGroupByMap(options),
			SampleSize: count,
		}

		if count > 0 {
			result.Min = allResponseTimes[0]
			result.Max = allResponseTimes[count-1]

			// 计算平均值
			var sum float64
			for _, rt := range allResponseTimes {
				sum += rt
			}
			result.Avg = sum / float64(count)

			// 计算P95
			p95Index := int(math.Ceil(float64(count)*0.95)) - 1
			if p95Index >= 0 && p95Index < count {
				result.P95 = allResponseTimes[p95Index]
			}

			// 计算P99
			p99Index := int(math.Ceil(float64(count)*0.99)) - 1
			if p99Index >= 0 && p99Index < count {
				result.P99 = allResponseTimes[p99Index]
			}
		}

		return result, nil
	}

	// 无时间间隔，直接返回整体统计
	return s.QueryAPIResponseTime(options)
}

// QueryAPIQPSWithInterval 按时间间隔查询API QPS
func (s *Storage) QueryAPIQPSWithInterval(options models2.APIMetricsQueryOptions) (*models2.APIQPSResult, error) {
	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	// 计算总时间范围（秒）
	timeRangeSec := options.EndTime.Sub(options.StartTime).Seconds()
	if timeRangeSec <= 0 {
		timeRangeSec = 1 // 避免除以零
	}

	// 按时间间隔聚合
	var qps float64
	if options.Interval > 0 {
		// 时间序列预处理
		timePoints := generateTimePoints(options.StartTime, options.EndTime, options.Interval)
		callsInIntervals := groupCallsByInterval(calls, timePoints)

		// 计算平均QPS
		totalCalls := 0
		for _, callsInInterval := range callsInIntervals {
			totalCalls += len(callsInInterval)
		}
		qps = float64(totalCalls) / timeRangeSec
	} else {
		// 无时间间隔，直接计算整体QPS
		qps = float64(len(calls)) / timeRangeSec
	}

	result := &models2.APIQPSResult{
		Period: models2.TimeRange{
			Start: options.StartTime,
			End:   options.EndTime,
		},
		GroupBy:    makeGroupByMap(options),
		QPS:        qps,
		SampleSize: len(calls),
	}

	return result, nil
}

// 辅助函数: 生成时间点数组
func generateTimePoints(start, end time.Time, interval time.Duration) []time.Time {
	var points []time.Time
	current := start

	for current.Before(end) || current.Equal(end) {
		points = append(points, current)
		current = current.Add(interval)
	}

	// 确保最后一个点是结束时间
	if !points[len(points)-1].Equal(end) {
		points = append(points, end)
	}

	return points
}

// 辅助函数: 按时间间隔分组响应时间
func groupResponseTimesByInterval(calls []models2.APICall, timePoints []time.Time) [][]float64 {
	result := make([][]float64, len(timePoints)-1)

	for i := range result {
		result[i] = make([]float64, 0)
	}

	for _, call := range calls {
		// 找到对应的时间区间
		for i := 0; i < len(timePoints)-1; i++ {
			if (call.Timestamp.Equal(timePoints[i]) || call.Timestamp.After(timePoints[i])) &&
				call.Timestamp.Before(timePoints[i+1]) {
				result[i] = append(result[i], call.DurationMs)
				break
			}
		}
	}

	return result
}

// 辅助函数: 按时间间隔分组API调用
func groupCallsByInterval(calls []models2.APICall, timePoints []time.Time) [][]models2.APICall {
	result := make([][]models2.APICall, len(timePoints)-1)

	for i := range result {
		result[i] = make([]models2.APICall, 0)
	}

	for _, call := range calls {
		// 找到对应的时间区间
		for i := 0; i < len(timePoints)-1; i++ {
			if (call.Timestamp.Equal(timePoints[i]) || call.Timestamp.After(timePoints[i])) &&
				call.Timestamp.Before(timePoints[i+1]) {
				result[i] = append(result[i], call)
				break
			}
		}
	}

	return result
}

// QueryAPIResponseTime 查询接口响应时间统计信息
func (s *Storage) QueryAPIResponseTime(options models2.APIMetricsQueryOptions) (*models2.APIResponseTimeResult, error) {
	s.logger.Debug("查询API响应时间",
		zap.Time("startTime", options.StartTime),
		zap.Time("endTime", options.EndTime),
		zap.String("source", options.Source),
		zap.String("endpoint", options.Endpoint),
		zap.Any("groupBy", options.GroupBy))

	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	if len(calls) == 0 {
		return &models2.APIResponseTimeResult{
			Period: models2.TimeRange{
				Start: options.StartTime,
				End:   options.EndTime,
			},
			GroupBy:    makeGroupByMap(options),
			SampleSize: 0,
		}, nil
	}

	// 提取所有响应时间
	var responseTimes []float64
	for _, call := range calls {
		responseTimes = append(responseTimes, call.DurationMs)
	}

	// 排序用于计算分位数
	sort.Float64s(responseTimes)
	count := len(responseTimes)

	// 计算统计值
	result := &models2.APIResponseTimeResult{
		Period: models2.TimeRange{
			Start: options.StartTime,
			End:   options.EndTime,
		},
		GroupBy:    makeGroupByMap(options),
		Min:        responseTimes[0],
		Max:        responseTimes[count-1],
		SampleSize: count,
	}

	// 计算平均值
	var sum float64
	for _, rt := range responseTimes {
		sum += rt
	}
	result.Avg = sum / float64(count)

	// 计算P95
	p95Index := int(math.Ceil(float64(count)*0.95)) - 1
	if p95Index >= 0 && p95Index < count {
		result.P95 = responseTimes[p95Index]
	}

	// 计算P99
	p99Index := int(math.Ceil(float64(count)*0.99)) - 1
	if p99Index >= 0 && p99Index < count {
		result.P99 = responseTimes[p99Index]
	}

	return result, nil
}

// QueryAPIQPS 查询API的每秒查询数
func (s *Storage) QueryAPIQPS(options models2.APIMetricsQueryOptions) (*models2.APIQPSResult, error) {
	s.logger.Debug("查询API QPS",
		zap.Time("startTime", options.StartTime),
		zap.Time("endTime", options.EndTime),
		zap.String("source", options.Source),
		zap.String("endpoint", options.Endpoint))

	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	// 计算总时间范围（秒）
	timeRange := options.EndTime.Sub(options.StartTime).Seconds()
	if timeRange <= 0 {
		timeRange = 1 // 避免除以零
	}

	result := &models2.APIQPSResult{
		Period: models2.TimeRange{
			Start: options.StartTime,
			End:   options.EndTime,
		},
		GroupBy:    makeGroupByMap(options),
		SampleSize: len(calls),
		QPS:        float64(len(calls)) / timeRange,
	}

	return result, nil
}

// QueryAPIErrorRate 查询API错误率
func (s *Storage) QueryAPIErrorRate(options models2.APIMetricsQueryOptions) (*models2.APIErrorRateResult, error) {
	s.logger.Debug("查询API错误率",
		zap.Time("startTime", options.StartTime),
		zap.Time("endTime", options.EndTime),
		zap.String("source", options.Source),
		zap.String("endpoint", options.Endpoint))

	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	// 计算错误次数
	errorCount := 0
	totalCount := len(calls)

	for _, call := range calls {
		if call.HasError {
			errorCount++
		}
	}

	// 计算错误率
	errorRate := 0.0
	if totalCount > 0 {
		errorRate = float64(errorCount) / float64(totalCount)
	}

	result := &models2.APIErrorRateResult{
		Period: models2.TimeRange{
			Start: options.StartTime,
			End:   options.EndTime,
		},
		GroupBy:    makeGroupByMap(options),
		ErrorRate:  errorRate,
		ErrorCount: errorCount,
		TotalCount: totalCount,
	}

	return result, nil
}

// QueryAPICallDistribution 查询API调用频率分布
func (s *Storage) QueryAPICallDistribution(options models2.APIMetricsQueryOptions) (*models2.APICallDistributionResult, error) {
	s.logger.Debug("查询API调用分布",
		zap.Time("startTime", options.StartTime),
		zap.Time("endTime", options.EndTime),
		zap.String("source", options.Source))

	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	// 统计每个端点的调用次数
	endpointMap := make(map[string]int)
	endpointMethodMap := make(map[string]map[string]int)                   // endpoint -> method -> count
	endpointMethodTagsMap := make(map[string]map[string]map[string]string) // endpoint -> method -> tags

	for _, call := range calls {
		key := call.Endpoint
		endpointMap[key]++

		// 记录方法
		if _, exists := endpointMethodMap[key]; !exists {
			endpointMethodMap[key] = make(map[string]int)
			endpointMethodTagsMap[key] = make(map[string]map[string]string)
		}
		endpointMethodMap[key][call.Method]++
		endpointMethodTagsMap[key][call.Method] = call.Tags
	}

	// 构建结果
	totalCount := len(calls)
	items := make([]models2.APICallDistributionItem, 0)

	for endpoint, methodMap := range endpointMethodMap {
		for method, count := range methodMap {
			percent := 0.0
			if totalCount > 0 {
				percent = float64(count) / float64(totalCount) * 100
			}

			items = append(items, models2.APICallDistributionItem{
				Endpoint: endpoint,
				Method:   method,
				Tags:     endpointMethodTagsMap[endpoint][method],
				Count:    count,
				Percent:  percent,
			})
		}
	}

	// 按调用次数降序排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	// 限制返回数量
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}

	result := &models2.APICallDistributionResult{
		Period: models2.TimeRange{
			Start: options.StartTime,
			End:   options.EndTime,
		},
		GroupBy: makeGroupByMap(options),
		Total:   totalCount,
		Items:   items,
	}

	return result, nil
}

// QueryAPISummary 查询API调用概要统计
func (s *Storage) QueryAPISummary(options models2.APIMetricsQueryOptions) (*models2.APISummaryResult, error) {
	s.logger.Debug("查询API概要统计",
		zap.Time("startTime", options.StartTime),
		zap.Time("endTime", options.EndTime),
		zap.String("source", options.Source))

	// 查询原始API调用数据
	calls, err := s.queryFilteredAPICalls(options)
	if err != nil {
		return nil, fmt.Errorf("查询API调用数据失败: %w", err)
	}

	// 如果没有数据，返回空结果
	if len(calls) == 0 {
		return &models2.APISummaryResult{
			Period: models2.TimeRange{
				Start: options.StartTime,
				End:   options.EndTime,
			},
			GroupBy:         makeGroupByMap(options),
			TotalCalls:      0,
			UniqueEndpoints: 0,
			ErrorCount:      0,
			ErrorRate:       0,
			AvgResponseTime: 0,
			P95ResponseTime: 0,
			TopEndpoints:    []models2.APICallDistributionItem{},
		}, nil
	}

	// 统计总体指标
	totalCalls := len(calls)
	errorCount := 0

	// 端点统计
	endpointMap := make(map[string]bool)
	endpointMethodMap := make(map[string]map[string]int)                   // endpoint -> method -> count
	endpointMethodTagsMap := make(map[string]map[string]map[string]string) // endpoint -> method -> tags

	// 响应时间统计
	var responseTimes []float64

	for _, call := range calls {
		// 错误计数
		if call.HasError {
			errorCount++
		}

		// 响应时间
		responseTimes = append(responseTimes, call.DurationMs)

		// 端点统计
		endpointMap[call.Endpoint] = true

		// 方法统计
		key := call.Endpoint
		if _, exists := endpointMethodMap[key]; !exists {
			endpointMethodMap[key] = make(map[string]int)
			endpointMethodTagsMap[key] = make(map[string]map[string]string)
		}
		endpointMethodMap[key][call.Method]++
		endpointMethodTagsMap[key][call.Method] = call.Tags
	}

	// 计算唯一端点数
	uniqueEndpoints := len(endpointMap)

	// 计算错误率
	errorRate := 0.0
	if totalCalls > 0 {
		errorRate = float64(errorCount) / float64(totalCalls)
	}

	// 计算响应时间指标
	var avgResponseTime, p95ResponseTime float64
	if len(responseTimes) > 0 {
		// 排序用于计算分位数
		sort.Float64s(responseTimes)

		// 计算平均值
		var sum float64
		for _, rt := range responseTimes {
			sum += rt
		}
		avgResponseTime = sum / float64(len(responseTimes))

		// 计算P95
		p95Index := int(math.Ceil(float64(len(responseTimes))*0.95)) - 1
		if p95Index >= 0 && p95Index < len(responseTimes) {
			p95ResponseTime = responseTimes[p95Index]
		}
	}

	// 准备TopEndpoints数据
	var topEndpoints []models2.APICallDistributionItem
	for endpoint, methodMap := range endpointMethodMap {
		for method, count := range methodMap {
			percent := 0.0
			if totalCalls > 0 {
				percent = float64(count) / float64(totalCalls) * 100
			}

			topEndpoints = append(topEndpoints, models2.APICallDistributionItem{
				Endpoint: endpoint,
				Method:   method,
				Tags:     endpointMethodTagsMap[endpoint][method],
				Count:    count,
				Percent:  percent,
			})
		}
	}

	// 按调用次数降序排序
	sort.Slice(topEndpoints, func(i, j int) bool {
		return topEndpoints[i].Count > topEndpoints[j].Count
	})

	// 限制返回top 5
	topLimit := 5
	if len(topEndpoints) > topLimit {
		topEndpoints = topEndpoints[:topLimit]
	}

	// 构建结果
	result := &models2.APISummaryResult{
		Period: models2.TimeRange{
			Start: options.StartTime,
			End:   options.EndTime,
		},
		GroupBy:         makeGroupByMap(options),
		TotalCalls:      totalCalls,
		UniqueEndpoints: uniqueEndpoints,
		ErrorCount:      errorCount,
		ErrorRate:       errorRate,
		AvgResponseTime: avgResponseTime,
		P95ResponseTime: p95ResponseTime,
		TopEndpoints:    topEndpoints,
	}

	return result, nil
}

// QueryAPISources 查询所有API数据源
func (s *Storage) QueryAPISources() ([]string, error) {
	s.logger.Debug("查询所有API数据源")

	// 查询所有API调用数据
	sources := make(map[string]bool)

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// 遍历API调用数据，提取所有数据源
		for it.Seek([]byte(apiCallsPrefix)); it.ValidForPrefix([]byte(apiCallsPrefix)); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var apiCalls models2.APICalls
				if err := json.Unmarshal(val, &apiCalls); err != nil {
					return err
				}

				// 记录数据源
				if apiCalls.Source != "" {
					sources[apiCalls.Source] = true
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
		s.logger.Error("查询API数据源失败", zap.Error(err))
		return nil, fmt.Errorf("查询API数据源失败: %w", err)
	}

	// 将map转换为排序后的切片
	result := make([]string, 0, len(sources))
	for source := range sources {
		result = append(result, source)
	}

	// 排序结果
	sort.Strings(result)

	return result, nil
}

// 辅助函数: 根据查询选项过滤API调用数据
func (s *Storage) queryFilteredAPICalls(options models2.APIMetricsQueryOptions) ([]models2.APICall, error) {
	var allCalls []models2.APICall

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := apiCallsPrefix
		// 遍历所有API调用数据
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()

			// 提取时间戳，检查是否在时间范围内
			timestamp := extractTimestampFromKey(item.Key())
			ts := time.Unix(0, timestamp)

			if ts.Before(options.StartTime) || ts.After(options.EndTime) {
				continue
			}

			err := item.Value(func(val []byte) error {
				var apiCalls models2.APICalls
				if err := json.Unmarshal(val, &apiCalls); err != nil {
					return err
				}

				// 过滤来源
				if options.Source != "" && apiCalls.Source != options.Source {
					return nil
				}

				// 过滤实例
				if options.Instance != "" && apiCalls.Instance != options.Instance {
					return nil
				}

				// 过滤具体调用
				for _, call := range apiCalls.Calls {
					// 过滤端点
					if options.Endpoint != "" && call.Endpoint != options.Endpoint {
						continue
					}

					// 过滤方法
					if options.Method != "" && call.Method != options.Method {
						continue
					}

					// 过滤标签
					if !matchTags(call.Tags, options.TagFilter) {
						continue
					}

					// 添加到结果
					allCalls = append(allCalls, call)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return allCalls, nil
}

// 辅助函数: 检查标签是否匹配
func matchTags(tags, filter map[string]string) bool {
	if len(filter) == 0 {
		return true
	}

	for k, v := range filter {
		if tags[k] != v {
			return false
		}
	}
	return true
}

// 辅助函数: 创建GroupBy标签映射
func makeGroupByMap(options models2.APIMetricsQueryOptions) map[string]string {
	result := make(map[string]string)

	if options.Source != "" {
		result["source"] = options.Source
	}

	if options.Instance != "" {
		result["instance"] = options.Instance
	}

	if options.Endpoint != "" {
		result["endpoint"] = options.Endpoint
	}

	if options.Method != "" {
		result["method"] = options.Method
	}

	return result
}
