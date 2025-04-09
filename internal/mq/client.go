package mq

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

var (
	// GlobalNatsClient 全局的NATS客户端实例
	GlobalNatsClient *NatsClient

	// ErrNotConnected 表示客户端未连接错误
	ErrNotConnected = errors.New("NATS客户端未连接")
)

// NatsClient 代表一个NATS客户端
type NatsClient struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	config *ClientConfig
	logger *zap.Logger
}

// ClientConfig 代表NATS客户端配置
type ClientConfig struct {
	// 服务器地址，如 "nats://localhost:4222"
	Servers []string
	// 连接超时时间
	ConnectTimeout time.Duration
	// 重连等待时间
	ReconnectWait time.Duration
	// 最大重连次数
	MaxReconnects int
	// 用户名
	Username string
	// 密码
	Password string
	// 安全连接配置
	TLS *TLSConfig
	// 是否启用JetStream
	UseJetStream bool
	// 连接错误回调
	ErrorCallback func(error)
}

// TLSConfig 表示TLS配置
type TLSConfig struct {
	// 客户端证书文件路径
	CertFile string
	// 客户端密钥文件路径
	KeyFile string
	// CA证书文件路径，用于验证服务器证书
	TrustedCAFile string
	// 是否跳过服务器证书验证（仅用于测试环境）
	InsecureSkipVerify bool
}

// NewDefaultClientConfig 创建默认的客户端配置
func NewDefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Servers:        []string{"nats://localhost:4222"},
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  10,
		UseJetStream:   true,
	}
}

// NewNatsClient 创建一个新的NATS客户端
func NewNatsClient(config *ClientConfig, logger *zap.Logger) (*NatsClient, error) {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("创建日志记录器失败: %v", err)
		}
	}

	// 创建NATS连接选项
	opts := []nats.Option{
		nats.Timeout(config.ConnectTimeout),
		nats.ReconnectWait(config.ReconnectWait),
		nats.MaxReconnects(config.MaxReconnects),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS连接断开", zap.Error(err))
				if config.ErrorCallback != nil {
					config.ErrorCallback(err)
				}
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("NATS已重新连接")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Info("NATS连接已关闭")
		}),
	}

	// 配置认证
	if config.Username != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.Username, config.Password))
	}

	// 配置TLS
	if config.TLS != nil {
		tlsConfig, err := LoadClientTLSConfig(
			config.TLS.CertFile,
			config.TLS.KeyFile,
			config.TLS.TrustedCAFile,
			config.TLS.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("加载TLS配置失败: %v", err)
		}
		opts = append(opts, nats.Secure(tlsConfig))
		logger.Info("已启用TLS安全连接")
	}

	// 连接到NATS服务器
	var urls string
	if len(config.Servers) > 0 {
		urls = strings.Join(config.Servers, ",")
	} else {
		urls = nats.DefaultURL
	}
	conn, err := nats.Connect(urls, opts...)
	if err != nil {
		return nil, fmt.Errorf("连接NATS服务器失败: %v", err)
	}

	logger.Info("已连接到NATS服务器", zap.Strings("servers", conn.Servers()))

	client := &NatsClient{
		conn:   conn,
		config: config,
		logger: logger,
	}

	// 初始化JetStream（如果启用）
	if config.UseJetStream {
		js, err := conn.JetStream()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("初始化JetStream失败: %v", err)
		}
		client.js = js
		logger.Info("已启用JetStream支持")
	}

	return client, nil
}

// Close 关闭NATS客户端连接
func (c *NatsClient) Close() {
	if c.conn != nil {
		c.logger.Info("关闭NATS客户端连接")
		c.conn.Close()
		c.conn = nil
		c.js = nil
	}
}

// GetConn 获取底层的NATS连接
func (c *NatsClient) GetConn() *nats.Conn {
	return c.conn
}

// GetJetStream 获取JetStream上下文
func (c *NatsClient) GetJetStream() nats.JetStreamContext {
	return c.js
}

// Publish 发布消息到指定主题
func (c *NatsClient) Publish(subject string, data []byte) error {
	if c.conn == nil {
		return ErrNotConnected
	}
	return c.conn.Publish(subject, data)
}

// PublishAsync 异步发布消息到指定主题
func (c *NatsClient) PublishAsync(subject string, data []byte) (nats.PubAckFuture, error) {
	if c.js == nil {
		return nil, errors.New("JetStream未启用")
	}
	return c.js.PublishAsync(subject, data)
}

// Subscribe 订阅指定主题
func (c *NatsClient) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if c.conn == nil {
		return nil, ErrNotConnected
	}
	return c.conn.Subscribe(subject, handler)
}

// QueueSubscribe 创建队列订阅
func (c *NatsClient) QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if c.conn == nil {
		return nil, ErrNotConnected
	}
	return c.conn.QueueSubscribe(subject, queue, handler)
}

// JetStreamSubscribe 创建JetStream订阅
func (c *NatsClient) JetStreamSubscribe(subject string, handler nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	if c.js == nil {
		return nil, errors.New("JetStream未启用")
	}
	return c.js.Subscribe(subject, handler, opts...)
}

// JetStreamQueueSubscribe 创建JetStream队列订阅
func (c *NatsClient) JetStreamQueueSubscribe(subject, queue string, handler nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	if c.js == nil {
		return nil, errors.New("JetStream未启用")
	}
	return c.js.QueueSubscribe(subject, queue, handler, opts...)
}

// CreateStream 创建JetStream流
func (c *NatsClient) CreateStream(config *nats.StreamConfig) (*nats.StreamInfo, error) {
	if c.js == nil {
		return nil, errors.New("JetStream未启用")
	}
	return c.js.AddStream(config)
}

// Request 发送请求并等待响应
func (c *NatsClient) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	if c.conn == nil {
		return nil, ErrNotConnected
	}
	return c.conn.Request(subject, data, timeout)
}

// InitGlobalNatsClient 初始化全局NATS客户端
func InitGlobalNatsClient(config *ClientConfig, logger *zap.Logger) error {
	var err error
	GlobalNatsClient, err = NewNatsClient(config, logger)
	return err
}

// GetGlobalNatsClient 获取全局NATS客户端实例
func GetGlobalNatsClient() *NatsClient {
	return GlobalNatsClient
}

// CloseGlobalNatsClient 关闭全局NATS客户端
func CloseGlobalNatsClient() {
	if GlobalNatsClient != nil {
		GlobalNatsClient.Close()
		GlobalNatsClient = nil
	}
}

// WithClientCredentials 为客户端配置添加认证的选项函数
func WithClientCredentials(username, password string) func(*ClientConfig) {
	return func(config *ClientConfig) {
		config.Username = username
		config.Password = password
	}
}

// WithClientTLS 为客户端配置添加TLS的选项函数
func WithClientTLS(certFile, keyFile, caFile string) func(*ClientConfig) {
	return func(config *ClientConfig) {
		config.TLS = &TLSConfig{
			CertFile:      certFile,
			KeyFile:       keyFile,
			TrustedCAFile: caFile,
		}
	}
}

// WithInsecureSkipVerify 设置客户端跳过服务器证书验证（仅用于测试）
func WithInsecureSkipVerify() func(*ClientConfig) {
	return func(config *ClientConfig) {
		if config.TLS == nil {
			config.TLS = &TLSConfig{}
		}
		config.TLS.InsecureSkipVerify = true
	}
}
