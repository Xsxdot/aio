package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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
		tlsConfig := &mq.TLSConfig{}

		// 使用证书内容创建临时文件
		if config.CertContent != "" && config.KeyContent != "" && config.CATrustedContent != "" {
			// 创建临时文件夹
			tmpDir, err := os.MkdirTemp("", "nats-certs-")
			if err != nil {
				s.log.Warn("创建临时证书目录失败", zap.Error(err))
			} else {
				// 创建证书文件
				certFile := filepath.Join(tmpDir, "cert.pem")
				keyFile := filepath.Join(tmpDir, "key.pem")
				caFile := filepath.Join(tmpDir, "ca.pem")

				// 写入证书内容
				if err := os.WriteFile(certFile, []byte(config.CertContent), 0600); err == nil {
					tlsConfig.CertFile = certFile
				} else {
					s.log.Warn("写入证书文件失败", zap.Error(err))
				}

				// 写入密钥内容
				if err := os.WriteFile(keyFile, []byte(config.KeyContent), 0600); err == nil {
					tlsConfig.KeyFile = keyFile
				} else {
					s.log.Warn("写入密钥文件失败", zap.Error(err))
				}

				// 写入CA证书内容
				if err := os.WriteFile(caFile, []byte(config.CATrustedContent), 0600); err == nil {
					tlsConfig.TrustedCAFile = caFile
				} else {
					s.log.Warn("写入CA证书文件失败", zap.Error(err))
				}

				// 设置清理函数，在程序退出时删除临时文件
				finalizer := func() {
					os.RemoveAll(tmpDir)
				}

				// 注册清理函数
				runtime.SetFinalizer(tlsConfig, func(_ *mq.TLSConfig) {
					finalizer()
				})
			}
		} else if config.Cert != "" && config.Key != "" && config.TrustedCAFile != "" {
			// 如果没有证书内容但有证书路径，使用路径
			tlsConfig.CertFile = config.Cert
			tlsConfig.KeyFile = config.Key
			tlsConfig.TrustedCAFile = config.TrustedCAFile
		}

		m.TLS = tlsConfig
	}

	return m, nil
}
