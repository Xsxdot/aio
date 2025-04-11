package protocol

import (
	"encoding/json"
	"errors"
	"github.com/xsxdot/aio/pkg/network"
)

// 错误定义
var (
	ErrServiceNotFound    = errors.New("service not found")
	ErrHandlerNotFound    = errors.New("handler not found")
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrInvalidServiceType = errors.New("invalid service type")
	OK                    = "OK"
)

// MessageType 消息类型
type MessageType uint8

// 定义系统消息类型
const (
	MsgTypeHeartbeat MessageType = 1 //心跳消息
	MsgTypeAuth      MessageType = 2 //认证消息

	MsgTypeResponseSuccess MessageType = 1
	MsgTypeResponseFail    MessageType = 2
)

// ServiceType 服务类型
type ServiceType uint8

// 定义系统服务类型
const (
	// ServiceTypeSystem 系统服务（用于心跳等底层协议消息）
	ServiceTypeSystem      ServiceType = 1
	ServiceTypeResponse    ServiceType = 2
	ServiceTypeConfig      ServiceType = 10
	ServiceTypeReplication ServiceType = 11 // 复制服务
	ServiceTypeElection    ServiceType = 12
	ServiceTypeDiscovery   ServiceType = 13
)

// MessageHeader 消息头
type MessageHeader struct {
	MessageType MessageType
	ServiceType ServiceType
	ConnID      string // 连接ID
	MessageID   string // 消息ID，由发送方生成
}

// Response 统一的响应结构体
type Response struct {
	header *MessageHeader
	// OriginMsgId 原始消息ID，用于标识消息
	OriginMsgId string `json:"originMsgId"`
	// Data 响应数据，使用字符串传输，通常是JSON序列化后的数据,成功则返回结果，失败则返回原因
	Data []byte `json:"data,omitempty"`
}

func (r *Response) Header() *MessageHeader {
	return r.header
}

func (r *Response) Payload() []byte {
	return r.Data
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

var HeartbeatMsg = &CustomMessage{
	header: &MessageHeader{
		MessageType: MsgTypeHeartbeat,
		ServiceType: ServiceTypeSystem,
		ConnID:      "1744356370045651000",
		MessageID:   generateMsgID(),
	},
	payload: []byte("null"),
}

// NewParseMessage 创建一个被解析的消息
func NewParseMessage(msgType MessageType, svcType ServiceType, connID, msgID string, payload []byte) *CustomMessage {
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

// NewMessage 创建新消息
func NewMessage(msgType MessageType, svcType ServiceType, connID string, payload interface{}) *CustomMessage {
	jsonBytes, _ := json.Marshal(payload)
	return &CustomMessage{
		header: &MessageHeader{
			MessageType: msgType,
			ServiceType: svcType,
			ConnID:      connID,
			MessageID:   generateMsgID(),
		},
		payload: jsonBytes,
	}
}

func NewSuccessResponse(connId string, originMsgId string, result interface{}) *CustomMessage {
	var jsonBytes []byte
	if str, ok := result.(string); ok {
		jsonBytes = []byte(str)
	} else {
		jsonBytes, _ = json.Marshal(result)
	}
	a := &Response{
		OriginMsgId: originMsgId,
		Data:        jsonBytes,
	}
	payload, _ := json.Marshal(a)

	return &CustomMessage{
		header: &MessageHeader{
			MessageType: MsgTypeResponseSuccess,
			ServiceType: ServiceTypeResponse,
			ConnID:      connId,
			MessageID:   generateMsgID(),
		},
		payload: payload,
	}
}

func NewFailResponse(connId string, originMsgId string, err error) *CustomMessage {
	a := &Response{
		OriginMsgId: originMsgId,
		Data:        []byte(err.Error()),
	}
	payload, _ := json.Marshal(a)

	return &CustomMessage{
		header: &MessageHeader{
			MessageType: MsgTypeResponseFail,
			ServiceType: ServiceTypeResponse,
			ConnID:      connId,
			MessageID:   generateMsgID(),
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
