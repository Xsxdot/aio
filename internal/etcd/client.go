package etcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

var (
	// GlobalEtcdClient 全局的etcd客户端实例
	GlobalEtcdClient *EtcdClient
)

// EtcdClient 代表一个etcd客户端
type EtcdClient struct {
	client *clientv3.Client
	logger *zap.Logger
}

// JWTConfig 表示JWT配置
type JWTConfig struct {
	AccessTokenExpiry time.Duration `yaml:"access_token_expiry" json:"access_token_expiry"`
	Issuer            string        `yaml:"issuer" json:"issuer"`
	Audience          string        `yaml:"audience" json:"audience"`
	PublicKey         string        `yaml:"public_key" json:"public_key"`   // JWT公钥内容路径
	PrivateKey        string        `yaml:"private_key" json:"private_key"` // JWT私钥内容路径
	SignMethod        string        `yaml:"sign_method" json:"sign_method"` // JWT签名方法，如RS256
}

// ClientConfig 代表etcd客户端配置
type ClientConfig struct {
	// 端点列表，如 ["localhost:2379", "localhost:22379", "localhost:32379"]
	Endpoints []string
	// 拨号超时时间
	DialTimeout time.Duration
	// 用户名
	Username string
	// 密码
	Password string
	// 自动同步端点
	AutoSyncEndpoints bool
	// 安全连接配置
	TLS *TLSConfig
}

// TLSConfig 表示TLS配置
type TLSConfig struct {
	TLSEnabled bool   `yaml:"tls_enabled" json:"tls_enabled,omitempty"`
	AutoTls    bool   `yaml:"auto_tls" json:"auto_tls,omitempty"`
	Cert       string `yaml:"cert_file" json:"cert,omitempty"`
	Key        string `yaml:"key_file" json:"key,omitempty"`
	TrustedCA  string `yaml:"trusted_ca_file" json:"trusted_ca,omitempty"`
}

type UserAuthConfig struct {
	RootUsername string `yaml:"root_username" json:"root_username,omitempty"` // 根用户名
	RootPassword string `yaml:"root_password" json:"root_password,omitempty"` // 根用户密码
}

// NewDefaultClientConfig 创建默认的客户端配置
func NewDefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Endpoints:         []string{"localhost:2379"},
		DialTimeout:       5 * time.Second,
		AutoSyncEndpoints: false,
	}
}

// NewEtcdClient 创建一个新的etcd客户端
func NewEtcdClient(config *ClientConfig, logger *zap.Logger) (*EtcdClient, error) {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("创建日志记录器失败: %v", err)
		}
	}

	clientConfig := clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: config.DialTimeout,
		Username:    config.Username,
		Password:    config.Password,
		Logger:      logger,
	}

	if config.AutoSyncEndpoints {
		clientConfig.AutoSyncInterval = 30 * time.Second
	}

	if config.TLS != nil {
		tlsConfig, err := LoadTLSConfig(config.TLS.Cert, config.TLS.Key, config.TLS.TrustedCA)
		if err != nil {
			return nil, fmt.Errorf("加载TLS配置失败: %v", err)
		}
		clientConfig.TLS = tlsConfig
	}

	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建etcd客户端失败: %v", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer cancel()

	_, err = client.Status(ctx, config.Endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("连接etcd服务器失败: %v", err)
	}

	logger.Info("etcd客户端已连接", zap.Strings("endpoints", config.Endpoints))
	return &EtcdClient{
		client: client,
		logger: logger,
	}, nil
}

// Close 关闭etcd客户端
func (c *EtcdClient) Close() {
	if c.client != nil {
		c.client.Close()
		c.logger.Info("etcd客户端已关闭")
	}
}

// GetClient 获取原始的etcd客户端
func (c *EtcdClient) GetClient() *clientv3.Client {
	return c.client
}

// Put 放置键值对
func (c *EtcdClient) Put(ctx context.Context, key, value string) error {
	_, err := c.client.Put(ctx, key, value)
	if err != nil {
		c.logger.Error("放置键值对失败", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// Get 获取键对应的值
func (c *EtcdClient) Get(ctx context.Context, key string) (string, error) {
	resp, err := c.client.Get(ctx, key)
	if err != nil {
		c.logger.Error("获取键值对失败", zap.String("key", key), zap.Error(err))
		return "", err
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("键不存在: %s", key)
	}

	return string(resp.Kvs[0].Value), nil
}

// GetWithPrefix 获取具有相同前缀的所有键值对
func (c *EtcdClient) GetWithPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		c.logger.Error("获取前缀键值对失败", zap.String("prefix", prefix), zap.Error(err))
		return nil, err
	}

	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}
	return result, nil
}

// Delete 删除键
func (c *EtcdClient) Delete(ctx context.Context, key string) error {
	_, err := c.client.Delete(ctx, key)
	if err != nil {
		c.logger.Error("删除键失败", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// DeleteWithPrefix 删除具有相同前缀的所有键
func (c *EtcdClient) DeleteWithPrefix(ctx context.Context, prefix string) error {
	_, err := c.client.Delete(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		c.logger.Error("删除前缀键失败", zap.String("prefix", prefix), zap.Error(err))
		return err
	}
	return nil
}

// Watch 监视键的变化
func (c *EtcdClient) Watch(ctx context.Context, key string) clientv3.WatchChan {
	return c.client.Watch(ctx, key)
}

// WatchWithPrefix 监视具有相同前缀的所有键的变化
func (c *EtcdClient) WatchWithPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	withPrefix := clientv3.WithPrefix()
	client := c.client
	return client.Watch(ctx, prefix, withPrefix)
}

// InitGlobalEtcdClient 初始化全局的etcd客户端
func InitGlobalEtcdClient(config *ClientConfig, logger *zap.Logger) error {
	if GlobalEtcdClient != nil {
		return fmt.Errorf("全局etcd客户端已经初始化")
	}

	client, err := NewEtcdClient(config, logger)
	if err != nil {
		return err
	}

	GlobalEtcdClient = client
	return nil
}

// GetGlobalEtcdClient 获取全局的etcd客户端实例
func GetGlobalEtcdClient() *EtcdClient {
	return GlobalEtcdClient
}

// CloseGlobalEtcdClient 关闭全局的etcd客户端
func CloseGlobalEtcdClient() {
	if GlobalEtcdClient != nil {
		GlobalEtcdClient.Close()
		GlobalEtcdClient = nil
	}
}
