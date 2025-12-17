package nginx

import (
	"xiaozhizhang/system/nginx/api/client"
	"xiaozhizhang/system/nginx/internal/app"
	"xiaozhizhang/system/nginx/internal/facade"
)

// ModuleAgentified Nginx Agent化模块门面
type ModuleAgentified struct {
	internalApp *app.AppAgentified
	Client      *client.NginxClientAgentified
}

// NewModuleAgentified 创建 Nginx Agent化模块实例
func NewModuleAgentified(serverFacade facade.IServerFacade) *ModuleAgentified {
	internalApp := app.NewAppAgentified(serverFacade)
	nginxClient := client.NewNginxClientAgentified(internalApp)

	return &ModuleAgentified{
		internalApp: internalApp,
		Client:      nginxClient,
	}
}

