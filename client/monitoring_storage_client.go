// Package client 实现监控存储的gRPC客户端
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoringv1 "github.com/xsxdot/aio/api/proto/monitoring/v1"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"github.com/xsxdot/aio/pkg/monitoring/storage"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	maxCacheSize          = 10000          // 缓存队列最大容量
	nodeCheckInterval     = 10             // 每10次失败检查一次节点
	forceReassignDuration = 24 * time.Hour // 失败超过24小时强制重新分配
)

// MonitoringStorageClient 实现基于gRPC的指标存储客户端
type MonitoringStorageClient struct {
	manager     *GRPCClientManager
	logger      *zap.Logger
	serviceName string

	// 当前分配的存储节点
	currentNode *monitoringv1.StorageNode

	// 到指定节点的直接连接
	nodeConnection *grpc.ClientConn
	nodeClient     monitoringv1.MetricStorageServiceClient
	nodeMutex      sync.RWMutex

	// 缓存队列相关
	cache            []models.MetricPoint
	cacheMutex       sync.Mutex
	failureCount     int        // 失败次数计数器
	firstFailureTime *time.Time // 第一次失败的时间

	// 初始化标志
	initialized bool
	initMutex   sync.Mutex
}

// 确保 MonitoringStorageClient 实现了 UnifiedMetricStorage 接口
var _ storage.UnifiedMetricStorage = (*MonitoringStorageClient)(nil)

// NewMonitoringStorageClientFromManager 从gRPC客户端管理器创建监控存储客户端
func NewMonitoringStorageClientFromManager(manager *GRPCClientManager, serviceName string, logger *zap.Logger) *MonitoringStorageClient {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &MonitoringStorageClient{
		manager:     manager,
		logger:      logger,
		serviceName: serviceName,
		cache:       make([]models.MetricPoint, 0, maxCacheSize),
	}
}

// Initialize 初始化客户端，获取分配的存储节点
func (c *MonitoringStorageClient) Initialize(ctx context.Context) error {
	c.initMutex.Lock()
	defer c.initMutex.Unlock()

	if c.initialized {
		return nil
	}

	node, err := c.getStorageNode(ctx, false)
	if err != nil {
		return fmt.Errorf("获取存储节点失败: %w", err)
	}

	// 建立到指定节点的直接连接
	if err := c.connectToNode(node); err != nil {
		return fmt.Errorf("连接到存储节点失败: %w", err)
	}

	c.currentNode = node
	c.initialized = true
	c.logger.Info("监控存储客户端初始化成功",
		zap.String("service_name", c.serviceName),
		zap.String("node_id", node.NodeId),
		zap.String("node_address", node.Address))

	return nil
}

// connectToNode 建立到指定存储节点的直接连接
func (c *MonitoringStorageClient) connectToNode(node *monitoringv1.StorageNode) error {
	c.nodeMutex.Lock()
	defer c.nodeMutex.Unlock()

	// 关闭现有连接
	if c.nodeConnection != nil {
		c.nodeConnection.Close()
		c.nodeConnection = nil
		c.nodeClient = nil
	}

	// 获取认证凭据
	credentials := c.manager.GetCredentials()

	// 建立新连接
	var dialOptions []grpc.DialOption
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if credentials != nil {
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(credentials))
	}

	conn, err := grpc.NewClient(node.Address, dialOptions...)
	if err != nil {
		return fmt.Errorf("连接到节点 %s 失败: %w", node.Address, err)
	}

	// 测试连接
	client := monitoringv1.NewMetricStorageServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 发送一个简单的请求测试连接
	_, err = client.StoreMetricPoints(ctx, &monitoringv1.StoreMetricPointsRequest{
		Points: []*monitoringv1.MetricPoint{}, // 空请求用于测试连接
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("测试连接到节点 %s 失败: %w", node.Address, err)
	}

	c.nodeConnection = conn
	c.nodeClient = client

	c.logger.Info("成功建立到存储节点的连接",
		zap.String("node_id", node.NodeId),
		zap.String("node_address", node.Address))

	return nil
}

// getStorageNode 获取存储节点分配
func (c *MonitoringStorageClient) getStorageNode(ctx context.Context, forceReassign bool) (*monitoringv1.StorageNode, error) {
	client := c.manager.GetMonitoringClient()

	req := &monitoringv1.GetStorageNodeRequest{
		ServiceName:   c.serviceName,
		ForceReassign: forceReassign,
	}

	resp, err := client.GetStorageNode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用GetStorageNode失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取存储节点失败: %s", resp.Message)
	}

	return resp.Node, nil
}

