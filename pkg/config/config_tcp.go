package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xsxdot/aio/pkg/protocol"

	"go.uber.org/zap"
)

// 定义配置中心的服务类型
const (
	ServiceTypeConfig = protocol.ServiceTypeConfig
)

// 定义配置中心的消息类型
const (
	// 基本操作
	MsgTypeGetConfig    protocol.MessageType = 1
	MsgTypeSetConfig    protocol.MessageType = 2
	MsgTypeDeleteConfig protocol.MessageType = 3
	MsgTypeListConfigs  protocol.MessageType = 4
	MsgTypeConfigResult protocol.MessageType = 5

	// 环境相关
	MsgTypeGetEnvConfig  protocol.MessageType = 6
	MsgTypeSetEnvConfig  protocol.MessageType = 7
	MsgTypeListEnvConfig protocol.MessageType = 8

	// 历史相关
	MsgTypeGetHistory  protocol.MessageType = 9
	MsgTypeGetRevision protocol.MessageType = 10

	// 组合配置相关
	MsgTypeGetComposite   protocol.MessageType = 11
	MsgTypeMergeComposite protocol.MessageType = 12

	// JSON格式配置相关
	MsgTypeGetConfigJSON    protocol.MessageType = 13
	MsgTypeGetEnvConfigJSON protocol.MessageType = 14
)

// 请求/响应结构定义
type (
	// GetConfigRequest 获取配置请求
	GetConfigRequest struct {
		Key string `json:"key"`
	}

	// GetConfigJSONRequest 获取JSON格式配置请求
	GetConfigJSONRequest struct {
		Key string `json:"key"`
	}

	// GetEnvConfigJSONRequest 获取环境JSON格式配置请求
	GetEnvConfigJSONRequest struct {
		Key       string   `json:"key"`
		Env       string   `json:"env"`
		Fallbacks []string `json:"fallbacks,omitempty"`
	}

	// SetConfigRequest 设置配置请求
	SetConfigRequest struct {
		Key      string                  `json:"key"`
		Value    map[string]*ConfigValue `json:"value"`
		Metadata map[string]string       `json:"metadata"`
	}

	// DeleteConfigRequest 删除配置请求
	DeleteConfigRequest struct {
		Key string `json:"key"`
	}

	// EnvConfigRequest 环境配置请求
	EnvConfigRequest struct {
		Key       string                  `json:"key"`
		Env       string                  `json:"env"`
		Fallbacks []string                `json:"fallbacks,omitempty"`
		Value     map[string]*ConfigValue `json:"value,omitempty"`
		Metadata  map[string]string       `json:"metadata,omitempty"`
	}

	// HistoryRequest 历史请求
	HistoryRequest struct {
		Key      string `json:"key"`
		Limit    int64  `json:"limit,omitempty"`
		Revision int64  `json:"revision,omitempty"`
	}

	// CompositeRequest 组合配置请求
	CompositeRequest struct {
		Key       string   `json:"key"`
		Env       string   `json:"env,omitempty"`
		Keys      []string `json:"keys,omitempty"`
		Fallbacks []string `json:"fallbacks,omitempty"`
	}
)

// TCPAPI 配置中心的TCP API
type TCPAPI struct {
	service *Service
	logger  *zap.Logger
	// 已注册的ProtocolManager
	manager *protocol.ProtocolManager
}

// NewTCPAPI 创建新的配置中心TCP API
func NewTCPAPI(service *Service, logger *zap.Logger) *TCPAPI {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &TCPAPI{
		service: service,
		logger:  logger,
	}
}

// RegisterToManager 将配置服务注册到指定的协议管理器
func (t *TCPAPI) RegisterToManager(manager *protocol.ProtocolManager) {
	if manager == nil {
		t.logger.Error("协议管理器为空，无法注册配置服务")
		return
	}

	// 设置当前使用的管理器
	t.manager = manager

	t.registerHandlers()

	t.logger.Info("配置服务已注册到协议管理器")
}

