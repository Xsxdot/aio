package client

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/utils"
	"os"
	"time"
)

type ServiceInfo struct {
	// 服务名称
	Name string
	// 服务ID
	ID string
	// 服务端口
	Port int
	// 服务元数据
	Metadata map[string]string
	LocalIP  string
	PublicIP string
}

type ProtocolOptions struct {
	servers           []string
	ClientID          string
	ClientSecret      string
	ConnectionTimeout time.Duration
	RetryCount        int
	RetryInterval     time.Duration
}

type EtcdOptions struct {
	// 连接超时时间
	ConnectTimeout time.Duration
}

var DefaultEtcdOptions = &EtcdOptions{
	ConnectTimeout: 5 * time.Second,
}

// NatsClientOptions NATS客户端选项
type NatsClientOptions struct {
	// 连接超时时间
	ConnectTimeout time.Duration
	// 重连等待时间
	ReconnectWait time.Duration
	// 最大重连次数
	MaxReconnects int
	// 是否启用JetStream
	UseJetStream bool
	// 连接错误回调
	ErrorCallback func(error)
}

var DefaultNatsOptions = &NatsClientOptions{
	ConnectTimeout: 5 * time.Second,
	ReconnectWait:  3 * time.Second,
	MaxReconnects:  5,
	UseJetStream:   false,
}

// RedisClientOptions Redis客户端选项
type RedisClientOptions struct {
	// 连接超时
	ConnTimeout time.Duration
	// 读取超时
	ReadTimeout time.Duration
	// 写入超时
	WriteTimeout time.Duration
	// 数据库密码
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

var DefaultRedisOptions = &RedisClientOptions{
	ConnTimeout:   5 * time.Second,
	ReadTimeout:   5 * time.Second,
	WriteTimeout:  5 * time.Second,
	DB:            0,
	MaxRetries:    3,
	MinIdleConns:  10,
	PoolSize:      100,
	AutoReconnect: true,
}

type ServiceInfoBuilder struct {
	serviceInfo *ServiceInfo
}

type NodeInfo struct {
	// NodeID 节点ID
	NodeID string `json:"nodeId"`
	// IP 节点IP地址
	IP string `json:"ip"`
	// ProtocolPort 协议端口号
	ProtocolPort int `json:"protocolPort"`
	// CachePort 缓存端口号
	CachePort int `json:"cachePort"`
	// LastUpdate 最后更新时间
	LastUpdate time.Time `json:"lastUpdate"`
	// 是否是Leader节点
	IsLeader bool
	//连接ID
	ConnectionID string
}

func NewBuilder(id, name string, port int) *ServiceInfoBuilder {
	info := &ServiceInfo{
		Name:     name,
		ID:       id,
		Port:     port,
		Metadata: map[string]string{},
		LocalIP:  utils.GetLocalIP(),
		PublicIP: utils.GetPublicIP(),
	}
	if info.ID == "" {
		info.ID = GenerateStableServiceID(info)
	}
	return &ServiceInfoBuilder{
		serviceInfo: info,
	}
}

type ProtocolBuilder struct {
	serviceInfo     *ServiceInfo
	protocolOptions *ProtocolOptions
}

func (b *ServiceInfoBuilder) WithProtocolOptions(options *ProtocolOptions) *ProtocolBuilder {
	return &ProtocolBuilder{
		serviceInfo:     b.serviceInfo,
		protocolOptions: options,
	}
}

func (b *ServiceInfoBuilder) WithDefaultProtocolOptions(servers []string, clientId string, clientSecret string) *ProtocolBuilder {
	return &ProtocolBuilder{
		serviceInfo: b.serviceInfo,
		protocolOptions: &ProtocolOptions{
			servers:           servers,
			ClientID:          clientId,
			ClientSecret:      clientSecret,
			ConnectionTimeout: 5 * time.Second,
			RetryCount:        5,
			RetryInterval:     3 * time.Second,
		},
	}
}

type EtcdBuilder struct {
	serviceInfo     *ServiceInfo
	protocolOptions *ProtocolOptions
	etcdOptions     *EtcdOptions
}

func (b *ProtocolBuilder) WithEtcdOptions(options *EtcdOptions) *EtcdBuilder {
	return &EtcdBuilder{
		serviceInfo:     b.serviceInfo,
		protocolOptions: b.protocolOptions,
		etcdOptions:     options,
	}
}

type ClientOptions struct {
	serviceInfo     *ServiceInfo
	protocolOptions *ProtocolOptions
	etcdOptions     *EtcdOptions
}

func (b *EtcdBuilder) Build() *ClientOptions {
	return &ClientOptions{
		serviceInfo:     b.serviceInfo,
		protocolOptions: b.protocolOptions,
		etcdOptions:     b.etcdOptions,
	}
}

// GenerateStableServiceID 根据服务的唯一特征生成稳定的服务ID
func GenerateStableServiceID(service *ServiceInfo) string {
	// 获取主机名（如果可能）
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	// 组合唯一标识符
	uniqueInfo := struct {
		Hostname string            `json:"hostname"`
		Name     string            `json:"name"`
		Address  string            `json:"address"`
		Port     int               `json:"port"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}{
		Hostname: hostname,
		Name:     service.Name,
		Address:  service.LocalIP,
		Port:     service.Port,
	}

	// 序列化并计算哈希
	data, err := json.Marshal(uniqueInfo)
	if err != nil {
		// 如果序列化失败，使用简单的字符串连接
		data = []byte(fmt.Sprintf("%s:%s:%s:%d", hostname, service.Name, service.LocalIP, service.Port))
	}

	// 计算MD5哈希
	hash := md5.Sum(data)
	// 转换为十六进制字符串
	return hex.EncodeToString(hash[:])
}
