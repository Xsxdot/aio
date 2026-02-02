package user

import (
	"fmt"
	"github.com/xsxdot/aio/system/user/internal/service"
)

// TokenParserAdapter JWT Token 解析器适配器
// 实现 grpc.TokenParser 接口，用于将 internal 层的 JwtService 适配到 gRPC 层
type TokenParserAdapter struct {
	jwtService *service.JwtService
}

// NewTokenParserAdapter 创建 Token 解析器适配器
func NewTokenParserAdapter(jwtService *service.JwtService) *TokenParserAdapter {
	return &TokenParserAdapter{
		jwtService: jwtService,
	}
}

// ParseToken 解析令牌并返回主体信息
// 实现 grpc.TokenParser 接口
func (a *TokenParserAdapter) ParseToken(token string) (subjectID string, subjectType string, name string, extra map[string]interface{}, err error) {
	// 解析客户端 token
	claims, err := a.jwtService.ParseClientToken(token)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("解析客户端 token 失败: %w", err)
	}

	// 将 claims 转换为通用格式
	subjectID = fmt.Sprintf("%d", claims.ClientID)
	subjectType = "client"
	name = claims.ClientKey
	extra = map[string]interface{}{
		"client_id":  claims.ClientID,
		"client_key": claims.ClientKey,
		"status":     claims.Status,
	}

	return subjectID, subjectType, name, extra, nil
}



