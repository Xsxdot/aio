package authmanager

import (
	"context"
	"errors"

	grpcserver "github.com/xsxdot/aio/internal/grpc"
)

var (
	// ErrNoAuthInfo 无认证信息错误
	ErrNoAuthInfo = errors.New("no authentication information found in context")
	// ErrNoToken 无令牌错误
	ErrNoToken = errors.New("no token found in context")
)

// GetAuthInfoFromContext 从 gRPC 上下文中获取认证信息
func GetAuthInfoFromContext(ctx context.Context) (*grpcserver.AuthInfo, error) {
	authInfo, ok := ctx.Value("authInfo").(*grpcserver.AuthInfo)
	if !ok || authInfo == nil {
		return nil, ErrNoAuthInfo
	}
	return authInfo, nil
}

// GetTokenFromContext 从 gRPC 上下文中获取令牌
func GetTokenFromContext(ctx context.Context) (string, error) {
	token, ok := ctx.Value("token").(string)
	if !ok || token == "" {
		return "", ErrNoToken
	}
	return token, nil
}

// GetUserIDFromContext 从 gRPC 上下文中获取用户ID
func GetUserIDFromContext(ctx context.Context) (string, error) {
	authInfo, err := GetAuthInfoFromContext(ctx)
	if err != nil {
		return "", err
	}
	return authInfo.SubjectID, nil
}

// GetUserNameFromContext 从 gRPC 上下文中获取用户名
func GetUserNameFromContext(ctx context.Context) (string, error) {
	authInfo, err := GetAuthInfoFromContext(ctx)
	if err != nil {
		return "", err
	}
	return authInfo.Name, nil
}

// GetUserRolesFromContext 从 gRPC 上下文中获取用户角色
func GetUserRolesFromContext(ctx context.Context) ([]string, error) {
	authInfo, err := GetAuthInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return authInfo.Roles, nil
}

// HasRoleInContext 检查上下文中的用户是否拥有指定角色
func HasRoleInContext(ctx context.Context, role string) (bool, error) {
	roles, err := GetUserRolesFromContext(ctx)
	if err != nil {
		return false, err
	}

	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}
	return false, nil
}

// IsAdminInContext 检查上下文中的用户是否为管理员
func IsAdminInContext(ctx context.Context) (bool, error) {
	return HasRoleInContext(ctx, "admin")
}

// RequireAuth 中间件级别的认证检查，如果没有认证信息则返回错误
func RequireAuth(ctx context.Context) (*grpcserver.AuthInfo, error) {
	authInfo, err := GetAuthInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return authInfo, nil
}

// RequireRole 中间件级别的角色检查，如果用户没有指定角色则返回错误
func RequireRole(ctx context.Context, role string) (*grpcserver.AuthInfo, error) {
	authInfo, err := RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	hasRole, err := HasRoleInContext(ctx, role)
	if err != nil {
		return nil, err
	}

	if !hasRole {
		return nil, errors.New("insufficient role privileges")
	}

	return authInfo, nil
}

// RequireAdmin 中间件级别的管理员检查
func RequireAdmin(ctx context.Context) (*grpcserver.AuthInfo, error) {
	return RequireRole(ctx, "admin")
}
