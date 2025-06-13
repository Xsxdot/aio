package config

import (
	"context"
	"fmt"
	"strconv"

	configv1 "github.com/xsxdot/aio/api/proto/config/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCService 配置服务的gRPC实现
type GRPCService struct {
	configv1.UnimplementedConfigServiceServer
	service *Service
	logger  *zap.Logger
}

// NewGRPCService 创建新的配置gRPC服务
func NewGRPCService(service *Service, logger *zap.Logger) *GRPCService {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &GRPCService{
		service: service,
		logger:  logger,
	}
}

// RegisterService 实现ServiceRegistrar接口
func (s *GRPCService) RegisterService(server *grpc.Server) error {
	configv1.RegisterConfigServiceServer(server, s)
	return nil
}

// ServiceName 返回服务名称
func (s *GRPCService) ServiceName() string {
	return "config.v1.ConfigService"
}

// ServiceVersion 返回服务版本
func (s *GRPCService) ServiceVersion() string {
	return "v1.0.0"
}

// GetConfig 获取配置
func (s *GRPCService) GetConfig(ctx context.Context, req *configv1.GetConfigRequest) (*configv1.GetConfigResponse, error) {
	s.logger.Debug("收到获取配置请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	config, err := s.service.Get(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "获取配置失败: %v", err)
	}

	protoConfig, err := s.toProtoConfig(config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "转换配置失败: %v", err)
	}

	return &configv1.GetConfigResponse{
		Config: protoConfig,
	}, nil
}

// GetConfigJSON 获取JSON格式配置
func (s *GRPCService) GetConfigJSON(ctx context.Context, req *configv1.GetConfigJSONRequest) (*configv1.GetConfigJSONResponse, error) {
	s.logger.Debug("收到获取JSON格式配置请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	jsonData, err := s.service.ExportConfigAsJSON(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "获取配置失败: %v", err)
	}

	return &configv1.GetConfigJSONResponse{
		JsonData: jsonData,
	}, nil
}

// SetConfig 设置配置
func (s *GRPCService) SetConfig(ctx context.Context, req *configv1.SetConfigRequest) (*configv1.SetConfigResponse, error) {
	s.logger.Debug("收到设置配置请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	if len(req.Value) == 0 {
		return nil, status.Error(codes.InvalidArgument, "配置值不能为空")
	}

	// 转换proto配置值为内部格式
	configValues, err := s.fromProtoConfigValues(req.Value)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "转换配置值失败: %v", err)
	}

	err = s.service.Set(ctx, req.Key, configValues, req.Metadata)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "设置配置失败: %v", err)
	}

	// 获取新设置的配置
	config, _ := s.service.Get(ctx, req.Key)
	protoConfig, _ := s.toProtoConfig(config)

	return &configv1.SetConfigResponse{
		Config: protoConfig,
	}, nil
}

// DeleteConfig 删除配置
func (s *GRPCService) DeleteConfig(ctx context.Context, req *configv1.DeleteConfigRequest) (*configv1.DeleteConfigResponse, error) {
	s.logger.Debug("收到删除配置请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	// 检查配置是否存在
	_, err := s.service.Get(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "配置不存在: %v", err)
	}

	err = s.service.Delete(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "删除配置失败: %v", err)
	}

	return &configv1.DeleteConfigResponse{
		Success: true,
	}, nil
}

// ListConfigs 列出所有配置
func (s *GRPCService) ListConfigs(ctx context.Context, req *configv1.ListConfigsRequest) (*configv1.ListConfigsResponse, error) {
	s.logger.Debug("收到列出配置请求")

	configs, err := s.service.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取配置列表失败: %v", err)
	}

	var protoConfigs []*configv1.Config
	for _, config := range configs {
		protoConfig, err := s.toProtoConfig(config)
		if err != nil {
			s.logger.Warn("转换配置失败", zap.String("key", config.Key), zap.Error(err))
			continue
		}
		protoConfigs = append(protoConfigs, protoConfig)
	}

	return &configv1.ListConfigsResponse{
		Configs: protoConfigs,
	}, nil
}