// getNodeClient 获取到存储节点的客户端
func (c *MonitoringStorageClient) getNodeClient() (monitoringv1.MetricStorageServiceClient, error) {
	c.nodeMutex.RLock()
	defer c.nodeMutex.RUnlock()

	if c.nodeClient == nil {
		return nil, fmt.Errorf("节点客户端未初始化")
	}

	return c.nodeClient, nil
}

// StoreMetricPoints 存储指标数据点
func (c *MonitoringStorageClient) StoreMetricPoints(points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	// 确保客户端已初始化
	if !c.initialized {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.Initialize(ctx); err != nil {
			return fmt.Errorf("初始化客户端失败: %w", err)
		}
	}

	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	// 先处理缓存中的数据
	if len(c.cache) > 0 {
		if err := c.sendCachedData(); err != nil {
			c.logger.Error("发送缓存数据失败", zap.Error(err))
		}
	}

	// 尝试直接发送新数据
	if err := c.sendMetricPoints(points); err != nil {
		// 发送失败，记录第一次失败时间
		if c.firstFailureTime == nil {
			now := time.Now()
			c.firstFailureTime = &now
			c.logger.Info("记录第一次发送失败时间", zap.Time("first_failure_time", now))
		}

		// 将数据添加到缓存
		c.addToCache(points)
		return fmt.Errorf("发送指标数据失败: %w", err)
	}

	// 发送成功，重置失败状态
	if c.firstFailureTime != nil {
		c.firstFailureTime = nil
		c.failureCount = 0
		c.logger.Debug("直接发送成功，重置失败状态")
	}

	return nil
}

// StoreMetricProvider 存储实现了MetricProvider接口的数据
func (c *MonitoringStorageClient) StoreMetricProvider(provider models.MetricProvider) error {
	points := provider.ToMetricPoints()
	return c.StoreMetricPoints(points)
}

// sendCachedData 发送缓存中的数据
func (c *MonitoringStorageClient) sendCachedData() error {
	if len(c.cache) == 0 {
		return nil
	}

	// 取出第一条数据尝试发送
	firstPoint := []models.MetricPoint{c.cache[0]}
	if err := c.sendMetricPoints(firstPoint); err != nil {
		c.failureCount++

		// 记录第一次失败时间
		if c.firstFailureTime == nil {
			now := time.Now()
			c.firstFailureTime = &now
			c.logger.Info("记录第一次发送失败时间", zap.Time("first_failure_time", now))
		}

		c.logger.Warn("发送缓存数据失败",
			zap.Int("failure_count", c.failureCount),
			zap.Time("first_failure_time", *c.firstFailureTime),
			zap.Error(err))

		// 检查是否需要重新获取节点
		shouldCheckNode := false
		forceReassign := false

		// 每10次失败检查一次节点
		if c.failureCount%nodeCheckInterval == 0 {
			shouldCheckNode = true
			c.logger.Info("达到检查节点间隔，尝试获取最新节点",
				zap.Int("failure_count", c.failureCount))
		}

		// 如果第一次失败时间超过24小时，强制重新分配
		if c.firstFailureTime != nil && time.Since(*c.firstFailureTime) > forceReassignDuration {
			shouldCheckNode = true
			forceReassign = true
			c.logger.Warn("第一次失败时间超过24小时，强制重新分配节点",
				zap.Duration("failure_duration", time.Since(*c.firstFailureTime)))
		}

		if shouldCheckNode {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			node, err := c.getStorageNode(ctx, forceReassign)
			if err != nil {
				c.logger.Error("重新获取节点失败", zap.Error(err), zap.Bool("force_reassign", forceReassign))
				return err
			}

			// 重新连接到新节点
			if err := c.connectToNode(node); err != nil {
				c.logger.Error("连接到新节点失败", zap.Error(err))
				return err
			}

			c.currentNode = node
			c.logger.Info("成功重新获取并连接到节点",
				zap.String("new_node_id", node.NodeId),
				zap.String("new_node_address", node.Address),
				zap.Bool("force_reassign", forceReassign))
		}

		return err
	}

	// 第一条数据发送成功，重置失败相关状态
	c.cache = c.cache[1:]    // 移除已发送的第一条数据
	c.failureCount = 0       // 重置失败计数
	c.firstFailureTime = nil // 清除第一次失败时间

	c.logger.Debug("发送成功，重置失败状态")

	if len(c.cache) > 0 {
		if err := c.sendMetricPoints(c.cache); err != nil {
			c.logger.Warn("发送剩余缓存数据失败", zap.Error(err))
			return err
		}
		// 全部发送成功，清空缓存
		c.cache = c.cache[:0]
	}

	return nil
}

