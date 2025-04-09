package replication

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"strconv"
	"sync"
)

// ServiceDiscoveryAdapter 服务发现适配器
type ServiceDiscoveryAdapter struct {
	discovery    discovery.DiscoveryService
	serviceName  string
	serviceID    string
	logger       *common.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	mutex        sync.RWMutex
	masterHost   string
	masterPort   int
	callback     func(string, int)
	oldCallback  func(ServiceInfo)
	watchStarted bool
	watcherID    string
}

// NewServiceDiscoveryAdapter 创建服务发现适配器
func NewServiceDiscoveryAdapter(discovery discovery.DiscoveryService, serviceName string) *ServiceDiscoveryAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceDiscoveryAdapter{
		discovery:   discovery,
		serviceName: serviceName,
		logger:      common.GetLogger(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// 通知主节点变更的辅助方法
func (sda *ServiceDiscoveryAdapter) notifyMasterChange(host string, port int) {
	sda.mutex.RLock()
	currentHost := sda.masterHost
	currentPort := sda.masterPort
	currentCallback := sda.callback
	oldCallback := sda.oldCallback
	sda.mutex.RUnlock()

	// 检查是否真的发生了变更，避免重复通知
	if currentHost == host && currentPort == port {
		sda.logger.Debugf("跳过相同主节点的重复通知: %s:%d", host, port)
		return
	}

	// 更新主节点信息
	sda.mutex.Lock()
	sda.masterHost = host
	sda.masterPort = port
	sda.mutex.Unlock()

	if currentCallback != nil {
		sda.logger.Infof("通知主节点变更: %s:%d", host, port)
		currentCallback(host, port)
	}

	if oldCallback != nil {
		oldCallback(ServiceInfo{
			ID:   "",
			Host: host,
			Port: port,
			Role: RoleMaster,
		})
	}
}

// DeregisterService 注销服务
func (sda *ServiceDiscoveryAdapter) DeregisterService(instanceID string) error {
	// 注销服务
	err := sda.discovery.Deregister(sda.ctx, instanceID)
	if err != nil {
		sda.logger.Errorf("注销服务失败: %v", err)
		return err
	}

	sda.logger.Infof("服务已注销: %s/%s", sda.serviceName, instanceID)
	return nil
}

// FindMasterNode 查找主节点
func (sda *ServiceDiscoveryAdapter) FindMasterNode() (string, int, error) {
	// 查询当前服务列表
	services, err := sda.discovery.Discover(sda.ctx, sda.serviceName)
	if err != nil {
		sda.logger.Errorf("获取服务列表失败: %v", err)
		return "", 0, err
	}

	// 查找主节点
	for _, service := range services {
		if service.Metadata != nil {
			if isMaster, ok := service.Metadata["isMaster"]; ok && isMaster == "true" {
				sda.mutex.Lock()
				sda.masterHost = service.Address
				sda.masterPort = service.Port
				sda.mutex.Unlock()

				return service.Address, service.Port, nil
			}
		}
	}

	return "", 0, fmt.Errorf("没有找到主节点")
}

// WatchMasterNodeChange 监控主节点变更
func (sda *ServiceDiscoveryAdapter) WatchMasterNodeChange(callback func(string, int)) {
	sda.mutex.Lock()
	sda.callback = callback

	// 避免重复监控
	if sda.watchStarted {
		sda.mutex.Unlock()
		return
	}

	sda.watchStarted = true
	sda.mutex.Unlock()

	// 创建事件处理函数
	handler := func(event discovery.DiscoveryEvent) {
		// 只处理注册和更新事件
		if event.Type == discovery.DiscoveryEventAdd ||
			event.Type == discovery.DiscoveryEventUpdate {
			// 检查是否是主节点
			if event.Service.Metadata != nil {
				if isMaster, ok := event.Service.Metadata["isMaster"]; ok && isMaster == "true" {
					// 使用notifyMasterChange方法检查并通知主节点变更
					// 该方法内部会检查是否真的发生了变更
					sda.notifyMasterChange(event.Service.Address, event.Service.Port)
				}
			}
		} else if event.Type == discovery.DiscoveryEventDelete {
			// 如果主节点被注销，可能需要进行特殊处理
			sda.mutex.RLock()
			currentHost := sda.masterHost
			currentPort := sda.masterPort
			sda.mutex.RUnlock()

			// 检查删除的是否为当前主节点
			if event.Service.Address == currentHost &&
				event.Service.Port == currentPort {
				sda.logger.Warnf("主节点已下线: %s:%d", currentHost, currentPort)
				// 这里可以添加主节点下线后的特殊处理逻辑
			}
		}
	}

	// 添加监听器
	watcherID, err := sda.discovery.AddWatcher(sda.ctx, sda.serviceName, handler)
	if err != nil {
		sda.logger.Errorf("监控服务变更失败: %v", err)
		return
	}

	sda.mutex.Lock()
	sda.watcherID = watcherID
	sda.mutex.Unlock()

	// 立即尝试查找主节点，初始化主节点信息
	go func() {
		host, port, err := sda.FindMasterNode()
		if err == nil {
			// 通知发现的主节点 - 使用notifyMasterChange以避免重复通知
			sda.notifyMasterChange(host, port)
		}
	}()
}

// Close 关闭资源
func (sda *ServiceDiscoveryAdapter) Close() error {
	// 移除监听器
	if sda.watchStarted && sda.watcherID != "" {
		err := sda.discovery.RemoveWatcher(sda.serviceName, sda.watcherID)
		if err != nil {
			sda.logger.Warnf("移除服务监听器失败: %v", err)
		}
	}

	// 取消上下文
	sda.cancel()
	return nil
}

// --------- 实现 ServiceDiscover 接口 ---------

// Register 实现 ServiceDiscover 接口的 Register 方法
func (sda *ServiceDiscoveryAdapter) Register(info ServiceInfo) error {
	// 检查是否是主节点
	isMaster := info.Role == RoleMaster

	// 将协议端口添加到元数据中
	metadata := map[string]string{
		"isMaster": strconv.FormatBool(isMaster),
		"nodeId":   info.NodeID,
	}

	if info.ProtocolPort > 0 {
		metadata["protocolPort"] = strconv.Itoa(info.ProtocolPort)
	}

	// 创建适配后的服务信息
	serviceInfo := discovery.ServiceInfo{
		ID:       info.ID,
		Name:     sda.serviceName,
		Address:  info.Host,
		Port:     info.Port,
		Metadata: metadata,
	}

	// 注册服务
	return sda.discovery.Register(sda.ctx, serviceInfo)
}

// Deregister 实现 ServiceDiscover 接口的 Deregister 方法
func (sda *ServiceDiscoveryAdapter) Deregister(serviceID string) error {
	return sda.DeregisterService(serviceID)
}

// FindMaster 实现 ServiceDiscover 接口的 FindMaster 方法
func (sda *ServiceDiscoveryAdapter) FindMaster() (ServiceInfo, error) {
	// 先查找主节点
	host, port, err := sda.FindMasterNode()
	if err != nil {
		return ServiceInfo{}, err
	}

	// 查询当前服务列表以获取详细信息
	services, err := sda.discovery.Discover(sda.ctx, sda.serviceName)
	if err != nil {
		return ServiceInfo{}, err
	}

	// 查找匹配的主节点，提取协议端口
	var protocolPort int
	for _, service := range services {
		if service.Address == host && service.Port == port {
			// 尝试从元数据中提取协议端口
			if service.Metadata != nil {
				if portStr, ok := service.Metadata["protocolPort"]; ok {
					if p, err := strconv.Atoi(portStr); err == nil {
						protocolPort = p
					}
				}
			}
			break
		}
	}

	// 返回主节点信息
	return ServiceInfo{
		ID:           fmt.Sprintf("%s:%d", host, port),
		Host:         host,
		Port:         port,
		ProtocolPort: protocolPort,
		Role:         RoleMaster,
	}, nil
}

// WatchMasterChange 实现 ServiceDiscover 接口的 WatchMasterChange 方法
func (sda *ServiceDiscoveryAdapter) WatchMasterChange(handler func(ServiceInfo)) {
	sda.mutex.Lock()
	sda.oldCallback = handler
	sda.mutex.Unlock()

	// 如果还没有监控，启动监控
	if !sda.watchStarted {
		sda.WatchMasterNodeChange(func(host string, port int) {
			// 新接口的回调已在 WatchMasterNodeChange 中处理
		})
	}
}
