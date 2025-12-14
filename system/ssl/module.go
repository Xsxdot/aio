package ssl

import (
	"context"
	"xiaozhizhang/system/ssl/api/client"
	"xiaozhizhang/system/ssl/internal/app"
)

// Module SSL 证书组件模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用 SSL 证书能力
	Client *client.SslClient
}

// NewModule 创建 SSL 证书模块实例
func NewModule() *Module {
	internalApp := app.NewApp()
	sslClient := client.NewSslClient(internalApp)

	return &Module{
		internalApp: internalApp,
		Client:      sslClient,
	}
}

// RenewDueCertificates 扫描并续期即将过期的证书
// 供调度器任务调用
func (m *Module) RenewDueCertificates(ctx context.Context) error {
	return m.internalApp.RenewDueCertificates(ctx)
}
