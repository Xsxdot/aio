package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	config2 "github.com/xsxdot/aio/internal/config"
	"github.com/xsxdot/aio/pkg/protocol"

	"go.uber.org/zap"
)

// ConfigService 配置中心服务客户端
type ConfigService struct {
	client     *Client
	logger     *zap.Logger
	mtx        sync.RWMutex
	updateChan chan *ConfigUpdateEvent
	handlers   []func(*ConfigUpdateEvent)
	tcpClient  *TCPAPIClient // 添加TCP API客户端
}

// ConfigUpdateEvent 配置更新事件
type ConfigUpdateEvent struct {
	Key       string    // 更新的配置键
	Env       string    // 环境（如果适用）
	Timestamp time.Time // 更新时间戳
}

// ConfigServiceOptions 配置服务选项
type ConfigServiceOptions struct {
	Logger *zap.Logger
}

// NewConfigService 创建新的配置服务
func NewConfigService(client *Client, options *ConfigServiceOptions) *ConfigService {
	var logger *zap.Logger
	if options != nil && options.Logger != nil {
		logger = options.Logger
	} else {
		l, _ := zap.NewProduction()
		logger = l
	}

	service := &ConfigService{
		client:     client,
		logger:     logger,
		updateChan: make(chan *ConfigUpdateEvent, 100),
		handlers:   make([]func(*ConfigUpdateEvent), 0),
		tcpClient:  client.GetTCPAPIClient(), // 获取TCP API客户端
	}

	// 注册消息处理器
	service.registerConfigUpdateHandler()

	// 启动更新事件处理
	go service.processUpdateEvents()

	return service
}

// 注册配置更新处理器
func (s *ConfigService) registerConfigUpdateHandler() {
	handler := protocol.NewServiceHandler()
	handler.RegisterHandler(config2.MsgTypeConfigResult, s.handleConfigUpdate)
	s.client.RegisterServiceHandler(config2.ServiceTypeConfig, "config-update-handler", handler)
}

