package config

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Mode     string `yaml:"mode"`
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func InitRDB(redisConfig RedisConfig, proxyConfig ProxyConfig) *redis.Client {
	if redisConfig.Mode == "single" {
		opts := &redis.Options{
			Addr:     redisConfig.Host,
			Password: redisConfig.Password,
			DB:       redisConfig.DB,
		}

		// 如果启用代理，配置自定义dialer
		if proxyConfig.Enabled {
			dialer := proxyConfig.GetDialer()
			opts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}

		return redis.NewClient(opts)
	}

	failoverOpts := &redis.FailoverOptions{
		MasterName:       "mymaster",
		SentinelAddrs:    strings.Split(redisConfig.Host, ","),
		Password:         redisConfig.Password,
		SentinelPassword: redisConfig.Password,
		DB:               redisConfig.DB,
	}

	// 如果启用代理，配置自定义dialer
	if proxyConfig.Enabled {
		dialer := proxyConfig.GetDialer()
		failoverOpts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	}

	return redis.NewFailoverClient(failoverOpts)
}

func InitCache(rdb *redis.Client) *cache.Cache {
	return cache.New(&cache.Options{
		Redis:      rdb,
		LocalCache: cache.NewTinyLFU(5, time.Minute),
	})
}
