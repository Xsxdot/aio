package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xsxdot/aio/internal/config"
	"github.com/xsxdot/aio/pkg/protocol"
)

// ConfigService 是配置中心的客户端API封装
type ConfigService struct {
	requestService *RequestService
}

// NewConfigService 创建新的配置中心客户端
func NewConfigService(client *Client) *ConfigService {
	return &ConfigService{
		requestService: NewRequestService(client, client.protocolService),
	}
}

// GetConfig 获取配置
func (c *ConfigService) GetConfig(ctx context.Context, key string) (*config.Config, error) {
	request := &config.GetConfigRequest{
		Key: key,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetConfig,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %v", err)
	}

	return &result, nil
}

// GetConfigJSON 获取JSON格式配置
func (c *ConfigService) GetConfigJSONParse(ctx context.Context, key string, result interface{}) error {
	bytes, err := c.GetConfigJSON(ctx, key)
	if err != nil {
		return fmt.Errorf("获取JSON配置失败: %v", err)
	}

	return json.Unmarshal(bytes, result)
}

// GetConfigJSON 获取JSON格式配置
func (c *ConfigService) GetConfigJSON(ctx context.Context, key string) ([]byte, error) {
	request := &config.GetConfigJSONRequest{
		Key: key,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetConfigJSON,
		config.ServiceTypeConfig,
		"",
		request,
	)

	bytes, err := c.requestService.RequestRaw(msg)
	if err != nil {
		return nil, fmt.Errorf("获取JSON配置失败: %v", err)
	}

	return bytes, nil
}

// SetConfig 设置配置
func (c *ConfigService) SetConfig(ctx context.Context, key string, value map[string]*config.ConfigValue, metadata map[string]string) (*config.Config, error) {
	request := &config.SetConfigRequest{
		Key:      key,
		Value:    value,
		Metadata: metadata,
	}

	msg := protocol.NewMessage(
		config.MsgTypeSetConfig,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("设置配置失败: %v", err)
	}

	return &result, nil
}

// DeleteConfig 删除配置
func (c *ConfigService) DeleteConfig(ctx context.Context, key string) error {
	request := &config.DeleteConfigRequest{
		Key: key,
	}

	msg := protocol.NewMessage(
		config.MsgTypeDeleteConfig,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result protocol.Response
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return fmt.Errorf("删除配置失败: %v", err)
	}

	return nil
}

// ListConfigs 列出所有配置
func (c *ConfigService) ListConfigs(ctx context.Context) ([]*config.Config, error) {
	msg := protocol.NewMessage(
		config.MsgTypeListConfigs,
		config.ServiceTypeConfig,
		"",
		nil,
	)

	var result []*config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取配置列表失败: %v", err)
	}

	return result, nil
}

// GetEnvConfig 获取环境配置
func (c *ConfigService) GetEnvConfig(ctx context.Context, key string, env string, fallbacks []string) (*config.Config, error) {
	request := &config.EnvConfigRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetEnvConfig,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取环境配置失败: %v", err)
	}

	return &result, nil
}

// GetEnvConfigJSON 获取环境配置的JSON表示
func (c *ConfigService) GetEnvConfigJSON(ctx context.Context, key string, env string, fallbacks []string) ([]byte, error) {
	request := &config.GetEnvConfigJSONRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetEnvConfigJSON,
		config.ServiceTypeConfig,
		"",
		request,
	)

	bytes, err := c.requestService.RequestRaw(msg)
	if err != nil {
		return nil, fmt.Errorf("获取环境JSON配置失败: %v", err)
	}

	return bytes, nil
}

func (c *ConfigService) GetEnvConfigJSONParse(ctx context.Context, key string, env string, fallbacks []string, result interface{}) error {
	bytes, err := c.GetEnvConfigJSON(ctx, key, env, fallbacks)
	if err != nil {
		return fmt.Errorf("获取环境JSON配置失败: %v", err)
	}
	return json.Unmarshal(bytes, result)
}

// SetEnvConfig 设置环境配置
func (c *ConfigService) SetEnvConfig(ctx context.Context, key string, env string, value map[string]*config.ConfigValue, metadata map[string]string) (*config.Config, error) {
	request := &config.EnvConfigRequest{
		Key:      key,
		Env:      env,
		Value:    value,
		Metadata: metadata,
	}

	msg := protocol.NewMessage(
		config.MsgTypeSetEnvConfig,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("设置环境配置失败: %v", err)
	}

	return &result, nil
}

// ListEnvConfigs 列出环境配置
func (c *ConfigService) ListEnvConfigs(ctx context.Context, key string) (map[string]*config.Config, error) {
	request := &config.EnvConfigRequest{
		Key: key,
	}

	msg := protocol.NewMessage(
		config.MsgTypeListEnvConfig,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result map[string]*config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取环境配置列表失败: %v", err)
	}

	return result, nil
}

// GetHistory 获取配置历史
func (c *ConfigService) GetHistory(ctx context.Context, key string, limit int64) ([]map[string]interface{}, error) {
	request := &config.HistoryRequest{
		Key:   key,
		Limit: limit,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetHistory,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result []map[string]interface{}
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取配置历史失败: %v", err)
	}

	return result, nil
}

// GetByRevision 获取特定版本的配置
func (c *ConfigService) GetByRevision(ctx context.Context, key string, revision int64) (*config.Config, error) {
	request := &config.HistoryRequest{
		Key:      key,
		Revision: revision,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetRevision,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result config.Config
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取配置版本失败: %v", err)
	}

	return &result, nil
}

// GetCompositeConfig 获取组合配置
func (c *ConfigService) GetCompositeConfig(ctx context.Context, key string) (map[string]interface{}, error) {
	request := &config.CompositeRequest{
		Key: key,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetComposite,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result map[string]interface{}
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取组合配置失败: %v", err)
	}

	return result, nil
}

// GetCompositeConfigForEnvironment 获取环境下的组合配置
func (c *ConfigService) GetCompositeConfigForEnvironment(ctx context.Context, key string, env string, fallbacks []string) (map[string]interface{}, error) {
	request := &config.CompositeRequest{
		Key:       key,
		Env:       env,
		Fallbacks: fallbacks,
	}

	msg := protocol.NewMessage(
		config.MsgTypeGetComposite,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result map[string]interface{}
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("获取环境组合配置失败: %v", err)
	}

	return result, nil
}

// MergeCompositeConfigs 合并多个组合配置
func (c *ConfigService) MergeCompositeConfigs(ctx context.Context, keys []string) (map[string]interface{}, error) {
	request := &config.CompositeRequest{
		Keys: keys,
	}

	msg := protocol.NewMessage(
		config.MsgTypeMergeComposite,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result map[string]interface{}
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("合并组合配置失败: %v", err)
	}

	return result, nil
}

// MergeCompositeConfigsForEnvironment 在特定环境下合并多个组合配置
func (c *ConfigService) MergeCompositeConfigsForEnvironment(ctx context.Context, keys []string, env string, fallbacks []string) (map[string]interface{}, error) {
	request := &config.CompositeRequest{
		Keys:      keys,
		Env:       env,
		Fallbacks: fallbacks,
	}

	msg := protocol.NewMessage(
		config.MsgTypeMergeComposite,
		config.ServiceTypeConfig,
		"",
		request,
	)

	var result map[string]interface{}
	err := c.requestService.Request(msg, &result)
	if err != nil {
		return nil, fmt.Errorf("合并环境组合配置失败: %v", err)
	}

	return result, nil
}
