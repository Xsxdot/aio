package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/security"
	"xiaozhizhang/system/config/api/client"
	pb "xiaozhizhang/system/config/api/proto"
	internalapp "xiaozhizhang/system/config/internal/app"
	"xiaozhizhang/system/config/internal/model"
	"xiaozhizhang/system/config/internal/model/dto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConfigService gRPC配置服务实现
type ConfigService struct {
	pb.UnimplementedConfigServiceServer
	client *client.ConfigClient
	app    *internalapp.App
	err    *errorc.ErrorBuilder
	log    *logger.Log
}

// NewConfigService 创建配置服务实例
func NewConfigService(client *client.ConfigClient, app *internalapp.App, log *logger.Log) *ConfigService {
	return &ConfigService{
		client: client,
		app:    app,
		err:    errorc.NewErrorBuilder("ConfigGRPCService"),
		log:    log,
	}
}

// ServiceName 返回服务名称
func (s *ConfigService) ServiceName() string {
	return "config.v1.ConfigService"
}

// ServiceVersion 返回服务版本
func (s *ConfigService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册服务到gRPC服务器
func (s *ConfigService) RegisterService(server *grpc.Server) error {
	pb.RegisterConfigServiceServer(server, s)
	return nil
}

// CreateConfig 创建配置
func (s *ConfigService) CreateConfig(ctx context.Context, req *pb.CreateConfigRequest) (*pb.ConfigResponse, error) {
	// 获取操作者信息
	adminClaims, err := security.GetAdminClaimsByCtx(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "未授权的请求")
	}

	// 转换为内部DTO（value map 的 key 为属性名）
	configValue := make(map[string]*model.ConfigValue)
	for field, val := range req.Value {
		configValue[field] = &model.ConfigValue{
			Value: val.Value,
			Type:  convertProtoValueType(val.Type),
		}
	}

	createReq := &dto.CreateConfigRequest{
		Key:         req.Key,
		Value:       configValue,
		Metadata:    req.Metadata,
		Description: req.Description,
		ChangeNote:  req.ChangeNote,
	}

	// 调用app层创建配置
	if err := s.app.CreateConfig(ctx, createReq, adminClaims.Account, adminClaims.ID); err != nil {
		s.log.WithErr(err).WithField("key", req.Key).Error("创建配置失败")
		return nil, convertToGRPCError(err)
	}

	// 查询创建的配置并返回
	return s.GetConfigForAdmin(ctx, &pb.GetConfigForAdminRequest{Key: req.Key})
}

// UpdateConfig 更新配置
func (s *ConfigService) UpdateConfig(ctx context.Context, req *pb.UpdateConfigRequest) (*pb.ConfigResponse, error) {
	// 获取操作者信息
	adminClaims, err := security.GetAdminClaimsByCtx(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "未授权的请求")
	}

	// 先查询配置ID
	configItem, err := s.app.ConfigItemService.FindByKey(ctx, req.Key)
	if err != nil {
		s.log.WithErr(err).WithField("key", req.Key).Error("查询配置失败")
		return nil, convertToGRPCError(err)
	}

	// 转换为内部DTO（value map 的 key 为属性名）
	configValue := make(map[string]*model.ConfigValue)
	for field, val := range req.Value {
		configValue[field] = &model.ConfigValue{
			Value: val.Value,
			Type:  convertProtoValueType(val.Type),
		}
	}

	updateReq := &dto.UpdateConfigRequest{
		Value:       configValue,
		Metadata:    req.Metadata,
		Description: req.Description,
		ChangeNote:  req.ChangeNote,
	}

	// 调用app层更新配置
	if err := s.app.UpdateConfig(ctx, configItem.ID, updateReq, adminClaims.Account, adminClaims.ID); err != nil {
		s.log.WithErr(err).WithField("key", req.Key).Error("更新配置失败")
		return nil, convertToGRPCError(err)
	}

	// 查询更新后的配置并返回
	return s.GetConfigForAdmin(ctx, &pb.GetConfigForAdminRequest{Key: req.Key})
}

