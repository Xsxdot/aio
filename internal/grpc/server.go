package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ServiceRegistrar 服务注册接口
// 所有 gRPC 服务都需要实现此接口
type ServiceRegistrar interface {
	// RegisterService 注册服务到 gRPC 服务器
	RegisterService(server *grpc.Server) error
	// ServiceName 返回服务名称
	ServiceName() string
	// ServiceVersion 返回服务版本
	ServiceVersion() string
}

// Server gRPC 服务器管理器
type Server struct {
	config   *Config
	server   *grpc.Server
	logger   *zap.Logger
	services map[string]ServiceRegistrar
	listener net.Listener
	mu       sync.RWMutex
	running  bool
}

// NewServer 创建新的 gRPC 服务器
func NewServer(config *Config, logger *zap.Logger) *Server {
	// 根据配置构建服务器选项
	opts := config.BuildServerOptions(logger)

	return &Server{
		config:   config,
		server:   grpc.NewServer(opts...),
		logger:   logger,
		services: make(map[string]ServiceRegistrar),
	}
}

// NewServerWithOptions 使用自定义选项创建 gRPC 服务器
func NewServerWithOptions(address string, logger *zap.Logger, opts ...grpc.ServerOption) *Server {
	config := &Config{Address: address}
	return &Server{
		config:   config,
		server:   grpc.NewServer(opts...),
		logger:   logger,
		services: make(map[string]ServiceRegistrar),
	}
}

// SetAuthProvider 设置鉴权提供者
func (s *Server) SetAuthProvider(provider AuthProvider) {
	if s.config.Auth == nil {
		s.config.Auth = DefaultAuthConfig()
	}
	s.config.Auth.AuthProvider = provider

	// 如果设置了鉴权提供者，默认启用鉴权
	s.config.EnableAuth = true
	s.config.EnablePermission = true
}

// EnableAuth 启用鉴权功能
func (s *Server) EnableAuth(skipMethods []string) {
	if s.config.Auth == nil {
		s.config.Auth = DefaultAuthConfig()
	}

	if skipMethods != nil {
		s.config.Auth.SkipMethods = append(s.config.Auth.SkipMethods, skipMethods...)
	}

	s.config.EnableAuth = true
	s.config.EnablePermission = true
}

// DisableAuth 禁用鉴权功能
func (s *Server) DisableAuth() {
	s.config.EnableAuth = false
	s.config.EnablePermission = false
}

// RegisterService 注册服务
func (s *Server) RegisterService(service ServiceRegistrar) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	serviceName := service.ServiceName()

	// 检查服务是否已注册
	if _, exists := s.services[serviceName]; exists {
		return fmt.Errorf("服务 %s 已经注册", serviceName)
	}

	// 注册服务到 gRPC 服务器
	if err := service.RegisterService(s.server); err != nil {
		return fmt.Errorf("注册服务 %s 失败: %w", serviceName, err)
	}

	// 保存服务引用
	s.services[serviceName] = service

	s.logger.Info("gRPC 服务注册成功",
		zap.String("service", serviceName),
		zap.String("version", service.ServiceVersion()))

	return nil
}

// Start 启动 gRPC 服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("gRPC 服务器已经在运行")
	}

	// 启用反射服务（如果配置了）
	if s.config.EnableReflection {
		reflection.Register(s.server)
		s.logger.Info("gRPC 反射服务已启用")
	}

	// 创建监听器
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("创建监听器失败: %w", err)
	}
	s.listener = listener

	s.logger.Info("启动 gRPC 服务器",
		zap.String("address", s.config.Address),
		zap.Int("services", len(s.services)),
		zap.Bool("auth_enabled", s.config.EnableAuth),
		zap.Bool("permission_enabled", s.config.EnablePermission))

	// 打印已注册的服务
	for name, service := range s.services {
		s.logger.Info("已注册服务",
			zap.String("name", name),
			zap.String("version", service.ServiceVersion()))
	}

	s.running = true

	// 在 goroutine 中启动服务器
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.logger.Error("gRPC 服务器运行失败", zap.Error(err))
		}
	}()

	return nil
}

// Stop 停止 gRPC 服务器
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("停止 gRPC 服务器")
	s.server.GracefulStop()
	s.running = false
}

// GetRegisteredServices 获取已注册的服务列表
func (s *Server) GetRegisteredServices() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]string, 0, len(s.services))
	for name := range s.services {
		services = append(services, name)
	}
	return services
}

// IsRunning 检查服务器是否正在运行
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Address 获取服务器监听地址
func (s *Server) Address() string {
	return s.config.Address
}

// GetServer 获取原生 gRPC 服务器实例
func (s *Server) GetServer() *grpc.Server {
	return s.server
}

// ListServices 列出所有注册的服务
func (s *Server) ListServices() []ServiceRegistrar {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]ServiceRegistrar, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service)
	}
	return services
}

// Shutdown 强制关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("gRPC 服务器优雅关闭完成")
		return nil
	case <-ctx.Done():
		s.logger.Warn("gRPC 服务器优雅关闭超时，强制关闭")
		s.server.Stop()
		return ctx.Err()
	}
}
