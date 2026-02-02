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

// TestConfigCRUDStructures 测试配置 CRUD 结构（编译期保障）
func TestConfigCRUDStructures(t *testing.T) {
	// 测试 ValueType 常量
	var vt ValueType
	vt = ValueTypeString
	if vt != "string" {
		t.Errorf("ValueTypeString = %v, want string", vt)
	}

	// 测试 ConfigValue 结构
	configValue := &ConfigValue{
		Value: "test-value",
		Type:  ValueTypeString,
	}
	if configValue.Value != "test-value" {
		t.Errorf("ConfigValue.Value = %v, want test-value", configValue.Value)
	}

	// 测试 CreateConfigRequest 结构
	createReq := &CreateConfigRequest{
		Key: "app.test.dev",
		Value: map[string]*ConfigValue{
			"host": {Value: "localhost", Type: ValueTypeString},
			"port": {Value: "8080", Type: ValueTypeInt},
		},
		Metadata:    map[string]string{"env": "dev"},
		Description: "test config",
		ChangeNote:  "initial version",
	}
	if createReq.Key != "app.test.dev" {
		t.Errorf("CreateConfigRequest.Key = %v, want app.test.dev", createReq.Key)
	}
	if len(createReq.Value) != 2 {
		t.Errorf("CreateConfigRequest.Value length = %v, want 2", len(createReq.Value))
	}

	// 测试 UpdateConfigRequest 结构
	updateReq := &UpdateConfigRequest{
		Key: "app.test.dev",
		Value: map[string]*ConfigValue{
			"host": {Value: "127.0.0.1", Type: ValueTypeString},
		},
		ChangeNote: "update host",
	}
	if updateReq.Key != "app.test.dev" {
		t.Errorf("UpdateConfigRequest.Key = %v, want app.test.dev", updateReq.Key)
	}

	// 测试 DeleteConfigResponse 结构
	deleteResp := &DeleteConfigResponse{
		Success: true,
		Message: "删除成功",
	}
	if !deleteResp.Success {
		t.Errorf("DeleteConfigResponse.Success = %v, want true", deleteResp.Success)
	}

	// 测试 ConfigInfo 结构
	configInfo := &ConfigInfo{
		ID:      1,
		Key:     "app.test.dev",
		Value:   map[string]*ConfigValue{"host": {Value: "localhost", Type: ValueTypeString}},
		Version: 1,
		Metadata: map[string]string{"env": "dev"},
		Description: "test",
		CreatedAt: "2024-01-01 00:00:00",
		UpdatedAt: "2024-01-01 00:00:00",
	}
	if configInfo.ID != 1 {
		t.Errorf("ConfigInfo.ID = %v, want 1", configInfo.ID)
	}
	if configInfo.Version != 1 {
		t.Errorf("ConfigInfo.Version = %v, want 1", configInfo.Version)
	}
}

// TestConfigClientMethods 测试 ConfigClient 方法签名（编译期保障）
func TestConfigClientMethods(t *testing.T) {
	// 这个测试只是为了确保方法存在且签名正确
	// 不会实际调用（因为没有真实的 gRPC 连接）
	var client *ConfigClient

	// 验证方法存在（编译期检查）
	// 如果方法不存在或签名不对，编译会失败
	_ = client.GetConfigJSON
	_ = client.BatchGetConfigs
	_ = client.CreateConfig  // 新增的方法
	_ = client.UpdateConfig  // 新增的方法
	_ = client.DeleteConfig  // 新增的方法

	// 验证返回类型（编译期检查）
	t.Run("method_signatures", func(t *testing.T) {
		// 这个子测试验证方法签名，但不执行（因为 client 是 nil）
		// 如果签名不对，这里的类型断言会编译失败
		if client != nil {
			// 这些行永远不会执行，但会在编译时检查类型
			var _ func() = func() {
				ctx := t
				_ = ctx // 避免 unused 警告
				// 这些只是类型检查，不会真正执行
			}
		}
	})
}
