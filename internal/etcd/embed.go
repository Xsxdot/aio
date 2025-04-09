package etcd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.uber.org/zap"
)

var (
	// GlobalEtcdServer 全局的嵌入式etcd服务器实例
	GlobalEtcdServer *EtcdServer
)

// EtcdServer 代表一个嵌入式的etcd服务器
type EtcdServer struct {
	server *embed.Etcd
	cfg    *embed.Config
	logger *zap.Logger
}

// ServerConfig 代表etcd服务器配置
type ServerConfig struct {
	// 节点名称，必须唯一
	Name string
	// 数据目录
	DataDir string
	// 客户端URL
	ClientURLs []string
	// 客户端HTTP URL（用于REST API访问）
	ListenClientHttpUrls []string
	// 对等节点URL（用于集群内部通信）
	PeerURLs []string
	// 初始集群配置，格式为: nodeName1=http://ip1:2380,nodeName2=http://ip2:2380
	InitialCluster string
	// 初始集群状态: "new" 或 "existing"
	InitialClusterState string
	// 初始集群令牌
	InitialClusterToken string
	// 安全配置
	ClientTLSConfig TLSConfig
	PeerTLSConfig   TLSConfig
	AuthToken       string
	// JWT认证配置
	JWT JWTConfig
	// 用户名密码认证配置
	UserAuthConfig UserAuthConfig
}

// NewDefaultServerConfig 创建默认的服务器配置
func NewDefaultServerConfig(name, dataDir string) *ServerConfig {
	return &ServerConfig{
		Name:                 name,
		DataDir:              dataDir,
		ClientURLs:           []string{"http://localhost:2379"},
		ListenClientHttpUrls: []string{"http://localhost:2379/v3"},
		PeerURLs:             []string{"http://localhost:2380"},
		InitialCluster:       fmt.Sprintf("%s=http://localhost:2380", name),
		InitialClusterState:  "new",
		InitialClusterToken:  "etcd-cluster",
	}
}

