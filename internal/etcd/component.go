package etcd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/auth"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/common"
)

type Config struct {
	// 节点名称，必须唯一
	name              string
	bindIP            string
	localIp           string
	dataDir           string
	isMaster          bool
	initialClusterUrl string
	config.EtcdConfig
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

	clientURLs := []string{clientProto + c.bindIP + ":" + fmt.Sprintf("%d", c.ClientPort)}
	peerURLs := []string{serverProto + c.localIp + ":" + fmt.Sprintf("%d", c.PeerPort)}

	state := "new"

	if c.isMaster {
		_, err := os.Stat(filepath.Join(c.dataDir, "member", "wal"))
		if err == nil {
			state = "existing"
		}
	} else {
		state = "existing"
	}

	authToken := c.AuthToken
	if c.Jwt {
		authToken = "jwt"
	}

	serverConfig := &ServerConfig{
		Name:       c.name,
		DataDir:    c.dataDir,
		ClientURLs: clientURLs,
		//ListenClientHttpUrls: []string{ip + "/v3"},
		PeerURLs:            peerURLs,
		InitialCluster:      c.initialClusterUrl,
		InitialClusterState: state,
		InitialClusterToken: c.InitialClusterToken,
		PeerTLSConfig:       c.ServerTLSConfig,
		ClientTLSConfig:     c.ClientTLSConfig,
		AuthToken:           authToken,
		UserAuthConfig: UserAuthConfig{
			RootUsername: c.Username,
			RootPassword: c.Password,
		},
	}

	if c.Jwt {
		jwtConfig := auth.GlobalCertManager.JwtConfig
		serverConfig.JWT = *jwtConfig
	}

	server := auth.GlobalCertManager.NodeCert
	//client := auth.GlobalCertManager.ClientCert

	if c.ServerTLSConfig.TLSEnabled {
		if c.ServerTLSConfig.AutoTls {
			serverConfig.PeerTLSConfig = config.TLSConfig{
				AutoTls:    false,
				TLSEnabled: true,
				Cert:       server.Cert,
				Key:        server.Key,
				TrustedCA:  auth.GlobalCertManager.GetCAFilePath(),
			}
		}
	}

	if c.ClientTLSConfig.TLSEnabled {
		if c.ClientTLSConfig.AutoTls {
			serverConfig.ClientTLSConfig = config.TLSConfig{
				AutoTls:    false,
				TLSEnabled: true,
				Cert:       server.Cert,
				Key:        server.Key,
				TrustedCA:  auth.GlobalCertManager.GetCAFilePath(),
			}
		}
	}

	return serverConfig
}

func (c *Config) ToClientConfig() (*ClientConfig, error) {
	proto := "http://"
	if c.ClientTLSConfig.TLSEnabled {
		proto = "https://"
	}

	tlsConfig := config.TLSConfig{
		AutoTls:    false,
		TLSEnabled: false,
	}

	if c.ClientTLSConfig.TLSEnabled {
		if c.ClientTLSConfig.AutoTls {
			client := auth.GlobalCertManager.ClientCert
			tlsConfig.TLSEnabled = true
			tlsConfig.Cert = client.Cert
			tlsConfig.Key = client.Key
			tlsConfig.TrustedCA = auth.GlobalCertManager.GetCAFilePath()
		}
	}
	ip := proto + c.localIp + ":" + fmt.Sprintf("%d", c.ClientPort)
	return &ClientConfig{
		Endpoints:         []string{ip},
		DialTimeout:       5 * time.Second,
		Username:          c.Username,
		Password:          c.Password,
		AutoSyncEndpoints: true,
		TLS:               &tlsConfig,
	}, nil
}

// NewDefaultConfig 创建默认的服务器配置
func NewDefaultConfig() *Config {
	return &Config{
		EtcdConfig: config.EtcdConfig{
			ClientPort:          2379,
			PeerPort:            2380,
			InitialClusterToken: "etcd-cluster",
			AuthToken:           "jwt",
			ClientTLSConfig:     config.TLSConfig{AutoTls: true, TLSEnabled: true},
			ServerTLSConfig:     config.TLSConfig{AutoTls: true, TLSEnabled: true},
			Jwt:                 true,
			Username:            "root",
			Password:            "123456",
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
	serverConfig := NewDefaultConfig()
	serverConfig.EtcdConfig = *config.Etcd
	c.cfg = serverConfig

	proto := "http://"
	if serverConfig.ServerTLSConfig.TLSEnabled {
		proto = "https://"
	}

	name := "aio-etcd-" + config.System.NodeId
	serverConfig.name = name
	serverConfig.dataDir = filepath.Join(config.System.DataDir, "etcd")
	serverConfig.bindIP = config.Network.BindIP
	serverConfig.localIp = config.Network.LocalIP

	for _, node := range config.Nodes {
		if node.Master {
			serverConfig.isMaster = node.NodeId == config.System.NodeId
			serverConfig.initialClusterUrl = fmt.Sprintf("%s=%s%s:%d", name, proto, node.Addr, serverConfig.PeerPort)
		}
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

	clientConfig, err := c.cfg.ToClientConfig()
	if err != nil {
		return err
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
	return NewDefaultConfig()
}

// GetClientConfig 返回客户端配置
func (c *EtcdComponent) GetClientConfig() (bool, *config.ClientConfig) {
	value := config.ClientConfigFixedValue{
		Username:  c.cfg.Username,
		Password:  c.cfg.Password,
		EnableTls: c.cfg.ClientTLSConfig.TLSEnabled,
	}

	if c.cfg.ClientTLSConfig.TLSEnabled {
		// 读取证书文件内容
		certFile := c.cfg.ClientTLSConfig.Cert
		keyFile := c.cfg.ClientTLSConfig.Key
		caFile := c.cfg.ClientTLSConfig.TrustedCA
		if c.cfg.ClientTLSConfig.AutoTls {
			client := auth.GlobalCertManager.ClientCert
			certFile = client.Cert
			keyFile = client.Key
			caFile = auth.GlobalCertManager.GetCAFilePath()
		}

		// 保存文件路径以便兼容
		value.Cert = certFile
		value.Key = keyFile
		value.TrustedCAFile = caFile

		// 读取证书内容
		if certContent, err := os.ReadFile(certFile); err == nil {
			value.CertContent = string(certContent)
		} else {
			c.log.Errorf("读取证书文件失败: %v", err)
		}

		// 读取密钥内容
		if keyContent, err := os.ReadFile(keyFile); err == nil {
			value.KeyContent = string(keyContent)
		} else {
			c.log.Errorf("读取密钥文件失败: %v", err)
		}

		// 读取CA证书内容
		if caContent, err := os.ReadFile(caFile); err == nil {
			value.CATrustedContent = string(caContent)
		} else {
			c.log.Errorf("读取CA证书文件失败: %v", err)
		}
	}

	return true, config.NewClientConfig("etcd", value)
}
