package app

import (
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/registry"
	"xiaozhizhang/system/server"
	"xiaozhizhang/system/shorturl"
	"xiaozhizhang/system/ssl"
	"xiaozhizhang/system/user"
)

// App 是应用根对象（Application Root），
// 负责组合各个业务组件的 Module/Client。
// 后续可以在这里按需增加字段，例如：
//
//	ConfigModule *config.Module
//	UserClient   *userclient.UserClient
//
// 等等。
type App struct {
	// ConfigModule 配置中心模块
	// 通过 ConfigModule.Client 可以跨组件调用配置能力
	ConfigModule *config.Module
	// RegistryModule 注册中心模块
	RegistryModule *registry.Module
	// UserModule 用户组件模块
	// 包含用户认证、客户端凭证等能力
	UserModule *user.Module
	// SslModule SSL 证书组件模块
	// 提供证书申请、续期、部署能力
	SslModule *ssl.Module
	// ServerModule 服务器管理组件模块
	// 提供服务器清单管理、状态上报与聚合查询能力
	ServerModule *server.Module
	// ShortURLModule 短网址组件模块
	// 提供短链接生成、跳转、统计能力
	ShortURLModule *shorturl.Module
}

// NewApp 创建进程唯一的 App 实例，由 main.go 在启动时调用。
// 这里可以使用 base 包中的基础设施实例（DB、Logger 等）
// 注入到各个 system 组件的 Module 或 Client。
func NewApp() *App {
	// 先创建基础组件（注意顺序：server 要在 ssl 之前）
	configModule := config.NewModule()
	registryModule := registry.NewModule()
	userModule := user.NewModule()

	// 创建 server 组件（ssl 需要依赖它）
	serverModule := server.NewModule()

	// 创建 ssl 组件，注入 server client
	sslModule := ssl.NewModule(serverModule.Client)

	// 创建短网址组件
	shorturlModule := shorturl.NewModule()

	return &App{
		ConfigModule:   configModule,
		RegistryModule: registryModule,
		UserModule:     userModule,
		SslModule:      sslModule,
		ServerModule:   serverModule,
		ShortURLModule: shorturlModule,
	}
}