// NewEtcdServer 创建一个新的嵌入式etcd服务器
func NewEtcdServer(config *ServerConfig, logger *zap.Logger) (*EtcdServer, error) {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("创建日志记录器失败: %v", err)
		}
	}

	cfg := embed.NewConfig()
	cfg.Name = config.Name
	cfg.Dir = config.DataDir

	// 使用更严格的权限创建数据目录
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %v", err)
	}

	// 设置客户端URLs
	cfg.ListenClientUrls = make([]url.URL, 0, len(config.ClientURLs))
	cfg.AdvertiseClientUrls = make([]url.URL, 0, len(config.ClientURLs))
	for _, u := range config.ClientURLs {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("解析客户端URL失败: %v", err)
		}
		cfg.ListenClientUrls = append(cfg.ListenClientUrls, *parsedURL)
		cfg.AdvertiseClientUrls = append(cfg.AdvertiseClientUrls, *parsedURL)
	}

	// 设置客户端HTTP URLs（如果有）
	if len(config.ListenClientHttpUrls) > 0 {
		for _, u := range config.ListenClientHttpUrls {
			parsedURL, err := url.Parse(u)
			if err != nil {
				return nil, fmt.Errorf("解析客户端HTTP URL失败: %v", err)
			}
			cfg.ListenMetricsUrls = append(cfg.ListenMetricsUrls, *parsedURL)
		}
		// 启用HTTP服务器
		cfg.EnableGRPCGateway = true
	}

	// 设置安全选项
	if len(cfg.ListenClientUrls) > 0 {
		cfg.ClientTLSInfo = transport.TLSInfo{} // 初始化TLS配置
		// 设置为false可以启用独立的HTTP服务器
		cfg.EnableGRPCGateway = false
	}

	// 配置TLS
	clientTLSConfig := config.ClientTLSConfig
	if clientTLSConfig.TLSEnabled {
		// 客户端TLS配置
		if clientTLSConfig.AutoTls {
			cfg.ClientAutoTLS = true
		} else {
			if clientTLSConfig.Cert != "" && clientTLSConfig.Key != "" {
				cfg.ClientTLSInfo = transport.TLSInfo{
					CertFile:      clientTLSConfig.Cert,
					KeyFile:       clientTLSConfig.Key,
					TrustedCAFile: clientTLSConfig.TrustedCA,
				}
			}
		}
	}

	peerTLSConfig := config.PeerTLSConfig
	if peerTLSConfig.TLSEnabled {
		// 客户端TLS配置
		if peerTLSConfig.AutoTls {
			cfg.PeerAutoTLS = true
		} else {
			if peerTLSConfig.Cert != "" && peerTLSConfig.Key != "" {
				cfg.PeerTLSInfo = transport.TLSInfo{
					CertFile:      peerTLSConfig.Cert,
					KeyFile:       peerTLSConfig.Key,
					TrustedCAFile: peerTLSConfig.TrustedCA,
				}
			}
		}
	}

	// 设置认证选项
	// 注意：etcd v3中，不是通过配置文件直接设置认证，而是在服务启动后通过API启用

	setRoot := false
	switch config.AuthToken {
	case "jwt":
		if config.JWT.PublicKey != "" && config.JWT.PrivateKey != "" {
			signMethod := "RS256" // 默认签名方法
			if config.JWT.SignMethod != "" {
				signMethod = config.JWT.SignMethod
			}

			cfg.AuthToken = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=%s",
				config.JWT.PublicKey,
				config.JWT.PrivateKey,
				signMethod)

			logger.Info("已启用JWT认证")
		}
	case "simple":
		// 使用用户名密码认证
		// 注意：etcd使用simple token作为认证系统的一部分
		cfg.AuthToken = "simple"
		setRoot = true
		logger.Info("已配置用户名密码认证支持")
	default:
		// 如果没有配置认证，使用简单认证模式
		cfg.AuthToken = "simple"
		logger.Warn("使用简单认证模式，仅适用于测试环境")
	}

	// 设置令牌TTL
	cfg.AuthTokenTTL = 3600 // 默认1小时过期

	// 设置对等节点URLs
	cfg.ListenPeerUrls = make([]url.URL, 0, len(config.PeerURLs))
	cfg.AdvertisePeerUrls = make([]url.URL, 0, len(config.PeerURLs))
	for _, u := range config.PeerURLs {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("解析对等节点URL失败: %v", err)
		}
		cfg.ListenPeerUrls = append(cfg.ListenPeerUrls, *parsedURL)
		if !strings.HasPrefix(parsedURL.Host, "0.0.0.0") {
			cfg.AdvertisePeerUrls = append(cfg.AdvertisePeerUrls, *parsedURL)
		}
	}

	// 设置集群配置
	cfg.InitialCluster = config.InitialCluster
	cfg.ClusterState = config.InitialClusterState
	cfg.InitialClusterToken = config.InitialClusterToken

	// 设置日志记录器
	cfg.ZapLoggerBuilder = embed.NewZapLoggerBuilder(logger)
	cfg.Logger = "zap"

	// 启动etcd服务器
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, fmt.Errorf("启动etcd服务器失败: %v", err)
	}

	// 等待服务器准备好
	select {
	case <-e.Server.ReadyNotify():
		logger.Info("etcd服务器已启动",
			zap.String("name", config.Name),
			zap.Strings("clientURLs", config.ClientURLs),
			zap.Strings("peerURLs", config.PeerURLs),
			zap.String("initialCluster", config.InitialCluster))
	case <-time.After(60 * time.Second):
		e.Close()
		return nil, fmt.Errorf("etcd服务器启动超时")
	case err := <-e.Err():
		return nil, fmt.Errorf("etcd服务器启动错误: %v", err)
	}

	// 创建etcd服务器实例
	server := &EtcdServer{
		server: e,
		cfg:    cfg,
		logger: logger,
	}

	// 如果启用了用户认证，并且需要自动设置根用户
	authConfig := config.UserAuthConfig
	if setRoot && authConfig.RootUsername != "" && authConfig.RootPassword != "" {
		// 等待服务器完全启动
		time.Sleep(1 * time.Second)

		// 设置根用户
		if err := server.setupRootUser(authConfig.RootUsername, authConfig.RootPassword); err != nil {
			logger.Warn("设置根用户失败", zap.Error(err))
		} else {
			logger.Info("成功设置根用户", zap.String("username", authConfig.RootUsername))
		}
	}

	return server, nil
}

