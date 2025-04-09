package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestManagerCreation 测试协议管理器创建
func TestManagerCreation(t *testing.T) {
	manager := NewServer(nil)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.protocol)
	assert.NotNil(t, manager.serviceMap)

	// 验证系统服务已注册
	svcName, ok := manager.serviceMap[ServiceTypeSystem]
	assert.True(t, ok)
	assert.Equal(t, "system", svcName)
}

// TestServiceRegistration 测试服务注册
func TestServiceRegistration(t *testing.T) {
	manager := NewServer(nil)

	// 创建测试服务
	svcType := ServiceType(10)
	svcName := "test-service"
	handler := NewServiceHandler()

	// 注册测试消息类型
	msgType := MessageType(20)
	handler.RegisterHandler(msgType, func(connID string, msg *CustomMessage) error {
		// 只是一个测试处理器
		return nil
	})

	// 注册服务
	manager.RegisterService(svcType, svcName, handler)

	// 验证服务已注册到映射表
	name, ok := manager.serviceMap[svcType]
	assert.True(t, ok)
	assert.Equal(t, svcName, name)

	// 验证服务处理器已注册到协议
	registeredHandler, ok := manager.protocol.serviceHandlers[svcType]
	assert.True(t, ok)
	assert.Equal(t, handler, registeredHandler)

	// 验证消息处理函数可以获取
	msgHandler, ok := registeredHandler.GetHandler(msgType)
	assert.True(t, ok)
	assert.NotNil(t, msgHandler)
}

// TestMessageIDGeneration 测试消息ID生成功能
func TestMessageIDGeneration(t *testing.T) {
	id1 := generateMsgID()
	id2 := generateMsgID()

	// 验证ID长度
	assert.Equal(t, 16, len(id1))
	assert.Equal(t, 16, len(id2))

	// 验证两个ID不同
	assert.NotEqual(t, id1, id2)
}

// TestManagerMethods 测试管理器方法调用
func TestManagerMethods(t *testing.T) {
	manager := NewServer(nil)

	// 测试GetConnectionCount - 应该返回0，因为没有启动服务器
	count := manager.GetConnectionCount()
	assert.Equal(t, 0, count)

	// 无法测试需要启动服务器的方法，但确保它们存在且不会崩溃
	assert.NotPanics(t, func() {
		// 这些方法不会实际执行，因为没有启动服务器，但它们应该存在
		_ = manager.Stop()
		_ = manager.CloseConnection("non-existent")
	})
}
