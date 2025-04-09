package protocol

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMessageCreation 测试消息创建功能
func TestMessageCreation(t *testing.T) {
	// 准备测试数据
	msgType := MessageType(10)
	svcType := ServiceType(20)
	connID := "conn-123"
	msgID := "msg-12345678901234"
	payload := []byte("test payload")

	// 创建消息
	msg := NewMessage(msgType, svcType, connID, msgID, payload)

	// 验证消息头部
	assert.Equal(t, msgType, msg.Header().MessageType)
	assert.Equal(t, svcType, msg.Header().ServiceType)
	assert.Equal(t, connID, msg.Header().ConnID)
	assert.Equal(t, msgID, msg.Header().MessageID)

	// 验证消息负载
	assert.Equal(t, payload, msg.Payload())
}

// TestMessageSerialization 测试消息序列化和反序列化
func TestMessageSerialization(t *testing.T) {
	// 准备测试数据
	msgType := MessageType(10)
	svcType := ServiceType(20)
	connID := "conn-123"
	msgID := "1234567890123456" // 确保是16字节
	payload := []byte("test payload")

	// 创建消息并序列化
	msg := NewMessage(msgType, svcType, connID, msgID, payload)
	data := msg.ToBytes()

	// 验证序列化后的数据格式
	// 格式: [消息类型(1字节)][服务类型(1字节)][消息ID(16字节)][消息体长度(4字节)][消息体(变长)]
	assert.Equal(t, byte(msgType), data[0])
	assert.Equal(t, byte(svcType), data[1])

	// 验证消息ID (注意connID不在序列化数据中)
	assert.Equal(t, []byte(msgID), data[2:18])

	// 验证消息体长度 (大端序)
	payloadLen := uint32(len(payload))
	assert.Equal(t, byte(payloadLen>>24), data[18])
	assert.Equal(t, byte(payloadLen>>16), data[19])
	assert.Equal(t, byte(payloadLen>>8), data[20])
	assert.Equal(t, byte(payloadLen), data[21])

	// 验证消息体
	assert.Equal(t, payload, data[22:])

	// 创建协议解析器并测试解析
	protocol := NewCustomProtocol()
	parsedMsg, err := protocol.ParseMessage(data)

	// 验证解析结果
	assert.NoError(t, err)
	assert.Equal(t, msgType, parsedMsg.Header().MessageType)
	assert.Equal(t, svcType, parsedMsg.Header().ServiceType)
	assert.Equal(t, msgID, parsedMsg.Header().MessageID)
	assert.Equal(t, payload, parsedMsg.Payload())
}

// TestMsgIDGeneration 测试消息ID生成的唯一性
func TestMsgIDGeneration(t *testing.T) {
	// 生成大量ID并检查唯一性
	idCount := 1000
	idMap := make(map[string]bool)

	for i := 0; i < idCount; i++ {
		id := generateMsgID()

		// 验证ID长度
		assert.Equal(t, 16, len(id))

		// 验证ID唯一性
		_, exists := idMap[id]
		assert.False(t, exists, "ID重复: %s", id)

		idMap[id] = true
	}
}

// TestMsgIDGenerationConcurrent 测试并发条件下消息ID生成的唯一性
func TestMsgIDGenerationConcurrent(t *testing.T) {
	// 并发生成大量ID并检查唯一性
	idCount := 1000
	concurrency := 10
	var wg sync.WaitGroup

	// 使用互斥锁保护map
	var mu sync.Mutex
	idMap := make(map[string]bool)

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localIDs := make([]string, 0, idCount/concurrency)
			// 生成ID
			for i := 0; i < idCount/concurrency; i++ {
				id := generateMsgID()
				localIDs = append(localIDs, id)
			}

			// 检查唯一性
			mu.Lock()
			defer mu.Unlock()

			for _, id := range localIDs {
				// 验证ID长度
				assert.Equal(t, 16, len(id))

				// 验证ID唯一性
				_, exists := idMap[id]
				assert.False(t, exists, "ID重复: %s", id)

				idMap[id] = true
			}
		}()
	}

	wg.Wait()
}

// TestServiceHandler 测试服务处理器功能
func TestServiceHandler(t *testing.T) {
	// 创建服务处理器
	handler := NewServiceHandler()

	// 定义测试消息类型
	msgType1 := MessageType(1)
	msgType2 := MessageType(2)

	// 注册处理函数
	handlerCalled1 := false
	handler.RegisterHandler(msgType1, func(connID string, msg *CustomMessage) error {
		handlerCalled1 = true
		return nil
	})

	handlerCalled2 := false
	handler.RegisterHandler(msgType2, func(connID string, msg *CustomMessage) error {
		handlerCalled2 = true
		return nil
	})

	// 获取并调用处理函数
	h1, ok := handler.GetHandler(msgType1)
	assert.True(t, ok)
	err := h1("test-conn", &CustomMessage{})
	assert.NoError(t, err)
	assert.True(t, handlerCalled1)

	h2, ok := handler.GetHandler(msgType2)
	assert.True(t, ok)
	err = h2("test-conn", &CustomMessage{})
	assert.NoError(t, err)
	assert.True(t, handlerCalled2)

	// 测试获取不存在的处理函数
	_, ok = handler.GetHandler(MessageType(99))
	assert.False(t, ok)
}
