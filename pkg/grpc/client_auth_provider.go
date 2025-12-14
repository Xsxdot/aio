package grpc

import (
	"fmt"

	"go.uber.org/zap"
)

// TokenParser 令牌解析器接口（依赖倒置）
// 业务组件需要实现此接口来解析自己的 token
type TokenParser interface {
	// ParseToken 解析令牌并返回主体信息
	ParseToken(token string) (subjectID string, subjectType string, name string, extra map[string]interface{}, err error)
}

// ClientAuthProvider 客户端凭证鉴权提供者
// 只支持客户端凭证（Client Key + Client Secret）认证
// 认证通过后自动拥有所有权限
type ClientAuthProvider struct {
	tokenParser TokenParser
	log         *zap.Logger
}

// NewClientAuthProvider 创建客户端鉴权提供者
func NewClientAuthProvider(tokenParser TokenParser, logger *zap.Logger) *ClientAuthProvider {
	return &ClientAuthProvider{
		tokenParser: tokenParser,
		log:         logger,
	}
}

// VerifyToken 验证客户端令牌并返回认证信息
func (p *ClientAuthProvider) VerifyToken(token string) (*AuthInfo, error) {
	// 通过 TokenParser 解析 token
	subjectID, subjectType, name, extra, err := p.tokenParser.ParseToken(token)
	if err != nil {
		p.log.Warn("客户端 token 解析失败", zap.Error(err))
		return nil, fmt.Errorf("无效的客户端令牌")
	}

	// 构建认证信息
	authInfo := &AuthInfo{
		SubjectID:   subjectID,
		SubjectType: subjectType,
		Name:        name,
		Roles:       []string{"client"},
		// 客户端认证通过后拥有所有权限
		Permissions: []Permission{
			{
				Resource: "*",
				Action:   "*",
			},
		},
		Extra: extra,
	}

	p.log.Debug("客户端 token 验证成功",
		zap.String("subject_id", subjectID),
		zap.String("subject_type", subjectType))

	return authInfo, nil
}

// VerifyPermission 验证权限
// 客户端认证通过后自动允许所有操作
func (p *ClientAuthProvider) VerifyPermission(token string, resource, action string) (*PermissionResult, error) {
	// 只需验证 token 是否有效
	_, err := p.VerifyToken(token)
	if err != nil {
		return &PermissionResult{
			Allowed: false,
			Reason:  "客户端令牌无效",
		}, err
	}

	// 客户端认证通过后自动允许所有操作
	return &PermissionResult{
		Allowed: true,
		Reason:  "客户端拥有所有权限",
	}, nil
}



