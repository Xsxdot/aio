package sdk

import (
	"context"

	configpb "github.com/xsxdot/aio/system/config/api/proto"

	"google.golang.org/grpc"
)

// ConfigClient 配置中心客户端
// 封装配置查询端 RPC（GetConfig/BatchGetConfigs）以及管理端 RPC（Create/Update/Delete）
type ConfigClient struct {
	service configpb.ConfigServiceClient
}

// ValueType 配置值类型
type ValueType string

const (
	ValueTypeString    ValueType = "string"
	ValueTypeInt       ValueType = "int"
	ValueTypeFloat     ValueType = "float"
	ValueTypeBool      ValueType = "bool"
	ValueTypeRef       ValueType = "ref"
	ValueTypeObject    ValueType = "object"
	ValueTypeArray     ValueType = "array"
	ValueTypeEncrypted ValueType = "encrypted"
)

// ConfigValue 配置值（SDK 友好版）
type ConfigValue struct {
	Value string
	Type  ValueType
}

// CreateConfigRequest 创建配置请求（SDK 友好版）
type CreateConfigRequest struct {
	Key         string                  // 配置键（带环境后缀，如 app.cert.dev）
	Value       map[string]*ConfigValue // 配置值，key 为属性名
	Metadata    map[string]string       // 元数据（可选）
	Description string                  // 配置描述（可选）
	ChangeNote  string                  // 变更说明（可选）
}

// UpdateConfigRequest 更新配置请求（SDK 友好版）
type UpdateConfigRequest struct {
	Key         string                  // 配置键（带环境后缀，如 app.cert.dev）
	Value       map[string]*ConfigValue // 配置值，key 为属性名
	Metadata    map[string]string       // 元数据（可选）
	Description string                  // 配置描述（可选）
	ChangeNote  string                  // 变更说明（可选）
}

// DeleteConfigResponse 删除配置响应
type DeleteConfigResponse struct {
	Success bool
	Message string
}

