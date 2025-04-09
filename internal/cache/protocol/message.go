package protocol

// RESPMessageType 表示RESP消息类型
type RESPMessageType uint8

const (
	// MessageCommand 命令消息
	MessageCommand RESPMessageType = iota + 1
	// MessageReply 回复消息
	MessageReply
	// MessageHeartbeat 心跳消息
	MessageHeartbeat
)

// RESPMessage 实现网络消息接口
type RESPMessage struct {
	// 消息类型
	messageType RESPMessageType
	// 消息数据
	data []byte
}

// NewRESPMessage 创建RESP消息
func NewRESPMessage(messageType RESPMessageType, data []byte) *RESPMessage {
	return &RESPMessage{
		messageType: messageType,
		data:        data,
	}
}

// ToBytes 将消息转换为字节数组
func (m *RESPMessage) ToBytes() []byte {
	return m.data
}

// Type 返回消息类型
func (m *RESPMessage) Type() RESPMessageType {
	return m.messageType
}

// Data 返回消息数据
func (m *RESPMessage) Data() []byte {
	return m.data
}
