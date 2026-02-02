package service

import (
	"context"
	"time"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"

	"github.com/golang-jwt/jwt/v5"
)

// ClientClaims 客户端 JWT Claims
type ClientClaims struct {
	jwt.RegisteredClaims
	ClientID  int64  `json:"clientId"`
	ClientKey string `json:"clientKey"`
	Status    int8   `json:"status"`
}

// JwtService JWT 服务（用于客户端认证）
type JwtService struct {
	secret     []byte
	expireTime time.Duration
	log        *logger.Log
	err        *errorc.ErrorBuilder
}

// NewJwtService 创建 JWT 服务实例
func NewJwtService(secret string, expireTime time.Duration, log *logger.Log) *JwtService {
	if expireTime <= 0 {
		expireTime = 24 * time.Hour // 默认 24 小时
	}
	return &JwtService{
		secret:     []byte(secret),
		expireTime: expireTime,
		log:        log,
		err:        errorc.NewErrorBuilder("JwtService"),
	}
}

// CreateClientToken 创建客户端 token
func (s *JwtService) CreateClientToken(clientID int64, clientKey string, status int8) (string, int64, error) {
	now := time.Now()
	claims := &ClientClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expireTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "xiaozhizhang",
			Subject:   "client",
		},
		ClientID:  clientID,
		ClientKey: clientKey,
		Status:    status,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, err := token.SignedString(s.secret)
	if err != nil {
		return "", 0, s.err.New("创建 token 失败", err)
	}

	return signedString, claims.ExpiresAt.Unix(), nil
}

// ParseClientToken 解析客户端 token
func (s *JwtService) ParseClientToken(tokenString string) (*ClientClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ClientClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, s.err.New("无效的签名方法", nil)
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, s.err.New("解析 token 失败", err)
	}

	if claims, ok := token.Claims.(*ClientClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, s.err.New("无效的 token", nil)
}

// RenewClientToken 续期客户端 token（在未过期时生成新 token）
func (s *JwtService) RenewClientToken(tokenString string) (string, int64, error) {
	// 解析当前 token
	claims, err := s.ParseClientToken(tokenString)
	if err != nil {
		return "", 0, err
	}

	// 检查是否已过期
	if time.Now().After(claims.ExpiresAt.Time) {
		return "", 0, s.err.New("token 已过期，无法续期", nil).ValidWithCtx()
	}

	// 生成新 token（使用原有的客户端信息）
	return s.CreateClientToken(claims.ClientID, claims.ClientKey, claims.Status)
}

// SaveClientToContext 将客户端信息保存到 context
func (s *JwtService) SaveClientToContext(ctx context.Context, claims *ClientClaims) context.Context {
	ctx = context.WithValue(ctx, "client_id", claims.ClientID)
	ctx = context.WithValue(ctx, "client_key", claims.ClientKey)
	ctx = context.WithValue(ctx, "client_status", claims.Status)
	ctx = context.WithValue(ctx, "client_claims", claims)
	return ctx
}



