package protocol

import "github.com/xsxdot/aio/pkg/network"

// MessageHandler 消息处理函数
type MessageHandler func(connID string, msg *CustomMessage) (interface{}, error)

// ServiceHandler 服务处理器
type ServiceHandler struct {
	// 消息类型到处理函数的映射
	handlers map[MessageType]MessageHandler
}

// NewServiceHandler 创建服务处理器
func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{
		handlers: make(map[MessageType]MessageHandler),
	}
}

// RegisterHandler 注册消息处理函数
func (h *ServiceHandler) RegisterHandler(msgType MessageType, handler MessageHandler) {
	h.handlers[msgType] = handler
}

// GetHandler 获取消息处理函数
func (h *ServiceHandler) GetHandler(msgType MessageType) (MessageHandler, bool) {
	handler, ok := h.handlers[msgType]
	return handler, ok
}

type HandleManager struct {
	baseHandles []MessageHandler
	handles     map[ServiceType]*ServiceHandler
}

func NewHandleManager() *HandleManager {
	return &HandleManager{
		handles: make(map[ServiceType]*ServiceHandler),
	}
}

func (h *HandleManager) RegisterBaseHandle(handles ...MessageHandler) {
	h.baseHandles = append(h.baseHandles, handles...)
}

func (h *HandleManager) RegisterHandle(svcType ServiceType, msgType MessageType, handler MessageHandler) {
	if _, ok := h.handles[svcType]; !ok {
		h.handles[svcType] = NewServiceHandler()
	}
	h.handles[svcType].RegisterHandler(msgType, handler)
}

func (h *HandleManager) ProcessMsg(conn *network.Connection, msg *CustomMessage) (*CustomMessage, error) {
	var err error
	for _, handle := range h.baseHandles {
		_, err = handle(conn.ID(), msg)
		if err != nil {
			break
		}
	}

	if err != nil {
		return h.returnErr(conn, msg, err)
	}

	if svcHandler, ok := h.handles[msg.Header().ServiceType]; ok {
		if handler, ok := svcHandler.GetHandler(msg.Header().MessageType); ok {
			result, err := handler(conn.ID(), msg)
			if err != nil {
				return h.returnErr(conn, msg, err)
			}

			if result != nil {
				response := NewSuccessResponse(conn.ID(), msg.Header().MessageID, result)
				return response, nil
			}
		}
	}

	return nil, nil
}

func (h *HandleManager) returnErr(conn *network.Connection, msg *CustomMessage, err error) (*CustomMessage, error) {
	response := NewFailResponse(conn.ID(), msg.Header().MessageID, err)
	return response, nil
}