// registerHandlers 注册消息处理器
func (t *TCPAPI) registerHandlers() {
	// 基本操作
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetConfig, t.handleGetConfig)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeSetConfig, t.handleSetConfig)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeDeleteConfig, t.handleDeleteConfig)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeListConfigs, t.handleListConfigs)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetConfigJSON, t.handleGetConfigJSON)

	// 环境相关
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetEnvConfig, t.handleGetEnvConfig)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeSetEnvConfig, t.handleSetEnvConfig)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeListEnvConfig, t.handleListEnvConfig)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetEnvConfigJSON, t.handleGetEnvConfigJSON)

	// 历史相关
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetHistory, t.handleGetHistory)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetRevision, t.handleGetRevision)

	// 组合配置相关
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeGetComposite, t.handleGetComposite)
	t.manager.RegisterHandle(ServiceTypeConfig, MsgTypeMergeComposite, t.handleMergeComposite)
}

// handleGetConfig 处理获取配置请求
func (t *TCPAPI) handleGetConfig(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request GetConfigRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, err
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	config, err := t.service.Get(context.Background(), request.Key)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %v", err)
	}

	return config, nil
}

// handleGetConfigJSON 处理获取JSON格式配置请求
func (t *TCPAPI) handleGetConfigJSON(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取JSON格式配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request GetConfigJSONRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	// 直接使用ExportConfigAsJSON方法获取JSON格式配置
	jsonData, err := t.service.ExportConfigAsJSON(context.Background(), request.Key)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %v", err)
	}

	// 返回JSON字符串
	return jsonData, nil
}

// handleSetConfig 处理设置配置请求
func (t *TCPAPI) handleSetConfig(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到设置配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request SetConfigRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	if len(request.Value) == 0 {
		return nil, fmt.Errorf("配置值不能为空")
	}

	err := t.service.Set(context.Background(), request.Key, request.Value, request.Metadata)
	if err != nil {
		return nil, fmt.Errorf("设置配置失败: %v", err)
	}

	// 获取新设置的配置
	config, _ := t.service.Get(context.Background(), request.Key)

	return config, nil
}

// handleDeleteConfig 处理删除配置请求
func (t *TCPAPI) handleDeleteConfig(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到删除配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request DeleteConfigRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	// 检查配置是否存在
	_, err := t.service.Get(context.Background(), request.Key)
	if err != nil {
		return nil, fmt.Errorf("配置不存在: %v", err)
	}

	err = t.service.Delete(context.Background(), request.Key)
	if err != nil {
		return nil, fmt.Errorf("删除配置失败: %v", err)
	}

	return protocol.OK, nil
}

// handleListConfigs 处理列出所有配置请求
func (t *TCPAPI) handleListConfigs(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到列出配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	configs, err := t.service.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取配置列表失败: %v", err)
	}

	return configs, nil
}

// handleGetEnvConfig 处理获取环境配置请求
func (t *TCPAPI) handleGetEnvConfig(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取环境配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request EnvConfigRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	if request.Env == "" {
		return nil, fmt.Errorf("环境参数不能为空")
	}

	fallbacks := request.Fallbacks
	if len(fallbacks) == 0 {
		fallbacks = DefaultEnvironmentFallbacks(request.Env)
	}

	envConfig := NewEnvironmentConfig(request.Env, fallbacks...)
	config, err := t.service.GetForEnvironment(context.Background(), request.Key, envConfig)
	if err != nil {
		return nil, fmt.Errorf("获取环境配置失败: %v", err)
	}

	return config, nil
}

// handleSetEnvConfig 处理设置环境配置请求
func (t *TCPAPI) handleSetEnvConfig(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到设置环境配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request EnvConfigRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	if request.Env == "" {
		return nil, fmt.Errorf("环境参数不能为空")
	}

	if len(request.Value) == 0 {
		return nil, fmt.Errorf("配置值不能为空")
	}

	err := t.service.SetForEnvironment(context.Background(), request.Key, request.Env, request.Value, request.Metadata)
	if err != nil {
		return nil, fmt.Errorf("设置环境配置失败: %v", err)
	}

	envConfig := NewEnvironmentConfig(request.Env)
	config, _ := t.service.GetForEnvironment(context.Background(), request.Key, envConfig)

	return config, nil
}

// handleListEnvConfig 处理列出环境配置请求
func (t *TCPAPI) handleListEnvConfig(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到列出环境配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request EnvConfigRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	envConfigs, err := t.service.ListEnvironmentConfigs(context.Background(), request.Key)
	if err != nil {
		return nil, fmt.Errorf("获取环境配置列表失败: %v", err)
	}

	return envConfigs, nil
}

