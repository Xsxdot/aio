package sdk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/mq"

	"github.com/nats-io/nats.go"
)

// NatsClientOptions NATS客户端选项
type NatsClientOptions struct {
	// 连接超时
	ConnectTimeout time.Duration
	// 是否使用配置中心的配置信息
	UseConfigCenter bool
	// 以下字段仅在不使用配置中心时有效
	Username string
	Password string
	Token    string
}

// DefaultNatsOptions 默认NATS客户端选项
var DefaultNatsOptions = &NatsClientOptions{
	ConnectTimeout:  5 * time.Second,
	UseConfigCenter: true, // 默认使用配置中心
}

// NatsService NATS服务组件
type NatsService struct {
	// SDK客户端引用
	client *Client
	// NATS客户端选项
	options *NatsClientOptions
}

// NewNatsService 创建新的NATS服务组件
func NewNatsService(client *Client, options *NatsClientOptions) *NatsService {
	if options == nil {
		options = DefaultNatsOptions
	}

	return &NatsService{
		client:  client,
		options: options,
	}
}

// GetClientConfigFromCenter 从配置中心获取NATS客户端配置
func (s *NatsService) GetClientConfigFromCenter(ctx context.Context) (*config.ClientConfigFixedValue, error) {
	// 从配置中心获取客户端配置
	configKey := fmt.Sprintf("%s%s", consts.ClientConfigPrefix, consts.ComponentMQServer)

	// 使用 GetConfigWithStruct 方法直接获取并反序列化为结构体
	// 该方法会自动处理加密字段的解密
	var config config.ClientConfigFixedValue
	err := s.client.Config.GetConfigWithStruct(ctx, configKey, &config)
	if err != nil {
		return nil, fmt.Errorf("从配置中心获取NATS客户端配置失败: %w", err)
	}

	return &config, nil
}

// GetClient 获取NATS客户端实例
func (s *NatsService) GetClient(ctx context.Context) (*nats.Conn, error) {
	// 获取 NATS 服务节点信息
	services, err := s.client.Discovery.DiscoverServices(ctx, consts.ComponentMQServer)
	if err != nil {
		return nil, fmt.Errorf("发现 NATS 服务失败: %w", err)
	}

	// 检查是否有可用的 NATS 服务节点
	if len(services) == 0 {
		return nil, fmt.Errorf("没有可用的 NATS 服务节点")
	}

	fmt.Printf("发现 NATS 服务节点: %d 个\n", len(services))

	// 构建NATS服务URL列表
	urls := make([]string, 0, len(services))
	for _, service := range services {
		// 检查是否有指定的NATS端口
		port := service.Port
		if natsPort, ok := service.Metadata["nats_port"]; ok {
			fmt.Sscanf(natsPort, "%d", &port)
		}

		// 构建NATS URL
		url := fmt.Sprintf("nats://%s:%d", service.Address, port)
		urls = append(urls, url)
		fmt.Printf("添加 NATS 服务器: %s\n", url)
	}

	// 创建连接选项
	opts := []nats.Option{
		nats.Timeout(s.options.ConnectTimeout),
	}

	var username, password string
	var tlsConfig *mq.TLSConfig

	// 从配置中心获取认证信息
	if s.options.UseConfigCenter {
		config, err := s.GetClientConfigFromCenter(ctx)
		if err != nil {
			fmt.Printf("从配置中心获取NATS配置失败，将使用传入的配置: %v\n", err)
			// 使用传入的配置作为备选
			username = s.options.Username
			password = s.options.Password
		} else {
			username = config.Username
			password = config.Password

			// 如果启用了TLS，设置TLS配置
			if config.EnableTls {
				tlsConfig = &mq.TLSConfig{
					CertFile:      config.Cert,
					KeyFile:       config.Key,
					TrustedCAFile: config.TrustedCAFile,
				}
			}
		}
	} else {
		// 使用传入的配置
		username = s.options.Username
		password = s.options.Password
	}

	// 添加认证选项
	if username != "" && password != "" {
		opts = append(opts, nats.UserInfo(username, password))
	} else if s.options.Token != "" {
		opts = append(opts, nats.Token(s.options.Token))
	}

	// 如果有TLS配置，添加TLS选项
	if tlsConfig != nil && tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
		// 加载TLS配置
		natsTC, err := mq.LoadClientTLSConfig(
			tlsConfig.CertFile,
			tlsConfig.KeyFile,
			tlsConfig.TrustedCAFile,
			tlsConfig.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("加载TLS配置失败: %w", err)
		}
		opts = append(opts, nats.Secure(natsTC))
	}

	// 连接到NATS服务器
	serverURL := strings.Join(urls, ",")
	nc, err := nats.Connect(serverURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("连接到NATS服务器失败: %w", err)
	}

	fmt.Printf("已成功连接到NATS服务器\n")
	return nc, nil
}

// Close 关闭NATS客户端，此简化版不需要特别的关闭操作
func (s *NatsService) Close() {
	// 简化版本不需要特别的关闭操作
}
