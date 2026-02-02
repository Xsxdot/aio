package security

import (
	"context"
	"strings"
	"time"
	errorc "github.com/xsxdot/aio/pkg/core/err"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type UserAuth struct {
	jwtClient *JwtClient
}

const (
	UserKey        = "user"
	UserSuperAdmin = "ROLE_USER_SUPER_ADMIN"
)

type UserClaims struct {
	jwt.RegisteredClaims
	ID          int64    `json:"id"`
	Username    string   `json:"username,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

func NewUserAuth(secret []byte, expireTime time.Duration) *UserAuth {
	return &UserAuth{
		jwtClient: NewJwtClient(secret, expireTime),
	}
}

// CreateSimpleToken 创建用户token
func (a *UserAuth) CreateSimpleToken(userID int64, userName string) (string, error) {
	claims := &UserClaims{
		ID:       userID,
		Username: userName,
	}
	token, _, err := a.jwtClient.CreateUserToken(claims)
	return token, err
}

func (a *UserAuth) CreateToken(claims *UserClaims) (string, int64, error) {
	return a.jwtClient.CreateUserToken(claims)
}

// NoAuthRequired 无需校验权限，直接放行
func (a *UserAuth) NoAuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

// OptionalAuth 可选校验，有token则验证并保存ID
func (a *UserAuth) OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			claims, err := a.jwtClient.ParseUserToken(token)
			if err == nil {
				a.jwtClient.SaveUserToContext(c, claims)
			}
		}
		return c.Next()
	}
}

// RequireAuth 必须通过校验，并保存ID
func (a *UserAuth) RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			return errorc.New("authorization header is required", nil).NoAuth()
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := a.jwtClient.ParseUserToken(token)
		if err != nil {
			return errorc.New("invalid token", err).NoAuth()
		}

		a.jwtClient.SaveUserToContext(c, claims)
		return c.Next()
	}
}

// RequirePermission 要求特定权限
func (a *UserAuth) RequirePermission(permissionCode string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 先进行基本认证
		auth := c.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			return errorc.New("authorization header is required", nil).NoAuth()
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := a.jwtClient.ParseUserToken(token)
		if err != nil {
			return errorc.New("invalid token", err).NoAuth()
		}

		// 保存用户信息到上下文
		a.jwtClient.SaveUserToContext(c, claims)

		hasPermission := false
		for _, role := range claims.Permissions {
			if role == permissionCode {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return errorc.New("permission denied", nil).Forbidden()
		}

		return c.Next()
	}
}

// GetUserID 从上下文中获取用户ID
func GetUserID(c *fiber.Ctx) (int64, error) {
	if c == nil {
		return 0, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	id, ok := c.Locals("user_id").(int64)
	if !ok || id == 0 {
		return 0, errorc.New("user id not found or invalid", nil).NoAuth()
	}
	return id, nil
}

// GetUserRoles 从上下文中获取用户角色
func GetUserRoles(c *fiber.Ctx) ([]string, error) {
	if c == nil {
		return nil, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	ctx := c.UserContext()
	claims, ok := ctx.Value(UserKey).(*UserClaims)
	if !ok {
		return nil, errorc.New("user claims not found or invalid", nil).NoAuth()
	}
	return claims.Permissions, nil
}

// IsUserSuper 检查用户是否是超级管理员
func IsUserSuper(c *fiber.Ctx) bool {
	if c == nil {
		return false
	}
	isSuper, ok := c.Locals("is_super").(bool)
	return ok && isSuper
}

func GetUserClaimsByCtx(ctx context.Context) (*UserClaims, error) {
	claims, ok := ctx.Value(UserKey).(*UserClaims)
	if !ok {
		return nil, errorc.New("user claims not found or invalid", nil).NoAuth()
	}
	return claims, nil
}

// ParseToken 解析用户令牌（供外部使用）
func (a *UserAuth) ParseToken(token string) (*UserClaims, error) {
	return a.jwtClient.ParseUserToken(token)
}
