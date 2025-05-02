package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/internal/etcd"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/distributed/common"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// ServiceInfo 服务信息
type ServiceInfo struct {
	// ID 服务唯一标识
	ID string `json:"id"`
	// Name 服务名称
	Name string `json:"name"`
	// Address 服务地址
	Address string `json:"address"`
	// Port 服务端口
	Port int `json:"port"`
	// Metadata 元数据
	Metadata map[string]string `json:"metadata,omitempty"`
	// RegisterTime 注册时间
	RegisterTime time.Time `json:"registerTime,omitempty"`
}

type Config struct {
	ServiceRoot     string `json:"service_root" yaml:"service_root"`
	TTL             int    `json:"ttl" yaml:"ttl"`
	HeartbeatPeriod string `json:"heartbeat_period" yaml:"heartbeat_period"`
}

// DiscoveryEventType 服务发现事件类型
type DiscoveryEventType string

const (
	// DiscoveryEventAdd 服务添加事件
	DiscoveryEventAdd DiscoveryEventType = "add"
	// DiscoveryEventDelete 服务删除事件
	DiscoveryEventDelete DiscoveryEventType = "delete"
	// DiscoveryEventUpdate 服务更新事件
	DiscoveryEventUpdate DiscoveryEventType = "update"
)

// DiscoveryEvent 服务发现事件
type DiscoveryEvent struct {
	// Type 事件类型
	Type DiscoveryEventType `json:"type"`
	// Service 服务信息
	Service ServiceInfo `json:"service"`
	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`
}

// DiscoveryEventHandler 服务发现事件处理函数
type DiscoveryEventHandler func(event DiscoveryEvent)

// DiscoveryService 服务发现接口
type DiscoveryService interface {
	common.ServerComponent

	// Register 注册服务
	Register(ctx context.Context, service ServiceInfo) error
	// Deregister 注销服务
	Deregister(ctx context.Context, serviceID string) error
	// Discover 发现服务
	Discover(ctx context.Context, serviceName string) ([]ServiceInfo, error)
	// AddWatcher 添加服务变更监听器
	AddWatcher(ctx context.Context, serviceName string, handler DiscoveryEventHandler) (string, error)
	// RemoveWatcher 移除服务变更监听器
	RemoveWatcher(serviceName, watcherID string) error
	// GetAllServices 获取所有服务
	GetAllServices(ctx context.Context) (map[string][]ServiceInfo, error)
}

// 服务发现实现
type discoveryServiceImpl struct {
	etcdClient  *etcd.EtcdClient
	logger      *zap.Logger
	services    map[string]ServiceInfo
	watchers    map[string]map[string]context.CancelFunc
	handlers    map[string]map[string]DiscoveryEventHandler
	serviceRoot string
	mutex       sync.RWMutex
	isRunning   bool
	status      consts.ComponentStatus
}

func (d *discoveryServiceImpl) RegisterMetadata() (bool, int, map[string]string) {
	return false, 0, nil
}

func (d *discoveryServiceImpl) Name() string {
	return consts.ComponentDiscovery
}

func (d *discoveryServiceImpl) Status() consts.ComponentStatus {
	return d.status
}

// GetClientConfig 实现Component接口，返回客户端配置
func (d *discoveryServiceImpl) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}

// DefaultConfig 返回组件的默认配置
func (d *discoveryServiceImpl) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	return d.genConfig()
}

func (d *discoveryServiceImpl) genConfig() *Config {
	return &Config{
		ServiceRoot:     "/aio/services",
		TTL:             30,
		HeartbeatPeriod: "10s",
	}
}

func (d *discoveryServiceImpl) Init(config *config.BaseConfig, body []byte) error {
	cfg := d.genConfig()

	d.serviceRoot = cfg.ServiceRoot
	d.status = consts.StatusInitialized
	return nil
}

func (d *discoveryServiceImpl) Restart(ctx context.Context) error {
	if err := d.Stop(ctx); err != nil {
		return err
	}
	return d.Start(ctx)
}

// DiscoveryOption 服务发现选项函数类型
type DiscoveryOption func(*discoveryServiceImpl)

// WithServiceRoot 设置服务根路径
func WithServiceRoot(root string) DiscoveryOption {
	return func(d *discoveryServiceImpl) {
		d.serviceRoot = root
	}
}

// NewDiscoveryService 创建服务发现实例
func NewDiscoveryService(etcdClient *etcd.EtcdClient, logger *zap.Logger, options ...DiscoveryOption) (DiscoveryService, error) {
	discovery := &discoveryServiceImpl{
		etcdClient:  etcdClient,
		logger:      logger,
		services:    make(map[string]ServiceInfo),
		watchers:    make(map[string]map[string]context.CancelFunc),
		handlers:    make(map[string]map[string]DiscoveryEventHandler),
		serviceRoot: "/aio/services",
		isRunning:   false,
	}

	// 应用选项
	for _, option := range options {
		option(discovery)
	}

	return discovery, nil
}

// Start 启动服务发现
func (d *discoveryServiceImpl) Start(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.isRunning {
		return nil
	}

	d.logger.Info("Starting discovery service")

	d.isRunning = true
	d.status = consts.StatusRunning
	return nil
}

// Stop 停止服务发现
func (d *discoveryServiceImpl) Stop(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if !d.isRunning {
		return nil
	}

	d.logger.Info("Stopping discovery service")

	// 停止所有监听
	for serviceName, watchers := range d.watchers {
		for id, cancel := range watchers {
			d.logger.Debug("Stopping watcher",
				zap.String("serviceName", serviceName),
				zap.String("id", id))
			cancel()
		}
	}

	// 清理数据
	d.watchers = make(map[string]map[string]context.CancelFunc)
	d.handlers = make(map[string]map[string]DiscoveryEventHandler)

	d.isRunning = false
	d.status = consts.StatusStopped
	return nil
}

// Register 注册服务
func (d *discoveryServiceImpl) Register(ctx context.Context, service ServiceInfo) error {
	if service.ID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}

	if service.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if service.Address == "" {
		return fmt.Errorf("service address cannot be empty")
	}

	// 设置注册时间
	if service.RegisterTime.IsZero() {
		service.RegisterTime = time.Now()
	}

	// 处理IP地址
	// 如果地址是localhost或127.0.0.1，替换为内网IP
	if service.Address == "0.0.0.0" {
		//if service.Address == "localhost" || service.Address == "127.0.0.1" || service.Address == "0.0.0.0" {
		internalIP, err := getInternalIP()
		if err == nil && internalIP != "" {
			d.logger.Info("Replacing localhost with internal IP",
				zap.String("original", service.Address),
				zap.String("internal_ip", internalIP))
			service.Address = internalIP
		} else {
			d.logger.Warn("Failed to get internal IP for localhost replacement",
				zap.Error(err))
		}
	}

	// 获取公网IP并添加到元数据
	publicIP := getPublicIP()
	if publicIP != "" {
		if service.Metadata == nil {
			service.Metadata = make(map[string]string)
		}
		service.Metadata["public_ip"] = publicIP
		d.logger.Info("Added public IP to metadata",
			zap.String("public_ip", publicIP))
	}

	// 序列化服务信息
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service info: %w", err)
	}

	// 保存到etcd
	key := d.getServiceKey(service.Name, service.ID)
	err = d.etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 缓存到内存
	d.mutex.Lock()
	d.services[service.ID] = service
	d.mutex.Unlock()

	d.logger.Info("Registered service",
		zap.String("id", service.ID),
		zap.String("name", service.Name),
		zap.String("address", service.Address),
		zap.Int("port", service.Port))

	return nil
}

// Deregister 注销服务
func (d *discoveryServiceImpl) Deregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}

	// 从内存获取服务信息
	d.mutex.RLock()
	service, exists := d.services[serviceID]
	d.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("service not found: %s", serviceID)
	}

	// 从etcd删除服务
	key := d.getServiceKey(service.Name, serviceID)
	err := d.etcdClient.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	// 从内存删除
	d.mutex.Lock()
	delete(d.services, serviceID)
	d.mutex.Unlock()

	d.logger.Info("Deregistered service",
		zap.String("id", serviceID),
		zap.String("name", service.Name))

	return nil
}

// Discover 发现服务
func (d *discoveryServiceImpl) Discover(ctx context.Context, serviceName string) ([]ServiceInfo, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}

	// 从etcd获取服务列表
	prefix := d.getServicePrefix(serviceName)
	resp, err := d.etcdClient.Client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	services := make([]ServiceInfo, 0, len(resp.Kvs))

	for _, kv := range resp.Kvs {
		var service ServiceInfo
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			d.logger.Warn("Failed to unmarshal service info",
				zap.String("key", string(kv.Key)),
				zap.Error(err))
			continue
		}

		services = append(services, service)
	}

	return services, nil
}

// AddWatcher 添加服务变更监听器
func (d *discoveryServiceImpl) AddWatcher(ctx context.Context, serviceName string, handler DiscoveryEventHandler) (string, error) {
	if serviceName == "" {
		return "", fmt.Errorf("service name cannot be empty")
	}

	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}

	// 生成唯一ID
	watcherID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 服务前缀
	prefix := d.getServicePrefix(serviceName)

	// 创建可取消的上下文
	watchCtx, cancel := context.WithCancel(context.Background())

	// 保存取消函数
	d.mutex.Lock()
	if _, exists := d.watchers[serviceName]; !exists {
		d.watchers[serviceName] = make(map[string]context.CancelFunc)
	}
	d.watchers[serviceName][watcherID] = cancel

	// 保存处理函数
	if _, exists := d.handlers[serviceName]; !exists {
		d.handlers[serviceName] = make(map[string]DiscoveryEventHandler)
	}
	d.handlers[serviceName][watcherID] = handler
	d.mutex.Unlock()

	// 启动监听
	watchCh := d.etcdClient.Client.Watch(watchCtx, prefix, clientv3.WithPrefix())

	// 获取现有服务并触发初始事件
	go func() {
		services, err := d.Discover(ctx, serviceName)
		if err != nil {
			d.logger.Error("Failed to get initial services",
				zap.String("serviceName", serviceName),
				zap.Error(err))
		} else {
			// 对所有现有服务触发添加事件
			for _, svc := range services {
				event := DiscoveryEvent{
					Type:      DiscoveryEventAdd,
					Service:   svc,
					Timestamp: time.Now(),
				}

				// 调用事件处理函数
				d.mutex.RLock()
				handlersMap, exists := d.handlers[serviceName]
				d.mutex.RUnlock()

				if exists {
					for _, h := range handlersMap {
						h(event)
					}
				}
			}
		}
	}()

	// 启动事件处理
	go func() {
		for {
			select {
			case <-ctx.Done():
				d.RemoveWatcher(serviceName, watcherID)
				return

			case watchResp, ok := <-watchCh:
				if !ok {
					d.logger.Warn("Watch channel closed",
						zap.String("serviceName", serviceName))
					d.RemoveWatcher(serviceName, watcherID)
					return
				}

				if watchResp.Err() != nil {
					d.logger.Error("Watch error",
						zap.String("serviceName", serviceName),
						zap.Error(watchResp.Err()))
					continue
				}

				// 处理事件
				for _, ev := range watchResp.Events {
					var eventType DiscoveryEventType
					var serviceInfo ServiceInfo
					var serviceID string

					switch ev.Type {
					case clientv3.EventTypePut:
						if err := json.Unmarshal(ev.Kv.Value, &serviceInfo); err != nil {
							d.logger.Error("Failed to unmarshal service info",
								zap.String("key", string(ev.Kv.Key)),
								zap.Error(err))
							continue
						}

						if ev.IsCreate() {
							eventType = DiscoveryEventAdd
						} else {
							eventType = DiscoveryEventUpdate
						}

						// 更新内存缓存
						d.mutex.Lock()
						d.services[serviceInfo.ID] = serviceInfo
						d.mutex.Unlock()

					case clientv3.EventTypeDelete:
						// 解析服务ID
						key := string(ev.Kv.Key)
						parts := strings.Split(key, "/")
						serviceID = parts[len(parts)-1]

						// 尝试从缓存获取服务信息（在删除前）
						d.mutex.RLock()
						var exists bool
						serviceInfo, exists = d.services[serviceID]
						d.mutex.RUnlock()

						if !exists {
							// 如果缓存中不存在，创建最小信息
							serviceInfo = ServiceInfo{
								ID:   serviceID,
								Name: serviceName,
							}
						}

						eventType = DiscoveryEventDelete

						// 从内存缓存删除
						d.mutex.Lock()
						delete(d.services, serviceID)
						d.mutex.Unlock()
					}

					// 创建事件
					event := DiscoveryEvent{
						Type:      eventType,
						Service:   serviceInfo,
						Timestamp: time.Now(),
					}

					// 调用所有监听该服务的处理函数
					d.mutex.RLock()
					handlersMap, exists := d.handlers[serviceName]
					d.mutex.RUnlock()

					if exists {
						for _, h := range handlersMap {
							h(event)
						}
					}
				}
			}
		}
	}()

	return watcherID, nil
}

// RemoveWatcher 移除服务变更监听器
func (d *discoveryServiceImpl) RemoveWatcher(serviceName, watcherID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 调用取消函数
	if watchers, exists := d.watchers[serviceName]; exists {
		if cancel, found := watchers[watcherID]; found {
			cancel()
			delete(watchers, watcherID)
		}
	}

	// 移除处理函数
	if handlers, exists := d.handlers[serviceName]; exists {
		delete(handlers, watcherID)
	}

	// 清理空映射
	if len(d.watchers[serviceName]) == 0 {
		delete(d.watchers, serviceName)
	}
	if len(d.handlers[serviceName]) == 0 {
		delete(d.handlers, serviceName)
	}

	return nil
}

// GetAllServices 获取所有服务
func (d *discoveryServiceImpl) GetAllServices(ctx context.Context) (map[string][]ServiceInfo, error) {
	// 获取所有服务
	resp, err := d.etcdClient.Client.Get(ctx, d.serviceRoot, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	result := make(map[string][]ServiceInfo)

	for _, kv := range resp.Kvs {
		var service ServiceInfo
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			d.logger.Warn("Failed to unmarshal service info",
				zap.String("key", string(kv.Key)),
				zap.Error(err))
			continue
		}

		if _, exists := result[service.Name]; !exists {
			result[service.Name] = make([]ServiceInfo, 0)
		}

		result[service.Name] = append(result[service.Name], service)
	}

	return result, nil
}

// 获取服务键
func (d *discoveryServiceImpl) getServiceKey(serviceName, serviceID string) string {
	return path.Join(d.serviceRoot, serviceName, serviceID)
}

// 获取服务前缀
func (d *discoveryServiceImpl) getServicePrefix(serviceName string) string {
	return path.Join(d.serviceRoot, serviceName)
}

// 获取内网IP地址
func getInternalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no internal IP address found")
}

func getPublicIP() string {
	resp, err := http.Get("https://ifconfig.me/ip")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(ip))
}
