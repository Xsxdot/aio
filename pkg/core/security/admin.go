package security

import (
	"context"
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type AdminAuth struct {
	jwtClient *JwtClient
}

const AdminKey = "admin"

type AdminClaims struct {
	jwt.RegisteredClaims
	ID        int64    `json:"id"`
	Account   string   `json:"account,omitempty"`
	AdminType []string `json:"admin_type"`
}

func NewAdminAuth(secret []byte, expireTime time.Duration) *AdminAuth {
	return &AdminAuth{
		jwtClient: NewJwtClient(secret, expireTime),
	}
}

// CreateAdminToken 创建管理员token
func (a *AdminAuth) CreateAdminToken(claims *AdminClaims) (string, int64, error) {
	return a.jwtClient.CreateToken(claims)
}

// RequireAdminAuth 管理员权限校验中间件
func (a *AdminAuth) RequireAdminAuth(requiredRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			return errorc.New("authorization header is required", nil).NoAuth()
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := a.jwtClient.ParseToken(token)
		if err != nil {
			return errorc.New("invalid token", err).NoAuth()
		}

		// 保存管理员信息到上下文
		a.jwtClient.SaveToContext(c, claims)

		// 超级管理员跳过权限校验，直接放行
		if IsAdminSuper(c) {
			return c.Next()
		}

		// 非超级管理员，校验具体权限
		if err := a.jwtClient.ValidateRoles(c, requiredRoles); err != nil {
			return errorc.New("permission denied", err).Forbidden()
		}
		return c.Next()
	}
}

// GetAdminID 获取管理员ID
func (a *AdminAuth) GetAdminID(c *fiber.Ctx) (int64, error) {
	if c == nil {
		return 0, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	id, ok := c.Locals("user_id").(int64)
	if !ok || id == 0 {
		return 0, errorc.New("admin id not found or invalid", nil).NoAuth()
	}
	return id, nil
}

// GetAdminAccount 获取管理员账号
func (a *AdminAuth) GetAdminAccount(c *fiber.Ctx) (string, error) {
	if c == nil {
		return "", errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	account, ok := c.Locals("account").(string)
	if !ok || account == "" {
		return "", errorc.New("admin account not found or empty", nil).NoAuth()
	}
	return account, nil
}

// GetAdminRoles 获取管理员角色列表
func (a *AdminAuth) GetAdminRoles(c *fiber.Ctx) ([]string, error) {
	if c == nil {
		return nil, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	roles, ok := c.Locals("roles").([]string)
	if !ok {
		return nil, errorc.New("admin roles not found", nil).NoAuth()
	}
	return roles, nil
}

// IsAdminSuper 判断是否为超级管理员
func IsAdminSuper(c *fiber.Ctx) bool {
	if c == nil {
		return false
	}
	isSuper, ok := c.Locals("is_super").(bool)
	if !ok {
		return false
	}
	return isSuper
}

func GetAdminId(ctx *fiber.Ctx) (int64, error) {
	if ctx == nil {
		return 0, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	return GetAdminIDByCtx(ctx.UserContext())
}

func GetAdminRoles(ctx *fiber.Ctx) ([]string, error) {
	if ctx == nil {
		return nil, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	roles, ok := ctx.Locals("roles").([]string)
	if !ok {
		return nil, errorc.New("admin roles not found", nil).NoAuth()
	}
	return roles, nil
}

func GetAdminAccount(ctx *fiber.Ctx) (string, error) {
	if ctx == nil {
		return "", errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	return GetAdminAccountByCtx(ctx.UserContext())
}

func GetAdminClaimsByCtx(ctx context.Context) (*AdminClaims, error) {
	claims, ok := ctx.Value(AdminKey).(*AdminClaims)
	if !ok {
		return nil, errorc.New("admin claims not found or invalid", nil).NoAuth()
	}
	return claims, nil
}

func GetAdminIDByCtx(ctx context.Context) (int64, error) {
	claims, err := GetAdminClaimsByCtx(ctx)
	if err != nil {
		return 0, err
	}
	return claims.ID, nil
}

func GetAdminAccountByCtx(ctx context.Context) (string, error) {
	claims, err := GetAdminClaimsByCtx(ctx)
	if err != nil {
		return "", err
	}
	return claims.Account, nil
}

// ParseToken 解析管理员令牌（供外部使用）
func (a *AdminAuth) ParseToken(token string) (*AdminClaims, error) {
	return a.jwtClient.ParseToken(token)
}
