package user

import (
	"context"
	"time"
	"xiaozhizhang/base"
	"xiaozhizhang/system/user/api/client"
	grpcsvc "xiaozhizhang/system/user/external/grpc"
	"xiaozhizhang/system/user/internal/app"
	"xiaozhizhang/system/user/internal/service"
)

// Module 用户组件模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用用户能力
	Client *client.UserClient
	// GRPCService gRPC服务实例，供gRPC服务器注册使用
	GRPCService *grpcsvc.ClientAuthServiceImpl
	// JwtService JWT服务实例
	jwtService *service.JwtService
}

// NewModule 创建用户组件模块实例
func NewModule() *Module {
	// 创建内部 App
	internalApp := app.NewApp()

	// 创建对外 Client
	userClient := client.NewUserClient(internalApp)

	// 从配置中获取 JWT 配置
	jwtSecret := base.Configures.Config.Jwt.Secret
	if jwtSecret == "" {
		jwtSecret = "default-client-jwt-secret" // 默认值（生产环境应从配置中读取）
	}
	jwtExpireTime := time.Duration(base.Configures.Config.Jwt.ExpireTime) * time.Hour
	if jwtExpireTime <= 0 {
		jwtExpireTime = 24 * time.Hour // 默认 24 小时
	}

	// 创建 JWT 服务
	jwtService := service.NewJwtService(jwtSecret, jwtExpireTime, base.Logger)

	// 创建 gRPC 服务
	grpcService := grpcsvc.NewClientAuthService(internalApp, jwtService, base.Logger)

	return &Module{
		internalApp: internalApp,
		Client:      userClient,
		GRPCService: grpcService,
		jwtService:  jwtService,
	}
}

// GetJwtService 返回 JWT 服务实例（仅用于 gRPC 鉴权初始化）
func (m *Module) GetJwtService() *service.JwtService {
	return m.jwtService
}

// NewTokenParser 创建 Token 解析器（用于 gRPC 鉴权）
func (m *Module) NewTokenParser() *TokenParserAdapter {
	return NewTokenParserAdapter(m.jwtService)
}

// EnsureBootstrapSuperAdmin 确保存在默认超级管理员
// 当 user_admin 表为空时，自动创建账号/密码均为 admin 的超级管理员
func (m *Module) EnsureBootstrapSuperAdmin(ctx context.Context) error {
	count, err := m.internalApp.AdminService.Count(ctx)
	if err != nil {
		base.Logger.WithErr(err).Error("检查管理员数量失败")
		return err
	}

	if count > 0 {
		base.Logger.Info("管理员表已有数据，跳过默认超级管理员初始化")
		return nil
	}

	// 创建默认超级管理员 admin/admin
	admin, err := m.internalApp.AdminService.CreateSuperAdmin(ctx, "admin", "admin", "系统默认超级管理员")
	if err != nil {
		base.Logger.WithErr(err).Error("创建默认超级管理员失败")
		return err
	}

	base.Logger.WithField("adminId", admin.ID).WithField("account", admin.Account).Info("已创建默认超级管理员 admin/admin")
	return nil
}