// DeleteConfig 删除配置
func (s *ConfigService) DeleteConfig(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	// 获取操作者信息
	_, err := security.GetAdminClaimsByCtx(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "未授权的请求")
	}

	// 先查询配置ID
	configItem, err := s.app.ConfigItemService.FindByKey(ctx, req.Key)
	if err != nil {
		s.log.WithErr(err).WithField("key", req.Key).Error("查询配置失败")
		return nil, convertToGRPCError(err)
	}

	// 调用app层删除配置
	if err := s.app.DeleteConfig(ctx, configItem.ID); err != nil {
		s.log.WithErr(err).WithField("key", req.Key).Error("删除配置失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.DeleteConfigResponse{
		Success: true,
		Message: "配置删除成功",
	}, nil
}

// GetConfigForAdmin 获取配置（管理端）
func (s *ConfigService) GetConfigForAdmin(ctx context.Context, req *pb.GetConfigForAdminRequest) (*pb.ConfigResponse, error) {
	// 获取操作者信息
	_, err := security.GetAdminClaimsByCtx(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "未授权的请求")
	}

	// 查询配置
	configItem, err := s.app.ConfigItemService.FindByKey(ctx, req.Key)
	if err != nil {
		s.log.WithErr(err).WithField("key", req.Key).Error("查询配置失败")
		return nil, convertToGRPCError(err)
	}

	// 转换为proto响应
	return convertToProtoConfigResponse(configItem)
}

// ListConfigsForAdmin 列表查询（管理端）
func (s *ConfigService) ListConfigsForAdmin(ctx context.Context, req *pb.ListConfigsForAdminRequest) (*pb.ListConfigsForAdminResponse, error) {
	// 获取操作者信息
	_, err := security.GetAdminClaimsByCtx(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "未授权的请求")
	}

	// 构建查询请求
	queryReq := &dto.QueryConfigRequest{
		Key:     req.Key,
		PageNum: int(req.PageNum),
		Size:    int(req.Size),
	}

	// 查询配置列表
	configs, total, err := s.app.QueryConfigs(ctx, queryReq)
	if err != nil {
		s.log.WithErr(err).Error("查询配置列表失败")
		return nil, convertToGRPCError(err)
	}

	// 转换为proto响应
	content := make([]*pb.ConfigResponse, 0, len(configs))
	for _, config := range configs {
		resp, err := convertToProtoConfigResponse(config)
		if err != nil {
			s.log.WithErr(err).WithField("key", config.Key).Warn("转换配置响应失败，跳过")
			continue
		}
		content = append(content, resp)
	}

	return &pb.ListConfigsForAdminResponse{
		Content: content,
		Total:   total,
	}, nil
}

// UpdateConfigStatus 更新配置状态
func (s *ConfigService) UpdateConfigStatus(ctx context.Context, req *pb.UpdateConfigStatusRequest) (*pb.ConfigResponse, error) {
	// 获取操作者信息
	_, err := security.GetAdminClaimsByCtx(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "未授权的请求")
	}

	// 注意：当前模型中没有状态字段，这个接口暂时返回未实现
	return nil, status.Error(codes.Unimplemented, "配置状态更新功能暂未实现")
}

// GetConfig 获取配置（查询端）
func (s *ConfigService) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	// 调用client获取配置
	jsonStr, err := s.client.GetConfigJSON(ctx, req.Key, req.Env)
	if err != nil {
		s.log.WithErr(err).WithField("key", req.Key).WithField("env", req.Env).Error("获取配置失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.GetConfigResponse{
		JsonStr: jsonStr,
	}, nil
}

