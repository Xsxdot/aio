// Package storage 实现指标数据的本地存储
package storage

import (
	"encoding/json"
	"fmt"
	models2 "github.com/xsxdot/aio/internal/monitoring/models"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v3"
	"go.uber.org/zap"
)

// 服务相关的键前缀定义
const (
	serviceDataPrefix     = "service-data:"
	serviceAPIDataPrefix  = "service-api:"
	serviceInstancePrefix = "service-instance:"
	serviceEndpointPrefix = "service-endpoint:"
	serviceSummaryPrefix  = "service-summary:"
)

// 键格式:
// service-data:<serviceName>:<instanceId>:<timestamp>
// service-api:<serviceName>:<instanceId>:<timestamp>
// service-instance:<serviceName>:<instanceId>
// service-endpoint:<serviceName>:<endpoint>:<method>
// service-summary:<serviceName>

// 服务映射缓存，增强读取性能
var (
	serviceCache          = make(map[string]*models2.Service)
	serviceInstancesCache = make(map[string]map[string]*models2.ServiceInstance)
	serviceEndpointsCache = make(map[string]map[string]*models2.ServiceEndpoint)
	serviceCacheMutex     = &sync.RWMutex{}
)

// StoreServiceData 存储应用服务监控数据
func (s *Storage) StoreServiceData(data *models2.ServiceData) error {
	// 生成键：前缀 + 服务名 + 实例ID + 时间戳
	key := fmt.Sprintf("%s%s:%s:%d", serviceDataPrefix, data.Source, data.Instance, data.Timestamp.UnixNano())

	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("序列化服务监控数据失败", zap.Error(err))
		return fmt.Errorf("序列化服务监控数据失败: %w", err)
	}

	// 写入数据库
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), jsonData)
	})

	if err != nil {
		s.logger.Error("存储服务监控数据失败", zap.Error(err))
		return fmt.Errorf("存储服务监控数据失败: %w", err)
	}

	// 更新服务实例信息
	s.updateServiceInstance(data)

	s.logger.Debug("存储服务监控数据成功",
		zap.String("source", data.Source),
		zap.String("instance", data.Instance),
		zap.Time("timestamp", data.Timestamp))

	return nil
}

// StoreServiceAPIData 存储应用服务API调用数据
func (s *Storage) StoreServiceAPIData(data *models2.ServiceAPIData) error {
	// 生成键：前缀 + 服务名 + 实例ID + 时间戳
	key := fmt.Sprintf("%s%s:%s:%d", serviceAPIDataPrefix, data.Source, data.Instance, data.Timestamp.UnixNano())

	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("序列化服务API调用数据失败", zap.Error(err))
		return fmt.Errorf("序列化服务API调用数据失败: %w", err)
	}

	// 写入数据库
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), jsonData)
	})

	if err != nil {
		s.logger.Error("存储服务API调用数据失败", zap.Error(err))
		return fmt.Errorf("存储服务API调用数据失败: %w", err)
	}

	// 更新服务接口信息
	s.updateServiceEndpoints(data)

	s.logger.Debug("存储服务API调用数据成功",
		zap.String("source", data.Source),
		zap.String("instance", data.Instance),
		zap.Time("timestamp", data.Timestamp),
		zap.Int("callsCount", len(data.APICalls)))

	return nil
}

