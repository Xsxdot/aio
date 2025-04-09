package mq

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNatsServer(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "nats-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建日志记录器
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 创建默认配置
	config := NewDefaultServerConfig("test-server", tempDir)
	config.Port = 14227 // 使用非默认端口避免冲突
	// 不设置集群端口

	// 测试 WithXXX 选项函数
	WithTLS(filepath.Join(tempDir, "cert.pem"), filepath.Join(tempDir, "key.pem"))(config)
	WithAuth("testuser", "testpass")(config)
	WithCluster("test-cluster", []string{"nats://localhost:14248"})(config)
	WithJetStream(1024*1024*10, 1024*1024*100)(config)

	// 检查配置
	if config.TLSEnabled != true {
		t.Error("TLS配置未正确应用")
	}
	if config.Username != "testuser" || config.Password != "testpass" {
		t.Error("身份验证配置未正确应用")
	}
	if config.ClusterName != "test-cluster" {
		t.Error("集群配置未正确应用")
	}
	if config.JetStreamEnabled != true {
		t.Error("JetStream配置未正确应用")
	}

	// 还原TLS设置，避免测试失败（因为我们没有实际的证书文件）
	config.TLSEnabled = false

	// 创建服务器
	server, err := NewNatsServer(config, logger)
	if err != nil {
		t.Fatalf("创建NATS服务器失败: %v", err)
	}

	// 验证服务器实例
	if server.server == nil {
		t.Error("服务器实例为空")
	}

	// 获取服务器信息
	info := server.GetInfo()
	if info == nil {
		t.Error("获取服务器信息失败")
	} else {
		t.Logf("服务器信息: %v", info)
	}

	// 关闭服务器
	server.Close()

	// 测试全局实例
	err = InitGlobalNatsServer(config, logger)
	if err != nil {
		t.Fatalf("初始化全局NATS服务器失败: %v", err)
	}

	// 检查全局实例
	globalServer := GetGlobalNatsServer()
	if globalServer == nil {
		t.Error("全局服务器实例为空")
	}

	// 关闭全局实例
	CloseGlobalNatsServer()
	if GlobalNatsServer != nil {
		t.Error("关闭全局服务器后，全局实例仍然存在")
	}
}

func TestNatsServerStartupTimeout(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "nats-test-timeout")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建日志记录器
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 创建一个会导致启动超时的配置
	config1 := NewDefaultServerConfig("test-server-1", tempDir)
	config1.Port = 24222 // 使用不同的端口

	// 创建另一个使用同样端口的配置，导致端口冲突
	config2 := NewDefaultServerConfig("test-server-2", tempDir)
	config2.Port = 24222 // 同样的端口

	// 先启动第一个服务器
	server1, err := NewNatsServer(config1, logger)
	if err != nil {
		t.Fatalf("创建第一个NATS服务器失败: %v", err)
	}
	defer server1.Close()

	// 让服务器完全启动
	time.Sleep(100 * time.Millisecond)

	// 尝试在同一端口启动第二个服务器，应该会失败
	_, err = NewNatsServer(config2, logger)
	if err == nil {
		t.Error("在同一端口启动第二个服务器应该失败，但成功了")
	} else {
		t.Logf("预期的错误: %v", err)
	}
}
