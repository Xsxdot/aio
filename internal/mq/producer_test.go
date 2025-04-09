package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type TestMessage struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func TestProducer(t *testing.T) {
	// 创建日志记录器
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "nats-producer-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建服务器配置
	serverConfig := NewDefaultServerConfig("producer-test-server", tempDir)
	serverConfig.Port = 14224 // 使用非默认端口避免冲突
	// 不设置集群端口和名称

	// 创建并启动服务器
	server, err := NewNatsServer(serverConfig, logger)
	if err != nil {
		t.Fatalf("创建NATS服务器失败: %v", err)
	}
	defer server.Close()

	// 让服务器完全启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端配置
	config := NewDefaultClientConfig()
	config.Servers = []string{fmt.Sprintf("nats://localhost:%d", serverConfig.Port)}

	// 创建客户端
	client, err := NewNatsClient(config, logger)
	if err != nil {
		t.Fatalf("创建NATS客户端失败: %v", err)
	}
	defer client.Close()

	// 创建生产者
	producer, err := NewProducer(client, logger)
	if err != nil {
		t.Fatalf("创建生产者失败: %v", err)
	}

	// 测试发布消息
	testSubject := "test.producer.publish"
	testMessage := TestMessage{
		ID:      "test-123",
		Content: "这是一条测试消息",
	}

	// 监听接收
	received := make(chan bool, 1)
	msgData := make(chan []byte, 1)

	// 订阅消息
	sub, err := client.conn.Subscribe(testSubject, func(msg *nats.Msg) {
		msgData <- msg.Data
		received <- true
	})
	if err != nil {
		t.Fatalf("订阅主题失败: %v", err)
	}
	defer sub.Unsubscribe()

	// 发布消息
	err = producer.Publish(testSubject, testMessage)
	if err != nil {
		t.Fatalf("发布消息失败: %v", err)
	}

	// 等待接收消息
	select {
	case <-received:
		// 验证收到的消息
		data := <-msgData
		var receivedMsg TestMessage
		err := json.Unmarshal(data, &receivedMsg)
		if err != nil {
			t.Fatalf("解析接收到的消息失败: %v", err)
		}
		if receivedMsg.ID != testMessage.ID || receivedMsg.Content != testMessage.Content {
			t.Errorf("接收到的消息与发送的不匹配, 收到: %+v, 期望: %+v", receivedMsg, testMessage)
		}
	case <-time.After(2 * time.Second):
		t.Error("超时未收到消息")
	}

	// 测试带上下文的发布
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = producer.PublishWithContext(ctx, testSubject, testMessage)
	if err != nil {
		t.Fatalf("带上下文发布消息失败: %v", err)
	}

	// 等待接收消息
	select {
	case <-received:
		// 成功接收
	case <-time.After(2 * time.Second):
		t.Error("超时未收到带上下文发布的消息")
	}

	// 测试请求-响应模式
	requestSubject := "test.producer.request"

	// 创建响应处理程序
	_, err = client.conn.Subscribe(requestSubject, func(msg *nats.Msg) {
		var request TestMessage
		if err := json.Unmarshal(msg.Data, &request); err != nil {
			t.Errorf("解析请求消息失败: %v", err)
			return
		}

		// 发送响应
		response := TestMessage{
			ID:      "response-" + request.ID,
			Content: "响应: " + request.Content,
		}
		responseData, _ := json.Marshal(response)
		msg.Respond(responseData)
	})
	if err != nil {
		t.Fatalf("订阅请求主题失败: %v", err)
	}

	// 发送请求
	requestMsg := TestMessage{
		ID:      "req-123",
		Content: "请求消息",
	}
	var responseMsg TestMessage

	err = producer.Request(requestSubject, requestMsg, &responseMsg, 2*time.Second)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	// 验证响应
	if responseMsg.ID != "response-req-123" || responseMsg.Content != "响应: 请求消息" {
		t.Errorf("响应消息与预期不符, 收到: %+v", responseMsg)
	}

	// 测试异步发布（需要JetStream支持）
	if client.js != nil {
		// 创建一个JetStream流
		_, err = client.js.AddStream(&nats.StreamConfig{
			Name:     "TEST_STREAM",
			Subjects: []string{"test.async.*"},
		})
		if err != nil {
			t.Fatalf("创建JetStream流失败: %v", err)
		}

		// 异步发布消息
		asyncSubject := "test.async.publish"
		future, err := producer.PublishAsync(asyncSubject, testMessage)
		if err != nil {
			t.Fatalf("异步发布消息失败: %v", err)
		}

		// 等待确认
		select {
		case ack := <-future.Ok():
			t.Logf("异步发布消息已确认: stream=%s, sequence=%d", ack.Stream, ack.Sequence)
		case err := <-future.Err():
			t.Fatalf("异步发布消息失败: %v", err)
		case <-time.After(5 * time.Second):
			t.Error("等待异步发布确认超时")
		}
	} else {
		t.Log("JetStream未启用，跳过异步发布测试")
	}

	// 测试关闭生产者
	producer.Close()
}

func TestProducerWithNilClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 测试使用nil客户端创建生产者
	_, err := NewProducer(nil, logger)
	if err == nil {
		t.Error("使用nil客户端创建生产者应该失败，但成功了")
	}
}
