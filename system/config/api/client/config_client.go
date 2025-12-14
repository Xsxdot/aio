package client

import (
	"context"
	"encoding/json"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/config/internal/app"
)

// ConfigClient 配置中心对外客户端
type ConfigClient struct {
	app *app.App
	err *errorc.ErrorBuilder
}

// NewConfigClient 创建配置客户端实例
func NewConfigClient(app *app.App) *ConfigClient {
	return &ConfigClient{
		app: app,
		err: errorc.NewErrorBuilder("ConfigClient"),
	}
}

// GetConfigJSON 获取配置（返回 JSON 字符串）
func (c *ConfigClient) GetConfigJSON(ctx context.Context, key, env string) (string, error) {
	if key == "" {
		return "", c.err.New("配置键不能为空", nil).ValidWithCtx()
	}
	if env == "" {
		return "", c.err.New("环境不能为空", nil).ValidWithCtx()
	}

	return c.app.GetConfigJSONByKeyAndEnv(ctx, key, env)
}

// GetConfig 获取配置（反序列化到对象）
func (c *ConfigClient) GetConfig(ctx context.Context, key, env string, target interface{}) error {
	if target == nil {
		return c.err.New("目标对象不能为nil", nil).ValidWithCtx()
	}

	jsonStr, err := c.GetConfigJSON(ctx, key, env)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return c.err.New("反序列化配置失败", err)
	}

	return nil
}

// GetConfigs 批量获取配置
func (c *ConfigClient) GetConfigs(ctx context.Context, keys []string, env string) (map[string]string, error) {
	if len(keys) == 0 {
		return nil, c.err.New("配置键列表不能为空", nil).ValidWithCtx()
	}
	if env == "" {
		return nil, c.err.New("环境不能为空", nil).ValidWithCtx()
	}

	result := make(map[string]string, len(keys))
	for _, key := range keys {
		jsonStr, err := c.GetConfigJSON(ctx, key, env)
		if err != nil {
			// 如果某个配置不存在，记录错误但继续处理其他配置
			if errorc.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		result[key] = jsonStr
	}

	return result, nil
}
