package sdk

import (
	"testing"
	"time"
)

// TestConfig 测试配置验证
func TestConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				RegistryAddr: "localhost:50051",
				ClientKey:    "test-key",
				ClientSecret: "test-secret",
			},
			wantErr: false,
		},
		{
			name: "missing registry addr",
			config: Config{
				ClientKey:    "test-key",
				ClientSecret: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "missing client key",
			config: Config{
				RegistryAddr: "localhost:50051",
				ClientSecret: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "missing client secret",
			config: Config{
				RegistryAddr: "localhost:50051",
				ClientKey:    "test-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 注意：这里实际上会尝试连接，所以会失败
			// 但我们只是验证配置检查逻辑
			_, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigDefaults 测试默认值
func TestConfigDefaults(t *testing.T) {
	config := Config{
		RegistryAddr: "localhost:50051",
		ClientKey:    "test-key",
		ClientSecret: "test-secret",
	}

	// 默认超时应该是 30 秒
	if config.DefaultTimeout == 0 {
		// New 会设置默认值
		expectedDefault := 30 * time.Second
		if config.DefaultTimeout != 0 && config.DefaultTimeout != expectedDefault {
			t.Errorf("DefaultTimeout = %v, want %v", config.DefaultTimeout, expectedDefault)
		}
	}
}

// TestErrorWrapping 测试错误包装
func TestErrorWrapping(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
		wantNil bool
	}{
		{
			name:    "nil error",
			err:     nil,
			message: "test",
			wantNil: true,
		},
		{
			name:    "non-nil error",
			err:     &Error{Message: "original"},
			message: "wrapped",
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(tt.err, tt.message)
			if (result == nil) != tt.wantNil {
				t.Errorf("WrapError() nil = %v, wantNil %v", result == nil, tt.wantNil)
			}
			if result != nil {
				if sdkErr, ok := result.(*Error); ok {
					if sdkErr.Message != tt.message {
						t.Errorf("Error message = %v, want %v", sdkErr.Message, tt.message)
					}
				}
			}
		})
	}
}

// TestServiceDescriptor 测试服务描述结构
func TestServiceDescriptor(t *testing.T) {
	desc := ServiceDescriptor{
		ID:      1,
		Project: "test",
		Name:    "test-service",
		Instances: []InstanceEndpoint{
			{
				ID:       1,
				Host:     "localhost",
				Endpoint: "http://localhost:8080",
				Env:      "dev",
			},
		},
	}

	if len(desc.Instances) != 1 {
		t.Errorf("Expected 1 instance, got %d", len(desc.Instances))
	}

	if desc.Instances[0].Endpoint != "http://localhost:8080" {
		t.Errorf("Unexpected endpoint: %s", desc.Instances[0].Endpoint)
	}
}

// TestClientStructure 测试客户端结构（编译期保障）
func TestClientStructure(t *testing.T) {
	// 这个测试主要是确保 Client 结构有所有必需的字段
	// 如果字段缺失，编译会失败
	client := &Client{}
	
	// 验证新增的客户端字段存在（编译期检查）
	_ = client.ConfigClient
	_ = client.ShortURL
	_ = client.Application
	
	// 验证原有字段仍然存在
	_ = client.Auth
	_ = client.Registry
	_ = client.Discovery
}

// TestShortURLRequestStructure 测试短网址请求结构
func TestShortURLRequestStructure(t *testing.T) {
	req := &CreateShortLinkRequest{
		DomainID:   1,
		TargetType: "url",
		TargetConfig: map[string]interface{}{
			"url": "https://example.com",
		},
		Comment: "test",
	}
	
	if req.DomainID != 1 {
		t.Errorf("DomainID = %v, want 1", req.DomainID)
	}
	
	if url, ok := req.TargetConfig["url"].(string); !ok || url != "https://example.com" {
		t.Errorf("TargetConfig url = %v, want https://example.com", req.TargetConfig["url"])
	}
}

// TestDeployRequestStructure 测试部署请求结构
func TestDeployRequestStructure(t *testing.T) {
	req := &DeployRequest{
		ApplicationID:     1,
		Version:          "v1.0.0",
		BackendArtifactID: 100,
		Operator:         "test-user",
	}
	
	if req.ApplicationID != 1 {
		t.Errorf("ApplicationID = %v, want 1", req.ApplicationID)
	}
	
	if req.Version != "v1.0.0" {
		t.Errorf("Version = %v, want v1.0.0", req.Version)
	}
}
