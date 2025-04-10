package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/protocol"
	"go.uber.org/zap"
)

// TCPAPIServiceHandler 是TCP API服务处理器的函数类型
type TCPAPIServiceHandler func(connID string, msg *protocol.CustomMessage) error

// TCPAPIHandlerFunc 是处理具体API请求的处理函数
type TCPAPIHandlerFunc func(request interface{}) (interface{}, error)

// ServiceHandlerRegistration 服务处理器注册信息
type ServiceHandlerRegistration struct {
	ServiceType protocol.ServiceType
	ServiceName string
	Handler     *protocol.ServiceHandler
}

// TCPAPIClient 统一的TCP API客户端
type TCPAPIClient struct {
	client         *Client                                  // SDK客户端
	logger         *zap.Logger                              // 日志记录器
	requestTimeout time.Duration                            // 请求超时时间
	mu             sync.RWMutex                             // 互斥锁
	handlers       map[protocol.ServiceType]map[string]bool // 已注册的处理器
	responseChans  map[string]chan *protocol.CustomMessage  // 响应通道映射
	idCounter      int64                                    // ID计数器
}

// NewTCPAPIClient 创建新的TCP API客户端
func NewTCPAPIClient(client *Client) *TCPAPIClient {
	var logger *zap.Logger
	if client != nil && client.Scheduler != nil {
		// 使用zap.NewProduction创建生产级别的logger
		l, _ := zap.NewProduction()
		logger = l
	} else {
		l, _ := zap.NewProduction()
		logger = l
	}

	return &TCPAPIClient{
		client:         client,
		logger:         logger.Named("tcp-api-client"),
		requestTimeout: 10 * time.Second,
		handlers:       make(map[protocol.ServiceType]map[string]bool),
		responseChans:  make(map[string]chan *protocol.CustomMessage),
		idCounter:      0,
	}
}

// SetRequestTimeout 设置请求超时时间
func (c *TCPAPIClient) SetRequestTimeout(timeout time.Duration) {
	c.requestTimeout = timeout
}

// RegisterServiceHandler 注册服务处理器
func (c *TCPAPIClient) RegisterServiceHandler(serviceType protocol.ServiceType, serviceName string, handler *protocol.ServiceHandler) error {
	if handler == nil {
		return fmt.Errorf("处理器不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查服务类型是否已初始化
	if _, ok := c.handlers[serviceType]; !ok {
		c.handlers[serviceType] = make(map[string]bool)
	}

	// 检查服务名是否已注册
	if _, ok := c.handlers[serviceType][serviceName]; ok {
		return fmt.Errorf("服务 %s (类型: %d) 已注册", serviceName, serviceType)
	}

	// 注册到客户端
	c.client.RegisterServiceHandler(serviceType, serviceName, handler)
	c.handlers[serviceType][serviceName] = true

	c.logger.Info("已注册服务处理器",
		zap.Uint8("serviceType", uint8(serviceType)),
		zap.String("serviceName", serviceName))

	return nil
}

// NewServiceHandler 创建新的服务处理器
func (c *TCPAPIClient) NewServiceHandler() *protocol.ServiceHandler {
	return protocol.NewServiceHandler()
}

// RegisterHandler 注册消息处理函数
func (c *TCPAPIClient) RegisterHandler(serviceHandler *protocol.ServiceHandler, msgType protocol.MessageType, handler TCPAPIServiceHandler) {
	// 将TCPAPIServiceHandler转换为protocol.MessageHandler
	messageHandler := func(connID string, msg *protocol.CustomMessage) error {
		return handler(connID, msg)
	}
	serviceHandler.RegisterHandler(msgType, messageHandler)
}

// Send 发送请求并等待响应
func (c *TCPAPIClient) Send(
	ctx context.Context,
	msgType protocol.MessageType,
	svcType protocol.ServiceType,
	responseType protocol.MessageType,
	payload interface{}) (*protocol.APIResponse, error) {

	// 使用上下文或默认超时
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), c.requestTimeout)
		defer cancel()
	}

	// 序列化请求数据
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 检查连接并尝试发送请求
	if err := c.client.Connect(); err != nil {
		return nil, fmt.Errorf("连接服务失败: %w", err)
	}

	// 创建唯一的消息ID
	messageID := c.generateMessageID()

	// 创建一个用于接收响应的通道
	respChan := make(chan *protocol.CustomMessage, 1)

	// 注册响应通道
	c.mu.Lock()
	c.responseChans[messageID] = respChan
	c.mu.Unlock()

	// 在函数返回时清理
	defer func() {
		c.mu.Lock()
		delete(c.responseChans, messageID)
		c.mu.Unlock()
	}()

	// 注册通用响应处理器（如果尚未注册）
	c.ensureResponseHandlerRegistered(svcType, responseType)

	// 发送请求 - 注意：生成message但实际使用SDK提供的SendMessage方法发送
	// 这里省略使用自定义messageID的方式，依赖SDK内部生成ID
	if err := c.client.SendMessage(msgType, svcType, data); err != nil {
		return nil, fmt.Errorf("发送消息失败: %w", err)
	}

	// 等待响应或超时
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-respChan:
		var apiResp protocol.APIResponse
		if err := json.Unmarshal(msg.Payload(), &apiResp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		if !apiResp.Success {
			return &apiResp, fmt.Errorf("操作失败: %s", apiResp.Error)
		}

		return &apiResp, nil
	}
}