// BatchGetConfigs 批量获取配置
func (s *ConfigService) BatchGetConfigs(ctx context.Context, req *pb.BatchGetConfigsRequest) (*pb.BatchGetConfigsResponse, error) {
	// 调用client批量获取配置
	configs, err := s.client.GetConfigs(ctx, req.Keys, req.Env)
	if err != nil {
		s.log.WithErr(err).WithField("env", req.Env).Error("批量获取配置失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.BatchGetConfigsResponse{
		Configs: configs,
	}, nil
}

// convertProtoValueType 转换proto的ValueType为内部ValueType
func convertProtoValueType(protoType pb.ValueType) model.ValueType {
	switch protoType {
	case pb.ValueType_VALUE_TYPE_STRING:
		return model.ValueTypeString
	case pb.ValueType_VALUE_TYPE_INT:
		return model.ValueTypeInt
	case pb.ValueType_VALUE_TYPE_FLOAT:
		return model.ValueTypeFloat
	case pb.ValueType_VALUE_TYPE_BOOL:
		return model.ValueTypeBool
	case pb.ValueType_VALUE_TYPE_REF:
		return model.ValueTypeRef
	case pb.ValueType_VALUE_TYPE_OBJECT:
		return model.ValueTypeObject
	case pb.ValueType_VALUE_TYPE_ARRAY:
		return model.ValueTypeArray
	case pb.ValueType_VALUE_TYPE_ENCRYPTED:
		return model.ValueTypeEncrypted
	default:
		return model.ValueTypeString
	}
}

// convertInternalValueType 转换内部ValueType为proto的ValueType
func convertInternalValueType(internalType model.ValueType) pb.ValueType {
	switch internalType {
	case model.ValueTypeString:
		return pb.ValueType_VALUE_TYPE_STRING
	case model.ValueTypeInt:
		return pb.ValueType_VALUE_TYPE_INT
	case model.ValueTypeFloat:
		return pb.ValueType_VALUE_TYPE_FLOAT
	case model.ValueTypeBool:
		return pb.ValueType_VALUE_TYPE_BOOL
	case model.ValueTypeRef:
		return pb.ValueType_VALUE_TYPE_REF
	case model.ValueTypeObject:
		return pb.ValueType_VALUE_TYPE_OBJECT
	case model.ValueTypeArray:
		return pb.ValueType_VALUE_TYPE_ARRAY
	case model.ValueTypeEncrypted:
		return pb.ValueType_VALUE_TYPE_ENCRYPTED
	default:
		return pb.ValueType_VALUE_TYPE_STRING
	}
}

// convertToProtoConfigResponse 转换为proto配置响应
func convertToProtoConfigResponse(config *model.ConfigItemModel) (*pb.ConfigResponse, error) {
	// 解析Value字段
	var configValue map[string]*model.ConfigValue
	if err := json.Unmarshal([]byte(config.Value), &configValue); err != nil {
		return nil, fmt.Errorf("解析配置值失败: %w", err)
	}

	// 转换为proto格式（value map 的 key 为属性名）
	protoValue := make(map[string]*pb.ConfigValue)
	for field, val := range configValue {
		protoValue[field] = &pb.ConfigValue{
			Value: val.Value,
			Type:  convertInternalValueType(val.Type),
		}
	}

	// 解析Metadata字段
	var metadata map[string]string
	if config.Metadata != "" {
		if err := json.Unmarshal([]byte(config.Metadata), &metadata); err != nil {
			return nil, fmt.Errorf("解析元数据失败: %w", err)
		}
	}

	return &pb.ConfigResponse{
		Id:          config.ID,
		Key:         config.Key,
		Value:       protoValue,
		Version:     config.Version,
		Metadata:    metadata,
		Description: config.Description,
		CreatedAt:   config.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   config.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// convertToGRPCError 转换业务错误为gRPC错误
func convertToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// 检查是否是errorc错误
	if errorc.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}

	// 尝试解析为自定义Error类型，检查ErrorCode
	customErr := errorc.ParseError(err)
	if customErr != nil && customErr.ErrorCode != nil {
		switch customErr.ErrorCode {
		case errorc.ErrorCodeValid:
			return status.Error(codes.InvalidArgument, err.Error())
		case errorc.ErrorCodeNoAuth:
			return status.Error(codes.Unauthenticated, err.Error())
		case errorc.ErrorCodeForbidden:
			return status.Error(codes.PermissionDenied, err.Error())
		case errorc.ErrorCodeNotFound:
			return status.Error(codes.NotFound, err.Error())
		case errorc.ErrorCodeDB, errorc.ErrorCodeThird:
			return status.Error(codes.Internal, err.Error())
		}
	}

	// 默认返回内部错误
	return status.Error(codes.Internal, err.Error())
}
