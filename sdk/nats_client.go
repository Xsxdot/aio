package sdk

import (
	"context"
	"fmt"
	consts "github.com/xsxdot/aio/app/const"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// NatsClientOptions NATS客户端选项
type NatsClientOptions struct {
	// 连接超时
	ConnectTimeout time.Duration
	// 用户名
	Username string
	// 密码
	Password string
	// 令牌
	Token string
}

// DefaultNatsOptions 默认NATS客户端选项
var DefaultNatsOptions = &NatsClientOptions{
	ConnectTimeout: 5 * time.Second,
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
		nats.Name("aio-sdk-client"),
		nats.Timeout(s.options.ConnectTimeout),
	}

	// 添加认证选项
	if s.options.Username != "" && s.options.Password != "" {
		opts = append(opts, nats.UserInfo(s.options.Username, s.options.Password))
	} else if s.options.Token != "" {
		opts = append(opts, nats.Token(s.options.Token))
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
