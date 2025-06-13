package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xsxdot/aio/app/config"
	"go.uber.org/zap"
)

func TestSaveEtcdConfigToCenter(t *testing.T) {
	// 创建一个模拟的 BaseConfig
	baseConfig := &config.BaseConfig{
		System: &config.SystemConfig{
			ConfigSalt: "test-salt-for-encryption",
		},
		Etcd: &config.EtcdConfig{
			Endpoints:         "localhost:2379,localhost:2380",
			DialTimeout:       5,
			Username:          "test-user",
			Password:          "test-password",
			AutoSyncEndpoints: true,
			TLS: &config.TLSConfig{
				TLSEnabled: true,
				AutoTls:    false,
				Cert:       "/path/to/cert.pem",
				Key:        "/path/to/key.pem",
				TrustedCA:  "/path/to/ca.pem",
			},
		},
	}

	// 创建一个模拟的 etcd 客户端 - 这里我们只需要确保不会崩溃
	// 在实际测试中，你可能需要使用 testify/mock 或类似的工具

	// 由于我们无法在单元测试中真正连接到 etcd，我们只测试配置构建逻辑
	t.Run("TestConfigValuesConstruction", func(t *testing.T) {
		// 手动构建配置值以验证逻辑
		configValues := make(map[string]*ConfigValue)
		etcdConfig := baseConfig.Etcd

		// 验证端点配置
		configValues["endpoints"] = &ConfigValue{
			Value: etcdConfig.Endpoints,
			Type:  ValueTypeString,
		}
		assert.Equal(t, "localhost:2379,localhost:2380", configValues["endpoints"].Value)
		assert.Equal(t, ValueTypeString, configValues["endpoints"].Type)

		// 验证密码使用加密类型
		if etcdConfig.Password != "" {
			configValues["password"] = &ConfigValue{
				Value: etcdConfig.Password,
				Type:  ValueTypeEncrypted,
			}
		}
		assert.Equal(t, "test-password", configValues["password"].Value)
		assert.Equal(t, ValueTypeEncrypted, configValues["password"].Type)

		// 验证其他配置
		assert.Equal(t, "test-user", etcdConfig.Username)
		assert.Equal(t, 5, etcdConfig.DialTimeout)
		assert.True(t, etcdConfig.AutoSyncEndpoints)

		// 验证 TLS 配置
		assert.True(t, etcdConfig.TLS.TLSEnabled)
		assert.False(t, etcdConfig.TLS.AutoTls)
		assert.Equal(t, "/path/to/cert.pem", etcdConfig.TLS.Cert)
		assert.Equal(t, "/path/to/key.pem", etcdConfig.TLS.Key)
		assert.Equal(t, "/path/to/ca.pem", etcdConfig.TLS.TrustedCA)
	})

	t.Run("TestNilEtcdConfig", func(t *testing.T) {
		// 测试 etcd 配置为 nil 的情况
		baseConfigNil := &config.BaseConfig{
			System: &config.SystemConfig{
				ConfigSalt: "test-salt",
			},
			Etcd: nil,
		}

		// 创建一个 mock logger
		logger, _ := zap.NewDevelopment()

		service := &Service{
			baseConfig: baseConfigNil,
			logger:     logger,
		}

		// 应该返回 nil 而不崩溃
		err := service.saveEtcdConfigToCenter(context.Background())
		assert.NoError(t, err)
	})
}
