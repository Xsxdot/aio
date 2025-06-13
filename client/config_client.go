package client

import (
	"context"
	"encoding/json"
	"fmt"

	configv1 "github.com/xsxdot/aio/api/proto/config/v1"
)

// ConfigClient 配置服务客户端
type ConfigClient struct {
	manager *GRPCClientManager
}

// NewConfigClient 创建新的配置服务客户端
func NewConfigClient(manager *GRPCClientManager) *ConfigClient {
	return &ConfigClient{
		manager: manager,
	}
}

// GetConfig 获取配置
func (c *ConfigClient) GetConfig(ctx context.Context, key string) (*configv1.Config, error) {
	var result *configv1.Config
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetConfig(authCtx, &configv1.GetConfigRequest{
			Key: key,
		})
		if err != nil {
			return err
		}
		result = resp.Config
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %v", err)
	}
	return result, nil
}

// GetConfigJSON 获取JSON格式配置
func (c *ConfigClient) GetConfigJSON(ctx context.Context, key string) (string, error) {
	var result string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetConfigJSON(authCtx, &configv1.GetConfigJSONRequest{
			Key: key,
		})
		if err != nil {
			return err
		}
		result = resp.JsonData
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("获取JSON配置失败: %v", err)
	}
	return result, nil
}

// GetConfigJSONWithParse 获取JSON格式配置
func (c *ConfigClient) GetConfigJSONWithParse(ctx context.Context, key string, value interface{}) error {
	var result string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetConfigJSON(authCtx, &configv1.GetConfigJSONRequest{
			Key: key,
		})
		if err != nil {
			return err
		}
		result = resp.JsonData
		return nil
	})
	if err != nil {
		return fmt.Errorf("获取JSON配置失败: %v", err)
	}
	err = json.Unmarshal([]byte(result), value)
	if err != nil {
		return fmt.Errorf("解析JSON配置失败: %v", err)
	}
	return nil
}

// SetConfig 设置配置
func (c *ConfigClient) SetConfig(ctx context.Context, key string, value map[string]*configv1.ConfigValue, metadata map[string]string) (*configv1.Config, error) {
	var result *configv1.Config
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.SetConfig(authCtx, &configv1.SetConfigRequest{
			Key:      key,
			Value:    value,
			Metadata: metadata,
		})
		if err != nil {
			return err
		}
		result = resp.Config
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("设置配置失败: %v", err)
	}
	return result, nil
}

// DeleteConfig 删除配置
func (c *ConfigClient) DeleteConfig(ctx context.Context, key string) (bool, error) {
	var result bool
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.DeleteConfig(authCtx, &configv1.DeleteConfigRequest{
			Key: key,
		})
		if err != nil {
			return err
		}
		result = resp.Success
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("删除配置失败: %v", err)
	}
	return result, nil
}

// ListConfigs 列出所有配置
func (c *ConfigClient) ListConfigs(ctx context.Context, pageSize int32, pageToken string) ([]*configv1.Config, string, error) {
	var configs []*configv1.Config
	var nextToken string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.ListConfigs(authCtx, &configv1.ListConfigsRequest{
			PageSize:  pageSize,
			PageToken: pageToken,
		})
		if err != nil {
			return err
		}
		configs = resp.Configs
		nextToken = resp.NextPageToken
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("列出配置失败: %v", err)
	}
	return configs, nextToken, nil
}

// GetEnvConfig 获取环境配置
func (c *ConfigClient) GetEnvConfig(ctx context.Context, key, env string, fallbacks []string) (*configv1.Config, error) {
	var result *configv1.Config
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetEnvConfig(authCtx, &configv1.GetEnvConfigRequest{
			Key:       key,
			Env:       env,
			Fallbacks: fallbacks,
		})
		if err != nil {
			return err
		}
		result = resp.Config
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取环境配置失败: %v", err)
	}
	return result, nil
}

// SetEnvConfig 设置环境配置
func (c *ConfigClient) SetEnvConfig(ctx context.Context, key, env string, value map[string]*configv1.ConfigValue, metadata map[string]string) (*configv1.Config, error) {
	var result *configv1.Config
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.SetEnvConfig(authCtx, &configv1.SetEnvConfigRequest{
			Key:      key,
			Env:      env,
			Value:    value,
			Metadata: metadata,
		})
		if err != nil {
			return err
		}
		result = resp.Config
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("设置环境配置失败: %v", err)
	}
	return result, nil
}

// ListEnvConfig 列出环境配置
func (c *ConfigClient) ListEnvConfig(ctx context.Context, key string) ([]string, error) {
	var result []string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.ListEnvConfig(authCtx, &configv1.ListEnvConfigRequest{
			Key: key,
		})
		if err != nil {
			return err
		}
		result = resp.Environments
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("列出环境配置失败: %v", err)
	}
	return result, nil
}

