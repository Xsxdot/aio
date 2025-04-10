package sdk

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/protocol"
)

// DiscoveryServiceOptions 服务发现选项
type DiscoveryServiceOptions struct {
	// 服务监听间隔
	ServiceWatchInterval time.Duration
}

// DiscoveryServiceEvent 服务发现事件
type DiscoveryServiceEvent struct {
	// 事件类型： created, updated, deleted
	Type string
	// 服务信息
	Service discovery.ServiceInfo
}

// DiscoveryService 服务发现服务
type DiscoveryService struct {
	serviceInfo *ServiceInfo
	// 客户端引用
	client *Client
	// 选项
	options *DiscoveryServiceOptions
	// TCP API客户端
	tcpClient *TCPAPIClient

	// 服务相关
	servicesLock sync.RWMutex
	// 改为双层map：服务名称 -> 服务ID -> 服务信息
	services map[string]map[string]*discovery.ServiceInfo

	// 监听相关
	watcherIDs   map[string]string // 服务名称 -> 监听器ID
	watcherLock  sync.RWMutex
	watchedNames map[string]bool // 正在监听的服务名称

	// 事件处理器
	eventHandlers []func(event *DiscoveryServiceEvent)
}

// NewDiscoveryService 创建服务发现服务
func NewDiscoveryService(client *Client, options *DiscoveryServiceOptions) *DiscoveryService {
	if options == nil {
		options = &DiscoveryServiceOptions{
			ServiceWatchInterval: 5 * time.Second,
		}
	}

	info := client.serviceInfo

	return &DiscoveryService{
		serviceInfo:  info,
		client:       client,
		options:      options,
		tcpClient:    client.GetTCPAPIClient(),
		services:     make(map[string]map[string]*discovery.ServiceInfo),
		watcherIDs:   make(map[string]string),
		watchedNames: make(map[string]bool),
	}
}