// sendMetricPoints 发送指标数据点到服务器
func (c *MonitoringStorageClient) sendMetricPoints(points []models.MetricPoint) error {
	client, err := c.getNodeClient()
	if err != nil {
		return fmt.Errorf("获取节点客户端失败: %w", err)
	}

	protoPoints, err := c.convertMetricPointsToProto(points)
	if err != nil {
		return fmt.Errorf("转换指标点失败: %w", err)
	}

	req := &monitoringv1.StoreMetricPointsRequest{
		Points: protoPoints,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.StoreMetricPoints(ctx, req)
	if err != nil {
		return fmt.Errorf("调用StoreMetricPoints失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("存储指标数据失败: %s", resp.Message)
	}

	c.logger.Debug("成功发送指标数据到指定节点",
		zap.Int("point_count", len(points)),
		zap.String("node_id", c.currentNode.NodeId),
		zap.String("node_address", c.currentNode.Address))

	return nil
}

// addToCache 将数据添加到缓存队列
func (c *MonitoringStorageClient) addToCache(points []models.MetricPoint) {
	if len(points) == 0 {
		return
	}

	// 计算需要为新数据腾出的空间
	currentSize := len(c.cache)
	newPointsCount := len(points)
	totalAfterAdd := currentSize + newPointsCount

	// 如果添加后超过限制，需要删除最早的数据
	if totalAfterAdd > maxCacheSize {
		toRemove := totalAfterAdd - maxCacheSize
		if toRemove >= currentSize {
			// 新数据量过大，清空现有缓存
			c.cache = c.cache[:0]
			// 只保留最新的maxCacheSize条新数据
			if newPointsCount > maxCacheSize {
				points = points[newPointsCount-maxCacheSize:]
			}
		} else {
			// 删除最早的toRemove条数据
			c.cache = c.cache[toRemove:]
		}
	}

	c.cache = append(c.cache, points...)

	c.logger.Debug("数据已添加到缓存队列",
		zap.Int("added_count", len(points)),
		zap.Int("cache_size", len(c.cache)),
		zap.Int("max_cache_size", maxCacheSize))
}

// QueryMetricPoints 查询指标数据点（查询操作仍然使用管理器的客户端）
func (c *MonitoringStorageClient) QueryMetricPoints(query storage.MetricQuery) ([]models.MetricPoint, error) {
	// 查询操作可以使用任意节点，使用管理器的客户端即可
	client := c.manager.GetMonitoringClient()

	req := c.convertMetricQueryToProto(query)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.QueryMetricPoints(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用QueryMetricPoints失败: %w", err)
	}

	points, err := c.convertProtoToMetricPoints(resp.Points)
	if err != nil {
		return nil, fmt.Errorf("转换查询结果失败: %w", err)
	}

	return points, nil
}

// QueryTimeSeries 查询时间序列数据（查询操作仍然使用管理器的客户端）
func (c *MonitoringStorageClient) QueryTimeSeries(query storage.MetricQuery) (*models.QueryResult, error) {
	// 查询操作可以使用任意节点，使用管理器的客户端即可
	client := c.manager.GetMonitoringClient()

	req := c.convertTimeSeriesQueryToProto(query)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.QueryTimeSeries(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用QueryTimeSeries失败: %w", err)
	}

	result, err := c.convertProtoToQueryResult(resp.Series)
	if err != nil {
		return nil, fmt.Errorf("转换时间序列结果失败: %w", err)
	}

	return result, nil
}

// Close 关闭客户端，清理资源
func (c *MonitoringStorageClient) Close() error {
	c.nodeMutex.Lock()
	defer c.nodeMutex.Unlock()

	if c.nodeConnection != nil {
		err := c.nodeConnection.Close()
		c.nodeConnection = nil
		c.nodeClient = nil
		c.logger.Info("关闭存储节点连接",
			zap.String("node_id", c.currentNode.GetNodeId()))
		return err
	}

	return nil
}

// GetCacheStatus 获取缓存状态（用于监控和调试）
func (c *MonitoringStorageClient) GetCacheStatus() (cacheSize int, failureCount int) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	return len(c.cache), c.failureCount
}

// GetFailureStatus 获取失败状态信息（用于监控和调试）
func (c *MonitoringStorageClient) GetFailureStatus() (failureCount int, firstFailureTime *time.Time, failureDuration time.Duration) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	var duration time.Duration
	if c.firstFailureTime != nil {
		duration = time.Since(*c.firstFailureTime)
	}

	return c.failureCount, c.firstFailureTime, duration
}

