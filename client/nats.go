package client

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/mq"
	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"
)

type NatsClient struct {
	natsClient *mq.NatsClient
	client     *Client
	options    *NatsClientOptions
	log        *zap.Logger
}

func NewNatsClient(c *Client, options *NatsClientOptions) *NatsClient {
	return &NatsClient{
		client:  c,
		options: options,
		log:     common.GetLogger().GetZapLogger("aio-nats-client"),
	}
}

func (s *NatsClient) GetOriginClient(ctx context.Context) (*nats.Conn, error) {
	cfg, err := s.GetClientConfigFromCenter(ctx)
	if err != nil {
		return nil, err
	}

	client, err := mq.NewNatsClient(cfg, s.log)
	if err != nil {
		return nil, err
	}

	return client.GetConn(), nil
}

// GetClientConfigFromCenter 从配置中心获取NATS客户端配置
func (s *NatsClient) GetClientConfigFromCenter(ctx context.Context) (*mq.ClientConfig, error) {
	// 从配置中心获取客户端配置
	configKey := fmt.Sprintf("%s%s", consts.ClientConfigPrefix, consts.ComponentMQServer)

	// 使用 GetConfigWithStruct 方法直接获取并反序列化为结构体
	// 该方法会自动处理加密字段的解密
	var config config.ClientConfigFixedValue
	err := s.client.Config.GetConfigJSONParse(ctx, configKey, &config)
	if err != nil {
		return nil, fmt.Errorf("从配置中心获取NATS客户端配置失败: %w", err)
	}

	// 获取 NATS 服务节点信息
	services, err := s.client.Discovery.Discover(ctx, consts.ComponentMQServer)
	if err != nil {
		return nil, fmt.Errorf("发现 NATS 服务失败: %w", err)
	}

	// 检查是否有可用的 NATS 服务节点
	if len(services) == 0 {
		return nil, fmt.Errorf("没有可用的 NATS 服务节点")
	}

	s.log.Info("发现 NATS 服务节点", zap.Int("count", len(services)))

	// 构建端点列表
	endpoints := make([]string, 0, len(services))
	for _, service := range services {
		// 构建端点地址
		endpoint := fmt.Sprintf("nats://%s:%d", service.Address, service.Port)
		endpoints = append(endpoints, endpoint)
		s.log.Info("添加 NATS 端点", zap.String("endpoint", endpoint))
	}

	m := &mq.ClientConfig{
		Servers:        endpoints,
		Username:       config.Username,
		Password:       config.Password,
		UseJetStream:   s.options.UseJetStream,
		ConnectTimeout: s.options.ConnectTimeout,
		ReconnectWait:  s.options.ReconnectWait,
		MaxReconnects:  s.options.MaxReconnects,
		ErrorCallback:  s.options.ErrorCallback,
	}

	if config.EnableTls {
		m.TLS = &mq.TLSConfig{
			CertFile:      config.Cert,
			KeyFile:       config.Key,
			TrustedCAFile: config.TrustedCAFile,
		}
	}

	return m, nil

}