// RegisterHandler 注册服务发现事件处理器
func (d *DiscoveryService) RegisterServiceDiscoveryHandler() {
	handler := protocol.NewServiceHandler()

	// 注册服务事件处理函数
	handler.RegisterHandler(distributed.MsgTypeServiceEvent, func(connID string, msg *protocol.CustomMessage) error {
		// 解析服务事件
		var eventResp protocol.APIResponse
		if err := json.Unmarshal(msg.Payload(), &eventResp); err != nil {
			return fmt.Errorf("解析服务事件响应失败: %w", err)
		}

		// 检查响应类型
		if eventResp.Type != "event" {
			return fmt.Errorf("接收到非事件类型的服务事件: %s", eventResp.Type)
		}

		// 检查数据是否为空
		if eventResp.Data == "" {
			return fmt.Errorf("服务事件数据为空")
		}

		// 从Data字段解析具体事件内容（现在Data是字符串类型）
		var event discovery.DiscoveryEvent
		if err := json.Unmarshal([]byte(eventResp.Data), &event); err != nil {
			return fmt.Errorf("解析服务事件失败: %w", err)
		}

		fmt.Printf("收到服务事件: 类型=%s, 服务ID=%s\n", event.Type, event.Service.ID)

		// 处理服务事件
		d.handleServiceEvent(&event)
		return nil
	})

	// 注册服务响应处理函数
	handler.RegisterHandler(distributed.MsgTypeServiceResponse, func(connID string, msg *protocol.CustomMessage) error {
		// 解析为统一响应结构体
		var baseResp protocol.APIResponse
		if err := json.Unmarshal(msg.Payload(), &baseResp); err != nil {
			fmt.Printf("解析统一响应结构失败: %v\n", err)
			return fmt.Errorf("解析统一响应结构失败: %w", err)
		}

		// 检查操作是否成功
		if !baseResp.Success {
			fmt.Printf("服务操作失败: %s\n", baseResp.Message)
			return fmt.Errorf("服务操作失败: %s", baseResp.Message)
		}

		// 根据响应类型处理不同业务逻辑
		switch baseResp.Type {
		case "discover":
			// 处理服务发现响应
			if baseResp.Data == "" {
				fmt.Printf("服务发现响应数据为空\n")
				return nil
			}

			// 将data字段转换为[]discovery.ServiceInfo（现在data是字符串）
			var services []discovery.ServiceInfo
			if err := json.Unmarshal([]byte(baseResp.Data), &services); err != nil {
				return fmt.Errorf("解析服务列表失败: %w", err)
			}

			fmt.Printf("解析服务发现响应: 找到 %d 个服务节点\n", len(services))
			d.updateServiceList(services)

		case "watch":
			// 处理服务监听响应
			if baseResp.Data == "" {
				return nil
			}

			// 尝试获取watcherId
			var watchData struct {
				WatcherID   string `json:"watcherId"`
				ServiceName string `json:"serviceName"`
			}
			if err := json.Unmarshal([]byte(baseResp.Data), &watchData); err == nil && watchData.WatcherID != "" {
				d.watcherLock.Lock()
				// 保存对应服务的监听器ID
				if watchData.ServiceName != "" {
					d.watcherIDs[watchData.ServiceName] = watchData.WatcherID
				}
				d.watcherLock.Unlock()
				fmt.Printf("服务监听已启动，服务=%s，监听器ID: %s\n", watchData.ServiceName, watchData.WatcherID)
			}

		case "unwatch":
			// 处理取消监听响应
			fmt.Printf("服务监听已取消: %s\n", baseResp.Message)

		case "register":
			// 处理服务注册响应
			fmt.Printf("服务注册成功: %s\n", baseResp.Message)

		case "deregister":
			// 处理服务注销响应
			fmt.Printf("服务注销成功: %s\n", baseResp.Message)

		case "event":
			// 事件消息应该由MsgTypeServiceEvent处理，这里不应该出现
			fmt.Printf("收到意外的事件消息，忽略\n")

		default:
			fmt.Printf("收到未知类型的响应: %s\n", baseResp.Type)
		}

		return nil
	})

	// 注册到协议管理器
	d.client.RegisterServiceHandler(distributed.ServiceTypeDiscovery, "discovery-handler", handler)
}

// handleServiceEvent 处理服务事件
func (d *DiscoveryService) handleServiceEvent(event *discovery.DiscoveryEvent) {
	d.servicesLock.Lock()
	defer d.servicesLock.Unlock()

	serviceName := event.Service.Name
	serviceID := event.Service.ID

	switch event.Type {
	case "created", "updated":
		// 确保服务名称的map存在
		if _, ok := d.services[serviceName]; !ok {
			d.services[serviceName] = make(map[string]*discovery.ServiceInfo)
		}
		// 添加或更新服务
		d.services[serviceName][serviceID] = &event.Service
		fmt.Printf("服务节点添加/更新: ID=%s, 名称=%s, 地址=%s\n",
			serviceID, serviceName, event.Service.Address)

	case "deleted":
		// 删除服务
		if serviceMap, ok := d.services[serviceName]; ok {
			delete(serviceMap, serviceID)
			// 如果服务名称下没有更多的服务实例，删除整个服务名称条目
			if len(serviceMap) == 0 {
				delete(d.services, serviceName)
			}
			fmt.Printf("服务节点删除: ID=%s, 名称=%s\n", serviceID, serviceName)
		}
	}

	// 通知所有服务事件处理器
	d.notifyServiceEvent(&DiscoveryServiceEvent{
		Type:    string(event.Type),
		Service: event.Service,
	})
}

// updateServiceList 更新服务列表
func (d *DiscoveryService) updateServiceList(services []discovery.ServiceInfo) {
	d.servicesLock.Lock()
	defer d.servicesLock.Unlock()

	// 更新服务信息
	for i, service := range services {
		serviceName := service.Name
		serviceID := service.ID

		// 确保服务名称的map存在
		if _, ok := d.services[serviceName]; !ok {
			d.services[serviceName] = make(map[string]*discovery.ServiceInfo)
		}

		// 更新服务
		d.services[serviceName][serviceID] = &services[i]
		fmt.Printf("更新服务节点: ID=%s, 名称=%s, 地址=%s\n",
			serviceID, serviceName, service.Address)
	}
}

