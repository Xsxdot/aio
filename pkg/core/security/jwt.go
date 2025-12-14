package security

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type JwtClient struct {
	secret     []byte
	expireTime time.Duration
}

type Claims struct {
	jwt.RegisteredClaims
	ID      int64    `json:"id"`
	Account string   `json:"account,omitempty"`
	Roles   []string `json:"roles,omitempty"`
	IsSuper bool     `json:"is_super,omitempty"`
}

func NewJwtClient(secret []byte, expireTime time.Duration) *JwtClient {
	if expireTime <= 0 {
		expireTime = 24 * time.Hour
	}
	return &JwtClient{
		secret:     secret,
		expireTime: expireTime,
	}
}

func (c *JwtClient) CreateToken(claims *AdminClaims) (string, int64, error) {
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(c.expireTime))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, err := token.SignedString(c.secret)
	return signedString, claims.ExpiresAt.Unix(), err
}

func (c *JwtClient) CreateUserToken(claims *UserClaims) (string, int64, error) {
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(c.expireTime))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, err := token.SignedString(c.secret)
	return signedString, claims.ExpiresAt.Unix(), err
}

func (c *JwtClient) ParseToken(tokenString string) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		return c.secret, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (c *JwtClient) ParseUserToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return c.secret, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (c *JwtClient) SaveToContext(ctx *fiber.Ctx, claims *AdminClaims) {
	ctx.Locals("user_id", claims.ID)
	if claims.Account != "" {
		ctx.Locals("account", claims.Account)
	}
	if len(claims.AdminType) > 0 {
		ctx.Locals("roles", claims.AdminType)
	}

	for _, s := range claims.AdminType {
		if s == "SuperAdmin" {
			ctx.Locals("is_super", true)
			break
		}
	}

	userCtx := ctx.UserContext()
	userCtx = context.WithValue(userCtx, AdminKey, claims)
	ctx.SetUserContext(userCtx)
}

func (c *JwtClient) SaveUserToContext(ctx *fiber.Ctx, claims *UserClaims) {
	ctx.Locals("user_id", claims.ID)
	userCtx := ctx.UserContext()
	userCtx = context.WithValue(userCtx, UserKey, claims)
	ctx.SetUserContext(userCtx)
}

func (c *JwtClient) ValidateRoles(ctx *fiber.Ctx, requiredRoles []string) error {
	if len(requiredRoles) == 0 {
		return nil
	}

	isSuper, _ := ctx.Locals("is_super").(bool)
	if isSuper {
		return nil
	}

	userRoles, _ := ctx.Locals("roles").([]string)
	for _, required := range requiredRoles {
		hasRole := false
		for _, userRole := range userRoles {
			if required == userRole {
				hasRole = true
				break
			}
		}
		if !hasRole {
			return fiber.ErrForbidden
		}
	}
	return nil
}

// JWTMiddleware 返回JWT认证中间件
func JWTMiddleware() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		// 从请求头获取token
		tokenString := ctx.Get("Authorization")
		if tokenString == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "未提供认证信息")
		}

		// 移除Bearer前缀
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		// 创建JWT客户端
		jwtClient := NewJwtClient([]byte("your-secret-key"), 24*time.Hour)

		// 解析token
		claims, err := jwtClient.ParseToken(tokenString)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "无效的认证信息")
		}

		// 存储用户信息到上下文
		jwtClient.SaveToContext(ctx, claims)

		return ctx.Next()
	}
}
