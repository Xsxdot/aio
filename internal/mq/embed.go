package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/common"

	"github.com/nats-io/nats-server/v2/server"
	"go.uber.org/zap"
)

var (
	// GlobalNatsServer 全局的嵌入式NATS服务器实例
	GlobalNatsServer *NatsServer
)

// NatsServer 代表一个嵌入式的NATS服务器
type NatsServer struct {
	server  *server.Server
	options *server.Options
	logger  *zap.Logger
	status  consts.ComponentStatus
	config  *ServerConfig
}

func (s *NatsServer) RegisterMetadata() (bool, int, map[string]string) {
	return true, s.options.Port, map[string]string{
		"clusterName": s.options.Cluster.Name,
	}
}

func (s *NatsServer) Name() string {
	return consts.ComponentMQServer
}

func (s *NatsServer) Status() consts.ComponentStatus {
	return s.status
}

func (s *NatsServer) Init(config *config.BaseConfig, body []byte) error {
	serverConfig := s.genConfig(config)

	if err := json.Unmarshal(body, &serverConfig); err != nil {
		return err
	}
	// 确保数据目录存在
	if err := os.MkdirAll(serverConfig.DataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %v", err)
	}

	// 创建NATS服务器配置
	opts := &server.Options{
		ServerName:     serverConfig.Name,
		Host:           serverConfig.Host,
		Port:           serverConfig.Port,
		MaxConn:        serverConfig.MaxConnections,
		MaxControlLine: int32(serverConfig.MaxControlLine),
		MaxPayload:     int32(serverConfig.MaxPayload),
		WriteDeadline:  serverConfig.WriteTimeout,
		Debug:          serverConfig.Debug,
		Trace:          serverConfig.Trace,
		Username:       serverConfig.Username,
		Password:       serverConfig.Password,
		NoSigs:         true, // 禁用信号处理，在测试中很重要
		NoLog:          true, // 禁用内部日志，避免冲突
	}

	// 配置集群
	if serverConfig.ClusterPort > 0 && serverConfig.ClusterName != "" {
		opts.Cluster = server.ClusterOpts{
			Name: serverConfig.ClusterName,
			Host: serverConfig.Host,
			Port: serverConfig.ClusterPort,
		}

		// 配置集群TLS
		if serverConfig.ClusterTLSEnabled {
			opts.Cluster.TLSConfig, _ = LoadServerTLSConfig(
				serverConfig.ClusterCertFile,
				serverConfig.ClusterKeyFile,
				serverConfig.ClusterCAFile,
				false, // 集群通信不验证客户端
			)
			opts.Cluster.TLSTimeout = float64(serverConfig.TLSTimeout / time.Second)
		}

		// 添加路由信息
		if len(serverConfig.Routes) > 0 {
			for _, route := range serverConfig.Routes {
				opts.Routes = append(opts.Routes, server.RoutesFromStr(route)...)
			}
		}
	} else {
		// 如果没有正确配置集群，确保禁用集群功能
		serverConfig.ClusterPort = 0
		serverConfig.ClusterName = ""
		serverConfig.Routes = nil
	}

	// 配置JetStream (持久化存储)
	if serverConfig.JetStreamEnabled {
		opts.JetStream = true

		// 设置JetStream存储目录
		jsDir := fmt.Sprintf("%s/jetstream", serverConfig.DataDir)
		if err := os.MkdirAll(jsDir, 0755); err != nil {
			return fmt.Errorf("创建JetStream目录失败: %v", err)
		}

		opts.StoreDir = jsDir
		opts.JetStreamMaxMemory = serverConfig.JetStreamMaxMem
		opts.JetStreamMaxStore = serverConfig.JetStreamMaxFile
	}

	// 配置TLS
	if serverConfig.TLSEnabled {
		// 使用优化后的TLS配置加载函数
		tlsConfig, err := LoadServerTLSConfig(
			serverConfig.CertFile,
			serverConfig.KeyFile,
			serverConfig.ClientCAFile,
			serverConfig.VerifyClients,
		)
		if err != nil {
			return fmt.Errorf("加载TLS配置失败: %v", err)
		}

		opts.TLS = true
		opts.TLSConfig = tlsConfig
		opts.TLSTimeout = float64(serverConfig.TLSTimeout / time.Second)

		if serverConfig.VerifyClients {
			opts.TLSVerify = true
			opts.TLSCaCert = serverConfig.ClientCAFile
		}
	}

	s.options = opts
	s.status = consts.StatusInitialized
	s.config = serverConfig

	return nil
}

