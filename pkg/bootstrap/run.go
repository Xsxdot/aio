package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/xsxdot/aio/pkg/sdk"
)

// CloseFunc 关闭函数类型
type CloseFunc func()

// RegisterCloseFunc 注册关闭回调的函数类型
type RegisterCloseFunc func(CloseFunc)

// App 应用实例
type App struct {
	AppName       string
	Env           string
	Host          string
	Port          int
	Domain        string
	InstanceKey   string
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

	instanceID := bootCfg.Host

	if bootCfg.Aio.Register != nil {
		handle := EnableSDKRegisterSelf(aioClient, *bootCfg, bootCfg.Aio.Register)
		instanceID = handle.InstanceKey
		registerClose(func() {
			if err := handle.Stop(); err != nil {
				log.Printf("[Bootstrap] 停止组件 %s 失败: %v", "Aio Registry", err)
			}
		})
	}

	// 3. 创建 App 实例
	app := &App{
		AppName:       bootCfg.AppName,
		Env:           bootCfg.Env,
		Host:          bootCfg.Host,
		Port:          bootCfg.Port,
		Domain:        bootCfg.Domain,
		InstanceKey:   instanceID,
		SDKClient:     aioClient,
		Components:    nil, // 将通过 AddComponent 添加
		registerClose: registerClose,
	}

	// 4. 初始化所有组件（复用 AddComponent 的逻辑）
	for _, component := range components {
		if err := app.AddComponent(component); err != nil {
			log.Fatalf("[Bootstrap] %v", err)
		}
	}

	return app
}

// AddComponent 动态添加并启动组件
// 该方法复用 Run 函数中的组件初始化流程：
// 1. 获取配置（如果有 ConfigKey）
// 2. 启动组件
// 3. 注册关闭回调
func (a *App) AddComponent(component Component) error {
	// 获取配置（如果有 ConfigKey）
	if component.ConfigKey() != "" {
		err := a.SDKClient.ConfigClient.GetConfigInto(context.TODO(), component.ConfigKey(), component.ConfigPtr())
		if err != nil {
			return fmt.Errorf("获取组件 %s 配置失败: %w", component.Name(), err)
		}
	}

	// 启动组件
	err := component.Start(context.TODO(), component.ConfigPtr())
	if err != nil {
		return fmt.Errorf("启动组件 %s 失败: %w", component.Name(), err)
	}

	// 注册关闭回调
	if a.registerClose != nil {
		a.registerClose(func() {
			if err := component.Stop(); err != nil {
				log.Printf("[Bootstrap] 停止组件 %s 失败: %v", component.Name(), err)
			}
		})
	}

	// 添加到组件列表
	a.Components = append(a.Components, component)

	return nil
}

func EnableSDKRegisterSelf(client *sdk.Client, bootCfg LocalBootstrap, cfg *SdkRegisterConfig) *sdk.RegistrationHandle {
	// 检查必填字段
	if cfg.Project == "" {
		log.Panic("sdk.register.project is required for auto registration")
	}
	if cfg.Name == "" {
		log.Panic("sdk.register.name is required for auto registration")
	}
	if cfg.Owner == "" {
		log.Panic("sdk.register.owner is required for auto registration")
	}

	// 准备服务确保请求（EnsureService）
	svcReq := &sdk.EnsureServiceRequest{
		Project:     cfg.Project,
		Name:        cfg.Name,
		Owner:       cfg.Owner,
		Description: cfg.Description,
		SpecJSON:    cfg.SpecJSON,
	}

	// Description: 为空则使用默认值
	if svcReq.Description == "" {
		svcReq.Description = fmt.Sprintf("%s service", bootCfg.AppName)
	}

	// SpecJSON: 为空则使用默认值
	if svcReq.SpecJSON == "" {
		svcReq.SpecJSON = "{}"
	}

	// 准备实例注册请求（RegisterInstance）
	instReq := &sdk.RegisterInstanceRequest{}

	// InstanceKey: 为空则自动生成
	if cfg.InstanceKey != "" {
		instReq.InstanceKey = cfg.InstanceKey
	} else {
		instReq.InstanceKey = fmt.Sprintf("%s-%s-%d", bootCfg.AppName, bootCfg.Host, time.Now().Unix())
	}

	// Env: 为空则用全局 env
	if cfg.Env != "" {
		instReq.Env = cfg.Env
	} else {
		instReq.Env = bootCfg.Env
	}

	// Host: 为空则用全局 host
	if cfg.Host != "" {
		instReq.Host = cfg.Host
	} else {
		instReq.Host = bootCfg.Host
	}

	// HTTPPort: 使用全局配置
	instReq.HTTPPort = int64(bootCfg.Port)

	// GRPCPort: 使用 sdk.register 配置
	instReq.GRPCPort = int64(cfg.GRPCPort)

	// Endpoint: 为空则自动生成（向后兼容）
	if cfg.Endpoint != "" {
		instReq.Endpoint = cfg.Endpoint
	} else {
		instReq.Endpoint = fmt.Sprintf("%s:%d", bootCfg.Host, bootCfg.Port)
	}

	// Endpoints: 根据配置的网络类型自动检测
	if len(cfg.Networks) > 0 {
		detector := NewIPDetector()
		detected := detector.DetectEndpoints(cfg.Networks)
		if len(detected) > 0 {
			endpoints := make([]sdk.EndpointConfig, 0, len(detected))
			for _, ep := range detected {
				endpoints = append(endpoints, sdk.EndpointConfig{
					Host:     ep.Host,
					Network:  ep.Network,
					Priority: ep.Priority,
				})
			}
			if b, err := json.Marshal(endpoints); err == nil {
				instReq.EndpointsJSON = string(b)
			}
		}
	}

	// MetaJSON: 使用配置中的值，默认为空字符串
	instReq.MetaJSON = cfg.MetaJSON
	if instReq.MetaJSON == "" {
		instReq.MetaJSON = "{}"
	}

	// TTLSeconds: 为 0 则用默认值 60
	if cfg.TTLSeconds > 0 {
		instReq.TTLSeconds = cfg.TTLSeconds
	} else {
		instReq.TTLSeconds = 60
	}

	// 注册到注册中心（使用 EnsureService + RegisterInstance 完整流程）
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	handle, err := client.Registry.RegisterSelfWithEnsureService(ctx, svcReq, instReq)
	if err != nil {
		log.Panic("failed to register self to registry")
	}

	log.Printf("successfully registered to registry, heartbeat started")

	return handle
}
