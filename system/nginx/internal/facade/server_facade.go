package facade

import (
	"context"

	serverdto "xiaozhizhang/system/server/api/dto"
)

// IServerFacade Server 组件门面接口
// nginx 组件通过此接口获取服务器信息（agent 地址等）
type IServerFacade interface {
	// GetServerAgentInfo 获取服务器 Agent 信息
	GetServerAgentInfo(ctx context.Context, serverID int64) (*serverdto.ServerAgentInfo, error)
}

