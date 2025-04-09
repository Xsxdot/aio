package mq

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func TestProducerConsumerIntegration(t *testing.T) {
	// 创建日志记录器
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "nats-integration-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建服务器配置
	serverConfig := NewDefaultServerConfig("integration-test-server", tempDir)
	serverConfig.Port = 14226 // 使用非默认端口避免冲突
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

	// 创建消费者
	consumer, err := NewConsumer(client, logger)
	if err != nil {
		t.Fatalf("创建消费者失败: %v", err)
	}
	defer consumer.Close()

	// 测试发布-订阅
	t.Run("PubSub", func(t *testing.T) {
		// 创建接收通道
		received := make(chan TestMessage, 10)

		// 创建消息处理函数
		handler := func(msg *Message) error {
			var data TestMessage
			if err := UnmarshalJson(msg, &data); err != nil {
				t.Errorf("解析消息失败: %v", err)
				return err
			}
			received <- data
			return nil
		}

		// 订阅主题
		subID, err := consumer.Subscribe("test.integration.pubsub", handler)
		if err != nil {
			t.Fatalf("订阅失败: %v", err)
		}
		defer consumer.Unsubscribe(subID)

		// 发布一系列消息
		expectedMessages := []TestMessage{
			{ID: "int-1", Content: "集成测试消息1"},
			{ID: "int-2", Content: "集成测试消息2"},
			{ID: "int-3", Content: "集成测试消息3"},
		}

		for _, msg := range expectedMessages {
			if err := producer.Publish("test.integration.pubsub", msg); err != nil {
				t.Fatalf("发布消息失败: %v", err)
			}
		}

		// 接收所有消息
		receivedMessages := make([]TestMessage, 0, len(expectedMessages))
		for i := 0; i < len(expectedMessages); i++ {
			select {
			case msg := <-received:
				receivedMessages = append(receivedMessages, msg)
			case <-time.After(2 * time.Second):
				t.Fatalf("消息接收超时，仅收到 %d/%d 条消息", len(receivedMessages), len(expectedMessages))
			}
		}

		// 验证所有消息都已接收
		if len(receivedMessages) != len(expectedMessages) {
			t.Errorf("接收的消息数量不正确: 期望 %d, 实际 %d", len(expectedMessages), len(receivedMessages))
		}

		// 验证消息顺序和内容（在这个简单测试中，我们假设消息按顺序接收）
		for i, expected := range expectedMessages {
			if i >= len(receivedMessages) {
				break
			}
			actual := receivedMessages[i]
			if expected.ID != actual.ID || expected.Content != actual.Content {
				t.Errorf("消息 #%d 不匹配: 期望 %+v, 实际 %+v", i, expected, actual)
			}
		}
	})

	// 测试请求-响应模式
	t.Run("RequestResponse", func(t *testing.T) {
		// 创建服务处理函数
		handler := func(msg *Message) error {
			var request TestMessage
			if err := UnmarshalJson(msg, &request); err != nil {
				return err
			}

			// 构建响应
			response := TestMessage{
				ID:      "response-to-" + request.ID,
				Content: "响应: " + request.Content,
			}

			// 发送响应
			return ReplyJson(msg, response)
		}

		// 订阅服务主题
		subID, err := consumer.Subscribe("test.integration.service", handler)
		if err != nil {
			t.Fatalf("订阅服务主题失败: %v", err)
		}
		defer consumer.Unsubscribe(subID)

		// 发送请求
		request := TestMessage{
			ID:      "req-int-123",
			Content: "请求消息",
		}

		var response TestMessage
		err = producer.Request("test.integration.service", request, &response, 2*time.Second)
		if err != nil {
			t.Fatalf("发送请求失败: %v", err)
		}

		// 验证响应
		expectedResponse := TestMessage{
			ID:      "response-to-req-int-123",
			Content: "响应: 请求消息",
		}

		if response.ID != expectedResponse.ID || response.Content != expectedResponse.Content {
			t.Errorf("响应不匹配: 期望 %+v, 实际 %+v", expectedResponse, response)
		}
	})

	// 测试队列订阅（负载均衡）
	t.Run("QueueSubscribe", func(t *testing.T) {
		// 设置参数
		queueSubject := "test.integration.queue"
		queueName := "int-workers"
		messageCount := 20
		workerCount := 3

		// 创建计数器
		var (
			mu              sync.Mutex
			workerMessages  = make(map[int]int)
			wg              sync.WaitGroup
			done            = make(chan struct{})
			subscriptionIDs = make([]string, workerCount)
		)

		// 启动多个工作程序
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// 创建队列处理函数
				handler := func(msg *Message) error {
					var data TestMessage
					if err := UnmarshalJson(msg, &data); err != nil {
						t.Errorf("解析消息失败: %v", err)
						return err
					}

					mu.Lock()
					workerMessages[workerID]++
					mu.Unlock()

					return nil
				}

				// 订阅队列
				subID, err := consumer.QueueSubscribe(queueSubject, queueName, handler)
				if err != nil {
					t.Errorf("队列订阅失败: %v", err)
					return
				}

				subscriptionIDs[workerID] = subID

				// 等待测试完成信号
				<-done

				// 取消订阅
				if err := consumer.Unsubscribe(subID); err != nil {
					t.Errorf("取消订阅失败: %v", err)
				}
			}(i)
		}

		// 等待一小段时间让所有订阅生效
		time.Sleep(1 * time.Second)

		// 发送消息
		for i := 0; i < messageCount; i++ {
			msg := TestMessage{
				ID:      "q-" + fmt.Sprintf("%d", i),
				Content: "队列消息 " + fmt.Sprintf("%d", i),
			}

			if err := producer.Publish(queueSubject, msg); err != nil {
				t.Fatalf("发布队列消息失败: %v", err)
			}
		}

		// 等待消息处理完成
		time.Sleep(2 * time.Second)

		// 发送完成信号
		close(done)

		// 等待所有工作程序退出
		wg.Wait()

		// 统计结果
		totalProcessed := 0
		mu.Lock()
		for worker, count := range workerMessages {
			totalProcessed += count
			t.Logf("工作程序 #%d 处理了 %d 条消息", worker, count)
		}
		mu.Unlock()

		// 验证所有消息都被处理了
		if totalProcessed != messageCount {
			t.Errorf("处理的消息数量不正确: 期望 %d, 实际 %d", messageCount, totalProcessed)
		}

		// 验证负载是否分布在多个工作程序上
		if len(workerMessages) < 2 && workerCount > 1 {
			t.Errorf("负载分布不均衡: 只有 %d/%d 个工作程序处理了消息", len(workerMessages), workerCount)
		}
	})

	// 测试JetStream持久化
	if client.js != nil {
		t.Run("JetStream", func(t *testing.T) {
			// 创建JetStream流
			streamName := "INTEGRATION_TEST"
			_, err = client.js.AddStream(&nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{"test.jetstream.integration.*"},
				MaxAge:   time.Minute,
			})
			if err != nil {
				t.Fatalf("创建JetStream流失败: %v", err)
			}
			defer client.js.DeleteStream(streamName)

			// 创建接收通道
			jsReceived := make(chan TestMessage, 10)

			// 创建消息处理函数
			jsHandler := func(msg *Message) error {
				var data TestMessage
				if err := UnmarshalJson(msg, &data); err != nil {
					t.Errorf("解析JetStream消息失败: %v", err)
					return err
				}
				jsReceived <- data
				return nil
			}

			// 订阅JetStream主题
			_, err = consumer.JetStreamSubscribe(
				"test.jetstream.integration.pubsub",
				jsHandler,
				WithDurable("integration-test"),
				WithDeliverAll(),
			)
			if err != nil {
				t.Fatalf("JetStream订阅失败: %v", err)
			}

			// 发布JetStream消息
			jsMsg := TestMessage{
				ID:      "js-int-123",
				Content: "JetStream集成测试消息",
			}

			for i := 0; i < 3; i++ {
				ack, err := producer.PublishAsync("test.jetstream.integration.pubsub", jsMsg)
				if err != nil {
					t.Fatalf("JetStream发布失败: %v", err)
				}

				// 等待确认
				select {
				case <-ack.Ok():
					// 确认成功
				case err := <-ack.Err():
					t.Fatalf("JetStream发布确认失败: %v", err)
				case <-time.After(2 * time.Second):
					t.Fatal("等待JetStream确认超时")
				}
			}

			// 接收所有消息
			for i := 0; i < 3; i++ {
				select {
				case received := <-jsReceived:
					if received.ID != jsMsg.ID || received.Content != jsMsg.Content {
						t.Errorf("接收的JetStream消息不匹配: 期望 %+v, 实际 %+v", jsMsg, received)
					}
				case <-time.After(2 * time.Second):
					t.Fatalf("JetStream消息接收超时，仅收到 %d/3 条消息", i)
				}
			}
		})
	} else {
		t.Log("JetStream未启用，跳过JetStream集成测试")
	}
}
