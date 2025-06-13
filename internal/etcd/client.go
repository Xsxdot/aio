package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xsxdot/aio/app/config"
	"github.com/xsxdot/aio/pkg/common"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdClient 代表一个etcd客户端
type EtcdClient struct {
	*clientv3.Client
	logger *zap.Logger
}

func NewClient(config *config.EtcdConfig) (*EtcdClient, error) {
	logger := common.GetLogger().GetZapLogger("aio-etcd-client")
	endpoints := strings.Split(config.Endpoints, ",")
	clientConfig := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Duration(config.DialTimeout) * time.Second,
		Username:    config.Username,
		Password:    config.Password,
		Logger:      logger,
	}

	if config.AutoSyncEndpoints {
		clientConfig.AutoSyncInterval = 30 * time.Second
	}

	if config.TLS != nil && config.TLS.TLSEnabled {
		if !config.TLS.AutoTls {
			// 使用手动配置的证书
			if config.TLS.Cert != "" && config.TLS.Key != "" {
				tlsConfig, err := LoadTLSConfig(config.TLS.Cert, config.TLS.Key, config.TLS.TrustedCA)
				if err != nil {
					return nil, fmt.Errorf("加载TLS配置失败: %v", err)
				}
				clientConfig.TLS = tlsConfig
				logger.Info("已配置客户端手动TLS证书",
					zap.Strings("endpoints", clientConfig.Endpoints),
					zap.String("cert", config.TLS.Cert),
					zap.String("ca", config.TLS.TrustedCA))
			} else {
				logger.Warn("客户端TLS已启用但未提供证书文件，可能导致连接失败")
			}
		} else {
			clientConfig.TLS = &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			}
			logger.Info("客户端TLS已启用，使用自动TLS连接",
				zap.Strings("endpoints", clientConfig.Endpoints))
		}
	} else {
		logger.Info("客户端TLS未启用，使用不加密连接",
			zap.Strings("endpoints", clientConfig.Endpoints))
	}

	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建etcd客户端失败: %v", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.DialTimeout)*time.Second)
	defer cancel()

	_, err = client.Status(ctx, endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("连接etcd服务器失败: %v", err)
	}

	logger.Info("etcd客户端已连接", zap.Strings("endpoints", endpoints))

	return &EtcdClient{
		Client: client,
		logger: logger,
	}, nil
}

// LoadTLSConfig 加载TLS配置
func LoadTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书/密钥对失败: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if caFile != "" {
		caData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("读取CA证书失败: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("解析CA证书失败")
		}
		tlsConfig.RootCAs = caCertPool
		// 设置服务器CA
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
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
	return c.Client.Watch(ctx, key)
}

// WatchWithPrefix 监视具有相同前缀的所有键的变化
func (c *EtcdClient) WatchWithPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	withPrefix := clientv3.WithPrefix()
	client := c
	return client.Client.Watch(ctx, prefix, withPrefix)
}
