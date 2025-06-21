// Package storage 实现指标数据的gRPC存储服务
package storage

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/pkg/monitoring/models"
	"time"

	monitoringv1 "github.com/xsxdot/aio/api/proto/monitoring/v1"
	"github.com/xsxdot/aio/internal/grpc"
	"github.com/xsxdot/aio/pkg/registry"

	"go.uber.org/zap"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GrpcStorage 实现基于gRPC的指标存储服务
type GrpcStorage struct {
	monitoringv1.UnimplementedMetricStorageServiceServer
	storage       *Storage
	nodeAllocator *NodeAllocator
	logger        *zap.Logger
}

// 确保 GrpcStorage 实现了 ServiceRegistrar 接口
var _ grpc.ServiceRegistrar = (*GrpcStorage)(nil)

// GrpcStorageConfig gRPC存储服务配置
type GrpcStorageConfig struct {
	Storage  *Storage          // 存储引擎
	Registry registry.Registry // 服务注册中心
	Logger   *zap.Logger       // 日志记录器
}

// NewGrpcStorage 创建新的gRPC存储服务实例
func NewGrpcStorage(config GrpcStorageConfig) *GrpcStorage {
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	// 创建节点分配器
	var nodeAllocator *NodeAllocator
	if config.Registry != nil {
		nodeAllocator = NewNodeAllocator(NodeAllocatorConfig{
			Registry: config.Registry,
			Logger:   logger,
		})
	}

	return &GrpcStorage{
		storage:       config.Storage,
		nodeAllocator: nodeAllocator,
		logger:        logger,
	}
}

// RegisterService 实现 ServiceRegistrar 接口，注册gRPC服务
func (s *GrpcStorage) RegisterService(server *grpcpkg.Server) error {
	monitoringv1.RegisterMetricStorageServiceServer(server, s)
	return nil
}

// ServiceName 返回服务名称
func (s *GrpcStorage) ServiceName() string {
	return "monitoring.v1.MetricStorageService"
}

// ServiceVersion 返回服务版本
func (s *GrpcStorage) ServiceVersion() string {
	return "v1.0.0"
}

// GetStorageNode 获取服务的存储节点分配
func (s *GrpcStorage) GetStorageNode(ctx context.Context, req *monitoringv1.GetStorageNodeRequest) (*monitoringv1.GetStorageNodeResponse, error) {
	s.logger.Debug("收到获取存储节点请求",
		zap.String("service_name", req.GetServiceName()),
		zap.Bool("force_reassign", req.GetForceReassign()))

	// 参数验证
	if req.GetServiceName() == "" {
		return &monitoringv1.GetStorageNodeResponse{
			Success: false,
			Message: "服务名称不能为空",
		}, status.Error(codes.InvalidArgument, "服务名称不能为空")
	}

	// 检查节点分配器是否可用
	if s.nodeAllocator == nil {
		return &monitoringv1.GetStorageNodeResponse{
			Success: false,
			Message: "节点分配器未初始化",
		}, status.Error(codes.Internal, "节点分配器未初始化")
	}

	// 获取节点分配
	allocation, err := s.nodeAllocator.GetStorageNode(ctx, req.GetServiceName(), req.GetForceReassign())
	if err != nil {
		s.logger.Error("获取存储节点分配失败",
			zap.String("service_name", req.GetServiceName()),
			zap.Error(err))

		return &monitoringv1.GetStorageNodeResponse{
			Success: false,
			Message: fmt.Sprintf("获取存储节点失败: %v", err),
		}, nil
	}

	// 转换为proto格式
	node := &monitoringv1.StorageNode{
		NodeId:       allocation.NodeID,
		Address:      allocation.NodeAddress,
		Status:       monitoringv1.StorageNodeStatus_STORAGE_NODE_STATUS_ONLINE,
		ServiceCount: int32(s.getNodeServiceCount(allocation.NodeID)),
	}

	s.logger.Debug("成功获取存储节点分配",
		zap.String("service_name", req.GetServiceName()),
		zap.String("node_id", allocation.NodeID),
		zap.String("node_address", allocation.NodeAddress))

	return &monitoringv1.GetStorageNodeResponse{
		Node:    node,
		Success: true,
		Message: fmt.Sprintf("成功分配存储节点: %s", allocation.NodeAddress),
	}, nil
}

