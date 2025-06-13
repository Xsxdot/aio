package authmanager

import (
	"context"

	authv1 "github.com/xsxdot/aio/api/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCService 是认证服务的 gRPC 实现
type GRPCService struct {
	authv1.UnimplementedAuthServiceServer
	manager *AuthManager
}

// NewGRPCService 创建新的认证 gRPC 服务
func NewGRPCService(manager *AuthManager) *GRPCService {
	return &GRPCService{
		manager: manager,
	}
}

// RegisterService 实现 ServiceRegistrar 接口
func (s *GRPCService) RegisterService(server *grpc.Server) error {
	authv1.RegisterAuthServiceServer(server, s)
	return nil
}

// ServiceName 返回服务名称
func (s *GRPCService) ServiceName() string {
	return "auth.v1.AuthService"
}

// ServiceVersion 返回服务版本
func (s *GRPCService) ServiceVersion() string {
	return "v1.0.0"
}

// ClientAuth 实现客户端认证 gRPC 接口
func (s *GRPCService) ClientAuth(ctx context.Context, req *authv1.ClientAuthRequest) (*authv1.ClientAuthResponse, error) {
	// 验证请求参数
	if req.ClientId == "" || req.ClientSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "客户端ID和密钥为必填项")
	}

	// 构造内部认证请求
	authReq := ClientAuthRequest{
		ClientID:     req.ClientId,
		ClientSecret: req.ClientSecret,
	}

	// 调用认证管理器进行认证
	token, err := s.manager.AuthenticateClient(authReq)
	if err != nil {
		// 将内部错误转换为 gRPC 状态码
		switch err {
		case ErrInvalidCredentials:
			return nil, status.Error(codes.Unauthenticated, "无效的客户端凭证")
		case ErrUserDisabled:
			return nil, status.Error(codes.PermissionDenied, "客户端已被禁用")
		default:
			return nil, status.Error(codes.Internal, "认证失败")
		}
	}

	// 构造响应
	response := &authv1.ClientAuthResponse{
		AccessToken: token.AccessToken,
		ExpiresIn:   token.ExpiresIn,
		TokenType:   token.TokenType,
	}

	return response, nil
}