func (s *NatsServer) Start(ctx context.Context) error {
	// 创建并启动服务器
	srv, err := server.NewServer(s.options)
	if err != nil {
		return fmt.Errorf("创建NATS服务器失败: %v", err)
	}

	// 启动NATS服务
	srv.Start()

	// 等待服务器初始化
	if !srv.ReadyForConnections(5 * time.Second) {
		srv.Shutdown()
		return fmt.Errorf("NATS服务器启动超时")
	}

	s.logger.Info("NATS服务器已启动",
		zap.String("name", s.Name()),
		zap.String("address", fmt.Sprintf("%s:%d", s.options.Host, s.options.Port)),
		zap.Bool("jetstream", s.options.JetStream),
	)
	s.status = consts.StatusRunning
	return nil
}

func (s *NatsServer) GetClient() (*NatsClient, error) {
	var servers []string
	serverConfig := s.config
	if serverConfig == nil {
		return nil, fmt.Errorf("服务器配置为空")
	}
	if len(serverConfig.Routes) > 0 {
		servers = serverConfig.Routes
	} else {
		servers = []string{fmt.Sprintf("nats://%s:%d", s.options.Host, s.options.Port)}
	}

	cfg := &ClientConfig{
		Servers:        servers,
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  10,
		Username:       serverConfig.Username,
		Password:       serverConfig.Password,
		UseJetStream:   serverConfig.JetStreamEnabled,
	}

	if serverConfig.TLSEnabled {
		cfg.TLS = &TLSConfig{
			CertFile:           serverConfig.CertFile,
			KeyFile:            serverConfig.KeyFile,
			TrustedCAFile:      serverConfig.ClientCAFile,
			InsecureSkipVerify: false,
		}
	}

	return NewNatsClient(cfg, s.logger)
}

func (s *NatsServer) Restart(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (s *NatsServer) Stop(ctx context.Context) error {
	s.server.Shutdown()
	s.status = consts.StatusStopped
	return nil
}

// DefaultConfig 返回组件的默认配置
func (s *NatsServer) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	return s.genConfig(baseConfig)
}

func (s *NatsServer) genConfig(baseConfig *config.BaseConfig) *ServerConfig {
	return NewDefaultServerConfig("aio-mq-"+baseConfig.System.NodeId, filepath.Join(baseConfig.System.DataDir, "nats"))
}

// ServerConfig 代表NATS服务器配置
type ServerConfig struct {
	// 服务器名称，必须唯一
	Name string `json:"name" yaml:"name"`
	// 数据目录，用于持久化存储
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// 监听主机地址
	Host string `json:"host" yaml:"host"`
	// 监听客户端端口
	Port int `json:"port" yaml:"port"`
	// 监听集群端口
	ClusterPort int `json:"cluster_port" yaml:"cluster_port"`
	// 集群名称
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
	// 集群地址，用于集群节点间通信
	Routes []string `json:"routes" yaml:"routes"`
	// 最大连接数
	MaxConnections int `json:"max_connections" yaml:"max_connections"`
	// 最大控制线路大小
	MaxControlLine int `json:"max_control_line" yaml:"max_control_line"`
	// 最大有效负载大小
	MaxPayload int `json:"max_payload" yaml:"max_payload"`
	// 写入超时
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	// TLS安全配置
	TLSEnabled    bool          `json:"tls_enabled" yaml:"tls_enabled"`
	CertFile      string        `json:"cert_file" yaml:"cert_file"` // 服务端证书文件
	KeyFile       string        `json:"key_file" yaml:"key_file"`   // 服务端私钥文件
	TLSTimeout    time.Duration `json:"tls_timeout" yaml:"tls_timeout"`
	ClientCAFile  string        `json:"client_ca_file" yaml:"client_ca_file"`   // 用于验证客户端的CA证书文件
	ClusterCAFile string        `json:"cluster_ca_file" yaml:"cluster_ca_file"` // 用于验证集群通信的CA证书文件
	VerifyClients bool          `json:"verify_clients" yaml:"verify_clients"`   // 是否验证客户端证书
	// 集群TLS配置
	ClusterTLSEnabled bool   `json:"cluster_tls_enabled" yaml:"cluster_tls_enabled"`
	ClusterCertFile   string `json:"cluster_cert_file" yaml:"cluster_cert_file"` // 集群通信证书
	ClusterKeyFile    string `json:"cluster_key_file" yaml:"cluster_key_file"`   // 集群通信私钥
	// JetStream配置
	JetStreamEnabled bool  `json:"jet_stream_enabled" yaml:"jet_stream_enabled"`
	JetStreamMaxMem  int64 `json:"jet_stream_max_mem" yaml:"jet_stream_max_mem"`
	JetStreamMaxFile int64 `json:"jet_stream_max_file" yaml:"jet_stream_max_file"`
	// 授权配置
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	// 是否输出debug日志
	Debug bool `json:"debug" yaml:"debug"`
	// 是否输出跟踪日志
	Trace bool `json:"trace" yaml:"trace"`
}