// getNodeServiceCount 获取节点的服务数量
func (s *GrpcStorage) getNodeServiceCount(nodeID string) int {
	if s.nodeAllocator == nil {
		return 0
	}

	nodeStats := s.nodeAllocator.GetNodeStats()
	return nodeStats[nodeID]
}

// StoreMetricPoints 存储指标数据点
func (s *GrpcStorage) StoreMetricPoints(ctx context.Context, req *monitoringv1.StoreMetricPointsRequest) (*monitoringv1.StoreMetricPointsResponse, error) {
	s.logger.Debug("收到存储指标点请求", zap.Int("count", len(req.GetPoints())))

	// 转换proto数据为内部模型
	points, err := s.convertProtoToMetricPoints(req.GetPoints())
	if err != nil {
		s.logger.Error("转换指标点失败", zap.Error(err))
		return &monitoringv1.StoreMetricPointsResponse{
			Success: false,
			Message: fmt.Sprintf("转换指标点失败: %v", err),
		}, nil
	}

	// 调用底层存储
	if err := s.storage.StoreMetricPoints(points); err != nil {
		s.logger.Error("存储指标点失败", zap.Error(err))
		return &monitoringv1.StoreMetricPointsResponse{
			Success: false,
			Message: fmt.Sprintf("存储失败: %v", err),
		}, nil
	}

	s.logger.Debug("成功存储指标点", zap.Int("count", len(points)))
	return &monitoringv1.StoreMetricPointsResponse{
		Success: true,
		Message: fmt.Sprintf("成功存储 %d 个指标点", len(points)),
	}, nil
}

// StoreMetricProvider 存储实现了MetricProvider接口的数据
func (s *GrpcStorage) StoreMetricProvider(ctx context.Context, req *monitoringv1.StoreMetricProviderRequest) (*monitoringv1.StoreMetricProviderResponse, error) {
	s.logger.Debug("收到存储MetricProvider请求", zap.Int("count", len(req.GetPoints())))

	// 转换proto数据为内部模型
	points, err := s.convertProtoToMetricPoints(req.GetPoints())
	if err != nil {
		s.logger.Error("转换MetricProvider数据失败", zap.Error(err))
		return &monitoringv1.StoreMetricProviderResponse{
			Success: false,
			Message: fmt.Sprintf("转换数据失败: %v", err),
		}, nil
	}

	// 调用底层存储
	if err := s.storage.StoreMetricPoints(points); err != nil {
		s.logger.Error("存储MetricProvider数据失败", zap.Error(err))
		return &monitoringv1.StoreMetricProviderResponse{
			Success: false,
			Message: fmt.Sprintf("存储失败: %v", err),
		}, nil
	}

	s.logger.Debug("成功存储MetricProvider数据", zap.Int("count", len(points)))
	return &monitoringv1.StoreMetricProviderResponse{
		Success: true,
		Message: fmt.Sprintf("成功存储 %d 个指标点", len(points)),
	}, nil
}

