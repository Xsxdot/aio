package app

import (
	"xiaozhizhang/system/application"
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/nginx"
	"xiaozhizhang/system/registry"
	"xiaozhizhang/system/server"
	"xiaozhizhang/system/ssl"
	"xiaozhizhang/system/systemd"
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
	// NginxModule Nginx 管理组件模块
	// 提供远程 Nginx 配置文件 CRUD、校验、重载能力
	NginxModule *nginx.Module
	// SystemdModule Systemd 管理组件模块
	// 提供本机 systemd service 的 CRUD、启停、启用禁用、状态日志查询能力
	SystemdModule *systemd.Module
	// ApplicationModule 应用部署组件模块
	// 提供应用的自动化部署、更新、回滚能力
	ApplicationModule *application.Module
	// ServerModule 服务器管理组件模块
	// 提供服务器清单管理、状态上报与聚合查询能力
	ServerModule *server.Module
}

// NewApp 创建进程唯一的 App 实例，由 main.go 在启动时调用。
// 这里可以使用 base 包中的基础设施实例（DB、Logger 等）
// 注入到各个 system 组件的 Module 或 Client。
func NewApp() *App {
	// 先创建基础组件
	configModule := config.NewModule()
	registryModule := registry.NewModule()
	userModule := user.NewModule()
	sslModule := ssl.NewModule()
	nginxModule := nginx.NewModule()
	systemdModule := systemd.NewModule()

	// 创建 application 组件，需要注入依赖的其他组件
	applicationModule := application.NewModule(
		sslModule,
		nginxModule,
		systemdModule,
		registryModule,
	)

	// 创建 server 组件
	serverModule := server.NewModule()

	return &App{
		ConfigModule:      configModule,
		RegistryModule:    registryModule,
		UserModule:        userModule,
		SslModule:         sslModule,
		NginxModule:       nginxModule,
		SystemdModule:     systemdModule,
		ApplicationModule: applicationModule,
		ServerModule:      serverModule,
	}
}
