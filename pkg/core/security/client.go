package security

import (
	"context"
	"strings"

	errorc "github.com/xsxdot/aio/pkg/core/err"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type ClientAuth struct {
	secret []byte
}

const (
	ClientKey = "client"
)

type ClientClaims struct {
	jwt.RegisteredClaims
	ClientID  int64  `json:"clientId"`
	ClientKey string `json:"clientKey"`
	Status    int8   `json:"status"`
}

func NewClientAuth(secret []byte) *ClientAuth {
	return &ClientAuth{secret: secret}
}

func (a *ClientAuth) ParseToken(tokenString string) (*ClientClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ClientClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errorc.New("无效的签名方法", nil).NoAuth()
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, errorc.New("invalid token", err).NoAuth()
	}
	claims, ok := token.Claims.(*ClientClaims)
	if !ok || !token.Valid {
		return nil, errorc.New("invalid token", nil).NoAuth()
	}
	return claims, nil
}

func (a *ClientAuth) SaveClientToContext(c *fiber.Ctx, claims *ClientClaims) {
	c.Locals("client_id", claims.ClientID)
	c.Locals("client_key", claims.ClientKey)
	c.Locals("client_status", claims.Status)

	userCtx := c.UserContext()
	userCtx = context.WithValue(userCtx, ClientKey, claims)
	c.SetUserContext(userCtx)
}

// RequireClientAuth 必须通过校验，并保存 client 信息
func (a *ClientAuth) RequireClientAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			return errorc.New("authorization header is required", nil).NoAuth()
		}

		tokenString := strings.TrimPrefix(auth, "Bearer ")
		claims, err := a.ParseToken(tokenString)
		if err != nil {
			return err
		}
		a.SaveClientToContext(c, claims)
		return c.Next()
	}
}

func GetClientID(c *fiber.Ctx) (int64, error) {
	if c == nil {
		return 0, errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	id, ok := c.Locals("client_id").(int64)
	if !ok || id == 0 {
		return 0, errorc.New("client id not found or invalid", nil).NoAuth()
	}
	return id, nil
}

func GetClientKey(c *fiber.Ctx) (string, error) {
	if c == nil {
		return "", errorc.New("fiber context is nil", nil).WithCode(errorc.ErrorCodeInternal)
	}
	key, ok := c.Locals("client_key").(string)
	if !ok || key == "" {
		return "", errorc.New("client key not found or invalid", nil).NoAuth()
	}
	return key, nil
}

func GetClientClaimsByCtx(ctx context.Context) (*ClientClaims, error) {
	claims, ok := ctx.Value(ClientKey).(*ClientClaims)
	if !ok {
		return nil, errorc.New("client claims not found or invalid", nil).NoAuth()
	}
	return claims, nil
}
