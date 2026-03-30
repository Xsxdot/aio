package bootstrap

import (
	"context"
	"log"

	"github.com/xsxdot/aio/pkg/sdk"
)

// CloseFunc 关闭函数类型
type CloseFunc func()

// RegisterCloseFunc 注册关闭回调的函数类型
type RegisterCloseFunc func(CloseFunc)

// App 应用实例
type App struct {
	AppName       string `yaml:"app-name"`
	Env           string `yaml:"env"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	Domain        string `yaml:"domain"`
	LogLevel      string `yaml:"log-level"`
	SDKClient     *sdk.Client
	Components    []Component
	registerClose RegisterCloseFunc
}

// Run 启动引擎
// bootFilePath: 本地 bootstrap.yaml 的路径
// components: 组件列表
// registerClose: 注册关闭回调的函数（依赖注入）
func Run(bootFilePath string, components []Component, registerClose RegisterCloseFunc) *App {
	// 1. 读取本地引导配置
	bootCfg, err := loadBootstrap(bootFilePath)
	if err != nil {
		log.Fatalf("[Bootstrap] 读取引导配置失败: %v", err)
	}

	// 2. 使用引导配置初始化 AIO SDK 客户端
	aioClient, err := sdk.New(sdk.Config{
		RegistryAddr:   bootCfg.Aio.RegistryAddr,
		ClientKey:      bootCfg.Aio.ClientKey,
		ClientSecret:   bootCfg.Aio.ClientSecret,
		Env:            bootCfg.Env,
		DisableAuth:    bootCfg.Aio.DisableAuth,
		DefaultTimeout: bootCfg.Aio.DefaultTimeout,
	})
	if err != nil {
		log.Fatalf("[Bootstrap] 初始化 SDK 失败: %v", err)
	}

	cfgClient := aioClient.ConfigClient
	for _, component := range components {
		// 获取配置（如果有 ConfigKey）
		if component.ConfigKey() != "" {
			err := cfgClient.GetConfigInto(context.TODO(), component.ConfigKey(), component.ConfigPtr())
			if err != nil {
				log.Fatalf("[Bootstrap] 获取组件 %s 配置失败: %v", component.Name(), err)
			}
		}

		// 启动组件
		err := component.Start(context.TODO(), component.ConfigPtr())
		if err != nil {
			log.Fatalf("[Bootstrap] 启动组件 %s 失败: %v", component.Name(), err)
		}

		// 注册关闭回调（通过依赖注入）
		if registerClose != nil {
			registerClose(func() {
				if err := component.Stop(); err != nil {
					log.Printf("[Bootstrap] 停止组件 %s 失败: %v", component.Name(), err)
				}
			})
		}
	}

	return &App{
		AppName:       bootCfg.AppName,
		Env:           bootCfg.Env,
		Host:          bootCfg.Host,
		Port:          bootCfg.Port,
		Domain:        bootCfg.Domain,
		LogLevel:      bootCfg.LogLevel,
		SDKClient:     aioClient,
		Components:    components,
		registerClose: registerClose,
	}
}