// SendRawRequest 发送原始请求并接收原始响应
func (c *TCPAPIClient) SendRawRequest(
	ctx context.Context,
	msgType protocol.MessageType,
	svcType protocol.ServiceType,
	responseType protocol.MessageType,
	payload []byte) ([]byte, error) {

	// 使用上下文或默认超时
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), c.requestTimeout)
		defer cancel()
	}

	// 检查连接并尝试发送请求
	if err := c.client.Connect(); err != nil {
		return nil, fmt.Errorf("连接服务失败: %w", err)
	}

	// 创建唯一的消息ID
	messageID := c.generateMessageID()

	// 创建一个用于接收响应的通道
	respChan := make(chan *protocol.CustomMessage, 1)

	// 注册响应通道
	c.mu.Lock()
	c.responseChans[messageID] = respChan
	c.mu.Unlock()

	// 在函数返回时清理
	defer func() {
		c.mu.Lock()
		delete(c.responseChans, messageID)
		c.mu.Unlock()
	}()

	// 注册通用响应处理器（如果尚未注册）
	c.ensureResponseHandlerRegistered(svcType, responseType)

	// 发送请求
	if err := c.client.SendMessage(msgType, svcType, payload); err != nil {
		return nil, fmt.Errorf("发送消息失败: %w", err)
	}

	// 等待响应或超时
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-respChan:
		return msg.Payload(), nil
	}
}

// HandleAPIRequest 处理API请求并发送统一格式的响应
func (c *TCPAPIClient) HandleAPIRequest(
	connID string,
	msg *protocol.CustomMessage,
	request interface{},
	handlerFunc TCPAPIHandlerFunc) error {

	// 解析请求
	if err := json.Unmarshal(msg.Payload(), request); err != nil {
		return c.SendErrorResponse(connID, msg.Header().MessageID, msg.Header().ServiceType, msg.Header().MessageType, "请求解析失败", err)
	}

	// 调用处理函数
	result, err := handlerFunc(request)
	if err != nil {
		return c.SendErrorResponse(connID, msg.Header().MessageID, msg.Header().ServiceType, msg.Header().MessageType, "请求处理失败", err)
	}

	// 发送成功响应
	return c.SendSuccessResponse(connID, msg.Header().MessageID, msg.Header().ServiceType, msg.Header().MessageType, "操作成功", result)
}

// SendSuccessResponse 发送成功响应
func (c *TCPAPIClient) SendSuccessResponse(
	connID string,
	msgID string,
	svcType protocol.ServiceType,
	respType protocol.MessageType,
	message string,
	data interface{}) error {

	return protocol.SendServiceResponse(
		c.client.protocolMgr,
		connID,
		msgID,
		respType,
		svcType,
		true,
		"success",
		message,
		data,
		"",
	)
}

// SendErrorResponse 发送错误响应
func (c *TCPAPIClient) SendErrorResponse(
	connID string,
	msgID string,
	svcType protocol.ServiceType,
	respType protocol.MessageType,
	message string,
	err error) error {

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	return protocol.SendServiceResponse(
		c.client.protocolMgr,
		connID,
		msgID,
		respType,
		svcType,
		false,
		"error",
		message,
		nil,
		errMsg,
	)
}

// generateMessageID 生成唯一的消息ID
func (c *TCPAPIClient) generateMessageID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.idCounter++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), c.idCounter)
}

// ensureResponseHandlerRegistered 确保响应处理器已注册
func (c *TCPAPIClient) ensureResponseHandlerRegistered(svcType protocol.ServiceType, respType protocol.MessageType) {
	c.mu.RLock()
	serviceHandlers, ok := c.handlers[svcType]
	c.mu.RUnlock()

	if !ok || len(serviceHandlers) == 0 {
		// 注册通用响应处理器
		handler := protocol.NewServiceHandler()
		handler.RegisterHandler(respType, c.genericResponseHandler)

		// 注册到客户端
		serviceName := fmt.Sprintf("response-handler-%d-%d", svcType, respType)
		c.RegisterServiceHandler(svcType, serviceName, handler)
	}
}

// genericResponseHandler 通用响应处理器
func (c *TCPAPIClient) genericResponseHandler(connID string, msg *protocol.CustomMessage) error {
	messageID := msg.Header().MessageID

	c.mu.RLock()
	ch, exists := c.responseChans[messageID]
	c.mu.RUnlock()

	if exists {
		select {
		case ch <- msg:
			// 成功发送到通道
		default:
			// 通道已满或已关闭
			c.logger.Warn("无法发送响应到通道，可能已满或已关闭",
				zap.String("messageID", messageID),
				zap.String("connID", connID))
		}
	} else {
		c.logger.Debug("收到无对应请求的响应",
			zap.String("messageID", messageID),
			zap.String("connID", connID))
	}
	return nil
}