// GetEnvConfigJSON 获取环境JSON格式配置
func (c *ConfigClient) GetEnvConfigJSON(ctx context.Context, key, env string, fallbacks []string) (string, error) {
	var result string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetEnvConfigJSON(authCtx, &configv1.GetEnvConfigJSONRequest{
			Key:       key,
			Env:       env,
			Fallbacks: fallbacks,
		})
		if err != nil {
			return err
		}
		result = resp.JsonData
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("获取环境JSON配置失败: %v", err)
	}
	return result, nil
}

// GetEnvConfigJSONWithParse 获取环境JSON格式配置
func (c *ConfigClient) GetEnvConfigJSONWithParse(ctx context.Context, key, env string, fallbacks []string, value interface{}) error {
	var result string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetEnvConfigJSON(authCtx, &configv1.GetEnvConfigJSONRequest{
			Key:       key,
			Env:       env,
			Fallbacks: fallbacks,
		})
		if err != nil {
			return err
		}
		result = resp.JsonData
		return nil
	})
	if err != nil {
		return fmt.Errorf("获取环境JSON配置失败: %v", err)
	}
	err = json.Unmarshal([]byte(result), value)
	if err != nil {
		return fmt.Errorf("解析JSON配置失败: %v", err)
	}
	return nil
}

// GetHistory 获取配置历史
func (c *ConfigClient) GetHistory(ctx context.Context, key string, limit int64) ([]*configv1.ConfigHistory, error) {
	var result []*configv1.ConfigHistory
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetHistory(authCtx, &configv1.GetHistoryRequest{
			Key:   key,
			Limit: limit,
		})
		if err != nil {
			return err
		}
		result = resp.History
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取配置历史失败: %v", err)
	}
	return result, nil
}

// GetRevision 获取特定版本配置
func (c *ConfigClient) GetRevision(ctx context.Context, key string, revision int64) (*configv1.Config, error) {
	var result *configv1.Config
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetRevision(authCtx, &configv1.GetRevisionRequest{
			Key:      key,
			Revision: revision,
		})
		if err != nil {
			return err
		}
		result = resp.Config
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取版本配置失败: %v", err)
	}
	return result, nil
}

// GetComposite 获取组合配置
func (c *ConfigClient) GetComposite(ctx context.Context, key, env string, fallbacks []string) (map[string]string, error) {
	var result map[string]string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.GetComposite(authCtx, &configv1.GetCompositeRequest{
			Key:       key,
			Env:       env,
			Fallbacks: fallbacks,
		})
		if err != nil {
			return err
		}
		result = resp.Composite
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取组合配置失败: %v", err)
	}
	return result, nil
}

// MergeComposite 合并组合配置
func (c *ConfigClient) MergeComposite(ctx context.Context, keys []string, env string, fallbacks []string) (map[string]string, error) {
	var result map[string]string
	err := c.manager.ExecuteWithRetry(ctx, func(authCtx context.Context) error {
		client := c.manager.GetConfigClient()
		resp, err := client.MergeComposite(authCtx, &configv1.MergeCompositeRequest{
			Keys:      keys,
			Env:       env,
			Fallbacks: fallbacks,
		})
		if err != nil {
			return err
		}
		result = resp.Composite
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("合并组合配置失败: %v", err)
	}
	return result, nil
}

// ConfigValueHelper 提供创建 ConfigValue 的辅助方法
type ConfigValueHelper struct{}

// NewConfigValueHelper 创建 ConfigValue 辅助工具
func NewConfigValueHelper() *ConfigValueHelper {
	return &ConfigValueHelper{}
}

// String 创建字符串类型的配置值
func (h *ConfigValueHelper) String(value string) *configv1.ConfigValue {
	return &configv1.ConfigValue{
		StringValue: value,
		Type:        "string",
	}
}

// Int 创建整数类型的配置值
func (h *ConfigValueHelper) Int(value int64) *configv1.ConfigValue {
	return &configv1.ConfigValue{
		IntValue: value,
		Type:     "int",
	}
}

// Float 创建浮点数类型的配置值
func (h *ConfigValueHelper) Float(value float64) *configv1.ConfigValue {
	return &configv1.ConfigValue{
		FloatValue: value,
		Type:       "float",
	}
}

// Bool 创建布尔类型的配置值
func (h *ConfigValueHelper) Bool(value bool) *configv1.ConfigValue {
	return &configv1.ConfigValue{
		BoolValue: value,
		Type:      "bool",
	}
}

// Bytes 创建字节类型的配置值
func (h *ConfigValueHelper) Bytes(value []byte) *configv1.ConfigValue {
	return &configv1.ConfigValue{
		BytesValue: value,
		Type:       "bytes",
	}
}
