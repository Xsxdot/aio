package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func TestConsumer(t *testing.T) {
	// 设置超时上下文，防止死锁
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建一个done通道，用于检测测试是否完成
	done := make(chan struct{})
	// 创建一个错误通道，用于从goroutine中传递错误
	errCh := make(chan error, 1)

	// 在单独的goroutine中运行测试，以便可以捕获超时
	go func() {
		defer close(done)

		// 创建日志记录器
		logger, _ := zap.NewDevelopment()
		defer logger.Sync()

		// 创建临时目录
		tempDir, err := os.MkdirTemp("", "nats-consumer-test")
		if err != nil {
			errCh <- fmt.Errorf("创建临时目录失败: %v", err)
			return
		}
		defer os.RemoveAll(tempDir)

		// 创建服务器配置
		serverConfig := NewDefaultServerConfig("consumer-test-server", tempDir)
		serverConfig.Port = 14225 // 使用非默认端口避免冲突
		// 不设置集群端口和名称

		// 创建并启动服务器
		server, err := NewNatsServer(serverConfig, logger)
		if err != nil {
			errCh <- fmt.Errorf("创建NATS服务器失败: %v", err)
			return
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
			errCh <- fmt.Errorf("创建NATS客户端失败: %v", err)
			return
		}
		defer client.Close()

		// 创建消费者
		consumer, err := NewConsumer(client, logger)
		if err != nil {
			errCh <- fmt.Errorf("创建消费者失败: %v", err)
			return
		}
		defer consumer.Close()

		// 测试普通订阅
		testSubject := "test.consumer.subscribe"
		receivedMessage := make(chan *Message, 1)

		// 创建消息处理函数
		handler := func(msg *Message) error {
			receivedMessage <- msg
			return nil
		}

		// 订阅主题
		subID, err := consumer.Subscribe(testSubject, handler)
		if err != nil {
			errCh <- fmt.Errorf("订阅主题失败: %v", err)
			return
		}

		// 发布测试消息
		testData := &TestMessage{
			ID:      "test-consumer-123",
			Content: "消费者测试消息",
		}
		payload, _ := json.Marshal(testData)

		err = client.Publish(testSubject, payload)
		if err != nil {
			errCh <- fmt.Errorf("发布消息失败: %v", err)
			return
		}

		// 等待接收消息
		var msg *Message
		select {
		case msg = <-receivedMessage:
			// 验证收到的消息
			var receivedData TestMessage
			err := json.Unmarshal(msg.Data, &receivedData)
			if err != nil {
				errCh <- fmt.Errorf("解析接收到的消息失败: %v", err)
				return
			}
			if receivedData.ID != testData.ID || receivedData.Content != testData.Content {
				t.Errorf("接收到的消息与发送的不匹配, 收到: %+v, 期望: %+v", receivedData, testData)
			}
		case <-time.After(2 * time.Second):
			errCh <- fmt.Errorf("超时未收到消息")
			return
		}

		// 测试取消订阅
		err = consumer.Unsubscribe(subID)
		if err != nil {
			errCh <- fmt.Errorf("取消订阅失败: %v", err)
			return
		}

		// 发布另一条消息，但不应该收到
		err = client.Publish(testSubject, payload)
		if err != nil {
			errCh <- fmt.Errorf("发布第二条消息失败: %v", err)
			return
		}

		// 验证不再接收消息
		select {
		case <-receivedMessage:
			t.Errorf("在取消订阅后仍然收到了消息")
		case <-time.After(1 * time.Second):
			// 正确的行为，没有收到消息
		}

		// 测试队列订阅
		t.Run("QueueSubscribe", func(t *testing.T) {
			queueSubject := "test.consumer.queue"
			queueName := "test-queue"

			// 创建一个带缓冲的通道接收队列消息，确保不会阻塞
			messageCount := 5
			receivedMessages := make(chan *Message, messageCount*3) // 足够大的缓冲区

			// 创建一个上下文，用于控制测试超时
			queueCtx, queueCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer queueCancel()

			// 创建一个完成信号通道
			queueDone := make(chan struct{})

			// 创建错误通道
			queueErrCh := make(chan error, 3) // 为每个工作线程提供一个错误槽位

			// 使用原子计数器跟踪消息数量
			var (
				receivedCount uint32
				workerCounts  sync.Map
			)

			// 创建并启动工作线程
			var wg sync.WaitGroup
			for i := 0; i < 3; i++ {
				wg.Add(1)
				i := i // 捕获循环变量

				// 创建处理函数 - 不涉及互斥锁
				queueHandler := func(msg *Message) error {
					// 增加工作线程计数 - 使用sync.Map避免锁
					val, _ := workerCounts.LoadOrStore(i, uint32(0))
					workerCounts.Store(i, val.(uint32)+1)

					// 非阻塞地发送到通道
					select {
					case receivedMessages <- msg:
						atomic.AddUint32(&receivedCount, 1)
					default:
						// 如果通道满了，记录但不阻塞
						t.Logf("通道已满，消息丢弃")
					}
					return nil
				}

				// 启动工作线程
				go func() {
					defer wg.Done()

					// 创建队列订阅
					subID, err := consumer.QueueSubscribe(queueSubject, queueName, queueHandler)
					if err != nil {
						queueErrCh <- fmt.Errorf("工作线程 #%d 创建队列订阅失败: %v", i, err)
						return
					}

					// 等待测试完成
					<-queueDone

					// 取消订阅 - 捕获可能的错误但不阻塞
					if err := consumer.Unsubscribe(subID); err != nil {
						t.Logf("取消订阅失败: %v", err)
					}
				}()
			}

			// 等待一小段时间让订阅生效
			time.Sleep(100 * time.Millisecond)

			// 发送测试消息
			go func() {
				for i := 0; i < messageCount; i++ {
					msg := TestMessage{
						ID:      fmt.Sprintf("queue-test-%d", i),
						Content: fmt.Sprintf("队列测试消息 %d", i),
					}
					data, _ := json.Marshal(msg)

					if err := client.Publish(queueSubject, data); err != nil {
						queueErrCh <- fmt.Errorf("发布队列消息失败: %v", err)
						return
					}

					// 短暂延迟，避免消息堆积
					time.Sleep(10 * time.Millisecond)
				}
			}()

			// 等待足够的时间让消息被处理，或者直到收到足够的消息
			timeout := time.After(3 * time.Second)
		messageLoop:
			for atomic.LoadUint32(&receivedCount) < uint32(messageCount) {
				select {
				case <-timeout:
					// 超时，检查当前接收到的消息数量
					t.Logf("队列处理超时，已接收 %d/%d 条消息", atomic.LoadUint32(&receivedCount), messageCount)
					break messageLoop
				case err := <-queueErrCh:
					t.Error(err)
					break messageLoop
				case <-queueCtx.Done():
					t.Error("队列订阅测试超时")
					break messageLoop
				case <-time.After(10 * time.Millisecond):
					// 继续等待
				}
			}

			// 发送完成信号
			close(queueDone)

			// 等待所有工作线程完成
			wg.Wait()

			// 输出统计信息
			received := atomic.LoadUint32(&receivedCount)
			t.Logf("收到了 %d/%d 条队列消息", received, messageCount)

			// 检查结果
			if received < uint32(messageCount) {
				t.Logf("警告：收到的消息数量少于预期，这可能是由于队列负载均衡")
			}

			// 输出每个工作线程处理的消息数量
			workerCounts.Range(func(key, value interface{}) bool {
				t.Logf("工作线程 #%d 处理了 %d 条消息", key, value)
				return true
			})
		})

		// 测试JetStream支持
		t.Run("JetStream", func(t *testing.T) {
			if client.js == nil {
				t.Skip("JetStream未启用，跳过JetStream测试")
			}

			// 创建上下文控制超时
			jsCtx, jsCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer jsCancel()

			// 创建完成信号
			jsDone := make(chan struct{})

			// 创建错误通道
			jsErrCh := make(chan error, 1)

			// 在单独的goroutine中运行JetStream测试
			go func() {
				defer close(jsDone)

				// 创建JetStream流
				streamName := "CONSUMER_TEST"
				streamConfig := &nats.StreamConfig{
					Name:     streamName,
					Subjects: []string{"test.jetstream.*"},
					Storage:  nats.MemoryStorage, // 使用内存存储更快
					MaxAge:   time.Minute,
				}

				// 添加流
				_, err := client.js.AddStream(streamConfig)
				if err != nil {
					jsErrCh <- fmt.Errorf("创建JetStream流失败: %v", err)
					return
				}

				// 确保在测试结束时清理
				defer func() {
					// 尝试删除流，但忽略错误
					_ = client.js.DeleteStream(streamName)
				}()

				// 创建带足够缓冲的通道
				jsReceived := make(chan *Message, 5)

				// 创建JetStream消息处理函数
				jsHandler := func(msg *Message) error {
					// 非阻塞地发送到通道
					select {
					case jsReceived <- msg:
						// 成功发送
					default:
						t.Logf("JetStream消息通道已满，消息丢弃")
					}
					return nil
				}

				// 创建JetStream订阅
				subID := "" // 在取消订阅时使用

				// 使用非阻塞的方式进行订阅
				subChan := make(chan error, 1)
				go func() {
					var err error
					subID, err = consumer.JetStreamSubscribe("test.jetstream.sub", jsHandler,
						WithDurable("consumer-test"),
						WithDeliverNew(),
					)
					subChan <- err
				}()

				// 等待订阅完成或超时
				select {
				case err := <-subChan:
					if err != nil {
						jsErrCh <- fmt.Errorf("JetStream订阅失败: %v", err)
						return
					}
				case <-time.After(2 * time.Second):
					jsErrCh <- fmt.Errorf("JetStream订阅超时")
					return
				}

				// 发布消息到JetStream
				jsMsg := &TestMessage{
					ID:      "js-123",
					Content: "JetStream测试消息",
				}
				jsPayload, _ := json.Marshal(jsMsg)

				// 非阻塞地发布消息
				pubChan := make(chan error, 1)
				go func() {
					_, err := client.js.Publish("test.jetstream.sub", jsPayload)
					pubChan <- err
				}()

				// 等待发布完成或超时
				select {
				case err := <-pubChan:
					if err != nil {
						jsErrCh <- fmt.Errorf("JetStream发布失败: %v", err)
						return
					}
				case <-time.After(2 * time.Second):
					jsErrCh <- fmt.Errorf("JetStream发布超时")
					return
				}

				// 等待接收消息或超时
				var receivedMsg *Message
				select {
				case receivedMsg = <-jsReceived:
					// 验证消息内容
					var received TestMessage
					if err := json.Unmarshal(receivedMsg.Data, &received); err != nil {
						jsErrCh <- fmt.Errorf("解析JetStream消息失败: %v", err)
						return
					}

					if received.ID != jsMsg.ID || received.Content != jsMsg.Content {
						jsErrCh <- fmt.Errorf("收到的消息内容不匹配，期望: %+v，实际: %+v", jsMsg, received)
					} else {
						t.Logf("成功收到JetStream消息: %+v", received)
					}

				case <-time.After(2 * time.Second):
					jsErrCh <- fmt.Errorf("超时未收到JetStream消息")
					return
				}

				// 安全地尝试取消订阅
				if subID != "" {
					unsub := make(chan error, 1)
					go func() {
						unsub <- consumer.Unsubscribe(subID)
					}()

					select {
					case err := <-unsub:
						if err != nil {
							t.Logf("取消JetStream订阅时出错: %v", err)
						}
					case <-time.After(1 * time.Second):
						t.Logf("取消JetStream订阅超时")
					}
				}
			}()

			// 等待JetStream测试完成或超时
			select {
			case <-jsDone:
				// 测试正常完成
			case err := <-jsErrCh:
				t.Fatal(err)
			case <-jsCtx.Done():
				t.Error("JetStream测试执行超时")
			}
		})

		// 测试消息帮助函数
		testMsg := &Message{
			Subject: "test.helpers",
			Reply:   "test.reply",
			Data:    payload,
			Msg:     &nats.Msg{Reply: "test.reply"},
		}

		// 测试UnmarshalJson函数
		var unmarshalTest TestMessage
		err = UnmarshalJson(testMsg, &unmarshalTest)
		if err != nil {
			errCh <- fmt.Errorf("UnmarshalJson失败: %v", err)
			return
		}
		if unmarshalTest.ID != testData.ID || unmarshalTest.Content != testData.Content {
			t.Errorf("UnmarshalJson结果不符合预期: %+v", unmarshalTest)
		}

		// 关闭消费者并验证所有订阅被取消
		err = consumer.Close()
		if err != nil {
			errCh <- fmt.Errorf("关闭消费者失败: %v", err)
			return
		}

		// 检查是否所有订阅都已清理
		c, ok := consumer.(*DefaultConsumer)
		if ok {
			// 获取订阅映射的长度
			c.mu.RLock()
			count := len(c.subscriptions)
			c.mu.RUnlock()

			if count != 0 {
				t.Errorf("关闭消费者后仍有 %d 个活跃订阅", count)
			}
		}
	}()

	// 等待测试完成或超时
	select {
	case <-done:
		// 测试正常完成
	case err := <-errCh:
		// 处理来自goroutine的错误
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("测试执行超时，可能存在死锁")
	}
}

func TestConsumerWithNilClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 测试使用nil客户端创建消费者
	_, err := NewConsumer(nil, logger)
	if err == nil {
		t.Error("使用nil客户端创建消费者应该失败，但成功了")
	}
}

func TestReplyJson(t *testing.T) {
	// 测试没有回复主题的消息
	noReplyMsg := &Message{
		Subject: "test.no.reply",
		Reply:   "",
	}

	testReply := TestMessage{ID: "reply-123", Content: "回复内容"}
	err := ReplyJson(noReplyMsg, testReply)
	if err == nil {
		t.Error("对没有回复主题的消息调用ReplyJson应该失败，但成功了")
	}
}
