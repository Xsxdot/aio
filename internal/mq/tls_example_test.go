package mq

import (
	"fmt"
	"testing"
)

// TestTLSConfigExample 这不是一个真正的测试，而是一个示例
// 展示如何正确配置服务端和客户端的TLS
func TestTLSConfigExample(t *testing.T) {
	// 跳过此测试，因为它只是一个示例
	t.Skip("这只是一个配置示例，不是真正的测试")

	// 步骤1: 配置服务端TLS
	serverConfig := NewDefaultServerConfig("secure-nats", "/path/to/data")

	// 启用服务端TLS
	serverConfig.TLSEnabled = true
	serverConfig.CertFile = "/path/to/server/cert.pem"
	serverConfig.KeyFile = "/path/to/server/key.pem"
	serverConfig.ClientCAFile = "/path/to/ca/ca.pem"
	serverConfig.VerifyClients = true // 要求客户端提供证书

	// 如果需要配置集群TLS
	serverConfig.ClusterTLSEnabled = true
	serverConfig.ClusterCertFile = "/path/to/cluster/cert.pem"
	serverConfig.ClusterKeyFile = "/path/to/cluster/key.pem"
	serverConfig.ClusterCAFile = "/path/to/ca/ca.pem"

	// 也可以使用选项函数配置
	serverConfig = NewDefaultServerConfig("secure-nats", "/path/to/data")
	WithTLS("/path/to/server/cert.pem", "/path/to/server/key.pem")(serverConfig)
	WithClientAuth("/path/to/ca/ca.pem")(serverConfig)

	// 步骤2: 配置客户端TLS
	clientConfig := NewDefaultClientConfig()
	clientConfig.TLS = &TLSConfig{
		CertFile:      "/path/to/client/cert.pem",
		KeyFile:       "/path/to/client/key.pem",
		TrustedCAFile: "/path/to/ca/ca.pem",
	}

	// 或者使用选项函数
	clientConfig = NewDefaultClientConfig()
	WithClientTLS(
		"/path/to/client/cert.pem",
		"/path/to/client/key.pem",
		"/path/to/ca/ca.pem",
	)(clientConfig)

	fmt.Println("TLS配置示例完成")
}
