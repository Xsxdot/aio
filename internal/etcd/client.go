package etcd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xsxdot/aio/app/config"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdClient 代表一个etcd客户端
type EtcdClient struct {
	*clientv3.Client
	logger *zap.Logger
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
	TLS *config.TLSConfig
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
		logger.Info("正在连接etcd服务器...",
			zap.Strings("endpoints", clientConfig.Endpoints),
			zap.String("证书路径", config.TLS.Cert),
			zap.String("CA证书路径", config.TLS.TrustedCA))
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
		Client: client,
		logger: logger,
	}, nil
}

// setupRootUser 设置etcd根用户
func (c *EtcdClient) setupRootUser(username, password string) error {

	// 创建一个上下文，用于API调用
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 检查认证是否已启用
	authStatus, err := c.AuthStatus(ctx)
	if err != nil {
		return fmt.Errorf("获取认证状态失败: %v", err)
	}

	if authStatus.Enabled {
		c.logger.Info("认证已经启用，跳过设置")
		return nil
	}

	// 添加根用户
	_, err = c.UserAdd(ctx, username, password)
	if err != nil {
		// 如果用户已存在，忽略错误
		if strings.Contains(err.Error(), "user name already exists") {
			c.logger.Info("用户已存在", zap.String("username", username))
		} else {
			return fmt.Errorf("添加用户失败: %v", err)
		}
	}

	// 为根用户授予root角色
	_, err = c.UserGrantRole(ctx, username, "root")
	if err != nil {
		return fmt.Errorf("授予root角色失败: %v", err)
	}

	// 启用认证
	_, err = c.AuthEnable(ctx)
	if err != nil {
		return fmt.Errorf("启用认证失败: %v", err)
	}

	c.logger.Info("成功启用认证并设置根用户", zap.String("username", username))
	return nil
}

// Close 关闭etcd客户端
func (c *EtcdClient) Close() {
	if c != nil {
		c.Client.Close()
		c.logger.Info("etcd客户端已关闭")
	}
}

// Put 放置键值对
func (c *EtcdClient) Put(ctx context.Context, key, value string) error {
	_, err := c.Client.Put(ctx, key, value)
	if err != nil {
		c.logger.Error("放置键值对失败", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// Get 获取键对应的值
func (c *EtcdClient) Get(ctx context.Context, key string) (string, error) {
	resp, err := c.Client.Get(ctx, key)
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
	resp, err := c.Client.Get(ctx, prefix, clientv3.WithPrefix())
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
	_, err := c.Client.Delete(ctx, key)
	if err != nil {
		c.logger.Error("删除键失败", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// DeleteWithPrefix 删除具有相同前缀的所有键
func (c *EtcdClient) DeleteWithPrefix(ctx context.Context, prefix string) error {
	_, err := c.Client.Delete(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		c.logger.Error("删除前缀键失败", zap.String("prefix", prefix), zap.Error(err))
		return err
	}
	return nil
}

// Watch 监视键的变化
func (c *EtcdClient) Watch(ctx context.Context, key string) clientv3.WatchChan {
	return c.Watch(ctx, key)
}

// WatchWithPrefix 监视具有相同前缀的所有键的变化
func (c *EtcdClient) WatchWithPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	withPrefix := clientv3.WithPrefix()
	client := c
	return client.Client.Watch(ctx, prefix, withPrefix)
}
