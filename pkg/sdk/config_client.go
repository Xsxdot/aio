package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	configpb "github.com/xsxdot/aio/system/config/api/proto"

	"google.golang.org/grpc"
)

// ConfigClient 配置中心客户端
// 封装配置查询端 RPC（GetConfig/BatchGetConfigs）以及管理端 RPC（Create/Update/Delete）
type ConfigClient struct {
	env     string
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
func newConfigClient(conn *grpc.ClientConn, env string) *ConfigClient {
	if strings.TrimSpace(env) == "" {
		env = "dev"
	}
	return &ConfigClient{
		env:     env,
		service: configpb.NewConfigServiceClient(conn),
	}
}

// GetConfigJSON 获取配置（返回 JSON 字符串）
// key: 配置键
func (c *ConfigClient) GetConfigJSON(ctx context.Context, key string) (string, error) {
	req := &configpb.GetConfigRequest{
		Key: key,
		Env: c.env,
	}

	resp, err := c.service.GetConfig(ctx, req)
	if err != nil {
		return "", WrapError(err, "get config failed")
	}

	return resp.JsonStr, nil
}

// GetConfigInto 获取配置并直接反序列化到目标对象
// key: 配置键
// target: 目标对象指针，用于接收反序列化后的配置
func (c *ConfigClient) GetConfigInto(ctx context.Context, key string, target any) error {
	jsonStr, err := c.GetConfigJSON(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return WrapError(err, "failed to unmarshal config into target")
	}

	return nil
}

// BatchGetConfigs 批量获取配置
// keys: 配置键列表
// 返回: map[key]jsonStr
func (c *ConfigClient) BatchGetConfigs(ctx context.Context, keys []string) (map[string]string, error) {
	req := &configpb.BatchGetConfigsRequest{
		Keys: keys,
		Env:  c.env,
	}

	resp, err := c.service.BatchGetConfigs(ctx, req)
	if err != nil {
		return nil, WrapError(err, "batch get configs failed")
	}

	return resp.Configs, nil
}

// GetConfigsByPrefix 按前缀获取配置
// prefix: 配置键前缀
// 返回: map[key]jsonStr（key 已去除环境后缀）
func (c *ConfigClient) GetConfigsByPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	req := &configpb.GetConfigsByPrefixRequest{
		Prefix: prefix,
		Env:    c.env,
	}

	resp, err := c.service.GetConfigsByPrefix(ctx, req)
	if err != nil {
		return nil, WrapError(err, "get configs by prefix failed")
	}

	// 去掉环境后缀后返回
	envSuffix := "." + c.env
	result := make(map[string]string, len(resp.Configs))
	for fullKey, jsonStr := range resp.Configs {
		key := strings.TrimSuffix(fullKey, envSuffix)
		result[key] = jsonStr
	}

	return result, nil
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

// GetComposedConfigByPrefix 按前缀获取配置并组装成嵌套 JSON
// 适用于应用启动时需要从多个分散配置项组装成完整配置的场景
func (c *ConfigClient) GetComposedConfigByPrefix(ctx context.Context, prefix string) (string, error) {
	configs, err := c.GetConfigsByPrefix(ctx, prefix)
	if err != nil {
		return "", WrapError(err, "get composed config by prefix failed")
	}
	if len(configs) == 0 {
		return "", WrapError(fmt.Errorf("no configs found with prefix: %s", prefix), "get composed config by prefix failed")
	}
	return ComposeConfigsByPrefix(configs, prefix)
}

// GetComposedConfigInto 按前缀获取配置并组装后直接反序列化到目标对象
// prefix: 配置键前缀
// target: 目标对象指针，用于接收反序列化后的配置
func (c *ConfigClient) GetComposedConfigInto(ctx context.Context, prefix string, target any) error {
	jsonStr, err := c.GetComposedConfigByPrefix(ctx, prefix)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return WrapError(err, "failed to unmarshal composed config into target")
	}

	return nil
}

// configEntry 配置条目（用于排序）
type configEntry struct {
	section string
	obj     map[string]any
	depth   int
}

// ComposeConfigsByPrefix 纯函数，将配置 map 组装成嵌套 JSON（便于测试和复用）
// configs: map[key]jsonStr，key 已去除环境后缀
// prefix: 配置前缀（用于去除前缀得到 section）
func ComposeConfigsByPrefix(configs map[string]string, prefix string) (string, error) {
	// 收集所有条目，并按路径深度排序（父节点先写，子节点后写可覆盖冲突字段）
	entries := make([]configEntry, 0, len(configs))
	prefixDot := prefix + "."

	for fullKey, jsonStr := range configs {
		// 去掉 prefix. 前缀，得到 section
		if !strings.HasPrefix(fullKey, prefixDot) {
			continue
		}
		section := strings.TrimPrefix(fullKey, prefixDot)

		// 解析 JSON 对象
		var obj map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
			return "", fmt.Errorf("failed to parse config %s: %w", fullKey, err)
		}

		depth := strings.Count(section, ".") + 1
		entries = append(entries, configEntry{section: section, obj: obj, depth: depth})
	}

	// 按深度排序（父先写），同深度按 section 字典序
	sortEntriesByDepth(entries)

	// 组装大 JSON
	bigConfig := make(map[string]any)

	for _, e := range entries {
		// 特殊处理：{prefix}.app 的内容 merge 到根
		if e.section == "app" {
			for k, v := range e.obj {
				bigConfig[k] = v
			}
		} else {
			// 按 `.` 分段，写入嵌套路径
			setNestedValue(bigConfig, strings.Split(e.section, "."), e.obj)
		}
	}

	// 序列化为 JSON
	result, err := json.Marshal(bigConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal composed config: %w", err)
	}

	return string(result), nil
}

// sortEntriesByDepth 按深度（父节点优先）和 section 字典序排序
func sortEntriesByDepth(entries []configEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].depth != entries[j].depth {
			return entries[i].depth < entries[j].depth
		}
		return entries[i].section < entries[j].section
	})
}

// setNestedValue 将 value 写入嵌套路径 path，支持递归合并 map
func setNestedValue(root map[string]any, path []string, value map[string]any) {
	if len(path) == 0 {
		return
	}

	// 遍历到倒数第二层
	current := root
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		if _, exists := current[key]; !exists {
			current[key] = make(map[string]any)
		}
		// 如果中间节点不是 map，跳过（不覆盖）
		if nextMap, ok := current[key].(map[string]any); ok {
			current = nextMap
		} else {
			return
		}
	}

	// 最后一层：合并或覆盖
	lastKey := path[len(path)-1]
	if existing, exists := current[lastKey]; exists {
		if existingMap, ok := existing.(map[string]any); ok {
			// 双方都是 map，递归合并
			mergeMaps(existingMap, value)
			return
		}
	}
	// 否则直接覆盖
	current[lastKey] = value
}

// mergeMaps 将 src 的键值递归合并到 dst 中（子节点优先，冲突时覆盖）
func mergeMaps(dst, src map[string]any) {
	for k, v := range src {
		if dstVal, exists := dst[k]; exists {
			// 如果双方都是 map，递归合并
			if dstMap, dstOk := dstVal.(map[string]any); dstOk {
				if srcMap, srcOk := v.(map[string]any); srcOk {
					mergeMaps(dstMap, srcMap)
					continue
				}
			}
		}
		// 否则覆盖
		dst[k] = v
	}
}