// GetCurrentNode 获取当前分配的存储节点信息
func (c *MonitoringStorageClient) GetCurrentNode() *monitoringv1.StorageNode {
	c.nodeMutex.RLock()
	defer c.nodeMutex.RUnlock()
	return c.currentNode
}

// convertMetricPointsToProto 转换内部指标点为proto模型
func (c *MonitoringStorageClient) convertMetricPointsToProto(points []models.MetricPoint) ([]*monitoringv1.MetricPoint, error) {
	protoPoints := make([]*monitoringv1.MetricPoint, 0, len(points))

	for _, point := range points {
		proto := &monitoringv1.MetricPoint{
			Timestamp:   point.Timestamp.UnixNano(),
			MetricName:  point.MetricName,
			MetricType:  c.convertMetricTypeToProto(point.MetricType),
			Value:       point.Value,
			Source:      point.Source,
			Instance:    point.Instance,
			Category:    c.convertMetricCategoryToProto(point.Category),
			Labels:      point.Labels,
			Unit:        point.Unit,
			Description: point.Description,
		}

		protoPoints = append(protoPoints, proto)
	}

	return protoPoints, nil
}

// convertProtoToMetricPoints 转换proto指标点为内部模型
func (c *MonitoringStorageClient) convertProtoToMetricPoints(protoPoints []*monitoringv1.MetricPoint) ([]models.MetricPoint, error) {
	points := make([]models.MetricPoint, 0, len(protoPoints))

	for _, protoPoint := range protoPoints {
		if protoPoint == nil {
			continue
		}

		timestamp := time.Now()
		if protoPoint.Timestamp != 0 {
			timestamp = time.Unix(0, protoPoint.Timestamp)
		}

		point := models.MetricPoint{
			Timestamp:   timestamp,
			MetricName:  protoPoint.GetMetricName(),
			MetricType:  c.convertProtoMetricType(protoPoint.GetMetricType()),
			Value:       protoPoint.GetValue(),
			Source:      protoPoint.GetSource(),
			Instance:    protoPoint.GetInstance(),
			Category:    c.convertProtoMetricCategory(protoPoint.GetCategory()),
			Labels:      protoPoint.GetLabels(),
			Unit:        protoPoint.GetUnit(),
			Description: protoPoint.GetDescription(),
		}

		points = append(points, point)
	}

	return points, nil
}

// convertMetricQueryToProto 转换内部查询为proto请求
func (c *MonitoringStorageClient) convertMetricQueryToProto(query storage.MetricQuery) *monitoringv1.QueryMetricPointsRequest {
	req := &monitoringv1.QueryMetricPointsRequest{
		MetricNames:   query.MetricNames,
		Sources:       query.Sources,
		Instances:     query.Instances,
		LabelMatchers: query.LabelMatchers,
		Aggregation:   query.Aggregation,
		Interval:      query.Interval,
		Limit:         int32(query.Limit),
	}

	// 转换时间
	if !query.StartTime.IsZero() {
		req.StartTime = query.StartTime.UnixNano()
	}
	if !query.EndTime.IsZero() {
		req.EndTime = query.EndTime.UnixNano()
	}

	// 转换分类
	for _, category := range query.Categories {
		req.Categories = append(req.Categories, c.convertMetricCategoryToProto(category))
	}

	return req
}