// updateServiceInstance 更新服务实例信息
func (s *Storage) updateServiceInstance(data *models2.ServiceData) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	// 确保服务存在于缓存中
	if _, exists := serviceCache[data.Source]; !exists {
		serviceCache[data.Source] = &models2.Service{
			ServiceName: data.Source,
			Version:     data.Version,
			Tags:        data.Tags,
			Instances:   []models2.ServiceInstance{},
			Endpoints:   []models2.ServiceEndpoint{},
			UpdateTime:  data.Timestamp,
		}
	}

	// 更新服务的版本和标签
	if data.Version != "" {
		serviceCache[data.Source].Version = data.Version
	}
	if data.Tags != nil && len(data.Tags) > 0 {
		if serviceCache[data.Source].Tags == nil {
			serviceCache[data.Source].Tags = make(map[string]string)
		}
		for k, v := range data.Tags {
			serviceCache[data.Source].Tags[k] = v
		}
	}
	serviceCache[data.Source].UpdateTime = data.Timestamp

	// 确保实例映射存在
	if _, exists := serviceInstancesCache[data.Source]; !exists {
		serviceInstancesCache[data.Source] = make(map[string]*models2.ServiceInstance)
	}

	// 检查实例是否存在
	instance, exists := serviceInstancesCache[data.Source][data.Instance]
	if !exists {
		// 创建新实例
		instance = &models2.ServiceInstance{
			InstanceID:     data.Instance,
			IP:             data.IP,
			Port:           data.Port,
			Status:         models2.StatusUp,
			LastActive:     data.Timestamp,
			Version:        data.Version,
			StartTime:      data.Timestamp,
			Tags:           data.Tags,
			SystemMetrics:  make(map[string]interface{}),
			RuntimeMetrics: make(map[string]interface{}),
		}
		serviceInstancesCache[data.Source][data.Instance] = instance
	} else {
		// 更新实例信息
		instance.IP = data.IP
		instance.Port = data.Port
		instance.LastActive = data.Timestamp
		if data.Version != "" {
			instance.Version = data.Version
		}
		if data.Tags != nil && len(data.Tags) > 0 {
			if instance.Tags == nil {
				instance.Tags = make(map[string]string)
			}
			for k, v := range data.Tags {
				instance.Tags[k] = v
			}
		}
		// 根据最后活跃时间更新状态
		instance.Status = models2.StatusUp
	}

	// 将更新的实例信息存入数据库
	instanceKey := fmt.Sprintf("%s%s:%s", serviceInstancePrefix, data.Source, data.Instance)
	instanceData, _ := json.Marshal(instance)

	s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(instanceKey), instanceData)
	})

	// 更新服务缓存中的实例列表
	s.refreshServiceInstances(data.Source)

	// 将服务信息存入数据库
	summaryKey := fmt.Sprintf("%s%s", serviceSummaryPrefix, data.Source)
	summaryData, _ := json.Marshal(serviceCache[data.Source])

	s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(summaryKey), summaryData)
	})
}

// updateServiceEndpoints 更新服务接口信息
func (s *Storage) updateServiceEndpoints(data *models2.ServiceAPIData) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	// 确保服务存在于缓存中
	if _, exists := serviceCache[data.Source]; !exists {
		serviceCache[data.Source] = &models2.Service{
			ServiceName: data.Source,
			Instances:   []models2.ServiceInstance{},
			Endpoints:   []models2.ServiceEndpoint{},
			UpdateTime:  data.Timestamp,
		}
	}

	// 确保接口映射存在
	if _, exists := serviceEndpointsCache[data.Source]; !exists {
		serviceEndpointsCache[data.Source] = make(map[string]*models2.ServiceEndpoint)
	}

	// 处理每个API调用
	for _, call := range data.APICalls {
		// 生成唯一的接口标识符 (路径+方法)
		endpointID := fmt.Sprintf("%s:%s", call.Endpoint, call.Method)

		// 检查接口是否存在
		endpoint, exists := serviceEndpointsCache[data.Source][endpointID]
		if !exists {
			// 创建新接口
			endpoint = &models2.ServiceEndpoint{
				Path:    call.Endpoint,
				Method:  call.Method,
				Tags:    make(map[string]string),
				Metrics: make(map[string]interface{}),
			}
			serviceEndpointsCache[data.Source][endpointID] = endpoint

			// 将新接口信息存入数据库
			endpointKey := fmt.Sprintf("%s%s:%s:%s", serviceEndpointPrefix, data.Source, call.Endpoint, call.Method)
			endpointData, _ := json.Marshal(endpoint)

			s.db.Update(func(txn *badger.Txn) error {
				return txn.Set([]byte(endpointKey), endpointData)
			})
		}
	}

	// 更新服务缓存中的接口列表
	s.refreshServiceEndpoints(data.Source)

	// 更新服务的时间戳
	serviceCache[data.Source].UpdateTime = data.Timestamp

	// 将服务信息存入数据库
	summaryKey := fmt.Sprintf("%s%s", serviceSummaryPrefix, data.Source)
	summaryData, _ := json.Marshal(serviceCache[data.Source])

	s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(summaryKey), summaryData)
	})
}

// refreshServiceInstances 刷新服务的实例列表
func (s *Storage) refreshServiceInstances(serviceName string) {
	// 从实例映射中获取所有实例
	instances := make([]models2.ServiceInstance, 0)
	for _, instance := range serviceInstancesCache[serviceName] {
		instances = append(instances, *instance)
	}

	// 更新服务缓存
	if service, exists := serviceCache[serviceName]; exists {
		service.Instances = instances
	}
}

// refreshServiceEndpoints 刷新服务的接口列表
func (s *Storage) refreshServiceEndpoints(serviceName string) {
	// 从接口映射中获取所有接口
	endpoints := make([]models2.ServiceEndpoint, 0)
	for _, endpoint := range serviceEndpointsCache[serviceName] {
		endpoints = append(endpoints, *endpoint)
	}

	// 更新服务缓存
	if service, exists := serviceCache[serviceName]; exists {
		service.Endpoints = endpoints
	}
}

