package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"github.com/xsxdot/aio/pkg/protocol"
)

// ServiceWatcher 服务监听信息
type ServiceWatcher struct {
	// 服务端监听ID
	ServerWatchID string
	// 处理器映射，key是handlerID
	Handlers map[string]func(event discovery.DiscoveryEvent)
}

// DiscoveryService 是服务发现的客户端API封装
type DiscoveryService struct {
	client         *Client
	requestService *RequestService
	mutex          sync.RWMutex

	// 服务名称 -> 服务监听信息
	serviceWatchers map[string]*ServiceWatcher
	// handlerID -> {serviceName, handler} 反向映射，用于快速查找
	handlerIndex map[string]struct {
		ServiceName string
		Handler     func(event discovery.DiscoveryEvent)
	}
}

// NewDiscoveryService 创建新的服务发现客户端
func NewDiscoveryService(client *Client) *DiscoveryService {
	d := &DiscoveryService{
		client:          client,
		requestService:  NewRequestService(client, client.protocolService),
		serviceWatchers: make(map[string]*ServiceWatcher),
		handlerIndex: make(map[string]struct {
			ServiceName string
			Handler     func(event discovery.DiscoveryEvent)
		}),
	}

	client.protocolService.manager.RegisterHandle(distributed.ServiceTypeDiscovery, distributed.MsgTypeServiceEvent, d.handleEvent)

	return d
}

// DiscoverServiceRequest 发现服务请求
type DiscoverServiceRequest struct {
	ServiceName string `json:"serviceName"`
}

// WatchServiceRequest 监听服务请求
type WatchServiceRequest struct {
	ServiceName string `json:"serviceName"`
}

// UnwatchServiceRequest 取消监听服务请求
type UnwatchServiceRequest struct {
	ServiceName string `json:"serviceName"`
	WatcherID   string `json:"watcherId"`
}

// RegisterServiceRequest 注册服务请求
type RegisterServiceRequest struct {
	Service discovery.ServiceInfo `json:"service"`
}

// DeregisterServiceRequest 注销服务请求
type DeregisterServiceRequest struct {
	ServiceID string `json:"serviceId"`
}

// 处理服务发现事件
func (d *DiscoveryService) handleEvent(connID string, msg *protocol.CustomMessage) (interface{}, error) {
	if msg.Payload() == nil || len(msg.Payload()) == 0 {
		return nil, nil
	}

	var event discovery.DiscoveryEvent
	if err := json.Unmarshal(msg.Payload(), &event); err != nil {
		return nil, err
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// 查找服务相关的所有处理器并调用
	if watcher, ok := d.serviceWatchers[event.Service.Name]; ok {
		for _, handler := range watcher.Handlers {
			go handler(event) // 异步调用处理器，避免阻塞
		}
	}

	return nil, nil
}

// Discover 发现服务实例
func (d *DiscoveryService) Discover(ctx context.Context, serviceName string) ([]discovery.ServiceInfo, error) {
	request := DiscoverServiceRequest{
		ServiceName: serviceName,
	}

	msg := protocol.NewMessage(
		distributed.MsgTypeDiscoverService,
		distributed.ServiceTypeDiscovery,
		"",
		request,
	)

	var services []discovery.ServiceInfo
	err := d.requestService.Request(msg, &services)
	if err != nil {
		return nil, fmt.Errorf("发现服务失败: %v", err)
	}

	return services, nil
}

// Watch 监听服务变化
// 返回处理器ID，用于后续取消监听
func (d *DiscoveryService) Watch(ctx context.Context, serviceName string, handler func(event discovery.DiscoveryEvent)) (string, error) {
	if serviceName == "" || handler == nil {
		return "", fmt.Errorf("服务名称和处理器不能为空")
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 生成唯一的处理器ID
	handlerID := uuid.New().String()

	// 检查是否已经在监听该服务
	watcher, exists := d.serviceWatchers[serviceName]

	// 如果没有现有的监听，则向服务端发起请求
	if !exists {
		request := WatchServiceRequest{
			ServiceName: serviceName,
		}

		msg := protocol.NewMessage(
			distributed.MsgTypeWatchService,
			distributed.ServiceTypeDiscovery,
			"",
			request,
		)

		var serverWatchID string
		err := d.requestService.Request(msg, &serverWatchID)
		if err != nil {
			return "", fmt.Errorf("监听服务失败: %v", err)
		}

		// 创建新的服务监听信息
		watcher = &ServiceWatcher{
			ServerWatchID: serverWatchID,
			Handlers:      make(map[string]func(event discovery.DiscoveryEvent)),
		}
		d.serviceWatchers[serviceName] = watcher
	}

	// 保存处理器
	watcher.Handlers[handlerID] = handler

	// 保存反向索引
	d.handlerIndex[handlerID] = struct {
		ServiceName string
		Handler     func(event discovery.DiscoveryEvent)
	}{
		ServiceName: serviceName,
		Handler:     handler,
	}

	return handlerID, nil
}

// Unwatch 取消服务监听
// handlerID是Watch返回的处理器ID
func (d *DiscoveryService) Unwatch(ctx context.Context, handlerID string) error {
	if handlerID == "" {
		return nil
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 查找处理器信息
	handlerInfo, exists := d.handlerIndex[handlerID]
	if !exists {
		return nil // 处理器不存在，可能已经被移除
	}

	serviceName := handlerInfo.ServiceName

	// 从反向索引中移除
	delete(d.handlerIndex, handlerID)

	// 从服务监听信息中移除处理器
	if watcher, ok := d.serviceWatchers[serviceName]; ok {
		delete(watcher.Handlers, handlerID)

		// 如果该服务没有处理器了，则向服务端发送取消监听请求并清理资源
		if len(watcher.Handlers) == 0 {
			request := UnwatchServiceRequest{
				ServiceName: serviceName,
				WatcherID:   watcher.ServerWatchID,
			}

			msg := protocol.NewMessage(
				distributed.MsgTypeUnwatchService,
				distributed.ServiceTypeDiscovery,
				"",
				request,
			)

			var result protocol.Response
			err := d.requestService.Request(msg, &result)
			if err != nil {
				return fmt.Errorf("取消监听服务失败: %v", err)
			}

			// 清理资源
			delete(d.serviceWatchers, serviceName)
		}
	}

	return nil
}

// Register 注册服务
func (d *DiscoveryService) Register(ctx context.Context, service discovery.ServiceInfo) (string, error) {
	// 如果服务ID为空，生成一个ID
	if service.ID == "" {
		service.ID = fmt.Sprintf("%s-%d", service.Name, time.Now().UnixNano())
	}

	request := RegisterServiceRequest{
		Service: service,
	}

	msg := protocol.NewMessage(
		distributed.MsgTypeRegisterService,
		distributed.ServiceTypeDiscovery,
		"",
		request,
	)

	var serviceID string
	err := d.requestService.Request(msg, &serviceID)
	if err != nil {
		return "", fmt.Errorf("注册服务失败: %v", err)
	}

	return serviceID, nil
}

// Deregister 注销服务
func (d *DiscoveryService) Deregister(ctx context.Context, serviceID string) error {
	request := DeregisterServiceRequest{
		ServiceID: serviceID,
	}

	msg := protocol.NewMessage(
		distributed.MsgTypeDeregisterService,
		distributed.ServiceTypeDiscovery,
		"",
		request,
	)

	var result protocol.Response
	err := d.requestService.Request(msg, &result)
	if err != nil {
		return fmt.Errorf("注销服务失败: %v", err)
	}

	return nil
}
