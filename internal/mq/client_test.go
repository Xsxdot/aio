package mq

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func TestNatsClient(t *testing.T) {
	// 创建日志记录器
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "nats-client-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建服务器配置
	serverConfig := NewDefaultServerConfig("client-test-server", tempDir)
	serverConfig.Port = 14223 // 使用非默认端口避免冲突
	// 不设置集群端口，避免需要集群配置

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

	// 测试配置选项
	WithClientCredentials("testuser", "testpass")(config)
	if config.Username != "testuser" || config.Password != "testpass" {
		t.Error("凭证配置未正确应用")
	}

	WithClientTLS(
		tempDir+"/cert.pem",
		tempDir+"/key.pem",
		tempDir+"/ca.pem",
	)(config)
	if config.TLS == nil {
		t.Error("TLS配置未正确应用")
	}

	// 移除TLS配置（因为我们没有实际的证书文件）
	config.TLS = nil

	// 创建客户端
	client, err := NewNatsClient(config, logger)
	if err != nil {
		t.Fatalf("创建NATS客户端失败: %v", err)
	}

	// 验证客户端实例
	if client.conn == nil {
		t.Error("客户端连接为空")
	}

	// 测试发布和订阅基本功能
	testSubject := "test.subject"
	testPayload := []byte("hello nats")

	received := make(chan bool, 1)

	// 订阅消息
	sub, err := client.Subscribe(testSubject, func(msg *nats.Msg) {
		if string(msg.Data) == string(testPayload) {
			received <- true
		}
	})
	if err != nil {
		t.Fatalf("订阅主题失败: %v", err)
	}
	defer sub.Unsubscribe()

	// 发布消息
	err = client.Publish(testSubject, testPayload)
	if err != nil {
		t.Fatalf("发布消息失败: %v", err)
	}

	// 等待接收消息
	select {
	case <-received:
		// 成功接收
	case <-time.After(2 * time.Second):
		t.Error("超时未收到消息")
	}

	// 测试JetStream功能
	if config.UseJetStream {
		js := client.GetJetStream()
		if js == nil {
			t.Error("JetStream上下文为空")
		}
	}

	// 关闭客户端
	client.Close()

	// 测试全局实例
	err = InitGlobalNatsClient(config, logger)
	if err != nil {
		t.Fatalf("初始化全局NATS客户端失败: %v", err)
	}

	// 检查全局实例
	globalClient := GetGlobalNatsClient()
	if globalClient == nil {
		t.Error("全局客户端实例为空")
	}

	// 关闭全局实例
	CloseGlobalNatsClient()
	if GlobalNatsClient != nil {
		t.Error("关闭全局客户端后，全局实例仍然存在")
	}

	// 测试连接到不存在的服务器
	badConfig := NewDefaultClientConfig()
	badConfig.Servers = []string{"nats://localhost:65000"} // 使用一个不太可能被使用的端口
	badConfig.ConnectTimeout = 1 * time.Second             // 设置较短的超时时间以加快测试

	_, err = NewNatsClient(badConfig, logger)
	if err == nil {
		t.Error("连接到不存在的服务器应该失败，但成功了")
	} else {
		t.Logf("预期的错误: %v", err)
	}
}