// NewDefaultServerConfig 创建默认的服务器配置
func NewDefaultServerConfig(name, dataDir string) *ServerConfig {
	return &ServerConfig{
		Name:              name,
		DataDir:           dataDir,
		Host:              "localhost",
		Port:              4222,
		ClusterPort:       0,
		ClusterName:       "",
		Routes:            []string{},
		MaxConnections:    1000,
		MaxControlLine:    4096,
		MaxPayload:        1024 * 1024, // 1MB
		WriteTimeout:      5 * time.Second,
		TLSEnabled:        false,
		TLSTimeout:        2 * time.Second,
		VerifyClients:     false,
		ClusterTLSEnabled: false,
		JetStreamEnabled:  true,
		JetStreamMaxMem:   1024 * 1024 * 1024,      // 1GB
		JetStreamMaxFile:  10 * 1024 * 1024 * 1024, // 10GB
		Debug:             false,
		Trace:             false,
	}
}

// NewNatsServer 创建一个新的NATS服务器
func NewNatsServer() (*NatsServer, error) {
	logger := common.GetLogger().GetZapLogger("nats")

	return &NatsServer{
		logger: logger,
	}, nil
}

// Close 关闭NATS服务器
func (s *NatsServer) Close() {
	if s.server != nil {
		s.logger.Info("关闭NATS服务器")
		s.server.Shutdown()
		s.server.WaitForShutdown()
		s.server = nil
	}
}

// GetServer 获取底层的NATS服务器实例
func (s *NatsServer) GetServer() *server.Server {
	return s.server
}

// GetInfo 获取服务器信息
func (s *NatsServer) GetInfo() map[string]interface{} {
	if s.server == nil {
		return nil
	}

	varz, _ := s.server.Varz(&server.VarzOptions{})

	// 由于NATS server库的限制，我们无法直接获取路由数量
	// 这里简化实现，只返回其他可访问的指标
	return map[string]interface{}{
		"serverId":      varz.ID,
		"serverName":    varz.Name,
		"version":       varz.Version,
		"host":          varz.Host,
		"port":          varz.Port,
		"clientsCount":  s.server.NumClients(),
		"subscriptions": s.server.NumSubscriptions(),
		"jetstream":     s.server.JetStreamEnabled(),
	}
}

// WithTLS 为服务器配置添加TLS配置的选项函数
func WithTLS(certFile, keyFile string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.TLSEnabled = true
		config.CertFile = certFile
		config.KeyFile = keyFile
	}
}

// WithClientAuth 为服务器配置添加客户端认证的选项函数
func WithClientAuth(clientCAFile string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.TLSEnabled = true
		config.ClientCAFile = clientCAFile
		config.VerifyClients = true
	}
}

// WithAuth 为服务器配置添加基本认证的选项函数
func WithAuth(username, password string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.Username = username
		config.Password = password
	}
}

// WithCluster 为服务器配置添加集群配置的选项函数
func WithCluster(clusterName string, routes []string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.ClusterName = clusterName
		config.Routes = routes
	}
}

// WithJetStream 为服务器配置添加JetStream配置的选项函数
func WithJetStream(maxMem, maxFile int64) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.JetStreamEnabled = true
		config.JetStreamMaxMem = maxMem
		config.JetStreamMaxFile = maxFile
	}
}

// GetClientConfig 实现Component接口，返回客户端配置
func (s *NatsServer) GetClientConfig() (bool, *config.ClientConfig) {
	value := config.ClientConfigFixedValue{
		Username:  s.config.Username,
		Password:  s.config.Password,
		EnableTls: s.config.TLSEnabled,
	}

	if s.config.TLSEnabled {
		value.Cert = s.config.CertFile
		value.Key = s.config.KeyFile
		value.TrustedCAFile = s.config.ClientCAFile
	}

	return true, config.NewClientConfig("nats", value)
}