// QueryMetricPoints 查询指标数据点
func (s *GrpcStorage) QueryMetricPoints(ctx context.Context, req *monitoringv1.QueryMetricPointsRequest) (*monitoringv1.QueryMetricPointsResponse, error) {
	s.logger.Debug("收到查询指标点请求")

	// 转换proto查询参数为内部查询结构
	query := s.convertProtoToMetricQuery(req)

	// 调用底层存储查询
	points, err := s.storage.QueryMetricPoints(query)
	if err != nil {
		s.logger.Error("查询指标点失败", zap.Error(err))
		return nil, fmt.Errorf("查询失败: %v", err)
	}

	// 转换内部模型为proto数据
	protoPoints, err := s.convertMetricPointsToProto(points)
	if err != nil {
		s.logger.Error("转换查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("转换结果失败: %v", err)
	}

	s.logger.Debug("成功查询指标点", zap.Int("count", len(protoPoints)))
	return &monitoringv1.QueryMetricPointsResponse{
		Points: protoPoints,
	}, nil
}

// QueryTimeSeries 查询时间序列数据
func (s *GrpcStorage) QueryTimeSeries(ctx context.Context, req *monitoringv1.QueryTimeSeriesRequest) (*monitoringv1.QueryTimeSeriesResponse, error) {
	s.logger.Debug("收到查询时间序列请求")

	// 转换proto查询参数为内部查询结构
	query := s.convertProtoToTimeSeriesQuery(req)

	// 调用底层存储查询
	result, err := s.storage.QueryTimeSeries(query)
	if err != nil {
		s.logger.Error("查询时间序列失败", zap.Error(err))
		return nil, fmt.Errorf("查询失败: %v", err)
	}

	// 转换内部模型为proto数据
	protoSeries, err := s.convertTimeSeriesResultToProto(result)
	if err != nil {
		s.logger.Error("转换时间序列结果失败", zap.Error(err))
		return nil, fmt.Errorf("转换结果失败: %v", err)
	}

	s.logger.Debug("成功查询时间序列", zap.Int("count", len(protoSeries)))
	return &monitoringv1.QueryTimeSeriesResponse{
		Series: protoSeries,
	}, nil
}

// convertProtoToMetricPoints 转换proto指标点为内部模型
func (s *GrpcStorage) convertProtoToMetricPoints(protoPoints []*monitoringv1.MetricPoint) ([]models.MetricPoint, error) {
	points := make([]models.MetricPoint, 0, len(protoPoints))

	for _, proto := range protoPoints {
		if proto == nil {
			continue
		}

		timestamp := time.Now()
		if proto.Timestamp != 0 {
			timestamp = time.Unix(0, proto.Timestamp)
		}

		point := models.MetricPoint{
			Timestamp:   timestamp,
			MetricName:  proto.GetMetricName(),
			MetricType:  s.convertProtoMetricType(proto.GetMetricType()),
			Value:       proto.GetValue(),
			Source:      proto.GetSource(),
			Instance:    proto.GetInstance(),
			Category:    s.convertProtoMetricCategory(proto.GetCategory()),
			Labels:      proto.GetLabels(),
			Unit:        proto.GetUnit(),
			Description: proto.GetDescription(),
		}

		points = append(points, point)
	}

	return points, nil
}

// convertMetricPointsToProto 转换内部指标点为proto模型
func (s *GrpcStorage) convertMetricPointsToProto(points []models.MetricPoint) ([]*monitoringv1.MetricPoint, error) {
	protoPoints := make([]*monitoringv1.MetricPoint, 0, len(points))

	for _, point := range points {
		proto := &monitoringv1.MetricPoint{
			Timestamp:   point.Timestamp.UnixNano(),
			MetricName:  point.MetricName,
			MetricType:  s.convertMetricTypeToProto(point.MetricType),
			Value:       point.Value,
			Source:      point.Source,
			Instance:    point.Instance,
			Category:    s.convertMetricCategoryToProto(point.Category),
			Labels:      point.Labels,
			Unit:        point.Unit,
			Description: point.Description,
		}

		protoPoints = append(protoPoints, proto)
	}

	return protoPoints, nil
}

// convertProtoToMetricQuery 转换proto查询为内部查询结构
func (s *GrpcStorage) convertProtoToMetricQuery(req *monitoringv1.QueryMetricPointsRequest) MetricQuery {
	query := MetricQuery{
		MetricNames:   req.GetMetricNames(),
		Sources:       req.GetSources(),
		Instances:     req.GetInstances(),
		LabelMatchers: req.GetLabelMatchers(),
		Aggregation:   req.GetAggregation(),
		Interval:      req.GetInterval(),
		Limit:         int(req.GetLimit()),
	}

	// 转换时间
	if req.GetStartTime() != 0 {
		query.StartTime = time.Unix(0, req.GetStartTime())
	}
	if req.GetEndTime() != 0 {
		query.EndTime = time.Unix(0, req.GetEndTime())
	}

	// 转换分类
	for _, protoCategory := range req.GetCategories() {
		query.Categories = append(query.Categories, s.convertProtoMetricCategory(protoCategory))
	}

	return query
}

// convertProtoToTimeSeriesQuery 转换proto时间序列查询为内部查询结构
func (s *GrpcStorage) convertProtoToTimeSeriesQuery(req *monitoringv1.QueryTimeSeriesRequest) MetricQuery {
	query := MetricQuery{
		MetricNames:   req.GetMetricNames(),
		Sources:       req.GetSources(),
		Instances:     req.GetInstances(),
		LabelMatchers: req.GetLabelMatchers(),
		Aggregation:   req.GetAggregation(),
		Interval:      req.GetInterval(),
		Limit:         int(req.GetLimit()),
	}

	// 转换时间
	if req.GetStartTime() != 0 {
		query.StartTime = time.Unix(0, req.GetStartTime())
	}
	if req.GetEndTime() != 0 {
		query.EndTime = time.Unix(0, req.GetEndTime())
	}

	// 转换分类
	for _, protoCategory := range req.GetCategories() {
		query.Categories = append(query.Categories, s.convertProtoMetricCategory(protoCategory))
	}

	return query
}

// convertTimeSeriesResultToProto 转换时间序列结果为proto格式
func (s *GrpcStorage) convertTimeSeriesResultToProto(result *models.QueryResult) ([]*monitoringv1.TimeSeries, error) {
	protoSeries := make([]*monitoringv1.TimeSeries, 0, len(result.Series))

	for _, series := range result.Series {
		protoPoints := make([]*monitoringv1.TimeSeriesPoint, 0, len(series.Points))
		for _, point := range series.Points {
			protoPoints = append(protoPoints, &monitoringv1.TimeSeriesPoint{
				Timestamp: point.Timestamp.UnixNano(),
				Value:     point.Value,
			})
		}

		protoSeries = append(protoSeries, &monitoringv1.TimeSeries{
			Name:   series.Name,
			Labels: series.Labels,
			Points: protoPoints,
		})
	}

	return protoSeries, nil
}

// convertProtoMetricType 转换proto指标类型为内部类型
func (s *GrpcStorage) convertProtoMetricType(protoType monitoringv1.MetricType) models.MetricType {
	switch protoType {
	case monitoringv1.MetricType_METRIC_TYPE_GAUGE:
		return models.MetricTypeGauge
	case monitoringv1.MetricType_METRIC_TYPE_COUNTER:
		return models.MetricTypeCounter
	default:
		return models.MetricTypeGauge
	}
}

// convertMetricTypeToProto 转换内部指标类型为proto类型
func (s *GrpcStorage) convertMetricTypeToProto(internalType models.MetricType) monitoringv1.MetricType {
	switch internalType {
	case models.MetricTypeGauge:
		return monitoringv1.MetricType_METRIC_TYPE_GAUGE
	case models.MetricTypeCounter:
		return monitoringv1.MetricType_METRIC_TYPE_COUNTER
	default:
		return monitoringv1.MetricType_METRIC_TYPE_GAUGE
	}
}

// convertProtoMetricCategory 转换proto指标分类为内部分类
func (s *GrpcStorage) convertProtoMetricCategory(protoCategory monitoringv1.MetricCategory) models.MetricCategory {
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

// convertMetricCategoryToProto 转换内部指标分类为proto分类
func (s *GrpcStorage) convertMetricCategoryToProto(internalCategory models.MetricCategory) monitoringv1.MetricCategory {
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