// GetEnvConfig 获取环境配置
func (s *GRPCService) GetEnvConfig(ctx context.Context, req *configv1.GetEnvConfigRequest) (*configv1.GetEnvConfigResponse, error) {
	s.logger.Debug("收到获取环境配置请求", zap.String("key", req.Key), zap.String("env", req.Env))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	if req.Env == "" {
		return nil, status.Error(codes.InvalidArgument, "环境参数不能为空")
	}

	fallbacks := req.Fallbacks
	if len(fallbacks) == 0 {
		fallbacks = DefaultEnvironmentFallbacks(req.Env)
	}

	envConfig := NewEnvironmentConfig(req.Env, fallbacks...)
	config, err := s.service.GetForEnvironment(ctx, req.Key, envConfig)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "获取环境配置失败: %v", err)
	}

	protoConfig, err := s.toProtoConfig(config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "转换配置失败: %v", err)
	}

	return &configv1.GetEnvConfigResponse{
		Config: protoConfig,
	}, nil
}

// SetEnvConfig 设置环境配置
func (s *GRPCService) SetEnvConfig(ctx context.Context, req *configv1.SetEnvConfigRequest) (*configv1.SetEnvConfigResponse, error) {
	s.logger.Debug("收到设置环境配置请求", zap.String("key", req.Key), zap.String("env", req.Env))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	if req.Env == "" {
		return nil, status.Error(codes.InvalidArgument, "环境参数不能为空")
	}

	if len(req.Value) == 0 {
		return nil, status.Error(codes.InvalidArgument, "配置值不能为空")
	}

	// 转换proto配置值为内部格式
	configValues, err := s.fromProtoConfigValues(req.Value)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "转换配置值失败: %v", err)
	}

	err = s.service.SetForEnvironment(ctx, req.Key, req.Env, configValues, req.Metadata)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "设置环境配置失败: %v", err)
	}

	envConfig := NewEnvironmentConfig(req.Env)
	config, _ := s.service.GetForEnvironment(ctx, req.Key, envConfig)
	protoConfig, _ := s.toProtoConfig(config)

	return &configv1.SetEnvConfigResponse{
		Config: protoConfig,
	}, nil
}

// ListEnvConfig 列出环境配置
func (s *GRPCService) ListEnvConfig(ctx context.Context, req *configv1.ListEnvConfigRequest) (*configv1.ListEnvConfigResponse, error) {
	s.logger.Debug("收到列出环境配置请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	envConfigs, err := s.service.ListEnvironmentConfigs(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取环境配置列表失败: %v", err)
	}

	// 提取环境名称列表
	environments := make([]string, 0, len(envConfigs))
	for env := range envConfigs {
		environments = append(environments, env)
	}

	return &configv1.ListEnvConfigResponse{
		Environments: environments,
	}, nil
}

// GetEnvConfigJSON 获取环境JSON格式配置
func (s *GRPCService) GetEnvConfigJSON(ctx context.Context, req *configv1.GetEnvConfigJSONRequest) (*configv1.GetEnvConfigJSONResponse, error) {
	s.logger.Debug("收到获取环境JSON格式配置请求", zap.String("key", req.Key), zap.String("env", req.Env))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	if req.Env == "" {
		return nil, status.Error(codes.InvalidArgument, "环境参数不能为空")
	}

	// 构建环境配置
	fallbacks := req.Fallbacks
	if len(fallbacks) == 0 {
		fallbacks = DefaultEnvironmentFallbacks(req.Env)
	}
	envConfig := NewEnvironmentConfig(req.Env, fallbacks...)

	jsonData, err := s.service.ExportConfigAsJSONForEnvironment(ctx, req.Key, envConfig)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "获取环境配置失败: %v", err)
	}

	return &configv1.GetEnvConfigJSONResponse{
		JsonData: jsonData,
	}, nil
}

