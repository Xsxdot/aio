package grpc

import (
	"context"
	"errors"
	"strings"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/user/api/proto"
	"github.com/xsxdot/aio/system/user/internal/app"
	"github.com/xsxdot/aio/system/user/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ClientAuthServiceImpl 客户端认证服务实现
type ClientAuthServiceImpl struct {
	proto.UnimplementedClientAuthServiceServer
	app        *app.App
	jwtService *service.JwtService
	log        *logger.Log
	err        *errorc.ErrorBuilder
}

// NewClientAuthService 创建客户端认证服务实例
func NewClientAuthService(app *app.App, jwtService *service.JwtService, log *logger.Log) *ClientAuthServiceImpl {
	return &ClientAuthServiceImpl{
		app:        app,
		jwtService: jwtService,
		log:        log.WithEntryName("ClientAuthService"),
		err:        errorc.NewErrorBuilder("ClientAuthService"),
	}
}

// AuthenticateClient 客户端认证，返回 JWT token
func (s *ClientAuthServiceImpl) AuthenticateClient(ctx context.Context, req *proto.AuthenticateClientRequest) (*proto.AuthenticateClientResponse, error) {
	// 参数校验
	if req.ClientKey == "" || req.ClientSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "客户端 key 和 secret 不能为空")
	}

	// 验证客户端凭证
	client, err := s.app.ClientCredentialService.ValidateClient(ctx, req.ClientKey, req.ClientSecret)
	if err != nil {
		s.log.WithErr(err).Error("客户端认证失败")
		// 检查是否为 NotFound 或 Validation 错误
		var e *errorc.Error
		if errors.As(err, &e) && (e.ErrorCode == errorc.ErrorCodeNotFound || e.ErrorCode == errorc.ErrorCodeValid) {
			return nil, status.Error(codes.Unauthenticated, "客户端凭证无效")
		}
		return nil, status.Error(codes.Internal, "认证失败")
	}

	// 生成 JWT token
	token, expiresAt, err := s.jwtService.CreateClientToken(client.ID, client.ClientKey, client.Status)
	if err != nil {
		s.log.WithErr(err).Error("生成 token 失败")
		return nil, status.Error(codes.Internal, "生成 token 失败")
	}

	s.log.WithField("clientKey", client.ClientKey).Info("客户端认证成功")

	return &proto.AuthenticateClientResponse{
		AccessToken: token,
		ExpiresAt:   expiresAt,
		TokenType:   "Bearer",
	}, nil
}

// RenewToken 续期 token（在未过期时延长有效期）
func (s *ClientAuthServiceImpl) RenewToken(ctx context.Context, req *proto.RenewTokenRequest) (*proto.RenewTokenResponse, error) {
	// 从 metadata 中获取 token
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "缺少认证信息")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "缺少 authorization header")
	}

	tokenString := authHeaders[0]
	// 移除 "Bearer " 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimSpace(tokenString)

	if tokenString == "" {
		return nil, status.Error(codes.Unauthenticated, "token 为空")
	}

	// 解析 token 以获取客户端信息
	claims, err := s.jwtService.ParseClientToken(tokenString)
	if err != nil {
		s.log.WithErr(err).Error("解析 token 失败")
		return nil, status.Error(codes.Unauthenticated, "无效的 token")
	}

	// 验证客户端当前状态（确保未被禁用）
	client, err := s.app.ClientCredentialService.FindById(ctx, claims.ClientID)
	if err != nil {
		s.log.WithErr(err).Error("查询客户端失败")
		if errorc.IsNotFound(err) {
			return nil, status.Error(codes.Unauthenticated, "客户端不存在")
		}
		return nil, status.Error(codes.Internal, "查询客户端失败")
	}

	// 检查客户端是否仍然活跃
	if !client.IsActive() {
		s.log.WithField("clientKey", client.ClientKey).Warn("客户端已禁用或过期")
		return nil, status.Error(codes.PermissionDenied, "客户端已被禁用或过期")
	}

	// 续期 token
	newToken, expiresAt, err := s.jwtService.RenewClientToken(tokenString)
	if err != nil {
		s.log.WithErr(err).Error("续期 token 失败")
		// 检查是否为 Validation 错误
		var e *errorc.Error
		if errors.As(err, &e) && e.ErrorCode == errorc.ErrorCodeValid {
			return nil, status.Error(codes.Unauthenticated, "token 已过期，无法续期")
		}
		return nil, status.Error(codes.Internal, "续期 token 失败")
	}

	s.log.WithField("clientKey", client.ClientKey).Info("token 续期成功")

	return &proto.RenewTokenResponse{
		AccessToken: newToken,
		ExpiresAt:   expiresAt,
		TokenType:   "Bearer",
	}, nil
}

// RegisterService 实现 ServiceRegistrar 接口（供 gRPC server 注册使用）
func (s *ClientAuthServiceImpl) RegisterService(server *grpc.Server) error {
	proto.RegisterClientAuthServiceServer(server, s)
	return nil
}

// ServiceName 返回服务名称
func (s *ClientAuthServiceImpl) ServiceName() string {
	return "ClientAuthService"
}

// ServiceVersion 返回服务版本
func (s *ClientAuthServiceImpl) ServiceVersion() string {
	return "v1"
}



