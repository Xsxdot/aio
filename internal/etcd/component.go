package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/common"
)

type Config struct {
	// 节点名称，必须唯一
	Name string `json:"name" yaml:"name"`
	// 数据目录
	DataDir    string `json:"data_dir" yaml:"data_dir"`
	BindIP     string `json:"bind_ip" yaml:"bind_ip"`
	LocalIp    string `json:"local_ip" yaml:"local_ip"`
	ClientPort int    `json:"client_port" yaml:"client_port"`
	PeerPort   int    `json:"peer_port" yaml:"peer_port"`
	// 初始集群配置，格式为: nodeName1=http://ip1:2380,nodeName2=http://ip2:2380
	InitialCluster string `json:"initial_cluster" yaml:"initial_cluster"`
	// 初始集群状态: "new" 或 "existing"
	InitialClusterState string `json:"initial_cluster_state" yaml:"initial_cluster_state"`
	// 初始集群令牌
	InitialClusterToken string `json:"initial_cluster_token" yaml:"initial_cluster_token"`
	AuthToken           string `json:"auth_token" yaml:"auth_token"`
	// 客户端TLS
	ClientTLSConfig TLSConfig `json:"client_tls_config" yaml:"client_tls_config"`
	ServerTLSConfig TLSConfig `json:"server_tls_config" yaml:"server_tls_config"`
	// JWT认证配置
	Jwt JWTConfig `json:"jwt" yaml:"jwt"`
	// 用户名密码认证配置
	User UserAuthConfig `json:"user" yaml:"user"`
}

func (c *Config) ToServerConfig() *ServerConfig {
	clientProto := "http://"
	serverProto := "http://"
	if c.ClientTLSConfig.TLSEnabled {
		clientProto = "https://"
	}
	if c.ServerTLSConfig.TLSEnabled {
		serverProto = "https://"
	}

	clientURLs := []string{clientProto + c.BindIP + ":" + fmt.Sprintf("%d", c.ClientPort)}
	peerURLs := []string{serverProto + c.BindIP + ":" + fmt.Sprintf("%d", c.PeerPort)}

	if c.BindIP != c.LocalIp {
		clientURLs = append(clientURLs, clientProto+c.LocalIp+":"+fmt.Sprintf("%d", c.ClientPort))
		peerURLs = append(peerURLs, serverProto+c.LocalIp+":"+fmt.Sprintf("%d", c.PeerPort))
	}

	serverConfig := &ServerConfig{
		Name:       c.Name,
		DataDir:    c.DataDir,
		ClientURLs: clientURLs,
		//ListenClientHttpUrls: []string{ip + "/v3"},
		PeerURLs:            peerURLs,
		InitialCluster:      fmt.Sprintf("%s=%s%s:%d", c.Name, serverProto, c.LocalIp, c.PeerPort),
		InitialClusterState: c.InitialClusterState,
		InitialClusterToken: c.InitialClusterToken,
		PeerTLSConfig:       c.ServerTLSConfig,
		ClientTLSConfig:     c.ClientTLSConfig,
		AuthToken:           c.AuthToken,
		JWT:                 c.Jwt,
		UserAuthConfig:      c.User,
	}
	return serverConfig
}

func (c *Config) ToClientConfig() (*ClientConfig, error) {
	proto := "http://"
	if c.ClientTLSConfig.TLSEnabled {
		proto = "https://"
	}
	ip := proto + c.BindIP + ":" + fmt.Sprintf("%d", c.ClientPort)
	return &ClientConfig{
		Endpoints:         []string{ip},
		DialTimeout:       5 * time.Second,
		Username:          c.User.RootUsername,
		Password:          c.User.RootPassword,
		AutoSyncEndpoints: true,
		TLS:               &c.ClientTLSConfig,
	}, nil
}

// NewDefaultConfig 创建默认的服务器配置
func NewDefaultConfig(config *config.BaseConfig) *Config {
	name := "aio-etcd-" + config.System.NodeId
	return &Config{
		Name:                name,
		DataDir:             filepath.Join(config.System.DataDir, "etcd"),
		BindIP:              config.Network.BindIP,
		LocalIp:             config.Network.LocalIp,
		ClientPort:          2379,
		PeerPort:            2380,
		InitialCluster:      fmt.Sprintf("%s=https://%s:2380", name, config.Network.LocalIp),
		InitialClusterState: "new",
		InitialClusterToken: "etcd-cluster",
		AuthToken:           "root",
		ClientTLSConfig:     TLSConfig{AutoTls: true},
		ServerTLSConfig:     TLSConfig{AutoTls: true},
		Jwt:                 JWTConfig{},
		User: UserAuthConfig{
			RootUsername: "root",
			RootPassword: "123456",
		},
	}
}

