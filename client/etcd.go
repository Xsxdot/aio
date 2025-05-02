package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/idgen"
	"github.com/xsxdot/aio/pkg/distributed/lock"
	"github.com/xsxdot/aio/pkg/distributed/manager"
	"github.com/xsxdot/aio/pkg/distributed/state"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

type EtcdService struct {
	client         *Client
	ConnectTimeout time.Duration
	log            *zap.Logger

	etcdClient         *etcd.EtcdClient
	distributedManager manager.DistributedManager
}

func NewEtcdService(client *Client) *EtcdService {
	timeout := 5 * time.Second
	if client.options.etcdOptions != nil && client.options.etcdOptions.ConnectTimeout > 0 {
		timeout = client.options.etcdOptions.ConnectTimeout
	}

	return &EtcdService{
		client:         client,
		ConnectTimeout: timeout,
		log:            common.GetLogger().GetZapLogger("aio-etcd-client"),
	}
}

func (e *EtcdService) Connect(ctx context.Context) error {
	// 从配置中心获取客户端配置
	cfg, err := e.GetClientConfigFromCenter(ctx)
	if err != nil {
		return fmt.Errorf("从配置中心获取ETCD客户端配置失败: %w", err)
	}

	// 创建 ETCD 客户端
	etcdClient, err := etcd.NewEtcdClient(cfg, e.log)
	if err != nil {
		return fmt.Errorf("创建 ETCD 客户端失败: %w", err)
	}

	e.etcdClient = etcdClient

	// 初始化分布式服务管理器
	rawClient := etcdClient
	if rawClient != nil {
		e.distributedManager = manager.NewManager(rawClient, manager.WithLogger(e.log))

		// 启动分布式服务管理器
		startCtx, cancel := context.WithTimeout(ctx, e.ConnectTimeout)
		defer cancel()

		if err := e.distributedManager.Start(startCtx); err != nil {
			e.log.Warn("启动分布式服务管理器失败", zap.Error(err))
			// 继续执行，不返回错误，因为基础的 ETCD 客户端已经创建成功
		} else {
			e.log.Info("分布式服务管理器启动成功")
		}
	}
	return nil
}

// GetClient 获取 ETCD 客户端实例
func (e *EtcdService) GetClient() *etcd.EtcdClient {
	return e.etcdClient
}

// GetRawClient 获取原始的 ETCD 客户端
func (e *EtcdService) GetRawClient() *clientv3.Client {
	if e.etcdClient == nil {
		return nil
	}
	return e.etcdClient.Client
}

// Close 关闭 ETCD 客户端
func (e *EtcdService) Close() {
	// 先停止分布式服务管理器
	if e.distributedManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.distributedManager.Stop(ctx); err != nil {
			e.log.Warn("停止分布式服务管理器失败", zap.Error(err))
		}
		e.distributedManager = nil
	}

	// 再关闭 ETCD 客户端
	if e.etcdClient != nil {
		e.etcdClient.Close()
		e.etcdClient = nil
	}
}

