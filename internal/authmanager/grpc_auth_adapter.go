package authmanager

import (
	"fmt"

	grpcserver "github.com/xsxdot/aio/internal/grpc"
)

// AuthManagerAdapter AuthManager 适配器，实现 grpc.AuthProvider 接口
type AuthManagerAdapter struct {
	manager *AuthManager
}

// NewAuthManagerAdapter 创建 AuthManager 适配器
func NewAuthManagerAdapter(manager *AuthManager) *AuthManagerAdapter {
	return &AuthManagerAdapter{
		manager: manager,
	}
}

// VerifyToken 验证令牌并返回认证信息
func (a *AuthManagerAdapter) VerifyToken(token string) (*grpcserver.AuthInfo, error) {
	// 使用 JWT 服务验证令牌
	authInfo, err := a.manager.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// 检查主体是否已禁用
	subject, err := a.manager.storage.GetSubject(authInfo.SubjectID)
	if err != nil {
		return nil, fmt.Errorf("subject not found: %w", err)
	}

	if subject.Disabled {
		return nil, fmt.Errorf("subject is disabled")
	}

	// 转换权限格式
	permissions := make([]grpcserver.Permission, len(authInfo.Permissions))
	for i, perm := range authInfo.Permissions {
		permissions[i] = grpcserver.Permission{
			Resource: perm.Resource,
			Action:   perm.Action,
		}
	}

	// 构造 gRPC 认证信息
	grpcAuthInfo := &grpcserver.AuthInfo{
		SubjectID:   authInfo.SubjectID,
		SubjectType: string(authInfo.SubjectType),
		Name:        authInfo.Name,
		Roles:       authInfo.Roles,
		Permissions: permissions,
		Extra:       authInfo.Extra,
	}

	return grpcAuthInfo, nil
}

// VerifyPermission 验证权限
func (a *AuthManagerAdapter) VerifyPermission(token string, resource, action string) (*grpcserver.PermissionResult, error) {
	// 调用 AuthManager 的权限验证方法
	result, err := a.manager.VerifyPermission(token, resource, action)
	if err != nil {
		return nil, fmt.Errorf("permission verification failed: %w", err)
	}

	// 转换结果格式
	grpcResult := &grpcserver.PermissionResult{
		Allowed: result.Allowed,
		Reason:  result.Reason,
	}

	return grpcResult, nil
}
