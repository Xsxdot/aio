package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xsxdot/aio/pkg/auth"

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
	natsConfig := serverConfig.NatsConfig

	if err := json.Unmarshal(body, &natsConfig); err != nil {
		return err
	}
	serverConfig.NatsConfig = natsConfig
	serverConfig.DataDir = filepath.Join(config.System.DataDir, serverConfig.DataDir)
	serverConfig.localIP = config.Network.LocalIP

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
		if serverConfig.ServerTLSConfig.TLSEnabled {
			tlsCfg := &serverConfig.ServerTLSConfig
			if tlsCfg.TLSEnabled {
				tlsCfg = auth.GlobalCertManager.NodeCert
			}

			opts.Cluster.TLSConfig, _ = LoadServerTLSConfig(
				tlsCfg.Cert,
				tlsCfg.Key,
				tlsCfg.TrustedCA,
				false,
			)
			opts.Cluster.TLSTimeout = float64(serverConfig.TLSTimeout / time.Second)
		}

		// 添加路由信息
		if len(serverConfig.Routes) == 0 {
			auth := ""
			if serverConfig.Username != "" && serverConfig.Password != "" {
				auth = fmt.Sprintf("%s:%s@", serverConfig.Username, serverConfig.Password)
			}
			opts.Routes = server.RoutesFromStr(fmt.Sprintf("nats-route://%s%s:%d", auth, serverConfig.localIP, serverConfig.ClusterPort))
		} else {
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
	if serverConfig.ServerTLSConfig.TLSEnabled {
		tlsCfg := &serverConfig.ServerTLSConfig
		if tlsCfg.TLSEnabled {
			tlsCfg = auth.GlobalCertManager.NodeCert
		}
		// 使用优化后的TLS配置加载函数
		tlsConfig, err := LoadServerTLSConfig(
			tlsCfg.Cert,
			tlsCfg.Key,
			tlsCfg.TrustedCA,
			true,
		)
		if err != nil {
			return fmt.Errorf("加载TLS配置失败: %v", err)
		}

		opts.TLS = true
		opts.TLSConfig = tlsConfig
		opts.TLSTimeout = float64(serverConfig.TLSTimeout / time.Second)

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

	s.server = srv // 将创建的服务器实例保存到结构体的server字段

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

	if serverConfig.ClientTLSConfig.TLSEnabled {
		client := &serverConfig.ClientTLSConfig
		if client.TLSEnabled {
			client = auth.GlobalCertManager.ClientCert
		}

		cfg.TLS = &TLSConfig{
			CertFile:           client.Cert,
			KeyFile:            client.Key,
			TrustedCAFile:      client.TrustedCA,
			InsecureSkipVerify: false,
		}
	}

	return NewNatsClient(cfg, s.logger)
}

func (s *NatsServer) Restart(ctx context.Context) error {
	err := s.Stop(ctx)
	if err != nil {
		return err
	}
	return s.Start(ctx)
}

func (s *NatsServer) Stop(ctx context.Context) error {
	if s.server != nil {
		s.server.Shutdown()
	}
	s.status = consts.StatusStopped
	return nil
}

// DefaultConfig 返回组件的默认配置
func (s *NatsServer) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	genConfig := s.genConfig(baseConfig).NatsConfig
	return &genConfig
}

func (s *NatsServer) genConfig(baseConfig *config.BaseConfig) *ServerConfig {
	serverConfig := NewDefaultServerConfig("aio-mq-"+baseConfig.System.NodeId, "nats")
	serverConfig.Host = baseConfig.Network.BindIP
	return serverConfig
}

// ServerConfig 代表NATS服务器配置
type ServerConfig struct {
	Name       string
	Host       string
	TLSTimeout time.Duration
	localIP    string

	config.NatsConfig
}

// NewDefaultServerConfig 创建默认的服务器配置
func NewDefaultServerConfig(name, dataDir string) *ServerConfig {
	return &ServerConfig{
		Name:       name,
		Host:       "localhost",
		TLSTimeout: 2 * time.Second,
		NatsConfig: config.NatsConfig{
			Port:             4222,
			DataDir:          dataDir,
			ClusterPort:      5222,
			ClusterName:      "aio",
			Routes:           []string{},
			MaxConnections:   1000,
			MaxControlLine:   4096,
			MaxPayload:       1024 * 1024, // 1MB
			WriteTimeout:     5 * time.Second,
			JetStreamEnabled: true,
			JetStreamMaxMem:  1024 * 1024 * 1024,      // 1GB
			JetStreamMaxFile: 10 * 1024 * 1024 * 1024, // 10GB
			Username:         "root",
			Password:         "12345678",
			Debug:            false,
			Trace:            false,
			ClientTLSConfig: config.TLSConfig{
				TLSEnabled: true,
				AutoTls:    true,
			},
			ServerTLSConfig: config.TLSConfig{
				TLSEnabled: true,
				AutoTls:    true,
			},
		},
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

// GetClientConfig 实现Component接口，返回客户端配置
func (s *NatsServer) GetClientConfig() (bool, *config.ClientConfig) {
	value := config.ClientConfigFixedValue{
		Username:  s.config.Username,
		Password:  s.config.Password,
		EnableTls: s.config.ClientTLSConfig.TLSEnabled,
	}

	if s.config.ClientTLSConfig.TLSEnabled {
		client := &s.config.ClientTLSConfig
		if client.TLSEnabled {
			client = auth.GlobalCertManager.ClientCert
		}

		// 读取证书文件内容而不是存储路径
		certContent, err := os.ReadFile(client.Cert)
		if err == nil {
			value.CertContent = string(certContent)
		} else {
			s.logger.Error("读取证书文件失败", zap.Error(err))
		}

		keyContent, err := os.ReadFile(client.Key)
		if err == nil {
			value.KeyContent = string(keyContent)
		} else {
			s.logger.Error("读取密钥文件失败", zap.Error(err))
		}

		caContent, err := os.ReadFile(client.TrustedCA)
		if err == nil {
			value.CATrustedContent = string(caContent)
		} else {
			s.logger.Error("读取CA证书文件失败", zap.Error(err))
		}

		// 保留文件路径以便兼容
		value.Cert = client.Cert
		value.Key = client.Key
		value.TrustedCAFile = client.TrustedCA
	}

	return true, config.NewClientConfig("nats", value)
}