// 处理配置更新通知
func (s *ConfigService) handleConfigUpdate(connID string, msg *protocol.CustomMessage) error {
	var updateInfo struct {
		Key       string `json:"key"`
		Env       string `json:"env"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(msg.Payload(), &updateInfo); err != nil {
		s.logger.Error("解析配置更新通知失败", zap.Error(err))
		return err
	}

	// 如果是配置响应而不是更新通知，则忽略
	response := &config2.ConfigResponse{}
	if err := json.Unmarshal(msg.Payload(), response); err == nil {
		if response.Success {
			// 这是一个正常的响应，不是更新通知
			return nil
		}
	}

	// 创建更新事件
	event := &ConfigUpdateEvent{
		Key:       updateInfo.Key,
		Env:       updateInfo.Env,
		Timestamp: time.Unix(updateInfo.Timestamp, 0),
	}

	// 发送到更新通道
	select {
	case s.updateChan <- event:
	default:
		s.logger.Warn("配置更新事件通道已满，丢弃更新", zap.String("key", event.Key))
	}

	return nil
}

// 处理更新事件
func (s *ConfigService) processUpdateEvents() {
	for event := range s.updateChan {
		s.mtx.RLock()
		handlers := s.handlers
		s.mtx.RUnlock()

		// 通知所有处理器
		for _, handler := range handlers {
			go handler(event)
		}
	}
}

// OnConfigUpdate 注册配置更新事件处理器
func (s *ConfigService) OnConfigUpdate(handler func(*ConfigUpdateEvent)) {
	if handler == nil {
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.handlers = append(s.handlers, handler)
}

// 发送请求并获取响应
func (s *ConfigService) sendRequest(ctx context.Context, msgType protocol.MessageType, payload interface{}) (*protocol.APIResponse, error) {
	// 使用TCP API客户端发送请求
	return s.tcpClient.Send(
		ctx,
		msgType,
		config2.ServiceTypeConfig,
		config2.MsgTypeConfigResult, // 响应消息类型
		payload,
	)
}

// generateRandomID 生成随机ID，用于临时服务名
func generateRandomID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// GetConfig 获取配置
func (s *ConfigService) GetConfig(ctx context.Context, key string) (*config2.ConfigItem, error) {
	req := config2.GetConfigRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetConfig, req)
	if err != nil {
		return nil, err
	}

	// 检查数据是否为空
	if resp.Data == "" {
		return nil, fmt.Errorf("返回的配置数据为空")
	}

	// 从字符串反序列化为配置项对象
	var configItem config2.ConfigItem
	if err := json.Unmarshal([]byte(resp.Data), &configItem); err != nil {
		return nil, fmt.Errorf("反序列化配置数据失败: %w", err)
	}

	return &configItem, nil
}

// GetConfigJSON 获取配置并反序列化到指定的结构体
func (s *ConfigService) GetConfigJSON(ctx context.Context, key string, result interface{}) error {
	req := config2.GetConfigRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetConfig, req)
	if err != nil {
		return err
	}

	// 如果数据为空，返回错误
	if resp.Data == "" {
		return fmt.Errorf("返回的配置数据为空")
	}

	// 从字符串直接反序列化到结果对象
	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化配置数据失败: %w", err)
	}

	return nil
}

// SetConfig 设置配置
func (s *ConfigService) SetConfig(ctx context.Context, key string, value map[string]*config2.ConfigValue, metadata map[string]string) error {
	req := config2.SetConfigRequest{
		Key:      key,
		Value:    value,
		Metadata: metadata,
	}

	_, err := s.sendRequest(ctx, config2.MsgTypeSetConfig, req)
	return err
}

// DeleteConfig 删除配置
func (s *ConfigService) DeleteConfig(ctx context.Context, key string) error {
	req := config2.DeleteConfigRequest{
		Key: key,
	}

	_, err := s.sendRequest(ctx, config2.MsgTypeDeleteConfig, req)
	return err
}

// ListConfigs 列出所有配置
func (s *ConfigService) ListConfigs(ctx context.Context) ([]*config2.ConfigItem, error) {
	resp, err := s.sendRequest(ctx, config2.MsgTypeListConfigs, struct{}{})
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的配置列表为空")
	}

	// 从字符串反序列化为配置项列表
	var configItems []*config2.ConfigItem
	if err := json.Unmarshal([]byte(resp.Data), &configItems); err != nil {
		return nil, fmt.Errorf("反序列化配置列表失败: %w", err)
	}

	return configItems, nil
}

// ListConfigsJSON 列出所有配置并反序列化到指定的结构体
func (s *ConfigService) ListConfigsJSON(ctx context.Context, result interface{}) error {
	resp, err := s.sendRequest(ctx, config2.MsgTypeListConfigs, struct{}{})
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的配置列表为空")
	}

	// 从字符串直接反序列化到结果对象
	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化配置列表失败: %w", err)
	}

	return nil
}

// GetEnvConfig 获取环境配置
func (s *ConfigService) GetEnvConfig(ctx context.Context, key string, env string, fallbacks []string) (*config2.ConfigItem, error) {
	req := config2.EnvConfigRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetEnvConfig, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的环境配置数据为空")
	}

	// 从字符串反序列化为配置项对象
	var configItem config2.ConfigItem
	if err := json.Unmarshal([]byte(resp.Data), &configItem); err != nil {
		return nil, fmt.Errorf("反序列化环境配置数据失败: %w", err)
	}

	return &configItem, nil
}

// GetEnvConfigJSON 获取环境配置并反序列化到指定的结构体
func (s *ConfigService) GetEnvConfigJSON(ctx context.Context, key string, env string, fallbacks []string, result interface{}) error {
	req := config2.EnvConfigRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetEnvConfig, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的环境配置数据为空")
	}

	// 从字符串直接反序列化到结果对象
	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化环境配置数据失败: %w", err)
	}

	return nil
}

// SetEnvConfig 设置环境配置
func (s *ConfigService) SetEnvConfig(ctx context.Context, key string, env string, value map[string]*config2.ConfigValue, metadata map[string]string) error {
	req := config2.EnvConfigRequest{
		Key:      key,
		Env:      env,
		Value:    value,
		Metadata: metadata,
	}

	_, err := s.sendRequest(ctx, config2.MsgTypeSetEnvConfig, req)
	return err
}

// ListEnvConfigs 列出环境配置
func (s *ConfigService) ListEnvConfigs(ctx context.Context, key string) ([]*config2.ConfigItem, error) {
	req := config2.EnvConfigRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeListEnvConfig, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的环境配置列表为空")
	}

	var configItems []*config2.ConfigItem
	if err := json.Unmarshal([]byte(resp.Data), &configItems); err != nil {
		return nil, fmt.Errorf("反序列化环境配置列表失败: %w", err)
	}

	return configItems, nil
}

// ListEnvConfigsJSON 列出环境配置并反序列化到指定的结构体
func (s *ConfigService) ListEnvConfigsJSON(ctx context.Context, key string, result interface{}) error {
	req := config2.EnvConfigRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeListEnvConfig, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的环境配置列表为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化环境配置列表失败: %w", err)
	}

	return nil
}

// GetHistory 获取配置历史
func (s *ConfigService) GetHistory(ctx context.Context, key string, limit int64) ([]*config2.HistoryItem, error) {
	req := config2.HistoryRequest{
		Key:   key,
		Limit: limit,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetHistory, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的配置历史为空")
	}

	var historyItems []*config2.HistoryItem
	if err := json.Unmarshal([]byte(resp.Data), &historyItems); err != nil {
		return nil, fmt.Errorf("反序列化配置历史失败: %w", err)
	}

	return historyItems, nil
}

// GetHistoryJSON 获取配置历史并反序列化到指定的结构体
func (s *ConfigService) GetHistoryJSON(ctx context.Context, key string, limit int64, result interface{}) error {
	req := config2.HistoryRequest{
		Key:   key,
		Limit: limit,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetHistory, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的配置历史为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化配置历史失败: %w", err)
	}

	return nil
}

// GetRevision 获取特定版本的配置
func (s *ConfigService) GetRevision(ctx context.Context, key string, revision int64) (*config2.HistoryItem, error) {
	req := config2.HistoryRequest{
		Key:      key,
		Revision: revision,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetRevision, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的配置版本为空")
	}

	var historyItem config2.HistoryItem
	if err := json.Unmarshal([]byte(resp.Data), &historyItem); err != nil {
		return nil, fmt.Errorf("反序列化配置版本失败: %w", err)
	}

	return &historyItem, nil
}

// GetRevisionJSON 获取特定版本的配置并反序列化到指定的结构体
func (s *ConfigService) GetRevisionJSON(ctx context.Context, key string, revision int64, result interface{}) error {
	req := config2.HistoryRequest{
		Key:      key,
		Revision: revision,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetRevision, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的配置版本为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化配置版本失败: %w", err)
	}

	return nil
}

// GetCompositeConfig 获取组合配置
func (s *ConfigService) GetCompositeConfig(ctx context.Context, key string) (map[string]interface{}, error) {
	req := config2.CompositeRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetComposite, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的组合配置为空")
	}

	var compositeConfig map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Data), &compositeConfig); err != nil {
		return nil, fmt.Errorf("反序列化组合配置失败: %w", err)
	}

	return compositeConfig, nil
}

// GetCompositeConfigJSON 获取组合配置并反序列化到指定的结构体
func (s *ConfigService) GetCompositeConfigJSON(ctx context.Context, key string, result interface{}) error {
	req := config2.CompositeRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetComposite, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的组合配置为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化组合配置失败: %w", err)
	}

	return nil
}

// GetCompositeConfigForEnv 获取环境组合配置
func (s *ConfigService) GetCompositeConfigForEnv(ctx context.Context, key string, env string, fallbacks []string) (map[string]interface{}, error) {
	req := config2.CompositeRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetComposite, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的环境组合配置为空")
	}

	var compositeConfig map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Data), &compositeConfig); err != nil {
		return nil, fmt.Errorf("反序列化环境组合配置失败: %w", err)
	}

	return compositeConfig, nil
}

// GetCompositeConfigForEnvJSON 获取环境组合配置并反序列化到指定的结构体
func (s *ConfigService) GetCompositeConfigForEnvJSON(ctx context.Context, key string, env string, fallbacks []string, result interface{}) error {
	req := config2.CompositeRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetComposite, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的环境组合配置为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化环境组合配置失败: %w", err)
	}

	return nil
}

// MergeCompositeConfigs 合并多个组合配置
func (s *ConfigService) MergeCompositeConfigs(ctx context.Context, keys []string) (map[string]interface{}, error) {
	req := config2.CompositeRequest{
		Keys: keys,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeMergeComposite, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的合并组合配置为空")
	}

	var compositeConfig map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Data), &compositeConfig); err != nil {
		return nil, fmt.Errorf("反序列化合并组合配置失败: %w", err)
	}

	return compositeConfig, nil
}

// MergeCompositeConfigsJSON 合并多个组合配置并反序列化到指定的结构体
func (s *ConfigService) MergeCompositeConfigsJSON(ctx context.Context, keys []string, result interface{}) error {
	req := config2.CompositeRequest{
		Keys: keys,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeMergeComposite, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的合并组合配置为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化合并组合配置失败: %w", err)
	}

	return nil
}

// MergeCompositeConfigsForEnv 合并环境的多个组合配置
func (s *ConfigService) MergeCompositeConfigsForEnv(ctx context.Context, keys []string, env string, fallbacks []string) (map[string]interface{}, error) {
	req := config2.CompositeRequest{
		Keys:      keys,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeMergeComposite, req)
	if err != nil {
		return nil, err
	}

	if resp.Data == "" {
		return nil, fmt.Errorf("返回的环境合并组合配置为空")
	}

	var compositeConfig map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Data), &compositeConfig); err != nil {
		return nil, fmt.Errorf("反序列化环境合并组合配置失败: %w", err)
	}

	return compositeConfig, nil
}

// MergeCompositeConfigsForEnvJSON 合并环境的多个组合配置并反序列化到指定的结构体
func (s *ConfigService) MergeCompositeConfigsForEnvJSON(ctx context.Context, keys []string, env string, fallbacks []string, result interface{}) error {
	req := config2.CompositeRequest{
		Keys:      keys,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeMergeComposite, req)
	if err != nil {
		return err
	}

	if resp.Data == "" {
		return fmt.Errorf("返回的环境合并组合配置为空")
	}

	if err := json.Unmarshal([]byte(resp.Data), result); err != nil {
		return fmt.Errorf("反序列化环境合并组合配置失败: %w", err)
	}

	return nil
}

// GetConfigJSONString 获取配置的JSON字符串表示
func (s *ConfigService) GetConfigJSONString(ctx context.Context, key string) (string, error) {
	req := config2.GetConfigJSONRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetConfigJSON, req)
	if err != nil {
		return "", err
	}

	// 直接返回Data字段中的JSON字符串
	if resp.Data == "" {
		return "", fmt.Errorf("返回的配置JSON数据为空")
	}

	return resp.Data, nil
}

// GetEnvConfigJSONString 获取环境配置的JSON字符串表示
func (s *ConfigService) GetEnvConfigJSONString(ctx context.Context, key string, env string, fallbacks []string) (string, error) {
	req := config2.GetEnvConfigJSONRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetEnvConfigJSON, req)
	if err != nil {
		return "", err
	}

	// 直接返回Data字段中的JSON字符串
	if resp.Data == "" {
		return "", fmt.Errorf("返回的环境配置JSON数据为空")
	}

	return resp.Data, nil
}

// GetConfigWithStruct 从配置中心获取配置并反序列化到结构体
func (s *ConfigService) GetConfigWithStruct(ctx context.Context, key string, result interface{}) error {
	jsonString, err := s.GetConfigJSONString(ctx, key)
	if err != nil {
		return fmt.Errorf("获取配置JSON字符串失败: %w", err)
	}

	// 将JSON字符串反序列化到结构体
	if err := json.Unmarshal([]byte(jsonString), result); err != nil {
		return fmt.Errorf("反序列化配置失败: %w", err)
	}

	return nil
}

// GetEnvConfigWithStruct 从配置中心获取环境配置并反序列化到结构体
func (s *ConfigService) GetEnvConfigWithStruct(ctx context.Context, key string, env string, fallbacks []string, result interface{}) error {
	jsonString, err := s.GetEnvConfigJSONString(ctx, key, env, fallbacks)
	if err != nil {
		return fmt.Errorf("获取环境配置JSON字符串失败: %w", err)
	}

	// 将JSON字符串反序列化到结构体
	if err := json.Unmarshal([]byte(jsonString), result); err != nil {
		return fmt.Errorf("反序列化环境配置失败: %w", err)
	}

	return nil
}
