package protocol

import (
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/internal/authmanager"
)

type SystemHandle struct {
	authManager *authmanager.AuthManager
	tokens      map[string]string
}

func NewSystemHandle(manager *authmanager.AuthManager) *SystemHandle {
	return &SystemHandle{
		authManager: manager,
		tokens:      make(map[string]string),
	}
}

func (h *SystemHandle) Auth(connId string, msg *CustomMessage) (interface{}, error) {
	if h.authManager == nil {
		return nil, fmt.Errorf("auth manager is unable")
	}

	// 解析认证请求
	var authReq authmanager.ClientAuthRequest
	err := json.Unmarshal(msg.Payload(), &authReq)
	if err != nil {
		return nil, fmt.Errorf("unmarshal auth request failed: %w", err)
	}

	// 进行认证
	token, err := h.authManager.AuthenticateClient(authReq)
	if err != nil {
		return nil, err
	}

	// 认证成功，保存令牌
	h.tokens[connId] = token.AccessToken

	return token, nil
}

func (h *SystemHandle) ValidAuth(connID string, msg *CustomMessage) (interface{}, error) {
	if h.authManager == nil {
		return nil, nil
	}

	if msg.Header().ServiceType == ServiceTypeSystem || msg.Header().MessageType == MsgTypeAuth {
		return nil, nil
	}
	// 检查连接是否已认证
	if _, ok := h.tokens[connID]; !ok {
		return nil, fmt.Errorf("connection not authenticated")
	} else {
		//todo 现在只需要验证token
		// 已认证的连接，验证操作权限
		//resource := fmt.Sprintf("service.%s", msg.Header().ServiceType)
		//action := fmt.Sprintf("message.%s", msg.Header().MessageType)
		//
		//// 验证权限
		//verifyResp, err := m.VerifyPermission(conn.ID(), resource, action)
		//if err != nil {
		//	return fmt.Errorf("verify permission failed: %w", err)
		//}
		//
		//if !verifyResp.Allowed {
		//	return fmt.Errorf("permission denied: %s", verifyResp.Reason)
		//}
		return nil, nil
	}
}

func (h *SystemHandle) Heartbeat(connId string, msg *CustomMessage) (interface{}, error) {
	return OK, nil
}

func (h *SystemHandle) removeToken(connID string) {
	delete(h.tokens, connID)
}