// handleGetHistory 处理获取配置历史请求
func (t *TCPAPI) handleGetHistory(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取配置历史请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request HistoryRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	if request.Limit <= 0 {
		request.Limit = 10 // 默认获取10条历史记录
	}

	history, err := t.service.GetHistory(context.Background(), request.Key, request.Limit)
	if err != nil {
		return nil, fmt.Errorf("获取配置历史失败: %v", err)
	}

	return history, nil
}

// handleGetRevision 处理获取特定版本配置请求
func (t *TCPAPI) handleGetRevision(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取特定版本配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request HistoryRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	if request.Revision <= 0 {
		return nil, fmt.Errorf("修订版本号必须大于0")
	}

	config, err := t.service.GetByRevision(context.Background(), request.Key, request.Revision)
	if err != nil {
		return nil, fmt.Errorf("获取配置修订版本失败: %v", err)
	}

	return config, nil
}

// handleGetComposite 处理获取组合配置请求
func (t *TCPAPI) handleGetComposite(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取组合配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request CompositeRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	var config interface{}
	var err error

	if request.Env == "" {
		// 获取普通组合配置
		config, err = t.service.GetCompositeConfig(context.Background(), request.Key)
	} else {
		// 获取特定环境的组合配置
		fallbacks := request.Fallbacks
		if len(fallbacks) == 0 {
			fallbacks = DefaultEnvironmentFallbacks(request.Env)
		}

		envConfig := NewEnvironmentConfig(request.Env, fallbacks...)
		config, err = t.service.GetCompositeConfigForEnvironment(context.Background(), request.Key, envConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("获取组合配置失败: %v", err)
	}

	return config, nil
}

// handleMergeComposite 处理合并组合配置请求
func (t *TCPAPI) handleMergeComposite(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到合并组合配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request CompositeRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if len(request.Keys) == 0 {
		return nil, fmt.Errorf("至少需要一个配置键")
	}

	var config interface{}
	var err error

	if request.Env == "" {
		// 合并普通组合配置
		config, err = t.service.MergeCompositeConfigs(context.Background(), request.Keys)
	} else {
		// 合并特定环境的组合配置
		fallbacks := request.Fallbacks
		if len(fallbacks) == 0 {
			fallbacks = DefaultEnvironmentFallbacks(request.Env)
		}

		envConfig := NewEnvironmentConfig(request.Env, fallbacks...)
		config, err = t.service.MergeCompositeConfigsForEnvironment(context.Background(), request.Keys, envConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("合并组合配置失败: %v", err)
	}

	return config, nil
}

// handleGetEnvConfigJSON 处理获取环境JSON格式配置请求
func (t *TCPAPI) handleGetEnvConfigJSON(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	t.logger.Debug("收到获取环境JSON格式配置请求", zap.String("connID", connID), zap.String("msgID", msg.Header().MessageID))

	var request GetEnvConfigJSONRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		t.logger.Error("解析请求失败", zap.Error(err))
		return nil, fmt.Errorf("解析请求失败: %v", err)
	}

	if request.Key == "" {
		return nil, fmt.Errorf("配置键不能为空")
	}

	if request.Env == "" {
		return nil, fmt.Errorf("环境参数不能为空")
	}

	// 构建环境配置
	fallbacks := request.Fallbacks
	if len(fallbacks) == 0 {
		fallbacks = DefaultEnvironmentFallbacks(request.Env)
	}
	envConfig := NewEnvironmentConfig(request.Env, fallbacks...)

	// 使用ExportConfigAsJSONForEnvironment获取环境JSON格式配置
	jsonData, err := t.service.ExportConfigAsJSONForEnvironment(context.Background(), request.Key, envConfig)
	if err != nil {
		return nil, fmt.Errorf("获取环境配置失败: %v", err)
	}

	// 返回JSON字符串
	return jsonData, nil
}

// BroadcastConfigUpdate 广播配置更新
func (t *TCPAPI) BroadcastConfigUpdate(key string, env string) error {
	if t.manager == nil {
		return fmt.Errorf("未注册到协议管理器")
	}

	updateInfo := struct {
		Key       string `json:"key"`
		Env       string `json:"env,omitempty"`
		Timestamp int64  `json:"timestamp"`
	}{
		Key:       key,
		Env:       env,
		Timestamp: time.Now().Unix(),
	}

	payload, err := json.Marshal(updateInfo)
	if err != nil {
		t.logger.Error("序列化更新通知失败", zap.Error(err))
		return err
	}

	return t.manager.BroadcastMessage(MsgTypeConfigResult, ServiceTypeConfig, payload)
}
