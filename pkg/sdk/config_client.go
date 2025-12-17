package sdk

import (
	"context"

	configpb "xiaozhizhang/system/config/api/proto"

	"google.golang.org/grpc"
)

// ConfigClient 配置中心客户端
// 封装配置查询端 RPC（GetConfig/BatchGetConfigs）
type ConfigClient struct {
	service configpb.ConfigServiceClient
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

