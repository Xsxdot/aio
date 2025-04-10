package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"

	"github.com/redis/go-redis/v9"
)

// RedisClientOptions Redis客户端选项
type RedisClientOptions struct {
	// 连接超时
	ConnTimeout time.Duration
	// 读取超时
	ReadTimeout time.Duration
	// 写入超时
	WriteTimeout time.Duration
	// 是否使用配置中心的配置信息
	UseConfigCenter bool
	// 以下字段仅在不使用配置中心时有效
	// 密码
	Password string
	// 数据库索引
	DB int
	// 最大重试次数
	MaxRetries int
	// 最小空闲连接数
	MinIdleConns int
	// 连接池大小
	PoolSize int
	// 是否自动重连接到主节点
	AutoReconnect bool
}

// DefaultRedisOptions 默认Redis客户端选项
var DefaultRedisOptions = &RedisClientOptions{
	ConnTimeout:     3 * time.Second,
	ReadTimeout:     3 * time.Second,
	WriteTimeout:    3 * time.Second,
	UseConfigCenter: true, // 默认使用配置中心
	Password:        "",
	DB:              0,
	MaxRetries:      3,
	MinIdleConns:    5,
	PoolSize:        10,
	AutoReconnect:   true,
}

// RedisClient Redis客户端，简单包装go-redis客户端
type RedisClient struct {
	*redis.Client
	// 保护Client替换操作的互斥锁
	mutex sync.RWMutex
}

// SetClient 设置内部的redis客户端
func (c *RedisClient) SetClient(client *redis.Client) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Client = client
}

// GetLockedClient 获取当前客户端并锁定读取
// 返回客户端和一个释放锁的函数
func (c *RedisClient) GetLockedClient() (*redis.Client, func()) {
	c.mutex.RLock()
	return c.Client, c.mutex.RUnlock
}

// RedisService Redis服务组件
type RedisService struct {
	// SDK客户端引用
	client *Client
	// Redis客户端选项
	options *RedisClientOptions
	// Redis客户端实例
	redisClient *RedisClient
	// 保护redisClient的互斥锁
	mutex sync.RWMutex
	// 最后一次连接的主节点ID
	lastLeaderID string
}

// NewRedisService 创建新的Redis服务组件
func NewRedisService(client *Client, options *RedisClientOptions) *RedisService {
	if options == nil {
		options = DefaultRedisOptions
	}

	service := &RedisService{
		client:       client,
		options:      options,
		redisClient:  &RedisClient{}, // 创建一个空的RedisClient
		lastLeaderID: "",
	}

	// 注册主节点变更处理函数
	if options.AutoReconnect {
		client.OnLeaderChange(service.handleLeaderChange)
	}

	return service
}

// GetClientConfigFromCenter 从配置中心获取Redis客户端配置
func (s *RedisService) GetClientConfigFromCenter(ctx context.Context) (*config.ClientConfigFixedValue, error) {
	// 从配置中心获取客户端配置
	configKey := fmt.Sprintf("%s%s", consts.ClientConfigPrefix, "redis") // 使用redis组件名

	// 使用 GetConfigWithStruct 方法直接获取并反序列化为结构体
	// 该方法会自动处理加密字段的解密
	var config config.ClientConfigFixedValue
	err := s.client.Config.GetConfigWithStruct(ctx, configKey, &config)
	if err != nil {
		return nil, fmt.Errorf("从配置中心获取Redis客户端配置失败: %w", err)
	}

	return &config, nil
}

