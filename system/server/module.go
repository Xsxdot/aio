package server

import (
	"context"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/server/api/client"
	grpcservice "github.com/xsxdot/aio/system/server/external/grpc"
	internalapp "github.com/xsxdot/aio/system/server/internal/app"
)

// Module 服务器组件模块
type Module struct {
	internalApp *internalapp.App
	Client      *client.ServerClient
	GRPCService *grpcservice.ServerService
}

// NewModule 创建服务器组件模块
func NewModule() *Module {
	log := logger.GetLogger().WithEntryName("ServerModule")

	// 创建内部 App
	app := internalapp.NewApp()

	// 创建对外 Client
	clientInstance := client.NewServerClient(app)

	// 创建 gRPC 服务
	grpcSvc := grpcservice.NewServerService(clientInstance, app, log)

	return &Module{
		internalApp: app,
		Client:      clientInstance,
		GRPCService: grpcSvc,
	}
}

// EnsureBootstrapServers 确保 bootstrap 服务器已存在
func (m *Module) EnsureBootstrapServers(ctx context.Context) error {
	return m.internalApp.EnsureBootstrapServers(ctx)
}

// EnsureBootstrapServerSSHCredentials 确保 bootstrap 服务器的 SSH 凭证已存在
func (m *Module) EnsureBootstrapServerSSHCredentials(ctx context.Context) error {
	return m.internalApp.EnsureBootstrapServerSSHCredentials(ctx)
}