// GetAllServices 获取所有服务列表
func (s *Storage) GetAllServices(options models2.ServiceListOptions) ([]*models2.Service, error) {
	result := make([]*models2.Service, 0)

	// 更新服务实例状态
	s.updateServiceStatuses()

	serviceCacheMutex.RLock()
	defer serviceCacheMutex.RUnlock()

	// 将缓存中的服务添加到结果中，并应用筛选条件
	for _, service := range serviceCache {
		// 根据搜索关键词筛选
		if options.SearchTerm != "" && !strings.Contains(strings.ToLower(service.ServiceName), strings.ToLower(options.SearchTerm)) {
			continue
		}

		// 根据标签筛选
		if options.Tag != "" {
			found := false
			for key, value := range service.Tags {
				if key == options.Tag || value == options.Tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 根据状态筛选
		if options.Status != "" {
			// 检查是否所有实例都符合状态条件
			instancesWithStatus := 0
			for _, instance := range service.Instances {
				if string(instance.Status) == options.Status {
					instancesWithStatus++
				}
			}

			// 如果没有实例符合条件，则跳过此服务
			if instancesWithStatus == 0 {
				continue
			}
		}

		// 添加到结果中
		result = append(result, service)
	}

	// 限制返回数量
	if options.Limit > 0 && options.Offset < len(result) {
		end := options.Offset + options.Limit
		if end > len(result) {
			end = len(result)
		}
		if options.Offset < end {
			result = result[options.Offset:end]
		} else {
			result = []*models2.Service{}
		}
	}

	return result, nil
}

// GetService 获取特定服务的详情
func (s *Storage) GetService(serviceName string) (*models2.Service, error) {
	// 更新服务实例状态
	s.updateServiceStatuses()

	serviceCacheMutex.RLock()
	defer serviceCacheMutex.RUnlock()

	// 检查服务是否存在
	service, exists := serviceCache[serviceName]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", serviceName)
	}

	return service, nil
}

// GetServiceInstances 获取服务的所有实例
func (s *Storage) GetServiceInstances(serviceName string) ([]models2.ServiceInstance, error) {
	// 更新服务实例状态
	s.updateServiceStatuses()

	serviceCacheMutex.RLock()
	defer serviceCacheMutex.RUnlock()

	// 检查服务是否存在
	service, exists := serviceCache[serviceName]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", serviceName)
	}

	return service.Instances, nil
}

// GetServiceInstance 获取服务的特定实例
func (s *Storage) GetServiceInstance(serviceName, instanceID string) (*models2.ServiceInstance, error) {
	// 更新服务实例状态
	s.updateServiceStatuses()

	serviceCacheMutex.RLock()
	defer serviceCacheMutex.RUnlock()

	// 检查服务是否存在
	instanceMap, exists := serviceInstancesCache[serviceName]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", serviceName)
	}

	// 检查实例是否存在
	instance, exists := instanceMap[instanceID]
	if !exists {
		return nil, fmt.Errorf("实例 %s 不存在", instanceID)
	}

	return instance, nil
}

// GetServiceEndpoints 获取服务的所有接口
func (s *Storage) GetServiceEndpoints(serviceName string) ([]models2.ServiceEndpoint, error) {
	serviceCacheMutex.RLock()
	defer serviceCacheMutex.RUnlock()

	// 检查服务是否存在
	service, exists := serviceCache[serviceName]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", serviceName)
	}

	return service.Endpoints, nil
}

// GetServiceEndpoint 获取服务的特定接口
func (s *Storage) GetServiceEndpoint(serviceName, path, method string) (*models2.ServiceEndpoint, error) {
	serviceCacheMutex.RLock()
	defer serviceCacheMutex.RUnlock()

	// 检查服务是否存在
	endpointMap, exists := serviceEndpointsCache[serviceName]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", serviceName)
	}

	// 检查接口是否存在
	endpointID := fmt.Sprintf("%s:%s", path, method)
	endpoint, exists := endpointMap[endpointID]
	if !exists {
		return nil, fmt.Errorf("接口 %s %s 不存在", method, path)
	}

	return endpoint, nil
}