// setupRootUser 设置etcd根用户
func (s *EtcdServer) setupRootUser(username, password string) error {
	clientURL := s.server.Clients[0].Addr().String()
	if clientURL == "" && len(s.cfg.AdvertiseClientUrls) > 0 {
		clientURL = s.cfg.AdvertiseClientUrls[0].String()
	}

	s.logger.Info("准备设置根用户",
		zap.String("clientURL", clientURL),
		zap.String("username", username))

	// 创建一个新的客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{clientURL},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("创建etcd客户端失败: %v", err)
	}
	defer cli.Close()

	// 创建一个上下文，用于API调用
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 检查认证是否已启用
	authStatus, err := cli.AuthStatus(ctx)
	if err != nil {
		return fmt.Errorf("获取认证状态失败: %v", err)
	}

	if authStatus.Enabled {
		s.logger.Info("认证已经启用，跳过设置")
		return nil
	}

	// 添加根用户
	_, err = cli.UserAdd(ctx, username, password)
	if err != nil {
		// 如果用户已存在，忽略错误
		if strings.Contains(err.Error(), "user name already exists") {
			s.logger.Info("用户已存在", zap.String("username", username))
		} else {
			return fmt.Errorf("添加用户失败: %v", err)
		}
	}

	// 为根用户授予root角色
	_, err = cli.UserGrantRole(ctx, username, "root")
	if err != nil {
		return fmt.Errorf("授予root角色失败: %v", err)
	}

	// 启用认证
	_, err = cli.AuthEnable(ctx)
	if err != nil {
		return fmt.Errorf("启用认证失败: %v", err)
	}

	s.logger.Info("成功启用认证并设置根用户", zap.String("username", username))
	return nil
}

// Close 关闭etcd服务器
func (s *EtcdServer) Close() {
	if s.server == nil {
		return
	}

	// 记录关闭信息
	s.logger.Info("正在关闭etcd服务器...")

	// 尝试优雅关闭
	closeTimeout := 5 * time.Second
	closeTimer := time.NewTimer(closeTimeout)
	closeChan := make(chan struct{})

	go func() {
		// 关闭服务器
		s.server.Close()
		close(closeChan)
	}()

	// 等待关闭完成或超时
	select {
	case <-closeChan:
		closeTimer.Stop()
		s.logger.Info("etcd服务器已关闭")
	case <-closeTimer.C:
		s.logger.Warn("etcd服务器关闭超时")
	}
}

// InitGlobalEtcdServer 初始化全局的etcd服务器
func InitGlobalEtcdServer(config *ServerConfig, logger *zap.Logger) error {
	if GlobalEtcdServer != nil {
		return fmt.Errorf("全局etcd服务器已经初始化")
	}

	server, err := NewEtcdServer(config, logger)
	if err != nil {
		return err
	}

	GlobalEtcdServer = server
	return nil
}

// GetGlobalEtcdServer 获取全局的etcd服务器实例
func GetGlobalEtcdServer() *EtcdServer {
	return GlobalEtcdServer
}

// CloseGlobalEtcdServer 关闭全局的etcd服务器
func CloseGlobalEtcdServer() {
	if GlobalEtcdServer != nil {
		GlobalEtcdServer.Close()
		GlobalEtcdServer = nil
	}
}

// ParseEndpoints 将逗号分隔的端点字符串解析为字符串切片
func ParseEndpoints(endpoints string) []string {
	if endpoints == "" {
		return []string{}
	}
	return strings.Split(endpoints, ",")
}

// BuildInitialCluster 构建初始集群配置字符串
func BuildInitialCluster(clusterNodes map[string]string) string {
	if len(clusterNodes) == 0 {
		return ""
	}

	var sb strings.Builder
	first := true
	for name, url := range clusterNodes {
		if !first {
			sb.WriteString(",")
		}
		sb.WriteString(name)
		sb.WriteString("=")
		sb.WriteString(url)
		first = false
	}
	return sb.String()
}
