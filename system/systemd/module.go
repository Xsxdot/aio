package systemd

import (
	"xiaozhizhang/system/systemd/api/client"
	"xiaozhizhang/system/systemd/internal/app"
)

// Module Systemd 管理模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用 Systemd 管理能力
	Client *client.SystemdClient
}

// NewModule 创建 Systemd 管理模块实例
func NewModule() *Module {
	internalApp := app.NewApp()
	systemdClient := client.NewSystemdClient(internalApp)

	return &Module{
		internalApp: internalApp,
		Client:      systemdClient,
	}
}

