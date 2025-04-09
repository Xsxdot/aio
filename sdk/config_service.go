package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	config2 "github.com/xsxdot/aio/internal/config"
	"github.com/xsxdot/aio/pkg/protocol"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ConfigService 配置中心服务客户端
type ConfigService struct {
	client     *Client
	logger     *zap.Logger
	mtx        sync.RWMutex
	updateChan chan *ConfigUpdateEvent
	handlers   []func(*ConfigUpdateEvent)
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
		logger, _ = zap.NewProduction()
	}

	service := &ConfigService{
		client:     client,
		logger:     logger,
		updateChan: make(chan *ConfigUpdateEvent, 100),
		handlers:   make([]func(*ConfigUpdateEvent), 0),
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
func (s *ConfigService) sendRequest(ctx context.Context, msgType protocol.MessageType, payload interface{}) (*config2.ConfigResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 检查连接并尝试发送请求
	if err := s.client.Connect(); err != nil {
		return nil, fmt.Errorf("连接服务失败: %w", err)
	}

	// 创建一个用于接收响应的通道
	response := make(chan *protocol.CustomMessage, 1)

	// 创建一个临时的服务处理器来接收响应
	respHandler := protocol.NewServiceHandler()

	// 注册响应处理函数
	respHandler.RegisterHandler(config2.MsgTypeConfigResult, func(connID string, msg *protocol.CustomMessage) error {
		select {
		case response <- msg:
		default:
		}
		return nil
	})

	// 生成一个唯一的临时服务名
	tempServiceName := fmt.Sprintf("config-response-%s", generateRandomID())

	// 注册临时服务处理器
	s.client.RegisterServiceHandler(config2.ServiceTypeConfig, tempServiceName, respHandler)

	// 在函数返回时清理临时服务处理器
	defer s.client.protocolMgr.RegisterService(config2.ServiceTypeConfig, "", nil)

	// 发送请求
	if err := s.client.SendMessage(msgType, config2.ServiceTypeConfig, data); err != nil {
		return nil, fmt.Errorf("发送消息失败: %w", err)
	}

	// 等待响应或超时
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-response:
		var configResp config2.ConfigResponse
		if err := json.Unmarshal(msg.Payload(), &configResp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		if !configResp.Success {
			return nil, fmt.Errorf("配置操作失败: %s", configResp.Error)
		}

		return &configResp, nil
	}
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
func (s *ConfigService) GetConfig(ctx context.Context, key string) (interface{}, error) {
	req := config2.GetConfigRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetConfig, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
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
func (s *ConfigService) ListConfigs(ctx context.Context) (interface{}, error) {
	resp, err := s.sendRequest(ctx, config2.MsgTypeListConfigs, struct{}{})
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetEnvConfig 获取环境配置
func (s *ConfigService) GetEnvConfig(ctx context.Context, key string, env string, fallbacks []string) (interface{}, error) {
	req := config2.EnvConfigRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetEnvConfig, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
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
func (s *ConfigService) ListEnvConfigs(ctx context.Context, key string) (interface{}, error) {
	req := config2.EnvConfigRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeListEnvConfig, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetHistory 获取配置历史
func (s *ConfigService) GetHistory(ctx context.Context, key string, limit int64) (interface{}, error) {
	req := config2.HistoryRequest{
		Key:   key,
		Limit: limit,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetHistory, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetRevision 获取指定版本的配置
func (s *ConfigService) GetRevision(ctx context.Context, key string, revision int64) (interface{}, error) {
	req := config2.HistoryRequest{
		Key:      key,
		Revision: revision,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetRevision, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetCompositeConfig 获取组合配置
func (s *ConfigService) GetCompositeConfig(ctx context.Context, key string) (interface{}, error) {
	req := config2.CompositeRequest{
		Key: key,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetComposite, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetCompositeConfigForEnv 获取环境的组合配置
func (s *ConfigService) GetCompositeConfigForEnv(ctx context.Context, key string, env string, fallbacks []string) (interface{}, error) {
	req := config2.CompositeRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeGetComposite, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// MergeCompositeConfigs 合并多个组合配置
func (s *ConfigService) MergeCompositeConfigs(ctx context.Context, keys []string) (interface{}, error) {
	req := config2.CompositeRequest{
		Keys: keys,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeMergeComposite, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// MergeCompositeConfigsForEnv 合并环境的多个组合配置
func (s *ConfigService) MergeCompositeConfigsForEnv(ctx context.Context, keys []string, env string, fallbacks []string) (interface{}, error) {
	req := config2.CompositeRequest{
		Keys:      keys,
		Env:       env,
		Fallbacks: fallbacks,
	}

	resp, err := s.sendRequest(ctx, config2.MsgTypeMergeComposite, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}
