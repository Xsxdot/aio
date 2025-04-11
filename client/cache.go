package client

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"
)

type CacheService struct {
	redisClient *RedisClient
	client      *Client
	options     *RedisClientOptions
	log         *zap.Logger
}

type RedisClient struct {
	*redis.Client
}

func NewCacheClient(c *Client, options *RedisClientOptions) *CacheService {
	serv := &CacheService{
		client:  c,
		options: options,
		log:     common.GetLogger().GetZapLogger("aio-nats-client"),
	}
	c.RegisterLeaderChangeHandle(serv.ChangeLeaderInfo)
	return serv
}

func (s *CacheService) GetOriginClient(ctx context.Context) (*RedisClient, error) {
	options, err := s.GetClientConfigFromCenter(ctx)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)
	s.redisClient = &RedisClient{
		Client: client,
	}
	return s.redisClient, nil
}

// GetClientConfigFromCenter 从配置中心获取Cache客户端配置
func (s *CacheService) GetClientConfigFromCenter(ctx context.Context) (*redis.Options, error) {
	if s.client.leaderInfo == nil {
		return nil, fmt.Errorf("没有可用的 Cache 服务节点")
	}

	// 从配置中心获取客户端配置
	configKey := fmt.Sprintf("%s%s", consts.ClientConfigPrefix, consts.ComponentCacheServer)

	// 使用 GetConfigWithStruct 方法直接获取并反序列化为结构体
	// 该方法会自动处理加密字段的解密
	var config config.ClientConfigFixedValue
	err := s.client.Config.GetConfigJSONParse(ctx, configKey, &config)
	if err != nil {
		return nil, fmt.Errorf("从配置中心获取Cache客户端配置失败: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", s.client.leaderInfo.IP, s.client.leaderInfo.CachePort)
	return &redis.Options{
		Addr:         addr,
		Password:     config.Password,
		DB:           s.options.DB,
		DialTimeout:  s.options.ConnTimeout,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
		MaxRetries:   s.options.MaxRetries,
		MinIdleConns: s.options.MinIdleConns,
		PoolSize:     s.options.PoolSize,
	}, nil

}

func (s *CacheService) ChangeLeaderInfo(info *NodeInfo) {
	options, err := s.GetClientConfigFromCenter(context.Background())
	if err != nil {
		s.log.Error("切换主节点时从配置中心获取Cache客户端配置失败", zap.Error(err))
		return
	}

	client := redis.NewClient(options)
	s.redisClient.Client = client
	return
}