// OnServiceEvent 注册服务事件处理器
func (d *DiscoveryService) OnServiceEvent(handler func(event *DiscoveryServiceEvent)) {
	d.eventHandlers = append(d.eventHandlers, handler)
}

// notifyServiceEvent 通知服务事件
func (d *DiscoveryService) notifyServiceEvent(event *DiscoveryServiceEvent) {
	for _, handler := range d.eventHandlers {
		go handler(event)
	}
}

// GenerateStableServiceID 根据服务的唯一特征生成稳定的服务ID
func (d *DiscoveryService) GenerateStableServiceID(service discovery.ServiceInfo) string {
	// 获取主机名（如果可能）
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	// 组合唯一标识符
	uniqueInfo := struct {
		Hostname string            `json:"hostname"`
		Name     string            `json:"name"`
		Address  string            `json:"address"`
		Port     int               `json:"port"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}{
		Hostname: hostname,
		Name:     service.Name,
		Address:  service.Address,
		Port:     service.Port,
		Metadata: service.Metadata,
	}

	// 序列化并计算哈希
	data, err := json.Marshal(uniqueInfo)
	if err != nil {
		// 如果序列化失败，使用简单的字符串连接
		data = []byte(fmt.Sprintf("%s:%s:%s:%d", hostname, service.Name, service.Address, service.Port))
	}

	// 计算MD5哈希
	hash := md5.Sum(data)
	// 转换为十六进制字符串
	return hex.EncodeToString(hash[:])
}

// RegisterService 注册服务
func (d *DiscoveryService) RegisterService(ctx context.Context, service discovery.ServiceInfo) (string, error) {
	// 验证服务信息
	if service.Name == "" || service.Address == "" {
		return "", fmt.Errorf("服务信息不完整: 必须提供名称和地址")
	}

	// 如果服务ID为空，则生成一个稳定的ID
	if service.ID == "" {
		service.ID = d.GenerateStableServiceID(service)
		fmt.Printf("已生成稳定的服务ID: %s\n", service.ID)
	}

	// 设置注册时间（如果未设置）
	if service.RegisterTime.IsZero() {
		service.RegisterTime = time.Now()
	}

	// 构造注册请求
	req := distributed.RegisterServiceRequest{
		Service: service,
	}

	// 使用TCP API客户端发送请求
	_, err := d.tcpClient.Send(
		ctx,
		distributed.MsgTypeRegisterService,
		distributed.ServiceTypeDiscovery,
		distributed.MsgTypeServiceResponse,
		req,
	)
	if err != nil {
		return "", fmt.Errorf("发送注册请求失败: %w", err)
	}

	fmt.Printf("已发送服务注册请求: ID=%s, 名称=%s\n", service.ID, service.Name)
	return service.ID, nil
}

// DeregisterService 注销服务
func (d *DiscoveryService) DeregisterService(ctx context.Context, serviceID string) error {
	// 验证服务ID
	if serviceID == "" {
		return fmt.Errorf("服务ID不能为空")
	}

	// 构造注销请求
	req := distributed.DeregisterServiceRequest{
		ServiceID: serviceID,
	}

	// 使用TCP API客户端发送请求
	_, err := d.tcpClient.Send(
		ctx,
		distributed.MsgTypeDeregisterService,
		distributed.ServiceTypeDiscovery,
		distributed.MsgTypeServiceResponse,
		req,
	)
	if err != nil {
		return fmt.Errorf("发送注销请求失败: %w", err)
	}

	fmt.Printf("已发送服务注销请求: ID=%s\n", serviceID)
	return nil
}

// WatchService 监听服务变更
func (d *DiscoveryService) WatchService(ctx context.Context, serviceName string) error {
	// 构造监听请求
	req := distributed.WatchServiceRequest{
		ServiceName: serviceName,
	}

	// 使用TCP API客户端发送请求
	_, err := d.tcpClient.Send(
		ctx,
		distributed.MsgTypeWatchService,
		distributed.ServiceTypeDiscovery,
		distributed.MsgTypeServiceResponse,
		req,
	)
	if err != nil {
		return fmt.Errorf("发送监听请求失败: %w", err)
	}

	// 记录监听的服务名称
	d.watcherLock.Lock()
	d.watchedNames[serviceName] = true
	d.watcherLock.Unlock()

	fmt.Printf("已发送服务监听请求: 服务名称=%s\n", serviceName)
	return nil
}

// StopWatchService 停止监听特定服务
func (d *DiscoveryService) StopWatchService(ctx context.Context, serviceName string) error {
	d.watcherLock.Lock()
	defer d.watcherLock.Unlock()

	// 获取对应的监听器ID
	watcherID, ok := d.watcherIDs[serviceName]
	if !ok || watcherID == "" {
		// 如果找不到监听器ID，直接从监听列表移除
		delete(d.watchedNames, serviceName)
		return nil
	}

	// 从监听列表中移除
	delete(d.watchedNames, serviceName)
	delete(d.watcherIDs, serviceName)

	// 如果还有其他监听服务，不发送停止请求
	if len(d.watchedNames) > 0 {
		fmt.Printf("仍在监听其他服务，不发送停止特定服务 %s 的请求\n", serviceName)
		return nil
	}

	return d.stopWatchAll(ctx)
}

// StopWatchAll 停止所有服务监听
func (d *DiscoveryService) StopWatchAll(ctx context.Context) error {
	d.watcherLock.Lock()
	defer d.watcherLock.Unlock()

	// 清空监听列表和监听器ID
	d.watchedNames = make(map[string]bool)
	watcherIDs := d.watcherIDs
	d.watcherIDs = make(map[string]string)

	// 逐个停止监听
	var lastErr error
	for serviceName, watcherID := range watcherIDs {
		if err := d.stopWatchWithID(ctx, watcherID); err != nil {
			lastErr = err
			fmt.Printf("停止服务 %s 的监听失败: %v\n", serviceName, err)
		}
	}

	return lastErr
}

// stopWatchWithID 使用特定ID停止监听
func (d *DiscoveryService) stopWatchWithID(ctx context.Context, watcherID string) error {
	if watcherID == "" {
		return nil
	}

	// 构造停止监听请求
	req := struct {
		WatcherID string `json:"watcherId"`
	}{
		WatcherID: watcherID,
	}

	// 使用TCP API客户端发送请求
	_, err := d.tcpClient.Send(
		ctx,
		distributed.MsgTypeUnwatchService,
		distributed.ServiceTypeDiscovery,
		distributed.MsgTypeServiceResponse,
		req,
	)
	if err != nil {
		return fmt.Errorf("发送停止监听请求失败: %w", err)
	}

	fmt.Printf("已发送停止服务监听请求: 监听器ID=%s\n", watcherID)
	return nil
}

// stopWatchAll 实际停止所有监听的内部方法
func (d *DiscoveryService) stopWatchAll(ctx context.Context) error {
	// 获取所有监听器ID进行批量停止
	watcherIDs := make([]string, 0, len(d.watcherIDs))
	for _, id := range d.watcherIDs {
		if id != "" {
			watcherIDs = append(watcherIDs, id)
		}
	}

	if len(watcherIDs) == 0 {
		return nil
	}

	var lastErr error
	for _, id := range watcherIDs {
		if err := d.stopWatchWithID(ctx, id); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// DiscoverServices 发现服务
func (d *DiscoveryService) DiscoverServices(ctx context.Context, serviceName string) ([]discovery.ServiceInfo, error) {
	// 构造发现请求
	req := distributed.DiscoverServiceRequest{
		ServiceName: serviceName,
	}

	// 使用TCP API客户端发送请求
	resp, err := d.tcpClient.Send(
		ctx,
		distributed.MsgTypeDiscoverService,
		distributed.ServiceTypeDiscovery,
		distributed.MsgTypeServiceResponse,
		req,
	)
	if err != nil {
		return nil, fmt.Errorf("发送发现请求失败: %w", err)
	}

	// 检查响应数据
	if resp.Data == "" {
		return nil, fmt.Errorf("服务发现响应数据为空")
	}

	// 解析服务列表
	var services []discovery.ServiceInfo
	if err := json.Unmarshal([]byte(resp.Data), &services); err != nil {
		return nil, fmt.Errorf("解析服务列表失败: %w", err)
	}

	// 更新本地服务缓存
	d.updateServiceList(services)

	return services, nil
}

// GetServices 获取当前已知的所有服务
func (d *DiscoveryService) GetServices() []discovery.ServiceInfo {
	d.servicesLock.RLock()
	defer d.servicesLock.RUnlock()

	var result []discovery.ServiceInfo

	for _, serviceMap := range d.services {
		for _, service := range serviceMap {
			result = append(result, *service)
		}
	}

	return result
}

// GetServiceByID 根据ID获取服务
func (d *DiscoveryService) GetServiceByID(serviceID string) (discovery.ServiceInfo, bool) {
	d.servicesLock.RLock()
	defer d.servicesLock.RUnlock()

	// 需要遍历所有服务名称下的服务
	for _, serviceMap := range d.services {
		if service, ok := serviceMap[serviceID]; ok {
			return *service, true
		}
	}
	return discovery.ServiceInfo{}, false
}

// GetServicesByName 根据名称获取服务
func (d *DiscoveryService) GetServicesByName(serviceName string) []discovery.ServiceInfo {
	d.servicesLock.RLock()
	defer d.servicesLock.RUnlock()

	var result []discovery.ServiceInfo

	if serviceMap, ok := d.services[serviceName]; ok {
		for _, service := range serviceMap {
			result = append(result, *service)
		}
	}

	return result
}

// GetServiceNodes 获取服务的所有节点
func (d *DiscoveryService) GetServiceNodes(serviceName string) ([]discovery.ServiceInfo, error) {
	// 先尝试从本地缓存获取
	d.servicesLock.RLock()
	serviceMap, ok := d.services[serviceName]
	d.servicesLock.RUnlock()

	if ok && len(serviceMap) > 0 {
		services := make([]discovery.ServiceInfo, 0, len(serviceMap))
		for _, svc := range serviceMap {
			services = append(services, *svc)
		}
		return services, nil
	}

	// 如果本地缓存没有找到，尝试主动发现服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Printf("本地缓存未找到服务 %s，尝试发现服务\n", serviceName)
	discoveredServices, err := d.DiscoverServices(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("未找到服务 %s 的节点: %w", serviceName, err)
	}

	if len(discoveredServices) == 0 {
		return nil, fmt.Errorf("服务 %s 没有可用节点", serviceName)
	}

	return discoveredServices, nil
}

// OnServiceNodesChange 监听服务节点变更
func (d *DiscoveryService) OnServiceNodesChange(handler func(serviceName string, added, removed []discovery.ServiceInfo)) {
	// 基于服务事件处理器实现节点变更通知
	d.OnServiceEvent(func(event *DiscoveryServiceEvent) {
		// 只处理已注册的服务变更
		serviceName := event.Service.Name

		// 根据事件类型处理
		switch event.Type {
		case "created", "updated":
			// 直接使用服务信息通知添加节点
			handler(serviceName, []discovery.ServiceInfo{event.Service}, nil)

		case "deleted":
			// 直接使用服务信息通知删除节点
			handler(serviceName, nil, []discovery.ServiceInfo{event.Service})
		}
	})
}