type EtcdComponent struct {
	Server *EtcdServer
	Client *EtcdClient
	cfg    *Config
	log    *common.Logger
	status consts.ComponentStatus
	mu     sync.Mutex
}

func (c *EtcdComponent) RegisterMetadata() (bool, int, map[string]string) {
	return true, c.cfg.ClientPort, map[string]string{
		"client_port": strconv.Itoa(c.cfg.ClientPort),
		"peer_port":   strconv.Itoa(c.cfg.PeerPort),
	}
}

func (c *EtcdComponent) Name() string {
	return consts.ComponentEtcd
}

func (c *EtcdComponent) Init(config *config.BaseConfig, body []byte) error {
	serverConfig := NewDefaultConfig(config)
	c.cfg = serverConfig

	err := json.Unmarshal(body, serverConfig)
	if err != nil {
		return err
	}

	c.status = consts.StatusInitialized

	return nil
}

func (c *EtcdComponent) Restart(ctx context.Context) error {
	if err := c.Stop(ctx); err != nil {
		return err
	}
	return c.Start(ctx)
}
func NewEtcdComponent() *EtcdComponent {
	return &EtcdComponent{
		log:    common.GetLogger(),
		status: consts.StatusNotInitialized,
	}
}

func (c *EtcdComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == consts.StatusRunning {
		return nil
	} else if c.status == consts.StatusNotInitialized {
		return fmt.Errorf("组件未初始化，无法启动")
	}

	clientConfig, err := c.cfg.ToClientConfig()
	if err != nil {
		return err
	}

	serverConfig := c.cfg.ToServerConfig()

	c.log.Infof("正在启动嵌入式etcd服务器，数据目录: %v", serverConfig.DataDir)

	server, err := NewEtcdServer(serverConfig, c.log.GetZapLogger("etcd-server"))
	if err != nil {
		// 无论是独立模式还是集群模式，如果etcd启动失败都返回错误
		return fmt.Errorf("启动嵌入式etcd服务器失败: %w", err)
	} else {
		c.Server = server
		c.log.Infof("嵌入式etcd服务器已启动，客户端地址: %v", serverConfig.ClientURLs)
	}

	client, err := NewEtcdClient(clientConfig, c.log.GetZapLogger("etcd-client"))
	if err != nil {
		return fmt.Errorf("初始化etcd客户端失败: %w", err)
	}
	c.Client = client
	c.log.Infof("etcd客户端已初始化，连接到: %v", clientConfig.Endpoints)

	c.status = consts.StatusRunning
	return nil
}

func (c *EtcdComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == consts.StatusError {
		return fmt.Errorf("组件处于错误状态，无法停止")
	} else if c.status != consts.StatusRunning {
		return nil
	}

	c.log.Infof("正在停止嵌入式etcd服务器")

	if c.Server != nil {
		c.Server.Close()
		c.Server = nil
	}
	if c.Client != nil {
		c.Client.Close()
		c.Client = nil
	}

	c.status = consts.StatusStopped
	c.log.Infof("嵌入式etcd服务器已停止")
	return nil
}

func (c *EtcdComponent) Status() consts.ComponentStatus {
	return c.status
}
func (c *EtcdComponent) GetServer() *EtcdServer {
	return c.Server
}

func (c *EtcdComponent) GetClient() *EtcdClient {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Client == nil {
		c.log.Error("尝试访问未初始化的etcd客户端")
		// 由于接口原因我们不能直接返回错误，但可以记录错误
		// 在正常使用流程中不应该出现nil客户端
	}
	return c.Client
}

// DefaultConfig 返回组件的默认配置
func (c *EtcdComponent) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	return NewDefaultConfig(baseConfig)
}

// GetClientConfig 返回客户端配置
func (c *EtcdComponent) GetClientConfig() (bool, *config.ClientConfig) {
	value := map[string]interface{}{
		"username": c.cfg.User.RootUsername,
		"password": config.NewEncryptedValue(c.cfg.User.RootPassword),
	}

	return true, config.NewClientConfig("etcd", value)
}
