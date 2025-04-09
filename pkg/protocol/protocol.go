package protocol

import (
	"fmt"
	network2 "github.com/xsxdot/aio/pkg/network"
	netprotocol "github.com/xsxdot/aio/pkg/network/protocol"
	"io"
	"log"
)

// CustomProtocol 自定义协议
type CustomProtocol struct {
	// 协议使用长度字段协议作为基础传输协议
	baseProtocol network2.Protocol
	// 服务处理器映射
	serviceHandlers map[ServiceType]*ServiceHandler
}

// NewCustomProtocol 创建自定义协议
func NewCustomProtocol() *CustomProtocol {
	// 使用4字节长度头，最大消息大小为10MB
	baseProtocol := netprotocol.NewLengthFieldProtocol(4, 10*1024*1024)
	return &CustomProtocol{
		baseProtocol:    baseProtocol,
		serviceHandlers: make(map[ServiceType]*ServiceHandler),
	}
}

// RegisterService 注册服务处理器
func (p *CustomProtocol) RegisterService(svcType ServiceType, handler *ServiceHandler) {
	p.serviceHandlers[svcType] = handler
}

// Read 读取消息
func (p *CustomProtocol) Read(reader io.Reader) ([]byte, error) {
	// 使用基础协议读取数据
	return p.baseProtocol.Read(reader)
}

// Write 写入消息
func (p *CustomProtocol) Write(writer io.Writer, data []byte) error {
	// 使用基础协议写入数据
	return p.baseProtocol.Write(writer, data)
}

// Name 获取协议名称
func (p *CustomProtocol) Name() string {
	return "custom-protocol"
}

// ParseMessage 解析消息
func (p *CustomProtocol) ParseMessage(data []byte) (*CustomMessage, error) {
	// 定义消息ID的固定长度 (雪花ID格式)
	const msgIDLen = 16

	if len(data) < 2+msgIDLen { // 至少需要消息类型、服务类型和消息ID
		return nil, fmt.Errorf("message too short")
	}

	// 解析消息头
	msgType := MessageType(data[0])
	svcType := ServiceType(data[1])

	// 解析消息ID (固定长度)
	offset := 2
	msgID := string(data[offset : offset+msgIDLen])
	offset += msgIDLen

	// 解析消息体长度
	if len(data) < offset+4 { // 前面的部分+消息体长度
		return nil, fmt.Errorf("invalid message format")
	}

	payloadLen := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3])
	offset += 4

	if len(data) < offset+int(payloadLen) {
		return nil, fmt.Errorf("payload too short")
	}

	// 解析消息体
	payload := data[offset : offset+int(payloadLen)]

	// 注意：连接ID为空，将在消息处理阶段通过连接对象设置
	return NewMessage(msgType, svcType, "", msgID, payload), nil
}

// ProcessMessage 处理消息
func (p *CustomProtocol) ProcessMessage(conn *network2.Connection, data []byte) error {
	msg, err := p.ParseMessage(data)
	if err != nil {
		return err
	}

	log.Printf("[协议] 收到消息内容: %s", string(msg.Payload()))

	// 设置连接ID
	msg.header.ConnID = conn.ID()

	// 获取服务处理器
	serviceHandler, ok := p.serviceHandlers[msg.Header().ServiceType]
	if !ok {
		return ErrServiceNotFound
	}

	// 获取消息处理函数
	handler, ok := serviceHandler.GetHandler(msg.Header().MessageType)
	if !ok {
		println(fmt.Sprintf("Handle:%d", msg.Header().MessageType))
		return ErrHandlerNotFound
	}

	// 处理消息
	return handler(msg.Header().ConnID, msg)
}

// CreateNetworkHandler 创建网络处理器
func (p *CustomProtocol) CreateNetworkHandler() *network2.MessageHandler {
	return &network2.MessageHandler{
		Handle: func(conn *network2.Connection, data []byte) error {
			// 处理接收到的消息
			return p.ProcessMessage(conn, data)
		},
		GetHeartbeat: func() network2.Message {
			// 创建简单的心跳消息
			return NewMessage(MsgTypeHeartbeat, ServiceTypeSystem, "", generateMsgID(), CreateHeartbeatMessage())
		},
	}
}
