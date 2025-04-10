package protocol

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xsxdot/aio/pkg/network"
)

// 错误定义
var (
	ErrServiceNotFound    = errors.New("service not found")
	ErrHandlerNotFound    = errors.New("handler not found")
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrInvalidServiceType = errors.New("invalid service type")
)

// MessageType 消息类型
type MessageType uint8

// 定义系统消息类型
const (
	// MsgTypeHeartbeat 心跳消息
	MsgTypeHeartbeat MessageType = 1
	// MsgTypeHeartbeatAck 心跳响应消息
	MsgTypeHeartbeatAck MessageType = 2
	// MsgTypeAuth 认证消息
	MsgTypeAuth MessageType = 3
	// MsgTypeAuthResponse 认证响应消息
	MsgTypeAuthResponse MessageType = 4
	// MsgTypeRefreshToken 刷新令牌请求消息
	MsgTypeRefreshToken MessageType = 5
	// MsgTypeRefreshTokenResponse 刷新令牌响应消息
	MsgTypeRefreshTokenResponse MessageType = 6
)

// ServiceType 服务类型
type ServiceType uint8

// 定义系统服务类型
const (
	// ServiceTypeSystem 系统服务（用于心跳等底层协议消息）
	ServiceTypeSystem      ServiceType = 1
	ServiceTypeConfig      ServiceType = 2
	ServiceTypeReplication ServiceType = 10 // 复制服务
	ServiceTypeElection    ServiceType = 20
	ServiceTypeDiscovery   ServiceType = 21
)

// MessageHeader 消息头
type MessageHeader struct {
	MessageType MessageType
	ServiceType ServiceType
	ConnID      string // 连接ID
	MessageID   string // 消息ID，由发送方生成
}

// APIResponse 统一的API响应结构体
type APIResponse struct {
	// Success 表示操作是否成功
	Success bool `json:"success"`
	// Type 响应类型，用于区分不同的响应
	Type string `json:"type,omitempty"`
	// Message 响应消息
	Message string `json:"message,omitempty"`
	// Data 响应数据，使用字符串传输，通常是JSON序列化后的数据
	Data string `json:"data,omitempty"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// Message 消息接口
type Message interface {
	network.Message
	// Header 获取消息头
	Header() *MessageHeader
	// Payload 获取消息体
	Payload() []byte
}

// CustomMessage 自定义消息实现
type CustomMessage struct {
	header  *MessageHeader
	payload []byte
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, svcType ServiceType, connID string, msgID string, payload []byte) *CustomMessage {
	return &CustomMessage{
		header: &MessageHeader{
			MessageType: msgType,
			ServiceType: svcType,
			ConnID:      connID,
			MessageID:   msgID,
		},
		payload: payload,
	}
}

// Header 获取消息头
func (m *CustomMessage) Header() *MessageHeader {
	return m.header
}

// Payload 获取消息体
func (m *CustomMessage) Payload() []byte {
	return m.payload
}

// ToBytes 将消息转换为字节数组
func (m *CustomMessage) ToBytes() []byte {
	// 消息格式: [消息类型(1字节)][服务类型(1字节)][消息ID(16字节)][消息体长度(4字节)][消息体(变长)]
	// 注意: 连接ID不在消息中传递，由接收方在处理消息时设置
	msgIDBytes := []byte(m.header.MessageID)
	msgIDLen := len(msgIDBytes) // 应该是固定长度的
	payloadLen := uint32(len(m.payload))

	// 计算总长度
	totalLen := 1 + 1 + msgIDLen + 4 + int(payloadLen)
	data := make([]byte, totalLen)

	// 写入消息头
	data[0] = uint8(m.header.MessageType)
	data[1] = uint8(m.header.ServiceType)

	// 写入消息ID (固定长度)
	offset := 2
	copy(data[offset:], msgIDBytes)
	offset += msgIDLen

	// 写入消息体长度 (大端序)
	data[offset] = byte(payloadLen >> 24)
	data[offset+1] = byte(payloadLen >> 16)
	data[offset+2] = byte(payloadLen >> 8)
	data[offset+3] = byte(payloadLen)
	offset += 4

	// 写入消息体
	copy(data[offset:], m.payload)

	return data
}

// MessageHandler 消息处理函数
type MessageHandler func(connID string, msg *CustomMessage) error

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

// API响应处理相关工具函数

// CreateAPIResponse 创建统一的API响应对象
func CreateAPIResponse(success bool, responseType string, message string, data interface{}, errMsg string) (*APIResponse, error) {
	dataStr := ""
	if str, ok := data.(string); ok {
		dataStr = str
	} else if data != nil {
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("序列化响应数据失败: %v", err)
		}
		dataStr = string(dataBytes)
	}

	return &APIResponse{
		Success: success,
		Type:    responseType,
		Message: message,
		Data:    dataStr,
		Error:   errMsg,
	}, nil
}

// SendAPIResponse 发送API响应
func SendAPIResponse(
	manager *ProtocolManager,
	connID string,
	msgID string,
	msgType MessageType,
	svcType ServiceType,
	success bool,
	responseType string,
	message string,
	data interface{},
	errMsg string) error {

	// 创建响应对象
	response, err := CreateAPIResponse(success, responseType, message, data, errMsg)
	if err != nil {
		return err
	}

	// 序列化响应
	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("序列化响应失败: %v", err)
	}

	// 创建并发送消息
	msg := NewMessage(msgType, svcType, connID, msgID, payload)

	conn, found := manager.GetConnection(connID)
	if !found {
		return fmt.Errorf("连接未找到: %s", connID)
	}

	return conn.Send(msg)
}

// SendServiceResponse 向指定连接发送服务响应
func SendServiceResponse(
	manager *ProtocolManager,
	connID string,
	msgID string,
	msgType MessageType,
	svcType ServiceType,
	success bool,
	responseType string,
	message string,
	data interface{},
	errMsg string) error {

	if manager == nil {
		return fmt.Errorf("协议管理器为空")
	}

	return SendAPIResponse(manager, connID, msgID, msgType, svcType, success, responseType, message, data, errMsg)
}

// SendErrorResponse 发送错误响应
func SendErrorResponse(
	manager *ProtocolManager,
	connID string,
	msgID string,
	msgType MessageType,
	svcType ServiceType,
	responseType string,
	err error) error {

	if manager == nil {
		return fmt.Errorf("协议管理器为空")
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	return SendAPIResponse(manager, connID, msgID, msgType, svcType, false, responseType, "", nil, errMsg)
}

// SendDirectResponse 直接发送响应消息
// 适用于不需要创建APIResponse的场景，比如已有序列化好的消息体
func SendDirectResponse(
	manager *ProtocolManager,
	connID string,
	msgID string,
	msgType MessageType,
	svcType ServiceType,
	payload []byte) error {

	if manager == nil {
		return fmt.Errorf("协议管理器为空")
	}

	msg := NewMessage(msgType, svcType, connID, msgID, payload)

	conn, found := manager.GetConnection(connID)
	if !found {
		return fmt.Errorf("连接未找到: %s", connID)
	}

	return conn.Send(msg)
}
