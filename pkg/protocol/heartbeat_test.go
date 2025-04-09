package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeartbeatMessage 测试心跳消息创建和解析
func TestHeartbeatMessage(t *testing.T) {
	// 测试心跳消息创建
	heartbeatData := CreateHeartbeatMessage()
	assert.NotEmpty(t, heartbeatData)

	// 解析心跳消息
	var heartbeatMsg HeartbeatMessage
	err := json.Unmarshal(heartbeatData, &heartbeatMsg)
	require.NoError(t, err)

	// 验证时间戳
	now := time.Now().UnixNano() / 1e6
	assert.InDelta(t, now, heartbeatMsg.Timestamp, 1000) // 允许1秒误差
}

// TestHeartbeatHandlerRegistration 测试心跳处理器注册
func TestHeartbeatHandlerRegistration(t *testing.T) {
	// 创建协议管理器
	manager := NewServer(nil)

	// 获取系统服务处理器
	// 注意：RegisterHeartbeatHandlers已在NewProtocolManager中调用
	protocol := manager.protocol
	handler, ok := protocol.serviceHandlers[ServiceTypeSystem]
	require.True(t, ok, "系统服务处理器应该已注册")

	// 验证心跳消息处理函数已注册
	_, ok = handler.GetHandler(MsgTypeHeartbeat)
	assert.True(t, ok, "心跳消息处理函数应该已注册")

	// 验证心跳响应消息处理函数已注册
	_, ok = handler.GetHandler(MsgTypeHeartbeatAck)
	assert.True(t, ok, "心跳响应消息处理函数应该已注册")
}

// 此测试需要模拟网络连接，由于依赖较多，我们仅测试消息格式
func TestHeartbeatMessaging(t *testing.T) {
	// 测试心跳消息格式
	msgType := MsgTypeHeartbeat
	svcType := ServiceTypeSystem
	connID := "test-conn"
	msgID := generateMsgID()
	payload := CreateHeartbeatMessage()

	// 创建心跳消息
	msg := NewMessage(msgType, svcType, connID, msgID, payload)

	// 验证心跳消息头
	assert.Equal(t, msgType, msg.Header().MessageType)
	assert.Equal(t, svcType, msg.Header().ServiceType)
	assert.Equal(t, connID, msg.Header().ConnID)

	// 序列化消息
	data := msg.ToBytes()

	// 创建协议解析器
	protocol := NewCustomProtocol()

	// 解析心跳消息
	parsedMsg, err := protocol.ParseMessage(data)
	require.NoError(t, err)

	// 验证解析结果
	assert.Equal(t, msgType, parsedMsg.Header().MessageType)
	assert.Equal(t, svcType, parsedMsg.Header().ServiceType)
	assert.Equal(t, msgID, parsedMsg.Header().MessageID)

	// 解析心跳消息内容
	var heartbeatMsg HeartbeatMessage
	err = json.Unmarshal(parsedMsg.Payload(), &heartbeatMsg)
	require.NoError(t, err)

	// 验证时间戳
	assert.Greater(t, heartbeatMsg.Timestamp, int64(0))
}
