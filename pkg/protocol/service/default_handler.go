package service

import (
	"context"
	"github.com/xsxdot/aio/pkg/protocol"
	"sync"
)

// DefaultServiceHandler 默认服务处理器实现
type DefaultServiceHandler struct {
	// 已注册的服务类型
	serviceTypes []protocol.ServiceType
	// 同步锁，保护服务类型列表
	mu sync.RWMutex
	// 处理请求的函数
	handlerFunc func(ctx context.Context, req *protocol.Request) (*protocol.Response, error)
}

// NewDefaultServiceHandler 创建默认服务处理器
func NewDefaultServiceHandler(handlerFunc func(ctx context.Context, req *protocol.Request) (*protocol.Response, error)) *DefaultServiceHandler {
	return &DefaultServiceHandler{
		serviceTypes: make([]protocol.ServiceType, 0),
		handlerFunc:  handlerFunc,
	}
}

// HandleRequest 处理请求
func (h *DefaultServiceHandler) HandleRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	if h.handlerFunc != nil {
		return h.handlerFunc(ctx, req)
	}
	return protocol.NewErrorResponse(500, "no handler function registered"), nil
}

// Register 注册服务
func (h *DefaultServiceHandler) Register(serviceType protocol.ServiceType) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查是否已注册
	for _, st := range h.serviceTypes {
		if st == serviceType {
			return
		}
	}

	h.serviceTypes = append(h.serviceTypes, serviceType)
}

// GetServiceTypes 获取服务类型列表
func (h *DefaultServiceHandler) GetServiceTypes() []protocol.ServiceType {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 复制服务类型列表，避免外部修改
	result := make([]protocol.ServiceType, len(h.serviceTypes))
	copy(result, h.serviceTypes)

	return result
}
