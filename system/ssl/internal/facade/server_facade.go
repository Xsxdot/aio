package facade

import (
	"context"
	serverdto "github.com/xsxdot/aio/system/server/api/dto"
)

// IServerFacade server 组件对 SSL 组件提供的外观接口
// 用于隔离 SSL 对 server 组件的依赖
// server/api/client.ServerClient 隐式实现了此接口
type IServerFacade interface {
	// GetServerSSHConfigByID 获取服务器的 SSH 连接配置（已解密）
	// 返回 host + SSH 凭证信息，供部署证书时使用
	GetServerSSHConfigByID(ctx context.Context, serverID int64) (*serverdto.ServerSSHConfig, error)
}