// ConfigInfo 配置信息（SDK 友好版）
type ConfigInfo struct {
	ID          int64
	Key         string
	Value       map[string]*ConfigValue // 配置值，key 为属性名
	Version     int64
	Metadata    map[string]string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

// newConfigClient 创建配置中心客户端
func newConfigClient(conn *grpc.ClientConn) *ConfigClient {
	return &ConfigClient{
		service: configpb.NewConfigServiceClient(conn),
	}
}

// GetConfigJSON 获取配置（返回 JSON 字符串）
// key: 配置键
// env: 环境（如 dev, prod, test）
func (c *ConfigClient) GetConfigJSON(ctx context.Context, key, env string) (string, error) {
	req := &configpb.GetConfigRequest{
		Key: key,
		Env: env,
	}

	resp, err := c.service.GetConfig(ctx, req)
	if err != nil {
		return "", WrapError(err, "get config failed")
	}

	return resp.JsonStr, nil
}

// BatchGetConfigs 批量获取配置
// keys: 配置键列表
// env: 环境（如 dev, prod, test）
// 返回: map[key]jsonStr
func (c *ConfigClient) BatchGetConfigs(ctx context.Context, keys []string, env string) (map[string]string, error) {
	req := &configpb.BatchGetConfigsRequest{
		Keys: keys,
		Env:  env,
	}

	resp, err := c.service.BatchGetConfigs(ctx, req)
	if err != nil {
		return nil, WrapError(err, "batch get configs failed")
	}

	return resp.Configs, nil
}

// GetConfigsByPrefix 按前缀获取配置
// prefix: 配置键前缀
// env: 环境（如 dev, prod, test）
// 返回: map[fullKey]jsonStr（fullKey 含环境后缀）
func (c *ConfigClient) GetConfigsByPrefix(ctx context.Context, prefix, env string) (map[string]string, error) {
	req := &configpb.GetConfigsByPrefixRequest{
		Prefix: prefix,
		Env:    env,
	}

	resp, err := c.service.GetConfigsByPrefix(ctx, req)
	if err != nil {
		return nil, WrapError(err, "get configs by prefix failed")
	}

	return resp.Configs, nil
}

// CreateConfig 创建配置
func (c *ConfigClient) CreateConfig(ctx context.Context, req *CreateConfigRequest) (*ConfigInfo, error) {
	// 转换 SDK 请求为 proto 请求
	pbValue := make(map[string]*configpb.ConfigValue)
	for field, val := range req.Value {
		pbValue[field] = &configpb.ConfigValue{
			Value: val.Value,
			Type:  convertSDKValueTypeToPB(val.Type),
		}
	}

	pbReq := &configpb.CreateConfigRequest{
		Key:         req.Key,
		Value:       pbValue,
		Metadata:    req.Metadata,
		Description: req.Description,
		ChangeNote:  req.ChangeNote,
	}

	resp, err := c.service.CreateConfig(ctx, pbReq)
	if err != nil {
		return nil, WrapError(err, "create config failed")
	}

	return convertPBConfigResponseToSDK(resp), nil
}

// UpdateConfig 更新配置
func (c *ConfigClient) UpdateConfig(ctx context.Context, req *UpdateConfigRequest) (*ConfigInfo, error) {
	// 转换 SDK 请求为 proto 请求
	pbValue := make(map[string]*configpb.ConfigValue)
	for field, val := range req.Value {
		pbValue[field] = &configpb.ConfigValue{
			Value: val.Value,
			Type:  convertSDKValueTypeToPB(val.Type),
		}
	}

	pbReq := &configpb.UpdateConfigRequest{
		Key:         req.Key,
		Value:       pbValue,
		Metadata:    req.Metadata,
		Description: req.Description,
		ChangeNote:  req.ChangeNote,
	}

	resp, err := c.service.UpdateConfig(ctx, pbReq)
	if err != nil {
		return nil, WrapError(err, "update config failed")
	}

	return convertPBConfigResponseToSDK(resp), nil
}

// DeleteConfig 删除配置
func (c *ConfigClient) DeleteConfig(ctx context.Context, key string) (*DeleteConfigResponse, error) {
	pbReq := &configpb.DeleteConfigRequest{
		Key: key,
	}

	resp, err := c.service.DeleteConfig(ctx, pbReq)
	if err != nil {
		return nil, WrapError(err, "delete config failed")
	}

	return &DeleteConfigResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}

// convertSDKValueTypeToPB 转换 SDK ValueType 为 proto ValueType
func convertSDKValueTypeToPB(sdkType ValueType) configpb.ValueType {
	switch sdkType {
	case ValueTypeString:
		return configpb.ValueType_VALUE_TYPE_STRING
	case ValueTypeInt:
		return configpb.ValueType_VALUE_TYPE_INT
	case ValueTypeFloat:
		return configpb.ValueType_VALUE_TYPE_FLOAT
	case ValueTypeBool:
		return configpb.ValueType_VALUE_TYPE_BOOL
	case ValueTypeRef:
		return configpb.ValueType_VALUE_TYPE_REF
	case ValueTypeObject:
		return configpb.ValueType_VALUE_TYPE_OBJECT
	case ValueTypeArray:
		return configpb.ValueType_VALUE_TYPE_ARRAY
	case ValueTypeEncrypted:
		return configpb.ValueType_VALUE_TYPE_ENCRYPTED
	default:
		return configpb.ValueType_VALUE_TYPE_STRING
	}
}

// convertPBValueTypeToSDK 转换 proto ValueType 为 SDK ValueType
func convertPBValueTypeToSDK(pbType configpb.ValueType) ValueType {
	switch pbType {
	case configpb.ValueType_VALUE_TYPE_STRING:
		return ValueTypeString
	case configpb.ValueType_VALUE_TYPE_INT:
		return ValueTypeInt
	case configpb.ValueType_VALUE_TYPE_FLOAT:
		return ValueTypeFloat
	case configpb.ValueType_VALUE_TYPE_BOOL:
		return ValueTypeBool
	case configpb.ValueType_VALUE_TYPE_REF:
		return ValueTypeRef
	case configpb.ValueType_VALUE_TYPE_OBJECT:
		return ValueTypeObject
	case configpb.ValueType_VALUE_TYPE_ARRAY:
		return ValueTypeArray
	case configpb.ValueType_VALUE_TYPE_ENCRYPTED:
		return ValueTypeEncrypted
	default:
		return ValueTypeString
	}
}

// convertPBConfigResponseToSDK 转换 proto ConfigResponse 为 SDK ConfigInfo
func convertPBConfigResponseToSDK(pbResp *configpb.ConfigResponse) *ConfigInfo {
	if pbResp == nil {
		return nil
	}

	// 转换 Value
	sdkValue := make(map[string]*ConfigValue)
	for field, val := range pbResp.Value {
		sdkValue[field] = &ConfigValue{
			Value: val.Value,
			Type:  convertPBValueTypeToSDK(val.Type),
		}
	}

	return &ConfigInfo{
		ID:          pbResp.Id,
		Key:         pbResp.Key,
		Value:       sdkValue,
		Version:     pbResp.Version,
		Metadata:    pbResp.Metadata,
		Description: pbResp.Description,
		CreatedAt:   pbResp.CreatedAt,
		UpdatedAt:   pbResp.UpdatedAt,
	}
}