// GetClientConfigFromCenter 从配置中心获取ETCD客户端配置
func (e *EtcdService) GetClientConfigFromCenter(ctx context.Context) (*etcd.ClientConfig, error) {
	// 从配置中心获取客户端配置
	configKey := fmt.Sprintf("%s%s", consts.ClientConfigPrefix, consts.ComponentEtcd)

	// 使用 GetConfigWithStruct 方法直接获取并反序列化为结构体
	// 该方法会自动处理加密字段的解密
	var baseCfg config.ClientConfigFixedValue
	err := e.client.Config.GetConfigJSONParse(ctx, configKey, &baseCfg)
	if err != nil {
		return nil, fmt.Errorf("从配置中心获取ETCD客户端配置失败: %w", err)
	}

	// 获取 ETCD 服务节点信息
	services, err := e.client.Discovery.Discover(ctx, consts.ComponentEtcd)
	if err != nil {
		return nil, fmt.Errorf("发现 ETCD 服务失败: %w", err)
	}

	// 检查是否有可用的 ETCD 服务节点
	if len(services) == 0 {
		return nil, fmt.Errorf("没有可用的 ETCD 服务节点")
	}

	e.log.Info("发现 ETCD 服务节点", zap.Int("count", len(services)))

	// 构建端点列表
	endpoints := make([]string, 0, len(services))
	for _, service := range services {
		// 检查是否有指定的 ETCD 端口
		port := service.Port
		if etcdPort, ok := service.Metadata["etcd_port"]; ok {
			fmt.Sscanf(etcdPort, "%d", &port)
		}

		// 检查是否有 TLS 配置
		scheme := "http"
		if _, ok := service.Metadata["tls_enabled"]; ok {
			scheme = "https"
		}

		// 构建端点地址
		endpoint := fmt.Sprintf("%s://%s:%d", scheme, service.Address, port)
		endpoints = append(endpoints, endpoint)
		e.log.Info("添加 ETCD 端点", zap.String("endpoint", endpoint))
	}

	cfg := &etcd.ClientConfig{
		Endpoints:         endpoints,
		DialTimeout:       e.ConnectTimeout,
		Username:          baseCfg.Username,
		Password:          baseCfg.Password,
		AutoSyncEndpoints: true,
	}

	if baseCfg.EnableTls {
		tlsConfig := &config.TLSConfig{
			TLSEnabled: baseCfg.EnableTls,
			AutoTls:    false,
		}

		// 使用证书内容创建临时文件
		if baseCfg.CertContent != "" && baseCfg.KeyContent != "" && baseCfg.CATrustedContent != "" {
			// 创建临时文件夹
			tmpDir, err := os.MkdirTemp("", "etcd-certs-")
			if err != nil {
				e.log.Warn("创建临时证书目录失败", zap.Error(err))
			} else {
				// 创建证书文件
				certFile := filepath.Join(tmpDir, "cert.pem")
				keyFile := filepath.Join(tmpDir, "key.pem")
				caFile := filepath.Join(tmpDir, "ca.pem")

				// 写入证书内容
				if err := os.WriteFile(certFile, []byte(baseCfg.CertContent), 0600); err == nil {
					tlsConfig.Cert = certFile
				} else {
					e.log.Warn("写入证书文件失败", zap.Error(err))
				}

				// 写入密钥内容
				if err := os.WriteFile(keyFile, []byte(baseCfg.KeyContent), 0600); err == nil {
					tlsConfig.Key = keyFile
				} else {
					e.log.Warn("写入密钥文件失败", zap.Error(err))
				}

				// 写入CA证书内容
				if err := os.WriteFile(caFile, []byte(baseCfg.CATrustedContent), 0600); err == nil {
					tlsConfig.TrustedCA = caFile
				} else {
					e.log.Warn("写入CA证书文件失败", zap.Error(err))
				}

				// 设置清理函数，在程序退出时删除临时文件
				finalizer := func() {
					os.RemoveAll(tmpDir)
				}

				// 注册清理函数
				runtime.SetFinalizer(tlsConfig, func(_ *config.TLSConfig) {
					finalizer()
				})
			}
		} else if baseCfg.Cert != "" && baseCfg.Key != "" && baseCfg.TrustedCAFile != "" {
			// 如果没有证书内容但有证书路径，使用路径
			tlsConfig.Cert = baseCfg.Cert
			tlsConfig.Key = baseCfg.Key
			tlsConfig.TrustedCA = baseCfg.TrustedCAFile
		}

		cfg.TLS = tlsConfig
	}

	return cfg, nil
}

// GetElectionService 获取选举服务
//func (e *EtcdService) GetElectionService() (election.ElectionService, error) {
//	if e.distributedManager == nil {
//		return nil, fmt.Errorf("分布式服务管理器未初始化")
//	}
//	return e.distributedManager.Election(), nil
//}

// GetLockService 获取锁服务
func (e *EtcdService) GetLockService() (lock.LockService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.Lock(), nil
}

// GetIDGeneratorService 获取ID生成器服务
func (e *EtcdService) GetIDGeneratorService() (idgen.IDGeneratorService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.IDGenerator(), nil
}

// GetStateManagerService 获取状态管理服务
func (e *EtcdService) GetStateManagerService() (state.StateManagerService, error) {
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务管理器未初始化")
	}
	return e.distributedManager.StateManager(), nil
}

// CreateLock 创建锁实例，实现scheduler.LockProvider接口
func (e *EtcdService) CreateLock(ctx context.Context, key string, options ...lock.LockOption) (lock.Lock, error) {
	// 确保分布式服务已初始化
	if e.distributedManager == nil {
		return nil, fmt.Errorf("分布式服务未初始化")
	}

	// 获取锁服务
	lockService := e.distributedManager.Lock()
	if lockService == nil {
		return nil, fmt.Errorf("锁服务未初始化")
	}

	// 创建锁
	return lockService.Create(key, options...)
}

// CreateElection 创建一个命名的分布式选举
//func (e *EtcdService) CreateElection(ctx context.Context, name string, options ...election.ElectionOption) (election.Election, error) {
//	electionService, err := e.GetElectionService()
//	if err != nil {
//		return nil, err
//	}
//	return electionService.Create(name, options...)
//}

// CreateIDGenerator 创建一个命名的ID生成器
func (e *EtcdService) CreateIDGenerator(ctx context.Context, name string, options ...idgen.IDGenOption) (idgen.IDGenerator, error) {
	idGenService, err := e.GetIDGeneratorService()
	if err != nil {
		return nil, err
	}
	return idGenService.Create(name, options...)
}