// convertTimeSeriesQueryToProto 转换内部时间序列查询为proto请求
func (c *MonitoringStorageClient) convertTimeSeriesQueryToProto(query storage.MetricQuery) *monitoringv1.QueryTimeSeriesRequest {
	req := &monitoringv1.QueryTimeSeriesRequest{
		MetricNames:   query.MetricNames,
		Sources:       query.Sources,
		Instances:     query.Instances,
		LabelMatchers: query.LabelMatchers,
		Aggregation:   query.Aggregation,
		Interval:      query.Interval,
		Limit:         int32(query.Limit),
	}

	// 转换时间
	if !query.StartTime.IsZero() {
		req.StartTime = query.StartTime.UnixNano()
	}
	if !query.EndTime.IsZero() {
		req.EndTime = query.EndTime.UnixNano()
	}

	// 转换分类
	for _, category := range query.Categories {
		req.Categories = append(req.Categories, c.convertMetricCategoryToProto(category))
	}

	return req
}

// convertProtoToQueryResult 转换proto时间序列为内部结果
func (c *MonitoringStorageClient) convertProtoToQueryResult(protoSeries []*monitoringv1.TimeSeries) (*models.QueryResult, error) {
	result := &models.QueryResult{
		Series: make([]models.TimeSeries, 0, len(protoSeries)),
	}

	for _, protoSerie := range protoSeries {
		if protoSerie == nil {
			continue
		}

		points := make([]models.TimeSeriesPoint, 0, len(protoSerie.Points))
		for _, protoPoint := range protoSerie.Points {
			if protoPoint == nil {
				continue
			}

			timestamp := time.Now()
			if protoPoint.Timestamp != 0 {
				timestamp = time.Unix(0, protoPoint.Timestamp)
			}

			points = append(points, models.TimeSeriesPoint{
				Timestamp: timestamp,
				Value:     protoPoint.Value,
			})
		}

		series := models.TimeSeries{
			Name:   protoSerie.Name,
			Labels: protoSerie.Labels,
			Points: points,
		}

		result.Series = append(result.Series, series)
	}

	return result, nil
}

// convertMetricTypeToProto 转换内部指标类型为proto类型
func (c *MonitoringStorageClient) convertMetricTypeToProto(internalType models.MetricType) monitoringv1.MetricType {
	switch internalType {
	case models.MetricTypeGauge:
		return monitoringv1.MetricType_METRIC_TYPE_GAUGE
	case models.MetricTypeCounter:
		return monitoringv1.MetricType_METRIC_TYPE_COUNTER
	default:
		return monitoringv1.MetricType_METRIC_TYPE_GAUGE
	}
}

// convertProtoMetricType 转换proto指标类型为内部类型
func (c *MonitoringStorageClient) convertProtoMetricType(protoType monitoringv1.MetricType) models.MetricType {
	switch protoType {
	case monitoringv1.MetricType_METRIC_TYPE_GAUGE:
		return models.MetricTypeGauge
	case monitoringv1.MetricType_METRIC_TYPE_COUNTER:
		return models.MetricTypeCounter
	default:
		return models.MetricTypeGauge
	}
}

// convertMetricCategoryToProto 转换内部指标分类为proto分类
func (c *MonitoringStorageClient) convertMetricCategoryToProto(internalCategory models.MetricCategory) monitoringv1.MetricCategory {
	switch internalCategory {
	case models.CategoryServer:
		return monitoringv1.MetricCategory_METRIC_CATEGORY_SERVER
	case models.CategoryApp:
		return monitoringv1.MetricCategory_METRIC_CATEGORY_APP
	case models.CategoryAPI:
		return monitoringv1.MetricCategory_METRIC_CATEGORY_API
	case models.CategoryCustom:
		return monitoringv1.MetricCategory_METRIC_CATEGORY_CUSTOM
	default:
		return monitoringv1.MetricCategory_METRIC_CATEGORY_CUSTOM
	}
}

// convertProtoMetricCategory 转换proto指标分类为内部分类
func (c *MonitoringStorageClient) convertProtoMetricCategory(protoCategory monitoringv1.MetricCategory) models.MetricCategory {
	switch protoCategory {
	case monitoringv1.MetricCategory_METRIC_CATEGORY_SERVER:
		return models.CategoryServer
	case monitoringv1.MetricCategory_METRIC_CATEGORY_APP:
		return models.CategoryApp
	case monitoringv1.MetricCategory_METRIC_CATEGORY_API:
		return models.CategoryAPI
	case monitoringv1.MetricCategory_METRIC_CATEGORY_CUSTOM:
		return models.CategoryCustom
	default:
		return models.CategoryCustom
	}
}
