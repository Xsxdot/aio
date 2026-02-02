package registry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/scheduler"
	"github.com/xsxdot/aio/system/registry/api/client"
	"github.com/xsxdot/aio/system/registry/api/dto"
	grpcsvc "github.com/xsxdot/aio/system/registry/external/grpc"
	"github.com/xsxdot/aio/system/registry/internal/app"
)

// Module 注册中心组件模块门面（对外暴露的根对象）
// 封装了内部 app 和对外 client，只暴露需要的能力
type Module struct {
	// internalApp 内部应用实例，不对外暴露，仅供组件内部使用
	internalApp *app.App
	// Client 对外客户端，供其他组件调用注册中心能力
	Client *client.RegistryClient
	// GRPCService gRPC服务实例，供gRPC服务器注册使用
	GRPCService *grpcsvc.RegistryService

	// 自注册信息（私有字段，仅供心跳任务使用）
	selfServiceID   int64
	selfInstanceKey string
}

// NewModule 创建注册中心模块实例
func NewModule() *Module {
	internalApp := app.NewApp()
	registryClient := client.NewRegistryClient(internalApp)
	grpcService := grpcsvc.NewRegistryService(registryClient, base.Logger)

	m := &Module{
		internalApp: internalApp,
		Client:      registryClient,
		GRPCService: grpcService,
	}

	// 自动注册本程序到注册中心
	m.autoRegisterSelf()

	return m
}

// autoRegisterSelf 自动将本控制面程序注册到注册中心
// 失败只记录 warn，不阻塞启动
func (m *Module) autoRegisterSelf() {
	log := base.Logger.WithEntryName("RegistrySelfRegister")

	// 1. 准备 service 信息
	appName := base.Configures.Config.AppName
	if appName == "" {
		appName = "aio" // fallback 默认值
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. EnsureService（project/name 都使用 appName）
	svcReq := &dto.CreateServiceReq{
		Project:     appName,
		Name:        appName,
		Owner:       "system",
		Description: fmt.Sprintf("%s control plane", appName),
		Spec: map[string]interface{}{
			"type": "control-plane",
		},
	}

	svc, isNew, err := m.Client.EnsureService(ctx, svcReq)
	if err != nil {
		log.WithErr(err).Warn("自注册：EnsureService 失败")
		return
	}

	if isNew {
		log.WithField("serviceID", svc.ID).Info("自注册：创建 Service 成功")
	} else {
		log.WithField("serviceID", svc.ID).Debug("自注册：Service 已存在")
	}

	// 3. 准备 instance 信息
	env := base.Configures.Config.Env
	if env == "" {
		env = "dev"
	}

	host := base.Configures.Config.Host
	if host == "" {
		host = "127.0.0.1"
	}

	port := base.Configures.Config.Port
	if port == 0 {
		port = 9000 // fallback
	}

	// endpoint 规则：优先使用 domain，否则使用 host:port
	var endpoint string
	if base.Configures.Config.Domain != "" {
		endpoint = fmt.Sprintf("http://%s", base.Configures.Config.Domain)
	} else {
		endpoint = fmt.Sprintf("http://%s:%d", host, port)
	}

	// instanceKey：唯一标识，避免多实例冲突
	instanceKey := fmt.Sprintf("%s-%s-%s:%d", appName, env, host, port)

	// meta：包含 gRPC 地址、进程信息等
	meta := map[string]interface{}{
		"httpPort":  port,
		"pid":       os.Getpid(),
		"startedAt": time.Now().Format(time.RFC3339),
	}

	// 如果配置了 gRPC，将地址规范化并写入 meta
	if base.Configures.Config.GRPC.Address != "" {
		grpcAddr := base.Configures.Config.GRPC.Address
		// 如果是 :port 格式，补全为 host:port
		if strings.HasPrefix(grpcAddr, ":") {
			grpcAddr = fmt.Sprintf("%s%s", host, grpcAddr)
		}
		meta["grpcAddress"] = grpcAddr
	}

	ttlSeconds := int64(90) // 90 秒 TTL

	// 4. RegisterInstance
	instanceReq := &dto.RegisterInstanceReq{
		ServiceID:   svc.ID,
		InstanceKey: instanceKey,
		Env:         env,
		Host:        host,
		Endpoint:    endpoint,
		Meta:        meta,
		TTLSeconds:  ttlSeconds,
	}

	resp, err := m.Client.RegisterInstance(ctx, instanceReq)
	if err != nil {
		log.WithErr(err).Warn("自注册：RegisterInstance 失败")
		return
	}

	log.WithField("instanceKey", resp.InstanceKey).
		WithField("expiresAt", resp.ExpiresAt).
		Info("自注册：注册实例成功")

	// 5. 保存自注册信息，供心跳任务使用
	m.selfServiceID = svc.ID
	m.selfInstanceKey = resp.InstanceKey

	// 6. 注册心跳任务
	m.registerHeartbeatTask(ttlSeconds)
}

// registerHeartbeatTask 注册心跳续租任务
func (m *Module) registerHeartbeatTask(ttlSeconds int64) {
	log := base.Logger.WithEntryName("RegistrySelfRegister")

	if base.Scheduler == nil {
		log.Warn("自注册：Scheduler 未初始化，跳过心跳任务注册")
		return
	}

	// 心跳间隔：1/3 TTL（确保在过期前续租）
	heartbeatInterval := time.Duration(ttlSeconds/3) * time.Second
	if heartbeatInterval < 10*time.Second {
		heartbeatInterval = 10 * time.Second
	}

	heartbeatTask := scheduler.NewIntervalTask(
		"注册中心自注册心跳",
		time.Now().Add(heartbeatInterval), // 首次执行时间
		heartbeatInterval,
		scheduler.TaskExecuteModeLocal, // 本地任务
		5*time.Second,                  // 超时时间
		func(ctx context.Context) error {
			req := &dto.HeartbeatReq{
				ServiceID:   m.selfServiceID,
				InstanceKey: m.selfInstanceKey,
			}

			_, err := m.Client.HeartbeatInstance(ctx, req)
			if err != nil {
				log.WithErr(err).Warn("自注册心跳失败")
				return err
			}

			log.Debug("自注册心跳成功")
			return nil
		},
	)

	if err := base.Scheduler.AddTask(heartbeatTask); err != nil {
		log.WithErr(err).Warn("自注册：添加心跳任务失败")
		return
	}

	log.WithField("interval", heartbeatInterval).Info("自注册：心跳任务已注册")
}
