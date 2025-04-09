package protocol

import (
	"encoding/json"
	"log"
	"time"
)

// HeartbeatMessage 简单的心跳消息
type HeartbeatMessage struct {
	Timestamp int64 `json:"timestamp"`
}

// 定义心跳相关的消息类型常量
const (
	// 复制心跳消息类型
	MsgTypeReplHeartbeat MessageType = 107 // 心跳消息
)

// 全局协议管理器
var globalManager *ProtocolManager

// RegisterHeartbeatHandlers 注册心跳处理器
func RegisterHeartbeatHandlers(manager *ProtocolManager) {
	// 保存管理器引用
	globalManager = manager

	// 创建系统服务处理器
	sysHandler := NewServiceHandler()

	// 注册心跳消息处理函数
	sysHandler.RegisterHandler(MsgTypeHeartbeat, handleHeartbeat)
	sysHandler.RegisterHandler(MsgTypeHeartbeatAck, handleHeartbeatAck)

	// 注册系统服务
	manager.RegisterService(ServiceTypeSystem, "system", sysHandler)

	// 创建复制服务处理器
	replHandler := NewServiceHandler()

	// 注册复制心跳消息处理函数
	replHandler.RegisterHandler(MsgTypeReplHeartbeat, handleReplicationHeartbeat)

	// 注册复制服务
	manager.RegisterService(ServiceTypeReplication, "replication", replHandler)

	// 记录日志，确认注册成功
	log.Printf("已注册心跳处理器和复制心跳处理器，系统服务类型=%d，复制服务类型=%d", ServiceTypeSystem, ServiceTypeReplication)
	log.Printf("Handle:%d", MsgTypeReplHeartbeat)
}

// 处理心跳消息
func handleHeartbeat(connID string, msg *CustomMessage) error {
	// 简单记录收到心跳
	log.Printf("收到心跳: 连接ID=%s", connID)

	// 发送心跳响应
	return sendHeartbeatAck(connID)
}

// 处理复制心跳消息
func handleReplicationHeartbeat(connID string, msg *CustomMessage) error {
	// 简单记录收到复制心跳
	log.Printf("收到复制心跳: 连接ID=%s", connID)

	// 直接回复PONG响应
	return globalManager.SendMessage(connID, MsgTypeReplHeartbeat, ServiceTypeReplication, []byte("PONG"))
}

// 处理心跳响应
func handleHeartbeatAck(connID string, msg *CustomMessage) error {
	// 简单记录收到心跳响应
	log.Printf("收到心跳响应: 连接ID=%s", connID)
	return nil
}

// 发送心跳响应
func sendHeartbeatAck(connID string) error {
	msg := HeartbeatMessage{
		Timestamp: time.Now().UnixNano() / 1e6,
	}

	data, _ := json.Marshal(msg)

	return globalManager.SendMessage(connID, MsgTypeHeartbeatAck, ServiceTypeSystem, data)
}

// CreateHeartbeatMessage 创建心跳消息
func CreateHeartbeatMessage() []byte {
	msg := HeartbeatMessage{
		Timestamp: time.Now().UnixNano() / 1e6,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		// 心跳消息序列化失败，使用简单版本
		return []byte("heartbeat")
	}

	return data
}