// handleLeaderChange 处理主节点变更事件
func (s *RedisService) handleLeaderChange(oldLeader, newLeader *NodeInfo) {
	// 如果没有新的主节点，仅记录状态，不关闭连接
	if newLeader == nil {
		s.mutex.Lock()
		s.lastLeaderID = ""
		s.mutex.Unlock()
		return
	}

	// 如果主节点未变化，无需重新连接
	if s.lastLeaderID == newLeader.NodeID {
		return
	}

	// 检查新主节点是否有缓存端口配置
	if newLeader.CachePort <= 0 {
		fmt.Printf("新主节点 %s 未配置缓存端口，无法连接Redis\n", newLeader.NodeID)
		return
	}

	// 尝试从配置中心获取密码
	password := s.options.Password
	if s.options.UseConfigCenter {
		ctx, cancel := context.WithTimeout(context.Background(), s.options.ConnTimeout)
		defer cancel()

		config, err := s.GetClientConfigFromCenter(ctx)
		if err == nil && config.Password != "" {
			password = config.Password
		} else {
			fmt.Printf("从配置中心获取Redis密码失败，将使用传入的密码: %v\n", err)
		}
	}

	// 创建新的Redis客户端实例
	addr := fmt.Sprintf("%s:%d", newLeader.IP, newLeader.CachePort)
	newClient := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           s.options.DB,
		DialTimeout:  s.options.ConnTimeout,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
		MaxRetries:   s.options.MaxRetries,
		MinIdleConns: s.options.MinIdleConns,
		PoolSize:     s.options.PoolSize,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), s.options.ConnTimeout)
	defer cancel()
	_, err := newClient.Ping(ctx).Result()
	if err != nil {
		newClient.Close()
		fmt.Printf("连接到主节点 %s 的Redis服务器失败: %v\n", newLeader.NodeID, err)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 如果redisClient不存在，创建一个新的
	if s.redisClient == nil {
		s.redisClient = &RedisClient{Client: newClient}
	} else {
		// 替换内部的redis.Client
		// 先暂存旧客户端以便后续关闭
		oldClient := s.redisClient.Client
		// 设置新客户端
		s.redisClient.SetClient(newClient)
		// 异步关闭旧客户端（不阻塞当前操作，避免影响用户体验）
		if oldClient != nil {
			go func(c *redis.Client) {
				// 给足够的时间让现有操作完成
				time.Sleep(10 * time.Second)
				c.Close()
			}(oldClient)
		}
	}

	s.lastLeaderID = newLeader.NodeID
	fmt.Printf("已成功连接到主节点 %s 的Redis服务器\n", newLeader.NodeID)
}

// Get 获取Redis客户端实例
func (s *RedisService) Get() (*RedisClient, error) {
	s.mutex.RLock()

	// 如果redisClient存在且已初始化，直接返回
	if s.redisClient != nil && s.redisClient.Client != nil {
		defer s.mutex.RUnlock()
		return s.redisClient, nil
	}
	s.mutex.RUnlock()

	// 需要新建连接
	leaderInfo, err := s.client.GetLeaderInfo(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取主节点信息失败: %w", err)
	}

	if leaderInfo.CachePort <= 0 {
		return nil, fmt.Errorf("主节点 %s 未配置缓存端口", leaderInfo.NodeID)
	}

	// 尝试从配置中心获取密码
	password := s.options.Password
	if s.options.UseConfigCenter {
		ctx, cancel := context.WithTimeout(context.Background(), s.options.ConnTimeout)
		defer cancel()

		config, err := s.GetClientConfigFromCenter(ctx)
		if err == nil && config.Password != "" {
			password = config.Password
		} else {
			fmt.Printf("从配置中心获取Redis密码失败，将使用传入的密码: %v\n", err)
		}
	}

	// 创建新的Redis客户端实例
	addr := fmt.Sprintf("%s:%d", leaderInfo.IP, leaderInfo.CachePort)
	newClient := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           s.options.DB,
		DialTimeout:  s.options.ConnTimeout,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
		MaxRetries:   s.options.MaxRetries,
		MinIdleConns: s.options.MinIdleConns,
		PoolSize:     s.options.PoolSize,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), s.options.ConnTimeout)
	defer cancel()
	_, err = newClient.Ping(ctx).Result()
	if err != nil {
		newClient.Close()
		return nil, fmt.Errorf("连接到主节点的Redis服务器失败: %w", err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 再次检查是否已经被其他goroutine初始化
	if s.redisClient != nil && s.redisClient.Client != nil {
		// 另一个goroutine已经创建了连接，关闭我们刚创建的
		newClient.Close()
		return s.redisClient, nil
	}

	// 如果redisClient为nil，创建一个新的
	if s.redisClient == nil {
		s.redisClient = &RedisClient{Client: newClient}
	} else {
		// 否则只替换内部的redis.Client
		s.redisClient.SetClient(newClient)
	}

	s.lastLeaderID = leaderInfo.NodeID
	return s.redisClient, nil
}