// 更新所有服务实例的状态
func (s *Storage) updateServiceStatuses() {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	now := time.Now()

	// 遍历所有服务的所有实例
	for serviceName, instanceMap := range serviceInstancesCache {
		for instanceID, instance := range instanceMap {
			// 检查最后活跃时间，如果超过30秒则认为实例已停止
			if now.Sub(instance.LastActive) > 30*time.Second {
				instance.Status = models2.StatusDown
			} else {
				instance.Status = models2.StatusUp
			}

			// 更新缓存
			serviceInstancesCache[serviceName][instanceID] = instance
		}

		// 更新服务的实例列表
		s.refreshServiceInstances(serviceName)
	}
}

// QueryServiceMetrics 查询服务的指标数据
func (s *Storage) QueryServiceMetrics(options models2.ServiceQueryOptions) (*models2.QueryResult, error) {
	// 基于查询选项构建查询参数
	queryOpts := models2.QueryOptions{
		StartTime:   options.StartTime,
		EndTime:     options.EndTime,
		Aggregation: options.Aggregation,
		Interval:    options.Interval,
	}

	// 构建标签匹配器
	queryOpts.LabelMatchers = make(map[string]string)
	queryOpts.LabelMatchers["source"] = options.ServiceName
	if options.InstanceID != "" {
		queryOpts.LabelMatchers["instance"] = options.InstanceID
	}

	// 根据不同的查询内容设置度量名称
	// 如果是查询接口指标
	if options.Endpoint != "" {
		queryOpts.LabelMatchers["endpoint"] = options.Endpoint
		if options.Method != "" {
			queryOpts.LabelMatchers["method"] = options.Method
		}
		queryOpts.MetricNames = []string{
			string(models2.MetricAPIRequestCount),
			string(models2.MetricAPIRequestDuration),
			string(models2.MetricAPIRequestError),
		}

		// 查询API相关指标
		return s.QueryTimeSeries(queryOpts)
	} else {
		// 查询应用相关指标
		queryOpts.MetricNames = []string{
			string(models2.MetricAppMemoryUsed),
			string(models2.MetricAppCPUUsage),
			string(models2.MetricAppThreadTotal),
			string(models2.MetricAppGCCount),
		}

		// 查询应用指标
		return s.QueryTimeSeries(queryOpts)
	}
}

// QueryServiceSummary 查询服务的汇总指标
func (s *Storage) QueryServiceSummary(serviceName string) (*models2.ServiceMetricsSummary, error) {
	// 构建汇总指标
	summary := &models2.ServiceMetricsSummary{}

	// 查询时间范围（最近5分钟）
	endTime := time.Now()
	startTime := endTime.Add(-5 * time.Minute)

	// 查询请求总数
	requestCountQuery := models2.QueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		MetricNames: []string{string(models2.MetricAPIRequestCount)},
		LabelMatchers: map[string]string{
			"source": serviceName,
		},
		Aggregation: "sum",
	}

	requestCountResult, err := s.QueryTimeSeries(requestCountQuery)
	if err != nil {
		return nil, err
	}

	// 统计总请求数
	for _, series := range requestCountResult.Series {
		for _, point := range series.Points {
			summary.TotalRequests += int64(point.Value)
		}
	}

	// 查询错误数
	errorCountQuery := models2.QueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		MetricNames: []string{string(models2.MetricAPIRequestError)},
		LabelMatchers: map[string]string{
			"source": serviceName,
		},
		Aggregation: "sum",
	}

	errorCountResult, err := s.QueryTimeSeries(errorCountQuery)
	if err != nil {
		return nil, err
	}

	// 统计错误数
	for _, series := range errorCountResult.Series {
		for _, point := range series.Points {
			summary.ErrorCount += int64(point.Value)
		}
	}

	// 计算错误率
	if summary.TotalRequests > 0 {
		summary.ErrorRate = float64(summary.ErrorCount) / float64(summary.TotalRequests) * 100
	}

	// 查询响应时间
	responseTimeQuery := models2.QueryOptions{
		StartTime:   startTime,
		EndTime:     endTime,
		MetricNames: []string{string(models2.MetricAPIRequestDuration)},
		LabelMatchers: map[string]string{
			"source": serviceName,
		},
		Aggregation: "avg",
	}

	responseTimeResult, err := s.QueryTimeSeries(responseTimeQuery)
	if err != nil {
		return nil, err
	}

	// 计算平均响应时间
	totalTime := 0.0
	count := 0
	for _, series := range responseTimeResult.Series {
		for _, point := range series.Points {
			totalTime += point.Value
			count++
		}
	}

	if count > 0 {
		summary.AvgResponseMs = totalTime / float64(count)
	}

	// 计算QPS
	duration := endTime.Sub(startTime).Seconds()
	if duration > 0 {
		summary.QPS = float64(summary.TotalRequests) / duration
	}

	return summary, nil
}