// GetHistory 获取配置历史
func (s *GRPCService) GetHistory(ctx context.Context, req *configv1.GetHistoryRequest) (*configv1.GetHistoryResponse, error) {
	s.logger.Debug("收到获取配置历史请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10 // 默认获取10条历史记录
	}

	history, err := s.service.GetHistory(ctx, req.Key, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取配置历史失败: %v", err)
	}

	var protoHistory []*configv1.ConfigHistory
	for _, h := range history {
		protoH, err := s.toProtoConfigHistory(h.ConfigItem)
		if err != nil {
			s.logger.Warn("转换历史记录失败", zap.Error(err))
			continue
		}
		// 设置历史记录的修订版本和时间戳
		protoH.Revision = h.ModRevision
		protoH.Timestamp = h.CreateTime.Unix()
		protoH.Operation = "update" // 默认操作类型
		protoHistory = append(protoHistory, protoH)
	}

	return &configv1.GetHistoryResponse{
		History: protoHistory,
	}, nil
}

// GetRevision 获取特定版本配置
func (s *GRPCService) GetRevision(ctx context.Context, req *configv1.GetRevisionRequest) (*configv1.GetRevisionResponse, error) {
	s.logger.Debug("收到获取特定版本配置请求", zap.String("key", req.Key), zap.Int64("revision", req.Revision))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	if req.Revision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "修订版本号必须大于0")
	}

	historyItem, err := s.service.GetByRevision(ctx, req.Key, req.Revision)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "获取配置修订版本失败: %v", err)
	}

	protoConfig, err := s.toProtoConfig(historyItem.ConfigItem)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "转换配置失败: %v", err)
	}

	return &configv1.GetRevisionResponse{
		Config: protoConfig,
	}, nil
}

// GetComposite 获取组合配置
func (s *GRPCService) GetComposite(ctx context.Context, req *configv1.GetCompositeRequest) (*configv1.GetCompositeResponse, error) {
	s.logger.Debug("收到获取组合配置请求", zap.String("key", req.Key))

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "配置键不能为空")
	}

	var config interface{}
	var err error

	if req.Env == "" {
		// 获取普通组合配置
		config, err = s.service.GetCompositeConfig(ctx, req.Key)
	} else {
		// 获取特定环境的组合配置
		fallbacks := req.Fallbacks
		if len(fallbacks) == 0 {
			fallbacks = DefaultEnvironmentFallbacks(req.Env)
		}

		envConfig := NewEnvironmentConfig(req.Env, fallbacks...)
		config, err = s.service.GetCompositeConfigForEnvironment(ctx, req.Key, envConfig)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "获取组合配置失败: %v", err)
	}

	// 将组合配置转换为string map
	composite := make(map[string]string)
	if configMap, ok := config.(map[string]interface{}); ok {
		for k, v := range configMap {
			if str, ok := v.(string); ok {
				composite[k] = str
			}
		}
	}

	return &configv1.GetCompositeResponse{
		Composite: composite,
	}, nil
}

// MergeComposite 合并组合配置
func (s *GRPCService) MergeComposite(ctx context.Context, req *configv1.MergeCompositeRequest) (*configv1.MergeCompositeResponse, error) {
	s.logger.Debug("收到合并组合配置请求", zap.Strings("keys", req.Keys))

	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "至少需要一个配置键")
	}

	var config interface{}
	var err error

	if req.Env == "" {
		// 合并普通组合配置
		config, err = s.service.MergeCompositeConfigs(ctx, req.Keys)
	} else {
		// 合并特定环境的组合配置
		fallbacks := req.Fallbacks
		if len(fallbacks) == 0 {
			fallbacks = DefaultEnvironmentFallbacks(req.Env)
		}

		envConfig := NewEnvironmentConfig(req.Env, fallbacks...)
		config, err = s.service.MergeCompositeConfigsForEnvironment(ctx, req.Keys, envConfig)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "合并组合配置失败: %v", err)
	}

	// 将组合配置转换为string map
	composite := make(map[string]string)
	if configMap, ok := config.(map[string]interface{}); ok {
		for k, v := range configMap {
			if str, ok := v.(string); ok {
				composite[k] = str
			}
		}
	}

	return &configv1.MergeCompositeResponse{
		Composite: composite,
	}, nil
}

// toProtoConfig 将内部配置转换为proto配置
func (s *GRPCService) toProtoConfig(config *ConfigItem) (*configv1.Config, error) {
	protoValues := make(map[string]*configv1.ConfigValue)
	for k, v := range config.Value {
		protoValue, err := s.toProtoConfigValue(v)
		if err != nil {
			return nil, err
		}
		protoValues[k] = protoValue
	}

	return &configv1.Config{
		Key:        config.Key,
		Value:      protoValues,
		Metadata:   config.Metadata,
		Revision:   config.Version,
		CreateTime: config.UpdatedAt.Unix(),
		UpdateTime: config.UpdatedAt.Unix(),
	}, nil
}

// toProtoConfigValue 将内部配置值转换为proto配置值
func (s *GRPCService) toProtoConfigValue(value *ConfigValue) (*configv1.ConfigValue, error) {
	protoValue := &configv1.ConfigValue{
		Type: string(value.Type),
	}

	// 根据类型将string值转换为对应的proto字段
	switch value.Type {
	case ValueTypeString:
		protoValue.StringValue = value.Value

	case ValueTypeInt:
		// 将string转换为int64
		if intVal, err := strconv.ParseInt(value.Value, 10, 64); err == nil {
			protoValue.IntValue = intVal
		} else {
			return nil, fmt.Errorf("无法将值 '%s' 转换为整数: %v", value.Value, err)
		}

	case ValueTypeFloat:
		// 将string转换为float64
		if floatVal, err := strconv.ParseFloat(value.Value, 64); err == nil {
			protoValue.FloatValue = floatVal
		} else {
			return nil, fmt.Errorf("无法将值 '%s' 转换为浮点数: %v", value.Value, err)
		}

	case ValueTypeBool:
		// 将string转换为bool
		if boolVal, err := strconv.ParseBool(value.Value); err == nil {
			protoValue.BoolValue = boolVal
		} else {
			return nil, fmt.Errorf("无法将值 '%s' 转换为布尔值: %v", value.Value, err)
		}

	default:
		// 其他类型统一作为字符串处理
		protoValue.StringValue = value.Value
	}

	return protoValue, nil
}

// fromProtoConfigValues 将proto配置值转换为内部格式
func (s *GRPCService) fromProtoConfigValues(protoValues map[string]*configv1.ConfigValue) (map[string]*ConfigValue, error) {
	configValues := make(map[string]*ConfigValue)
	for k, v := range protoValues {
		value, err := s.fromProtoConfigValue(v)
		if err != nil {
			return nil, err
		}
		configValues[k] = value
	}
	return configValues, nil
}

// fromProtoConfigValue 将proto配置值转换为内部格式
func (s *GRPCService) fromProtoConfigValue(protoValue *configv1.ConfigValue) (*ConfigValue, error) {
	value := &ConfigValue{
		Type: ValueType(protoValue.Type),
	}

	// 根据类型从proto字段转换回string值
	switch ValueType(protoValue.Type) {
	case ValueTypeString:
		value.Value = protoValue.StringValue

	case ValueTypeInt:
		value.Value = strconv.FormatInt(protoValue.IntValue, 10)

	case ValueTypeFloat:
		value.Value = strconv.FormatFloat(protoValue.FloatValue, 'f', -1, 64)

	case ValueTypeBool:
		value.Value = strconv.FormatBool(protoValue.BoolValue)

	default:
		// 其他类型从StringValue获取
		value.Value = protoValue.StringValue
	}

	return value, nil
}

// toProtoConfigHistory 将内部历史记录转换为proto格式
func (s *GRPCService) toProtoConfigHistory(config *ConfigItem) (*configv1.ConfigHistory, error) {
	protoValues := make(map[string]*configv1.ConfigValue)
	for k, v := range config.Value {
		protoValue, err := s.toProtoConfigValue(v)
		if err != nil {
			return nil, err
		}
		protoValues[k] = protoValue
	}

	return &configv1.ConfigHistory{
		Key:       config.Key,
		Value:     protoValues,
		Metadata:  config.Metadata,
		Revision:  config.Version, // ConfigItem的Version对应proto的Revision
		Timestamp: config.UpdatedAt.Unix(),
		Operation: "update", // 默认操作类型
	}, nil
}
